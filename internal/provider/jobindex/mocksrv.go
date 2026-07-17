package jobindex

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/jobs_rsp.html
var mockJobsRsp []byte

//go:embed testdata/jobs_filtered_rsp.html
var mockJobsFilteredRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_robot_rsp.html
var mockJobDetailRobotRsp []byte

// NewMockServer returns an httptest.Server that serves Jobindex-shaped HTML
// fixtures so tests never hit the live site. The caller must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobsoegning", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Filtered fixture when jobage is set (mirrors jobs_filtered_req.hurl).
		if r.URL.Query().Get("jobage") != "" {
			w.Write(mockJobsFilteredRsp)
			return
		}
		w.Write(mockJobsRsp)
	})
	mux.HandleFunc("/jobsoegning/", func(w http.ResponseWriter, r *http.Request) {
		// Area-prefixed path still returns the main search fixture.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(mockJobsRsp)
	})
	mux.HandleFunc("/vis-job/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/vis-job/")
		if id == "" || id == "missing999" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Aggregated (r*) postings use jix_robotjob-inner; hosted (h*) use PaidJob.
		if strings.HasPrefix(id, "r") {
			w.Write(mockJobDetailRobotRsp)
			return
		}
		w.Write(mockJobDetailRsp)
	})
	return httptest.NewServer(mux)
}
