package google

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const defaultBaseURL = "https://www.google.com/about/careers/applications"

type Config struct {
	HTTPClient *http.Client
	BaseURL    string
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

type JobsRequest struct {
	Query          string
	Locations      []string
	HasRemote      bool
	TargetLevels   []string
	Skills         string
	Degrees        []string
	EmploymentType []string
	Companies      []string
	SortBy         string
	Page           int
}

type JobsResponse struct {
	Jobs []Job
}

type Job struct {
	ID       string
	Path     string
	Title    string
	Company  string
	Location string
}

type JobDetailResponse struct {
	ID               string
	Path             string
	Title            string
	Company          string
	Location         string
	About            string
	Qualifications   string
	Responsibilities string
}

func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: cmp.Or(cfg.HTTPClient, http.DefaultClient),
		baseURL:    strings.TrimRight(cmp.Or(cfg.BaseURL, defaultBaseURL), "/"),
	}
}

func (c *Client) Jobs(ctx context.Context, p *JobsRequest) (*JobsResponse, error) {
	u, err := url.Parse(c.baseURL + "/jobs/results")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if p.Query != "" {
		q.Set("q", p.Query)
	}
	addAll(q, "location", p.Locations)
	if p.HasRemote {
		q.Set("has_remote", "true")
	}
	addAll(q, "target_level", p.TargetLevels)
	if p.Skills != "" {
		q.Set("skills", p.Skills)
	}
	addAll(q, "degree", p.Degrees)
	addAll(q, "employment_type", p.EmploymentType)
	addAll(q, "company", p.Companies)
	if p.SortBy != "" {
		q.Set("sort_by", p.SortBy)
	}
	if p.Page > 0 {
		q.Set("page", strconv.Itoa(p.Page))
	}
	u.RawQuery = q.Encode()

	body, err := c.getHTML(ctx, u.String(), c.baseURL+"/jobs")
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	return &JobsResponse{Jobs: parseSearchHTML(body)}, nil
}

func (c *Client) JobDetail(ctx context.Context, jobIDOrPath string) (*JobDetailResponse, error) {
	path := strings.Trim(jobIDOrPath, "/")
	u := c.baseURL + "/jobs/results/" + path
	body, err := c.getHTML(ctx, u, c.baseURL+"/jobs/results")
	if err != nil {
		return nil, fmt.Errorf("job detail %s: %w", jobIDOrPath, err)
	}
	id := numericID(path)
	detail, ok := parseDetailHTML(body, id)
	if !ok {
		return nil, fmt.Errorf("job detail %s: not found in response", jobIDOrPath)
	}
	if detail.Path == "" {
		detail.Path = path
	}
	return &detail, nil
}

func (c *Client) getHTML(ctx context.Context, rawURL, referer string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func addAll(q url.Values, key string, values []string) {
	for _, value := range values {
		if value != "" {
			q.Add(key, value)
		}
	}
}
