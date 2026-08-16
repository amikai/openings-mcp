// Package successfactors is a client for the SAP SuccessFactors Career Site
// Builder (Jobs2Web) public career-site pages, documented in openapi.yaml.
package successfactors

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ErrJobNotFound indicates the requested job ID redirected to the platform's
// /errorpage/. A 200 page with an unrecognized template remains a distinct
// parsing error so outages and bot/login pages aren't mislabeled as missing
// jobs.
var ErrJobNotFound = errors.New("successfactors: job not found")

const (
	searchPath = "/search/"
	facetsPath = "/services/jobs/options/facetValues/"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient uses [http.DefaultClient] when httpClient is nil.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		httpClient: cmp.Or(httpClient, http.DefaultClient),
		baseURL:    baseURL,
	}
}

// SearchRequest mirrors the /search/ query parameters documented in
// openapi.yaml.
type SearchRequest struct {
	Query          string
	LocationSearch string
	// Filters maps a facet dimension name (e.g. "department", "country",
	// or any tenant-defined dimension reported by FacetValuesResponse) to
	// a single raw facet `name` value (e.g. a country code like "DE", not
	// a translated label), sent as optionsFacetsDD_<dimension>=<value>.
	// Each dimension is a single-select dropdown upstream, so this holds
	// at most one value per dimension per request.
	Filters  map[string]string
	StartRow int
}

type SearchResponse struct {
	Jobs       []Job
	TotalCount int
}

type Job struct {
	// ID is the numeric job posting ID, extracted from the row's
	// /job/{slug}/{id}/ link.
	ID       string
	Title    string
	Location string
}

// JobDetailResponse fields beyond ID/Title/Description are best-effort:
// SuccessFactors tenants configure detail-page templates independently of
// the search-results table, and a tenant may omit them entirely. See
// openapi.yaml's "Key Behaviors" section.
type JobDetailResponse struct {
	ID              string
	Title           string
	Location        string
	Employer        string
	PostedAtRaw     string
	DescriptionHTML string
}

// FacetValuesResponse is keyed by facet dimension name (e.g. "country",
// "department"); dimensions this tenant hasn't configured are simply
// absent.
type FacetValuesResponse struct {
	Facets map[string][]FacetOption
}

type FacetOption struct {
	// Name is the raw value to send back as optionsFacetsDD_<dimension>.
	Name string
	// Translated is the display label when the platform localizes this
	// dimension (populated for "country"; empty for tenant-defined
	// dimensions, whose Name already is the display label).
	Translated string
	Count      int
}

// Search returns summaries whose [Job.ID] values are accepted by [Client.JobDetail].
func (c *Client) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url %q: %w", c.baseURL, err)
	}
	u = u.JoinPath(searchPath)

	q := u.Query()
	q.Set("q", req.Query)
	q.Set("locationsearch", req.LocationSearch)
	for dimension, value := range req.Filters {
		q.Set("optionsFacetsDD_"+dimension, value)
	}
	q.Set("startrow", strconv.Itoa(max(req.StartRow, 0)))
	u.RawQuery = q.Encode()

	doc, _, err := c.getHTML(ctx, u.String())
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	jobs, total, err := parseSearchHTML(doc)
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	return &SearchResponse{Jobs: jobs, TotalCount: total}, nil
}

// JobDetail expects a [Job.ID] returned by [Client.Search]. The slug half
// of the /job/{slug}/{id}/ path is cosmetic (see openapi.yaml); this always
// repeats id in its place.
func (c *Client) JobDetail(ctx context.Context, id string) (*JobDetailResponse, error) {
	if id == "" {
		return nil, fmt.Errorf("successfactors: empty job id")
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url %q: %w", c.baseURL, err)
	}
	u = u.JoinPath("job", id, id)
	u.Path += "/"

	doc, finalURL, err := c.getHTML(ctx, u.String())
	if err != nil {
		return nil, fmt.Errorf("job detail %q: %w", id, err)
	}
	if isErrorPageURL(finalURL) {
		return nil, fmt.Errorf("job detail %q: %w", id, ErrJobNotFound)
	}
	detail, ok := parseJobDetailHTML(doc, id)
	if !ok {
		return nil, fmt.Errorf("job detail %q: unrecognized detail page", id)
	}
	return detail, nil
}

func (c *Client) FacetValues(ctx context.Context, req *SearchRequest) (*FacetValuesResponse, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url %q: %w", c.baseURL, err)
	}
	u = u.JoinPath(facetsPath)
	u.Path += "/"

	bodyFields := map[string]string{
		"q":              req.Query,
		"locationsearch": req.LocationSearch,
	}
	for dimension, value := range req.Filters {
		bodyFields["optionsFacetsDD_"+dimension] = value
	}
	body, err := json.Marshal(bodyFields)
	if err != nil {
		return nil, fmt.Errorf("marshal facetValues request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("facetValues: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facetValues: HTTP %d", resp.StatusCode)
	}

	var raw facetValuesJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("facetValues: decode response: %w", err)
	}
	return raw.toResponse(), nil
}

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

func (c *Client) getHTML(ctx context.Context, rawURL string) (*goquery.Document, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.Request.URL, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, resp.Request.URL, fmt.Errorf("parse html: %w", err)
	}
	return doc, resp.Request.URL, nil
}

func isErrorPageURL(u *url.URL) bool {
	return u != nil && strings.EqualFold(strings.Trim(u.Path, "/"), "errorpage")
}
