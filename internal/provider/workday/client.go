package workday

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// TenantClient is a Workday CXS client that can query any confirmed tenant
// (see [CompaniesByTenant]) by slug, so callers only need one instance to
// use across all tenants and calls.
type TenantClient struct {
	client *Client
}

// NewTenantClient builds a TenantClient. opts configure it the same way as
// NewClient (timeouts, tracing, a custom http.Client, etc.).
func NewTenantClient(opts ...ClientOption) (*TenantClient, error) {
	c, err := NewClient("", opts...)
	if err != nil {
		return nil, err
	}
	return &TenantClient{
		client: c,
	}, nil
}

// serverURLByTenant resolves tenant to its confirmed Workday CXS base URL,
// erroring if tenant isn't a confirmed tenant (see CompaniesByTenant).
func serverURLByTenant(tenant string) (serverURL *url.URL, err error) {
	company, ok := CompaniesByTenant[strings.ToLower(tenant)]
	if !ok {
		return nil, fmt.Errorf("tenant %q not found", tenant)
	}
	u, err := url.Parse(company.BaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse base URL for tenant %q: %w", tenant, err)
	}
	return u, nil
}

// JobsByTenant searches jobs for tenant, a confirmed Workday tenant slug
// (see [CompaniesByTenant]). A posting's [JobSummary.ExternalPath] can be
// split by [JobDetailKeyFromPath] for [TenantClient.JobDetailByTenant].
// JobsByTenant errors if tenant isn't confirmed.
func (c *TenantClient) JobsByTenant(ctx context.Context, tenant string, request *JobsRequest) (*JobsResponse, error) {
	serverURL, err := serverURLByTenant(tenant)
	if err != nil {
		return nil, err
	}
	ctx = WithServerURL(ctx, serverURL)
	return c.client.SearchJobs(ctx, request)
}

// ToGetJobDetailParams returns [GetJobDetailParams] for the first posting in
// [JobsResponse.JobPostings] whose [JobSummary.ExternalPath] is set and
// splits into a valid {location}/{titleSlug} pair via
// [JobDetailKeyFromPath], skipping any that do not. It reports false if no
// posting qualifies.
func (rsp *JobsResponse) ToGetJobDetailParams() (GetJobDetailParams, bool) {
	for _, posting := range rsp.JobPostings {
		externalPath, ok := posting.ExternalPath.Get()
		if !ok {
			continue
		}
		location, titleSlug, ok := JobDetailKeyFromPath(externalPath)
		if !ok {
			continue
		}
		return GetJobDetailParams{Location: location, TitleSlug: titleSlug}, true
	}
	return GetJobDetailParams{}, false
}

// JobDetailByTenant fetches a single job posting for tenant, a confirmed
// Workday tenant slug (see [CompaniesByTenant]). location and titleSlug come
// from [JobDetailKeyFromPath] applied to [JobSummary.ExternalPath].
// JobDetailByTenant errors if tenant isn't confirmed.
func (c *TenantClient) JobDetailByTenant(ctx context.Context, tenant, location, titleSlug string) (*JobDetailResponse, error) {
	serverURL, err := serverURLByTenant(tenant)
	if err != nil {
		return nil, err
	}
	ctx = WithServerURL(ctx, serverURL)
	return c.client.GetJobDetail(ctx, GetJobDetailParams{Location: location, TitleSlug: titleSlug})
}
