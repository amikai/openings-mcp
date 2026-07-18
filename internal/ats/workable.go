package ats

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/workable"
)

var _ Adapter = (*WorkableAdapter)(nil)

// workableCareersURLRE matches Workable-hosted careers URLs and captures the
// account subdomain (first path segment).
//
// Examples (hostname + escaped path):
//   - apply.workable.com/blueground
//   - apply.workable.com/blueground/j/B02DA69C8F
//   - apply.workable.com/some-unknown-co/
var workableCareersURLRE = regexp.MustCompile(
	`(?i)^apply\.workable\.com/(?P<slug>[^/]+)`,
)

const (
	// workableUpstreamPageSize is the API's fixed page size; there is no
	// limit field, so one unified page costs pageSize/workableUpstreamPageSize
	// cursor requests.
	workableUpstreamPageSize = 10
	// maxWorkableCandidates bounds the local-AND candidate walk to
	// maxWorkableCandidates/workableUpstreamPageSize upstream requests.
	maxWorkableCandidates = 200
)

// WorkableAdapter serves Workable-hosted companies via the public job board
// API behind apply.workable.com. Structured filters and locations run
// server-side. Text search uses query only to select a bounded candidate set,
// then applies the unified Query semantics locally, because Workable ORs
// query terms and matches location text too.
type WorkableAdapter struct {
	client *workable.Client
}

func NewWorkableAdapter(baseURL string, hc *http.Client) (*WorkableAdapter, error) {
	c, err := workable.NewClient(baseURL, workable.WithClient(hc))
	if err != nil {
		return nil, err
	}
	return &WorkableAdapter{client: c}, nil
}

func (a *WorkableAdapter) Name() string { return "workable" }

