package ats

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/adp_wfn"
)

var _ Adapter = (*ADPWFNAdapter)(nil)

// Filter keys this adapter publishes. They are fixed rather than derived from
// the tenant, unlike ADP MyJobs, because Workforce Now has exactly two
// dimensions and each one goes on the wire through its own request header.
const (
	adpWFNLocationKey = "location"
	adpWFNJobTypeKey  = "job_type"
)

// ADPWFNAdapter serves public ADP Workforce Now career centers
// (workforcenow.adp.com). ADP MyJobs is a separate surface served by
// [ADPMyJobsAdapter]; see docs/adr/0001-separate-adp-provider-packages.md.
//
// Search, location, and job type are all applied upstream in one request. The
// keyword is the exception that shapes the rest of this adapter: the upstream
// orders relevance results differently between identical calls, so a keyword
// search serves page one and reports a single page rather than paging into
// duplicates and gaps.
type ADPWFNAdapter struct {
	hc        *http.Client
	baseURL   string
	legacyURL string
}

// NewADPWFNAdapter builds an adapter using the shared HTTP client. It takes no
// [DumpCache]: this provider pages its board upstream rather than dumping it,
// so the only thing worth caching would be each tenant's filter catalog, and
// omitting that costs one extra request rather than a growing number of them.
func NewADPWFNAdapter(hc *http.Client) *ADPWFNAdapter {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &ADPWFNAdapter{
		hc:        hc,
		baseURL:   adp_wfn.DefaultBaseURL,
		legacyURL: adp_wfn.DefaultLegacyBaseURL,
	}
}

func (a *ADPWFNAdapter) Name() string { return "adp_wfn" }

func (a *ADPWFNAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(adp_wfn.Companies))
	for _, c := range adp_wfn.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Slug, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes both the current recruitment.html form and the
// retired posting.html?client=<slug> form.
//
// A roster tenant resolves to its curated slug. Anything else is minted as a
// canonical careers URL rather than a bare cid, because the cid alone cannot
// carry the tenant's locale — and this surface answers the wrong locale with
// an empty board rather than an error, so losing it would silently report an
// unlisted company as having no openings.
//
// Resolving the retired form costs a live request, which is why it is tried
// only after the current form fails to match. Not every legacy slug resolves;
// an unresolvable one is reported as an unrecognized URL so the caller is
// steered to the current form rather than handed an error it cannot act on.
func (a *ADPWFNAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	host := strings.ToLower(u.Hostname())
	if host != "workforcenow.adp.com" {
		return "", false
	}
	path := strings.ToLower(u.EscapedPath())
	q := u.Query()

	switch {
	case strings.HasSuffix(path, "/recruitment.html"), strings.HasSuffix(path, "/intermediateredirect.html"):
		cid := strings.TrimSpace(q.Get("cid"))
		if cid == "" {
			return "", false
		}
		return a.slugForCID(cid, strings.TrimSpace(q.Get("ccId")), strings.TrimSpace(q.Get("lang"))), true
	case strings.HasSuffix(path, "/posting.html"):
		slug := strings.TrimSpace(q.Get("client"))
		if slug == "" {
			return "", false
		}
		cid, ok := a.client().ResolveLegacySlug(slug)
		if !ok {
			return "", false
		}
		return a.slugForCID(cid, strings.TrimSpace(q.Get("ccId")), ""), true
	default:
		return "", false
	}
}

// slugForCID prefers the roster's curated slug, falling back to a canonical
// URL that carries everything Search and Detail will need.
func (a *ADPWFNAdapter) slugForCID(cid, ccID, locale string) string {
	if c, ok := adp_wfn.CompaniesByCID[strings.ToLower(cid)]; ok {
		return c.Slug
	}
	return adp_wfn.BoardURL(cid, ccID, locale)
}

// CareersURL renders the roster company's public career page.
func (a *ADPWFNAdapter) CareersURL(slug string) (string, bool) {
	c, ok := adp_wfn.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return "", false
	}
	return c.CareersURL(), true
}

// adpWFNTenant is a resolved company: everything needed to address the API,
// plus the display name when the roster supplies one.
type adpWFNTenant struct {
	cid    string
	ccID   string
	locale string
	name   string
	listed bool
}

