package indeed

import (
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/jobs_filtered_rsp.json
var mockJobsFilteredRsp []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_notfound_rsp.json
var mockJobDetailNotFoundRsp []byte

// MockNotFoundJobKey is the job key the mock server treats as not-found,
// mirroring the real jobData empty-results shape captured in
// testdata/job_detail_notfound_rsp.json.
const MockNotFoundJobKey = "0000000000000000"

type mockGraphQLBody struct {
	Query     string          `json:"query"`
	Variables json.RawMessage `json:"variables"`
}

// NewMockServer returns an httptest.Server that mimics Indeed's GraphQL
// endpoint with canned fixture responses, so tests never hit the real API.
// Dispatch looks at the operation document and variables the same way the
// real single-endpoint API keys off the request body. The caller owns the
// server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		var body mockGraphQLBody
		defer r.Body.Close()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)

		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(body.Query, "jobData"):
			// jobKeys live in variables under genqlient; fall back to the
			// document string for hand-written clients / hurl captures.
			if strings.Contains(string(body.Variables), MockNotFoundJobKey) ||
				strings.Contains(body.Query, MockNotFoundJobKey) {
				_, _ = w.Write(mockJobDetailNotFoundRsp)
				return
			}
			_, _ = w.Write(mockJobDetailRsp)
		case strings.Contains(string(body.Variables), "dateOnIndeed") ||
			strings.Contains(body.Query, "dateOnIndeed"):
			_, _ = w.Write(mockJobsFilteredRsp)
		default:
			_, _ = w.Write(mockJobsRsp)
		}
	})
	return httptest.NewServer(mux)
}
