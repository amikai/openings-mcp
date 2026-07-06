package lever

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/postings_rsp.json
var mockPostingsRsp []byte

//go:embed testdata/posting_detail_rsp.json
var mockPostingDetailRsp []byte

// MockNotFoundSite and MockNotFoundPostingID trigger the mock server's
// error path so tests can exercise non-200 handling: listing
// MockNotFoundSite's postings or requesting MockNotFoundPostingID's detail
// returns a 404 with the same JSON error body the real API sends for an
// unknown site or posting.
const (
	MockNotFoundSite      = "mock-404-site"
	MockNotFoundPostingID = "mock-404-posting"
)

// NewMockServer returns an httptest.Server that mimics the Lever Postings
// API with canned leverdemo fixture responses, so tests never hit the real
// site. The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/postings/{site}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("site") == MockNotFoundSite {
			serveMockError(w, http.StatusNotFound, "Document not found")
			return
		}
		serveMockJSON(mockPostingsRsp)(w, r)
	})
	mux.HandleFunc("/v0/postings/{site}/{postingId}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("postingId") == MockNotFoundPostingID {
			serveMockError(w, http.StatusNotFound, "Document not found")
			return
		}
		serveMockJSON(mockPostingDetailRsp)(w, r)
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
	fmt.Fprintf(w, `{"ok":false,"error":%q}`, msg)
}
