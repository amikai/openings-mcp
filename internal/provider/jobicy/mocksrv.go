package jobicy

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var _mockJobsRsp []byte

//go:embed testdata/jobs_filtered_rsp.json
var _mockJobsFilteredRsp []byte

//go:embed testdata/jobs_empty_rsp.json
var _mockJobsEmptyRsp []byte

//go:embed testdata/jobs_invalid_industry_rsp.json
var _mockJobsInvalidIndustryRsp []byte

//go:embed testdata/locations_rsp.json
var _mockLocationsRsp []byte

//go:embed testdata/industries_rsp.json
var _mockIndustriesRsp []byte

// NewMockServer returns an httptest.Server serving canned Jobicy API
// fixture responses, so tests never hit the live feed. It mirrors the real
// endpoint's query-based dispatch for the exact queries captured in
// testdata/*.hurl. The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/remote-jobs", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("get") == "locations":
			serveMockJSON(w, http.StatusOK, _mockLocationsRsp)
		case q.Get("get") == "industries":
			serveMockJSON(w, http.StatusOK, _mockIndustriesRsp)
		case q.Get("industry") == "not-a-real-industry":
			serveMockJSON(w, http.StatusBadRequest, _mockJobsInvalidIndustryRsp)
		case q.Get("tag") == "zzzznomatchzzz":
			serveMockJSON(w, http.StatusNotFound, _mockJobsEmptyRsp)
		case q.Get("geo") == "usa" && q.Get("industry") == "dev" && q.Get("tag") == "golang":
			serveMockJSON(w, http.StatusOK, _mockJobsFilteredRsp)
		default:
			serveMockJSON(w, http.StatusOK, _mockJobsRsp)
		}
	})
	return httptest.NewServer(mux)
}

func serveMockJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(data)
}
