package greenhouse

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/jobs_content_rsp.json
var mockJobsContentRsp []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_full_rsp.json
var mockJobDetailFullRsp []byte

// MockNonRosterBoard is a board token deliberately absent from
// companies.yaml, so ats-layer tests can exercise non-roster behavior.
const MockNonRosterBoard = "somestartup"

// NewMockServer returns an httptest.Server serving canned Greenhouse Job
// Board API fixture responses, so tests never hit a live board. Most
// fixtures were captured from real boards (see testdata/*.sh);
// jobs_content_rsp.json is hand-crafted in the same shape so the ats
// adapter tests have stable content=true data. The caller owns the server
// and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/boards/safariai/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("content") == "true" {
			serveMockJSON(mockJobsContentRsp)(w, r)
			return
		}
		serveMockJSON(mockJobsRsp)(w, r)
	})

	mux.HandleFunc("/boards/anthropic/jobs/4461450008", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("questions") == "true" {
			serveMockJSON(mockJobDetailFullRsp)(w, r)
			return
		}
		serveMockJSON(mockJobDetailRsp)(w, r)
	})

	mux.HandleFunc("/boards/"+MockNonRosterBoard+"/jobs/4461450008", serveMockJSON(mockJobDetailRsp))

	mux.HandleFunc("/boards/doesnotexist/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mux.HandleFunc("/boards/anthropic/jobs/999999999999", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
