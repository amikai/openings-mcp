package avature

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ErrJobNotFound indicates the requested job id redirected to the portal's
// Error page (Avature's response to unknown or expired ids).
var ErrJobNotFound = errors.New("avature: job not found")

// ErrCompanyNotFound indicates the portal path does not exist on the host
// (HTTP 404 on SearchJobs).
var ErrCompanyNotFound = errors.New("avature: company not found")

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// Client talks to one Avature career portal.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a client for one portal base URL without a locale
// segment (e.g. "https://koch.avature.net/careers").
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		httpClient: cmp.Or(httpClient, http.DefaultClient),
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// SearchRequest mirrors the SearchJobs query parameters.
type SearchRequest struct {
	// Search is the platform-level full-text query over titles and
	// descriptions. Empty lists the whole board.
	Search string
	// Offset is the zero-based index of the first job. The server honors
	// arbitrary offsets; the page size is portal-configured.
	Offset int
}

// SearchResponse holds one portal listing page.
type SearchResponse struct {
	Jobs []Job
	// Total comes from the "1-12 of 436 results" legend, or -1 when the
	// portal hides it.
	Total int
	// HasNext reports whether a next-page pagination link is present.
	HasNext bool
}

// Job is one listing item.
type Job struct {
	// ID is the numeric final segment of the posting URL — the only stable
	// job key.
	ID    string
	Title string
	// Location is best-effort per portal theme; empty when the theme
	// renders none that the parser recognizes.
	Location string
	// URL is the posting link as rendered (slug-full, locale-full).
	URL string
}

// Field is one label/value metadata pair from a detail page. Labels are
// portal-specific (e.g. "Location(s)", "Business Area", "Ref #").
type Field struct {
	Label string
	Value string
}

// JobDetailResponse is one parsed posting page.
type JobDetailResponse struct {
	ID     string
	Title  string
	Fields []Field
	// DescriptionHTML is the posting body: every details section minus the
	// label/value fields reported in Fields.
	DescriptionHTML string
	URL             string
}

// Location returns the first field whose label mentions "location".
func (d *JobDetailResponse) Location() string {
	for _, f := range d.Fields {
		if strings.Contains(strings.ToLower(f.Label), "location") {
			return f.Value
		}
	}
	return ""
}

// Company returns the portal's "Company" field, set by tenants that host
// several brands on one portal (e.g. Koch subsidiaries).
func (d *JobDetailResponse) Company() string {
	for _, f := range d.Fields {
		if strings.EqualFold(f.Label, "Company") {
			return f.Value
		}
	}
	return ""
}

// Search fetches one listing page.
func (c *Client) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	if req == nil {
		req = &SearchRequest{}
	}
	u, err := url.Parse(c.baseURL + "/SearchJobs")
	if err != nil {
		return nil, fmt.Errorf("parse base url %q: %w", c.baseURL, err)
	}
	q := u.Query()
	if s := strings.TrimSpace(req.Search); s != "" {
		q.Set("search", s)
	}
	if req.Offset > 0 {
		q.Set("jobOffset", strconv.Itoa(req.Offset))
	}
	u.RawQuery = q.Encode()

	doc, status, _, err := c.getHTML(ctx, u.String())
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	switch {
	case status == http.StatusNotFound:
		return nil, fmt.Errorf("search jobs: %w", ErrCompanyNotFound)
	case status == http.StatusAccepted:
		return nil, fmt.Errorf("search jobs: HTTP 202 bot challenge; this portal blocks non-browser clients")
	case status != http.StatusOK:
		return nil, fmt.Errorf("search jobs: HTTP %d", status)
	}
	return parseSearchHTML(doc), nil
}

// JobDetail expects a [Job.ID] returned by [Client.Search]. The slug
// segment of the live URL is cosmetic; this always uses "job".
func (c *Client) JobDetail(ctx context.Context, id string) (*JobDetailResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("avature: empty job id")
	}
	if _, err := strconv.Atoi(id); err != nil {
		return nil, fmt.Errorf("avature: job id %q must be numeric", id)
	}

	doc, status, finalURL, err := c.getHTML(ctx, c.JobURL(id))
	if err != nil {
		return nil, fmt.Errorf("job detail %q: %w", id, err)
	}
	// Unknown ids redirect to <base>/Error, which serves 404.
	if status == http.StatusNotFound || status == http.StatusGone ||
		(finalURL != nil && strings.HasSuffix(finalURL.Path, "/Error")) {
		return nil, fmt.Errorf("job detail %q: %w", id, ErrJobNotFound)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("job detail %q: HTTP %d", id, status)
	}

	detail, ok := parseJobDetailHTML(doc, id)
	if !ok {
		return nil, fmt.Errorf("job detail %q: unrecognized detail page", id)
	}
	detail.URL = c.JobURL(id)
	return detail, nil
}

// JobURL builds a canonical posting URL for id.
func (c *Client) JobURL(id string) string {
	return c.baseURL + "/JobDetail/job/" + id
}

// getHTML issues one GET, following redirects (locale-less URLs 302 to the
// portal's default locale). It returns the final response's document,
// status, and URL.
func (c *Client) getHTML(ctx context.Context, rawURL string) (*goquery.Document, int, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, resp.Request.URL, fmt.Errorf("read body: %w", err)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, resp.StatusCode, resp.Request.URL, fmt.Errorf("parse html: %w", err)
	}
	return doc, resp.StatusCode, resp.Request.URL, nil
}
