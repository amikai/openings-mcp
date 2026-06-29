package tsmc

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"golang.org/x/net/html"
)

const defaultBaseURL = "https://careers.tsmc.com"

const (
	defaultPerPage = 10
	pathSearchJobs = "/zh_TW/careers/SearchJobs/"
	pathJobDetail  = "/zh_TW/careers/JobDetail"
)

// Query parameter field IDs
const (
	ParamOrganization   = "4177"
	ParamLocation       = "1277"
	ParamCategory       = "558"
	ParamJobType        = "147"
	ParamEmploymentType = "542"
)

// Organization (field 4177)
const (
	OrgTSMCGroup = "5410262"
	OrgESMC      = "5410263"
	OrgJASM      = "5410264"
)

// Location (field 1277)
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

// Category (field 558)
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

// JobType (field 147)
const (
	JobTypeTechnician        = "5710"
	JobTypeAssociateEngineer = "39075"
	JobTypeEngineer          = "5709"
	JobTypeManager           = "5708"
	JobTypeOthers            = "39076"
)

// EmploymentType (field 542)
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

func NewClient(httpClient *http.Client) *Client {
	return &Client{
		httpClient: cmp.Or(httpClient, http.DefaultClient),
		baseURL:    defaultBaseURL,
	}
}

func (c *Client) jobsURL(p *JobsRequest) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	u = u.JoinPath(pathSearchJobs)
	if p.Keyword != "" {
		u = u.JoinPath(p.Keyword)
	}
	q := u.Query()
	q.Set("listFilterMode", "1")

	perPage := p.PerPage
	if perPage <= 0 {
		perPage = defaultPerPage
	}
	q.Set("jobRecordsPerPage", strconv.Itoa(perPage))

	if p.Page > 1 {
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

func (c *Client) Jobs(ctx context.Context, p *JobsRequest) (*JobsResponse, error) {
	rawURL, err := c.jobsURL(p)
	if err != nil {
		return nil, err
	}
	doc, err := c.getHTML(ctx, rawURL, c.baseURL+pathSearchJobs)
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	jobs, total := parseSearchHTML(doc)
	return &JobsResponse{Total: total, Jobs: jobs}, nil
}

func (c *Client) JobDetail(ctx context.Context, jobID string) (*JobDetailResponse, error) {
	u := c.baseURL + pathJobDetail + "?jobId=" + url.QueryEscape(jobID) + "&source=External+Career+Site"
	doc, err := c.getHTML(ctx, u, c.baseURL+pathSearchJobs)
	if err != nil {
		return nil, fmt.Errorf("job detail %s: %w", jobID, err)
	}
	detail, ok := parseDetailHTML(doc)
	if !ok {
		return nil, fmt.Errorf("job detail %s: not found in response", jobID)
	}
	detail.ID = jobID
	return &detail, nil
}

func (c *Client) getHTML(ctx context.Context, rawURL, referer string) (*html.Node, error) {
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
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}
