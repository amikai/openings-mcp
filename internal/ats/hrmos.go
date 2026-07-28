package ats

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/amikai/openings-mcp/internal/provider/hrmos"
)

// hrmosRemoteMarkers are the substrings [hrmosIsRemote] checks the
// sg-tag-location chip against (case-insensitive). Every tenant observed
// yields false today — this documents a known gap rather than scanning
// free JD prose, which this repo's soft-filter policy rejects.
var hrmosRemoteMarkers = []string{"リモート", "フルリモート", "在宅", "テレワーク", "remote"}

// HrmosAdapter serves HRMOS 採用 (hrmos.co)-hosted companies. HRMOS has no
// server-side keyword search (?word=/?keyword=/?q= are byte-identical), so
// Search fetches a tenant's whole job dump and filters it via searchDump,
// the same shape as Lever and join.com.
type HrmosAdapter struct {
	client *hrmos.Client
}

// hrmosCareersURLRE matches hrmos.co tenant URLs and captures the slug: it
// matches /pages/{slug}, /pages/{slug}/jobs, and /pages/{slug}/jobs/{id}
// alike since only the prefix is anchored.
//
// Example (hostname + escaped path): hrmos.co/pages/moneyforward/jobs
var hrmosCareersURLRE = regexp.MustCompile(`(?i)^hrmos\.co/pages/(?P<slug>[^/]+)`)

func NewHrmosAdapter(baseURL string, hc *http.Client) *HrmosAdapter {
	return &HrmosAdapter{client: hrmos.NewClient(baseURL, hc)}
}

func (a *HrmosAdapter) Name() string { return "hrmos" }

func (a *HrmosAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(hrmos.Companies))
	for _, c := range hrmos.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Slug, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes any hrmos.co tenant URL, not just curated
// roster ones: unlike join.com, a HRMOS slug is itself the identifier
// [Adapter.Search] and [Adapter.Detail] need, so no companyId lookup gates
// it (the Workday shape, not the join.com one).
func (a *HrmosAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	return matchCareersSlug(hrmosCareersURLRE, u)
}

// CareersURL renders any tenant's jobs page, roster or not, for the same
// reason [HrmosAdapter.ParseCareersURL] accepts any tenant: the slug alone
// identifies the page, so there is nothing to look up.
func (a *HrmosAdapter) CareersURL(slug string) (string, bool) {
	if c, ok := hrmos.CompaniesBySlug[strings.ToLower(slug)]; ok {
		return c.CareersURL(), true
	}
	if slug == "" {
		return "", false
	}
	return hrmos.RosterCompany{Slug: slug}.CareersURL(), true
}

func (a *HrmosAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *HrmosAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *HrmosAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	d, err := a.client.JobDetail(ctx, slug, jobID)
	if err != nil {
		if errors.Is(err, hrmos.ErrNotFound) {
			return nil, errHrmosJobNotFound(slug, jobID)
		}
		return nil, fmt.Errorf("hrmos: fetch job %q for %q: %w", jobID, slug, err)
	}
	return &JobDetail{
		JobID:       d.ID,
		Title:       d.Title,
		Company:     cmp.Or(hrmos.CompaniesBySlug[strings.ToLower(slug)].Name, d.Company),
		Location:    hrmosDetailLocation(d.Locations),
		PostedAt:    hrmosPostedAt(d.DatePosted),
		URL:         d.URL,
		Description: d.Description,
	}, nil
}

func errHrmosJobNotFound(slug, jobID string) error {
	return fmt.Errorf("hrmos: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
}

// dump fetches a tenant's whole board and reshapes it for the filter
// engine. Unlike join.com, slug needs no roster lookup to resolve — any
// hrmos.co tenant slug works directly against the client.
func (a *HrmosAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	resp, err := a.client.AllJobs(ctx, slug)
	if err != nil {
		if errors.Is(err, hrmos.ErrNotFound) {
			return nil, fmt.Errorf("hrmos: company %q not found", slug)
		}
		return nil, fmt.Errorf("hrmos: list jobs for %q: %w", slug, err)
	}

	// chipGroups classifies a chip label into every facet group that
	// claims it (fact 13 — label->group is not a function: a label can be
	// in more than one group, or in none at all). Built from the dump's
	// own facet nav, so Filters and Search agree by construction (the
	// facet nav's own advertised values are NOT the source of truth here —
	// see distinctFilters).
	chipGroups := make(map[string][]string)
	for _, g := range resp.Facets {
		for _, opt := range g.Options {
			chipGroups[opt] = append(chipGroups[opt], g.Label)
		}
	}

	jobs := make([]dumpJob, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		fields := make(map[string][]string)
		for _, chip := range j.Chips {
			// The location chip is classified separately (locations/
			// isRemote below), never as a generic facet dimension.
			if chip == "" || chip == j.Location {
				continue
			}
			for _, group := range chipGroups[chip] {
				fields[group] = append(fields[group], chip)
			}
		}
		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    j.ID,
				Title:    j.Title,
				Location: j.Location,
				PostedAt: "", // fact 16: no date on the list page
				URL:      j.URL,
			},
			// sortKey stays the zero time: fact 16 says the list page
			// carries no posted date, so searchDump's newest-first
			// ordering falls back to rank, then JobID.
			orgUnit:     "", // HRMOS exposes no department field
			description: j.Description,
			locations:   j.Location,
			fields:      fields,
			isRemote:    hrmosIsRemote(j.Location),
		})
	}
	return jobs, nil
}

// hrmosDetailLocation renders a detail page's jobLocation list for display:
// each Place's region+locality+street, in list order, joined for readability.
func hrmosDetailLocation(locs []hrmos.Location) string {
	parts := make([]string, 0, len(locs))
	for _, l := range locs {
		s := strings.TrimSpace(l.Region + l.Locality + l.Street)
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "; ")
}

// hrmosPostedAt trims a JSON-LD datePosted timestamp
// ("2021-06-02T13:25:56.000Z") down to its date component.
func hrmosPostedAt(raw string) string {
	date, _, ok := strings.Cut(raw, "T")
	if !ok {
		return raw
	}
	return date
}

// hrmosIsRemote reports whether a job's sg-tag-location text matches one of
// [hrmosRemoteMarkers]. It never inspects JD prose (fact 16 / the package
// decision documented on [HrmosAdapter]).
func hrmosIsRemote(location string) bool {
	loc := strings.ToLower(location)
	for _, marker := range hrmosRemoteMarkers {
		if strings.Contains(loc, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}
