package oracle

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/search_resp.json
var mockSearchResponse []byte

//go:embed testdata/search_filtered_resp.json
var mockFilteredSearchResponse []byte

//go:embed testdata/search_facets_resp.json
var mockFacetSearchResponse []byte

//go:embed testdata/job_detail_resp.json
var mockJobDetailResponse []byte

//go:embed testdata/job_detail_not_found_resp.json
var mockJobDetailNotFoundResponse []byte

// NewMockServer returns an httptest.Server that mimics the Oracle Recruiting
// Cloud Candidate Experience API with captured public response fixtures. The
// caller owns the server and must close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/hcmRestApi/resources/latest/recruitingCEJobRequisitions", func(w http.ResponseWriter, r *http.Request) {
		finder := r.URL.Query().Get("finder")
		switch {
		case strings.Contains(finder, `keyword="analyst"`):
			serveMockJSON(mockFilteredSearchResponse)(w, r)
		case strings.Contains(finder, "facetsList=TITLES;LOCATIONS;CATEGORIES;WORKPLACE_TYPES"):
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
	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}
}
