package ats

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
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

// ParseCareersURL recognizes UltiPro career-board URLs. Roster companies
// fold back to their roster slug (company code) so display names stay
// identical to name-based resolution; unknown boards get the canonical URL
// as a self-describing slug (host, company code, and board id all matter,
// which a bare code can't carry).
func (a *UltiProAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	site, ok := ultipro.ParseCareersURL(u)
	if !ok {
		return "", false
	}
	if _, ok := ultipro.CompaniesByCode[strings.ToLower(site.CompanyCode)]; ok {
		return strings.ToLower(site.CompanyCode), true
	}
	return site.CanonicalURL(), true
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

	filters, err := a.buildFilters(ctx, client, p.Filters)
	if err != nil {
		return nil, err
	}
	if loc := strings.TrimSpace(p.Location); loc != "" {
		id, err := resolveUltiProCatalogValue(ctx, client.Locations, loc, "location")
		if err != nil {
			return nil, err
		}
		filters = append(filters, ultipro.SearchFilter{FieldName: 4, Values: []string{id}})
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
		Jobs:       ultiproSummaries(res.Opportunities),
		TotalCount: res.TotalCount,
		Page:       page,
		TotalPages: totalPages(res.TotalCount),
	}, nil
}

// buildFilters maps the unified department/location_type filter keys onto
// LoadSearchResults' fieldName codes. FieldName 6 (schedule) is
// deliberately unsupported — see internal/provider/ultipro/openapi.yaml.
func (a *UltiProAdapter) buildFilters(ctx context.Context, client *ultipro.Client, filters FilterSet) ([]ultipro.SearchFilter, error) {
	var out []ultipro.SearchFilter
	for key, values := range filters {
		switch key {
		case "department":
			for _, v := range values {
				id, err := resolveUltiProCatalogValue(ctx, client.Categories, v, "department")
				if err != nil {
					return nil, err
				}
				out = append(out, ultipro.SearchFilter{FieldName: 5, Values: []string{id}})
			}
		case "location_type":
			for _, v := range values {
				code, ok := ultiproLocationTypeCodes[strings.ToLower(strings.TrimSpace(v))]
				if !ok {
					return nil, fmt.Errorf("filter value %q not found for %q; available: %s", v, key, strings.Join(ultiproLocationTypeLabels, ", "))
				}
				out = append(out, ultipro.SearchFilter{FieldName: 37, Values: []string{code}})
			}
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
	const maxListed = 20
	listed := labels
	suffix := ""
	if len(listed) > maxListed {
		listed = listed[:maxListed]
		suffix = ", …"
	}
	return "", fmt.Errorf("filter value %q not found for %q; available: %s%s", input, key, strings.Join(listed, ", "), suffix)
}

func ultiproSummaries(items []ultipro.Opportunity) []JobSummary {
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
			URL:      "", // the search response carries no per-posting URL; Detail's URL is authoritative.
		})
	}
	return jobs
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
		return nil, fmt.Errorf("ultipro: job %q not found for company %q; pass a job_id exactly as returned by the job search: %w", jobID, slug, err)
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
		URL:         site.CanonicalURL() + "OpportunityDetail?opportunityId=" + url.QueryEscape(d.ID),
		Description: desc,
	}, nil
}