// resolveSlug maps a slug to its tenant. A roster slug carries its locale from
// companies.yaml; a minted careers URL carries whatever the input URL had.
func (a *ADPWFNAdapter) resolveSlug(slug string) (adpWFNTenant, error) {
	if c, ok := adp_wfn.CompaniesBySlug[strings.ToLower(strings.TrimSpace(slug))]; ok {
		return adpWFNTenant{cid: c.CID, ccID: c.CCID, locale: c.Locale, name: c.Name, listed: true}, nil
	}
	if u, ok := parseCareersInput(slug); ok {
		q := u.Query()
		if cid := strings.TrimSpace(q.Get("cid")); cid != "" {
			return adpWFNTenant{
				cid:    cid,
				ccID:   strings.TrimSpace(q.Get("ccId")),
				locale: strings.TrimSpace(q.Get("lang")),
			}, nil
		}
	}
	return adpWFNTenant{}, fmt.Errorf(
		"adp_wfn: unknown company %q; pass a roster slug or a workforcenow.adp.com careers URL", slug,
	)
}

// locale returns the locale to pin on a request, discovering it for an
// unlisted tenant that arrived without one.
//
// The upstream answers an unknown locale by falling back to en_US, and answers
// a known-but-wrong one with an empty board. Both are indistinguishable from a
// tenant with no openings, so guessing is not an option when the tenant did
// not tell us.
//
// A discovery failure is therefore reported rather than absorbed: proceeding
// without a locale would turn a timed-out or malformed lookup into a confident
// "this company has no openings". An empty return is reserved for the one case
// that is genuinely benign — a tenant that advertises no locale at all, where
// the upstream default is as good an answer as exists.
func (a *ADPWFNAdapter) locale(ctx context.Context, t adpWFNTenant) (string, error) {
	if t.locale != "" {
		return t.locale, nil
	}
	locale, err := a.client().PrimaryLocale(ctx, t.cid)
	if err != nil {
		return "", fmt.Errorf("adp_wfn: discover locale for %q: %w", t.cid, err)
	}
	return locale, nil
}

func (a *ADPWFNAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	tenant, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	page := clampPage(p.Page)
	query := strings.TrimSpace(p.Query)

	// Relevance ordering is not stable between identical calls, so windows
	// past the first overlap and drop rows. Rather than serve a page that is
	// quietly wrong, a keyword search is one page long.
	if query != "" && page > 1 {
		return &SearchResult{Jobs: []JobSummary{}, TotalCount: 0, Page: page, TotalPages: 1}, nil
	}

	// Resolved once and threaded through: discovering it costs a request for a
	// tenant that arrived without one, and the rendered URLs must carry the
	// same locale the board was read under or the link opens an empty page.
	locale, err := a.locale(ctx, tenant)
	if err != nil {
		return nil, err
	}

	locations, categories, err := a.resolveFilters(ctx, tenant, locale, p)
	if err != nil {
		return nil, err
	}

	res, err := a.client().List(ctx, tenant.cid, adp_wfn.ListParams{
		Locale:           locale,
		Query:            query,
		Locations:        locations,
		WorkerCategories: categories,
		Page:             page,
	})
	if err != nil {
		return nil, err
	}

	jobs := make([]JobSummary, 0, len(res.Jobs))
	for _, r := range res.Jobs {
		// Detail accepts either id, but only the external one builds a share
		// link that resolves, so a row without it keeps its posting and loses
		// its URL rather than carrying a link that 404s.
		id, url := adp_wfn.ExternalJobID(r), ""
		if id != "" {
			url = adp_wfn.JobURL(tenant.cid, tenant.ccID, id, locale)
		} else {
			id = strings.TrimSpace(r.ItemID.Value)
		}
		if id == "" {
			continue
		}
		posted := ""
		if t, ok := adp_wfn.PostedTime(r); ok {
			posted = isoDate(t)
		}
		jobs = append(jobs, JobSummary{
			JobID:    id,
			Title:    strings.TrimSpace(r.RequisitionTitle.Value),
			Location: adp_wfn.PrimaryLocation(r),
			PostedAt: posted,
			URL:      url,
		})
	}

	if query != "" {
		// One page by construction, so the rows in hand are the whole result.
		return &SearchResult{Jobs: jobs, TotalCount: len(jobs), Page: page, TotalPages: 1}, nil
	}
	if res.TotalTrusted {
		return &SearchResult{
			Jobs:       jobs,
			TotalCount: res.TotalNumber,
			Page:       page,
			TotalPages: totalPages(res.TotalNumber),
		}, nil
	}
	// Under a filter the upstream count is a relevance tally that overshoots
	// the board, so report the lower bound this walk proved and let a full
	// page stand in for "there is more", the same way the Avature adapter
	// handles portals whose totals are unknowable from one page.
	total := (page-1)*pageSize + len(jobs)
	if res.HasMore {
		total++
	}
	return &SearchResult{Jobs: jobs, TotalCount: total, Page: page, TotalPages: totalPages(total)}, nil
}

