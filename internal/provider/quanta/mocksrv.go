package quanta

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

// NewMockServer returns an httptest.Server serving the canned jobs dump
// fixture, so tests never hit the live board. The fixture was captured
// live on 2026-07-21 (see testdata/jobs_req.hurl). The handler ignores
// the query string exactly like the real endpoint does (see the no-op
// quirk in openapi.yaml), so there is no filtered variant to serve. The
// caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/QuantaRecruit/Home/QueryJob", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(mockJobsRsp)
	})
	return httptest.NewServer(mux)
}
