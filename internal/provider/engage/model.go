package engage

import "time"

// CategoryCap is the per-employment-category ceiling engage imposes on a
// tenant board (verified live across cookbiz_jobs, nkk-group_jobs,
// nova_career, aspark-tokyo). There is no pagination to raise it: a category
// that reaches CategoryCap is silently truncated, and jobs exist outside the
// returned id window (see doc.go).
const CategoryCap = 100

// Job is one listing on a tenant board (dd.dataArea entry).
type Job struct {
	// WorkID is the numeric id from the posting's /<slug>/work_<id>/ link,
	// the same value [Client.Job] takes.
	WorkID string
	Title  string
	// Salary and Area are the board's own display text (e.g. "年俸　3,600,000円～",
	// "北海道札幌市"), not normalized into a structured amount or address.
	Salary string
	Area   string
	// LastUpdated is parsed from the board's "最終更新日：2026/7/16" line. It is
	// the zero time when the line is missing or unparsable.
	LastUpdated time.Time
	// EmploymentType is the enclosing dt.category label (e.g. 中途採用,
	// アルバイト・パート採用) — the board's only filter dimension.
	EmploymentType string
}

// Category is one dt.category block: an employment type and the jobs listed
// under it.
type Category struct {
	Name string
	Jobs []Job
	// AtCap reports whether this category returned exactly [CategoryCap]
	// jobs. When true, the category's true size is larger than Jobs — the
	// board offers no way to see the rest (see doc.go).
	AtCap bool
}

// Board is one tenant's full listing page.
type Board struct {
	Slug       string
	Categories []Category
}

// Jobs flattens every category's jobs into one slice, in board order.
func (b *Board) Jobs() []Job {
	var all []Job
	for _, cat := range b.Categories {
		all = append(all, cat.Jobs...)
	}
	return all
}

// AnyAtCap reports whether any category hit [CategoryCap], meaning the board
// as a whole is a lower bound on the tenant's true job count.
func (b *Board) AnyAtCap() bool {
	for _, cat := range b.Categories {
		if cat.AtCap {
			return true
		}
	}
	return false
}

// Organization is a detail page's hiringOrganization.
type Organization struct {
	Name string
	// Logo and SameAs (the employer's own site) are best-effort: SameAs is
	// absent on some tenants (e.g. small storefronts with no company URL on
	// file).
	Logo   string
	SameAs string
}

// Salary is one baseSalary entry. engage's JSON-LD represents baseSalary as
// either a single object or, for tenants that publish both an annual and a
// monthly figure for the same posting, an array of them — see doc.go.
type Salary struct {
	Currency string
	// UnitText is schema.org's pay period (YEAR, MONTH, HOUR, ...).
	UnitText string
	// MinValue and MaxValue are the JSON-LD's own string-typed numbers,
	// passed through as-is (engage sometimes carries only MinValue).
	MinValue string
	MaxValue string
}

// Location is a detail page's jobLocation.address.
type Location struct {
	Country    string
	Region     string
	Locality   string
	Street     string
	PostalCode string
}

// Section is one h2.item / dd.data block on a detail page (仕事内容, 応募資格・条件,
// 勤務時間, ...). The JSON-LD description already concatenates every section
// engage renders, but callers that want them separately — or need a section
// the JSON-LD's own top-level fields don't surface — can read Sections
// directly instead of re-splitting Description.
type Section struct {
	Heading string
	Text    string
}

// JobDetail is one posting's detail page, built from its embedded
// schema.org JobPosting JSON-LD plus the HTML section fallback (see doc.go).
type JobDetail struct {
	WorkID          string
	Title           string
	DescriptionHTML string
	DatePosted      time.Time
	EmploymentType  string
	Organization    Organization
	Salaries        []Salary
	Location        Location
	DirectApply     bool
	Sections        []Section
}

// CompanyRecord is one aggregator search result, carrying a verified
// slug/name/id pair for roster curation. It is not a job listing in the
// adapter sense — see API.md.
type CompanyRecord struct {
	Slug string
	// Name and OfficialName are usually identical; OfficialName is the
	// aggregator's own "official_corporate_name" field, kept separate since
	// they diverge for a handful of tenants.
	Name         string
	OfficialName string
	CompanyID    int64
	WorkID       string
}

// CompaniesPage is one page of the aggregator search.
type CompaniesPage struct {
	// TotalCount is the aggregator's own count across all tenants, unrelated
	// to any one company. Callers stitching pages must pass it back as the
	// next call's prevTotal (see [Client.Companies]).
	TotalCount int
	Records    []CompanyRecord
	HasMore    bool
}
