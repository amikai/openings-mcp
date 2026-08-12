package ats

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
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
var adpMyJobsCareersURLRE = regexp.MustCompile(
	`(?i)^myjobs\.adp\.com/(?P<slug>[^/]+)`,
)

const adpLocationFilterKey = "location"

// adpGeoPad widens a resolved bounding box on every side. A box drawn around a
// single posting location has zero area, which upstream answers with HTTP 500.
// 0.0001 degree is roughly 11 m, small enough to keep one store distinct from
// its neighbours.
const adpGeoPad = 0.0001

// ADPMyJobsAdapter serves public ADP MyJobs career boards (myjobs.adp.com).
// Keyword search uses the upstream OData $search parameter with server-side
// pagination. Location labels come from a cached board listing and are resolved
// to the coordinates of the matching posting locations, sent upstream as the
// geographic bounding box that is the endpoint's only usable $filter.
//
// A label matching several locations widens the box to enclose all of them, so
// results are a superset: postings that merely sit inside the box are returned
// rather than dropped, the same way the board's own radius search returns
// neighbouring towns.
// Workforce Now (workforcenow.adp.com) is out of scope (future adp_wfn).
type ADPMyJobsAdapter struct {
	hc          *http.Client
	careerBase  string
	listingBase string
	dumpCache   *DumpCache
}

