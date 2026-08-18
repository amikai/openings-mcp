package freehire

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/jobs_search_rsp.json
var mockSearchRsp []byte

//go:embed testdata/jobs_filtered_rsp.json
var mockSearchFilteredRsp []byte

//go:embed testdata/jobs_unknown_company_rsp.json
var mockSearchUnknownCompanyRsp []byte

//go:embed testdata/jobs_facets_rsp.json
var mockFacetsRsp []byte

//go:embed testdata/companies_rsp.json
var mockCompaniesRsp []byte

//go:embed testdata/companies_no_match_rsp.json
var mockCompaniesNoMatchRsp []byte

//go:embed testdata/job_detail_rsp.json
var mockDetailRsp []byte

//go:embed testdata/job_detail_not_found_rsp.json
var mockDetailNotFoundRsp []byte

// MockJobSlug is the public_slug served by NewMockServer's detail
// endpoint, matching testdata/job_detail_rsp.json.
const MockJobSlug = "program-manager-intake-portfolio-management-stripe-plftlklg"

// MockUnknownJobSlug is a slug deliberately absent from the catalogue,
// matching testdata/job_detail_not_found_rsp.json.
const MockUnknownJobSlug = "does-not-exist-zzzz"

// MockCompanyQuery is the company-name query NewMockServer resolves,
// matching testdata/companies_rsp.json. Two catalogue companies share
// the name, so it also covers the ambiguous case.
const MockCompanyQuery = "adria"

// MockNoMatchCompanyQuery is a query with no near neighbour, which
// company search answers with an empty page rather than a fuzzy guess.
const MockNoMatchCompanyQuery = "zzzzzzzzzz"

// MockUnknownCompanySlug is a company_slug that matches nothing,
// matching testdata/jobs_unknown_company_rsp.json.
const MockUnknownCompanySlug = "definitely-not-a-real-company-zzzz"

// NewMockServer returns an httptest.Server serving canned freehire API
// fixture responses, so tests never hit the live API. All fixtures were
// captured live on 2026-08-17 (see testdata/*.hurl). The caller owns the
// server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/facets", func(w http.ResponseWriter, r *http.Request) {
		serveMockJSON(w, http.StatusOK, mockFacetsRsp)
	})
	mux.HandleFunc("/agent/jobs/search", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("company_slug") {
		case "stripe":
			serveMockJSON(w, http.StatusOK, mockSearchFilteredRsp)
		case MockUnknownCompanySlug:
			serveMockJSON(w, http.StatusOK, mockSearchUnknownCompanyRsp)
		default:
			serveMockJSON(w, http.StatusOK, mockSearchRsp)
		}
	})
	mux.HandleFunc("/companies", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == MockNoMatchCompanyQuery {
			serveMockJSON(w, http.StatusOK, mockCompaniesNoMatchRsp)
			return
		}
		serveMockJSON(w, http.StatusOK, mockCompaniesRsp)
	})
	mux.HandleFunc("/jobs/", func(w http.ResponseWriter, r *http.Request) {
		// "/jobs/facets" has its own exact pattern above, which
		// ServeMux prefers over this subtree.
		switch strings.TrimPrefix(r.URL.Path, "/jobs/") {
		case MockJobSlug:
			serveMockJSON(w, http.StatusOK, mockDetailRsp)
		default:
			serveMockJSON(w, http.StatusNotFound, mockDetailNotFoundRsp)
		}
	})
	return httptest.NewServer(mux)
}

func serveMockJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(data)
}
