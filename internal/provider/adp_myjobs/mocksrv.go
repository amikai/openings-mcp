package adp_myjobs

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
)

// MockSlug is the Guitar Center tenant captured in testdata.
const MockSlug = "guitarcenterexternal"

// MockSecondSlug is the Church Mutual tenant config fixture.
const MockSecondSlug = "churchmutual"

// MockUnknownSlug is a non-existent career-site slug (HTTP 400 upstream).
const MockUnknownSlug = "this-company-does-not-exist-xyz-openings-mcp"

//go:embed testdata/career_site_guitarcenterexternal.json
var mockCareerSiteGC []byte

//go:embed testdata/career_site_churchmutual.json
var mockCareerSiteCM []byte

//go:embed testdata/career_site_unknown_rsp.json
var mockCareerSiteUnknown []byte

// mockBoard is the tiny board the mock server filters over.
func mockBoard() []JobRequisition {
	return []JobRequisition{
		{
			ReqID:             "1001",
			JobTitle:          "Engineer A",
			PublishedJobTitle: "Engineer A",
			JobDescription:    "<p>A</p>",
			WorkLevelCode:     "Full Time",
			PostingDate:       "2026-01-01",
			RequisitionLocations: []RequisitionLocation{{
				ItemID:           "loc-1",
				PrimaryIndicator: true,
				Address: &LocationAddress{
					CityName:                 "Langhorne",
					CountrySubdivisionLevel1: &CodeVal{CodeValue: "PA"},
					Country:                  &CodeVal{CodeValue: "USA"},
					GeoCoordinate:            &GeoCoordinate{Latitude: 40.177719, Longitude: -74.890955},
				},
				NameCode: &NameCode{CodeValue: "1", LongName: "GC - Langhorne-1"},
			}},
		},
		{
			ReqID:             "1002",
			JobTitle:          "Music Teacher Store 1",
			PublishedJobTitle: "Music Teacher Store 1",
			JobDescription:    "<p>Teach music</p>",
			WorkLevelCode:     "Part Time",
			PostingDate:       "2026-01-02",
			RequisitionLocations: []RequisitionLocation{{
				ItemID:           "loc-2",
				PrimaryIndicator: true,
				Address: &LocationAddress{
					CityName:                 "Orem",
					CountrySubdivisionLevel1: &CodeVal{CodeValue: "UT"},
					Country:                  &CodeVal{CodeValue: "USA"},
					GeoCoordinate:            &GeoCoordinate{Latitude: 40.2968979, Longitude: -111.6946475},
				},
				NameCode: &NameCode{CodeValue: "2", LongName: "GC - Orem-2"},
			}},
		},
		{
			ReqID:             "1003",
			JobTitle:          "Support Specialist",
			PublishedJobTitle: "Support Specialist",
			JobDescription:    "<p>Support</p>",
			WorkLevelCode:     "Full Time",
			PostingDate:       "2026-01-03",
			RequisitionLocations: []RequisitionLocation{{
				ItemID:           "loc-3",
				PrimaryIndicator: true,
				Address: &LocationAddress{
					CityName:                 "Merrill",
					CountrySubdivisionLevel1: &CodeVal{CodeValue: "NE"},
					Country:                  &CodeVal{CodeValue: "USA"},
					GeoCoordinate:            &GeoCoordinate{Latitude: 45.180, Longitude: -89.683},
				},
				NameCode: &NameCode{CodeValue: "3", LongName: "GC - Merrill-3"},
			}},
		},
	}
}

// mockCustomFilterRE matches one "FIELDn eq 'value'" clause, the only $filter
// shape upstream honors. Clauses are ANDed with "&&"; see [CustomFilter].
var mockCustomFilterRE = regexp.MustCompile(`^(FIELD[0-9]+) eq '(.*)'$`)

// mockFacets is the catalog the mock board files its jobs under. FIELD1 is
// deliberately not "location": the slot codes are positional, so nothing may
// assume a fixed meaning for them.
func mockFacets() CustomFilterCatalog {
	return CustomFilterCatalog{FilterList: []FilterCategory{
		{Category: "FIELD1", CategoryLabel: "State", FilterList: []FilterValue{
			{Value: "Pennsylvania", Label: "Pennsylvania"},
			{Value: "Utah", Label: "Utah"},
		}},
		{Category: "FIELD2", CategoryLabel: "Position Type", FilterList: []FilterValue{
			{Value: "Full-time", Label: "Full-time"},
			{Value: "Part-time", Label: "Part-time"},
		}},
		// A second dimension sharing a label, as Guitar Center configures.
		{Category: "FIELD3", CategoryLabel: "State", FilterList: []FilterValue{
			{Value: "Nebraska", Label: "Nebraska"},
		}},
	}}
}

