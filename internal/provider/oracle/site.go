package oracle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

const discoveryUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// ErrJobNotFound indicates that Oracle returned an empty detail collection.
var ErrJobNotFound = errors.New("oracle: job not found")

// Site identifies one public Oracle Recruiting Cloud Candidate Experience
// site. Site is the public URL alias, while SiteNumber is the internal finder
// value read from the career page.
type Site struct {
	CareersURL string `json:"careersUrl"`
	APIBaseURL string `json:"apiBaseUrl"`
	Site       string `json:"site"`
	SiteNumber string `json:"siteNumber"`
	Language   string `json:"language"`
}

// JobURL returns the human-facing URL for a public requisition.
func (s Site) JobURL(id string) string {
	base := strings.TrimSuffix(s.CareersURL, "/jobs")
	return base + "/job/" + url.PathEscape(id)
}

// DiscoverSite fetches a Candidate Experience page and reads its API origin,
// public site alias, internal site number, and language from the base element.
// Older Oracle themes omit data-apibaseurl and data-sitenumber; those fall back
// to the page origin and public site alias.
func DiscoverSite(ctx context.Context, rawURL string, httpClient *http.Client) (Site, error) {
	pageURL, err := url.Parse(rawURL)
	if err != nil {
		return Site{}, fmt.Errorf("parse careers url: %w", err)
	}
	if pageURL.Scheme != "http" && pageURL.Scheme != "https" {
		return Site{}, fmt.Errorf("careers url must use http or https")
	}
	if pageURL.Hostname() == "" {
		return Site{}, errors.New("careers url must include a host")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
	if err != nil {
		return Site{}, fmt.Errorf("create careers request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", discoveryUserAgent)

	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return Site{}, fmt.Errorf("fetch careers page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Site{}, fmt.Errorf("fetch careers page: http %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return Site{}, fmt.Errorf("parse careers page: %w", err)
	}
	return parseSiteDocument(doc, resp.Request.URL)
}

func parseSiteDocument(doc *goquery.Document, pageURL *url.URL) (Site, error) {
	base := doc.Find("base").First()
	if base.Length() == 0 {
		return Site{}, errors.New("oracle: careers page has no base element")
	}

	baseURL := pageURL
	if href, ok := base.Attr("href"); ok && strings.TrimSpace(href) != "" {
		ref, err := url.Parse(href)
		if err != nil {
			return Site{}, fmt.Errorf("oracle: parse base href: %w", err)
		}
		baseURL = pageURL.ResolveReference(ref)
	}

	language, siteAlias, ok := candidateExperiencePath(baseURL.Path)
	if !ok {
		language, siteAlias, ok = candidateExperiencePath(pageURL.Path)
	}
	if !ok {
		return Site{}, fmt.Errorf("oracle: unrecognized candidate experience path %q", pageURL.Path)
	}

	siteNumber := strings.TrimSpace(base.AttrOr("data-sitenumber", ""))
	if siteNumber == "" {
		siteNumber = siteAlias
	}
	if err := validateFinderAtom("site number", siteNumber); err != nil {
		return Site{}, err
	}

	apiBaseURL := originURL(pageURL)
	if rawAPIBase := strings.TrimSpace(base.AttrOr("data-apibaseurl", "")); rawAPIBase != "" {
		apiURL, err := url.Parse(rawAPIBase)
		if err != nil {
			return Site{}, fmt.Errorf("oracle: parse api base url: %w", err)
		}
		if apiURL.Scheme != "http" && apiURL.Scheme != "https" {
			return Site{}, errors.New("oracle: api base url must use http or https")
		}
		if apiURL.Hostname() == "" {
			return Site{}, errors.New("oracle: api base url must include a host")
		}
		apiBaseURL = strings.TrimRight(apiURL.String(), "/")
	}

	careersBase := *baseURL
	careersBase.RawQuery = ""
	careersBase.Fragment = ""
	careersBase.Path = strings.TrimRight(careersBase.Path, "/") + "/jobs"
	careersBase.RawPath = ""

	return Site{
		CareersURL: careersBase.String(),
		APIBaseURL: apiBaseURL,
		Site:       siteAlias,
		SiteNumber: siteNumber,
		Language:   language,
	}, nil
}

func candidateExperiencePath(rawPath string) (language, site string, ok bool) {
	segments := strings.Split(strings.Trim(rawPath, "/"), "/")
	for i := 0; i+4 < len(segments); i++ {
		if !strings.EqualFold(segments[i], "hcmUI") ||
			!strings.EqualFold(segments[i+1], "CandidateExperience") ||
			!strings.EqualFold(segments[i+3], "sites") {
			continue
		}
		if segments[i+2] == "" || segments[i+4] == "" {
			return "", "", false
		}
		return segments[i+2], segments[i+4], true
	}
	return "", "", false
}

func originURL(u *url.URL) string {
	return (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
}

// Facet names one standard Oracle Candidate Experience facet.
type Facet string

const (
	FacetTitle         Facet = "title"
	FacetLocation      Facet = "location"
	FacetCategory      Facet = "category"
	FacetPostingDate   Facet = "posting-date"
	FacetWorkLocation  Facet = "work-location"
	FacetOrganization  Facet = "organization"
	FacetWorkplaceType Facet = "workplace-type"
)

type facetDefinition struct {
	listToken string
	selected  string
}

var facetDefinitions = map[Facet]facetDefinition{
	FacetTitle:         {listToken: "TITLES", selected: "selectedTitlesFacet"},
	FacetLocation:      {listToken: "LOCATIONS", selected: "selectedLocationsFacet"},
	FacetCategory:      {listToken: "CATEGORIES", selected: "selectedCategoriesFacet"},
	FacetPostingDate:   {listToken: "POSTING_DATES", selected: "selectedPostingDatesFacet"},
	FacetWorkLocation:  {listToken: "WORK_LOCATIONS", selected: "selectedWorkLocationsFacet"},
	FacetOrganization:  {listToken: "ORGANIZATIONS", selected: "selectedOrganizationsFacet"},
	FacetWorkplaceType: {listToken: "WORKPLACE_TYPES", selected: "selectedWorkplaceTypesFacet"},
}

var allFacets = []Facet{
	FacetTitle,
	FacetLocation,
	FacetCategory,
	FacetWorkplaceType,
	FacetPostingDate,
	FacetWorkLocation,
	FacetOrganization,
}

// AllFacets returns the standard facets in stable Oracle request order.
func AllFacets() []Facet {
	return append([]Facet(nil), allFacets...)
}

// ParseFacet parses a CLI/API facet name.
func ParseFacet(value string) (Facet, error) {
	facet := Facet(strings.ToLower(strings.TrimSpace(value)))
	if _, ok := facetDefinitions[facet]; !ok {
		return "", fmt.Errorf("unknown facet %q", value)
	}
	return facet, nil
}

// SearchRequest describes one server-side requisition search.
type SearchRequest struct {
	Keyword string
	Limit   int
	Offset  int
	Facets  []Facet
	Filters map[Facet][]string
}

// SearchResult is one page of public requisitions and optional facets.
type SearchResult struct {
	Total  int                     `json:"total"`
	Limit  int                     `json:"limit"`
	Offset int                     `json:"offset"`
	Jobs   []JobSummary            `json:"jobs"`
	Facets map[Facet][]FacetOption `json:"facets,omitempty"`
}

// JobSummary contains the compact fields returned by Oracle search.
type JobSummary struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	PostedAt           time.Time `json:"postedAt,omitzero"`
	PrimaryLocation    string    `json:"primaryLocation,omitempty"`
	SecondaryLocations []string  `json:"secondaryLocations,omitempty"`
	WorkplaceType      string    `json:"workplaceType,omitempty"`
	WorkplaceTypeCode  string    `json:"workplaceTypeCode,omitempty"`
	URL                string    `json:"url"`
}

// FacetOption is one filter value and its live result count.
type FacetOption struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Job is one public requisition detail record.
type Job struct {
	ID                       string    `json:"id"`
	Title                    string    `json:"title"`
	PostedAt                 time.Time `json:"postedAt,omitzero"`
	PrimaryLocation          string    `json:"primaryLocation,omitempty"`
	SecondaryLocations       []string  `json:"secondaryLocations,omitempty"`
	WorkplaceType            string    `json:"workplaceType,omitempty"`
	DescriptionHTML          string    `json:"descriptionHtml,omitempty"`
	CorporateDescriptionHTML string    `json:"corporateDescriptionHtml,omitempty"`
	ResponsibilitiesHTML     string    `json:"responsibilitiesHtml,omitempty"`
	QualificationsHTML       string    `json:"qualificationsHtml,omitempty"`
	URL                      string    `json:"url"`
}

// SiteClient binds the generated Oracle client to one discovered career site.
type SiteClient struct {
	site Site
	api  *Client
}

type acceptJSONTransport struct {
	base http.RoundTripper
}

func (t acceptJSONTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	cloned.Header.Set("Accept", "application/json")
	return t.base.RoundTrip(cloned)
}

// NewSiteClient constructs a client for a previously discovered site.
func NewSiteClient(site Site, httpClient *http.Client) (*SiteClient, error) {
	if site.APIBaseURL == "" {
		return nil, errors.New("oracle: api base url is required")
	}
	if err := validateFinderAtom("site number", site.SiteNumber); err != nil {
		return nil, err
	}
	opts := []ClientOption{WithClient(withJSONAccept(httpClient))}
	api, err := NewClient(site.APIBaseURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("oracle: create api client: %w", err)
	}
	return &SiteClient{site: site, api: api}, nil
}

func withJSONAccept(httpClient *http.Client) *http.Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	cloned := *httpClient
	transport := cloned.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	cloned.Transport = acceptJSONTransport{base: transport}
	return &cloned
}

// DiscoverClient discovers a careers URL and returns a bound API client.
func DiscoverClient(ctx context.Context, rawURL string, httpClient *http.Client) (*SiteClient, error) {
	site, err := DiscoverSite(ctx, rawURL, httpClient)
	if err != nil {
		return nil, err
	}
	return NewSiteClient(site, httpClient)
}

// Site returns the discovered site metadata.
func (c *SiteClient) Site() Site {
	return c.site
}

const siteSearchFields = "TotalJobsCount,Limit,Offset;" +
	"requisitionList:Id,Title,PostedDate,PrimaryLocation,WorkplaceType,WorkplaceTypeCode;" +
	"requisitionList.secondaryLocations:Name;" +
	"titlesFacet:Id,Name,TotalCount;locationsFacet:Id,Name,TotalCount;" +
	"categoriesFacet:Id,Name,TotalCount;postingDatesFacet:Id,Name,TotalCount;" +
	"workLocationsFacet:Id,Name,TotalCount;organizationsFacet:Id,Name,TotalCount;" +
	"workplaceTypesFacet:Id,Name,TotalCount"

const siteDetailFields = "Id,Title,ExternalPostedStartDate,PrimaryLocation,WorkplaceType," +
	"ExternalDescriptionStr,CorporateDescriptionStr,ExternalResponsibilitiesStr," +
	"ExternalQualificationsStr;secondaryLocations:Name"

// Search runs one server-side search page.
func (c *SiteClient) Search(ctx context.Context, request SearchRequest) (*SearchResult, error) {
	if request.Limit < 1 || request.Limit > 100 {
		return nil, fmt.Errorf("oracle: limit must be between 1 and 100, got %d", request.Limit)
	}
	if request.Offset < 0 {
		return nil, fmt.Errorf("oracle: offset must be >= 0, got %d", request.Offset)
	}

	finder, err := c.searchFinder(request)
	if err != nil {
		return nil, err
	}
	language := languageOrDefault(c.site.Language)
	response, err := c.api.SearchJobs(ctx, SearchJobsParams{
		OnlyData:       SearchJobsOnlyDataTrue,
		Fields:         NewOptString(siteSearchFields),
		Finder:         finder,
		AcceptLanguage: NewOptString(language),
		OraIrcLanguage: NewOptString(language),
	})
	if err != nil {
		return nil, fmt.Errorf("oracle: search jobs: %w", err)
	}
	if len(response.Items) != 1 {
		return nil, fmt.Errorf("oracle: search returned %d state items", len(response.Items))
	}

	page := response.Items[0]
	result := &SearchResult{
		Total:  page.TotalJobsCount,
		Limit:  page.Limit,
		Offset: page.Offset,
		Jobs:   make([]JobSummary, 0, len(page.RequisitionList)),
		Facets: make(map[Facet][]FacetOption),
	}
	for _, item := range page.RequisitionList {
		result.Jobs = append(result.Jobs, JobSummary{
			ID:                 item.ID.Or(""),
			Title:              item.Title.Or(""),
			PostedAt:           item.PostedDate.Or(time.Time{}),
			PrimaryLocation:    item.PrimaryLocation.Or(""),
			SecondaryLocations: secondaryLocationNames(item.SecondaryLocations),
			WorkplaceType:      item.WorkplaceType.Or(""),
			WorkplaceTypeCode:  item.WorkplaceTypeCode.Or(""),
			URL:                c.site.JobURL(item.ID.Or("")),
		})
	}
	addStringFacets(result.Facets, FacetTitle, page.TitlesFacet)
	addIntegerFacets(result.Facets, FacetLocation, page.LocationsFacet)
	addIntegerFacets(result.Facets, FacetCategory, page.CategoriesFacet)
	addIntegerFacets(result.Facets, FacetPostingDate, page.PostingDatesFacet)
	addIntegerFacets(result.Facets, FacetWorkLocation, page.WorkLocationsFacet)
	addIntegerFacets(result.Facets, FacetOrganization, page.OrganizationsFacet)
	addStringFacets(result.Facets, FacetWorkplaceType, page.WorkplaceTypesFacet)
	if len(result.Facets) == 0 {
		result.Facets = nil
	}
	return result, nil
}

// Detail fetches one public requisition by ID.
func (c *SiteClient) Detail(ctx context.Context, id string) (*Job, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("oracle: job id is required")
	}
	language := languageOrDefault(c.site.Language)
	response, err := c.api.GetJobDetail(ctx, GetJobDetailParams{
		OnlyData:       GetJobDetailOnlyDataTrue,
		Fields:         NewOptString(siteDetailFields),
		Finder:         fmt.Sprintf(`ById;Id="%s",siteNumber=%s`, escapeFinderString(id), c.site.SiteNumber),
		AcceptLanguage: NewOptString(language),
		OraIrcLanguage: NewOptString(language),
	})
	if err != nil {
		return nil, fmt.Errorf("oracle: get job detail: %w", err)
	}
	if len(response.Items) == 0 {
		return nil, fmt.Errorf("oracle: job %q: %w", id, ErrJobNotFound)
	}
	if len(response.Items) != 1 {
		return nil, fmt.Errorf("oracle: job %q returned %d detail items", id, len(response.Items))
	}

	item := response.Items[0]
	return &Job{
		ID:                       item.ID.Or(""),
		Title:                    item.Title.Or(""),
		PostedAt:                 item.ExternalPostedStartDate.Or(time.Time{}),
		PrimaryLocation:          item.PrimaryLocation.Or(""),
		SecondaryLocations:       secondaryLocationNames(item.SecondaryLocations),
		WorkplaceType:            item.WorkplaceType.Or(""),
		DescriptionHTML:          item.ExternalDescriptionStr.Or(""),
		CorporateDescriptionHTML: item.CorporateDescriptionStr.Or(""),
		ResponsibilitiesHTML:     item.ExternalResponsibilitiesStr.Or(""),
		QualificationsHTML:       item.ExternalQualificationsStr.Or(""),
		URL:                      c.site.JobURL(id),
	}, nil
}

