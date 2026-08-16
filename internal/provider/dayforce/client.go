package dayforce

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"sync"
)

// userAgent is sent on every request, including the SSR fetch in siteinfo.go
// that ogen doesn't cover. Mirrored verbatim into every testdata/*.hurl req
// so live replay matches this client.
const _userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// ErrCSRFRequired marks a search rejected because the CSRF cookie and
// X-CSRF-TOKEN header did not both reach the server — a missing token, a
// missing cookie, or a token that no longer matches the cookie.
var ErrCSRFRequired = errors.New("dayforce: csrf cookie and header required")

// ErrBoardNotFound marks an unknown clientNamespace, or a jobBoardCode that
// does not exist for an otherwise-known tenant.
var ErrBoardNotFound = errors.New("dayforce: tenant or job board not found")

// ErrJobNotFound marks a posting id that does not exist, or that belongs to
// a different tenant than the one queried.
var ErrJobNotFound = errors.New("dayforce: job posting not found")

// BoardClient composes the generated OAS [Client] with Dayforce's next-auth
// CSRF pre-flight.
type BoardClient struct {
	api     *Client
	http    *http.Client // shared with api; used directly by SiteInfo's SSR fetch, which ogen doesn't cover
	baseURL string

	tokenMu sync.Mutex
	token   string
}

// NewBoardClient creates a Dayforce candidate portal client. The supplied
// HTTP client is cloned before a private cookie jar and the userAgent
// transport are attached, so other providers can safely share its base
// transport and timeout without sharing Dayforce session state.
func NewBoardClient(baseURL string, httpClient *http.Client) (*BoardClient, error) {
	// Shallow-copy the client so replacing Transport and Jar below does not
	// mutate the caller's client; the underlying transport remains shared.
	var sessionHTTPClient http.Client
	if httpClient != nil {
		sessionHTTPClient = *httpClient
	}

	base := sessionHTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	sessionHTTPClient.Transport = &userAgentTransport{base: base}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create dayforce cookie jar: %w", err)
	}
	sessionHTTPClient.Jar = jar

	generated, err := NewClient(baseURL, WithClient(&sessionHTTPClient))
	if err != nil {
		return nil, fmt.Errorf("create dayforce api client: %w", err)
	}
	return &BoardClient{api: generated, http: &sessionHTTPClient, baseURL: baseURL}, nil
}

// userAgentTransport sets userAgent on every outgoing request, so live
// traffic matches the browser session the fixtures were captured from.
type userAgentTransport struct {
	base http.RoundTripper
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", _userAgent)
	return t.base.RoundTrip(req)
}

// csrfToken returns the cached CSRF token, fetching and caching one on
// first use. force bypasses the cache to fetch a fresh token, e.g. after
// the cached one is rejected.
func (c *BoardClient) csrfToken(ctx context.Context, force bool) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token != "" && !force {
		return c.token, nil
	}

	resp, err := c.api.GetCsrf(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch dayforce csrf token: %w", err)
	}
	c.token = resp.CsrfToken
	return c.token, nil
}

