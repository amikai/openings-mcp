package bamboohr

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

// MockSlug is the live tenant whose board is captured in
// testdata/list_rsp.json (see testdata/list_req.hurl).
const MockSlug = "giatecscientific"

// MockVarietySlug is the live tenant whose board is captured in
// testdata/list_variety_rsp.json: all three locationType codes, null
// location.city rows, and populated atsLocation rows.
const MockVarietySlug = "curtinmaritime"

// MockNonRosterSlug is deliberately absent from companies.yaml so ATS tests
// can exercise a URL-resolved BambooHR tenant outside the curated roster.
const MockNonRosterSlug = "somestartup"

// MockJobID is the job served with every optional detail field populated
// (testdata/detail_rsp.json, a MockVarietySlug posting).
const MockJobID = "201"

// MockNullsJobID is the job served with the optional detail fields nulled
// out (testdata/detail_nulls_rsp.json, a MockSlug posting also present in
// the list fixture).
const MockNullsJobID = "167"

//go:embed testdata/list_rsp.json
var mockListRsp []byte

//go:embed testdata/list_variety_rsp.json
var mockListVarietyRsp []byte

//go:embed testdata/list_empty_rsp.json
var mockListEmptyRsp []byte

//go:embed testdata/detail_rsp.json
var mockDetailRsp []byte

//go:embed testdata/detail_nulls_rsp.json
var mockDetailNullsRsp []byte

//go:embed testdata/detail_not_found_rsp.json
var mockDetailNotFoundRsp []byte

// NewMockServer returns a fixture-replaying BambooHR careers site for the
// MockSlug tenant. Both fixture details are served so tests can pick either
// variant; unknown job ids get the captured 404 body. The caller owns the
// server and must close it.
func NewMockServer() *httptest.Server {
	return newTenantServer(mockListRsp)
}

// NewVarietyMockServer returns a fixture-replaying careers site for the
// MockVarietySlug tenant, whose board exercises the list feed's value
// variety.
func NewVarietyMockServer() *httptest.Server {
	return newTenantServer(mockListVarietyRsp)
}

// NewEmptyMockServer returns a fixture-replaying careers site for a tenant
// whose board has no public postings (HTTP 200, totalCount 0).
func NewEmptyMockServer() *httptest.Server {
	return newTenantServer(mockListEmptyRsp)
}

// NewRedirectMockServer replays the unknown-tenant behavior: every path
// 302-redirects to the marketing site instead of returning an error status
// (see testdata/list_unknown_company_req.hurl).
func NewRedirectMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://www.bamboohr.com", http.StatusFound)
	}))
}

func newTenantServer(listRsp []byte) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/careers/list", serveMockJSON(http.StatusOK, listRsp))
	mux.HandleFunc("/careers/"+MockJobID+"/detail", serveMockJSON(http.StatusOK, mockDetailRsp))
	mux.HandleFunc("/careers/"+MockNullsJobID+"/detail", serveMockJSON(http.StatusOK, mockDetailNullsRsp))
	mux.HandleFunc("/careers/", serveMockJSON(http.StatusNotFound, mockDetailNotFoundRsp))
	return httptest.NewServer(mux)
}

func serveMockJSON(status int, data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(data)
	}
}
