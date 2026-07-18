package mynavi

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/jobs_rsp.html
var mockJobsRsp []byte

//go:embed testdata/jobs_empty_rsp.html
var mockJobsEmptyRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

// MockNotFoundJobID is the job ID NewMockServer answers with HTTP 404, for
// exercising the expired/unknown-posting path.
const MockNotFoundJobID = "999999-1-1-1"

// NewMockServer returns an httptest.Server that mimics Mynavi Tenshoku with canned
// fixture responses, so tests never hit the real site. A /list/ URL whose kw
// token matches the zero-hit fixture's keyword serves the empty results
// page; any other /list/ URL serves the 50-card Python search. The caller
// owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/list/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "kwzzzqqqxyzabc") {
			serveMockHTML(w, mockJobsEmptyRsp)
			return
		}
		serveMockHTML(w, mockJobsRsp)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/jobinfo-") && !strings.Contains(r.URL.Path, MockNotFoundJobID) {
			serveMockHTML(w, mockJobDetailRsp)
			return
		}
		http.NotFound(w, r)
	})
	return httptest.NewServer(mux)
}

func serveMockHTML(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
