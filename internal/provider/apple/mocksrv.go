package apple

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
)

const (
	mockCSRFToken            = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	mockSearchKeyword        = "software engineer"
	mockFilteredKeyword      = "engineer"
	mockMultiLocationKeyword = "distributed engineer"
	mockSearchLocation       = "postLocation-TWN"
	mockFilteredLocation     = "postLocation-USA"
	MockJobID                = "200624996"
	MockNotFoundJobID        = "999999999"
)

// mockFilteredFilters is the exact filter set captured in
// testdata/jobs_filtered_req.hurl; the mock search endpoint only serves the
// filtered fixture for a byte-identical filter payload.
var mockFilteredFilters = mockSearchFilters{
	Locations: []string{mockFilteredLocation},
	Keywords:  []string{"camera"},
	Teams:     []mockTeamFilter{{Team: "teamsAndSubTeams-HRDWR", SubTeam: "subTeam-CAM"}},
	Products:  []string{"productsAndServices-IPHN"},
	Languages: []string{"language-en_US"},
}

// mockMultiLocationFilters exercises a request combining more than one
// location ID at different granularities (state and city), reusing the
// jobs_rsp.json fixture since only the request plumbing is under test.
var mockMultiLocationFilters = mockSearchFilters{
	Locations: []string{"postLocation-TPEI", "postLocation-NTC9"},
}

//go:embed testdata/jobs_rsp.json
var mockJobsResponse []byte

//go:embed testdata/jobs_filtered_rsp.json
var mockFilteredJobsResponse []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailResponse []byte

//go:embed testdata/job_detail_not_found_rsp.json
var mockJobDetailNotFoundResponse []byte

//go:embed testdata/teams_rsp.json
var mockTeamsResponse []byte

// NewMockServer returns an httptest.Server that replays captured Apple Jobs
// responses, including the CSRF header and session-cookie search contract.
// The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/CSRFToken", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("x-apple-csrf-token", mockCSRFToken)
		http.SetCookie(w, &http.Cookie{
			Name:     "jssid",
			Value:    "fixture-session",
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("jssid")
		if err != nil || cookie.Value != "fixture-session" || r.Header.Get("x-apple-csrf-token") != mockCSRFToken {
			serveMockJSON(w, 436, mockJobDetailNotFoundResponse)
			return
		}
		fixture, ok := searchFixture(r)
		if !ok {
			serveMockJSON(w, 436, mockJobDetailNotFoundResponse)
			return
		}
		serveMockJSON(w, http.StatusOK, fixture)
	})
	mux.HandleFunc("GET /api/v1/refData/teamsofinterest", func(w http.ResponseWriter, _ *http.Request) {
		serveMockJSON(w, http.StatusOK, mockTeamsResponse)
	})
	mux.HandleFunc("GET /api/v1/jobDetails/{jobId}", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("locale") != "en-us" || r.PathValue("jobId") == MockNotFoundJobID {
			serveMockJSON(w, http.StatusNotFound, mockJobDetailNotFoundResponse)
			return
		}
		serveMockJSON(w, http.StatusOK, mockJobDetailResponse)
	})
	return httptest.NewTLSServer(mux)
}

type mockDateFormat struct {
	LongDate   string `json:"longDate"`
	MediumDate string `json:"mediumDate"`
}

type mockTeamFilter struct {
	Team    string `json:"team"`
	SubTeam string `json:"subTeam"`
}

type mockSearchFilters struct {
	Locations  []string         `json:"locations"`
	HomeOffice *bool            `json:"homeOffice"`
	Keywords   []string         `json:"keywords"`
	Teams      []mockTeamFilter `json:"teams"`
	Products   []string         `json:"products"`
	Languages  []string         `json:"languages"`
}

type mockSearchRequest struct {
	Query   string            `json:"query"`
	Locale  string            `json:"locale"`
	Sort    string            `json:"sort"`
	Format  mockDateFormat    `json:"format"`
	Filters mockSearchFilters `json:"filters"`
	Page    int               `json:"page"`
}

func searchFixture(r *http.Request) ([]byte, bool) {
	var request mockSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, false
	}
	if !request.hasValidEnvelope() {
		return nil, false
	}
	switch {
	case request.matches(mockSearchKeyword, "relevance", 1, mockSearchFilters{Locations: []string{mockSearchLocation}}):
		return mockJobsResponse, true
	case request.matches(mockFilteredKeyword, "newest", 2, mockFilteredFilters):
		return mockFilteredJobsResponse, true
	case request.matches(mockMultiLocationKeyword, "relevance", 1, mockMultiLocationFilters):
		return mockJobsResponse, true
	default:
		return nil, false
	}
}

func (r mockSearchRequest) hasValidEnvelope() bool {
	return r.Locale == "en-us" &&
		r.Format.LongDate == "MMMM D, YYYY" &&
		r.Format.MediumDate == "MMM D, YYYY" &&
		len(r.Filters.Locations) >= 1
}

func (r mockSearchRequest) matches(query, sort string, page int, filters mockSearchFilters) bool {
	return r.Query == query &&
		r.Sort == sort &&
		r.Page == page &&
		reflect.DeepEqual(r.Filters, filters)
}

func serveMockJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
