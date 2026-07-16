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
	"time"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/icims"
)

var _ Adapter = (*ICIMSAdapter)(nil)

// icimsCareersHostRE matches *.icims.com hosts. login*, cdn*, api.*, and
// www.icims.com are rejected after the match — they are not job boards.
// Host labels must start and end alphanumeric (stricter than a bare
// HasSuffix check); intentional, since iCIMS has no roster gate and the
// host is the slug. Go's RE2 has no negative lookahead.
//
// Examples (hostname):
//   - careers-peraton.icims.com
//   - uscareers-example.icims.com
//
// Rejected:
//   - login.icims.com
//   - www.icims.com
var icimsCareersHostRE = regexp.MustCompile(
	`(?i)^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\.icims\.com$`,
)

// ICIMSAdapter serves public iCIMS career portals. Search and detail are
// server-rendered HTML (see internal/provider/icims/openapi.yaml). Roster
// slugs are lowercase portal hostnames (e.g. careers-peraton.icims.com).
// ParseCareersURL also accepts any *.icims.com host so uncurated tenants
// work when passed as a careers URL.
type ICIMSAdapter struct {
	hc      *http.Client
	baseURL func(host string) string
}

func NewICIMSAdapter(hc *http.Client) *ICIMSAdapter {
	return &ICIMSAdapter{
		hc:      hc,
		baseURL: func(host string) string { return "https://" + host },
	}
}

func (a *ICIMSAdapter) Name() string { return "icims" }

func (a *ICIMSAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(icims.Companies))
	for _, c := range icims.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Host, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes any *.icims.com host. Curated roster names are
// preferred via the registry's name index; uncurated hosts still resolve so
// callers can pass a careers URL directly.
func (a *ICIMSAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	host := strings.ToLower(u.Hostname())
	if !icimsCareersHostRE.MatchString(host) ||
		strings.HasPrefix(host, "login") || strings.HasPrefix(host, "cdn") ||
		strings.HasPrefix(host, "api.") || host == "www.icims.com" {
		return "", false
	}
	return host, true
}

