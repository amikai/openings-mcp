package job104

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const baseURL = "https://www.104.com.tw"

var baseHeader = http.Header{
	"User-Agent":      {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"},
	"Accept":          {"application/json, text/plain, */*"},
	"Accept-Language": {"zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7"},
}

func newRequest(ctx context.Context, method, rawURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for key, values := range baseHeader {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(httpClient *http.Client) *Client {
	return &Client{
		httpClient: cmp.Or(httpClient, http.DefaultClient),
		baseURL:    baseURL,
	}
}

func (c *Client) Jobs(ctx context.Context, p *JobsRequest) (*JobsResponse, error) {
	q := url.Values{}
	if p.Keyword != "" {
		q.Set("keyword", p.Keyword)
	}
	if p.Area != "" {
		q.Set("area", p.Area)
	}
	if p.RO != nil {
		q.Set("ro", strconv.Itoa(*p.RO))
	}
	if p.Order != nil {
		q.Set("order", strconv.Itoa(*p.Order))
	}
	if p.Page != nil {
		q.Set("page", strconv.Itoa(*p.Page))
	}
	if p.Edu != "" {
		q.Set("edu", p.Edu)
	}
	if p.RemoteWork != nil {
		q.Set("remoteWork", strconv.Itoa(*p.RemoteWork))
	}
	if p.S9 != "" {
		q.Set("s9", p.S9)
	}
	q.Set("asc", "0")
	q.Set("jobsource", "2018indexpoc")

	req, err := newRequest(ctx, http.MethodGet, c.baseURL+"/jobs/search/api/jobs?"+q.Encode(), c.baseURL+"/jobs/search/")
	if err != nil {
		return nil, fmt.Errorf("jobs: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jobs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jobs: HTTP %d", resp.StatusCode)
	}
	var result JobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jobs: decode: %w", err)
	}
	return &result, nil
}

func (c *Client) JobDetail(ctx context.Context, jobCode string) (*JobDetailResponse, error) {
	req, err := newRequest(ctx, http.MethodGet, c.baseURL+"/job/ajax/content/"+jobCode, c.baseURL+"/job/"+jobCode)
	if err != nil {
		return nil, fmt.Errorf("job detail: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("job detail: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("job detail: HTTP %d", resp.StatusCode)
	}
	var result JobDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("job detail: decode: %w", err)
	}
	return &result, nil
}

func (c *Client) Companies(ctx context.Context, p *CompaniesRequest) (*CompaniesResponse, error) {
	page := max(p.Page, 1)
	pageSize := p.PageSize
	if pageSize == 0 {
		pageSize = 10
	}
	q := url.Values{
		"keyword":  {p.Keyword},
		"page":     {strconv.Itoa(page)},
		"pageSize": {strconv.Itoa(pageSize)},
	}
	req, err := newRequest(ctx, http.MethodGet, c.baseURL+"/company/ajax/list?"+q.Encode(), c.baseURL+"/company/search/")
	if err != nil {
		return nil, fmt.Errorf("companies: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("companies: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("companies: HTTP %d", resp.StatusCode)
	}
	var result CompaniesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("companies: decode: %w", err)
	}
	return &result, nil
}

func (c *Client) CompanyDetail(ctx context.Context, companyCode string) (*CompanyDetailResponse, error) {
	req, err := newRequest(ctx, http.MethodGet, c.baseURL+"/api/companies/"+companyCode+"/content", c.baseURL+"/company/"+companyCode)
	if err != nil {
		return nil, fmt.Errorf("company detail: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("company detail: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("company detail: HTTP %d", resp.StatusCode)
	}
	var result CompanyDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("company detail: decode: %w", err)
	}
	return &result, nil
}

func (c *Client) CompanyJobs(ctx context.Context, companyCode string) (*CompanyJobsResponse, error) {
	req, err := newRequest(ctx, http.MethodGet, c.baseURL+"/api/companies/"+companyCode+"/jobs?page=1&pageSize=10", c.baseURL+"/company/"+companyCode)
	if err != nil {
		return nil, fmt.Errorf("company jobs: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("company jobs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("company jobs: HTTP %d", resp.StatusCode)
	}
	var result CompanyJobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("company jobs: decode: %w", err)
	}
	return &result, nil
}
