package synopsys

import (
	"cmp"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const orgID = "44408"

type FacetFilter struct {
	ID        string
	FacetType int
	Display   string
	Count     int
	IsApplied bool
	FieldName string
}

type JobRequest struct {
	Keywords       string
	Location       string
	Page           int
	RecordsPerPage int
	SortCriteria   int // 0=Most Relevant, 13=Most Recent
	IsPagination   bool
	FacetFilters   []FacetFilter
}

type Job struct {
	Title     string
	Location  string
	Category  string
	Posted    string
	DisplayID string
	JobID     string // numeric ID in URL
	City      string // URL segment
	Slug      string // URL segment
}

func (j Job) URL() string {
	return "https://careers.synopsys.com/job/" + j.City + "/" + j.Slug + "/" + orgID + "/" + j.JobID
}

type SearchResults struct {
	TotalResults int
	TotalPages   int
	CurrentPage  int
	Jobs         []Job
	HasJobs      bool
	HasContent   bool
}

type JobDetail struct {
	Title          string
	DatePosted     string
	Locations      []string
	Category       string
	HireType       string
	RemoteEligible string
	Description    string
	DisplayID      string
}

func buildSearchQuery(p *JobRequest) url.Values {
	q := url.Values{
		"SearchType":              {"5"},
		"ResultsType":             {"0"},
		"SortDirection":           {"0"},
		"Distance":                {"50"},
		"RadiusUnitType":          {"0"},
		"ShowRadius":              {"False"},
		"SearchResultsModuleName": {"Search Results"},
		"SearchFiltersModuleName": {"Search Filters"},
	}
	if p.Keywords != "" {
		q.Set("Keywords", p.Keywords)
	}
	if p.Location != "" {
		q.Set("Location", p.Location)
	}
	q.Set("CurrentPage", strconv.Itoa(cmp.Or(p.Page, 1)))
	q.Set("RecordsPerPage", strconv.Itoa(cmp.Or(p.RecordsPerPage, 15)))
	q.Set("SortCriteria", strconv.Itoa(p.SortCriteria))
	if p.IsPagination {
		q.Set("IsPagination", "True")
	} else {
		q.Set("IsPagination", "False")
	}
	for i, f := range p.FacetFilters {
		prefix := fmt.Sprintf("FacetFilters[%d].", i)
		q.Set(prefix+"ID", f.ID)
		q.Set(prefix+"FacetType", strconv.Itoa(f.FacetType))
		q.Set(prefix+"Display", f.Display)
		q.Set(prefix+"Count", strconv.Itoa(f.Count))
		isApplied := "false"
		if f.IsApplied {
			isApplied = "true"
		}
		q.Set(prefix+"IsApplied", isApplied)
		q.Set(prefix+"FieldName", f.FieldName)
		if f.ID != "" {
			q.Set("ActiveFacetID", f.ID)
		}
	}
	return q
}

func FormatSearchResults(r *SearchResults) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d jobs (page %d/%d)\n\n", r.TotalResults, r.CurrentPage, r.TotalPages)
	for _, job := range r.Jobs {
		fmt.Fprintf(&sb, "[%s] %s\n", job.DisplayID, job.Title)
		fmt.Fprintf(&sb, "  Location: %s\n", job.Location)
		fmt.Fprintf(&sb, "  Category: %s\n", job.Category)
		fmt.Fprintf(&sb, "  Posted: %s\n", job.Posted)
		fmt.Fprintf(&sb, "  URL: %s\n", job.URL())
		sb.WriteByte('\n')
	}
	return sb.String()
}

func FormatJobDetail(d *JobDetail) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "=== %s ===\n", d.Title)
	if len(d.Locations) > 0 {
		fmt.Fprintf(&sb, "Location: %s\n", strings.Join(d.Locations, ", "))
	}
	fmt.Fprintf(&sb, "Posted: %s\n", d.DatePosted)
	if d.DisplayID != "" {
		fmt.Fprintf(&sb, "Job ID: %s\n", d.DisplayID)
	}
	if d.Category != "" {
		fmt.Fprintf(&sb, "Category: %s\n", d.Category)
	}
	if d.HireType != "" {
		fmt.Fprintf(&sb, "Hire Type: %s\n", d.HireType)
	}
	if d.RemoteEligible != "" {
		fmt.Fprintf(&sb, "Remote Eligible: %s\n", d.RemoteEligible)
	}
	fmt.Fprintf(&sb, "\n--- Job Description ---\n%s\n", d.Description)
	return sb.String()
}