func (a *WorkableAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(workable.Companies))
	for _, c := range workable.Companies {
		infos = append(infos, CompanyInfo{Slug: strings.ToLower(c.Account), Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes apply.workable.com careers URLs; the first path
// segment is the account subdomain, which alone addresses a company, so
// non-roster companies need no special slug form. "api" is the API prefix on
// the same host, not an account.
func (a *WorkableAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	slug, ok := matchCareersSlug(workableCareersURLRE, u)
	if !ok || strings.EqualFold(slug, "api") {
		return "", false
	}
	return strings.ToLower(slug), true
}

// resolveWorkableCompany maps a slug to the roster display name. Non-roster
// slugs from ParseCareersURL pass through as both.
func resolveWorkableCompany(slug string) (account, name string) {
	if c, ok := workable.CompaniesByAccount[slug]; ok {
		return c.Account, c.Name
	}
	return slug, slug
}

func (a *WorkableAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	req := workable.SearchRequest{}
	query := strings.TrimSpace(p.Query)
	location := strings.TrimSpace(p.Location)
	if strings.EqualFold(location, "remote") {
		req.Remote = []string{"true"}
		location = ""
	}
	if len(p.Filters) > 0 || location != "" {
		facets, err := a.facets(ctx, slug)
		if err != nil {
			return nil, err
		}
		if err := applyWorkableFilters(facets, p.Filters, &req); err != nil {
			return nil, err
		}
		if location != "" {
			locs := matchWorkableFacetLocations(facets.Locations, location)
			if len(locs) == 0 {
				// The facets list every location with published jobs, so no
				// match means no jobs there — same empty page a structured
				// filter would produce.
				return &SearchResult{Page: clampPage(p.Page)}, nil
			}
			req.Location = locs
		}
	}
	if query == "" {
		return a.searchWorkablePage(ctx, slug, p.Page, req)
	}
	return a.searchWorkableCandidates(ctx, slug, query, p.Page, req)
}

// searchWorkablePage is the cheap path when no local text matching is needed:
// the upstream total and pagination remain exact. Pagination is cursor-only,
// so reaching unified page N walks the cursor from the start; the walk stops
// early when the board runs out of pages.
func (a *WorkableAdapter) searchWorkablePage(
	ctx context.Context,
	slug string,
	requestedPage int,
	req workable.SearchRequest,
) (*SearchResult, error) {
	const perUnified = pageSize / workableUpstreamPageSize
	page := clampPage(requestedPage)
	if page-1 > math.MaxInt/perUnified {
		return nil, fmt.Errorf("workable: page %d is too large; retry with a smaller page", page)
	}
	skip := (page - 1) * perUnified

	var jobs []JobSummary
	total := 0
	for i := range skip + perUnified {
		rsp, err := a.searchJobs(ctx, slug, req)
		if err != nil {
			return nil, err
		}
		total = rsp.Total
		if i >= skip {
			for _, j := range rsp.Results {
				jobs = append(jobs, workableSummary(slug, j))
			}
		}
		token, ok := rsp.NextPage.Get()
		if !ok {
			break
		}
		req.Token = workable.NewOptString(token)
	}
	return &SearchResult{
		Jobs:       jobs,
		TotalCount: total,
		Page:       page,
		TotalPages: totalPages(total),
	}, nil
}

// searchWorkableCandidates sends query upstream only to select a bounded
// candidate set (a superset of the AND matches, since every AND match also
// OR-matches), collects every cursor page, and applies the unified Query
// semantics locally with searchDump. Structured filters and locations stay
// on the request, so the candidate set is already narrowed server-side.
func (a *WorkableAdapter) searchWorkableCandidates(
	ctx context.Context,
	slug, query string,
	page int,
	req workable.SearchRequest,
) (*SearchResult, error) {
	req.Query = workable.NewOptString(query)
	var items []workable.JobSummary
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("workable: collect candidates for %q: %w", slug, err)
		}
		rsp, err := a.searchJobs(ctx, slug, req)
		if err != nil {
			return nil, err
		}
		if rsp.Total > maxWorkableCandidates {
			return nil, fmt.Errorf("workable: search is too broad (%d candidates); add a more specific query, location, or filter", rsp.Total)
		}
		items = append(items, rsp.Results...)
		token, ok := rsp.NextPage.Get()
		// The total also terminates the walk, so a cursor that stops
		// advancing cannot loop forever.
		if !ok || len(items) >= rsp.Total {
			break
		}
		req.Token = workable.NewOptString(token)
	}
	jobs := make([]dumpJob, 0, len(items))
	for _, j := range items {
		jobs = append(jobs, workableDumpJob(slug, j))
	}
	// Location went upstream as structured filters (or remote), so only
	// Query runs locally.
	return searchDump(jobs, SearchParams{Query: query, Page: page})
}

// searchJobs performs one search request, mapping the text/plain 404 an
// unknown account produces to a teaching error.
func (a *WorkableAdapter) searchJobs(ctx context.Context, slug string, req workable.SearchRequest) (*workable.SearchResponse, error) {
	res, err := a.client.SearchJobs(ctx, &req, workable.SearchJobsParams{Account: slug})
	if err != nil {
		return nil, fmt.Errorf("workable: search %q: %w", slug, err)
	}
	rsp, ok := res.(*workable.SearchResponse)
	if !ok {
		return nil, fmt.Errorf("workable: company %q not found (no such account on apply.workable.com)", slug)
	}
	return rsp, nil
}

func (a *WorkableAdapter) facets(ctx context.Context, slug string) (*workable.FiltersResponse, error) {
	res, err := a.client.ListJobFilters(ctx, workable.ListJobFiltersParams{Account: slug})
	if err != nil {
		return nil, fmt.Errorf("workable: list facets for %q: %w", slug, err)
	}
	facets, ok := res.(*workable.FiltersResponse)
	if !ok {
		return nil, fmt.Errorf("workable: company %q not found (no such account on apply.workable.com)", slug)
	}
	return facets, nil
}

// applyWorkableFilters maps unified filters onto the search body, failing
// with teaching errors that name the valid alternatives. Everything
// validates against the account's facets, whose values Filters() reports.
func applyWorkableFilters(facets *workable.FiltersResponse, filters FilterSet, req *workable.SearchRequest) error {
	for key, values := range filters {
		switch key {
		case "department":
			ids, err := resolveWorkableDepartments(facets.Departments, values)
			if err != nil {
				return err
			}
			req.Department = ids
		case "workplace":
			for _, v := range values {
				normalized := strings.ToLower(strings.TrimSpace(v))
				if !slices.Contains(facets.Workplaces, normalized) {
					return fmt.Errorf("filter value %q not found for %q; available: %s", v, key, strings.Join(facets.Workplaces, ", "))
				}
				req.Workplace = append(req.Workplace, workable.SearchRequestWorkplaceItem(normalized))
			}
		case "worktype":
			for _, v := range values {
				normalized := strings.ToLower(strings.TrimSpace(v))
				if !slices.Contains(facets.Worktypes, normalized) {
					return fmt.Errorf("filter value %q not found for %q; available: %s", v, key, strings.Join(facets.Worktypes, ", "))
				}
				req.Worktype = append(req.Worktype, normalized)
			}
		default:
			return errUnknownFilterKey(key, map[string]bool{"department": true, "workplace": true, "worktype": true})
		}
	}
	return nil
}

// resolveWorkableDepartments maps department display names to the facet's
// filter id set (the department plus its descendants), matching names
// case-insensitively.
func resolveWorkableDepartments(departments []workable.FacetDepartment, values []string) ([]int, error) {
	var ids []int
	seen := map[int]bool{}
	for _, v := range values {
		i := slices.IndexFunc(departments, func(d workable.FacetDepartment) bool {
			return strings.EqualFold(strings.TrimSpace(v), d.Name)
		})
		if i < 0 {
			names := make([]string, len(departments))
			for j, d := range departments {
				names[j] = d.Name
			}
			slices.Sort(names)
			return nil, fmt.Errorf("filter value %q not found for %q; available: %s", v, "department", strings.Join(names, ", "))
		}
		// The facet's filter set covers the department and its descendants;
		// the bare id alone misses jobs filed under child departments.
		resolved := departments[i].Filter
		if len(resolved) == 0 {
			resolved = []int{departments[i].ID}
		}
		for _, id := range resolved {
			if seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// matchWorkableFacetLocations selects every facet location entry matching
// the fuzzy location text — each word must appear in the entry's combined
// text — and converts the matches to the search body's structured location
// filters (multiple entries OR together).
func matchWorkableFacetLocations(entries []workable.FacetLocation, location string) []workable.LocationFilter {
	words := strings.Fields(strings.ToLower(strings.ReplaceAll(location, ",", " ")))
	var out []workable.LocationFilter
	for _, e := range entries {
		blob := strings.ToLower(strings.Join([]string{
			e.Display.Or(""), e.Country.Or(""), e.CountryCode.Or(""), e.Region.Or(""), e.City.Or(""),
		}, " "))
		if !containsAllWords(blob, words) {
			continue
		}
		// Facet fields are OptNilString (API may send null); the search body
		// uses OptString, so only forward present non-null values.
		lf := workable.LocationFilter{}
		if v, ok := e.Country.Get(); ok {
			lf.Country = workable.NewOptString(v)
		}
		if v, ok := e.Region.Get(); ok {
			lf.Region = workable.NewOptString(v)
		}
		if v, ok := e.City.Get(); ok {
			lf.City = workable.NewOptString(v)
		}
		if !lf.Country.Set && !lf.Region.Set && !lf.City.Set {
			// A display-only entry has nothing the search body can match on.
			continue
		}
		out = append(out, lf)
	}
	return out
}

func workableSummary(account string, j workable.JobSummary) JobSummary {
	return JobSummary{
		JobID:    j.Shortcode,
		Title:    j.Title,
		Location: workableLocationText(j.Location),
		PostedAt: workablePostedAt(j.Published),
		URL:      workableJobURL(account, j.Shortcode),
	}
}

func workableDumpJob(account string, j workable.JobSummary) dumpJob {
	summary := workableSummary(account, j)
	published, _ := time.Parse(time.RFC3339, j.Published.Value)
	locations := []string{summary.Location}
	for _, l := range j.Locations {
		locations = append(locations, strings.Join([]string{l.City.Or(""), l.Region.Or(""), l.Country.Or("")}, " "))
	}
	return dumpJob{
		summary:   summary,
		sortKey:   published,
		orgUnit:   strings.Join(j.Department, " "),
		locations: strings.Join(locations, "; "),
		isRemote:  j.Remote.Or(false) || j.Workplace.Value == workable.JobSummaryWorkplaceRemote,
	}
}

// workableLocationText renders one location object, preferring the API's
// own display string; facet-less accounts (and hidden locations) still
// carry the structured fields.
func workableLocationText(loc workable.OptLocation) string {
	v, ok := loc.Get()
	if !ok {
		return ""
	}
	if d, ok := v.Display.Get(); ok && d != "" {
		return d
	}
	parts := []string{}
	for _, p := range []string{v.City.Or(""), v.Region.Or(""), v.Country.Or("")} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

// workablePostedAt guards a present-but-missing published timestamp, and
// falls back to the raw text if the timestamp shape ever drifts.
func workablePostedAt(published workable.OptString) string {
	v, ok := published.Get()
	if !ok {
		return ""
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return v
	}
	return isoDate(t)
}

// workableJobURL derives the public posting page; no API response field
// carries it.
func workableJobURL(account, shortcode string) string {
	return "https://apply.workable.com/" + url.PathEscape(account) + "/j/" + url.PathEscape(shortcode) + "/"
}

func (a *WorkableAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	facets, err := a.facets(ctx, slug)
	if err != nil {
		return nil, err
	}
	fs := FilterSet{}
	if len(facets.Departments) > 0 {
		names := make([]string, len(facets.Departments))
		for i, d := range facets.Departments {
			names[i] = d.Name
		}
		slices.Sort(names)
		fs["department"] = names
	}
	if len(facets.Workplaces) > 0 {
		fs["workplace"] = slices.Sorted(slices.Values(facets.Workplaces))
	}
	if len(facets.Worktypes) > 0 {
		fs["worktype"] = slices.Sorted(slices.Values(facets.Worktypes))
	}
	return fs, nil
}

func (a *WorkableAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	res, err := a.client.GetJob(ctx, workable.GetJobParams{Account: slug, Shortcode: jobID})
	if err != nil {
		return nil, fmt.Errorf("workable: fetch job %q for %q: %w", jobID, slug, err)
	}
	d, ok := res.(*workable.JobDetail)
	if !ok {
		// The only other GetJobRes variant is the text/plain 404, for an
		// unknown account or shortcode.
		return nil, fmt.Errorf("workable: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
	}
	account, name := resolveWorkableCompany(slug)
	return &JobDetail{
		JobID:       cmp.Or(d.Shortcode, jobID),
		Title:       d.Title,
		Company:     name,
		Location:    workableLocationText(d.Location),
		PostedAt:    workablePostedAt(d.Published),
		URL:         workableJobURL(account, cmp.Or(d.Shortcode, jobID)),
		Description: workableDescription(d),
	}, nil
}

// workableDescription joins the posting's non-empty HTML body fields as
// titled plain-text blocks. Workable splits the body across three fields;
// many postings keep everything in description and leave the rest empty.
func workableDescription(d *workable.JobDetail) string {
	sections := []struct {
		title string
		html  string
	}{
		{"Description", d.Description.Value},
		{"Requirements", d.Requirements.Value},
		{"Benefits", d.Benefits.Value},
	}
	var parts []string
	for _, s := range sections {
		if s.html == "" {
			continue
		}
		text, err := html2text.FromString(s.html, html2text.Options{})
		if err != nil {
			// Keep the section as raw HTML rather than dropping it.
			text = s.html
		}
		parts = append(parts, s.title+":\n"+text)
	}
	return strings.Join(parts, "\n\n")
}
