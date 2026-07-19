package ats

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/avature"
)

var _ Adapter = (*AvatureAdapter)(nil)

// avatureLocaleSegmentRE matches a portal locale path segment such as
// "en_US"; careers URLs may carry one before the portal name.
var avatureLocaleSegmentRE = regexp.MustCompile(`^[a-z]{2}_[A-Z]{2}$`)

// AvatureAdapter serves public Avature career portals. Search and detail
// are server-rendered HTML (see internal/provider/avature/doc.go). Roster
// slugs are portal bases without the scheme (e.g. "koch.avature.net/careers"
// — one tenant can host several portals, so the portal name is part of the
// key). ParseCareersURL also accepts any *.avature.net portal URL so
// uncurated tenants work when passed as a careers URL; custom-domain portals
// (e.g. careers.unifiservice.com) resolve through the roster only.
type AvatureAdapter struct {
	hc      *http.Client
	baseURL func(slug string) string
}

func NewAvatureAdapter(hc *http.Client) *AvatureAdapter {
	return &AvatureAdapter{
		hc:      hc,
		baseURL: func(slug string) string { return "https://" + slug },
	}
}

func (a *AvatureAdapter) Name() string { return "avature" }

func (a *AvatureAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(avature.Companies))
	for _, c := range avature.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Slug(), Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes any *.avature.net portal URL, dropping an
// optional locale segment (e.g. /en_US/careers/SearchJobs and
// /careers/JobDetail/... both yield "<host>/careers"). A bare host is
// rejected: the portal name cannot be inferred.
func (a *AvatureAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".avature.net") || host == "www.avature.net" {
		return "", false
	}
	segs := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	if len(segs) > 0 && avatureLocaleSegmentRE.MatchString(segs[0]) {
		segs = segs[1:]
	}
	if len(segs) == 0 || segs[0] == "" {
		return "", false
	}
	return host + "/" + strings.ToLower(segs[0]), true
}

// Search maps the unified page onto jobOffset requests. The portal page
// size is tenant-configured and cannot be raised, so one unified page may
// stitch several consecutive upstream fetches; jobOffset accepts arbitrary
// offsets, which keeps the stitch a plain cursor walk.
func (a *AvatureAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	base, _, err := a.resolvePortal(slug)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.Location) != "" {
		return nil, errors.New("avature: location filtering is not supported — the portals expose only a full-text keyword search (search covers descriptions, so location terms over-match); omit location for this company")
	}
	if len(p.Filters) > 0 {
		return nil, errors.New("avature: no filters are available for this company; the portal facets have no public server-side surface")
	}

	page := clampPage(p.Page)
	pageIndex := page - 1
	// Page is user-controlled with no schema maximum; reject values that
	// would overflow the start offset (same guard as the Workday adapter).
	if pageIndex > math.MaxInt/pageSize {
		return nil, fmt.Errorf("avature: page %d is too large; retry with a smaller page", page)
	}
	start := pageIndex * pageSize

	client := avature.NewClient(base, a.hc)
	query := strings.TrimSpace(p.Query)

	var (
		collected []avature.Job
		seen      = make(map[string]bool)
		total     = -1
		hasNext   bool
	)
	for {
		res, err := client.Search(ctx, &avature.SearchRequest{Search: query, Offset: start + len(collected)})
		if err != nil {
			return nil, fmt.Errorf("avature: search %q: %w", slug, err)
		}
		if res.Total >= 0 {
			total = res.Total
		}
		hasNext = res.HasNext
		fresh := 0
		for _, j := range res.Jobs {
			if seen[j.ID] {
				continue
			}
			seen[j.ID] = true
			collected = append(collected, j)
			fresh++
		}
		// fresh == 0 also breaks on a portal that ignored jobOffset and
		// replayed the same page — repeating the request cannot progress.
		if fresh == 0 || !res.HasNext || len(collected) >= pageSize {
			break
		}
	}

	// Legend-less portals (e.g. Koch) leave the count unknowable from one
	// page; report the lower bound the walk proved, +1 when a next page
	// exists so TotalPages still signals it.
	if total < 0 {
		total = start + len(collected)
		if hasNext {
			total++
		}
	}

	jobs := collected
	if len(jobs) > pageSize {
		jobs = jobs[:pageSize]
	}
	summaries := make([]JobSummary, 0, len(jobs))
	for _, j := range jobs {
		summaries = append(summaries, JobSummary{
			JobID:    j.ID,
			Title:    j.Title,
			Location: j.Location,
			URL:      j.URL,
		})
	}
	return &SearchResult{
		Jobs:       summaries,
		TotalCount: total,
		Page:       page,
		TotalPages: totalPages(total),
	}, nil
}

// Filters reports no dimensions: portal facets use per-tenant numeric field
// ids whose options load only via JS autocomplete, so there is nothing
// portable to offer.
func (a *AvatureAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	if _, _, err := a.resolvePortal(slug); err != nil {
		return nil, err
	}
	return FilterSet{}, nil
}

func (a *AvatureAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	base, companyName, err := a.resolvePortal(slug)
	if err != nil {
		return nil, err
	}
	client := avature.NewClient(base, a.hc)
	d, err := client.JobDetail(ctx, jobID)
	if errors.Is(err, avature.ErrJobNotFound) {
		return nil, fmt.Errorf(
			"avature: job %q not found for company %q; pass a job_id exactly as returned by the job search",
			jobID, slug,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("avature: fetch job %q for %q: %w", jobID, slug, err)
	}

	desc := d.DescriptionHTML
	if desc != "" {
		if text, err := html2text.FromString(desc, html2text.Options{}); err == nil {
			desc = text
		}
	}

	// Multi-brand portals (e.g. Koch) name the hiring subsidiary in a
	// "Company" field; the roster display name still wins for consistency
	// with the registry's resolution.
	company := companyName
	if company == "" {
		company = d.Company()
	}

	return &JobDetail{
		JobID:       jobID,
		Title:       d.Title,
		Company:     company,
		Location:    d.Location(),
		URL:         d.URL,
		Description: desc,
	}, nil
}

// resolvePortal returns the portal base URL and a display name (empty when
// the slug is not on the curated roster).
func (a *AvatureAdapter) resolvePortal(slug string) (base, name string, err error) {
	key := strings.ToLower(strings.TrimSpace(slug))
	if c, ok := avature.CompaniesBySlug[key]; ok {
		return a.baseURL(c.Slug()), c.Name, nil
	}
	// Careers-URL path: accept an uncurated <host>/<portal> slug, but only
	// on Avature's own domain — a custom-domain portal cannot be verified
	// as Avature from its slug alone.
	if host, portal, ok := strings.Cut(key, "/"); ok &&
		strings.HasSuffix(host, ".avature.net") && host != ".avature.net" &&
		portal != "" && !strings.Contains(portal, "/") {
		return a.baseURL(key), "", nil
	}
	return "", "", fmt.Errorf("avature: unknown company %q; pass a roster company or an Avature careers URL", slug)
}