// NewADPMyJobsAdapter builds an adapter using the shared HTTP client.
// dumpCache caches the location index used by Filters and location searches.
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
	slug, ok := matchCareersSlug(adpMyJobsCareersURLRE, u)
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
	skip := pageIndex * pageSize

	locationValues := append([]string(nil), p.Filters[adpLocationFilterKey]...)
	for key := range p.Filters {
		if key != adpLocationFilterKey {
			return nil, fmt.Errorf(
				"adp_myjobs: unsupported filter key %q; available: %s",
				key,
				adpLocationFilterKey,
			)
		}
	}
	if location := strings.TrimSpace(p.Location); location != "" {
		if len(locationValues) > 0 {
			return nil, fmt.Errorf("adp_myjobs: location is already set in filters; pass it only once")
		}
		locationValues = []string{location}
	}

	var geoBox *adp_myjobs.GeoBox
	if len(locationValues) > 0 {
		// Upstream takes one bounding box per request and cannot OR two, so
		// several distinct locations would have to collapse into the box
		// enclosing them all -- for far-apart values that is a corridor of
		// unrelated postings. Ask for one location instead of answering with it.
		if len(locationValues) > 1 {
			return nil, fmt.Errorf(
				"adp_myjobs: upstream filters one location per search, got %d (%s); search each separately",
				len(locationValues), strings.Join(locationValues, ", "),
			)
		}
		options, err := a.locationOptions(ctx, slug)
		if err != nil {
			return nil, err
		}
		box, err := resolveADPGeoBox(options, locationValues[0])
		if err != nil {
			return nil, err
		}
		geoBox = &box
	}

	// Keyword search uses upstream $search, location the geographic $filter, so
	// both constraints are applied server-side in the same request.
	search := strings.TrimSpace(p.Query)

	client := a.client()
	res, err := client.ListJobRequisitions(ctx, slug, adp_myjobs.ListParams{
		Search: search,
		GeoBox: geoBox,
		Skip:   skip,
		Top:    pageSize,
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
	options, err := a.locationOptions(ctx, slug)
	if err != nil {
		return nil, err
	}
	// Only locations with coordinates are listed: upstream filters location by
	// coordinates alone, so a label the board never geocoded is not a value
	// Search could honor. A board that geocoded nothing reports no location
	// dimension at all rather than values that always fail.
	labels := make([]string, 0, len(options))
	for _, option := range options {
		if len(option.points) > 0 {
			labels = append(labels, option.label)
		}
	}
	if len(labels) == 0 {
		return FilterSet{}, nil
	}
	return FilterSet{adpLocationFilterKey: labels}, nil
}

// adpLocationOption is one posting location from a board dump: the labels a
// caller can name it by, plus the coordinates that make it filterable upstream.
// points is empty for locations the board never geocoded.
type adpLocationOption struct {
	id      string
	label   string
	aliases []string
	points  []adpGeoPoint
}

type adpGeoPoint struct {
	lat, lon float64
}

func (a *ADPMyJobsAdapter) locationOptions(ctx context.Context, slug string) ([]adpLocationOption, error) {
	_, side, err := a.dumpCache.getOrLoadDump(ctx, a.Name(), slug, func(ctx context.Context) ([]dumpJob, any, error) {
		reqs, err := a.client().ListAllJobRequisitions(ctx, slug)
		if err != nil {
			return nil, nil, fmt.Errorf("adp_myjobs: load location filters for %q: %w", slug, err)
		}
		return nil, buildADPLocationOptions(reqs), nil
	})
	if err != nil {
		return nil, err
	}
	options, ok := side.([]adpLocationOption)
	if !ok {
		return nil, fmt.Errorf("adp_myjobs: location filter cache for %q has an invalid value", slug)
	}
	return options, nil
}

func buildADPLocationOptions(reqs []adp_myjobs.JobRequisition) []adpLocationOption {
	byID := make(map[string]adpLocationOption)
	for _, req := range reqs {
		for _, loc := range req.RequisitionLocations {
			id := strings.TrimSpace(loc.ItemID)
			if id == "" {
				continue
			}
			labels := adpLocationLabels(loc)
			if len(labels) == 0 {
				continue
			}
			option := byID[id]
			if option.id == "" {
				option = adpLocationOption{id: id, label: labels[0]}
			}
			for _, label := range labels {
				if !containsFold(option.aliases, label) {
					option.aliases = append(option.aliases, label)
				}
			}
			if loc.Address != nil {
				if lat, lon, ok := loc.Address.GeoCoordinate.Point(); ok && !containsPoint(option.points, lat, lon) {
					option.points = append(option.points, adpGeoPoint{lat: lat, lon: lon})
				}
			}
			byID[id] = option
		}
	}

	options := make([]adpLocationOption, 0, len(byID))
	for _, option := range byID {
		options = append(options, option)
	}
	sort.Slice(options, func(i, j int) bool {
		return strings.ToLower(options[i].label) < strings.ToLower(options[j].label)
	})
	return options
}

func adpLocationLabels(loc adp_myjobs.RequisitionLocation) []string {
	labels := make([]string, 0, 4)
	if loc.NameCode != nil {
		for _, label := range []string{
			loc.NameCode.LongName,
			loc.NameCode.ShortName,
			loc.NameCode.CodeValue,
		} {
			if strings.TrimSpace(label) != "" && !containsFold(labels, label) {
				labels = append(labels, strings.TrimSpace(label))
			}
		}
	}
	if loc.Address != nil {
		// Two spellings of the same address, because a caller naming a region
		// may use either: boards label states and countries by code
		// ("Peterborough, ON, CAN"), so without the long form "Ontario" and
		// "Canada" match nothing while "ON" and "CAN" work.
		for _, name := range []func(*adp_myjobs.CodeVal) string{adpCodeString, adpLongName} {
			parts := make([]string, 0, 3)
			if loc.Address.CityName != "" {
				parts = append(parts, loc.Address.CityName)
			}
			if s := name(loc.Address.CountrySubdivisionLevel1); s != "" {
				parts = append(parts, s)
			}
			if s := name(loc.Address.Country); s != "" {
				parts = append(parts, s)
			}
			if label := strings.Join(parts, ", "); label != "" && !containsFold(labels, label) {
				labels = append(labels, label)
			}
		}
	}
	return labels
}

// adpLongName prefers a code's spelled-out name, the opposite of adpCodeString.
func adpLongName(c *adp_myjobs.CodeVal) string {
	if c == nil {
		return ""
	}
	if c.LongName != "" {
		return c.LongName
	}
	if c.ShortName != "" {
		return c.ShortName
	}
	return c.CodeValue
}

func adpCodeString(c *adp_myjobs.CodeVal) string {
	if c == nil {
		return ""
	}
	if c.CodeValue != "" {
		return c.CodeValue
	}
	if c.ShortName != "" {
		return c.ShortName
	}
	return c.LongName
}

// adpLocationMatch ranks how well one catalog alias answers a caller's location
// value; only the best rank any option reaches is used.
//
// A plain substring test alone is too loose once matches decide a bounding box:
// on a nationwide board "OH" also hits "CO - Johnstown" and "Johnson City, TN",
// which would stretch the box from Colorado to Tennessee. Requiring the value to
// be a whole label token keeps "OH" to Ohio, while substring stays available for
// partial words like "Colum".
type adpLocationMatch int

const (
	adpNoMatch adpLocationMatch = iota
	adpSubstringMatch
	adpTokenMatch
	adpExactMatch
)

// resolveADPGeoBox matches value against the catalog's location labels and
// returns the padded bounding box enclosing every coordinate they carry.
//
// A value that legitimately names many locations ("California", "TX") widens the
// box to cover them all, which is how a region search is expressed here.
func resolveADPGeoBox(options []adpLocationOption, value string) (adp_myjobs.GeoBox, error) {
	needle := strings.ToLower(strings.TrimSpace(value))
	if needle == "" {
		return adp_myjobs.GeoBox{}, fmt.Errorf("adp_myjobs: empty location")
	}

	ranks := make([]adpLocationMatch, len(options))
	best := adpNoMatch
	for i, option := range options {
		for _, alias := range option.aliases {
			ranks[i] = max(ranks[i], adpMatchAlias(alias, needle))
		}
		best = max(best, ranks[i])
	}

	var (
		matched []string
		points  []adpGeoPoint
	)
	for i, option := range options {
		if ranks[i] != best || best == adpNoMatch {
			continue
		}
		matched = append(matched, option.label)
		points = append(points, option.points...)
	}

	switch {
	case len(matched) == 0:
		return adp_myjobs.GeoBox{}, fmt.Errorf(
			"adp_myjobs: no location matching %q; call get_filters_by_company first", value,
		)
	case len(points) == 0:
		// The board names these locations but never geocoded them, and
		// coordinates are the only location value upstream can filter on.
		sort.Strings(matched)
		return adp_myjobs.GeoBox{}, fmt.Errorf(
			"adp_myjobs: location %q matches %s, which this board publishes without coordinates; "+
				"upstream can only filter by location coordinates, so drop the location and use a keyword instead",
			value, strings.Join(matched, ", "),
		)
	}

	box := adp_myjobs.NewGeoBox(points[0].lon, points[0].lat, points[0].lon, points[0].lat)
	for _, p := range points[1:] {
		box.West = min(box.West, p.lon)
		box.East = max(box.East, p.lon)
		box.South = min(box.South, p.lat)
		box.North = max(box.North, p.lat)
	}
	return box.Pad(adpGeoPad), nil
}

// adpMatchAlias ranks alias against an already-lowercased needle.
func adpMatchAlias(alias, needle string) adpLocationMatch {
	alias = strings.ToLower(alias)
	switch {
	case alias == needle:
		return adpExactMatch
	case slices.Contains(adpAliasTokens(alias), needle):
		return adpTokenMatch
	case strings.Contains(alias, needle):
		return adpSubstringMatch
	}
	return adpNoMatch
}

// adpAliasTokens splits a label into its alphanumeric words, so "OH - Grove
// City" and "Grove City, OH, USA" both yield the token "oh".
func adpAliasTokens(alias string) []string {
	return strings.FieldsFunc(alias, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func containsFold(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}

func containsPoint(points []adpGeoPoint, lat, lon float64) bool {
	for _, p := range points {
		if p.lat == lat && p.lon == lon {
			return true
		}
	}
	return false
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
