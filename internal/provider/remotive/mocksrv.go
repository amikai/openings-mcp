package remotive

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/categories_rsp.json
var mockCategoriesRsp []byte

// NewMockServer returns an httptest.Server serving canned Remotive API
// fixture responses, so tests never hit the live (and tightly
// rate-limited) API. Both fixtures were captured live on 2026-07-19 (see
// testdata/*.hurl). The jobs handler ignores the query string exactly
// like the real endpoint does (see the no-op quirk in openapi.yaml), so
// there is no filtered variant to serve. The caller owns the server and
// must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/remote-jobs", serveMockJSON(mockJobsRsp))
	mux.HandleFunc("/remote-jobs/categories", serveMockJSON(mockCategoriesRsp))
	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Matches the live API's header; the generated decoder strips the
		// charset parameter before matching.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)
	}
}