func (a *ICIMSAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	host, _, err := a.resolveHost(slug)
	if err != nil {
		return nil, err
	}

	page := clampPage(p.Page)
	pageIndex := page - 1
	// Page is user-controlled with no schema maximum; reject values that
	// would overflow the start offset (same guard as the Workday adapter).
	if pageIndex > math.MaxInt/pageSize {
		return nil, fmt.Errorf("icims: page %d is too large; retry with a smaller page", page)
	}
	start := pageIndex * pageSize

	client := icims.NewClient(a.baseURL(host), a.hc)
	keyword := strings.TrimSpace(p.Query)
	location := strings.TrimSpace(p.Location)

	// Probe pr=0 with the keyword only to learn the portal's select options
	// and, when no other constraint applies, the stable upstream page size.
	probe, err := client.Search(ctx, &icims.SearchRequest{Keyword: keyword, Page: 0})
	if err != nil {
		return nil, fmt.Errorf("icims: search %q: %w", host, err)
	}

	base := icims.SearchRequest{Keyword: keyword}
	base.Categories, base.PositionTypes, err = icimsResolveFilters(probe, p.Filters)
	if err != nil {
		return nil, err
	}

	if location != "" {
		locValues := icims.MatchLocationOptions(probe.Locations, location)
		if len(locValues) == 0 {
			// Encoded option values that are not free-text matches still work
			// when the caller already holds a portal token.
			if !icims.LooksLikeLocationValue(location) {
				return &SearchResult{Jobs: []JobSummary{}, TotalCount: 0, Page: page, TotalPages: 0}, nil
			}
			locValues = []string{location}
		}
		// Every match is kept: the server ORs repeated searchLocation values
		// in one paginated query, and the encoded options stay the source of
		// truth because listing-card text may omit country or state tokens.
		base.Locations = locValues
	}

	// Discover the tenant page size from pr=0 under the active filters.
	// When TotalPages > 1, pr=0 is always a full page.
	first := probe
	hasServerFilters := len(base.Locations) > 0 || len(base.Categories) > 0 || len(base.PositionTypes) > 0
	if hasServerFilters {
		first, err = client.Search(ctx, &base)
		if err != nil {
			return nil, fmt.Errorf("icims: search %q: %w", host, err)
		}
	}

	upSize := first.PageSize
	totalPagesUp := first.TotalPages
	if upSize == 0 {
		return &SearchResult{Jobs: []JobSummary{}, TotalCount: 0, Page: page, TotalPages: 0}, nil
	}

	total, err := icimsExactTotal(ctx, client, base, first)
	if err != nil {
		return nil, fmt.Errorf("icims: search %q: %w", host, err)
	}
	if start >= total {
		return &SearchResult{Jobs: []JobSummary{}, TotalCount: total, Page: page, TotalPages: totalPages(total)}, nil
	}

	correctPr := start / upSize
	offsetInPage := start % upSize

	res := first
	if correctPr != 0 {
		r := base
		r.Page = correctPr
		res, err = client.Search(ctx, &r)
		if err != nil {
			return nil, fmt.Errorf("icims: search %q page %d: %w", host, correctPr, err)
		}
	}

	selected := make([]icims.Job, 0)
	if offsetInPage < len(res.Jobs) {
		selected = append(selected, res.Jobs[offsetInPage:]...)
	}

	// Stitch further upstream pages while the slice starts mid-page or the
	// tenant page size is smaller than the unified pageSize.
	for next := correctPr + 1; len(selected) < pageSize && next < totalPagesUp; next++ {
		r := base
		r.Page = next
		more, err := client.Search(ctx, &r)
		if err != nil {
			return nil, fmt.Errorf("icims: search %q page %d: %w", host, next, err)
		}
		selected = append(selected, more.Jobs...)
	}

	if len(selected) > pageSize {
		selected = selected[:pageSize]
	}

	return &SearchResult{
		Jobs:       icimsJobSummaries(selected, host),
		TotalCount: total,
		Page:       page,
		TotalPages: totalPages(total),
	}, nil
}

// icimsExactTotal returns the job count for the current filters.
// Single upstream page: len(first.Jobs). Multi-page: (pages-1)*pageSize +
// last page length, so a partial final page is not inflated to a full page.
func icimsExactTotal(ctx context.Context, client *icims.Client, base icims.SearchRequest, first *icims.SearchResponse) (int, error) {
	if first.TotalPages <= 1 {
		return len(first.Jobs), nil
	}
	last := base
	last.Page = first.TotalPages - 1
	res, err := client.Search(ctx, &last)
	if err != nil {
		return 0, err
	}
	return (first.TotalPages-1)*first.PageSize + len(res.Jobs), nil
}

// Filters reports the portal's category and position-type selects, resolved
// per tenant at call time. Tenants that do not render a select simply omit
// that dimension.
func (a *ICIMSAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	host, _, err := a.resolveHost(slug)
	if err != nil {
		return nil, err
	}
	client := icims.NewClient(a.baseURL(host), a.hc)
	probe, err := client.Search(ctx, &icims.SearchRequest{})
	if err != nil {
		return nil, fmt.Errorf("icims: filters %q: %w", host, err)
	}
	fs := FilterSet{}
	for _, d := range icimsFilterDims(probe) {
		if len(d.options) == 0 {
			continue
		}
		labels := make([]string, 0, len(d.options))
		for _, o := range d.options {
			labels = append(labels, o.Label)
		}
		fs[d.key] = labels
	}
	return fs, nil
}

// icimsFilterDim binds one unified filter key to a portal search select.
type icimsFilterDim struct {
	key     string
	options []icims.SelectOption
}

func icimsFilterDims(res *icims.SearchResponse) []icimsFilterDim {
	return []icimsFilterDim{
		{key: "category", options: res.Categories},
		{key: "positionType", options: res.PositionTypes},
	}
}

