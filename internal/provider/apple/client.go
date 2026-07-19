package apple

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
)

const (
	defaultLocale    = SearchJobsRequestLocaleEnUs
	longDateFormat   = DateFormatLongDateMMMMDYYYY
	mediumDateFormat = DateFormatMediumDateMMMDYYYY
)

// ErrJobNotFound marks an Apple position ID that has no active public posting.
var ErrJobNotFound = errors.New("apple: job not found")

// Sort is an Apple search-result ordering.
type Sort = SearchJobsRequestSort

const (
	// SortRelevance ranks results against the keyword query.
	SortRelevance Sort = SearchJobsRequestSortRelevance
	// SortNewest orders results by posting date, newest first.
	SortNewest Sort = SearchJobsRequestSortNewest
	// SortTeamAsc and the orderings below sort by team or location name.
	SortTeamAsc      Sort = SearchJobsRequestSortTeamAsc
	SortTeamDesc     Sort = SearchJobsRequestSortTeamDesc
	SortLocationAsc  Sort = SearchJobsRequestSortLocationAsc
	SortLocationDesc Sort = SearchJobsRequestSortLocationDesc
)

// TeamFilter selects one Apple team and sub-team pair by bare code, such as
// {SFTWR, AF} for "Software and Services: Apps and Frameworks". Codes are
// listed by [JobsClient.ListTeams].
type TeamFilter struct {
	TeamCode    string
	SubTeamCode string
}

// ParseTeamFilter splits a TEAM/SUBTEAM code pair such as HRDWR/CAM.
func ParseTeamFilter(value string) (TeamFilter, error) {
	teamCode, subTeamCode, ok := strings.Cut(value, "/")
	if !ok {
		return TeamFilter{}, fmt.Errorf("team filter must be TEAM/SUBTEAM codes such as HRDWR/CAM, got %q", value)
	}
	return TeamFilter{TeamCode: teamCode, SubTeamCode: subTeamCode}, nil
}

// SearchRequest contains the stable, caller-facing Apple search parameters.
// CountryCode is an ISO 3166-1 alpha-3 code such as TWN or USA; Keyword and
// CountryCode are required and every other filter narrows the result set.
type SearchRequest struct {
	Keyword     string
	CountryCode string
	Sort        Sort

	// Keywords are extra keyword filter chips, applied separately from the
	// ranked Keyword query.
	Keywords []string
	// Teams selects team and sub-team pairs; results match any listed pair.
	Teams []TeamFilter
	// Products are bare product codes such as IPHN, MAC, or ICLD.
	Products []string
	// Languages are case-sensitive language codes such as en_US or zh_HK,
	// listed by the public /api/v1/refData/languagesByInput endpoint.
	Languages []string

	Page int
	// HomeOffice keeps only remote-eligible postings when true.
	HomeOffice bool
}

// JobsClient composes the generated OAS [Client] with Apple's anonymous search
// session protocol. Search calls are serialized because each CSRF token is
// bound to the session cookie set by the immediately preceding token request.
//
// The wrapper is named JobsClient (not Client) so it can share this package
// with ogen-generated types, matching peers such as workday.TenantClient.
type JobsClient struct {
	api      *Client
	searchMu sync.Mutex
}

// NewJobsClient creates a session-aware Apple Jobs client. The supplied HTTP
// client is cloned before a private cookie jar is attached, so other providers
// can safely share its transport and timeout without sharing Apple session state.
func NewJobsClient(baseURL string, httpClient *http.Client) (*JobsClient, error) {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	sessionHTTPClient := *httpClient
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create apple cookie jar: %w", err)
	}
	sessionHTTPClient.Jar = jar

	generated, err := NewClient(baseURL, WithClient(&sessionHTTPClient))
	if err != nil {
		return nil, fmt.Errorf("create apple api client: %w", err)
	}
	return &JobsClient{api: generated}, nil
}

// SearchJobs initializes an anonymous session and returns one 20-item page of
// Apple job summaries. Zero Page and Sort values default to page 1 and
// relevance; Keyword and CountryCode are required.
func (c *JobsClient) SearchJobs(ctx context.Context, request SearchRequest) (*SearchResponse, error) {
	apiRequest, err := searchAPIRequest(request)
	if err != nil {
		return nil, err
	}

	c.searchMu.Lock()
	defer c.searchMu.Unlock()

	session, err := c.api.InitSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize apple search session: %w", err)
	}
	token, ok := session.XAppleCsrfToken.Get()
	if !ok || token == "" {
		return nil, errors.New("initialize apple search session: missing csrf token")
	}

	response, err := c.api.PostSearchJobs(ctx, apiRequest, PostSearchJobsParams{
		XAppleCsrfToken: token,
	})
	if err != nil {
		return nil, fmt.Errorf("search apple jobs: %w", err)
	}

	switch response := response.(type) {
	case *SearchResponse:
		return response, nil
	case *ErrorResponse:
		return nil, fmt.Errorf("search apple jobs: upstream rejected session: %s", response.Error)
	default:
		return nil, fmt.Errorf("search apple jobs: unexpected response type %T", response)
	}
}

// ListTeams returns Apple's team and sub-team filter taxonomy. The endpoint
// is anonymous reference data and needs no search session.
func (c *JobsClient) ListTeams(ctx context.Context) (*TeamsResponse, error) {
	response, err := c.api.ListTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("list apple teams: %w", err)
	}
	return response, nil
}

