package jobindex

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	searchPath = "/jobsoegning"
	detailPath = "/vis-job"
	// DefaultPageSize is Jobindex's fixed search page size observed live.
	DefaultPageSize = 20
)

// Sort order for search results (query param sort=).
const (
	SortScore = "score"
	SortDate  = "date"
)

// Client talks to Jobindex.dk's public HTML search surface.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// JobsRequest is a server-side search. Keyword may be empty to browse, but
// Jobindex returns broader noise without it.
type JobsRequest struct {
	Keyword string
	// Area is an optional path slug (e.g. "storkoebenhavn"). Free-text cities
	// can instead be included in Keyword.
	Area string
	// Page is 1-based. Zero means page 1.
	Page int
	// JobAgeDays caps posting age (1, 7, 14, 30). Zero means no filter.
	JobAgeDays int
	// Sort is SortScore (default) or SortDate.
	Sort string
}

// SearchResponse is the Stash searchResponse object from /jobsoegning, plus the
// request page. Field names match Jobindex's embedded JSON (hitcount,
// total_pages, results), not an ai-job-search style card schema.
//
// Each element of Results is one upstream result object. The only deliberate
// drop is the per-result "html" field (full card markup); every other key is
// preserved as returned by Jobindex.
type SearchResponse struct {
	Hitcount   int              `json:"hitcount"`
	TotalPages int              `json:"total_pages,omitempty"`
	Results    []map[string]any `json:"results"`
	// Page is the 1-based page that was requested. It is not a Stash field.
	Page int `json:"page"`
}

// JobDetail is scraped from the /vis-job/{tid} HTML page. Jobindex does not
// expose a JSON detail API for that view; field names mirror the Stash search
// result keys where the same concept exists (tid, headline, area, firstdate,
// share_url, apply_url, company). No fields are synthesized by merging
// deadlines (e.g. we never invent "ASAP").
type JobDetail struct {
	Tid       string         `json:"tid"`
	Headline  string         `json:"headline"`
	Company   map[string]any `json:"company,omitempty"`
	Area      string         `json:"area,omitempty"`
	Firstdate string         `json:"firstdate,omitempty"`
	// ShareURL is og:url / canonical on the vis-job page (same role as
	// search's share_url).
	ShareURL string `json:"share_url,omitempty"`
	// ApplyURL is the "Se jobbet" deep link when present (search's apply_url).
	ApplyURL string `json:"apply_url,omitempty"`
	// Description is plain text from og:description and/or the PaidJob body
	// appetizer — not a Stash search field name, but the page's description.
	Description string `json:"description,omitempty"`
	// EmploymentType / Hours / ApplyDeadline are only set when the page's
	// jix-info labels expose them; values are the page text, unnormalized.
	EmploymentType string `json:"employment_type,omitempty"`
	Hours          string `json:"hours,omitempty"`
	ApplyDeadline  string `json:"apply_deadline,omitempty"`
}

// NewClient builds a Client. When httpClient is nil, http.DefaultClient is used.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

// Jobs searches Jobindex and returns the Stash searchResponse payload.
func (c *Client) Jobs(ctx context.Context, req *JobsRequest) (*SearchResponse, error) {
	if req == nil {
		req = &JobsRequest{}
	}
	u, err := c.searchURL(req)
	if err != nil {
		return nil, err
	}
	html, err := c.getHTML(ctx, u)
	if err != nil {
		return nil, err
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	return parseSearchHTML(html, page)
}

// JobDetail fetches /vis-job/{tid} and scrapes the HTML page.
func (c *Client) JobDetail(ctx context.Context, tid string) (*JobDetail, error) {
	tid = strings.TrimSpace(tid)
	if tid == "" {
		return nil, fmt.Errorf("job tid is required")
	}
	if strings.Contains(tid, "/") {
		if extracted := tidFromURL(tid); extracted != "" {
			tid = extracted
		}
	}
	u := c.baseURL + detailPath + "/" + url.PathEscape(tid)
	html, err := c.getHTML(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseDetailHTML(html, tid)
}

func (c *Client) searchURL(req *JobsRequest) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	if area := strings.Trim(req.Area, "/"); area != "" {
		u = u.JoinPath(searchPath, area)
	} else {
		u = u.JoinPath(searchPath)
	}
	q := u.Query()
	if req.Keyword != "" {
		q.Set("q", req.Keyword)
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	if req.JobAgeDays > 0 {
		q.Set("jobage", strconv.Itoa(req.JobAgeDays))
	}
	sort := req.Sort
	if sort == "" {
		sort = SortScore
	}
	if sort != SortScore && sort != SortDate {
		return "", fmt.Errorf("invalid sort %q (want %q or %q)", sort, SortScore, SortDate)
	}
	if sort != SortScore {
		q.Set("sort", sort)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) getHTML(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; openings-mcp/jobindex)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "da,en;q=0.9")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("jobindex: not found")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("jobindex: unexpected status %d for %s", res.StatusCode, rawURL)
	}
	return string(body), nil
}

func tidFromURL(raw string) string {
	for _, seg := range []string{"/jobannonce/", "/vis-job/"} {
		if i := strings.LastIndex(raw, seg); i >= 0 {
			rest := raw[i+len(seg):]
			if j := strings.IndexAny(rest, "?#/"); j >= 0 {
				rest = rest[:j]
			}
			return rest
		}
	}
	return ""
}
