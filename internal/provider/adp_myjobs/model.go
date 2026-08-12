package adp_myjobs

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CareerSite is the public career-site configuration for one MyJobs slug.
// MyJobsToken is a short-lived public board session from the career SPA
// (not a partner/HCM credential).
type CareerSite struct {
	ClientName  string `json:"clientName"`
	Domain      string `json:"domain"`
	OrgOID      string `json:"orgoid"`
	MyJobsToken string `json:"myJobsToken"`
	Active      *bool  `json:"active"`
	Name        string `json:"name"`
}

// JobRequisition is one public requisition from the listings endpoint.
type JobRequisition struct {
	ReqID                any                   `json:"reqId"`
	JobTitle             string                `json:"jobTitle"`
	PublishedJobTitle    string                `json:"publishedJobTitle"`
	Type                 string                `json:"type"`
	JobDescription       string                `json:"jobDescription"`
	JobQualifications    string                `json:"jobQualifications"`
	WorkLevelCode        string                `json:"workLevelCode"`
	ClientRequisitionID  string                `json:"clientRequisitionID"`
	PostingDate          string                `json:"postingDate"`
	RequisitionLocations []RequisitionLocation `json:"requisitionLocations"`
}

// RequisitionLocation is a posting location row.
type RequisitionLocation struct {
	ItemID           string           `json:"itemID"`
	PrimaryIndicator bool             `json:"primaryIndicator"`
	Address          *LocationAddress `json:"address"`
	NameCode         *NameCode        `json:"nameCode"`
}

// LocationAddress holds geo fields from a requisition location.
type LocationAddress struct {
	CityName                 string         `json:"cityName"`
	CountrySubdivisionLevel1 *CodeVal       `json:"countrySubdivisionLevel1"`
	Country                  *CodeVal       `json:"country"`
	PostalCode               string         `json:"postalCode"`
	LineOne                  string         `json:"lineOne"`
	GeoCoordinate            *GeoCoordinate `json:"geoCoordinate"`
}

// GeoCoordinate is a posting location's WGS84 point. Boards that never
// geocoded a location report 0/0 rather than omitting it.
type GeoCoordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// CustomFilterCatalog is the tenant's own filter dimensions, as returned by
// [Client.GetCustomFilters].
type CustomFilterCatalog struct {
	FilterList []FilterCategory `json:"filterList"`
}

// FilterCategory is one filter dimension a tenant has configured.
//
// Category is an opaque slot code ("FIELD1".."FIELD5") that is positional, not
// semantic: FIELD3 is "Full-Time/Part-Time" on one board, "Area of Interest" on
// another and "Compensation Range" on a third, so it can only be interpreted
// through the catalog of the same slug. CategoryLabel is the tenant's display
// name and is not unique — a board may configure two dimensions that are both
// labelled "Location".
type FilterCategory struct {
	Category      string        `json:"category"`
	CategoryLabel string        `json:"categoryLabel"`
	FilterList    []FilterValue `json:"filterList"`
}

// FilterValue is one selectable value. Value is what a $filter must compare
// against, exactly and case-sensitively.
type FilterValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// CodeVal is a short code/name pair.
type CodeVal struct {
	CodeValue string `json:"codeValue"`
	ShortName string `json:"shortName"`
	LongName  string `json:"longName"`
}

// NameCode wraps a location alias name.
type NameCode struct {
	CodeValue string `json:"codeValue"`
	ShortName string `json:"shortName"`
	LongName  string `json:"longName"`
}

// ListResult is one page of job requisitions.
type ListResult struct {
	Count           int              `json:"count"`
	JobRequisitions []JobRequisition `json:"jobRequisitions"`
}

// searchMetaResult is the search-meta detail payload (field names differ from list).
type searchMetaResult struct {
	JobRequisitions []searchMetaJob `json:"jobRequisitions"`
}

type searchMetaJob struct {
	ItemID                 any                   `json:"itemID"`
	RequisitionTitle       string                `json:"requisitionTitle"`
	RequisitionDescription string                `json:"requisitionDescription"`
	ClientRequisitionID    string                `json:"clientRequisitionID"`
	Type                   string                `json:"type"`
	RequisitionLocations   []RequisitionLocation `json:"requisitionLocations"`
	PostingInstructions    []struct {
		TimestampLastPosted string `json:"timestampLastPosted"`
		PostDate            string `json:"postDate"`
	} `json:"postingInstructions"`
	ScreeningRequirements []struct {
		RequirementDescription string `json:"requirementDescription"`
	} `json:"screeningRequirements"`
}

func (j searchMetaJob) toJobRequisition(fallbackID string) JobRequisition {
	id := j.ItemID
	if id == nil || id == "" {
		id = fallbackID
	}
	desc := j.RequisitionDescription
	var quals strings.Builder
	for _, s := range j.ScreeningRequirements {
		if s.RequirementDescription == "" {
			continue
		}
		if quals.Len() > 0 {
			quals.WriteString("\n")
		}
		quals.WriteString(s.RequirementDescription)
	}
	posted := ""
	for _, pi := range j.PostingInstructions {
		if pi.TimestampLastPosted != "" {
			posted = pi.TimestampLastPosted
			break
		}
		if pi.PostDate != "" {
			posted = pi.PostDate
			break
		}
	}
	out := JobRequisition{
		ReqID:                id,
		JobTitle:             j.RequisitionTitle,
		PublishedJobTitle:    j.RequisitionTitle,
		Type:                 j.Type,
		JobDescription:       desc,
		JobQualifications:    quals.String(),
		ClientRequisitionID:  j.ClientRequisitionID,
		PostingDate:          posted,
		RequisitionLocations: j.RequisitionLocations,
	}
	return out
}

// ReqIDString normalizes reqId to a string job id.
func (j JobRequisition) ReqIDString() string {
	switch v := j.ReqID.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// Title prefers the published job title.
func (j JobRequisition) Title() string {
	if j.PublishedJobTitle != "" {
		return j.PublishedJobTitle
	}
	return j.JobTitle
}

// PrimaryLocation formats a human-readable location string.
func (j JobRequisition) PrimaryLocation() string {
	locs := j.RequisitionLocations
	if len(locs) == 0 {
		return ""
	}
	loc := &locs[0]
	for i := range locs {
		if locs[i].PrimaryIndicator {
			loc = &locs[i]
			break
		}
	}
	if loc.Address != nil {
		var parts []string
		if loc.Address.CityName != "" {
			parts = append(parts, loc.Address.CityName)
		}
		if s := codeString(loc.Address.CountrySubdivisionLevel1); s != "" {
			parts = append(parts, s)
		}
		if s := codeString(loc.Address.Country); s != "" {
			parts = append(parts, s)
		}
		if len(parts) > 0 {
			return strings.Join(parts, ", ")
		}
	}
	if loc.NameCode != nil {
		if loc.NameCode.ShortName != "" {
			return loc.NameCode.ShortName
		}
		return loc.NameCode.CodeValue
	}
	return ""
}

func codeString(c *CodeVal) string {
	if c == nil {
		return ""
	}
	if c.CodeValue != "" {
		return c.CodeValue
	}
	return c.ShortName
}

// PostedTime parses postingDate when possible.
func (j JobRequisition) PostedTime() (time.Time, bool) {
	if j.PostingDate == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, j.PostingDate); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ApplyURL builds the public apply/detail URL for a requisition.
func ApplyURL(slug, reqID string) string {
	return "https://myjobs.adp.com/" + slug + "/cx/job/" + reqID
}