// mockJobFacets maps each mock job to its facet values, keyed by slot code.
func mockJobFacets() map[string]map[string]string {
	return map[string]map[string]string{
		"1001": {"FIELD1": "Pennsylvania", "FIELD2": "Full-time"},
		"1002": {"FIELD1": "Utah", "FIELD2": "Part-time"},
		"1003": {"FIELD3": "Nebraska", "FIELD2": "Full-time"},
	}
}

// NewMockServer serves career-site, the custom-filter catalog, list ($search +
// custom-filter $filter), and search-meta.
//
// The list handler reproduces the upstream behaviour that makes a wrong filter
// hard to notice: a $filter naming a slot code the board has not configured is
// answered with HTTP 200 and the whole board, not an error. A value whose case
// is wrong simply matches nothing, exactly as upstream answers it.
func NewMockServer() *httptest.Server {
	const detail = `{"jobRequisitions":[{
		"itemID":"1002",
		"requisitionTitle":"Music Teacher Store 1",
		"requisitionDescription":"<p>Teach music full description</p>",
		"clientRequisitionID":"c1",
		"requisitionLocations":[],
		"postingInstructions":[{"timestampLastPosted":"2026-01-02T00:00:00Z"}]
	}]}`

	mux := http.NewServeMux()
	mux.HandleFunc("/public/staffing/v1/career-site/", func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimPrefix(r.URL.Path, "/public/staffing/v1/career-site/")
		slug = strings.Trim(slug, "/")
		switch strings.ToLower(slug) {
		case MockSlug:
			serveJSON(w, http.StatusOK, mockCareerSiteGC)
		case MockSecondSlug:
			serveJSON(w, http.StatusOK, mockCareerSiteCM)
		default:
			serveJSON(w, http.StatusBadRequest, mockCareerSiteUnknown)
		}
	})
	mux.HandleFunc("/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("myjobstoken") == "" || r.Header.Get("rolecode") == "" {
			http.Error(w, "bad headers", http.StatusBadRequest)
			return
		}
		q := r.URL.Query()

		jobs := mockBoard()
		if search := strings.ToLower(strings.TrimSpace(q.Get("$search"))); search != "" {
			jobs = filterMockBySearch(jobs, search)
		}
		if filter := q.Get("$filter"); filter != "" {
			jobs = filterMockByCustomFilters(jobs, filter)
		}

		total := len(jobs)
		serveJSON(w, http.StatusOK, mustJSON(ListResult{
			Count:           total,
			JobRequisitions: pageMockJobs(jobs, q.Get("$skip"), q.Get("$top")),
		}))
	})
	mux.HandleFunc("/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/search-custom-filters", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("myjobstoken") == "" || r.Header.Get("rolecode") == "" {
			http.Error(w, "bad headers", http.StatusBadRequest)
			return
		}
		serveJSON(w, http.StatusOK, mustJSON(mockFacets()))
	})
	mux.HandleFunc("/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/search-meta/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("myjobstoken") == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		serveJSON(w, http.StatusOK, []byte(detail))
	})
	return httptest.NewServer(mux)
}

// NewTinyBoardMockServer is an alias of NewMockServer for older tests.
func NewTinyBoardMockServer() *httptest.Server { return NewMockServer() }

func filterMockBySearch(jobs []JobRequisition, needle string) []JobRequisition {
	out := make([]JobRequisition, 0, len(jobs))
	for _, j := range jobs {
		if strings.Contains(strings.ToLower(j.Title()), needle) {
			out = append(out, j)
		}
	}
	return out
}

// filterMockByCustomFilters applies every clause it recognizes. A clause naming
// an unconfigured slot code is ignored, which is what upstream does.
func filterMockByCustomFilters(jobs []JobRequisition, filter string) []JobRequisition {
	configured := make(map[string]bool)
	for _, c := range mockFacets().FilterList {
		configured[c.Category] = true
	}
	byJob := mockJobFacets()

	for _, clause := range strings.Split(filter, " && ") {
		m := mockCustomFilterRE.FindStringSubmatch(strings.TrimSpace(clause))
		if m == nil || !configured[m[1]] {
			continue
		}
		field, value := m[1], m[2]
		kept := make([]JobRequisition, 0, len(jobs))
		for _, j := range jobs {
			if byJob[j.ReqIDString()][field] == value {
				kept = append(kept, j)
			}
		}
		jobs = kept
	}
	return jobs
}

func pageMockJobs(jobs []JobRequisition, skipRaw, topRaw string) []JobRequisition {
	skip, _ := strconv.Atoi(skipRaw)
	if skip < 0 {
		skip = 0
	}
	if skip >= len(jobs) {
		return nil
	}
	jobs = jobs[skip:]
	if top, err := strconv.Atoi(topRaw); err == nil && top > 0 && top < len(jobs) {
		jobs = jobs[:top]
	}
	return jobs
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func serveJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
