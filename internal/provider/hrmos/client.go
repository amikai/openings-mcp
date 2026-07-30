package hrmos

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ErrNotFound is returned for an unknown tenant slug or job ID — both are a
// clean HTTP 404 upstream (fact 9).
var ErrNotFound = errors.New("hrmos: not found")

// maxEmptyPageGuard bounds AllJobs's page loop independently of the pager's
// advertised max page, in case a tenant's dump ever disagrees with its own
// pager (fact 14 says the pager is otherwise the reliable stop condition).
const maxEmptyPageGuard = 1000

// DefaultBaseURL is the public website base URL for HRMOS.
const DefaultBaseURL = "https://hrmos.co"

type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a Client. When httpClient is nil, http.DefaultClient is
// used. HRMOS needs no cookies, headers, or session warm-up (fact 10).
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, baseURL: baseURL}
}

// FacetGroup is one tenant-defined facet dimension from nav.sg-job-filters:
// Label is the group's <th> text (tenant-defined, varies — fact 4), Options
// are its non-"すべて" (all) values. A tenant may define zero groups.
type FacetGroup struct {
	Label   string
	Options []string
}

// Job is one search-result cassette.
type Job struct {
	ID string // e.g. "0000265" or "00000des01"; pass to Client.JobDetail
	// Title is the cassette's h2 text verbatim.
	Title string
	// Description is the cassette body's full job description as flattened
	// plain text (fact 2 — the list page carries the FULL JD, not a summary).
	Description string
	// Chips are every ul.sg-tags > li chip's text, in document order,
	// including the location chip. Facet classification happens one layer
	// up, where the tenant's label->group map is available.
	Chips []string
	// Location is the sg-tag-location chip's text verbatim: a street address,
	// sometimes prefixed with a bracketed office label and suffixed with
	// "他(N)" meaning N further worksites the list page does not name. It is
	// empty when the employer left the address blank, which happens — the
	// chip element is always present, its text is not always filled in.
	Location string
	URL      string
}

// JobsResponse is one search-results page.
type JobsResponse struct {
	// Total is the true match count across every page, read from
	// p.pg-count. It is 0 on a past-the-end page (fact 14), where p.pg-count
	// is absent.
	Total  int
	Jobs   []Job
	Facets []FacetGroup
	// MaxPage is the highest page number nav.pg-pagenation advertises. It is
	// 1 when the nav is absent (a single-page dump).
	MaxPage int
}

// jobsURL builds a tenant's jobs-listing URL. page <= 1 omits the query
// parameter, matching the site's own canonical page-1 URL.
func (c *Client) jobsURL(slug string, page int) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse url %q: %w", c.baseURL, err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/pages/" + slug + "/jobs"
	if page > 1 {
		q := u.Query()
		q.Set("page", fmt.Sprint(page))
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// Jobs returns one page of search results. A page past the last one returns
// zero jobs and Total 0, not an error (fact 14).
func (c *Client) Jobs(ctx context.Context, slug string, page int) (*JobsResponse, error) {
	rawURL, err := c.jobsURL(slug, page)
	if err != nil {
		return nil, fmt.Errorf("build jobs url: %w", err)
	}
	doc, err := c.getHTML(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("list jobs for %q page %d: %w", slug, page, err)
	}
	resp, err := parseJobsHTML(doc)
	if err != nil {
		return nil, fmt.Errorf("list jobs for %q page %d: %w", slug, page, err)
	}
	return resp, nil
}

// AllJobs fetches every page of a tenant's dump and concatenates the jobs.
// Facets come from page 1 only: nav.sg-job-filters is tenant-global, not
// page-scoped (fact 12). Total and MaxPage also come from page 1.
func (c *Client) AllJobs(ctx context.Context, slug string) (*JobsResponse, error) {
	first, err := c.Jobs(ctx, slug, 1)
	if err != nil {
		return nil, err
	}
	all := &JobsResponse{
		Total:   first.Total,
		Jobs:    first.Jobs,
		Facets:  first.Facets,
		MaxPage: first.MaxPage,
	}
	for page := 2; page <= first.MaxPage && page <= maxEmptyPageGuard; page++ {
		resp, err := c.Jobs(ctx, slug, page)
		if err != nil {
			return nil, err
		}
		// Empty-page guard (secondary bound, fact 14): stop even if MaxPage
		// claimed more, rather than trust a pager that disagrees with the
		// dump it describes.
		if len(resp.Jobs) == 0 {
			break
		}
		all.Jobs = append(all.Jobs, resp.Jobs...)
	}
	return all, nil
}

// JobDetailResponse is one posting's detail page: the JSON-LD JobPosting
// block plus both pg-descriptions tables (fact 15 — the job's own condition
// table and the employer's 会社情報 table), so no upstream field is dropped
// even when [ats.JobDetail] only surfaces a subset of it.
type JobDetailResponse struct {
	ID         string
	URL        string
	Title      string
	Company    string
	CompanyURL string
	// EmploymentType is schema.org's enum (e.g. FULL_TIME).
	EmploymentType string
	// DatePosted and ValidThrough are RFC 3339 timestamps as HRMOS writes
	// them (fractional seconds, "Z" suffix).
	DatePosted   string
	ValidThrough string
	Locations    []Location
	// SalaryCurrency/Min/Max/Unit come from JSON-LD baseSalary; all empty
	// when the tenant wrote 応相談 (negotiable) instead of a figure.
	SalaryCurrency string
	SalaryMin      string
	SalaryMax      string
	SalaryUnit     string
	// SalaryNote is the job-info table's 給与 row heading verbatim (e.g.
	// "応相談"), the only place that text survives when SalaryMin is empty.
	SalaryNote string
	// Description is the JSON-LD description, HTML flattened to plain text.
	Description string
	// JobInfo holds the first pg-descriptions table's other rows
	// (th text -> flattened td text), in document order.
	JobInfo []KV
	// CompanyInfo holds the second (会社情報) pg-descriptions table's rows,
	// in document order.
	CompanyInfo []KV
}

// KV is one label/value row from a pg-descriptions table.
type KV struct {
	Key   string
	Value string
}

// Location is one schema.org PostalAddress from a detail page's jobLocation.
type Location struct {
	PostalCode string
	Region     string // addressRegion, e.g. 東京都
	Locality   string // addressLocality, e.g. 港区 (visional prepends a
	// bracketed office label here instead, e.g. "［渋谷本社］東京都渋谷区" —
	// HRMOS's own data, passed through verbatim)
	Street string // streetAddress
}

// JobDetail fetches one posting. jobID is the value from a [Job.ID].
func (c *Client) JobDetail(ctx context.Context, slug, jobID string) (*JobDetailResponse, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse url %q: %w", c.baseURL, err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/pages/" + slug + "/jobs/" + jobID
	doc, err := c.getHTML(ctx, u.String())
	if err != nil {
		return nil, fmt.Errorf("job detail %q for %q: %w", jobID, slug, err)
	}
	detail, err := parseJobDetailHTML(doc, jobID, u.String())
	if err != nil {
		return nil, fmt.Errorf("job detail %q for %q: %w", jobID, slug, err)
	}
	return detail, nil
}

func (c *Client) getHTML(ctx context.Context, rawURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// continue
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}
