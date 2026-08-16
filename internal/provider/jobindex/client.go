package jobindex

import (
	"context"
	"fmt"
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
// request page. Field names mostly match Jobindex's embedded JSON (hitcount,
// total_pages, results).
//
// Why Results is []map[string]any (not a typed struct):
//
//   - Jobindex has no public JSON search API; /jobsoegning.json is 204. Results
//     are reverse-engineered from the HTML-embedded Stash blob. Key sets are
//     incomplete and unstable (optional nulls, occasional geojson/source/…),
//     so a fixed DTO would either drop unknown fields or churn on every Stash
//     tweak. A map is a pass-through bag: decode path is already map[string]any.
//   - We deliberately avoid re-shaping into a thin "card" schema. Mapping to
//     id/title/company loses Jobindex-native keys (tid, apply_deadline_asap)
//     that MCP agents and debug CLI callers need for detail lookup and fidelity.
//   - MCP documents typical keys in the tool jsonschema description instead of
//     encoding a closed Go type into the schema.
//
// Why only light renames / drops per result (see slimJobResult):
//
//   - firstdate→posted_at, lastdate→expired_at: same ISO date names as
//     ats.JobSummary.PostedAt so agents do not relearn Jobindex-only names.
//   - single url: tracking /c?t=… and share/apply twins confuse "open this job";
//     one apply/open destination is enough.
//   - drop html card markup: it is UI fullcard HTML, redundant with structured
//     fields and huge/brittle to re-parse; structured Stash keys already carry
//     tid/headline/company/area/dates.
//   - company name-only: homeurl is employer marketing, not the job apply link.
type SearchResponse struct {
	Hitcount   int              `json:"hitcount"`
	TotalPages int              `json:"total_pages,omitempty"`
	Results    []map[string]any `json:"results"`
	// Page is the 1-based page that was requested. It is not a Stash field.
	Page int `json:"page"`
}

// JobDetail is scraped from the /vis-job/{tid} HTML page. Field names mirror
// search output where concepts match. No deadline synthesis (e.g. ASAP).
// URL is the single link for applying/opening the job (prefer "Se jobbet"
// deep link, else the Jobindex vis-job page).
type JobDetail struct {
	Tid      string         `json:"tid"`
	Headline string         `json:"headline"`
	Company  map[string]any `json:"company,omitempty"` // name only
	Area     string         `json:"area,omitempty"`
	// PostedAt is when the ad was published (Jobindex firstdate), ISO 8601 date
	// (YYYY-MM-DD), same convention as ats.JobSummary.PostedAt.
	PostedAt string `json:"posted_at,omitempty"`
	// URL is where to open/apply for this job.
	URL string `json:"url,omitempty"`
	// Description is plain text from og:description and/or the body appetizer.
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
	res, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	page := req.Page
	if page < 1 {
		page = 1
	}
	return parseSearchHTML(res.Body, page)
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
	res, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return parseDetailHTML(res.Body, tid)
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

// get issues a browser-shaped GET and returns the response for the caller to
// stream from; the caller owns res.Body. Parsers read the body incrementally,
// so search stops downloading once the Stash object ends.
func (c *Client) get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; openings-mcp/jobindex)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "da,en;q=0.9")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		res.Body.Close()
		return nil, fmt.Errorf("jobindex: not found")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, fmt.Errorf("jobindex: unexpected status %d for %s", res.StatusCode, rawURL)
	}
	return res, nil
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