func (c *SiteClient) searchFinder(request SearchRequest) (string, error) {
	facetTokens := make([]string, 0, len(request.Facets))
	seenFacets := make(map[Facet]bool, len(request.Facets))
	for _, facet := range request.Facets {
		definition, ok := facetDefinitions[facet]
		if !ok {
			return "", fmt.Errorf("oracle: unknown facet %q", facet)
		}
		if seenFacets[facet] {
			continue
		}
		seenFacets[facet] = true
		facetTokens = append(facetTokens, definition.listToken)
	}
	if len(facetTokens) == 0 {
		facetTokens = append(facetTokens, "NONE")
	}

	parts := []string{
		"siteNumber=" + c.site.SiteNumber,
		"facetsList=" + strings.Join(facetTokens, ";"),
		"limit=" + strconv.Itoa(request.Limit),
		"offset=" + strconv.Itoa(request.Offset),
		"sortBy=POSTING_DATES_DESC",
	}
	if request.Keyword != "" {
		parts = append(parts, `keyword="`+escapeFinderString(request.Keyword)+`"`)
	}
	for _, facet := range allFacets {
		values := request.Filters[facet]
		if len(values) == 0 {
			continue
		}
		definition := facetDefinitions[facet]
		clean := make([]string, 0, len(values))
		for _, value := range values {
			value = strings.TrimSpace(value)
			if err := validateFacetValue(facet, value); err != nil {
				return "", err
			}
			clean = append(clean, value)
		}
		parts = append(parts, definition.selected+"="+strings.Join(clean, ";"))
	}
	for facet := range request.Filters {
		if _, ok := facetDefinitions[facet]; !ok {
			return "", fmt.Errorf("oracle: unknown facet filter %q", facet)
		}
	}
	return "findReqs;" + strings.Join(parts, ","), nil
}

