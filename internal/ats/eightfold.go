package ats

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/ogen-go/ogen/validate"

	"github.com/amikai/openings-mcp/internal/provider/eightfold"
)

var _ Adapter = (*EightfoldAdapter)(nil)

// upstreamPageSize is Eightfold's fixed search page size: every observed
// tenant returns exactly 10 positions per request regardless of any
// num/limit/size/count/pageSize query parameter tried, so Search fetches
// two consecutive upstream pages to fill one unified pageSize (20) page.
const upstreamPageSize = 10

// EightfoldAdapter serves Eightfold-hosted career sites
// (<tenant>.eightfold.ai). Search runs server-side: query and location are
// plain fuzzy-text parameters, but named facet filters (businessarea,
// employmenttype, city, ...) are tenant-specific, their query value often
// differs from the display label only in case, and the set of valid
// filters is unknowable ahead of time — so a filtered Search first probes
// an unfiltered search to resolve requested labels to values (mirrors
// Workday's GUID-facet probe, minus the GUIDs).
//
// Some tenants have the pcsx API this all otherwise runs on disabled
// (403 "PCSX is not enabled for this user.") and serve postings exclusively
// through the newer, unfiltered-only v2 API instead; Search and Detail fall
// back to it on that specific error (see openapi.yaml's v2 tag). Filters
// and filtered Search still require pcsx and simply fail for v2-only
// tenants — narrower than not supporting them at all, since plain search
// still works.
//
// Roster membership is required, unlike Workday/Greenhouse/Lever: every
// PCSX request needs the tenant's registered `domain` value alongside its
// subdomain, and that value can't be derived from a careers URL alone or
// safely guessed — a wrong domain gets the site's HTML shell back, not a
// clean error (see companies.yaml and provider/eightfold/openapi.yaml).
type EightfoldAdapter struct {
	hc *http.Client
	// baseURL derives a tenant's API origin; tests point it at a mock.
	baseURL func(tenant string) string
}

// NewEightfoldAdapter takes an *http.Client already wrapped in
// eightfold.BrowserTransport — Eightfold's edge 403s Go's default
// User-Agent instead of returning JSON.
func NewEightfoldAdapter(hc *http.Client) *EightfoldAdapter {
	return &EightfoldAdapter{hc: hc, baseURL: eightfoldBaseURL}
}

func (a *EightfoldAdapter) Name() string { return "eightfold" }

func (a *EightfoldAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(eightfold.Companies))
	for _, c := range eightfold.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Tenant, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes <tenant>.eightfold.ai career pages, but only
// resolves tenants already on the roster — see the domain-derivation note
// on EightfoldAdapter.
func (a *EightfoldAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".eightfold.ai") {
		return "", false
	}
	tenant := strings.TrimSuffix(host, ".eightfold.ai")
	if _, ok := eightfold.CompaniesByTenant[tenant]; !ok {
		return "", false
	}
	return tenant, true
}

func (a *EightfoldAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	c, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}

	page := clampPage(p.Page)
	start := (page - 1) * pageSize

	params := eightfold.SearchParams{Domain: c.Domain, Start: eightfold.NewOptInt(start)}
	if p.Query != "" {
		params.Query = eightfold.NewOptString(p.Query)
	}
	if loc := strings.TrimSpace(p.Location); loc != "" {
		params.Location = eightfold.NewOptString(loc)
	}

	var filterValues map[string][]string
	if len(p.Filters) > 0 {
		filterValues, err = a.resolveFilters(ctx, c, p.Filters)
		if err != nil {
			return nil, err
		}
	}

	jobs, total, err := a.searchPage(ctx, c, params, filterValues)
	if err != nil {
		return nil, err
	}

	// Fill the unified 20-job page with a second upstream page, only when
	// the first page was full and more results actually remain.
	if len(jobs) == upstreamPageSize && total > start+upstreamPageSize {
		params.Start = eightfold.NewOptInt(start + upstreamPageSize)
		more, _, err := a.searchPage(ctx, c, params, filterValues)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, more...)
	}

	return &SearchResult{Jobs: jobs, TotalCount: total, Page: page, TotalPages: totalPages(total)}, nil
}

// searchPage runs one upstream page through pcsx, falling back to the v2
// API on pcsx's "not enabled for this user" 403 — but only for unfiltered
// requests, since v2 has no facet-filter equivalent here.
func (a *EightfoldAdapter) searchPage(ctx context.Context, c eightfold.RosterCompany, params eightfold.SearchParams, filterValues map[string][]string) ([]JobSummary, int, error) {
	res, err := a.fetchSearch(ctx, c, params, filterValues)
	if err != nil {
		if len(filterValues) > 0 || !isPCSXDisabled(err) {
			return nil, 0, err
		}
		v2, err := a.fetchSearchV2(ctx, c, params)
		if err != nil {
			return nil, 0, err
		}
		return v2JobSummaries(v2.Positions), v2.Count, nil
	}
	return eightfoldJobSummaries(res.Data.Positions, a.baseURL(c.Tenant)), res.Data.Count, nil
}

