// Package ats unifies the ATS-backed providers (workday, greenhouse,
// lever, ashby) behind one company-parameterized search interface, so MCP
// clients name a company and never learn which ATS serves it.
package ats

import (
	"context"
	"time"
)

// PageSize is the fixed page size for every adapter. Workday caps limit at
// 20 on at least one tenant, so 20 is the largest safe uniform value.
const PageSize = 20

// clampPage and totalPages centralize the unified pagination contract —
// 1-based pages, fixed PageSize, ceil-div page count — so adapters can't
// drift on it.
func clampPage(p int) int { return max(p, 1) }

func totalPages(total int) int { return (total + PageSize - 1) / PageSize }

// isoDate renders the unified PostedAt format for upstreams that provide a
// real timestamp.
func isoDate(t time.Time) string { return t.UTC().Format("2006-01-02") }

// Adapter is one ATS's implementation of the unified search interface.
// Methods address a company by slug; slugs are declared by Roster() and
// indexed by Registry, so a slug that reaches an adapter is always one it
// declared.
type Adapter interface {
	// Name identifies the adapter ("workday", "greenhouse", "lever",
	// "ashby") in logs and error messages only; it never reaches tool
	// schemas.
	Name() string
	// Roster lists every curated company on this ATS.
	Roster() []CompanyInfo
	Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error)
	Filters(ctx context.Context, slug string) (FilterSet, error)
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
	Query    string              // keywords: titles, skills, tech — never locations
	Location string              // fuzzy text match; full-dump adapters special-case "remote" via their remote fields, workday matches location facet labels
	Filters  map[string][]string // escape hatch; keys/values come from Filters(); OR within a key, AND across keys
	Page     int                 // 1-based; values < 1 mean page 1
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
	JobID    string // provider-native id (workday externalPath, lever uuid, ashby id)
	Title    string
	Location string
	PostedAt string // ISO 8601 date where the upstream provides one; otherwise its raw text
	URL      string // human-clickable posting page
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
