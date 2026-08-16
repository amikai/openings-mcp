package amazon

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var _mockJobsResponse []byte

//go:embed testdata/jobs_filtered_rsp.json
var _mockFilteredJobsResponse []byte

//go:embed testdata/job_detail_rsp.json
var _mockJobDetailResponse []byte

//go:embed testdata/job_detail_notfound_rsp.json
var _mockJobDetailNotFoundResponse []byte

//go:embed testdata/jobs_soft_error_rsp.json
var _mockSoftErrorResponse []byte

// NewMockServer returns an httptest.Server that replays captured Amazon Jobs
// API responses. The caller owns the server and must close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/en/search.json", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch {
		case query.Get("base_query") == "3164253":
			serveMockJSON(_mockJobDetailResponse)(w, r)
		case query.Get("base_query") == "9999999999":
			serveMockJSON(_mockJobDetailNotFoundResponse)(w, r)
		case query.Get("base_query") == "soft-error":
			serveMockJSON(_mockSoftErrorResponse)(w, r)
		case len(query["normalized_country_code[]"]) > 0:
			serveMockJSON(_mockFilteredJobsResponse)(w, r)
		default:
			serveMockJSON(_mockJobsResponse)(w, r)
		}
	})
	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err := w.Write(data); err != nil {
			return
		}
	}
}
