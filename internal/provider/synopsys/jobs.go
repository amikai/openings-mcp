package synopsys

import (
	"cmp"
	"fmt"
	"net/url"
	"strconv"
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

type JobsRequest struct {
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

type JobsResponse struct {
	TotalResults int
	TotalPages   int
	CurrentPage  int
	Jobs         []Job
	HasJobs      bool
	HasContent   bool
}

type JobDetailResponse struct {
	Title          string
	DatePosted     string
	Locations      []string
	Category       string
	HireType       string
	RemoteEligible string
	Description    string
	DisplayID      string
}

func buildSearchQuery(p *JobsRequest) url.Values {
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

