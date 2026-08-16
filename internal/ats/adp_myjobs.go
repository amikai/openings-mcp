package ats

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/adp_myjobs"
)

var _ Adapter = (*ADPMyJobsAdapter)(nil)

// adpMyJobsCareersURLRE matches public MyJobs board URLs and captures the
// tenant slug (first path segment).
//
// Examples (host + path):
//   - myjobs.adp.com/guitarcenterexternal
//   - myjobs.adp.com/guitarcenterexternal/cx/job/123
var _adpMyJobsCareersURLRE = regexp.MustCompile(
	`(?i)^myjobs\.adp\.com/(?P<slug>[^/]+)`,
)

// ADPMyJobsAdapter serves public ADP MyJobs career boards (myjobs.adp.com).
// Keyword search uses the upstream OData $search parameter with server-side
// pagination, and filters use the tenant's own dimensions, both applied in the
// same request. Nothing is matched locally and no board dump is needed.
//
// The dimensions are whatever the tenant configured — State and City on one
// board, Store or Brand or Compensation Range on another — so [Adapter.Filters]
// reports them per company and [SearchParams.Location] is not supported: a
// board that files its jobs by store cannot answer a free-text place.
// Workforce Now (workforcenow.adp.com) is out of scope (future adp_wfn).
type ADPMyJobsAdapter struct {
	hc          *http.Client
	careerBase  string
	listingBase string
	// dumpCache holds no dump for this adapter; see facets for what it stores.
	dumpCache *DumpCache
}

// NewADPMyJobsAdapter builds an adapter using the shared HTTP client.
// dumpCache is reused to hold each board's filter catalog rather than any board
// dump (see facets); nil disables that caching without changing behaviour.
func NewADPMyJobsAdapter(hc *http.Client, dumpCache *DumpCache) *ADPMyJobsAdapter {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &ADPMyJobsAdapter{
		hc:          hc,
		careerBase:  adp_myjobs.DefaultCareerSiteBase,
		listingBase: adp_myjobs.DefaultListingBase,
		dumpCache:   dumpCache,
	}
}

func (a *ADPMyJobsAdapter) Name() string { return "adp_myjobs" }

func (a *ADPMyJobsAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(adp_myjobs.Companies))
	for _, c := range adp_myjobs.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Slug, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes myjobs.adp.com/{slug}/... boards only.
func (a *ADPMyJobsAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	slug, ok := matchCareersSlug(_adpMyJobsCareersURLRE, u)
	if !ok {
		return "", false
	}
	slug = strings.ToLower(slug)
	if slug == "public" {
		return "", false
	}
	return slug, true
}

// CareersURL renders the roster company's public MyJobs page.
func (a *ADPMyJobsAdapter) CareersURL(slug string) (string, bool) {
	c, ok := adp_myjobs.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return "", false
	}
	return c.CareersURL(), true
}

