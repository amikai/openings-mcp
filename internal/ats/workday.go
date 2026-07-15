package ats

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/workday"
)

var _ Adapter = (*WorkdayAdapter)(nil)

// WorkdayAdapter serves Workday CXS tenants. Search runs server-side;
// location and filters name facet labels, which the adapter resolves to
// tenant-specific GUIDs via a probe request (appliedFacets wants GUIDs but
// get_filters reports labels — the stateless price is one extra upstream
// call whenever location or filters are set).
type WorkdayAdapter struct {
	hc *http.Client
	// baseURL and siteBaseURL derive CXS base URLs for roster tenants and
	// URL-resolved career sites respectively; tests point them at a mock.
	baseURL     func(workday.Company) string
	siteBaseURL func(workday.CareersSite) string
}

func NewWorkdayAdapter(hc *http.Client) *WorkdayAdapter {
	return &WorkdayAdapter{
		hc:          hc,
		baseURL:     workday.Company.BaseURL,
		siteBaseURL: workday.CareersSite.BaseURL,
	}
}

func (a *WorkdayAdapter) Name() string { return "workday" }

// Roster dedupes by tenant slug: fox and dowjones each hold two
// share-class rows in companies.yaml sharing one tenant, and the registry
// treats duplicate slugs as curation bugs.
func (a *WorkdayAdapter) Roster() []CompanyInfo {
	seen := make(map[string]bool, len(workday.Companies))
	infos := make([]CompanyInfo, 0, len(workday.Companies))
	for _, c := range workday.Companies {
		slug := strings.ToLower(c.Tenant)
		if seen[slug] {
			continue
		}
		seen[slug] = true
		infos = append(infos, CompanyInfo{Slug: slug, Name: c.Name})
	}
	return infos
}

// Search accepts two slug forms: a roster tenant slug, or the canonical
// careers URL that ParseCareersURL mints for tenants outside the roster.
// The URL form exists because addressing a Workday site takes three values
// (tenant, instance host, site name); the roster stores them for curated
// tenants, and for everyone else the URL is the only slug that can carry
// all three across stateless calls. Filters and Detail accept the same two
// forms.
func (a *WorkdayAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	_, base, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	client, err := workday.NewClient(base, workday.WithClient(a.hc))
	if err != nil {
		return nil, err
	}
	page := clampPage(p.Page)
	pageIndex := page - 1
	if pageIndex > math.MaxInt/pageSize {
		return nil, fmt.Errorf("workday: page %d is too large; retry with a smaller page", page)
	}
	offset := pageIndex * pageSize
	// Trim before deciding whether location filtering is requested: a
	// whitespace location would otherwise substring-match every facet label.
	location := strings.TrimSpace(p.Location)
	var applied workday.AppliedFacets
	if location != "" || len(p.Filters) > 0 {
		flat, err := a.probeFacets(ctx, client, slug)
		if err != nil {
			return nil, err
		}
		applied, err = resolveFacets(flat, location, p.Filters)
		if err != nil {
			return nil, err
		}
	}
	rsp, err := client.SearchJobs(ctx, &workday.JobsRequest{
		AppliedFacets: applied,
		Limit:         pageSize,
		Offset:        offset,
		SearchText:    p.Query,
	})
	if err != nil {
		return nil, fmt.Errorf("workday: search %q: %w", slug, err)
	}

	// Public posting URLs derive from the tenant's career-site origin;
	// derivation can fail only on malformed base URLs (e.g. a test mock),
	// in which case summaries simply omit URLs.
	publicURL, pubErr := workday.PublicSiteURL(base)
	jobs := make([]JobSummary, 0, len(rsp.JobPostings))
	for _, js := range rsp.JobPostings {
		path, ok := js.ExternalPath.Get()
		if !ok || path == "" {
			// Transient posting with no fetchable path; skip rather than
			// hand out a job_id that can't be detailed.
			continue
		}
		var url string
		if pubErr == nil {
			url = publicURL + path
		}
		jobs = append(jobs, JobSummary{
			// use externalPath as the job ID
			JobID:    path,
			Title:    js.Title.Value,
			Location: js.LocationsText.Value,
			PostedAt: js.PostedOn.Value,
			URL:      url,
		})
	}
	return &SearchResult{
		Jobs:       jobs,
		TotalCount: rsp.Total.Value,
		Page:       page,
		TotalPages: totalPages(rsp.Total.Value),
	}, nil
}

