package ats

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"golang.org/x/sync/errgroup"

	"github.com/amikai/openings-mcp/internal/provider/mokahr"
)

var _ Adapter = (*MokaHRAdapter)(nil)

// mokaHRCareersURLRE matches MokaHR careers-site URLs and captures
// "<org>/<siteId>" as one slug. Both segments are required: a tenant can own
// several sites with different job sets, so the tenant alone does not
// identify one, and ParseCareersURL cannot follow the redirect that a bare
// /social-recruitment/<org> URL answers with.
//
// Examples (hostname + escaped path):
//   - app.mokahr.com/social-recruitment/high-flyer/140576
//   - app.mokahr.com/social-recruitment/high-flyer/140576#/job/7dcd6fde-...
//   - app.mokahr.com/campus_apply/moka/118263
var mokaHRCareersURLRE = regexp.MustCompile(
	`(?i)^app\.mokahr\.com/(?:social-recruitment|campus[_-](?:apply|recruitment))/(?P<slug>[^/]+/[0-9]+)`,
)

const (
	// mokaHRDumpPageSize is the largest page MokaHR serves; 60 is rejected.
	mokaHRDumpPageSize = 50
	// maxMokaHRCandidates bounds the local-search path. The largest roster
	// board is around 1300 postings, so real tenants fit; anything past this
	// is told to narrow with the server-side filters instead.
	maxMokaHRCandidates = 2000
	// mokaHRDumpConcurrency parallelizes the full-board read. The first page
	// reports the total, so every later offset is known at once and their
	// requests do not have to queue behind each other; on the slowest roster
	// tenant that is the difference between ~17s and ~4s.
	mokaHRDumpConcurrency = 8
)

// MokaHRAdapter serves MokaHR-hosted careers sites. Structured filters
// (city, category) run server-side. Text search does not: MokaHR's keyword
// parameter matches job titles only, so honoring the unified Query semantics
// means pulling the filtered board and matching locally. The list endpoint
// already carries every posting's full description, so that dump needs no
// per-job requests.
type MokaHRAdapter struct {
	client *mokahr.JobsClient
	// dumpPageSize is mokaHRDumpPageSize outside tests, which set it small so
	// the captured fixtures span several pages.
	dumpPageSize int
}

func NewMokaHRAdapter(baseURL string, hc *http.Client) (*MokaHRAdapter, error) {
	c, err := mokahr.NewJobsClient(baseURL, hc)
	if err != nil {
		return nil, err
	}
	return &MokaHRAdapter{client: c, dumpPageSize: mokaHRDumpPageSize}, nil
}

func (a *MokaHRAdapter) Name() string { return "mokahr" }

