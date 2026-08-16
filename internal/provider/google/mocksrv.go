package google

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.html
var _mockJobsRsp []byte

//go:embed testdata/job_detail_rsp.html
var _mockJobDetailRsp []byte

// NewMockServer returns an httptest.Server that mimics the Google Careers
// site with canned fixture responses, so tests never hit the real site. The
// caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/results/", serveMockHTML(_mockJobDetailRsp))
	mux.HandleFunc("/jobs/results", serveMockHTML(_mockJobsRsp))
	return httptest.NewServer(mux)
}

func serveMockHTML(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}
