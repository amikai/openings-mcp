package synopsys

import (
	"cmp"
	"fmt"
	"net/url"
	"strconv"
)

// orgID is Synopsys's company ID on the TalentBrew ATS platform (a multi-tenant
// system shared across many employers' career sites). It identifies which
// company's job data to query and is fixed, not user-supplied.
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
	Location       *Location
	Page           int
	RecordsPerPage int
	SortCriteria   int // 0=Most Relevant, 13=Most Recent
	IsPagination   bool
	FacetFilters   []FacetFilter
}

// Location filters search results to a geocoded place. Free-text alone does
// nothing on this API; all four fields must be sent together, resolved via
// Client.ResolveLocation. Build one from a LocationSuggestion with AsFilter.
type Location struct {
	Display   string // human-readable label, cosmetic only
	Latitude  float64
	Longitude float64
	Type      int    // suggestion's "lt" field
	Path      string // suggestion's "lp" field, dash-separated ancestor IDs
}

// LocationSuggestion is one geocoded candidate returned by
// Client.ResolveLocation for a partial place name typed by the user.
type LocationSuggestion struct {
	ID           int     `json:"id"`
	Value        string  `json:"value"`
	Latitude     float64 `json:"lat"`
	Longitude    float64 `json:"lon"`
	Type         int     `json:"type"`
	City         string  `json:"city"`
	Division1    string  `json:"division1"`
	Country      string  `json:"country"`
	Path         string  `json:"lp"`
	LocationType int     `json:"lt"`
	PostalCode   string  `json:"pc"`
}

// AsFilter converts a suggestion into the Location filter accepted by
// JobsRequest.
func (s LocationSuggestion) AsFilter() *Location {
	return &Location{
		Display:   s.Value,
		Latitude:  s.Latitude,
		Longitude: s.Longitude,
		Type:      s.LocationType,
		Path:      s.Path,
	}
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
	// Fixed params the TalentBrew /search-jobs/results endpoint expects on every
	// call, regardless of query; not user-controllable.
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
	if p.Location != nil {
		q.Set("Location", p.Location.Display)
		q.Set("Latitude", strconv.FormatFloat(p.Location.Latitude, 'f', -1, 64))
		q.Set("Longitude", strconv.FormatFloat(p.Location.Longitude, 'f', -1, 64))
		q.Set("LocationType", strconv.Itoa(p.Location.Type))
		q.Set("LocationPath", p.Location.Path)
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
		q.Set(prefix+"IsApplied", strconv.FormatBool(f.IsApplied))
		q.Set(prefix+"FieldName", f.FieldName)
		if f.ID != "" {
			q.Set("ActiveFacetID", f.ID)
		}
	}
	return q
}
