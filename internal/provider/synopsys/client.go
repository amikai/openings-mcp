package synopsys

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const baseURL = "https://careers.synopsys.com"

var baseHeader = http.Header{
	"User-Agent": {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"},
	"Accept":     {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
}

type Config struct {
	HTTPClient *http.Client
	BaseURL    string
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: cmp.Or(cfg.HTTPClient, http.DefaultClient),
		baseURL:    cmp.Or(cfg.BaseURL, baseURL),
	}
}

func newRequest(ctx context.Context, method, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for key, values := range baseHeader {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
	return req, nil
}

func (c *Client) Jobs(ctx context.Context, p *JobsRequest) (*JobsResponse, error) {
	q := buildSearchQuery(p)
	req, err := newRequest(ctx, http.MethodGet, c.baseURL+"/search-jobs/results?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("synopsys: search: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("synopsys: search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("synopsys: search: HTTP %d", resp.StatusCode)
	}
	var raw struct {
		Results    string `json:"results"`
		HasJobs    bool   `json:"hasJobs"`
		HasContent bool   `json:"hasContent"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("synopsys: search: decode: %w", err)
	}
	result, err := parseSearchResults(raw.Results)
	if err != nil {
		return nil, fmt.Errorf("synopsys: search: parse: %w", err)
	}
	result.HasJobs = raw.HasJobs
	result.HasContent = raw.HasContent
	return result, nil
}

func (c *Client) JobDetail(ctx context.Context, city, slug, jobID string) (*JobDetailResponse, error) {
	path := fmt.Sprintf("/job/%s/%s/44408/%s", city, slug, jobID)
	req, err := newRequest(ctx, http.MethodGet, c.baseURL+path)
	if err != nil {
		return nil, fmt.Errorf("synopsys: job detail: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("synopsys: job detail: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("synopsys: job detail: HTTP %d", resp.StatusCode)
	}
	result, err := parseJobDetail(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("synopsys: job detail: parse: %w", err)
	}
	return result, nil
}
