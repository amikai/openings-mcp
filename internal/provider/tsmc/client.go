package tsmc

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const (
	defaultPerPage = 10
	jobsPath       = "/zh_TW/careers/SearchJobs/"
	jobDetailPath  = "/zh_TW/careers/JobDetail"
)

// Query parameter field IDs.
const (
	ParamLocation       = "1277"
	ParamCategory       = "558"
	ParamJobType        = "147"
	ParamEmploymentType = "542"
)

// Location values for field 1277.
const (
	LocTaiwan           = "13209"
	LocCanada           = "13210"
	LocChina            = "13211"
	LocGermanyDresden   = "2326764"
	LocGermanyMunich    = "4762540"
	LocJapanYokohama    = "13214"
	LocJapanOsaka       = "13215"
	LocJapanTsukuba     = "13216"
	LocJapanKumamoto    = "13217"
	LocKorea            = "13212"
	LocNetherlands      = "13213"
	LocUSAArizona       = "13221"
	LocUSACalifornia    = "13218"
	LocUSAMassachusetts = "13219"
	LocUSATexas         = "13220"
	LocUSAWashington    = "13222"
	LocUSAWashingtonDC  = "13223"
)

// Category values for field 558.
const (
	CatRD                      = "38617"
	CatSpecialtyTechnology     = "38618"
	CatICDesignTechnology      = "38619"
	CatManufacturing           = "38620"
	CatFacilityAndSafety       = "38621"
	CatProductDevelopment      = "38622"
	CatICPackagingTechnology   = "38623"
	CatTestingDevelopment      = "38635"
	CatQualityAndReliability   = "38624"
	CatIT                      = "38625"
	CatInternalAudit           = "38626"
	CatBusinessDevelopment     = "38627"
	CatCustomerService         = "38628"
	CatCorporatePlanning       = "38629"
	CatFinance                 = "38630"
	CatHumanResources          = "38631"
	CatLegal                   = "38632"
	CatMaterialsManagement     = "38633"
	CatCorporateSustainability = "7898835"
	CatAdministration          = "38634"
	CatAccessibilityInclusion  = "38636"
)

// Job type values for field 147.
const (
	JobTypeTechnician        = "5710"
	JobTypeAssociateEngineer = "39075"
	JobTypeEngineer          = "5709"
	JobTypeManager           = "5708"
	JobTypeOthers            = "39076"
)

// Employment type values for field 542.
const (
	EmployRegular        = "5701"
	EmployTemporary      = "5702"
	EmployIntern         = "13100"
	EmployApprenticeship = "4348108"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

// JobsRequest treats Page as one-based and PerPage as the requested page size.
type JobsRequest struct {
	Keyword         string
	Locations       []string
	Categories      []string
	JobTypes        []string
	EmploymentTypes []string
	Page            int
	PerPage         int
}

type JobsResponse struct {
	Total int
	Jobs  []Job
}

type Job struct {
	ID             string
	Slug           string
	Title          string
	Location       string
	CareerArea     string
	EmploymentType string
	Posted         string
}

type JobDetailResponse struct {
	ID               string
	Slug             string
	Title            string
	Company          string
	Location         string
	CareerArea       string
	JobType          string
	EmploymentType   string
	Posted           string
	Responsibilities string
	Qualifications   string
}

// NewClient uses [http.DefaultClient] when httpClient is nil.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		httpClient: cmp.Or(httpClient, http.DefaultClient),
		baseURL:    baseURL,
	}
}

func (c *Client) jobsURL(p *JobsRequest) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	u = u.JoinPath(jobsPath)
	if p.Keyword != "" {
		// The keyword is free text; JoinPath would treat "/" as a segment
		// separator and clean "..", so append it as one escaped segment.
		if strings.Trim(p.Keyword, ".") == "" {
			return "", fmt.Errorf("invalid keyword %q", p.Keyword)
		}
		basePath, baseRaw := u.Path, u.EscapedPath()
		if !strings.HasSuffix(basePath, "/") {
			basePath += "/"
			baseRaw += "/"
		}
		u.Path = basePath + p.Keyword
		u.RawPath = baseRaw + url.PathEscape(p.Keyword)
	}
	q := u.Query()
	q.Set("listFilterMode", "1")

	perPage := p.PerPage
	if perPage <= 0 {
		perPage = defaultPerPage
	}
	q.Set("jobRecordsPerPage", strconv.Itoa(perPage))

	if p.Page > 1 {
		if p.Page-1 > math.MaxInt/perPage {
			return "", fmt.Errorf("page %d is too large", p.Page)
		}
		q.Set("jobOffset", strconv.Itoa((p.Page-1)*perPage))
	}
	for _, v := range p.Locations {
		q.Add(ParamLocation, v)
	}

	for _, v := range p.Categories {
		q.Add(ParamCategory, v)
	}

	for _, v := range p.JobTypes {
		q.Add(ParamJobType, v)
	}

	for _, v := range p.EmploymentTypes {
		q.Add(ParamEmploymentType, v)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Jobs returns summaries whose [Job.ID] values are accepted by [Client.JobDetail].
func (c *Client) Jobs(ctx context.Context, p *JobsRequest) (*JobsResponse, error) {
	rawURL, err := c.jobsURL(p)
	if err != nil {
		return nil, err
	}
	doc, err := c.getHTML(ctx, rawURL, c.baseURL+jobsPath)
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	jobs, total, err := parseSearchHTML(doc)
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	return &JobsResponse{Total: total, Jobs: jobs}, nil
}

// JobDetail expects a [Job.ID] returned by [Client.Jobs].
func (c *Client) JobDetail(ctx context.Context, jobID string) (*JobDetailResponse, error) {
	if jobID == "" {
		return nil, errors.New("job detail: empty job id")
	}
	u := c.baseURL + jobDetailPath + "?jobId=" + url.QueryEscape(jobID) + "&source=External+Career+Site"
	doc, err := c.getHTML(ctx, u, c.baseURL+jobsPath)
	if err != nil {
		return nil, fmt.Errorf("job detail %q: %w", jobID, err)
	}
	detail, ok := parseDetailHTML(doc)
	if !ok {
		return nil, fmt.Errorf("job detail %q: not found in response", jobID)
	}
	// The canonical link names the job the page is actually for; a mismatch
	// means upstream served different content than requested.
	if detail.ID != jobID {
		return nil, fmt.Errorf("job detail %q: response is for job %q", jobID, detail.ID)
	}
	return &detail, nil
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