// Search returns one 25-row page of job postings for request.ClientNamespace,
// starting at request.PaginationStart (an item offset, not a page number).
// The tenant slug is taken from request.ClientNamespace and repeated as the
// URL path parameter, matching what the live API expects in both places.
func (c *BoardClient) Search(ctx context.Context, request SearchRequest) (*SearchResponse, error) {
	token, err := c.csrfToken(ctx, false)
	if err != nil {
		return nil, err
	}

	response, err := c.api.SearchJobPostings(ctx, &request, SearchJobPostingsParams{
		ClientNamespace: request.ClientNamespace,
		XCSRFTOKEN:      token,
	})
	if err != nil {
		return nil, fmt.Errorf("search dayforce jobs: %w", err)
	}

	switch response := response.(type) {
	case *SearchResponse:
		return response, nil
	case *SearchJobPostingsForbidden:
		// Refresh once and retry; a second 403 remains a CSRF error.
		token, refreshErr := c.csrfToken(ctx, true)
		if refreshErr != nil {
			return nil, fmt.Errorf("%w: refresh token failed: %w", ErrCSRFRequired, refreshErr)
		}
		retryResponse, retryErr := c.api.SearchJobPostings(ctx, &request, SearchJobPostingsParams{
			ClientNamespace: request.ClientNamespace,
			XCSRFTOKEN:      token,
		})
		if retryErr != nil {
			return nil, fmt.Errorf("search dayforce jobs: %w", retryErr)
		}
		switch response := retryResponse.(type) {
		case *SearchJobPostingsForbidden:
			return nil, ErrCSRFRequired
		case *SearchResponse:
			return response, nil
		case *ProblemDetails:
			return nil, fmt.Errorf("%w: %s", ErrBoardNotFound, request.ClientNamespace)
		default:
			return nil, fmt.Errorf("search dayforce jobs: unexpected response type %T", response)
		}
	case *ProblemDetails:
		return nil, fmt.Errorf("%w: %s", ErrBoardNotFound, request.ClientNamespace)
	default:
		return nil, fmt.Errorf("search dayforce jobs: unexpected response type %T", response)
	}
}

// Job returns the full posting for postingID. ns is the tenant slug
// (repeated for both path positions the live API requires), and jobBoardID
// comes from a search row's JobBoardId or from [BoardClient.SiteInfo].
func (c *BoardClient) Job(ctx context.Context, ns, culture string, jobBoardID, postingID int) (*JobPostingDetail, error) {
	response, err := c.api.GetJobPosting(ctx, GetJobPostingParams{
		ClientNamespace:  ns,
		PostingNamespace: ns,
		CultureCode:      culture,
		JobBoardId:       jobBoardID,
		PostingId:        postingID,
	})
	if err != nil {
		return nil, fmt.Errorf("get dayforce job posting: %w", err)
	}

	switch response := response.(type) {
	case *JobPostingDetail:
		return response, nil
	case *ProblemDetails:
		return nil, fmt.Errorf("%w: %s/%d", ErrJobNotFound, ns, postingID)
	default:
		return nil, fmt.Errorf("get dayforce job posting: unexpected response type %T", response)
	}
}

// Departments returns a board's department filter dimension, keyed by
// attributeId. ns is repeated for both path positions, as in [BoardClient.Job].
func (c *BoardClient) Departments(ctx context.Context, ns string, jobBoardID int, culture string) (*PostingAttributeList, error) {
	response, err := c.api.ListDepartments(ctx, ListDepartmentsParams{
		ClientNamespace:    ns,
		AttributeNamespace: ns,
		JobBoardId:         jobBoardID,
		CultureCode:        culture,
	})
	if err != nil {
		return nil, fmt.Errorf("list dayforce departments: %w", err)
	}
	return response, nil
}

// PayClasses returns a board's pay-class filter dimension (e.g.
// "Full-time"), keyed by attributeId.
func (c *BoardClient) PayClasses(ctx context.Context, ns string, jobBoardID int, culture string) (*PostingAttributeList, error) {
	response, err := c.api.ListPayClasses(ctx, ListPayClassesParams{
		ClientNamespace:    ns,
		AttributeNamespace: ns,
		JobBoardId:         jobBoardID,
		CultureCode:        culture,
	})
	if err != nil {
		return nil, fmt.Errorf("list dayforce pay classes: %w", err)
	}
	return response, nil
}

// PayTypes returns a board's pay-type filter dimension (e.g. "Hourly",
// "Salary"), keyed by attributeId.
func (c *BoardClient) PayTypes(ctx context.Context, ns string, jobBoardID int, culture string) (*PostingAttributeList, error) {
	response, err := c.api.ListPayTypes(ctx, ListPayTypesParams{
		ClientNamespace:    ns,
		AttributeNamespace: ns,
		JobBoardId:         jobBoardID,
		CultureCode:        culture,
	})
	if err != nil {
		return nil, fmt.Errorf("list dayforce pay types: %w", err)
	}
	return response, nil
}
