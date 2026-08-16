package join

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Khan/genqlient/graphql"
	"github.com/PuerkitoBio/goquery"
)

const (
	graphqlPath = "/candidate-api/graphql"
	// dumpPageSize is generous enough to cover a curated company's whole
	// board in one call for the common case; Jobs still loops on
	// pageInfo.pageCount rather than assuming a single call always
	// suffices (see API.md's Pagination note — no per-request cap was
	// observed, but nothing guarantees one doesn't exist).
	dumpPageSize = 100
)

// ErrNotFound is returned by JobDetail for a nonexistent job or company
// slug (upstream 404) and by ResolveCompany for a nonexistent slug.
var ErrNotFound = errors.New("join: not found")

var userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Client talks to join.com's public candidate-facing surfaces: the
// unauthenticated GraphQL search endpoint and the SSR HTML pages scraped
// for job/company detail (see API.md).
type Client struct {
	baseURL    string
	httpClient *http.Client
	gql        graphql.Client
}

// NewClient builds a Client against baseURL (e.g. "https://join.com").
// When httpClient is nil, http.DefaultClient is used.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		gql:        graphql.NewClient(baseURL+graphqlPath, httpClient),
	}
}

// Jobs returns every job in companyID's board, looping pages until
// pageInfo.pageCount is exhausted. There is no server-side keyword search
// (see API.md); callers filter the returned dump themselves.
func (c *Client) Jobs(ctx context.Context, companyID int) ([]Job, error) {
	var all []Job
	page := 1
	for {
		wire, err := GetCompanyJobs(ctx, c.gql, companyID, page, dumpPageSize)
		if err != nil {
			return nil, fmt.Errorf("join: list jobs for company %d: %w", companyID, err)
		}
		res := wire.PublicJobs
		for _, item := range res.Items {
			all = append(all, jobFromWire(item))
		}
		if page >= res.PageInfo.PageCount {
			break
		}
		page++
	}
	return all, nil
}

// jobFromWire maps a search result. City/Country/Category/EmploymentType
// are nullable upstream (e.g. a fully remote job may carry no city); a
// null one decodes as its Go zero value, so an empty field here means
// "not set upstream," not a parse failure.
func jobFromWire(j GetCompanyJobsPublicJobsPublicJobsResultItemsPublicJob) Job {
	return Job{
		IdParam:        j.IdParam,
		Title:          j.Title,
		Status:         j.Status,
		WorkplaceType:  j.WorkplaceType,
		RemoteType:     j.RemoteType,
		CreatedAt:      parseJoinTime(j.CreatedAt),
		UpdatedAt:      parseJoinTime(j.UpdatedAt),
		City:           j.City.CityName,
		Country:        j.Country.Name,
		Category:       j.Category.Name,
		EmploymentType: j.EmploymentType.Name,
	}
}

// JobDetail fetches the full posting at /companies/{slug}/{idParam}. slug
// and idParam must come from the same roster entry / [Client.Jobs] result
// pair Search used — see API.md's "Job identity" section for why idParam,
// not the job's numeric id, is the value to pass here.
func (c *Client) JobDetail(ctx context.Context, slug, idParam string) (*JobDetail, error) {
	if idParam == "" {
		return nil, errors.New("join: empty job idParam")
	}
	doc, err := c.getHTML(ctx, c.baseURL+"/companies/"+slug+"/"+idParam)
	if err != nil {
		return nil, fmt.Errorf("join: job detail %q for %q: %w", idParam, slug, err)
	}
	detail, err := parseJobDetailHTML(doc)
	if err != nil {
		return nil, fmt.Errorf("join: job detail %q for %q: %w", idParam, slug, err)
	}
	return detail, nil
}

// ResolveCompany scrapes /companies/{slug} for the company's numeric id and
// canonical name. There is no GraphQL field for this lookup (see API.md);
// roster curation is the intended caller, not the request-time adapter path.
func (c *Client) ResolveCompany(ctx context.Context, slug string) (*Company, error) {
	doc, err := c.getHTML(ctx, c.baseURL+"/companies/"+slug)
	if err != nil {
		return nil, fmt.Errorf("join: resolve company %q: %w", slug, err)
	}
	company, err := parseCompanyHTML(doc)
	if err != nil {
		return nil, fmt.Errorf("join: resolve company %q: %w", slug, err)
	}
	return company, nil
}

func (c *Client) getHTML(ctx context.Context, rawURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}
