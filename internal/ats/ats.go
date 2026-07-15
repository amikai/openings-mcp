// Package ats unifies the ATS-backed providers behind one company-parameterized search interface, so MCP
// clients name a company and never learn which ATS serves it.
package ats

import (
	"context"
	"net/url"
	"time"
)

// pageSize is the fixed page size for every adapter. Workday caps limit at
// 20 on at least one tenant, so 20 is the largest safe uniform value.
const pageSize = 20

// Adapter is one ATS's implementation of the unified search interface.
// Methods address a company by slug: either one declared by Roster() and
// indexed by Registry, or one the adapter itself minted via
// ParseCareersURL for careers-URL input.
type Adapter interface {
	// Name of the adapter
	Name() string
	// Roster lists every curated company on this ATS.
	Roster() []CompanyInfo
	// ParseCareersURL recognizes a careers URL for this ATS and returns a slug
	// that can be passed to Search, Filters, and Detail. For curated companies,
	// it returns the roster slug. For example, Workday returns a canonical
	// careers URL for unlisted tenants, since a tenant name alone cannot
	// identify its site. It returns ("", false) when u is not a careers URL
	// for this ATS.
	ParseCareersURL(u *url.URL) (slug string, ok bool)
	// Search returns one page of jobs for the company identified by slug.
	Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error)
	// Filters returns the filter dimensions and values available for the company
	// identified by slug.
	Filters(ctx context.Context, slug string) (FilterSet, error)
	// Detail returns the full posting for jobID at the company identified by slug.
	Detail(ctx context.Context, slug, jobID string) (*JobDetail, error)
}

// CompanyInfo is one company as the registry sees it: enough to resolve a
// user-supplied name to (adapter, slug). Connection config (Workday
// tenant/instance/site etc.) stays inside each adapter, looked up by slug.
type CompanyInfo struct {
	Slug string // unique key; the provider roster's tenant/site/board slug
	Name string // display name; the resolver also matches on it
}

// SearchParams are the unified search inputs. Semantics are identical
// across adapters; how each maps them upstream is the adapter's business.
type SearchParams struct {
	// Query searches titles, skills, and technology. It excludes locations.
	Query string
	// Location performs a fuzzy text match. Full-dump adapters use their remote
	// fields for "remote".
	Location string
	// Filters uses keys and values returned by Filters(). Values within a key use
	// OR semantics; different keys use AND semantics.
	Filters FilterSet
	// Page is 1-based. Values below 1 request page 1.
	Page int
}

// SearchResult is one page of unified search results.
type SearchResult struct {
	Jobs       []JobSummary
	TotalCount int
	Page       int
	TotalPages int
}

// JobSummary carries summary fields only — full descriptions are Detail's
// job, keeping search responses small for the LLM.
type JobSummary struct {
	// JobID is the provider-native ID, such as a Workday externalPath, Lever
	// UUID, or Ashby ID.
	JobID    string
	Title    string
	Location string
	// PostedAt is an ISO 8601 date when the upstream provides one; otherwise,
	// it is the upstream's raw text.
	PostedAt string
	// URL is the human-clickable posting page.
	URL string
}

// FilterSet maps a filter dimension to its currently valid values, as
// display labels. Tenant-specific and discovered at call time.
type FilterSet map[string][]string

// JobDetail is one full posting, description normalized to plain text.
type JobDetail struct {
	JobID       string
	Title       string
	Company     string
	Location    string
	PostedAt    string
	URL         string
	Description string
}

// clampPage and totalPages enforce 1-based pages, pageSize, and ceil-div
// totals.
func clampPage(p int) int { return max(p, 1) }

func totalPages(total int) int { return (total + pageSize - 1) / pageSize }

// isoDate renders the unified PostedAt format for upstreams that provide a
// real timestamp.
func isoDate(t time.Time) string { return t.UTC().Format("2006-01-02") }
