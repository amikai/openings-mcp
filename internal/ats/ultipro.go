package ats

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/ultipro"
)

var _ Adapter = (*UltiProAdapter)(nil)

// ultiproLocationTypeCodes maps the location_type filter's display values
// to LoadSearchResults' fieldName-37 codes (verified live — see
// internal/provider/ultipro/openapi.yaml).
var ultiproLocationTypeCodes = map[string]string{
	"hybrid": "0",
	"onsite": "1",
	"remote": "2",
}

var ultiproLocationTypeLabels = []string{"Hybrid", "Onsite", "Remote"}

// UltiProAdapter serves UltiPro (UKG Pro Recruiting) career boards. Search
// and its filter catalogs are server-side JSON; job detail is HTML with the
// posting embedded as a JSON object literal (see
// internal/provider/ultipro/openapi.yaml). Roster slugs are lowercase
// company codes (e.g. "tec1006teser"); ParseCareersURL mints a canonical
// board URL as the slug for non-roster boards, since a company code alone
// cannot carry the host and board id a non-roster board also needs.
type UltiProAdapter struct {
	hc *http.Client
	// baseURL derives the CXS-equivalent base URL for a resolved board;
	// tests point it at a mock.
	baseURL func(ultipro.CareersSite) string
}

func NewUltiProAdapter(hc *http.Client) *UltiProAdapter {
	return &UltiProAdapter{hc: hc, baseURL: ultipro.CareersSite.BaseURL}
}

func (a *UltiProAdapter) Name() string { return "ultipro" }

