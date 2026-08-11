package ats

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

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

var adpLocationIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ADPMyJobsAdapter serves public ADP MyJobs career boards (myjobs.adp.com).
// Keyword search uses the upstream OData $search parameter with server-side
// pagination. Location labels are resolved from a board listing and sent as
// upstream OData $filter expressions.
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

	filter := ""
	if len(locationValues) > 0 {
		options, err := a.locationOptions(ctx, slug)
		if err != nil {
			return nil, err
		}
		ids, err := resolveADPLocationIDs(options, locationValues)
		if err != nil {
			return nil, err
		}
		filter = buildADPLocationFilter(ids)
	}

	// Keyword search uses upstream $search. Location uses the structured
	// requisitionLocations/itemID $filter so both constraints can be sent.
	search := strings.TrimSpace(p.Query)

	client := a.client()
	res, err := client.ListJobRequisitions(ctx, slug, adp_myjobs.ListParams{
		Search: search,
		Filter: filter,
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
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	return &SearchResult{
		Jobs:       jobs,
		TotalCount: total,
		Page:       page,
		TotalPages: totalPages,
	}, nil
}

func (a *ADPMyJobsAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	options, err := a.locationOptions(ctx, slug)
	if err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(options))
	for _, option := range options {
		labels = append(labels, option.label)
	}
	return FilterSet{adpLocationFilterKey: labels}, nil
}

type adpLocationOption struct {
	id      string
	label   string
	aliases []string
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
			if id == "" || !adpLocationIDRE.MatchString(id) {
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
		parts := make([]string, 0, 3)
		if loc.Address.CityName != "" {
			parts = append(parts, loc.Address.CityName)
		}
		if s := adpCodeString(loc.Address.CountrySubdivisionLevel1); s != "" {
			parts = append(parts, s)
		}
		if s := adpCodeString(loc.Address.CountryCode); s != "" {
			parts = append(parts, s)
		}
		if label := strings.Join(parts, ", "); label != "" && !containsFold(labels, label) {
			labels = append(labels, label)
		}
	}
	return labels
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

func resolveADPLocationIDs(options []adpLocationOption, values []string) ([]string, error) {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		needle := strings.ToLower(strings.TrimSpace(value))
		if needle == "" {
			continue
		}
		for _, option := range options {
			for _, alias := range option.aliases {
				if strings.Contains(strings.ToLower(alias), needle) {
					if !containsFold(ids, option.id) {
						ids = append(ids, option.id)
					}
					break
				}
			}
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("adp_myjobs: no location matching %q; call get_filters_by_company first", strings.Join(values, ", "))
	}
	sort.Strings(ids)
	return ids, nil
}

func buildADPLocationFilter(ids []string) string {
	clauses := make([]string, 0, len(ids))
	for _, id := range ids {
		clauses = append(clauses, "requisitionLocations/itemID eq "+id)
	}
	if len(clauses) <= 1 {
		return strings.Join(clauses, "")
	}
	return "(" + strings.Join(clauses, " or ") + ")"
}

func containsFold(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
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
