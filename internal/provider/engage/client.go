package engage

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// ErrBoardNotFound indicates the tenant slug does not exist (upstream 404 on
// GET /<slug>/).
var ErrBoardNotFound = errors.New("engage: board not found")

// ErrJobNotFound indicates the work_id does not exist on the given tenant
// (upstream 404 on GET /<slug>/work_<id>/).
var ErrJobNotFound = errors.New("engage: job not found")

// ErrEmptyBoard indicates a 200 board response parsed to zero jobs. engage
// serves no empty boards (verified across 641 slugs drawn from its company
// sitemap — see doc.go), so this means selector drift, not an empty tenant.
var ErrEmptyBoard = errors.New("engage: board parsed to zero jobs (selector drift suspected)")

// Client talks to en-gage.net's public tenant boards, tenant detail pages,
// and the cross-tenant aggregator search (roster tooling only — see
// API.md).
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient builds a Client against baseURL (e.g. "https://en-gage.net").
// When httpClient is nil, http.DefaultClient is used.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: cmp.Or(httpClient, http.DefaultClient),
	}
}

// Board fetches slug's tenant career page and parses every category's
// listings. Each category is capped at [CategoryCap] jobs — see
// [Category.AtCap] and doc.go. A 200 response that parses to zero jobs
// returns [ErrEmptyBoard] rather than an empty Board.
func (c *Client) Board(ctx context.Context, slug string) (*Board, error) {
	doc, status, err := c.getHTML(ctx, c.boardURL(slug))
	if err != nil {
		return nil, fmt.Errorf("engage: board %q: %w", slug, err)
	}
	if status == http.StatusNotFound {
		return nil, fmt.Errorf("engage: board %q: %w", slug, ErrBoardNotFound)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("engage: board %q: HTTP %d", slug, status)
	}
	board, err := parseBoard(doc)
	if err != nil {
		return nil, fmt.Errorf("engage: board %q: %w", slug, err)
	}
	board.Slug = slug
	return board, nil
}

// Job fetches one posting's detail page and parses its embedded schema.org
// JobPosting JSON-LD, falling back to the page's own section headings for
// fields the JSON-LD omits (see doc.go).
func (c *Client) Job(ctx context.Context, slug, workID string) (*JobDetail, error) {
	workID = strings.TrimSpace(workID)
	if workID == "" {
		return nil, errors.New("engage: empty work id")
	}
	doc, status, err := c.getHTML(ctx, c.jobURL(slug, workID))
	if err != nil {
		return nil, fmt.Errorf("engage: job %q/%q: %w", slug, workID, err)
	}
	if status == http.StatusNotFound {
		return nil, fmt.Errorf("engage: job %q/%q: %w", slug, workID, ErrJobNotFound)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("engage: job %q/%q: HTTP %d", slug, workID, status)
	}
	detail, err := parseJobDetail(doc)
	if err != nil {
		return nil, fmt.Errorf("engage: job %q/%q: %w", slug, workID, err)
	}
	if detail.WorkID == "" {
		detail.WorkID = workID
	}
	return detail, nil
}

// Companies pages the cross-tenant aggregator search, returning verified
// slug/name/id triples for roster curation — it is not a per-company job
// search (see API.md). page is 1-indexed. prevTotal must be the TotalCount
// observed on the previous call (0 for the first page): engage's own
// pagination requires it to reproduce the same result set on later pages.
func (c *Client) Companies(ctx context.Context, page, prevTotal int) (*CompaniesPage, error) {
	if page < 1 {
		page = 1
	}
	u := fmt.Sprintf("%s/user/api/search/result_work_list/?p_t=%d&f_t=0&page=%d", c.baseURL, prevTotal, page)
	body, status, err := c.getJSON(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("engage: companies page %d: %w", page, err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("engage: companies page %d: HTTP %d", page, status)
	}
	var wire aggregatorResponse
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("engage: companies page %d: decode response: %w", page, err)
	}
	if wire.Result != "success" {
		return nil, fmt.Errorf("engage: companies page %d: result %q: %s", page, wire.Result, wire.ErrorMessage)
	}
	records := make([]CompanyRecord, 0, len(wire.SearchResult))
	for _, r := range wire.SearchResult {
		records = append(records, CompanyRecord{
			Slug:         r.CompanyURLRootDir,
			Name:         r.CompanyName,
			OfficialName: r.OfficialCorporateName,
			CompanyID:    r.CompanyID,
			WorkID:       strconv.FormatInt(r.WorkID, 10),
		})
	}
	return &CompaniesPage{
		TotalCount: wire.TotalCount,
		Records:    records,
		HasMore:    wire.Paginator.HasMorePages,
	}, nil
}

func (c *Client) boardURL(slug string) string {
	return c.baseURL + "/" + slug + "/"
}

func (c *Client) jobURL(slug, workID string) string {
	return c.baseURL + "/" + slug + "/work_" + workID + "/"
}

// getHTML issues one GET and parses the body as HTML regardless of status
// code, so callers can inspect error pages (e.g. the 404 body) if needed.
func (c *Client) getHTML(ctx context.Context, rawURL string) (*goquery.Document, int, error) {
	body, status, err := c.get(ctx, rawURL, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if err != nil {
		return nil, 0, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, status, fmt.Errorf("parse html: %w", err)
	}
	return doc, status, nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string) ([]byte, int, error) {
	return c.get(ctx, rawURL, "application/json")
}

func (c *Client) get(ctx context.Context, rawURL, accept string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read body: %w", err)
	}
	return body, resp.StatusCode, nil
}

// aggregatorResponse is the result_work_list wire shape, trimmed to the
// fields this package reads — see API.md.
type aggregatorResponse struct {
	Result       string `json:"result"`
	ErrorMessage string `json:"error_message"`
	TotalCount   int    `json:"totalCount"`
	Paginator    struct {
		HasMorePages bool `json:"hasMorePages"`
	} `json:"paginator"`
	SearchResult []struct {
		WorkID                int64  `json:"work_id"`
		CompanyID             int64  `json:"company_id"`
		CompanyURLRootDir     string `json:"company_url_root_dir"`
		CompanyName           string `json:"company_name"`
		OfficialCorporateName string `json:"official_corporate_name"`
	} `json:"searchResult"`
}