// JobDetail returns the complete public posting for a numeric Apple position
// ID returned by SearchJobs.
func (c *JobsClient) JobDetail(ctx context.Context, jobID string) (*JobDetailResponse, error) {
	if !isASCIIInteger(jobID) {
		return nil, fmt.Errorf("job id must contain only digits, got %q", jobID)
	}

	response, err := c.api.GetJobDetail(ctx, GetJobDetailParams{
		JobId:  jobID,
		Locale: GetJobDetailLocaleEnUs,
	})
	if err != nil {
		return nil, fmt.Errorf("get apple job detail: %w", err)
	}

	switch response := response.(type) {
	case *JobDetailResponse:
		return response, nil
	case *ErrorResponse:
		return nil, fmt.Errorf("%w: %s", ErrJobNotFound, jobID)
	default:
		return nil, fmt.Errorf("get apple job detail: unexpected response type %T", response)
	}
}

// JobURL returns the public Apple Jobs page for a search or detail result.
func JobURL(positionID, titleSlug string) string {
	return fmt.Sprintf(
		"https://jobs.apple.com/en-us/details/%s/%s",
		url.PathEscape(positionID),
		url.PathEscape(titleSlug),
	)
}

func searchAPIRequest(request SearchRequest) (*SearchJobsRequest, error) {
	keyword := strings.TrimSpace(request.Keyword)
	if keyword == "" {
		return nil, errors.New("keyword is required")
	}

	locationID, err := countryLocationID(request.CountryCode)
	if err != nil {
		return nil, err
	}

	page := request.Page
	if page == 0 {
		page = 1
	}
	if page < 1 {
		return nil, fmt.Errorf("page must be >= 1, got %d", page)
	}

	sort := request.Sort
	if sort == "" {
		sort = SortRelevance
	}
	if validateErr := sort.Validate(); validateErr != nil {
		return nil, fmt.Errorf("invalid sort %q: %w", sort, validateErr)
	}

	filters, err := searchFilters(request, locationID)
	if err != nil {
		return nil, err
	}

	return &SearchJobsRequest{
		Query:   keyword,
		Filters: filters,
		Page:    page,
		Locale:  defaultLocale,
		Sort:    sort,
		Format: DateFormat{
			LongDate:   longDateFormat,
			MediumDate: mediumDateFormat,
		},
	}, nil
}

func searchFilters(request SearchRequest, locationID string) (SearchFilters, error) {
	filters := SearchFilters{
		Locations: []string{locationID},
	}
	if request.HomeOffice {
		filters.HomeOffice = NewOptBool(true)
	}
	for _, chip := range request.Keywords {
		chip = strings.TrimSpace(chip)
		if chip == "" {
			return SearchFilters{}, errors.New("keyword filters must not be blank")
		}
		filters.Keywords = append(filters.Keywords, chip)
	}
	teams, err := teamSearchFilters(request.Teams)
	if err != nil {
		return SearchFilters{}, err
	}
	filters.Teams = teams
	for _, product := range request.Products {
		productCode, err := filterCode("product", product)
		if err != nil {
			return SearchFilters{}, err
		}
		filters.Products = append(filters.Products, "productsAndServices-"+productCode)
	}
	for _, language := range request.Languages {
		languageCode, err := languageFilterCode(language)
		if err != nil {
			return SearchFilters{}, err
		}
		filters.Languages = append(filters.Languages, "language-"+languageCode)
	}
	return filters, nil
}

func teamSearchFilters(teams []TeamFilter) ([]SearchTeamFilter, error) {
	if len(teams) == 0 {
		return nil, nil
	}
	filters := make([]SearchTeamFilter, 0, len(teams))
	for _, team := range teams {
		teamCode, err := filterCode("team", team.TeamCode)
		if err != nil {
			return nil, err
		}
		subTeamCode, err := filterCode("sub-team", team.SubTeamCode)
		if err != nil {
			return nil, err
		}
		filters = append(filters, SearchTeamFilter{
			Team:    "teamsAndSubTeams-" + teamCode,
			SubTeam: "subTeam-" + subTeamCode,
		})
	}
	return filters, nil
}

// filterCode validates a bare Apple filter code such as SFTWR or IPHN and
// returns its canonical uppercase form.
func filterCode(kind, code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return "", fmt.Errorf("%s code must not be blank", kind)
	}
	for _, char := range code {
		if (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return "", fmt.Errorf("%s code must contain only ascii letters and digits, got %q", kind, code)
		}
	}
	return code, nil
}

// languageFilterCode validates a case-sensitive language code such as en_US.
func languageFilterCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", errors.New("language code must not be blank")
	}
	for _, char := range code {
		if (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') && char != '_' {
			return "", fmt.Errorf("language code must contain only ascii letters and underscores, got %q", code)
		}
	}
	return code, nil
}

func countryLocationID(countryCode string) (string, error) {
	countryCode = strings.ToUpper(strings.TrimSpace(countryCode))
	if len(countryCode) != 3 {
		return "", fmt.Errorf("country code must be three ascii letters, got %q", countryCode)
	}
	for _, char := range countryCode {
		if char < 'A' || char > 'Z' {
			return "", fmt.Errorf("country code must be three ascii letters, got %q", countryCode)
		}
	}
	return "postLocation-" + countryCode, nil
}

func isASCIIInteger(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
