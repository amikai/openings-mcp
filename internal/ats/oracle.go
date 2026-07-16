package ats

import (
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

	"github.com/amikai/openings-mcp/internal/provider/oracle"
)

// oracleCareersPathRE matches Candidate Experience paths and captures
// language + public site name. Fixed segments are case-insensitive; the
// site may sit under an optional path prefix.
//
// Examples (URL path):
//   - /hcmUI/CandidateExperience/en/sites/Mayo-US/job/386920
//   - /hcmUI/CandidateExperience/en-US/sites/Acme/jobs
//   - /hcmUI/CandidateExperience/en/sites/Acme/job/123
var oracleCareersPathRE = regexp.MustCompile(
	`(?i)(?:^|/)hcmUI/CandidateExperience/([^/]+)/sites/([^/]+)(?:/|$)`,
)

var _ Adapter = (*OracleAdapter)(nil)

// OracleAdapter serves Oracle Recruiting Cloud Candidate Experience sites.
// Curated companies use host/site_number roster slugs. Careers URLs outside
// the roster remain usable as self-describing slugs; the adapter discovers
// their internal site number from the public careers page before each call.
type OracleAdapter struct {
	hc           *http.Client
	apiBaseURL   func(oracle.Company) string
	discoverSite func(context.Context, string, *http.Client) (oracle.Site, error)
}

func NewOracleAdapter(hc *http.Client) *OracleAdapter {
	return &OracleAdapter{
		hc:           hc,
		apiBaseURL:   oracle.Company.APIBaseURL,
		discoverSite: oracle.DiscoverSite,
	}
}

func (a *OracleAdapter) Name() string { return "oracle" }

func (a *OracleAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(oracle.Companies))
	for _, c := range oracle.Companies {
		infos = append(infos, CompanyInfo{
			Slug: oracleCompanySlug(c),
			Name: c.Name,
		})
	}
	return infos
}

// ParseCareersURL recognizes Oracle Candidate Experience job-search and
// posting URLs. Curated sites fold back to their roster slug; uncurated sites
// get a canonical careers URL carrying the host, language, and public site.
func (a *OracleAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	host, _, site, canonical, ok := parseOracleCareersURL(u)
	if !ok {
		return "", false
	}
	if company, ok := oracleCompanyByCareersSite(host, site); ok {
		return oracleCompanySlug(company), true
	}
	return canonical, true
}

func (a *OracleAdapter) Search(
	ctx context.Context,
	slug string,
	p SearchParams,
) (*SearchResult, error) {
	_, client, err := a.client(ctx, slug)
	if err != nil {
		return nil, err
	}

	page := clampPage(p.Page)
	pageIndex := page - 1
	if pageIndex > math.MaxInt/pageSize {
		return nil, fmt.Errorf("oracle: page %d is too large; retry with a smaller page", page)
	}

	filters := map[oracle.Facet][]string{}
	location := strings.TrimSpace(p.Location)
	if location != "" || len(p.Filters) > 0 {
		facets, err := a.probeFacets(ctx, client, slug)
		if err != nil {
			return nil, err
		}
		filters, err = resolveOracleFilters(facets, location, p.Filters)
		if err != nil {
			return nil, err
		}
	}

	res, err := client.Search(ctx, oracle.SearchRequest{
		Keyword: strings.TrimSpace(p.Query),
		Limit:   pageSize,
		Offset:  pageIndex * pageSize,
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("oracle: search %q: %w", slug, err)
	}

	jobs := make([]JobSummary, 0, len(res.Jobs))
	for _, job := range res.Jobs {
		if job.ID == "" {
			continue
		}
		jobs = append(jobs, JobSummary{
			JobID:    job.ID,
			Title:    job.Title,
			Location: oracleLocation(job.PrimaryLocation, job.SecondaryLocations),
			PostedAt: oraclePostedAt(job.PostedAt),
			URL:      job.URL,
		})
	}
	return &SearchResult{
		Jobs:       jobs,
		TotalCount: res.Total,
		Page:       page,
		TotalPages: totalPages(res.Total),
	}, nil
}

func (a *OracleAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	_, client, err := a.client(ctx, slug)
	if err != nil {
		return nil, err
	}
	facets, err := a.probeFacets(ctx, client, slug)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]map[string]struct{}, len(facets))
	for facet, options := range facets {
		for _, option := range options {
			label := strings.TrimSpace(option.Name)
			if label == "" {
				continue
			}
			key := string(facet)
			if seen[key] == nil {
				seen[key] = make(map[string]struct{})
			}
			seen[key][label] = struct{}{}
		}
	}
	return toFilterSet(seen), nil
}