func (a *WorkdayAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	_, base, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	client, err := workday.NewClient(base, workday.WithClient(a.hc))
	if err != nil {
		return nil, err
	}
	flat, err := a.probeFacets(ctx, client, slug)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]map[string]struct{})
	for _, f := range flat {
		if seen[f.param] == nil {
			seen[f.param] = make(map[string]struct{})
		}
		seen[f.param][f.label] = struct{}{}
	}
	return toFilterSet(seen), nil
}

func (a *WorkdayAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	name, base, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	client, err := workday.NewClient(base, workday.WithClient(a.hc))
	if err != nil {
		return nil, err
	}
	loc, titleSlug, ok := workday.JobDetailKeyFromPath(jobID)
	if !ok {
		return nil, fmt.Errorf("workday: invalid job_id %q; pass a job_id exactly as returned by the job search", jobID)
	}
	rsp, err := client.GetJobDetail(ctx, workday.GetJobDetailParams{Location: loc, TitleSlug: titleSlug})
	if err != nil {
		return nil, fmt.Errorf("workday: fetch job %q for %q: %w", jobID, slug, err)
	}
	info := rsp.JobPostingInfo
	desc, err := html2text.FromString(info.JobDescription.Value, html2text.Options{})
	if err != nil {
		return nil, fmt.Errorf("workday: convert job description: %w", err)
	}
	location := info.Location.Value
	if len(info.AdditionalLocations) > 0 {
		location = strings.Join(append([]string{location}, info.AdditionalLocations...), "; ")
	}
	return &JobDetail{
		JobID:       jobID,
		Title:       info.Title.Value,
		Company:     name,
		Location:    location,
		PostedAt:    info.PostedOn.Value,
		URL:         info.ExternalUrl.Value,
		Description: desc,
	}, nil
}

// ParseCareersURL recognizes myworkdayjobs.com careers URLs. Roster
// tenants fold back to their roster slug so display names stay identical
// to name-based resolution; unknown tenants get the canonical URL as a
// self-describing slug (workday config is three values, which a bare
// tenant slug can't carry).
func (a *WorkdayAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	site, ok := workday.ParseCareersURL(u)
	if !ok {
		return "", false
	}
	// site.Tenant is already lowercase: the provider parse lowercases the
	// whole host before splitting.
	if _, ok := workday.CompaniesByTenant[site.Tenant]; ok {
		return site.Tenant, true
	}
	return site.CanonicalURL(), true
}

// resolveSlug maps a slug to its career site: roster key first, then the
// canonical-URL form ParseCareersURL hands out for non-roster tenants.
// name feeds JobDetail.Company; base is the CXS base URL.
func (a *WorkdayAdapter) resolveSlug(slug string) (name, base string, err error) {
	// 1. when slug is company
	if company, ok := workday.CompaniesByTenant[slug]; ok {
		return company.Name, a.baseURL(company), nil
	}
	// 2. when slug is careers URL
	if u, ok := parseCareersInput(slug); ok {
		if site, ok := workday.ParseCareersURL(u); ok {
			return site.Tenant, a.siteBaseURL(site), nil
		}
	}
	return "", "", fmt.Errorf("workday: unknown company %q; pass a roster slug or a myworkdayjobs.com careers URL", slug)
}

// flatFacet is one facet leaf attributed to its nearest ancestor group
// carrying a facetParameter (groups nest, e.g. locationMainGroup wraps
// locationHierarchy1 and locations).
type flatFacet struct {
	param string
	label string
	id    string
}

// probeFacets fetches the tenant's complete current facet tree with a
// minimal unfiltered search (searchText narrows the tree as much as a
// facet filter does, so the probe sends neither).
func (a *WorkdayAdapter) probeFacets(ctx context.Context, client *workday.Client, slug string) ([]flatFacet, error) {
	rsp, err := client.SearchJobs(ctx, &workday.JobsRequest{
		AppliedFacets: workday.AppliedFacets{},
		Limit:         1,
	})
	if err != nil {
		return nil, fmt.Errorf("workday: probe facets for %q: %w", slug, err)
	}
	nodes, ok := rsp.Facets.Get()
	if !ok || len(nodes) == 0 {
		return nil, fmt.Errorf("workday: company %q reports no filter dimensions; retry without location/filters", slug)
	}
	return flattenFacets(nodes), nil
}