// isPCSXDisabled reports whether err is the 403 pcsx returns for tenants
// that only serve postings through the v2 API.
func isPCSXDisabled(err error) bool {
	var sc *validate.UnexpectedStatusCodeError
	return errors.As(err, &sc) && sc.StatusCode == http.StatusForbidden
}

func (a *EightfoldAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	c, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	res, err := a.fetchSearch(ctx, c, eightfold.SearchParams{Domain: c.Domain}, nil)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]map[string]struct{})
	for _, sf := range eightfold.MergedFacets(res.Data.FilterDef) {
		if sf.Options == nil {
			// Non-list facets (e.g. Morgan Stanley's "include_remote"
			// toggle) carry a boolean, not a value to pick from.
			continue
		}
		labels := seen[sf.FilterName]
		if labels == nil {
			labels = make(map[string]struct{}, len(sf.Options))
			seen[sf.FilterName] = labels
		}
		for _, o := range sf.Options {
			labels[o.Label] = struct{}{}
		}
	}
	return toFilterSet(seen), nil
}

func (a *EightfoldAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	c, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(jobID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("eightfold: invalid job_id %q; pass a job_id exactly as returned by the job search", jobID)
	}

	client, err := eightfold.NewClient(a.baseURL(c.Tenant), eightfold.WithClient(a.hc))
	if err != nil {
		return nil, err
	}
	res, err := client.PositionDetails(ctx, eightfold.PositionDetailsParams{PositionID: id, Domain: c.Domain})
	if err != nil {
		if isPCSXDisabled(err) {
			return a.detailV2(ctx, detailV2Args{
				Client:  client,
				Company: c,
				Slug:    slug,
				JobID:   jobID,
				ID:      id,
			})
		}
		return nil, fmt.Errorf("eightfold: fetch job %q for %q: %w", jobID, slug, err)
	}

	switch d := res.(type) {
	case *eightfold.PositionDetailsResponse:
		desc, err := html2text.FromString(d.Data.JobDescription, html2text.Options{})
		if err != nil {
			return nil, fmt.Errorf("eightfold: convert job description: %w", err)
		}
		return &JobDetail{
			JobID:       jobID,
			Title:       d.Data.Name,
			Company:     c.Name,
			Location:    strings.Join(d.Data.Locations, "; "),
			PostedAt:    isoDate(time.Unix(int64(d.Data.PostedTs), 0)),
			URL:         d.Data.PublicUrl,
			Description: desc,
		}, nil
	case *eightfold.PositionNotFoundResponse:
		return nil, fmt.Errorf("eightfold: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
	default:
		return nil, fmt.Errorf("eightfold: unexpected response type %T", res)
	}
}

// detailV2Args groups the inputs for a pcsx-disabled v2 position detail fetch.
type detailV2Args struct {
	Client  *eightfold.Client
	Company eightfold.RosterCompany
	Slug    string
	JobID   string
	ID      int64
}

// detailV2 fetches a job's detail through the v2 API, for tenants where
// pcsx's position_details 403'd the same way pcsx's search did.
func (a *EightfoldAdapter) detailV2(ctx context.Context, args detailV2Args) (*JobDetail, error) {
	res, err := args.Client.PositionDetailsV2(ctx, eightfold.PositionDetailsV2Params{
		ID:     args.ID,
		Domain: args.Company.Domain,
	})
	if err != nil {
		return nil, fmt.Errorf("eightfold: fetch job %q for %q (v2): %w", args.JobID, args.Slug, err)
	}
	switch d := res.(type) {
	case *eightfold.V2PositionDetail:
		desc, err := html2text.FromString(d.JobDescription, html2text.Options{})
		if err != nil {
			return nil, fmt.Errorf("eightfold: convert job description: %w", err)
		}
		return &JobDetail{
			JobID:       args.JobID,
			Title:       d.Name,
			Company:     args.Company.Name,
			Location:    strings.Join(d.Locations, "; "),
			PostedAt:    isoDate(time.Unix(d.TCreate, 0)),
			URL:         d.CanonicalPositionUrl,
			Description: desc,
		}, nil
	case *eightfold.V2ErrorResponse:
		return nil, fmt.Errorf("eightfold: job %q not found for company %q; pass a job_id exactly as returned by the job search", args.JobID, args.Slug)
	default:
		return nil, fmt.Errorf("eightfold: unexpected response type %T", res)
	}
}

func (a *EightfoldAdapter) resolveSlug(slug string) (eightfold.RosterCompany, error) {
	c, ok := eightfold.CompaniesByTenant[strings.ToLower(slug)]
	if !ok {
		return eightfold.RosterCompany{}, fmt.Errorf("eightfold: unknown company %q; pass a roster tenant slug", slug)
	}
	return c, nil
}

// fetchSearch runs one upstream page: the generated typed client when no
// facet filters are resolved, eightfold.SearchFiltered (hand-built
// filter_<name> query params) otherwise — see SearchFiltered's doc for why
// facet filters can't go through the generated client.
func (a *EightfoldAdapter) fetchSearch(ctx context.Context, c eightfold.RosterCompany, params eightfold.SearchParams, filterValues map[string][]string) (*eightfold.SearchResponse, error) {
	base := a.baseURL(c.Tenant)
	if len(filterValues) > 0 {
		res, err := eightfold.SearchFiltered(ctx, eightfold.FilteredSearch{
			HTTPClient: a.hc,
			BaseURL:    base,
			Params:     params,
			Filters:    filterValues,
		})
		if err != nil {
			return nil, fmt.Errorf("eightfold: search %q: %w", c.Tenant, err)
		}
		return res, nil
	}
	client, err := eightfold.NewClient(base, eightfold.WithClient(a.hc))
	if err != nil {
		return nil, err
	}
	res, err := client.Search(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("eightfold: search %q: %w", c.Tenant, err)
	}
	return res, nil
}

// fetchSearchV2 runs one upstream page through the v2 API, for tenants
// where pcsx's search 403'd with "not enabled for this user".
func (a *EightfoldAdapter) fetchSearchV2(ctx context.Context, c eightfold.RosterCompany, params eightfold.SearchParams) (*eightfold.V2SearchResponse, error) {
	client, err := eightfold.NewClient(a.baseURL(c.Tenant), eightfold.WithClient(a.hc))
	if err != nil {
		return nil, err
	}
	res, err := client.SearchV2(ctx, eightfold.SearchV2Params{
		Domain:   c.Domain,
		Query:    params.Query,
		Location: params.Location,
		Start:    params.Start,
	})
	if err != nil {
		return nil, fmt.Errorf("eightfold: search %q (v2): %w", c.Tenant, err)
	}
	return res, nil
}

// resolveFilters turns unified filter labels (as reported by Filters())
// into the API's filter_<facetName>=<value> values, probing one unfiltered
// search to learn the tenant's current facet definitions. The probe is
// deliberately unscoped by query/location (mirrors Workday's
// probeFacets) — narrower facets aren't needed just to resolve labels.
func (a *EightfoldAdapter) resolveFilters(ctx context.Context, c eightfold.RosterCompany, filters map[string][]string) (map[string][]string, error) {
	probe, err := a.fetchSearch(ctx, c, eightfold.SearchParams{Domain: c.Domain}, nil)
	if err != nil {
		return nil, err
	}

	facets := eightfold.MergedFacets(probe.Data.FilterDef)
	byName := make(map[string]eightfold.SmartFilter, len(facets))
	valid := make(map[string]bool, len(facets))
	for _, sf := range facets {
		if sf.Options == nil {
			continue
		}
		key := strings.ToLower(sf.FilterName)
		if existing, ok := byName[key]; ok {
			// Same facet name in both smartFilters and allFilters
			// (unobserved but not ruled out): merge rather than let
			// the later list silently shadow the earlier one's options.
			existing.Options = append(existing.Options, sf.Options...)
			byName[key] = existing
			continue
		}
		byName[key] = sf
		valid[sf.FilterName] = true
	}

	resolved := make(map[string][]string, len(filters))
	for key, values := range filters {
		sf, ok := byName[strings.ToLower(key)]
		if !ok {
			return nil, errUnknownFilterKey(key, valid)
		}
		ids := make([]string, 0, len(values))
		for _, v := range values {
			id, ok := resolveSmartFilterValue(sf, v)
			if !ok {
				labels := make([]string, len(sf.Options))
				for i, o := range sf.Options {
					labels[i] = o.Label
				}
				return nil, fmt.Errorf("filter value %q not found for %q; available: %s", v, key, strings.Join(labels, ", "))
			}
			ids = append(ids, id)
		}
		resolved[sf.FilterName] = ids
	}
	return resolved, nil
}

func resolveSmartFilterValue(sf eightfold.SmartFilter, label string) (string, bool) {
	for _, o := range sf.Options {
		if strings.EqualFold(o.Label, label) {
			return o.Value, true
		}
	}
	return "", false
}

func eightfoldJobSummaries(positions []eightfold.Position, base string) []JobSummary {
	jobs := make([]JobSummary, 0, len(positions))
	for _, p := range positions {
		jobs = append(jobs, JobSummary{
			JobID:    strconv.FormatInt(p.ID, 10),
			Title:    p.Name,
			Location: strings.Join(p.Locations, "; "),
			PostedAt: isoDate(time.Unix(int64(p.PostedTs), 0)),
			URL:      base + p.PositionUrl,
		})
	}
	return jobs
}

// v2JobSummaries mirrors eightfoldJobSummaries for the v2 API; unlike
// pcsx's site-relative positionUrl, v2's canonicalPositionUrl is already
// absolute.
func v2JobSummaries(positions []eightfold.V2Position) []JobSummary {
	jobs := make([]JobSummary, 0, len(positions))
	for _, p := range positions {
		jobs = append(jobs, JobSummary{
			JobID:    strconv.FormatInt(p.ID, 10),
			Title:    p.Name,
			Location: strings.Join(p.Locations, "; "),
			PostedAt: isoDate(time.Unix(p.TCreate, 0)),
			URL:      p.CanonicalPositionUrl,
		})
	}
	return jobs
}

func eightfoldBaseURL(tenant string) string {
	return fmt.Sprintf("https://%s.eightfold.ai", tenant)
}
