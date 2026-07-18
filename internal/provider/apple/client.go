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
)

// SearchRequest contains the stable, caller-facing Apple search parameters.
// CountryCode is an ISO 3166-1 alpha-3 code such as TWN or USA.
type SearchRequest struct {
	Keyword     string
	CountryCode string
	Sort        Sort
	Page        int
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
	if err := sort.Validate(); err != nil {
		return nil, fmt.Errorf("invalid sort %q: %w", sort, err)
	}

	return &SearchJobsRequest{
		Query: keyword,
		Filters: SearchFilters{
			Locations: []string{locationID},
		},
		Page:   page,
		Locale: defaultLocale,
		Sort:   sort,
		Format: DateFormat{
			LongDate:   longDateFormat,
			MediumDate: mediumDateFormat,
		},
	}, nil
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
