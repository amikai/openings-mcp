package mynavi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// MinSalaries are the first-year annual-salary floors (初年度年収) the
// search accepts, in units of 10,000 JPY (万円) — the fixed steps of the
// site's own salary pulldown. Any other value is HTTP 404 upstream, so
// [Client.Jobs] rejects it client-side.
var MinSalaries = []int{
	150, 200, 250, 300, 350, 400, 450, 500, 550, 600, 650, 700,
	800, 900, 1000, 1100, 1200, 1300, 1400, 1500,
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a Client. When httpClient is nil, http.DefaultClient is
// used. Mynavi needs no cookies or session warm-up.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// JobsRequest describes one search-results page. Filters map to Mynavi
// Tenshoku's URL path-token DSL (see the package documentation).
type JobsRequest struct {
	Keywords  string // free text; space-separated terms AND together
	MinSalary int    // annual floor in units of 10,000 JPY, one of MinSalaries; 0 = no filter
	Page      int    // 1-based page of 50; 0 means page 1
}

type Job struct {
	ID               string // e.g. "348855-1-29-1"; pass to Client.JobDetail
	Title            string
	Company          string
	CatchCopy        string   // employer's tagline shown next to the company name
	EmploymentStatus string   // e.g. 正社員
	Conditions       []string // condition tags, e.g. 転勤なし, リモートワーク可
	Description      string   // job-description (仕事内容) summary, server-truncated
	Target           string   // target-applicant (対象となる方) summary, server-truncated
	Location         string   // work-location (勤務地) summary, server-truncated
	Salary           string   // salary (給与) summary, server-truncated
	FirstYearIncome  string   // first-year income (初年度年収) range, absent on some postings
	UpdatedDate      string   // last-updated date (情報更新日), YYYY/MM/DD
	EndDate          string   // listing end date (掲載終了予定日), YYYY/MM/DD
}

type JobsResponse struct {
	Total int // total matches across all pages
	Jobs  []Job
}

// Location is one posting work location (a prefecture, optionally narrowed
// to a city or ward).
type Location struct {
	Region   string // prefecture, e.g. 東京都
	Locality string // city or ward, e.g. 渋谷区; often empty
}

// JobDetailResponse carries every field of the detail page's schema.org
// JobPosting JSON-LD. HTML-valued fields are flattened to plain text.
type JobDetailResponse struct {
	ID                     string
	URL                    string
	Title                  string
	Company                string
	CompanyURL             string // the employer's own site
	EmploymentType         string // schema.org value, e.g. FULL_TIME
	Industry               string
	OccupationalCategory   string
	DatePosted             string // YYYY-MM-DD
	ValidThrough           string // YYYY-MM-DD; the posting 404s after this
	Locations              []Location
	SalaryCurrency         string // e.g. JPY
	SalaryMin              string // numeric string, e.g. "4200000"
	SalaryMax              string
	SalaryUnit             string // e.g. YEAR
	Description            string
	ExperienceRequirements string
	WorkHours              string
	JobBenefits            string
}

var jobIDPattern = regexp.MustCompile(`^\d+-\d+-\d+-\d+$`)

func (c *Client) jobsURL(r *JobsRequest) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse url %q: %w", c.baseURL, err)
	}
	// Path-segment order mirrors the site's own URLs: min, kw, pg. The
	// site's canonical list URLs end with "/".
	segs := []string{"list"}
	if r.MinSalary != 0 {
		if !slices.Contains(MinSalaries, r.MinSalary) {
			return "", fmt.Errorf("min salary %d万円 is not a value the site accepts; valid steps: %v", r.MinSalary, MinSalaries)
		}
		segs = append(segs, fmt.Sprintf("min%04d", r.MinSalary))
	}
	if kw := strings.TrimSpace(r.Keywords); kw != "" {
		// An encoded "/" in the kw token is HTTP 404 upstream; fail with a
		// clearer message before sending.
		if strings.Contains(kw, "/") {
			return "", fmt.Errorf("keywords %q: the site's keyword search cannot express %q; drop it or split the term", kw, "/")
		}
		segs = append(segs, "kw"+kw)
	}
	if r.Page < 0 {
		return "", fmt.Errorf("page %d: pages are 1-based", r.Page)
	}
	if r.Page > 1 {
		segs = append(segs, fmt.Sprintf("pg%d", r.Page))
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + strings.Join(segs, "/") + "/"
	return u.String(), nil
}

// Jobs returns one page of up to 50 search results; [JobsResponse.Total]
// counts every match. A page past the last one returns zero jobs, not an
// error.
func (c *Client) Jobs(ctx context.Context, r *JobsRequest) (*JobsResponse, error) {
	rawURL, err := c.jobsURL(r)
	if err != nil {
		return nil, fmt.Errorf("build jobs url: %w", err)
	}
	doc, err := c.getHTML(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	resp, err := parseJobsHTML(doc)
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	return resp, nil
}

// JobDetail expects a [Job.ID] returned by [Client.Jobs]. An unknown or
// expired ID is an error (the site serves a clean 404 once a posting
// passes its listing end date).
func (c *Client) JobDetail(ctx context.Context, jobID string) (*JobDetailResponse, error) {
	if !jobIDPattern.MatchString(jobID) {
		return nil, fmt.Errorf("job id %q: want four numbers separated by hyphens, e.g. 348855-1-29-1", jobID)
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse url %q: %w", c.baseURL, err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/jobinfo-" + jobID + "/"
	doc, err := c.getHTML(ctx, u.String())
	if err != nil {
		return nil, fmt.Errorf("job detail %q: %w", jobID, err)
	}
	detail, err := parseJobDetailHTML(doc, jobID)
	if err != nil {
		return nil, fmt.Errorf("job detail %q: %w", jobID, err)
	}
	return detail, nil
}

func (c *Client) getHTML(ctx context.Context, rawURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ja,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// continue
	case http.StatusNotFound:
		return nil, errors.New("HTTP 404: no such page — an expired/unknown job id, or a filter value the site does not accept")
	default:
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}
