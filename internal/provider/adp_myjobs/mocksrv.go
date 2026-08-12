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

// Coordinates of the mock board's geocoded locations, exported so tests can
// build boxes around them without restating literals.
const (
	MockLanghorneLat = 40.177719
	MockLanghorneLon = -74.890955
	MockOremLat      = 40.2968979
	MockOremLon      = -111.6946475
)

// mockBoard is the tiny board the mock server filters over. The third job's
// location carries the 0/0 placeholder MyJobs stores for locations it never
// geocoded, which no box can match.
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
					GeoCoordinate:            &GeoCoordinate{Latitude: MockLanghorneLat, Longitude: MockLanghorneLon},
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
					GeoCoordinate:            &GeoCoordinate{Latitude: MockOremLat, Longitude: MockOremLon},
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
					CityName:                 "Ungeocoded City",
					CountrySubdivisionLevel1: &CodeVal{CodeValue: "NE"},
					Country:                  &CodeVal{CodeValue: "USA"},
					GeoCoordinate:            &GeoCoordinate{Latitude: 0, Longitude: 0},
				},
				NameCode: &NameCode{CodeValue: "3", LongName: "GC - Ungeocoded-3"},
			}},
		},
	}
}

// mockGeoBoxRE extracts the corners of the one $filter shape upstream honors.
// The literal "undefined" tokens are part of that shape; see [GeoBox.Filter].
var mockGeoBoxRE = regexp.MustCompile(
	`geo\.intersects\(workLocations\.geoLocation, geography'POLYGON\(\(undefined, ` +
		`(-?[0-9.]+) (-?[0-9.]+), undefined, (-?[0-9.]+) (-?[0-9.]+)\)\)'\)`,
)

// NewMockServer serves career-site, list ($search + geo $filter), and search-meta.
//
// The list handler reproduces the two upstream behaviours that make a wrong
// location filter hard to notice: a $filter it does not recognize as a geo box
// is answered with HTTP 200 and the whole board, and a zero-area box is
// answered with HTTP 500.
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
			box, ok := parseMockGeoBox(filter)
			switch {
			case !ok:
				// Upstream ignores any other $filter without complaining.
			case box.West >= box.East || box.South >= box.North:
				http.Error(w, "invalid polygon", http.StatusInternalServerError)
				return
			default:
				jobs = filterMockByBox(jobs, box)
			}
		}

		total := len(jobs)
		serveJSON(w, http.StatusOK, mustJSON(ListResult{
			Count:           total,
			JobRequisitions: pageMockJobs(jobs, q.Get("$skip"), q.Get("$top")),
		}))
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

func filterMockByBox(jobs []JobRequisition, box GeoBox) []JobRequisition {
	out := make([]JobRequisition, 0, len(jobs))
	for _, j := range jobs {
		for _, loc := range j.RequisitionLocations {
			if loc.Address == nil {
				continue
			}
			lat, lon, ok := loc.Address.GeoCoordinate.Point()
			if !ok {
				continue
			}
			if lon >= box.West && lon <= box.East && lat >= box.South && lat <= box.North {
				out = append(out, j)
				break
			}
		}
	}
	return out
}

func parseMockGeoBox(filter string) (GeoBox, bool) {
	m := mockGeoBoxRE.FindStringSubmatch(filter)
	if m == nil {
		return GeoBox{}, false
	}
	nums := make([]float64, 4)
	for i := range nums {
		v, err := strconv.ParseFloat(m[i+1], 64)
		if err != nil {
			return GeoBox{}, false
		}
		nums[i] = v
	}
	return GeoBox{West: nums[0], South: nums[1], East: nums[2], North: nums[3]}, true
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
