package join

import (
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/jobs_empty_rsp.json
var mockJobsEmptyRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_notfound_rsp.html
var mockJobDetailNotFoundRsp []byte

//go:embed testdata/job_detail_remote_rsp.html
var mockJobDetailRemoteRsp []byte

//go:embed testdata/company_rsp.html
var mockCompanyRsp []byte

// MockCompanyID is the companyId the mock server serves mockJobsRsp for
// (routinelabs' real id — the fixture is captured live traffic, not
// hand-written).
const MockCompanyID = 172617

// MockEmptyCompanyID is a companyId the mock server serves an empty dump
// for, mirroring a real company with zero open jobs.
const MockEmptyCompanyID = 154838

// MockJobIdParam / MockJobSlug identify the job & company slug
// mockJobDetailRsp was captured for.
const (
	MockJobIdParam = "16397229-senior-software-engineer-backend-llm-infrastructure"
	MockJobSlug    = "routinelabs"
)

// MockRemoteJobIdParam / MockRemoteJobSlug identify a REMOTE/ANYWHERE job
// (mockJobDetailRemoteRsp): workplaceType REMOTE, remoteType ANYWHERE, but
// city/country still carry the employer's base location — see API.md's
// note on remoteType and internal/ats/join.go's joinLocation.
const (
	MockRemoteJobIdParam = "16433808-freelancer-m-w-d-gesucht-inbound-home-office-in-der-eu"
	MockRemoteJobSlug    = "hey-contact-heroes"
)

type mockGraphQLBody struct {
	Query     string          `json:"query"`
	Variables json.RawMessage `json:"variables"`
}

// NewMockServer returns an httptest.Server that mimics join.com's GraphQL
// search endpoint and SSR job detail pages with canned fixture responses,
// so tests never hit the real site. The caller owns the server and must
// Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc(graphqlPath, func(w http.ResponseWriter, r *http.Request) {
		var body mockGraphQLBody
		defer r.Body.Close()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)

		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(string(body.Variables), strconv.Itoa(MockCompanyID)):
			_, _ = w.Write(mockJobsRsp)
		default:
			// Any other companyId (including MockEmptyCompanyID and any
			// real roster id besides MockCompanyID) gets the empty dump —
			// mirroring live behavior where an id the mock has no
			// specific fixture for is indistinguishable from a real
			// company with zero open jobs (see API.md).
			_, _ = w.Write(mockJobsEmptyRsp)
		}
	})
	mux.HandleFunc("/companies/"+MockJobSlug+"/"+MockJobIdParam, serveMockHTML(mockJobDetailRsp))
	mux.HandleFunc("/companies/"+MockRemoteJobSlug+"/"+MockRemoteJobIdParam, serveMockHTML(mockJobDetailRemoteRsp))
	mux.HandleFunc("/companies/"+MockJobSlug, serveMockHTML(mockCompanyRsp))
	mux.HandleFunc("/companies/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(mockJobDetailNotFoundRsp)
	})
	return httptest.NewServer(mux)
}

func serveMockHTML(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}
}
