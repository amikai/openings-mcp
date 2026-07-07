package job104

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailRsp []byte

// MockErrorKeyword and MockNotFoundJobCode trigger upstream-error responses
// from the mock server so tests can exercise the non-200 paths: searching
// for MockErrorKeyword returns a 500 and requesting MockNotFoundJobCode's
// detail returns a 404, both with a JSON ErrorResponse body like the real
// API.
const (
	MockErrorKeyword    = "mock-500"
	MockNotFoundJobCode = "mock-404"
)

// MockCompanyKeyword triggers the real API's company-keyword mode: searching
// for it WITHOUT excludeCompanyKeyword=true returns the pagination-less
// `{"data":[],"metadata":{"companyKeyword":true}}` shape 104 sends when it
// recognizes the keyword as a company name; with the parameter it returns the
// normal search fixture, as the real API does.
const MockCompanyKeyword = "聯發科"

// NewMockServer returns an httptest.Server that mimics the 104 API with
// canned fixture responses, so tests never hit the real site. The caller
// owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/search/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("keyword") == MockErrorKeyword {
			serveMockError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if r.URL.Query().Get("keyword") == MockCompanyKeyword && r.URL.Query().Get("excludeCompanyKeyword") != "true" {
			serveMockJSON([]byte(`{"data":[],"metadata":{"companyKeyword":true}}`))(w, r)
			return
		}
		serveMockJSON(mockJobsRsp)(w, r)
	})
	mux.HandleFunc("/job/ajax/content/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/"+MockNotFoundJobCode) {
			serveMockError(w, http.StatusNotFound, "job not found")
			return
		}
		serveMockJSON(mockJobDetailRsp)(w, r)
	})
	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func serveMockError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"message":%q}`, msg)
}