func (a *ADPMyJobsAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	slug = strings.ToLower(slug)
	page := clampPage(p.Page)
	pageIndex := page - 1
	skip := pageIndex * _pageSize

	// Every board files its jobs by its own dimensions, and several file them
	// by store rather than by place, so there is nothing a free-text location
	// could be matched against without reading the whole board.
	if location := strings.TrimSpace(p.Location); location != "" {
		return nil, fmt.Errorf(
			"adp_myjobs: location is not supported; call get_filters_by_company and pass one of "+
				"its dimensions through filters instead (location %q)",
			location,
		)
	}

	var customFilters []adp_myjobs.CustomFilter
	if len(p.Filters) > 0 {
		facets, err := a.facets(ctx, slug)
		if err != nil {
			return nil, err
		}
		customFilters, err = facets.resolve(p.Filters)
		if err != nil {
			return nil, err
		}
	}

	// Keyword search uses upstream $search and filters the tenant's dimensions,
	// so both constraints are applied server-side in the same request.
	search := strings.TrimSpace(p.Query)

	client := a.client()
	res, err := client.ListJobRequisitions(ctx, slug, adp_myjobs.ListParams{
		Search:        search,
		CustomFilters: customFilters,
		Skip:          skip,
		Top:           _pageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("adp_myjobs: search %q: %w", slug, err)
	}

	jobs := make([]JobSummary, 0, len(res.JobRequisitions))
	for _, r := range res.JobRequisitions {
		id := r.ReqIDString()
		if id == "" {
			continue
		}
		posted := ""
		if t, ok := r.PostedTime(); ok {
			posted = isoDate(t)
		}
		jobs = append(jobs, JobSummary{
			JobID:    id,
			Title:    r.Title(),
			Location: r.PrimaryLocation(),
			PostedAt: posted,
			URL:      adp_myjobs.ApplyURL(slug, id),
		})
	}

	total := res.Count
	pages := 0
	if total > 0 {
		pages = totalPages(total)
	}
	return &SearchResult{
		Jobs:       jobs,
		TotalCount: total,
		Page:       page,
		TotalPages: pages,
	}, nil
}

func (a *ADPMyJobsAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	facets, err := a.facets(ctx, slug)
	if err != nil {
		return nil, err
	}
	return facets.filterSet(), nil
}

// adpFacets is one board's filter dimensions, in both directions: the labels
// and values published to callers, and the slot code plus canonical spelling
// each of them has to be translated back into before it can be sent upstream.
type adpFacets struct {
	keys   []string                     // published keys, in the tenant's own order
	fields map[string]string            // key -> slot code ("FIELD1")
	labels map[string][]string          // key -> published values
	values map[string]map[string]string // key -> lowercased value -> canonical value
}

func (f *adpFacets) filterSet() FilterSet {
	fs := make(FilterSet, len(f.keys))
	for _, key := range f.keys {
		fs[key] = f.labels[key]
	}
	return fs
}

// resolve turns caller-supplied filters into upstream clauses, rejecting
// anything it cannot translate. Both rejections matter: an unknown slot code is
// answered with the whole unfiltered board, and a value whose case is off is
// answered with no jobs at all, so neither may be forwarded unchecked.
func (f *adpFacets) resolve(filters FilterSet) ([]adp_myjobs.CustomFilter, error) {
	valid := make(map[string]bool, len(f.keys))
	for _, key := range f.keys {
		valid[key] = true
	}

	out := make([]adp_myjobs.CustomFilter, 0, len(filters))
	for _, key := range slices.Sorted(maps.Keys(filters)) {
		if !valid[key] {
			return nil, errUnknownFilterKey(key, valid)
		}
		values := filters[key]
		// Upstream has no OR: "a || b" matches nothing and "(a or b)" is
		// dropped in favour of the whole board, so a second value cannot be
		// expressed and must not be silently ignored either.
		if len(values) > 1 {
			return nil, fmt.Errorf(
				"adp_myjobs: filter %q takes one value per search, got %d (%s); search each separately",
				key, len(values), strings.Join(values, ", "),
			)
		}
		canonical, ok := f.values[key][strings.ToLower(strings.TrimSpace(values[0]))]
		if !ok {
			return nil, fmt.Errorf(
				"adp_myjobs: filter value %q not found for %q; available: %s",
				values[0], key, strings.Join(f.labels[key], ", "),
			)
		}
		out = append(out, adp_myjobs.CustomFilter{Field: f.fields[key], Value: canonical})
	}
	return out, nil
}

// facets returns the board's cached filter catalog, fetching it once per slug.
//
// Despite the call below, this adapter does not dump anything: it borrows
// [DumpCache] only for the per-slug keying, TTL and size bound, and stores the
// catalog in the side channel with nil jobs. Reading `getOrLoadDump` here does
// not mean the board is being paginated — a whole-board read is exactly what
// filtering through the tenant's own dimensions replaced. Two consequences are
// worth knowing: the entry occupies one of the cache's slots even though it is
// far smaller than a real dump, and --dump-cache-ttl therefore also governs how
// often this catalog is refetched. Nothing here depends on the cache for
// correctness; a miss or eviction just refetches, and a nil cache is fine.
func (a *ADPMyJobsAdapter) facets(ctx context.Context, slug string) (*adpFacets, error) {
	_, side, err := a.dumpCache.getOrLoadDump(ctx, a.Name(), slug, func(ctx context.Context) ([]dumpJob, any, error) {
		catalog, err := a.client().GetCustomFilters(ctx, slug)
		if err != nil {
			return nil, nil, err
		}
		return nil, buildADPFacets(catalog), nil
	})
	if err != nil {
		return nil, err
	}
	facets, ok := side.(*adpFacets)
	if !ok {
		return nil, fmt.Errorf("adp_myjobs: filter cache for %q has an invalid value", slug)
	}
	return facets, nil
}

func buildADPFacets(catalog *adp_myjobs.CustomFilterCatalog) *adpFacets {
	f := &adpFacets{
		fields: make(map[string]string),
		labels: make(map[string][]string),
		values: make(map[string]map[string]string),
	}
	for _, category := range catalog.FilterList {
		if strings.TrimSpace(category.Category) == "" || len(category.FilterList) == 0 {
			continue
		}
		key := adpFilterKey(category.CategoryLabel, category.Category)
		// A board may configure two dimensions under one label, and they are
		// not interchangeable -- Guitar Center's second "Location" carries 188
		// stores the first one never lists -- so the later one is suffixed
		// rather than dropped.
		for n := 2; f.fields[key] != ""; n++ {
			key = fmt.Sprintf("%s_%d", adpFilterKey(category.CategoryLabel, category.Category), n)
		}

		labels := make([]string, 0, len(category.FilterList))
		canonical := make(map[string]string, len(category.FilterList))
		for _, v := range category.FilterList {
			value := strings.TrimSpace(v.Value)
			if value == "" {
				continue
			}
			label := strings.TrimSpace(v.Label)
			if label == "" {
				label = value
			}
			labels = append(labels, label)
			canonical[strings.ToLower(value)] = value
			canonical[strings.ToLower(label)] = value
		}
		if len(labels) == 0 {
			continue
		}
		f.keys = append(f.keys, key)
		f.fields[key] = category.Category
		f.labels[key] = labels
		f.values[key] = canonical
	}
	return f
}

// adpFilterKey normalizes a tenant's display label into a filter key, falling
// back to the slot code for a label that normalizes to nothing.
func adpFilterKey(label, category string) string {
	key := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			return unicode.ToLower(r)
		default:
			return '_'
		}
	}, strings.TrimSpace(label))
	for strings.Contains(key, "__") {
		key = strings.ReplaceAll(key, "__", "_")
	}
	key = strings.Trim(key, "_")
	if key == "" {
		return strings.ToLower(category)
	}
	return key
}

func (a *ADPMyJobsAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	slug = strings.ToLower(slug)
	jobID = strings.TrimSpace(jobID)
	r, err := a.client().GetJobRequisition(ctx, slug, jobID)
	if err != nil {
		return nil, err
	}
	descHTML := r.JobDescription
	if r.JobQualifications != "" {
		descHTML = descHTML + "\n" + r.JobQualifications
	}
	desc, _ := html2text.FromString(descHTML, html2text.Options{PrettyTables: false})
	posted := ""
	if t, ok := r.PostedTime(); ok {
		posted = isoDate(t)
	}
	return &JobDetail{
		JobID:       r.ReqIDString(),
		Title:       r.Title(),
		Company:     cmp.Or(adp_myjobs.CompaniesBySlug[slug].Name, slug),
		Location:    r.PrimaryLocation(),
		PostedAt:    posted,
		URL:         adp_myjobs.ApplyURL(slug, r.ReqIDString()),
		Description: desc,
	}, nil
}

func (a *ADPMyJobsAdapter) client() *adp_myjobs.Client {
	return adp_myjobs.NewClient(adp_myjobs.Config{
		HTTPClient:     a.hc,
		CareerSiteBase: a.careerBase,
		ListingBase:    a.listingBase,
	})
}