// resolveFilters translates caller filters into wire values, rejecting
// anything it cannot validate.
//
// Rejection is not politeness. An unpaired location value is answered with the
// whole unfiltered board, and an unpublished job-type oid with a large subset
// that looks filtered but is not — both HTTP 200. Forwarding an unchecked
// value would turn a caller's typo into a confidently wrong answer.
func (a *ADPWFNAdapter) resolveFilters(ctx context.Context, t adpWFNTenant, locale string, p SearchParams) (locations, categories []string, err error) {
	location := strings.TrimSpace(p.Location)
	if len(p.Filters) == 0 && location == "" {
		return nil, nil, nil
	}

	catalog, err := a.client().SearchFilters(ctx, t.cid, locale)
	if err != nil {
		return nil, nil, err
	}

	valid := map[string]bool{adpWFNLocationKey: true, adpWFNJobTypeKey: true}
	for _, key := range slices.Sorted(maps.Keys(p.Filters)) {
		if !valid[key] {
			return nil, nil, errUnknownFilterKey(key, valid)
		}
		for _, raw := range p.Filters[key] {
			switch key {
			case adpWFNLocationKey:
				wire, ok := matchFilterValue(catalog.Locations, raw)
				if !ok {
					return nil, nil, adpWFNUnknownValue(key, raw, catalog.Locations)
				}
				locations = append(locations, wire)
			case adpWFNJobTypeKey:
				wire, ok := matchFilterValue(catalog.WorkerCategories, raw)
				if !ok {
					return nil, nil, adpWFNUnknownValue(key, raw, catalog.WorkerCategories)
				}
				categories = append(categories, wire)
			}
		}
	}

	if location != "" {
		wire, ok := adpWFNLocationPair(catalog.Locations, location)
		if !ok {
			return nil, nil, fmt.Errorf(
				"adp_wfn: location %q could not be turned into a city or state filter; "+
					"pass a city as \"City, ST\" or a two-letter state code, or call get_filters_by_company for this board's published locations",
				location,
			)
		}
		locations = append(locations, wire)
	}
	return locations, categories, nil
}

// matchFilterValue resolves caller text against a published dimension, on
// either the label or the wire value.
func matchFilterValue(values []adp_wfn.FilterValue, raw string) (string, bool) {
	needle := strings.ToLower(strings.TrimSpace(raw))
	if needle == "" {
		return "", false
	}
	for _, v := range values {
		if strings.ToLower(v.Label) == needle || strings.ToLower(v.Wire) == needle {
			return v.Wire, true
		}
	}
	return "", false
}

// adpWFNLocationPair turns free text into a wire-ready location.
//
// A published value is preferred, since it is guaranteed well formed. Failing
// that a pair is constructed, which is safe in a way that forwarding the raw
// text is not: a constructed pair naming somewhere the board does not use
// returns zero rows, while a bare token returns the whole board. Boards that
// publish no location dimension still filter correctly, so construction is the
// only path available for them.
func adpWFNLocationPair(published []adp_wfn.FilterValue, location string) (string, bool) {
	if wire, ok := matchFilterValue(published, location); ok {
		return wire, true
	}
	location = strings.TrimSpace(location)
	if location == "" {
		return "", false
	}
	if len(location) == 2 && isAllLetters(location) {
		return strings.ToUpper(location) + ",LOCATION_STATE", true
	}
	// "Markle, IN" already reads as a pair upstream, since a bare state code
	// acts as a qualifier. Both halves have to be present: "Clarinda," carries
	// a comma but is still an unpaired token upstream, and unpaired tokens are
	// answered with the whole unfiltered board.
	if value, qualifier, ok := strings.Cut(location, ","); ok {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(qualifier) != "" {
			return location, true
		}
	}
	return strings.Trim(location, " ,") + ",LOCATION_CITY", true
}