func flattenFacets(nodes []workday.FacetNode) []flatFacet {
	var out []flatFacet
	var walk func(n workday.FacetNode, param string)
	walk = func(n workday.FacetNode, param string) {
		if v, ok := n.FacetParameter.Get(); ok {
			param = v
		}
		if len(n.Values) == 0 {
			descriptor, ok := n.Descriptor.Get()
			isLeafFacet := param != "" && n.ID.Set && ok
			if isLeafFacet {
				out = append(out, flatFacet{param: param, label: descriptor, id: n.ID.Value})
			}
			return
		}
		for _, c := range n.Values {
			walk(c, param)
		}
	}
	for _, n := range nodes {
		walk(n, "")
	}
	return out
}

// resolveFacets turns unified location/filter inputs into appliedFacets
// GUIDs, failing with teaching errors that name the valid alternatives.
func resolveFacets(flat []flatFacet, location string, filters map[string][]string) (workday.AppliedFacets, error) {
	applied := workday.AppliedFacets{}
	var locParam string
	if location != "" {
		param, ids, err := resolveLocationFacet(flat, location)
		if err != nil {
			return nil, err
		}
		applied[param] = ids
		locParam = param
	}
	for key, values := range filters {
		// Workday ORs values within one facet, so merging the fuzzy location
		// IDs with explicit filter IDs would silently broaden the search.
		if key == locParam {
			return nil, fmt.Errorf("location %q resolves to the %q facet, which is also set in filters; pass the criteria as explicit %q filter values instead of a location", location, key, key)
		}
		ids, err := resolveFacetValues(flat, key, values)
		if err != nil {
			return nil, err
		}
		applied[key] = ids
	}
	return applied, nil
}

// resolveLocationFacet fuzzy-matches the location text against every
// location-flavored facet leaf (params prefixed "location"), then applies
// the single param with the most hits — mixing params would AND them and
// over-constrain.
func resolveLocationFacet(flat []flatFacet, location string) (string, []string, error) {
	loc := strings.ToLower(strings.TrimSpace(location))
	hits := make(map[string][]string)
	for _, f := range flat {
		if !strings.HasPrefix(strings.ToLower(f.param), "location") {
			continue
		}
		if strings.Contains(strings.ToLower(f.label), loc) {
			hits[f.param] = append(hits[f.param], f.id)
		}
	}
	if len(hits) == 0 {
		return "", nil, fmt.Errorf("no location matching %q; list the company's filters to see available location values", location)
	}
	params := make([]string, 0, len(hits))
	for p := range hits {
		params = append(params, p)
	}
	slices.SortFunc(params, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(hits[b]), len(hits[a])), strings.Compare(a, b))
	})
	return params[0], hits[params[0]], nil
}

// resolveFacetValues maps display labels to GUIDs within one facet param,
// matching labels case-insensitively.
func resolveFacetValues(flat []flatFacet, key string, values []string) ([]string, error) {
	byLabel := make(map[string]string)
	var labels []string
	params := make(map[string]bool)
	for _, f := range flat {
		params[f.param] = true
		if f.param != key {
			continue
		}
		lower := strings.ToLower(f.label)
		if _, ok := byLabel[lower]; !ok {
			byLabel[lower] = f.id
			labels = append(labels, f.label)
		}
	}
	if len(byLabel) == 0 {
		return nil, errUnknownFilterKey(key, params)
	}
	ids := make([]string, 0, len(values))
	for _, v := range values {
		id, ok := byLabel[strings.ToLower(v)]
		if !ok {
			slices.Sort(labels)
			const maxListed = 20
			listed := labels
			var suffix string
			if len(listed) > maxListed {
				listed = listed[:maxListed]
				suffix = ", …"
			}
			return nil, fmt.Errorf("filter value %q not found for %q; available: %s%s", v, key, strings.Join(listed, ", "), suffix)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
