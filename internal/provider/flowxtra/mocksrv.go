package flowxtra

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/jobs_filtered_rsp.json
var mockJobsFilteredRsp []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_not_found_rsp.json
var mockJobDetailNotFoundRsp []byte

// mockDetailHasID is the has_id pinned by testdata/job_detail_rsp.json.
const mockDetailHasID = "M88PB"

// NewMockServer returns an httptest.Server serving canned Flowxtra API
// fixture responses, all captured live on 2026-07-26 (see
// testdata/*.hurl). The jobs handler serves the filtered fixture when a
// search-key is present (mirroring the API's real server-side
// narrowing) and the full first page otherwise; the detail handler
// serves the pinned job and the API's JSON 404 for any other has_id.
// The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/central/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("search-key") != "" {
			w.Write(mockJobsFilteredRsp)
			return
		}
		w.Write(mockJobsRsp)
	})
	mux.HandleFunc("/candidate/jobs/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		hasID := strings.TrimPrefix(r.URL.Path, "/candidate/jobs/")
		if hasID != mockDetailHasID {
			w.WriteHeader(http.StatusNotFound)
			w.Write(mockJobDetailNotFoundRsp)
			return
		}
		w.Write(mockJobDetailRsp)
	})
	return httptest.NewServer(mux)
}
