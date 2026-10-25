package taiwanjobs

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.xml
var mockJobsRsp []byte

// NewMockServer returns an httptest.Server that serves a captured TaiwanJobs
// feed fixture so tests never hit the live service. The caller must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc(feedPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
		w.Write(mockJobsRsp)
	})
	return httptest.NewServer(mux)
}
