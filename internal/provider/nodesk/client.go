package nodesk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	// DefaultAlgoliaBaseURL is the Algolia DSN host serving NoDesk's
	// search index.
	DefaultAlgoliaBaseURL = "https://0586L1SOK8-dsn.algolia.net"
	// DefaultSiteBaseURL is the site itself, serving the job detail pages.
	DefaultSiteBaseURL = "https://nodesk.co"

	// NoDesk's Algolia application ID and search-only API key. Both are
	// public: they ship in the site's own /js/search.min.js. The key only
	// works with a nodesk.co Referer header (see the package doc).
	algoliaAppID  = "0586L1SOK8"
	algoliaAPIKey = "8dacb58c6f375cba28e19ecf1f03e9e1"

	refererValue = "https://nodesk.co/"
)

// MaxHitsPerPage is the largest page size the index actually honors;
// larger requested values are clamped to this by Algolia.
const MaxHitsPerPage = 100

// Client reads NoDesk's Algolia search index and job detail pages.
type Client struct {
	httpClient     *http.Client
	algoliaBaseURL string
	siteBaseURL    string
}

// NewClient builds a Client. When httpClient is nil, http.DefaultClient
// is used. algoliaBaseURL and siteBaseURL exist for tests; production
// callers pass [DefaultAlgoliaBaseURL] and [DefaultSiteBaseURL].
func NewClient(algoliaBaseURL, siteBaseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		httpClient:     httpClient,
		algoliaBaseURL: strings.TrimSuffix(algoliaBaseURL, "/"),
		siteBaseURL:    strings.TrimSuffix(siteBaseURL, "/"),
	}
}

// SearchOptions parameterizes one server-side query. The zero value asks
// for the first page of everything.
type SearchOptions struct {
	// Query is full-text matched by Algolia against titles, companies,
	// keywords, and regions. Empty matches every posting.
	Query string
	// Page is zero-based.
	Page int
	// HitsPerPage defaults to 20 when <= 0 and is clamped to
	// [MaxHitsPerPage] by the index.
	HitsPerPage int
	// Filter narrows to one site category path from the searchFilter
	// facet, e.g. "remote-jobs/engineering" or "remote-jobs/full-time"
	// (enumerate them with [Client.Facets]).
	Filter string
	// Region narrows to one applicantLocationRegions label, e.g.
	// "Remote - Europe". Combined with Filter, both must match.
	Region string
}

// SearchResult is one page of hits plus the index's pagination totals.
type SearchResult struct {
	Jobs        []Job
	NbHits      int // total matches across all pages (ad records included)
	Page        int
	NbPages     int
	HitsPerPage int
}

// Search runs one server-side query against the jobPosts index.
// Advertisement records (see the package doc) are dropped from Jobs;
// NbHits is the index's raw total and may count them.
func (c *Client) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	hitsPerPage := opts.HitsPerPage
	if hitsPerPage <= 0 {
		hitsPerPage = 20
	}

	params := url.Values{}
	params.Set("query", opts.Query)
	params.Set("hitsPerPage", strconv.Itoa(hitsPerPage))
	params.Set("page", strconv.Itoa(opts.Page))
	if ff := facetFilters(opts); ff != "" {
		params.Set("facetFilters", ff)
	}

	var rsp searchResponse
	if err := c.query(ctx, params, &rsp); err != nil {
		return nil, err
	}

	jobs := make([]Job, 0, len(rsp.Hits))
	for _, h := range rsp.Hits {
		if h.IsAd {
			continue
		}
		jobs = append(jobs, h.toJob(c.siteBaseURL))
	}
	return &SearchResult{
		Jobs:        jobs,
		NbHits:      rsp.NbHits,
		Page:        rsp.Page,
		NbPages:     rsp.NbPages,
		HitsPerPage: rsp.HitsPerPage,
	}, nil
}

// FacetCounts maps each filterable value to its current number of live
// postings.
type FacetCounts struct {
	// SearchFilters holds the site's category paths, usable as
	// [SearchOptions.Filter].
	SearchFilters map[string]int
	// Regions holds the applicantLocationRegions labels, usable as
	// [SearchOptions.Region].
	Regions map[string]int
}

// Facets enumerates every live searchFilter path and region label with
// its job count, via a zero-hit faceted query.
func (c *Client) Facets(ctx context.Context) (*FacetCounts, error) {
	params := url.Values{}
	params.Set("query", "")
	params.Set("hitsPerPage", "0")
	params.Set("facets", `["searchFilter","applicantLocationRegions"]`)

	var rsp searchResponse
	if err := c.query(ctx, params, &rsp); err != nil {
		return nil, err
	}
	return &FacetCounts{
		SearchFilters: rsp.Facets.SearchFilter,
		Regions:       rsp.Facets.ApplicantLocationRegions,
	}, nil
}

// Detail fetches the job page for id (a [Job.ID] slug) and parses its
// JobPosting JSON-LD block and apply link. An unknown or expired slug is
// an error (the site answers 404).
func (c *Client) Detail(ctx context.Context, id string) (*JobDetail, error) {
	pageURL := c.siteBaseURL + "/remote-jobs/" + url.PathEscape(id) + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("job %q not found; it may have expired", id)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, pageURL)
	}

	detail, err := parseDetailPage(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse job page %s: %w", pageURL, err)
	}
	detail.ID = id
	detail.URL = pageURL
	return detail, nil
}

// query POSTs one Algolia query with the public credentials and required
// Referer, decoding the JSON response into out.
func (c *Client) query(ctx context.Context, params url.Values, out *searchResponse) error {
	body, err := json.Marshal(map[string]string{"params": params.Encode()})
	if err != nil {
		return err
	}

	endpoint := c.algoliaBaseURL + "/1/indexes/jobPosts/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Algolia-Application-Id", algoliaAppID)
	req.Header.Set("X-Algolia-API-Key", algoliaAPIKey)
	req.Header.Set("Referer", refererValue)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &apiErr) == nil && apiErr.Message != "" {
			return fmt.Errorf("algolia HTTP %d: %s", resp.StatusCode, apiErr.Message)
		}
		return fmt.Errorf("algolia HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("parse search response: %w", err)
	}
	return nil
}

// facetFilters renders opts' facet narrowing as Algolia's JSON-array
// syntax; top-level entries AND together. Empty when nothing is set.
func facetFilters(opts SearchOptions) string {
	var filters []string
	if opts.Filter != "" {
		filters = append(filters, "searchFilter:"+opts.Filter)
	}
	if opts.Region != "" {
		filters = append(filters, "applicantLocationRegions:"+opts.Region)
	}
	if len(filters) == 0 {
		return ""
	}
	b, _ := json.Marshal(filters)
	return string(b)
}
