package linkedin

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.html
var mockJobsRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

// NewMockServer returns an httptest.Server that mimics LinkedIn's guest jobs
// endpoints with canned fixture responses, so tests never hit the real site.
// The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/view/", serveMockHTML(mockJobDetailRsp))
	mux.HandleFunc("/jobs-guest/jobs/api/seeMoreJobPostings/search", serveMockHTML(mockJobsRsp))
	return httptest.NewServer(mux)
}

func serveMockHTML(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}
