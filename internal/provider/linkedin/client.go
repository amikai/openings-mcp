package linkedin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

const (
	jobsPath      = "/jobs-guest/jobs/api/seeMoreJobPostings/search"
	jobDetailPath = "/jobs/view"
)

// Workplace type (f_WT). Only WorkplaceRemote was exercised against the live
// site; see openapi.yaml.
const (
	WorkplaceOnSite = "1"
	WorkplaceRemote = "2"
	WorkplaceHybrid = "3"
)

// Job type (f_JT), single-letter codes.
const (
	JobTypeFullTime   = "F"
	JobTypePartTime   = "P"
	JobTypeContract   = "C"
	JobTypeTemporary  = "T"
	JobTypeInternship = "I"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

type JobsRequest struct {
	Keywords            string
	Location            string
	Distance            int
	WorkplaceType       string // f_WT: one of Workplace* above
	JobType             string // f_JT: one of JobType* above
	EasyApply           bool   // f_AL
	CompanyIDs          []string
	PostedWithinSeconds int // f_TPR=r{n}
	Start               int
}

type Job struct {
	ID         string
	Title      string
	Company    string
	CompanyURL string
	Location   string
	PostedDate string // ISO 8601, e.g. "2026-06-03"
	Remote     bool
}

type JobsResponse struct {
	Jobs []Job
}

type JobDetailResponse struct {
	ID             string
	Title          string
	Company        string
	Location       string
	Posted         string // relative text, e.g. "1 month ago"
	SeniorityLevel string
	EmploymentType string
	JobFunction    string
	Industries     string
	Description    string
	CompanyLogo    string
	ApplyURL       string
	Remote         bool
}

// NewClient builds a Client. When httpClient is nil, the default carries a
// cookie jar: a cold request to jobDetailPath commonly gets HTTP 999
// (bot-suspected authwall redirect), but succeeds once it carries the cookies
// a prior request to the same host received — the same way a browser would
// behave across a search-then-click flow. See openapi.yaml Key Behaviors.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		jar, _ := cookiejar.New(nil)
		httpClient = &http.Client{Jar: jar}
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

func (c *Client) jobsURL(r *JobsRequest) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse url %s: %w", c.baseURL, err)
	}
	u = u.JoinPath(jobsPath)

	q := u.Query()
	if r.Keywords != "" {
		q.Set("keywords", r.Keywords)
	}
	if r.Location != "" {
		q.Set("location", r.Location)
	}
	if r.Distance > 0 {
		q.Set("distance", strconv.Itoa(r.Distance))
	}
	if r.WorkplaceType != "" {
		q.Set("f_WT", r.WorkplaceType)
	}
	if r.JobType != "" {
		q.Set("f_JT", r.JobType)
	}
	// pageNum is always 0 in practice; pagination is driven by start alone.
	q.Set("pageNum", "0")
	q.Set("start", strconv.Itoa(r.Start))
	if r.EasyApply {
		q.Set("f_AL", "true")
	}
	if len(r.CompanyIDs) > 0 {
		q.Set("f_C", strings.Join(r.CompanyIDs, ","))
	}
	if r.PostedWithinSeconds > 0 {
		q.Set("f_TPR", "r"+strconv.Itoa(r.PostedWithinSeconds))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) Jobs(ctx context.Context, r *JobsRequest) (*JobsResponse, error) {
	rawURL, err := c.jobsURL(r)
	if err != nil {
		return nil, fmt.Errorf("build jobs url: %w", err)
	}
	doc, err := c.getHTML(ctx, rawURL, c.baseURL+"/jobs/search")
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	return &JobsResponse{Jobs: parseJobsHTML(doc)}, nil
}

func (c *Client) JobDetail(ctx context.Context, jobID string) (*JobDetailResponse, error) {
	if jobID == "" {
		return nil, errors.New("empty job id")
	}
	u, err := url.JoinPath(c.baseURL, jobDetailPath, jobID)
	if err != nil {
		return nil, fmt.Errorf("build job detail url: %w", err)
	}
	doc, err := c.getHTML(ctx, u, c.baseURL+"/jobs/search")
	if err != nil {
		return nil, fmt.Errorf("job detail %s: %w", jobID, err)
	}
	detail, ok := parseJobDetailHTML(doc, jobID)
	if !ok {
		return nil, fmt.Errorf("job detail %s: not found in response", jobID)
	}
	return detail, nil
}

func (c *Client) getHTML(ctx context.Context, rawURL, referer string) (*html.Node, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 999 {
		return nil, errors.New("HTTP 999: bot-suspected, LinkedIn redirected to its authwall (retry with a warmed session; see openapi.yaml)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if final := resp.Request.URL.String(); strings.Contains(final, "linkedin.com/signup") || strings.Contains(final, "/authwall") {
		return nil, fmt.Errorf("redirected to %s: no usable session", final)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}
