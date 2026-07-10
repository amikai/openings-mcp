package google

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/PuerkitoBio/goquery"
)

const jobsPath = "/jobs/results"

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
	ID                    string
	Title                 string
	Company               string
	Location              string
	Remote                bool
	ExperienceLevel       string
	MinimumQualifications []string
}

type JobDetailResponse struct {
	ID               string
	Title            string
	Company          string
	Location         string
	Remote           bool
	About            string
	Qualifications   string
	Responsibilities string
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		httpClient: cmp.Or(httpClient, http.DefaultClient),
		baseURL:    baseURL,
	}
}

func (c *Client) jobsRawURL(req *JobsRequest) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse url %s: %w", c.baseURL, err)
	}
	u = u.JoinPath(jobsPath)

	q := u.Query()
	if req.Query != "" {
		q.Set("q", req.Query)
	}
	addAll(q, "location", req.Locations)
	if req.HasRemote {
		q.Set("has_remote", "true")
	}
	addAll(q, "target_level", req.TargetLevels)
	if req.Skills != "" {
		q.Set("skills", req.Skills)
	}
	addAll(q, "degree", req.Degrees)
	addAll(q, "employment_type", req.EmploymentType)
	addAll(q, "company", req.Companies)
	if req.SortBy != "" {
		q.Set("sort_by", req.SortBy)
	}
	if req.Page > 0 {
		q.Set("page", strconv.Itoa(req.Page))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) Jobs(ctx context.Context, req *JobsRequest) (*JobsResponse, error) {
	u, err := c.jobsRawURL(req)
	if err != nil {
		return nil, fmt.Errorf("build jobs raw url: %w", err)
	}
	doc, err := c.getHTML(ctx, u, c.baseURL+"/jobs")
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	return &JobsResponse{Jobs: parseJobsHTML(doc)}, nil
}

func (c *Client) JobDetail(ctx context.Context, jobID string) (*JobDetailResponse, error) {
	if jobID == "" {
		return nil, errors.New("empty job id")
	}
	u, err := url.JoinPath(c.baseURL, jobsPath, jobID)
	if err != nil {
		return nil, fmt.Errorf("build job detail url: %w", err)
	}

	doc, err := c.getHTML(ctx, u, c.baseURL+"/jobs/results")
	if err != nil {
		return nil, fmt.Errorf("job detail %q: %w", jobID, err)
	}
	detail, ok := parseJobDetailHTML(doc, jobID)
	if !ok {
		return nil, fmt.Errorf("job detail %q: not found in response", jobID)
	}
	return detail, nil
}

func (c *Client) getHTML(ctx context.Context, rawURL, referer string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}

func addAll(q url.Values, key string, values []string) {
	for _, value := range values {
		if value != "" {
			q.Add(key, value)
		}
	}
}
