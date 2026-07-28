package taiwanjobs

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	feedPath = "/webservice_taipei/Webservice.ashx"
	// MaxCount is the upstream row cap: the service returns at most 1000 rows
	// per request "to maintain database stability" (dataset 44062 docs).
	MaxCount = 1000
	// DefaultCount keeps the default fetch small; each row carries a full
	// posting body, so 1000 rows is a very large payload.
	DefaultCount = 100
)

// Client talks to the TaiwanJobs open XML feed.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// JobsRequest selects rows from the feed. All fields are optional.
type JobsRequest struct {
	// Count caps returned rows (upstream max 1000). Zero means DefaultCount.
	Count int
	// ZipNo filters by Taiwan postal code of the work location, e.g. "110".
	ZipNo string
	// JobNo filters by official job category code: 2-digit major (e.g. "07")
	// or 6-digit minor (e.g. "070113").
	JobNo string
	// Keyword is a client-side, case-insensitive substring filter over title,
	// company, and posting body. The upstream feed has no keyword parameter,
	// so this filters the fetched rows only — pair it with a generous Count.
	Keyword string
}

// SearchResponse is the parsed feed. Fetched counts rows returned by the
// feed before the Keyword filter; len(Jobs) is the count after it.
type SearchResponse struct {
	Fetched int   `json:"fetched"`
	Jobs    []Job `json:"jobs"`
}

// Job is one feed row. XML tags are the upstream element names with their
// Chinese annotations stripped (see tagAnnotation in parse.go). JSON names
// follow the repo-wide summary conventions (title, company, url) so agents
// don't relearn provider-only names.
type Job struct {
	Title            string `xml:"OCCU_DESC" json:"title"`
	EmploymentType   string `xml:"WK_TYPE" json:"employment_type,omitempty"`
	CategoryCode     string `xml:"CJOB1_COUNT" json:"category_code,omitempty"`
	CategoryName     string `xml:"CJOB_NAME1" json:"category_name,omitempty"`
	SubcategoryCode  string `xml:"CJOB2_COUNT" json:"subcategory_code,omitempty"`
	SubcategoryName  string `xml:"CJOB_NAME2" json:"subcategory_name,omitempty"`
	Openings         string `xml:"JOB_PERSON" json:"openings,omitempty"`
	ApplyDeadline    string `xml:"STOP_DATE" json:"apply_deadline,omitempty"`
	Description      string `xml:"JOB_DETAIL" json:"description,omitempty"`
	Location         string `xml:"CITYNAME" json:"location,omitempty"`
	Experience       string `xml:"EXPERIENCE" json:"experience,omitempty"`
	WorkHours        string `xml:"WKTIME" json:"work_hours,omitempty"`
	SalaryType       string `xml:"SALARYCD" json:"salary_type,omitempty"`
	SalaryLow        string `xml:"NT_L" json:"salary_low,omitempty"`
	SalaryHigh       string `xml:"NT_U" json:"salary_high,omitempty"`
	MinimumEducation string `xml:"EDGRDESC" json:"minimum_education,omitempty"`
	URL              string `xml:"URL_QUERY" json:"url,omitempty"`
	Company          string `xml:"COMPNAME" json:"company,omitempty"`
	UpdatedAt        string `xml:"TRANDATE" json:"updated_at,omitempty"`
}

// NewClient builds a Client. When httpClient is nil, http.DefaultClient is used.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

// Jobs fetches the feed and applies the client-side Keyword filter.
func (c *Client) Jobs(ctx context.Context, req *JobsRequest) (*SearchResponse, error) {
	if req == nil {
		req = &JobsRequest{}
	}
	count := req.Count
	if count <= 0 {
		count = DefaultCount
	}
	if count > MaxCount {
		return nil, fmt.Errorf("taiwanjobs: count %d exceeds upstream max %d", count, MaxCount)
	}
	u, err := url.Parse(c.baseURL + feedPath)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	q := u.Query()
	q.Set("count", strconv.Itoa(count))
	if req.ZipNo != "" {
		q.Set("zipno", req.ZipNo)
	}
	if req.JobNo != "" {
		q.Set("jobno", req.JobNo)
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; openings-mcp/taiwanjobs)")
	httpReq.Header.Set("Accept", "text/xml,application/xml;q=0.9,*/*;q=0.8")

	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("taiwanjobs: unexpected status %d for %s", res.StatusCode, u)
	}
	jobs, err := parseFeed(res.Body)
	if err != nil {
		return nil, err
	}
	resp := &SearchResponse{Fetched: len(jobs)}
	resp.Jobs = filterKeyword(jobs, req.Keyword)
	return resp, nil
}

// filterKeyword keeps jobs whose title, company, or description contains
// keyword, case-insensitively. Empty keyword keeps everything.
func filterKeyword(jobs []Job, keyword string) []Job {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return jobs
	}
	kept := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		hay := strings.ToLower(j.Title + "\n" + j.Company + "\n" + j.Description)
		if strings.Contains(hay, keyword) {
			kept = append(kept, j)
		}
	}
	return kept
}