// icimsResolveFilters turns unified filter labels into the encoded option
// values the upstream expects. Unknown keys and values are teaching errors:
// the upstream silently ignores unrecognized encoded values and returns
// unfiltered results, so nothing unresolved may pass through.
func icimsResolveFilters(probe *icims.SearchResponse, filters FilterSet) (categories, positionTypes []string, err error) {
	if len(filters) == 0 {
		return nil, nil, nil
	}
	valid := make(map[string]bool)
	byKey := make(map[string][]icims.SelectOption)
	for _, d := range icimsFilterDims(probe) {
		if len(d.options) == 0 {
			continue
		}
		valid[d.key] = true
		byKey[d.key] = d.options
	}
	resolved := make(map[string][]string, len(filters))
	for key, values := range filters {
		options, ok := byKey[key]
		if !ok {
			return nil, nil, errUnknownFilterKey(key, valid)
		}
		for _, v := range values {
			value, ok := icimsResolveOption(options, v)
			if !ok {
				labels := make([]string, len(options))
				for i, o := range options {
					labels[i] = o.Label
				}
				return nil, nil, fmt.Errorf("filter value %q not found for %q; available: %s", v, key, strings.Join(labels, ", "))
			}
			resolved[key] = append(resolved[key], value)
		}
	}
	return resolved["category"], resolved["positionType"], nil
}

// icimsResolveOption matches a unified filter value against option labels
// (case-insensitive) or an exact encoded value.
func icimsResolveOption(options []icims.SelectOption, v string) (string, bool) {
	for _, o := range options {
		if strings.EqualFold(o.Label, v) || o.Value == v {
			return o.Value, true
		}
	}
	return "", false
}

func (a *ICIMSAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	host, companyName, err := a.resolveHost(slug)
	if err != nil {
		return nil, err
	}
	client := icims.NewClient(a.baseURL(host), a.hc)
	d, err := client.JobDetail(ctx, jobID)
	if errors.Is(err, icims.ErrJobNotFound) {
		return nil, fmt.Errorf(
			"icims: job %q not found for company %q; pass a job_id exactly as returned by the job search",
			jobID, slug,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("icims: fetch job %q for %q: %w", jobID, slug, err)
	}

	desc := d.DescriptionHTML
	if desc != "" {
		if text, err := html2text.FromString(desc, html2text.Options{}); err == nil {
			desc = text
		}
	}

	company := companyName
	if company == "" {
		company = d.Employer
	}

	return &JobDetail{
		JobID:       jobID,
		Title:       d.Title,
		Company:     company,
		Location:    d.Location,
		PostedAt:    icimsPostedAt(d.PostedAtRaw),
		URL:         icims.JobURL(host, jobID),
		Description: desc,
	}, nil
}

// resolveHost returns the portal host and a display name (empty when the
// host is not on the curated roster).
func (a *ICIMSAdapter) resolveHost(slug string) (host, name string, err error) {
	key := strings.ToLower(strings.TrimSpace(slug))
	if c, ok := icims.CompaniesByHost[key]; ok {
		return c.Host, c.Name, nil
	}
	// Careers-URL path: accept any *.icims.com host even if uncurated.
	if strings.HasSuffix(key, ".icims.com") && key != "icims.com" {
		return key, "", nil
	}
	return "", "", fmt.Errorf("icims: unknown company %q; pass a roster career-portal host or a *.icims.com careers URL", slug)
}

func icimsJobSummaries(jobs []icims.Job, host string) []JobSummary {
	out := make([]JobSummary, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, JobSummary{
			JobID:    j.ID,
			Title:    j.Title,
			Location: j.Location,
			PostedAt: icimsPostedAt(j.PostedAt),
			URL:      icims.JobURL(host, j.ID),
		})
	}
	return out
}

func icimsPostedAt(raw string) string {
	if raw == "" {
		return ""
	}
	// JSON-LD datePosted is typically ISO-8601 with Z; listing cards carry
	// US-style timestamps in the date span's title attribute.
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02",
		"1/2/2006 3:04 PM",
		"1/2/2006",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return isoDate(t)
		}
	}
	return raw
}
