package join

import "time"

// Job is one row from a company's job dump (search). It carries no
// description: publicJobs never populates descriptionHtml regardless of
// requested fields (see API.md) — fetch [Client.JobDetail] with IdParam
// for the full posting.
type Job struct {
	// IdParam is the value [Client.JobDetail] takes to fetch this job's
	// full posting. Not the same as the job's numeric database id — see
	// API.md's "Job identity" section.
	IdParam        string
	Title          string
	Status         string
	WorkplaceType  string // e.g. ONSITE, HYBRID, REMOTE; empty when unset upstream.
	RemoteType     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	City           string
	Country        string
	Category       string
	EmploymentType string
}

// JobsResponse is one page of a company's job dump.
type JobsResponse struct {
	Jobs      []Job
	Page      int
	PageCount int
	PageSize  int
	RowCount  int
}

// JobDetail is a full posting, scraped from the SSR job detail page's
// embedded __NEXT_DATA__ (see parse.go). Fields beyond Job mirror what the
// detail page alone carries.
type JobDetail struct {
	ID             int
	IdParam        string
	Title          string
	CompanyID      int
	WorkplaceType  string
	RemoteType     string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	City           string
	Country        string
	Category       string
	EmploymentType string
	// Description is the full posting body in Markdown, assembled from
	// intro/tasks/requirements/benefits/outro (legacy jobs) or the single
	// description field (unifiedDescription jobs) — see API.md.
	Description string
}

// Company is a resolved company identity, scraped from the SSR
// /companies/{slug} page's embedded __NEXT_DATA__. There is no public
// GraphQL field to resolve a slug to its numeric id (see API.md), so this
// is the only way to learn a company's id from its slug.
type Company struct {
	ID   int
	Name string
	Slug string
}
