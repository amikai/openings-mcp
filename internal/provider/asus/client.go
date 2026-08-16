package asus

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	defaultBaseURL = "https://recruit.asus.com"
	jobsPath       = "/Jobs"
	detailPath     = "/Jobs/Detail"
	citiesPath     = "/Jobs/GetCities"
)

// Category represents a top-level job category prefix on ASUS Careers.
type Category string

const (
	CategoryRD        Category = "研究發展"
	CategoryMarketing Category = "業務/行銷"
	CategoryTech      Category = "工程技術"
	CategoryAdmin     Category = "管理/支援"
)

// AllCategories lists all valid top-level category prefixes.
var AllCategories = []Category{
	CategoryRD,
	CategoryMarketing,
	CategoryTech,
	CategoryAdmin,
}

// ExperienceLevel represents a work experience filter code on ASUS Careers.
type ExperienceLevel string

const (
	ExperienceUnder2Years ExperienceLevel = "0"
	Experience3To5Years   ExperienceLevel = "1"
	Experience6To10Years  ExperienceLevel = "2"
	Experience11To15Years ExperienceLevel = "3"
	ExperienceOver16Years ExperienceLevel = "4"
)

// AllExperienceLevels lists all valid experience level filter codes.
var AllExperienceLevels = []ExperienceLevel{
	ExperienceUnder2Years,
	Experience3To5Years,
	Experience6To10Years,
	Experience11To15Years,
	ExperienceOver16Years,
}

// Client talks to the ASUS Careers site.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// SearchRequest configures a query to /Jobs.
type SearchRequest struct {
	Keyword    string
	Categories []Category
	Location   string
	City       string
	Experience ExperienceLevel
	Page       int
}

// SearchResponse contains the parsed jobs and pagination state.
type SearchResponse struct {
	Jobs        []JobSummary `json:"jobs"`
	TotalPages  int          `json:"totalPages"`
	CurrentPage int          `json:"currentPage"`
}

// JobSummary represents a vacancy listed on the search results page.
type JobSummary struct {
	ID         string `json:"id"`
	JobNo      string `json:"jobNo,omitempty"`
	Title      string `json:"title"`
	Category   string `json:"category,omitempty"`
	Location   string `json:"location,omitempty"`
	Experience string `json:"experience,omitempty"`
	Education  string `json:"education,omitempty"`
	DetailURL  string `json:"detailUrl,omitempty"`
	ApplyURL   string `json:"applyUrl,omitempty"`
}

// JobDetail represents full vacancy information from /Jobs/Detail.
type JobDetail struct {
	ID             string `json:"id"`
	JobNo          string `json:"jobNo,omitempty"`
	Title          string `json:"title"`
	Category       string `json:"category,omitempty"`
	Location       string `json:"location,omitempty"`
	Experience     string `json:"experience,omitempty"`
	Education      string `json:"education,omitempty"`
	EmploymentType string `json:"employmentType,omitempty"`
	Description    string `json:"description,omitempty"`
	Requirements   string `json:"requirements,omitempty"`
	ApplyURL       string `json:"applyUrl,omitempty"`
}

// CityItem represents a city option returned by /Jobs/GetCities.
type CityItem struct {
	Disabled bool   `json:"disabled"`
	Selected bool   `json:"selected"`
	Text     string `json:"text"`
	Value    string `json:"value"`
}

// NewClient constructs an ASUS client. If httpClient is nil, http.DefaultClient is used.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		httpClient: cmp.Or(httpClient, http.DefaultClient),
		baseURL:    cmp.Or(strings.TrimRight(baseURL, "/"), defaultBaseURL),
	}
}

// Search queries the ASUS job board and parses the resulting HTML.
func (c *Client) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req == nil {
		req = &SearchRequest{}
	}
	if req.Page < 0 {
		return nil, fmt.Errorf("page must be >= 1, got %d", req.Page)
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse baseURL: %w", err)
	}
	u.Path = jobsPath

	q := u.Query()
	if req.Keyword != "" {
		q.Set("Keyword", req.Keyword)
	}
	for _, cat := range req.Categories {
		if cat != "" {
			q.Add("REQ_TYPEs_Prefix", string(cat))
		}
	}
	if req.Location != "" {
		q.Set("Location", req.Location)
	}
	if req.City != "" {
		q.Set("City", req.City)
	}
	if req.Experience != "" {
		q.Set("WORK_EXP", string(req.Experience))
	}
	if req.Page > 1 {
		q.Set("page", strconv.Itoa(req.Page))
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create search request: %w", err)
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	return parseSearchHTML(resp.Body, c.baseURL)
}

// Detail retrieves and parses full details for a job given its opaque UUID ID.
func (c *Client) Detail(ctx context.Context, id string) (*JobDetail, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("job id cannot be empty")
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse baseURL: %w", err)
	}
	u.Path = detailPath
	q := u.Query()
	q.Set("sn", id)
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create detail request: %w", err)
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute detail request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusInternalServerError || resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("job %q not found (status %d)", id, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("detail failed with status %d", resp.StatusCode)
	}

	return parseDetailHTML(resp.Body, id, c.baseURL)
}

// GetCities queries the city list available for a country code (e.g. "TW", "US").
func (c *Client) GetCities(ctx context.Context, countryCode string) ([]CityItem, error) {
	if strings.TrimSpace(countryCode) == "" {
		return nil, errors.New("country code cannot be empty")
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse baseURL: %w", err)
	}
	u.Path = citiesPath
	q := u.Query()
	q.Set("countryTw", countryCode)
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create getCities request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute getCities request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getCities failed with status %d: %s", resp.StatusCode, string(body))
	}

	var raw []struct {
		Disabled bool   `json:"Disabled"`
		Selected bool   `json:"Selected"`
		Text     string `json:"Text"`
		Value    string `json:"Value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode getCities response: %w", err)
	}

	cities := make([]CityItem, len(raw))
	for i, r := range raw {
		cities[i] = CityItem{
			Disabled: r.Disabled,
			Selected: r.Selected,
			Text:     strings.TrimSpace(r.Text),
			Value:    strings.TrimSpace(r.Value),
		}
	}
	return cities, nil
}
