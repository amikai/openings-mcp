package workingnomads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Job is one posting, flattened from an exposed_jobs entry.
type Job struct {
	ID          string // numeric id parsed out of URL, e.g. "1734670"; pass to [Client.Detail]
	Title       string
	Company     string
	Category    string // free text, e.g. "Development", "Design"; no fixed enum, see doc.go
	Tags        []string
	Location    string // free text, e.g. "Global", "Sweden - Remote", "Europe, North America, Latin America, APAC"
	Description string // full posting body, HTML
	PostedAt    time.Time
	URL         string // outbound apply link (redirects to the employer's site); NOT a WorkingNomads-hosted detail page
}

// Client fetches Working Nomads' public exposed_jobs feed.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a Client. When httpClient is nil, http.DefaultClient is
// used. The feed needs no auth, cookies, or User-Agent spoofing.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, baseURL: strings.TrimSuffix(baseURL, "/")}
}

// Jobs fetches the full, unfiltered dump.
func (c *Client) Jobs(ctx context.Context) ([]Job, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/exposed_jobs/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var entries []exposedJob
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	jobs := make([]Job, 0, len(entries))
	for _, e := range entries {
		jobs = append(jobs, e.toJob())
	}
	return jobs, nil
}

// Search fetches the full dump and filters it with [FilterJobs]. Working
// Nomads has no server-side search — see doc.go — so this is the only way
// to narrow results.
func (c *Client) Search(ctx context.Context, opts FilterOptions) ([]Job, error) {
	jobs, err := c.Jobs(ctx)
	if err != nil {
		return nil, err
	}
	return FilterJobs(jobs, opts), nil
}

// Detail resolves one job by [Job.ID] from a fresh [Client.Jobs] fetch.
// There is no per-job endpoint — see the package doc for why. An ID that
// has expired or rotated out of the dump is an error.
func (c *Client) Detail(ctx context.Context, id string) (*Job, error) {
	jobs, err := c.Jobs(ctx)
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		if jobs[i].ID == id {
			return &jobs[i], nil
		}
	}
	return nil, fmt.Errorf("job %q not found in the current dump; it may have expired or rotated out", id)
}

// exposedJob mirrors one entry of the exposed_jobs API response.
type exposedJob struct {
	URL          string `json:"url"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	CompanyName  string `json:"company_name"`
	CategoryName string `json:"category_name"`
	Tags         string `json:"tags"`
	Location     string `json:"location"`
	PubDate      string `json:"pub_date"`
}

// jobIDPattern extracts the numeric id from a job URL, e.g.
// "https://www.workingnomads.com/job/go/1734670/" -> "1734670".
var jobIDPattern = regexp.MustCompile(`/job/go/(\d+)/`)

func (e exposedJob) toJob() Job {
	return Job{
		ID:          jobID(e.URL),
		Title:       e.Title,
		Company:     e.CompanyName,
		Category:    e.CategoryName,
		Tags:        parseTags(e.Tags),
		Location:    e.Location,
		Description: e.Description,
		PostedAt:    parsePubDate(e.PubDate),
		URL:         e.URL,
	}
}

// jobID extracts the numeric id from a job URL's /job/go/<id>/ path. A URL
// that doesn't match the expected shape falls back to the raw URL, so a
// future format change degrades to a (still-unique, still-stable) opaque
// ID rather than colliding entries.
func jobID(rawURL string) string {
	if m := jobIDPattern.FindStringSubmatch(rawURL); m != nil {
		return m[1]
	}
	return rawURL
}

func parseTags(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// parsePubDate parses the feed's fixed ISO 8601 timestamp with a numeric
// UTC offset, e.g. "2026-07-17T07:17:36-04:00". An unparseable or empty
// value yields the zero time rather than an error.
func parsePubDate(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
