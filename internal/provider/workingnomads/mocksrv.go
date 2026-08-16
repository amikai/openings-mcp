package workingnomads

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

// NewMockServer returns an httptest.Server that mimics Working Nomads's
// exposed_jobs endpoint with a canned fixture response, so tests never
// hit the real site. Query parameters are accepted but ignored, mirroring
// the real endpoint's no-op behavior. Any other path is HTTP 404. The
// caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/exposed_jobs/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockJobsRsp)
	})
	return httptest.NewServer(mux)
}
