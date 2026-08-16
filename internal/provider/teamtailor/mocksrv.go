package teamtailor

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

// MockHost is the live career-site host whose response is captured in
// testdata/jobs_rsp.json.
const MockHost = "knaufsemea.teamtailor.com"

// MockNonRosterHost is deliberately absent from companies.yaml so ATS tests
// can exercise a URL-resolved Teamtailor tenant outside the curated roster.
const MockNonRosterHost = "somestartup.teamtailor.com"

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/jobs_nulls_rsp.json
var mockJobsNullsRsp []byte

//go:embed testdata/jobs_missing_location_rsp.json
var mockJobsMissingLocationRsp []byte

// NewMockServer returns a fixture-replaying Teamtailor career site. The
// caller owns the server and must close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs.json", serveMockFeed(mockJobsRsp))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	return httptest.NewServer(mux)
}

// NewNullMockServer returns a fixture-replaying career site whose address
// contains an explicit null region.
func NewNullMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs.json", serveMockFeed(mockJobsNullsRsp))
	return httptest.NewServer(mux)
}

// NewMissingLocationMockServer returns a fixture-replaying career site whose
// first posting omits the jobLocation field entirely.
func NewMissingLocationMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs.json", serveMockFeed(mockJobsMissingLocationRsp))
	return httptest.NewServer(mux)
}

func serveMockFeed(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/feed+json; charset=utf-8")
		_, _ = w.Write(data)
	}
}