func isAllLetters(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func adpWFNUnknownValue(key, raw string, values []adp_wfn.FilterValue) error {
	labels := make([]string, 0, len(values))
	for _, v := range values {
		labels = append(labels, v.Label)
	}
	if len(labels) == 0 {
		return fmt.Errorf("adp_wfn: filter %q is not published by this board", key)
	}
	return fmt.Errorf("adp_wfn: filter value %q not found for %q; available: %s", raw, key, strings.Join(labels, ", "))
}

// Filters reports the two dimensions this surface supports, omitting either
// one the tenant publishes no values for.
func (a *ADPWFNAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	tenant, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	locale, err := a.locale(ctx, tenant)
	if err != nil {
		return nil, err
	}
	catalog, err := a.client().SearchFilters(ctx, tenant.cid, locale)
	if err != nil {
		return nil, err
	}
	fs := FilterSet{}
	if labels := filterLabels(catalog.Locations); len(labels) > 0 {
		fs[adpWFNLocationKey] = labels
	}
	if labels := filterLabels(catalog.WorkerCategories); len(labels) > 0 {
		fs[adpWFNJobTypeKey] = labels
	}
	return fs, nil
}

func filterLabels(values []adp_wfn.FilterValue) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v.Label != "" {
			out = append(out, v.Label)
		}
	}
	return out
}

func (a *ADPWFNAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	tenant, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	locale, err := a.locale(ctx, tenant)
	if err != nil {
		return nil, err
	}
	r, err := a.client().Job(ctx, tenant.cid, strings.TrimSpace(jobID), locale)
	if err != nil {
		return nil, err
	}

	desc, _ := html2text.FromString(r.RequisitionDescription.Value, html2text.Options{PrettyTables: false})
	// The unified detail has no salary field, and this surface publishes a
	// structured range on the rows that have one, so it is folded into the
	// description rather than dropped.
	if salary := adp_wfn.SalaryLine(*r); salary != "" {
		desc = salary + "\n\n" + desc
	}

	// Same split as Search: the external id is the only one the public page
	// resolves, so a posting that lacks it is returned without a URL rather
	// than with a broken one.
	id, url := adp_wfn.ExternalJobID(*r), ""
	if id != "" {
		url = adp_wfn.JobURL(tenant.cid, tenant.ccID, id, locale)
	} else {
		id = strings.TrimSpace(r.ItemID.Value)
	}
	posted := ""
	if t, ok := adp_wfn.PostedTime(*r); ok {
		posted = isoDate(t)
	}

	return &JobDetail{
		JobID:       id,
		Title:       strings.TrimSpace(r.RequisitionTitle.Value),
		Company:     a.companyName(ctx, tenant),
		Location:    adp_wfn.PrimaryLocation(*r),
		PostedAt:    posted,
		URL:         url,
		Description: desc,
	}, nil
}

// companyName resolves who a posting belongs to.
//
// The roster is authoritative. For an unlisted tenant the name is read from
// ADP's own client record, verbatim — it shows truncation and odd casing, but
// the job endpoints carry no name at all, and rebuilding one from a slug or a
// domain would be inventing it. A failed lookup leaves the field empty rather
// than failing the call, since a description is what the caller came for.
func (a *ADPWFNAdapter) companyName(ctx context.Context, t adpWFNTenant) string {
	if t.listed {
		return t.name
	}
	info, err := a.client().TenantInfo(ctx, t.cid)
	if err != nil {
		return ""
	}
	return info.ClientName
}

func (a *ADPWFNAdapter) client() *adp_wfn.BoardClient {
	c, err := adp_wfn.NewBoardClient(adp_wfn.Config{
		HTTPClient:    a.hc,
		BaseURL:       a.baseURL,
		LegacyBaseURL: a.legacyURL,
	})
	if err != nil {
		// The only failure mode is an unparseable base URL, which is a
		// constant here and in tests.
		panic(fmt.Sprintf("adp_wfn: build client: %v", err))
	}
	return c
}
