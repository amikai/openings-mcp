package cake

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailRsp []byte

// NewMockServer returns an httptest.Server that mimics the Cake.me API with
// canned fixture responses, so tests never hit the real site. The caller
// owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/client/v1/jobs/search", serveMockJSON(mockJobsRsp))
	mux.HandleFunc("/api/client/v1/jobs/", serveMockJSON(mockJobDetailRsp))
	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
