package recruitee

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

// MockSlug is the live career-site subdomain whose response is captured in
// testdata/offers_rsp.json.
const MockSlug = "bunq"

// MockNonRosterSlug is deliberately absent from companies.yaml so ATS tests
// can exercise a URL-resolved Recruitee tenant outside the curated roster.
const MockNonRosterSlug = "somestartup"

//go:embed testdata/offers_rsp.json
var mockOffersRsp []byte

//go:embed testdata/offers_nulls_rsp.json
var mockOffersNullsRsp []byte

// NewMockServer returns a fixture-replaying Recruitee career site. The
// caller owns the server and must close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/offers", serveMockFeed(mockOffersRsp))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	return httptest.NewServer(mux)
}

// NewNullMockServer returns a fixture-replaying career site whose payload
// contains explicit null fields.
func NewNullMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/offers", serveMockFeed(mockOffersNullsRsp))
	return httptest.NewServer(mux)
}

func serveMockFeed(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(data)
	}
}