func (a *OracleAdapter) Detail(
	ctx context.Context,
	slug string,
	jobID string,
) (*JobDetail, error) {
	name, client, err := a.client(ctx, slug)
	if err != nil {
		return nil, err
	}
	job, err := client.Detail(ctx, jobID)
	if errors.Is(err, oracle.ErrJobNotFound) {
		return nil, fmt.Errorf(
			"oracle: job %q not found for company %q; pass a job_id exactly as returned by the job search",
			jobID,
			slug,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("oracle: fetch job %q for %q: %w", jobID, slug, err)
	}
	resolvedJobID := job.ID
	if resolvedJobID == "" {
		resolvedJobID = jobID
	}
	return &JobDetail{
		JobID:       resolvedJobID,
		Title:       job.Title,
		Company:     name,
		Location:    oracleLocation(job.PrimaryLocation, job.SecondaryLocations),
		PostedAt:    oraclePostedAt(job.PostedAt),
		URL:         job.URL,
		Description: oracleDescription(job),
	}, nil
}

func (a *OracleAdapter) client(
	ctx context.Context,
	slug string,
) (string, *oracle.SiteClient, error) {
	name, site, err := a.resolveSite(ctx, slug)
	if err != nil {
		return "", nil, err
	}
	client, err := oracle.NewSiteClient(site, a.hc)
	if err != nil {
		return "", nil, fmt.Errorf("oracle: create client for %q: %w", slug, err)
	}
	return name, client, nil
}

func (a *OracleAdapter) resolveSite(
	ctx context.Context,
	slug string,
) (string, oracle.Site, error) {
	if company, ok := oracle.CompaniesByKey[slug]; ok {
		return company.Name, a.siteForCompany(company), nil
	}

	u, ok := parseCareersInput(slug)
	if !ok {
		return "", oracle.Site{}, fmt.Errorf(
			"oracle: unknown company %q; pass a roster slug or an Oracle Candidate Experience careers URL",
			slug,
		)
	}
	host, _, publicSite, canonical, ok := parseOracleCareersURL(u)
	if !ok {
		return "", oracle.Site{}, fmt.Errorf(
			"oracle: invalid careers URL %q; pass an *.oraclecloud.com Candidate Experience URL",
			slug,
		)
	}
	if company, ok := oracleCompanyByCareersSite(host, publicSite); ok {
		return company.Name, a.siteForCompany(company), nil
	}

	site, err := a.discoverSite(ctx, canonical, a.hc)
	if err != nil {
		return "", oracle.Site{}, fmt.Errorf("oracle: discover careers site %q: %w", canonical, err)
	}
	return site.Site, site, nil
}

func (a *OracleAdapter) siteForCompany(company oracle.Company) oracle.Site {
	return oracle.Site{
		CareersURL: company.CareersURL(),
		APIBaseURL: a.apiBaseURL(company),
		Site:       company.Site,
		SiteNumber: company.SiteNumber,
		Language:   "en",
	}
}

func (a *OracleAdapter) probeFacets(
	ctx context.Context,
	client *oracle.SiteClient,
	slug string,
) (map[oracle.Facet][]oracle.FacetOption, error) {
	res, err := client.Search(ctx, oracle.SearchRequest{
		Limit:  1,
		Facets: oracle.AllFacets(),
	})
	if err != nil {
		return nil, fmt.Errorf("oracle: facets %q: %w", slug, err)
	}
	return res.Facets, nil
}

func resolveOracleFilters(
	facets map[oracle.Facet][]oracle.FacetOption,
	location string,
	filters FilterSet,
) (map[oracle.Facet][]string, error) {
	resolved := make(map[oracle.Facet][]string, len(filters)+1)
	valid := make(map[string]bool)
	for _, facet := range oracle.AllFacets() {
		valid[string(facet)] = true
	}

	for key, values := range filters {
		facet, err := oracle.ParseFacet(key)
		if err != nil {
			return nil, errUnknownFilterKey(key, valid)
		}
		ids, err := resolveOracleFacetValues(facet, facets[facet], values)
		if err != nil {
			return nil, err
		}
		resolved[facet] = ids
	}

	if location == "" {
		return resolved, nil
	}
	locationFacet := oracle.FacetLocation
	if strings.EqualFold(location, "remote") {
		locationFacet = oracle.FacetWorkplaceType
	}
	if _, exists := resolved[locationFacet]; exists {
		return nil, fmt.Errorf(
			"location %q uses the %q facet, which is also set in filters; pass the criteria only once",
			location,
			locationFacet,
		)
	}
	ids := matchOracleLocation(facets[locationFacet], location)
	if len(ids) == 0 {
		return nil, fmt.Errorf(
			"no Oracle %s matching %q; list the company's filters to see available values",
			locationFacet,
			location,
		)
	}
	resolved[locationFacet] = ids
	return resolved, nil
}

func resolveOracleFacetValues(
	facet oracle.Facet,
	options []oracle.FacetOption,
	values []string,
) ([]string, error) {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		matched := false
		for _, option := range options {
			if !strings.EqualFold(strings.TrimSpace(option.Name), value) {
				continue
			}
			id := strings.TrimSpace(option.ID)
			if id == "" {
				continue
			}
			matched = true
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		if !matched {
			available := make([]string, 0, len(options))
			for _, option := range options {
				label := strings.TrimSpace(option.Name)
				if label != "" && !slices.Contains(available, label) {
					available = append(available, label)
				}
			}
			slices.Sort(available)
			return nil, fmt.Errorf(
				"filter value %q not found for %q; available: %s",
				value,
				facet,
				strings.Join(available, ", "),
			)
		}
	}
	return ids, nil
}

func matchOracleLocation(options []oracle.FacetOption, location string) []string {
	needle := strings.ToLower(strings.TrimSpace(location))
	ids := make([]string, 0)
	for _, option := range options {
		label := strings.ToLower(strings.TrimSpace(option.Name))
		id := strings.TrimSpace(option.ID)
		if id == "" {
			continue
		}
		matches := strings.Contains(label, needle)
		if needle == "remote" {
			matches = matches || strings.EqualFold(id, "ORA_REMOTE")
		}
		if matches && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func oracleCompanySlug(company oracle.Company) string {
	return strings.ToLower(company.Host) + "/" + company.SiteNumber
}

func oracleCompanyByCareersSite(host, site string) (oracle.Company, bool) {
	for _, company := range oracle.Companies {
		if strings.EqualFold(company.Host, host) && strings.EqualFold(company.Site, site) {
			return company, true
		}
	}
	return oracle.Company{}, false
}

func parseOracleCareersURL(
	u *url.URL,
) (host, language, site, canonical string, ok bool) {
	host = strings.ToLower(u.Hostname())
	isFusionHost := strings.HasSuffix(host, ".oraclecloud.com") &&
		strings.Contains(host, ".fa.")
	if !isFusionHost || u.Port() != "" || u.User != nil {
		return "", "", "", "", false
	}

	m := oracleCareersPathRE.FindStringSubmatch(u.Path)
	if m == nil {
		return "", "", "", "", false
	}
	language = strings.TrimSpace(m[1])
	site = strings.TrimSpace(m[2])
	if language == "" || site == "" {
		return "", "", "", "", false
	}
	canonicalURL := url.URL{
		Scheme: "https",
		Host:   host,
		Path: fmt.Sprintf(
			"/hcmUI/CandidateExperience/%s/sites/%s/jobs",
			language,
			site,
		),
	}
	return host, language, site, canonicalURL.String(), true
}

func oracleLocation(primary string, secondary []string) string {
	locations := make([]string, 0, len(secondary)+1)
	locations = appendDistinct(locations, primary)
	for _, location := range secondary {
		locations = appendDistinct(locations, location)
	}
	return strings.Join(locations, "; ")
}

func oraclePostedAt(postedAt time.Time) string {
	if postedAt.IsZero() {
		return ""
	}
	return isoDate(postedAt)
}

func oracleDescription(job *oracle.Job) string {
	parts := make([]string, 0, 4)
	parts = appendOracleDescription(parts, "", job.DescriptionHTML)
	parts = appendOracleDescription(parts, "Responsibilities", job.ResponsibilitiesHTML)
	parts = appendOracleDescription(parts, "Qualifications", job.QualificationsHTML)
	parts = appendOracleDescription(parts, "About the organization", job.CorporateDescriptionHTML)
	return strings.Join(parts, "\n\n")
}

func appendOracleDescription(parts []string, heading, content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return parts
	}
	text, err := html2text.FromString(content, html2text.Options{})
	if err == nil {
		content = strings.TrimSpace(text)
	}
	if heading != "" {
		content = heading + "\n" + content
	}
	return append(parts, content)
}
