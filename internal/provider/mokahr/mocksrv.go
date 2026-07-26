package mokahr

import (
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
)

// Identifiers the fixtures were captured against, so tests and the ATS
// adapter address the mock server without repeating literals.
const (
	MockOrgID        = "high-flyer"
	MockSiteID       = "140576"
	MockJobID        = "7dcd6fde-84f1-4deb-890c-f1f275df0efc"
	MockUnknownOrgID = "openings-mcp-no-such-org"
	MockUnknownJobID = "00000000-0000-0000-0000-000000000000"
	// MockNoLocationOrgID / MockNoLocationSiteID address a second captured
	// tenant, one of those whose listing carries "locations": null and whose
	// aggregations come back empty.
	MockNoLocationOrgID  = "vanke"
	MockNoLocationSiteID = "36266"
	// MockKeyword matches exactly one posting's title upstream.
	MockKeyword = "运维"
	// MockPageSize is how many jobs each captured page holds. The mock hands
	// out two such pages and then the captured past-the-end page, so a caller
	// reading the whole board makes three requests for 10 distinct jobs — the
	// real board's total, 35, is what every page reports.
	MockPageSize = 5
	// MockTotal is the board size the captured fixtures report.
	MockTotal = 35
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/jobs_page2_rsp.json
var mockJobsPage2Rsp []byte

//go:embed testdata/jobs_end_rsp.json
var mockJobsEndRsp []byte

//go:embed testdata/jobs_keyword_rsp.json
var mockJobsKeywordRsp []byte

//go:embed testdata/jobs_unknown_site_rsp.json
var mockJobsUnknownSiteRsp []byte

//go:embed testdata/job_rsp.json
var mockJobRsp []byte

//go:embed testdata/job_not_found_rsp.json
var mockJobNotFoundRsp []byte

//go:embed testdata/filter_aggregations_rsp.json
var mockFilterAggregationsRsp []byte

//go:embed testdata/jobs_no_locations_rsp.json
var mockJobsNoLocationsRsp []byte

//go:embed testdata/filter_aggregations_empty_rsp.json
var mockFilterAggregationsEmptyRsp []byte

// NewMockServer returns an httptest.Server replaying the captured MokaHR
// fixtures, so tests never hit the live API. The fixtures are stored exactly
// as MokaHR sent them — still obfuscated — so a client talking to this server
// exercises the real deobfuscation path.
//
// Every endpoint is a POST to a fixed path, so requests are routed on their
// body rather than on the URL.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/outer/ats-apply/website/jobs/v2", func(w http.ResponseWriter, r *http.Request) {
		req := decodeMockRequest(r)
		switch {
		case req.OrgID == MockNoLocationOrgID && req.SiteID == MockNoLocationSiteID:
			writeMockJSON(w, mockJobsNoLocationsRsp)
		case req.OrgID != MockOrgID || req.SiteID != MockSiteID:
			writeMockJSON(w, mockJobsUnknownSiteRsp)
		case req.Keyword != "":
			writeMockJSON(w, mockJobsKeywordRsp)
		case req.Offset >= 2*MockPageSize:
			writeMockJSON(w, mockJobsEndRsp)
		case req.Offset >= MockPageSize:
			writeMockJSON(w, mockJobsPage2Rsp)
		default:
			writeMockJSON(w, mockJobsRsp)
		}
	})

	mux.HandleFunc("POST /api/outer/ats-apply/website/job", func(w http.ResponseWriter, r *http.Request) {
		req := decodeMockRequest(r)
		if req.OrgID != MockOrgID || req.SiteID != MockSiteID || req.JobID != MockJobID {
			writeMockJSON(w, mockJobNotFoundRsp)
			return
		}
		writeMockJSON(w, mockJobRsp)
	})

	mux.HandleFunc("POST /api/outer/ats-apply/website/jobs/v2/filterFieldsAggregations", func(w http.ResponseWriter, r *http.Request) {
		req := decodeMockRequest(r)
		switch {
		case req.OrgID == MockNoLocationOrgID && req.SiteID == MockNoLocationSiteID:
			writeMockJSON(w, mockFilterAggregationsEmptyRsp)
		case req.OrgID != MockOrgID || req.SiteID != MockSiteID:
			writeMockJSON(w, mockJobsUnknownSiteRsp)
		default:
			writeMockJSON(w, mockFilterAggregationsRsp)
		}
	})

	return httptest.NewServer(mux)
}

// mockRequest is the union of the request fields the mock routes on.
type mockRequest struct {
	OrgID   string `json:"orgId"`
	SiteID  string `json:"siteId"`
	JobID   string `json:"jobId"`
	Keyword string `json:"keyword"`
	Offset  int    `json:"offset"`
}

func decodeMockRequest(r *http.Request) mockRequest {
	var req mockRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return req
	}
	_ = json.Unmarshal(body, &req)
	return req
}

func writeMockJSON(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)
}
