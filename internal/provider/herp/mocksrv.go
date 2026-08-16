package herp

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

// MockSlug is the live company whose board is captured in
// testdata/company_rsp.json. It was picked for its field spread: full-remote,
// hybrid, and remote-less postings; two prefectures; postings with and
// without a city; and several job-role parents.
const MockSlug = "notainc"

// MockSparseSlug is the company captured in testdata/company_sparse_rsp.json,
// which carries a posting with no structured location, no free-text location,
// and no remote type. It is deliberately absent from companies.yaml so ATS
// tests can also exercise a URL-resolved company outside the curated roster.
const MockSparseSlug = "resmahr"

// MockMediaOptOutSlug is the company captured in
// testdata/company_media_optout_rsp.json, whose
// companyIsApplicationEnabled is false: its HERP Career media pages render
// without an apply action even though its postings are open and its HERP
// Hire career pages still accept applications.
const MockMediaOptOutSlug = "unifa"

// MockOutsideFounderSlug is the company captured in
// testdata/company_outside_founder_rsp.json. One of its directors carries
// isFounder for a previous company with isInside false, which is the shape
// that makes a naive founder check misattribute the founding.
const MockOutsideFounderSlug = "kaminashi"

// MockUnknownSlug is a slug the mock server 404s, replaying the upstream's
// message-less error body.
const MockUnknownSlug = "this-company-does-not-exist-xyz"

//go:embed testdata/company_rsp.json
var mockCompanyRsp []byte

//go:embed testdata/company_sparse_rsp.json
var mockCompanySparseRsp []byte

//go:embed testdata/company_media_optout_rsp.json
var mockCompanyMediaOptOutRsp []byte

//go:embed testdata/company_outside_founder_rsp.json
var mockCompanyOutsideFounderRsp []byte

//go:embed testdata/company_not_found_rsp.json
var mockCompanyNotFoundRsp []byte

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

// NewMockServer returns a fixture-replaying HERP Career API. The caller owns
// the server and must close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/careers/api/v1/companies/"+MockSlug, serveMockJSON(http.StatusOK, mockCompanyRsp))
	mux.HandleFunc("/careers/api/v1/companies/"+MockSparseSlug, serveMockJSON(http.StatusOK, mockCompanySparseRsp))
	mux.HandleFunc("/careers/api/v1/companies/"+MockMediaOptOutSlug, serveMockJSON(http.StatusOK, mockCompanyMediaOptOutRsp))
	mux.HandleFunc("/careers/api/v1/companies/"+MockOutsideFounderSlug, serveMockJSON(http.StatusOK, mockCompanyOutsideFounderRsp))
	mux.HandleFunc("/careers/api/v1/jobs", serveMockJSON(http.StatusOK, mockJobsRsp))
	mux.HandleFunc("/careers/api/v1/companies/", serveMockJSON(http.StatusNotFound, mockCompanyNotFoundRsp))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	return httptest.NewServer(mux)
}

func serveMockJSON(status int, data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(data)
	}
}
