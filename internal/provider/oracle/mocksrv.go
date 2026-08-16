package oracle

import (
	"bytes"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/careers_page_rsp.html
var mockCareersPageResponse []byte

//go:embed testdata/search_rsp.json
var mockSearchResponse []byte

//go:embed testdata/search_filtered_rsp.json
var mockFilteredSearchResponse []byte

//go:embed testdata/search_facets_rsp.json
var mockFacetSearchResponse []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailResponse []byte

//go:embed testdata/job_detail_not_found_rsp.json
var mockJobDetailNotFoundResponse []byte

// NewMockServer returns an httptest.Server that mimics the Oracle Recruiting
// Cloud Candidate Experience API with captured public response fixtures. The
// caller owns the server and must close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	var server *httptest.Server
	mux.HandleFunc("/hcmUI/CandidateExperience/en/sites/Mayo-US/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := bytes.ReplaceAll(
			mockCareersPageResponse,
			[]byte("https://fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com:443"),
			[]byte(server.URL),
		)
		_, _ = w.Write(page)
	})
	mux.HandleFunc("/hcmRestApi/resources/latest/recruitingCEJobRequisitions", func(w http.ResponseWriter, r *http.Request) {
		finder := r.URL.Query().Get("finder")
		switch {
		case strings.Contains(finder, `keyword="analyst"`):
			serveMockJSON(mockFilteredSearchResponse)(w, r)
		case strings.Contains(finder, "facetsList=TITLES;LOCATIONS;CATEGORIES;WORKPLACE_TYPES;POSTING_DATES;WORK_LOCATIONS;ORGANIZATIONS"):
			serveMockJSON(mockFacetSearchResponse)(w, r)
		default:
			serveMockJSON(mockSearchResponse)(w, r)
		}
	})
	mux.HandleFunc("/hcmRestApi/resources/latest/recruitingCEJobRequisitionDetails", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("finder"), `Id="999999999999"`) {
			serveMockJSON(mockJobDetailNotFoundResponse)(w, r)
			return
		}
		serveMockJSON(mockJobDetailResponse)(w, r)
	})
	server = httptest.NewServer(mux)
	return server
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}
}