func (a *MokaHRAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(mokahr.Companies))
	for _, c := range mokahr.Companies {
		infos = append(infos, CompanyInfo{Slug: strings.ToLower(c.Slug()), Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes app.mokahr.com careers sites. The captured
// "<org>/<siteId>" is already the slug form the roster uses, so a curated
// company's own URL resolves to its roster entry.
func (a *MokaHRAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	slug, ok := matchCareersSlug(mokaHRCareersURLRE, u)
	if !ok {
		return "", false
	}
	return strings.ToLower(slug), true
}

// CareersURL renders a curated site's public listing page. Slugs minted from
// a careers URL are not curated, so they resolve to nothing here even though
// the same "<org>/<siteId>" would address them upstream.
func (a *MokaHRAdapter) CareersURL(slug string) (string, bool) {
	c, ok := mokahr.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return "", false
	}
	return c.CareersURL(), true
}

// mokaHRSite is a resolved slug: the tenant and careers site to query, plus
// the display name to report. Sites outside the roster carry no name, so the
// tenant slug stands in.
type mokaHRSite struct {
	org  string
	site string
	name string
}

func resolveMokaHRSite(slug string) (mokaHRSite, error) {
	key := strings.ToLower(strings.TrimSpace(slug))
	if c, ok := mokahr.CompaniesBySlug[key]; ok {
		return mokaHRSite{org: c.Org, site: c.Site, name: c.Name}, nil
	}
	org, site, ok := strings.Cut(key, "/")
	if !ok || org == "" || site == "" {
		return mokaHRSite{}, fmt.Errorf(
			"mokahr: unknown company %q; pass a roster slug or an app.mokahr.com careers URL including its site id, e.g. https://app.mokahr.com/social-recruitment/high-flyer/140576",
			slug,
		)
	}
	return mokaHRSite{org: org, site: site, name: org}, nil
}

func (a *MokaHRAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	site, err := resolveMokaHRSite(slug)
	if err != nil {
		return nil, err
	}
	req := mokahr.ListJobsRequest{OrgId: site.org, SiteId: site.site}
	query := strings.TrimSpace(p.Query)
	location := strings.TrimSpace(p.Location)

	facets, err := a.siteFacets(ctx, site, len(p.Filters) > 0)
	if err != nil {
		return nil, err
	}
	if err := facets.apply(p.Filters, &req); err != nil {
		return nil, err
	}
	// A place the site itself publishes is resolved upstream rather than
	// matched locally. That is exact, it narrows the board before it is
	// read, and it is the only thing that works on a tenant whose postings
	// carry no locations at all.
	if ids, ok := facets.locationIDsFor(location); ok {
		req.LocationIds = append(req.LocationIds, ids...)
		location = ""
	}
	if query == "" && location == "" {
		return a.searchMokaHRPage(ctx, site, facets, p.Page, req)
	}
	return a.searchMokaHRDump(ctx, site, facets, query, location, p.Page, req)
}

// searchMokaHRPage is the cheap path when no local text matching is needed:
// one job request, and the upstream total and pagination stay exact.
func (a *MokaHRAdapter) searchMokaHRPage(
	ctx context.Context,
	site mokaHRSite,
	facets *mokaHRFacets,
	requestedPage int,
	req mokahr.ListJobsRequest,
) (*SearchResult, error) {
	page := clampPage(requestedPage)
	pageIndex := page - 1
	if pageIndex > math.MaxInt/pageSize {
		return nil, fmt.Errorf("mokahr: page %d is too large; retry with a smaller page", page)
	}
	req.Limit = mokahr.NewOptInt(pageSize)
	req.Offset = mokahr.NewOptInt(pageIndex * pageSize)
	req.NeedStat = mokahr.NewOptBool(true)
	list, err := a.client.ListJobs(ctx, req)
	if err != nil {
		return nil, mokaHRError(site, err)
	}
	total := list.JobStats.Value.Total.Or(len(list.Jobs))
	jobs := make([]JobSummary, 0, len(list.Jobs))
	for _, j := range list.Jobs {
		jobs = append(jobs, mokaHRSummary(site, facets, j))
	}
	return &SearchResult{
		Jobs:       jobs,
		TotalCount: total,
		Page:       page,
		TotalPages: totalPages(total),
	}, nil
}

// searchMokaHRDump pulls the board left by the server-side filters and
// applies the unified Query and Location semantics locally, because MokaHR's
// own keyword parameter ignores descriptions.
func (a *MokaHRAdapter) searchMokaHRDump(
	ctx context.Context,
	site mokaHRSite,
	facets *mokaHRFacets,
	query, location string,
	page int,
	req mokahr.ListJobsRequest,
) (*SearchResult, error) {
	pages, err := a.readMokaHRBoard(ctx, site, req)
	if err != nil {
		return nil, err
	}
	jobs := make([]dumpJob, 0, len(pages)*cmp.Or(a.dumpPageSize, mokaHRDumpPageSize))
	located := false
	for _, page := range pages {
		for _, j := range page {
			jobs = append(jobs, mokaHRDumpJob(site, facets, j))
			located = located || len(j.Locations) > 0
		}
	}
	// Some tenants keep workplaces off the list endpoint entirely. Matching a
	// location locally against a board that publishes none would answer "no
	// such jobs" when the truth is "this board cannot answer that here".
	if location != "" && !located && len(jobs) > 0 {
		return nil, fmt.Errorf(
			"mokahr: %q does not publish per-job locations in its listing, so it cannot be searched by location; drop the location, or use a city filter if get_filters reports one",
			site.name)
	}
	return searchDump(jobs, SearchParams{Query: query, Location: location, Page: page})
}

// readMokaHRBoard returns every posting the request selects, one slice per
// upstream page. The first page is fetched alone because only it reveals the
// board's size; the rest go out together, since offset paging makes each of
// them independent.
func (a *MokaHRAdapter) readMokaHRBoard(
	ctx context.Context,
	site mokaHRSite,
	req mokahr.ListJobsRequest,
) ([][]mokahr.Job, error) {
	pageSize := cmp.Or(a.dumpPageSize, mokaHRDumpPageSize)
	req.Limit = mokahr.NewOptInt(pageSize)
	req.NeedStat = mokahr.NewOptBool(true)
	req.Offset = mokahr.NewOptInt(0)
	first, err := a.client.ListJobs(ctx, req)
	if err != nil {
		return nil, mokaHRError(site, err)
	}
	total := first.JobStats.Value.Total.Or(len(first.Jobs))
	if total > maxMokaHRCandidates {
		return nil, fmt.Errorf(
			"mokahr: %q has %d postings, too many to text-search at once; narrow it first with a city or category filter",
			site.name, total)
	}
	pages := make([][]mokahr.Job, mokaHRPageCount(total, pageSize))
	pages[0] = first.Jobs
	// MokaHR answers with min(limit, remaining), so a short page is the last
	// one. Trusting that keeps every later offset a clean multiple of the
	// page size, with no gap to fall through if the total disagrees.
	if len(pages) == 1 || len(first.Jobs) < pageSize {
		return pages[:1], nil
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(mokaHRDumpConcurrency)
	for i := 1; i < len(pages); i++ {
		g.Go(func() error {
			pageReq := req
			pageReq.Offset = mokahr.NewOptInt(i * pageSize)
			list, err := a.client.ListJobs(gCtx, pageReq)
			if err != nil {
				return mokaHRError(site, err)
			}
			pages[i] = list.Jobs
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return pages, nil
}

func (a *MokaHRAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	site, err := resolveMokaHRSite(slug)
	if err != nil {
		return nil, err
	}
	facets, err := a.facets(ctx, site)
	if err != nil {
		return nil, err
	}
	fs := FilterSet{}
	if len(facets.cityLabels) > 0 {
		fs["city"] = slices.Clone(facets.cityLabels)
	}
	if len(facets.categoryLabels) > 0 {
		fs["category"] = slices.Clone(facets.categoryLabels)
	}
	return fs, nil
}

func (a *MokaHRAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	site, err := resolveMokaHRSite(slug)
	if err != nil {
		return nil, err
	}
	// The same facets the search path uses, so a workplace the listing also
	// carries reads identically in both. On the tenants that keep workplaces
	// off the listing there is nothing to agree with: search shows no place
	// and this is the only call that knows one.
	facets, err := a.siteFacets(ctx, site, false)
	if err != nil {
		return nil, err
	}
	d, err := a.client.GetJob(ctx, mokahr.GetJobRequest{
		OrgId:  site.org,
		SiteId: site.site,
		JobId:  jobID,
	})
	if err != nil {
		var apiErr *mokahr.APIError
		if errors.As(err, &apiErr) && apiErr.Code == mokahr.CodeJobNotFound {
			return nil, fmt.Errorf("mokahr: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
		}
		return nil, mokaHRError(site, err)
	}
	return &JobDetail{
		JobID:       jobID,
		Title:       d.Title,
		Company:     site.name,
		Location:    mokaHRLocationDisplay(facets, d.Locations),
		PostedAt:    mokaHRPostedAt(d.OpenedAt.Or(d.PublishedAt.Or(d.CreatedAt.Or("")))),
		URL:         mokahr.JobURL(site.org, site.site, jobID),
		Description: mokaHRDescription(d.JobDescription.Or("")),
	}, nil
}

// mokaHRPageCount is how many upstream pages a board of total postings
// occupies. It never returns zero: the first page has already been fetched by
// the time the count is needed, so there is always one page to hold it, even
// for an empty board or a total upstream declines to report.
func mokaHRPageCount(total, pageSize int) int {
	if total <= 0 || pageSize <= 0 {
		return 1
	}
	return max(1, (total+pageSize-1)/pageSize)
}

// mokaHRFacets is one site's filter catalog: display labels, the upstream ids
// each maps to, and the reverse lookup from a posting's location id to the
// place it sits in. The reverse lookup is what makes a workplace render the
// same everywhere, since the facet honors the requested locale and the
// posting's own fields do not.
type mokaHRFacets struct {
	cityLabels     []string
	cityIDs        map[string][]int // lowercased label -> location ids
	categoryLabels []string
	categoryIDs    map[string][]int // lowercased label -> 职能 ids
	cityByLocation map[int]string   // location id -> city label
	provinceByLoc  map[int]string   // location id -> province label
	provinceIDs    map[string][]int // lowercased province label -> location ids
}

// locationIDsFor resolves a free-text place to the ids MokaHR filters on,
// matching a city first and then a province. It is what lets a location
// search work on a board whose postings carry no locations at all.
func (f *mokaHRFacets) locationIDsFor(place string) ([]int, bool) {
	key := strings.ToLower(strings.TrimSpace(place))
	if ids, ok := f.cityIDs[key]; ok {
		return ids, true
	}
	ids, ok := f.provinceIDs[key]
	return ids, ok
}

// siteFacets fetches the catalog every read needs: it turns filter labels
// into the ids MokaHR takes, and it is the only thing that knows which city a
// posting's district sits in — a Hangzhou job reports "Gongshu" and nothing
// else. When no filter has to be resolved, a site that cannot report its
// facets still searches; it just falls back to district-level place names.
func (a *MokaHRAdapter) siteFacets(ctx context.Context, site mokaHRSite, required bool) (*mokaHRFacets, error) {
	facets, err := a.facets(ctx, site)
	if err != nil && required {
		return nil, err
	}
	if err != nil {
		return &mokaHRFacets{}, nil
	}
	return facets, nil
}

func (a *MokaHRAdapter) facets(ctx context.Context, site mokaHRSite) (*mokaHRFacets, error) {
	aggs, err := a.client.ListFilterAggregations(ctx, mokahr.SiteRef{OrgId: site.org, SiteId: site.site})
	if err != nil {
		return nil, mokaHRError(site, err)
	}
	f := &mokaHRFacets{
		cityIDs:        map[string][]int{},
		categoryIDs:    map[string][]int{},
		cityByLocation: map[int]string{},
		provinceByLoc:  map[int]string{},
		provinceIDs:    map[string][]int{},
	}
	system := aggs.SystemFieldsAggregations.Value
	// A city covers several districts, so its filter value maps to every
	// location id underneath it. One with no ids is never offered: it could
	// only produce a filter that quietly matches the whole board.
	for _, city := range system.LocationAggregation.Value.LocationList {
		label := strings.TrimSpace(city.Label.Or(city.ID.Or("")))
		if label == "" {
			continue
		}
		key := strings.ToLower(label)
		for _, row := range city.LocationRows {
			id, ok := row.ID.Get()
			if !ok {
				continue
			}
			if _, seen := f.cityIDs[key]; !seen {
				f.cityLabels = append(f.cityLabels, label)
			}
			f.cityIDs[key] = append(f.cityIDs[key], id)
			f.cityByLocation[id] = label
		}
	}
	// Provinces come from their own aggregation. A municipality repeats
	// itself as both province and city, which the display dedupes.
	for _, province := range system.ProvinceAggregation.Value.ProvinceList {
		label := strings.TrimSpace(province.Label.Or(province.ID.Or("")))
		if label == "" {
			continue
		}
		key := strings.ToLower(label)
		for _, row := range province.LocationRows {
			id, ok := row.ID.Get()
			if !ok {
				continue
			}
			f.provinceIDs[key] = append(f.provinceIDs[key], id)
			f.provinceByLoc[id] = label
		}
	}
	// 职能 is a two-level tree, and both levels are offered. A label that
	// appears at both collects every id it carries, which MokaHR ORs.
	var walk func(facets []mokahr.ZhinengFacet)
	walk = func(facets []mokahr.ZhinengFacet) {
		for _, z := range facets {
			label := strings.TrimSpace(z.Label.Or(""))
			id, ok := z.ID.Get()
			if label != "" && ok {
				key := strings.ToLower(label)
				if _, seen := f.categoryIDs[key]; !seen {
					f.categoryLabels = append(f.categoryLabels, label)
				}
				f.categoryIDs[key] = append(f.categoryIDs[key], id)
			}
			walk(z.Children)
		}
	}
	walk(system.ZhinengAggregation.Value.ZhinengList)
	slices.Sort(f.cityLabels)
	slices.Sort(f.categoryLabels)
	return f, nil
}

// apply maps filter labels onto the ids MokaHR takes. Values within a key OR
// together upstream, which is the unified semantics.
func (f *mokaHRFacets) apply(filters FilterSet, req *mokahr.ListJobsRequest) error {
	for key, values := range filters {
		var ids map[string][]int
		var available []string
		switch key {
		case "city":
			ids, available = f.cityIDs, f.cityLabels
		case "category":
			ids, available = f.categoryIDs, f.categoryLabels
		default:
			return errUnknownFilterKey(key, map[string]bool{"city": true, "category": true})
		}
		for _, v := range values {
			matched, ok := ids[strings.ToLower(strings.TrimSpace(v))]
			if !ok {
				if len(available) == 0 {
					return fmt.Errorf("filter key %q has no values on this careers site; it publishes no filter dimensions at all", key)
				}
				return fmt.Errorf("filter value %q not found for %q; available: %s", v, key, strings.Join(available, ", "))
			}
			if key == "city" {
				req.LocationIds = append(req.LocationIds, matched...)
			} else {
				req.ZhinengIds = append(req.ZhinengIds, matched...)
			}
		}
	}
	return nil
}

func mokaHRSummary(site mokaHRSite, facets *mokaHRFacets, j mokahr.Job) JobSummary {
	return JobSummary{
		JobID:    j.ID,
		Title:    j.Title,
		Location: mokaHRLocationDisplay(facets, j.Locations),
		PostedAt: mokaHRPostedAt(j.OpenedAt.Or(j.CreatedAt.Or(""))),
		URL:      mokahr.JobURL(site.org, site.site, j.ID),
	}
}

func mokaHRDumpJob(site mokaHRSite, facets *mokaHRFacets, j mokahr.Job) dumpJob {
	summary := mokaHRSummary(site, facets, j)
	fields := map[string][]string{}
	if cities := mokaHRCities(facets, j.Locations); len(cities) > 0 {
		fields["city"] = cities
	}
	category := ""
	if z, ok := j.Zhineng.Get(); ok {
		category = z.Name.Or("")
	}
	if category != "" {
		fields["category"] = []string{category}
	}
	posted, _ := mokaHRParseTime(j.OpenedAt.Or(j.CreatedAt.Or("")))
	return dumpJob{
		summary:     summary,
		sortKey:     posted,
		orgUnit:     category,
		description: mokaHRDescription(j.JobDescription.Or("")),
		locations:   mokaHRLocationSearch(facets, j.Locations),
		fields:      fields,
	}
}

// mokaHRCities names the cities a posting sits in. A posting reports its
// district ("Haidian"), so the city comes from the site's own location facet
// when it is known, and from the province otherwise.
func mokaHRCities(facets *mokaHRFacets, locations []mokahr.Location) []string {
	cities := make([]string, 0, len(locations))
	for _, l := range locations {
		city := facets.cityByLocation[l.ID.Or(0)]
		cities = appendDistinct(cities, cmp.Or(city, l.ProvinceName.Or(""), l.CityName.Or("")))
	}
	return cities
}

// mokaHRLocationDisplay renders "City, Province, Country" per workplace,
// joined with "; ". Every component prefers the site's facet over the
// posting's own field, because the facet is the one source that answers the
// same way for a search hit and for the posting behind it.
func mokaHRLocationDisplay(facets *mokaHRFacets, locations []mokahr.Location) string {
	places := make([]string, 0, len(locations))
	for _, l := range locations {
		id := l.ID.Or(0)
		parts := make([]string, 0, 3)
		for _, p := range []string{
			cmp.Or(facets.cityByLocation[id], l.CityName.Or("")),
			cmp.Or(facets.provinceByLoc[id], l.ProvinceName.Or("")),
			// getJob answers with a Chinese country and the localized name
			// beside it; listJobs answers with only the localized one.
			cmp.Or(l.CountryDescription.Or(""), l.Country.Or("")),
		} {
			parts = appendDistinct(parts, p)
		}
		places = appendDistinct(places, strings.Join(parts, ", "))
	}
	// The two endpoints list a posting's workplaces in different orders, and
	// neither order carries meaning, so sorting is what makes one job read
	// the same way wherever it is rendered.
	slices.Sort(places)
	return strings.Join(places, "; ")
}

// mokaHRLocationSearch joins every place component so free-text location
// matching hits on district, city, province, or country alike.
func mokaHRLocationSearch(facets *mokaHRFacets, locations []mokahr.Location) string {
	parts := make([]string, 0, len(locations)*4)
	for _, l := range locations {
		id := l.ID.Or(0)
		parts = appendDistinct(parts, facets.cityByLocation[id])
		parts = appendDistinct(parts, facets.provinceByLoc[id])
		parts = appendDistinct(parts, l.CityName.Or(""))
		parts = appendDistinct(parts, l.ProvinceName.Or(""))
		parts = appendDistinct(parts, l.CountryDescription.Or(""))
		parts = appendDistinct(parts, l.Country.Or(""))
	}
	return strings.Join(parts, "; ")
}

// mokaHRDescription renders a posting's HTML description as plain text. The
// list endpoint carries the same field as the detail endpoint, so both go
// through here.
func mokaHRDescription(html string) string {
	if html == "" {
		return ""
	}
	text, err := html2text.FromString(html, html2text.Options{})
	if err != nil {
		return html
	}
	return text
}

// mokaHRTimeLayouts are MokaHR's zone-less wall-clock stamps: seconds
// precision on createdAt/publishedAt, minutes on openedAt.
var mokaHRTimeLayouts = []string{"2006-01-02T15:04:05", "2006-01-02T15:04"}

func mokaHRParseTime(s string) (time.Time, bool) {
	for _, layout := range mokaHRTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// mokaHRPostedAt renders the unified date, falling back to the raw upstream
// text when the stamp is in a shape this client has not seen.
func mokaHRPostedAt(s string) string {
	if t, ok := mokaHRParseTime(s); ok {
		return isoDate(t)
	}
	return s
}

// mokaHRError names the site in errors from a provider that answers HTTP 200
// with a Chinese message for every failure, including unknown tenants.
func mokaHRError(site mokaHRSite, err error) error {
	var apiErr *mokahr.APIError
	if errors.As(err, &apiErr) && apiErr.Code == mokahr.CodeSiteNotFound {
		return fmt.Errorf("mokahr: careers site %s/%s not found upstream; check the org and site id in the careers URL", site.org, site.site)
	}
	return fmt.Errorf("mokahr: %q: %w", site.name, err)
}
