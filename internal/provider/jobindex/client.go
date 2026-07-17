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

// Sort order for search results.
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

// Job is a search-result summary.
type Job struct {
	ID         string
	Title      string
	Company    string
	CompanyURL string
	Location   string
	PostedDate string // YYYY-MM-DD when present
	Deadline   string // YYYY-MM-DD, "ASAP", or free text
	URL        string // canonical Jobindex URL (vis-job)
}

// JobsResponse is one page of search results.
type JobsResponse struct {
	Jobs       []Job
	TotalCount int
	Page       int
	TotalPages int
}

// JobDetail is a single posting from /vis-job/{id}.
type JobDetail struct {
	ID             string
	Title          string
	Company        string
	CompanyURL     string
	Location       string
	PostedDate     string
	Deadline       string
	EmploymentType string
	Hours          string
	Description    string
	ApplyURL       string
	URL            string
}

// NewClient builds a Client. When httpClient is nil, http.DefaultClient is used.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

// Jobs searches Jobindex and parses the Stash-embedded result list.
func (c *Client) Jobs(ctx context.Context, req *JobsRequest) (*JobsResponse, error) {
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

// JobDetail fetches /vis-job/{id}.
func (c *Client) JobDetail(ctx context.Context, id string) (*JobDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("job id is required")
	}
	if strings.Contains(id, "/") {
		// Allow full URLs: extract trailing tid.
		if tid := tidFromURL(id); tid != "" {
			id = tid
		}
	}
	u := c.baseURL + detailPath + "/" + url.PathEscape(id)
	html, err := c.getHTML(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseDetailHTML(html, id, c.baseURL+detailPath+"/"+id)
}

func (c *Client) searchURL(req *JobsRequest) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	path := searchPath
	if area := strings.Trim(req.Area, "/"); area != "" {
		path = searchPath + "/" + area
	}
	u.Path = path
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
	// .../jobannonce/h1683131 or .../vis-job/h1683131
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
