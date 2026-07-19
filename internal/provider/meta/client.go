package meta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const graphqlPath = "/graphql"

// Persisted-query document IDs; see doc.go for how to re-derive them when
// Meta redeploys the site.
const (
	searchDocID    = "27506805582236862" // CareersJobSearchResultsDataQuery
	detailDocID    = "27371134039243725" // CandidatePortalJobDetailsViewQuery
	filtersDocID   = "25103492705924273" // CareersJobSearchFiltersV3Query
	locationsDocID = "24867916029505828" // CareersJobSearchLocationFilterV3Query
)

// lsdToken satisfies the endpoint's presence-only anti-CSRF check; the value
// itself is never validated for logged-out requests (see doc.go).
const lsdToken = "openings-mcp"

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// ErrJobNotFound marks a requisition ID with no active public posting.
var ErrJobNotFound = errors.New("meta: job not found")

type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a Client against baseURL (https://www.metacareers.com in
// production). When httpClient is nil, http.DefaultClient is used.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// SearchRequest mirrors the site's search_input filters. All filters are
// server-side; results always arrive in full (no pagination — see doc.go).
//
// Filter value lists are dynamic — [Client.SearchFilters] returns the
// current ones. In terms of that response:
//   - Teams takes SearchFilters.Teams values ("Software Engineering",
//     "AR/VR", ...).
//   - Divisions takes SearchFilters.Technologies values ("Facebook",
//     "Meta Quest", ...) — the UI labels this filter "Technology" but
//     submits it under the divisions key.
//   - Offices takes a [Location] DisplayName or ID; both match.
//   - Roles takes SearchFilters.Roles values ("Full time employment", ...).
//   - SubTeams values are not enumerated by any public filter query; they
//     appear on search results as Job.SubTeams.
//   - LeadershipLevels exists in search_input but the public filter UI
//     exposes no value list for it.
type SearchRequest struct {
	Q                string
	Teams            []string
	SubTeams         []string
	Offices          []string
	Divisions        []string
	Roles            []string
	LeadershipLevels []string
	IsLeadership     bool
	IsRemoteOnly     bool
	SortByNew        bool
}

// Job is one search result. ID is the requisition ID accepted by
// [Client.JobDetail].
type Job struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Locations []string `json:"locations"`
	Teams     []string `json:"teams"`
	SubTeams  []string `json:"sub_teams"`
}

// SearchResponse carries every job matching the request; FeaturedJobs is the
// site's small curated subset and overlaps AllJobs.
type SearchResponse struct {
	AllJobs      []Job
	FeaturedJobs []Job
}

// Compensation is one country's public pay range for a posting.
type Compensation struct {
	ErrorApologyNote string `json:"error_apology_note"`
	HasBonus         bool   `json:"has_bonus"`
	HasEquity        bool   `json:"has_equity"`
	Minimum          string `json:"compensation_amount_minimum"` // e.g. "$201,000/year"
	Maximum          string `json:"compensation_amount_maximum"`
	CountryCode      string `json:"country_code"`
}

// JobDetail is a full posting. Fields with the HTML suffix hold raw HTML
// fragments.
type JobDetail struct {
	ID                              string
	Title                           string
	Locations                       []string
	Departments                     []string // e.g. "Design & User Experience"; matches Job.Teams values
	InternalDepartments             []string // matches Job.SubTeams values
	DescriptionHTML                 string
	MinimumQualifications           []string
	PreferredQualifications         []string
	Responsibilities                []string
	PublicCompensation              []Compensation
	ShowPartialPublicCompDisclaimer bool
	BoilerplateIntroHTML            string
	CaliforniaDisclaimerHTML        string
	IntlDisclaimerHTML              string
	EqualOpportunityMessageHTML     string
	AccommodationsMessageHTML       string
	// OwnershipInformation has only ever been observed as null; kept raw so
	// a future non-null shape is not silently dropped.
	OwnershipInformation json.RawMessage
}

// Location is one entry of the site's office filter; either ID or
// DisplayName is accepted as a [SearchRequest] office value.
type Location struct {
	ID          string `json:"id"`
	DisplayName string `json:"location_display_name"`
	IsRemote    bool   `json:"is_remote"`
	State       string `json:"state"`
	Country     string `json:"country"`
}

// SearchFilters carries the site's current filter value lists; see
// [SearchRequest] for which field each list feeds.
type SearchFilters struct {
	Teams        []string
	Technologies []string
	Roles        []string
	Locations    []Location
}