func validateFinderAtom(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("oracle: %s is required", name)
	}
	if strings.ContainsAny(value, ",;\"") {
		return fmt.Errorf("oracle: %s %q contains finder syntax", name, value)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("oracle: %s contains a control character", name)
		}
	}
	return nil
}

func validateFacetValue(facet Facet, value string) error {
	if value == "" {
		return fmt.Errorf("oracle: facet %q has an empty value", facet)
	}
	if strings.ContainsAny(value, ",;\"") {
		return fmt.Errorf("oracle: facet %q value %q contains finder syntax", facet, value)
	}
	return nil
}

func escapeFinderString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func languageOrDefault(language string) string {
	if language == "" {
		return "en"
	}
	return language
}

func secondaryLocationNames(locations []SecondaryLocation) []string {
	names := make([]string, 0, len(locations))
	for _, location := range locations {
		if name := strings.TrimSpace(location.Name.Or("")); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func addStringFacets(target map[Facet][]FacetOption, facet Facet, values []StringFacet) {
	if len(values) == 0 {
		return
	}
	options := make([]FacetOption, 0, len(values))
	for _, value := range values {
		options = append(options, FacetOption{
			ID:    value.ID.Or(""),
			Name:  value.Name.Or(""),
			Count: value.TotalCount.Or(0),
		})
	}
	target[facet] = options
}

func addIntegerFacets(target map[Facet][]FacetOption, facet Facet, values []IntegerFacet) {
	if len(values) == 0 {
		return
	}
	options := make([]FacetOption, 0, len(values))
	for _, value := range values {
		id, ok := value.ID.Get()
		option := FacetOption{
			Name:  value.Name.Or(""),
			Count: value.TotalCount.Or(0),
		}
		if ok {
			option.ID = strconv.FormatInt(id, 10)
		}
		options = append(options, option)
	}
	target[facet] = options
}