func (a *UltiProAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(ultipro.Companies))
	for _, c := range ultipro.Companies {
		infos = append(infos, CompanyInfo{Slug: strings.ToLower(c.CompanyCode), Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes UltiPro career-board URLs. A roster company
// folds back to its roster slug only when host, company code, AND board id
// all match the curated entry — UltiPro addresses a board with all three,
// so a same-coded tenant's *other* board must not collapse onto the
// curated one. Anything else (including a same-code URL with a different
// host or board id) gets the canonical URL as a self-describing slug.
func (a *UltiProAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	site, ok := ultipro.ParseCareersURL(u)
	if !ok {
		return "", false
	}
	if c, ok := ultipro.CompaniesByCode[strings.ToLower(site.CompanyCode)]; ok &&
		c.Host == site.Host && c.BoardID == site.BoardID {
		return strings.ToLower(site.CompanyCode), true
	}
	return site.CanonicalURL(), true
}

// CareersURL renders the roster company's public job board page.
func (a *UltiProAdapter) CareersURL(slug string) (string, bool) {
	c, ok := ultipro.CompaniesByCode[strings.ToLower(slug)]
	if !ok {
		return "", false
	}
	return c.CareersURL(), true
}

// resolveSlug maps a slug to its board: roster company code first, then
// the canonical-URL form ParseCareersURL hands out for non-roster boards.
// name feeds JobDetail.Company; site addresses the board for [ultipro.NewClient].
func (a *UltiProAdapter) resolveSlug(slug string) (name string, site ultipro.CareersSite, err error) {
	if c, ok := ultipro.CompaniesByCode[slug]; ok {
		return c.Name, ultipro.CareersSite{Host: c.Host, CompanyCode: c.CompanyCode, BoardID: c.BoardID}, nil
	}
	if u, ok := parseCareersInput(slug); ok {
		if s, ok := ultipro.ParseCareersURL(u); ok {
			return s.CompanyCode, s, nil
		}
	}
	return "", ultipro.CareersSite{}, fmt.Errorf("ultipro: unknown company %q; pass a roster slug or a recruiting.ultipro.com JobBoard URL", slug)
}

func (a *UltiProAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	_, site, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	client := ultipro.NewClient(a.baseURL(site), a.hc)

	page := clampPage(p.Page)
	pageIndex := page - 1
	if pageIndex > math.MaxInt/pageSize {
		return nil, fmt.Errorf("ultipro: page %d is too large; retry with a smaller page", page)
	}

	// Validate and resolve the caller's filters before applying any
	// location=remote special-casing below, so an invalid location_type
	// value or an unknown filter key always surfaces its teaching error —
	// short-circuiting on raw, unvalidated location_type strings let bad
	// input silently fall through as "excludes remote" instead.
	filters, err := a.buildFilters(ctx, client, p.Filters)
	if err != nil {
		return nil, err
	}

	// "remote" is handled entirely through the (now-validated) location_type
	// filter (field 37), not the physical location catalog (field 4) —
	// verified live, the two return different job sets (a literal "Remote"
	// physical location vs. the JobLocationType=Remote flag). An explicit
	// location_type filter is a separate, ANDed criterion, not something
	// "remote" ORs into: a resolved location_type filter that excludes
	// remote makes the combination impossible (short-circuit rather than
	// round-trip a query guaranteed to be empty), and one that includes
	// remote alongside other values (e.g. Remote+Hybrid) must narrow to
	// remote only — verified live, leaving Hybrid in place turned 2 Remote
	// jobs into 10 Remote-or-Hybrid jobs.
	location := strings.TrimSpace(p.Location)
	if strings.EqualFold(location, "remote") {
		var possible bool
		filters, possible = ultiproIntersectRemote(filters)
		if !possible {
			return &SearchResult{Page: page}, nil
		}
		location = ""
	}
	if location != "" {
		ids, err := resolveUltiProLocationValues(ctx, client, location)
		if err != nil {
			return nil, err
		}
		filters = append(filters, ultipro.SearchFilter{FieldName: 4, Values: ids})
	}

	res, err := client.Search(ctx, ultipro.SearchRequest{
		Query:   p.Query,
		Top:     pageSize,
		Skip:    pageIndex * pageSize,
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("ultipro: search %q: %w", slug, err)
	}

	return &SearchResult{
		Jobs:       ultiproSummaries(res.Opportunities, site),
		TotalCount: res.TotalCount,
		Page:       page,
		TotalPages: totalPages(res.TotalCount),
	}, nil
}

// ultiproIntersectRemote narrows an already-validated filter list to
// location_type=Remote only, operating on resolved fieldName-37 codes
// (not raw display labels — callers must validate via [UltiProAdapter.buildFilters]
// first). possible is false when a resolved location_type filter excludes
// remote entirely (a remote-location search combined with it can only ever
// be empty); otherwise out replaces any existing fieldName-37 filter with
// exactly {Values: ["2"]}, or appends one if none was present.
func ultiproIntersectRemote(filters []ultipro.SearchFilter) (out []ultipro.SearchFilter, possible bool) {
	out = make([]ultipro.SearchFilter, 0, len(filters)+1)
	found := false
	for _, f := range filters {
		if f.FieldName != 37 {
			out = append(out, f)
			continue
		}
		found = true
		if !slices.Contains(f.Values, "2") {
			return nil, false
		}
		out = append(out, ultipro.SearchFilter{FieldName: 37, Values: []string{"2"}})
	}
	if !found {
		out = append(out, ultipro.SearchFilter{FieldName: 37, Values: []string{"2"}})
	}
	return out, true
}

// buildFilters maps the unified department/location_type filter keys onto
// LoadSearchResults' fieldName codes. Every value for one key resolves into
// a single SearchFilter's Values slice — UltiPro ANDs separate filter
// objects for the same fieldName (verified live: two field-5 objects for
// Finance and IT independently returned 0 results; both ids in one Values
// array returned the union, 4), so splitting per value would silently turn
// the unified "OR within a key" contract into an impossible AND. FieldName
// 6 (schedule) is deliberately unsupported — see
// internal/provider/ultipro/openapi.yaml.
func (a *UltiProAdapter) buildFilters(ctx context.Context, client *ultipro.Client, filters FilterSet) ([]ultipro.SearchFilter, error) {
	var out []ultipro.SearchFilter
	for key, values := range filters {
		switch key {
		case "department":
			ids := make([]string, 0, len(values))
			for _, v := range values {
				id, err := resolveUltiProCatalogValue(ctx, client.Categories, v, "department")
				if err != nil {
					return nil, err
				}
				ids = append(ids, id)
			}
			out = append(out, ultipro.SearchFilter{FieldName: 5, Values: ids})
		case "location_type":
			codes := make([]string, 0, len(values))
			for _, v := range values {
				code, ok := ultiproLocationTypeCodes[strings.ToLower(strings.TrimSpace(v))]
				if !ok {
					return nil, fmt.Errorf("filter value %q not found for %q; available: %s", v, key, strings.Join(ultiproLocationTypeLabels, ", "))
				}
				codes = append(codes, code)
			}
			out = append(out, ultipro.SearchFilter{FieldName: 37, Values: codes})
		default:
			return nil, errUnknownFilterKey(key, map[string]bool{"department": true, "location_type": true})
		}
	}
	return out, nil
}

// resolveUltiProCatalogValue accepts either a raw catalog id (passed
// through unchanged, so a value round-tripped from [UltiProAdapter.Filters]
// always works) or a display label (resolved via one catalog call, exact
// case-insensitive match).
func resolveUltiProCatalogValue(ctx context.Context, fetch func(context.Context) ([]ultipro.FilterCatalog, error), input, key string) (string, error) {
	catalog, err := fetch(ctx)
	if err != nil {
		return "", fmt.Errorf("ultipro: fetch %s catalog: %w", key, err)
	}
	for _, c := range catalog {
		if c.ID == input {
			return c.ID, nil
		}
	}
	labels := make([]string, 0, len(catalog))
	for _, c := range catalog {
		if strings.EqualFold(c.Label, input) {
			return c.ID, nil
		}
		labels = append(labels, c.Label)
	}
	return "", fmt.Errorf("filter value %q not found for %q; available: %s", input, key, joinTruncated(labels))
}

// resolveUltiProLocationValues fuzzy-matches free text against every
// catalog location's display label (substring, case-insensitive) and
// returns every match as one OR'd id set, mirroring how the Workday and
// iCIMS adapters resolve free-text location input against a tenant's
// option catalog. A raw catalog id is also accepted and passed through
// unchanged.
func resolveUltiProLocationValues(ctx context.Context, client *ultipro.Client, input string) ([]string, error) {
	catalog, err := client.Locations(ctx)
	if err != nil {
		return nil, fmt.Errorf("ultipro: fetch location catalog: %w", err)
	}
	for _, c := range catalog {
		if c.ID == input {
			return []string{c.ID}, nil
		}
	}
	lower := strings.ToLower(input)
	var ids []string
	labels := make([]string, 0, len(catalog))
	for _, c := range catalog {
		if strings.Contains(strings.ToLower(c.Label), lower) {
			ids = append(ids, c.ID)
		}
		labels = append(labels, c.Label)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no location matching %q; available: %s", input, joinTruncated(labels))
	}
	return ids, nil
}

// joinTruncated renders a filter's available values for a teaching error,
// capped so a large catalog doesn't flood the message.
func joinTruncated(values []string) string {
	const maxListed = 20
	listed := values
	suffix := ""
	if len(listed) > maxListed {
		listed = listed[:maxListed]
		suffix = ", …"
	}
	return strings.Join(listed, ", ") + suffix
}

func ultiproSummaries(items []ultipro.Opportunity, site ultipro.CareersSite) []JobSummary {
	jobs := make([]JobSummary, 0, len(items))
	for _, o := range items {
		if o.ID == "" {
			continue
		}
		loc := ""
		if len(o.Locations) > 0 {
			loc = o.Locations[0].Display()
		}
		jobs = append(jobs, JobSummary{
			JobID:    o.ID,
			Title:    o.Title,
			Location: loc,
			PostedAt: ultiproPostedAt(o.PostedDate),
			URL:      ultiproDetailURL(site, o.ID),
		})
	}
	return jobs
}

// ultiproDetailURL builds the public posting page for opportunityID on
// site, the same URL [UltiProAdapter.Detail] reports.
func ultiproDetailURL(site ultipro.CareersSite, opportunityID string) string {
	return site.CanonicalURL() + "OpportunityDetail?opportunityId=" + url.QueryEscape(opportunityID)
}

// ultiproPostedAt trims LoadSearchResults' RFC3339 timestamp to a plain
// date, matching the other adapters' PostedAt convention.
func ultiproPostedAt(raw string) string {
	if len(raw) < len("2006-01-02") {
		return raw
	}
	return raw[:len("2006-01-02")]
}

func (a *UltiProAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	_, site, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	client := ultipro.NewClient(a.baseURL(site), a.hc)

	categories, err := client.Categories(ctx)
	if err != nil {
		return nil, fmt.Errorf("ultipro: filters %q: %w", slug, err)
	}

	fs := FilterSet{"location_type": ultiproLocationTypeLabels}
	if len(categories) > 0 {
		labels := make([]string, 0, len(categories))
		for _, c := range categories {
			labels = append(labels, c.Label)
		}
		fs["department"] = labels
	}
	return fs, nil
}

func (a *UltiProAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	name, site, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	client := ultipro.NewClient(a.baseURL(site), a.hc)

	d, err := client.Detail(ctx, jobID)
	if err != nil {
		if errors.Is(err, ultipro.ErrJobNotFound) {
			return nil, fmt.Errorf("ultipro: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
		}
		return nil, fmt.Errorf("ultipro: fetch job %q for %q: %w", jobID, slug, err)
	}

	desc := d.Description
	if desc != "" {
		if text, err := html2text.FromString(desc, html2text.Options{}); err == nil {
			desc = text
		}
	}
	loc := ""
	if len(d.Locations) > 0 {
		loc = d.Locations[0].Display()
	}

	return &JobDetail{
		JobID:       d.ID,
		Title:       d.Title,
		Company:     name,
		Location:    loc,
		PostedAt:    ultiproPostedAt(d.PostedDate),
		URL:         ultiproDetailURL(site, d.ID),
		Description: desc,
	}, nil
}