// JobURL returns the public posting URL for a requisition ID.
func JobURL(id string) string {
	return "https://www.metacareers.com/jobs/" + id + "/"
}

// SearchJobs returns all jobs matching r; see [SearchRequest] for filter
// semantics.
func (c *Client) SearchJobs(ctx context.Context, r SearchRequest) (*SearchResponse, error) {
	variables := searchVariables(r)
	var data searchData
	if err := c.graphql(ctx, searchDocID, variables, &data); err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	if data.JobSearch == nil {
		return nil, errors.New("search jobs: response has no job_search_with_featured_jobs")
	}
	return &SearchResponse{
		AllJobs:      data.JobSearch.AllJobs,
		FeaturedJobs: data.JobSearch.FeaturedJobs,
	}, nil
}

// SearchFilters returns the site's current filter value lists, merging the
// two filter queries (teams/technologies/roles and locations) it serves
// them from.
func (c *Client) SearchFilters(ctx context.Context) (*SearchFilters, error) {
	var filters filtersData
	if err := c.graphql(ctx, filtersDocID, map[string]any{}, &filters); err != nil {
		return nil, fmt.Errorf("search filters: %w", err)
	}
	if filters.Filters == nil {
		return nil, errors.New("search filters: response has no job_search_filters")
	}
	var locations locationsData
	if err := c.graphql(ctx, locationsDocID, map[string]any{}, &locations); err != nil {
		return nil, fmt.Errorf("search filters: %w", err)
	}
	if locations.Filters == nil {
		return nil, errors.New("search filters: locations response has no job_search_filters")
	}
	teams := make([]string, 0, len(filters.Filters.Teams))
	for _, team := range filters.Filters.Teams {
		teams = append(teams, team.DisplayName)
	}
	return &SearchFilters{
		Teams:        teams,
		Technologies: filters.Filters.Technologies,
		Roles:        filters.Filters.Roles,
		Locations:    locations.Filters.Locations,
	}, nil
}

// JobDetail expects a [Job.ID] returned by [Client.SearchJobs]. It returns
// [ErrJobNotFound] for an unknown or expired requisition ID.
func (c *Client) JobDetail(ctx context.Context, jobID string) (*JobDetail, error) {
	if strings.TrimSpace(jobID) == "" {
		return nil, errors.New("empty job id")
	}
	variables := map[string]any{
		"renderLoggedInView": false,
		"requisitionID":      jobID,
		"viewasUserID":       nil,
	}
	var data detailData
	if err := c.graphql(ctx, detailDocID, variables, &data); err != nil {
		return nil, fmt.Errorf("job detail %q: %w", jobID, err)
	}
	if data.Description == nil {
		return nil, fmt.Errorf("job detail %q: %w", jobID, ErrJobNotFound)
	}
	return data.Description.toJobDetail(), nil
}

func searchVariables(r SearchRequest) map[string]any {
	return map[string]any{
		"isLoggedIn": false,
		"search_input": map[string]any{
			"q":                 orNil(r.Q),
			"divisions":         orEmpty(r.Divisions),
			"offices":           orEmpty(r.Offices),
			"roles":             orEmpty(r.Roles),
			"leadership_levels": orEmpty(r.LeadershipLevels),
			"saved_jobs":        []string{},
			"saved_searches":    []string{},
			"sub_teams":         orEmpty(r.SubTeams),
			"teams":             orEmpty(r.Teams),
			"is_leadership":     r.IsLeadership,
			"is_remote_only":    r.IsRemoteOnly,
			"sort_by_new":       r.SortByNew,
			"page":              1,
			"results_per_page":  nil,
		},
		"viewasUserID": nil,
	}
}

// orNil maps an empty query to JSON null, matching the website's default
// search_input.
func orNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// orEmpty keeps absent list filters as [] rather than null, matching the
// website's default search_input.
func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// graphql posts one persisted query and decodes the response's data field
// into out. See doc.go for which form fields and headers the endpoint
// requires.
func (c *Client) graphql(ctx context.Context, docID string, variables any, out any) error {
	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		return fmt.Errorf("marshal variables: %w", err)
	}
	form := url.Values{
		"lsd":       {lsdToken},
		"variables": {string(variablesJSON)},
		"doc_id":    {docID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+graphqlPath, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-FB-LSD", lsdToken)
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("Referer", c.baseURL+"/jobsearch/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	var envelope gqlEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("graphql: %s", envelope.Errors[0].Message)
	}
	if len(envelope.Data) == 0 {
		return errors.New("graphql: response has no data")
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("parse data: %w", err)
	}
	return nil
}
