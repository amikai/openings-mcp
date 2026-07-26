package mokahr

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// DefaultBaseURL hosts every MokaHR careers site; tenants are path segments,
// not subdomains.
const DefaultBaseURL = "https://app.mokahr.com"

// defaultLocale is sent on every request. It selects the language of place
// names only, and picking one explicitly is what keeps the job endpoints and
// the filter endpoint from disagreeing: with no locale, jobs come back with
// Chinese place names while the aggregations come back with English ones.
const defaultLocale = "en-US"

// Error codes MokaHR returns inside an HTTP 200 body. There is no status-code
// signal at all, so callers distinguish outcomes on these.
const (
	// CodeSiteNotFound reports an orgId/siteId pair that names no careers site.
	CodeSiteNotFound = 703002
	// CodeJobNotFound reports a jobId that is unknown, closed, or belongs to
	// another site.
	CodeJobNotFound = 703015
)

// APIError is an unsuccessful MokaHR response. Msg is upstream's own wording
// and is written in Chinese.
type APIError struct {
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("mokahr: upstream error %d: %s", e.Code, e.Msg)
}

// JobsClient reads MokaHR careers sites. It exists so the response
// deobfuscation in [Transport] and the locale default cannot be forgotten,
// and so unsuccessful envelopes surface as Go errors rather than as an empty
// payload.
type JobsClient struct {
	api *Client
}

func NewJobsClient(baseURL string, hc *http.Client) (*JobsClient, error) {
	if hc == nil {
		hc = &http.Client{}
	}
	deobfuscating := *hc
	deobfuscating.Transport = Transport{Base: hc.Transport}
	api, err := NewClient(baseURL, WithClient(&deobfuscating))
	if err != nil {
		return nil, fmt.Errorf("mokahr: create api client: %w", err)
	}
	return &JobsClient{api: api}, nil
}

// ListJobs returns one page of a site's postings, each carrying its full HTML
// description. Set NeedStat to learn the board's total size.
func (c *JobsClient) ListJobs(ctx context.Context, req ListJobsRequest) (*JobList, error) {
	if !req.Locale.IsSet() {
		req.Locale = NewOptString(defaultLocale)
	}
	rsp, err := c.api.ListJobs(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("mokahr: list jobs for %s/%s: %w", req.OrgId, req.SiteId, err)
	}
	if !rsp.Success {
		return nil, &APIError{Code: rsp.Code, Msg: rsp.Msg.Or("")}
	}
	list, ok := rsp.Data.Get()
	if !ok {
		return nil, fmt.Errorf("mokahr: list jobs for %s/%s: successful response carried no data", req.OrgId, req.SiteId)
	}
	return &list, nil
}

// GetJob returns one posting with the fields the list endpoint omits, notably
// its department and employment type.
func (c *JobsClient) GetJob(ctx context.Context, req GetJobRequest) (*JobDetail, error) {
	if !req.Locale.IsSet() {
		req.Locale = NewOptString(defaultLocale)
	}
	rsp, err := c.api.GetJob(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("mokahr: get job %s: %w", req.JobId, err)
	}
	if !rsp.Success {
		return nil, &APIError{Code: rsp.Code, Msg: rsp.Msg.Or("")}
	}
	detail, ok := rsp.Data.Get()
	if !ok {
		return nil, fmt.Errorf("mokahr: get job %s: successful response carried no data", req.JobId)
	}
	return &detail, nil
}

// ListFilterAggregations returns the site's filterable dimensions: the
// locations and 职能 (job functions) it groups postings under, with the ids
// [JobsClient.ListJobs] accepts.
func (c *JobsClient) ListFilterAggregations(ctx context.Context, req SiteRef) (*FilterAggregations, error) {
	if !req.Locale.IsSet() {
		req.Locale = NewOptString(defaultLocale)
	}
	rsp, err := c.api.ListFilterAggregations(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("mokahr: list filters for %s/%s: %w", req.OrgId, req.SiteId, err)
	}
	if !rsp.Success {
		return nil, &APIError{Code: rsp.Code, Msg: rsp.Msg.Or("")}
	}
	aggs, ok := rsp.Data.Get()
	if !ok {
		return nil, fmt.Errorf("mokahr: list filters for %s/%s: successful response carried no data", req.OrgId, req.SiteId)
	}
	return &aggs, nil
}

// CareersURL returns a site's public job-listing page.
func CareersURL(orgID, siteID string) string {
	return fmt.Sprintf("%s/social-recruitment/%s/%s", DefaultBaseURL, url.PathEscape(orgID), url.PathEscape(siteID))
}

// JobURL returns a posting's public page. The site is a single-page app, so
// the posting is addressed in the fragment.
func JobURL(orgID, siteID, jobID string) string {
	return fmt.Sprintf("%s#/job/%s", CareersURL(orgID, siteID), url.PathEscape(jobID))
}
