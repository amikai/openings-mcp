package synopsys

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

const defaultBaseURL = "https://careers.synopsys.com"

var defaultHeader = http.Header{
	"User-Agent": {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"},
	"Accept":     {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient uses [http.DefaultClient] when httpClient is nil.
func NewClient(httpClient *http.Client) *Client {
	return &Client{
		httpClient: cmp.Or(httpClient, http.DefaultClient),
		baseURL:    defaultBaseURL,
	}
}

func newRequest(ctx context.Context, method, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header = defaultHeader.Clone()
	return req, nil
}

// Jobs returns summaries whose [Job.City], [Job.Slug], and [Job.JobID]
// values identify postings for [Client.JobDetail].
func (c *Client) Jobs(ctx context.Context, p *JobsRequest) (*JobsResponse, error) {
	q := buildSearchQuery(p)
	req, err := newRequest(ctx, http.MethodGet, c.baseURL+"/search-jobs/results?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search jobs: HTTP %d", resp.StatusCode)
	}
	var raw struct {
		Results    string `json:"results"`
		HasJobs    bool   `json:"hasJobs"`
		HasContent bool   `json:"hasContent"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("search jobs: decode: %w", err)
	}
	result, err := parseSearchResults(raw.Results)
	if err != nil {
		return nil, fmt.Errorf("search jobs: parse: %w", err)
	}
	// hasJobs is upstream's own signal; zero parsed cards despite it means
	// the results markup changed, not an empty search.
	if raw.HasJobs && len(result.Jobs) == 0 {
		return nil, errors.New("search jobs: upstream reports jobs but none parsed from results HTML")
	}
	result.HasJobs = raw.HasJobs
	result.HasContent = raw.HasContent
	return result, nil
}

// resolveLocation geocodes a partial place name typed by the user. Its result
// must be passed as JobsRequest.Location (via locationSuggestion.asFilter) for
// location filtering to have any effect on Jobs — see package docs.
func (c *Client) resolveLocation(ctx context.Context, term string) ([]locationSuggestion, error) {
	q := url.Values{"term": {term}, "countryCodes": {""}, "lat": {""}, "lon": {""}}
	req, err := newRequest(ctx, http.MethodGet, c.baseURL+"/search-jobs/locations?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("resolve location: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resolve location: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resolve location: HTTP %d", resp.StatusCode)
	}
	var suggestions []locationSuggestion
	if err := json.NewDecoder(resp.Body).Decode(&suggestions); err != nil {
		return nil, fmt.Errorf("resolve location: decode: %w", err)
	}
	return suggestions, nil
}

// JobDetail expects city, slug, and jobID from one [Job] returned by
// [Client.Jobs].
func (c *Client) JobDetail(ctx context.Context, city, slug, jobID string) (*JobDetailResponse, error) {
	path := fmt.Sprintf("/job/%s/%s/%s/%s", city, slug, orgID, jobID)
	req, err := newRequest(ctx, http.MethodGet, c.baseURL+path)
	if err != nil {
		return nil, fmt.Errorf("job detail: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("job detail: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("job detail: HTTP %d", resp.StatusCode)
	}
	result, err := parseJobDetail(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("job detail: parse: %w", err)
	}
	return result, nil
}
