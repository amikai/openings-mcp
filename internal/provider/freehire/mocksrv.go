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

//go:embed testdata/jobs_ignored_params_rsp.json
var mockSearchIgnoredParamsRsp []byte

//go:embed testdata/jobs_facets_rsp.json
var mockFacetsRsp []byte

//go:embed testdata/companies_rsp.json
var mockCompaniesRsp []byte

//go:embed testdata/companies_no_match_rsp.json
var mockCompaniesNoMatchRsp []byte

//go:embed testdata/company_detail_rsp.json
var mockCompanyDetailRsp []byte

//go:embed testdata/geo_cities_rsp.json
var mockCitiesRsp []byte

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

// MockCompanySlug is the company NewMockServer serves a full profile
// for, matching testdata/company_detail_rsp.json.
const MockCompanySlug = "stripe"

// MockCityQuery and MockCityCountry are the city typeahead NewMockServer
// resolves, matching testdata/geo_cities_rsp.json.
const (
	MockCityQuery   = "lond"
	MockCityCountry = "gb"
)

// MockIgnoredParamsQuery makes the search endpoint answer with the
// dropped-parameter response in testdata/jobs_ignored_params_rsp.json:
// the whole catalogue, a 200, and a meta.ignored_params report naming
// "country".
//
// The live trigger is a misspelled parameter NAME, which the generated
// client cannot emit — every name it sends comes from openapi.yaml. So
// the mock keys off a query value instead; the drift being covered is in
// how the response is handled, not in how the request is built.
const MockIgnoredParamsQuery = "trigger-ignored-params"

// NewMockServer returns an httptest.Server serving canned freehire API
// fixture responses, so tests never hit the live API. All fixtures were
// captured live on 2026-08-18 (see testdata/*.hurl). The caller owns the
// server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/facets", func(w http.ResponseWriter, r *http.Request) {
		serveMockJSON(w, http.StatusOK, mockFacetsRsp)
	})
	mux.HandleFunc("/jobs/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") == MockIgnoredParamsQuery {
			serveMockJSON(w, http.StatusOK, mockSearchIgnoredParamsRsp)
			return
		}
		switch q.Get("company_slug") {
		case MockCompanySlug:
			serveMockJSON(w, http.StatusOK, mockSearchFilteredRsp)
		case MockUnknownCompanySlug:
			serveMockJSON(w, http.StatusOK, mockSearchUnknownCompanyRsp)
		default:
			serveMockJSON(w, http.StatusOK, mockSearchRsp)
		}
	})
	mux.HandleFunc("/geo/cities", func(w http.ResponseWriter, r *http.Request) {
		serveMockJSON(w, http.StatusOK, mockCitiesRsp)
	})
	mux.HandleFunc("/companies", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == MockNoMatchCompanyQuery {
			serveMockJSON(w, http.StatusOK, mockCompaniesNoMatchRsp)
			return
		}
		serveMockJSON(w, http.StatusOK, mockCompaniesRsp)
	})
	mux.HandleFunc("/companies/", func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/companies/") {
		case MockCompanySlug:
			serveMockJSON(w, http.StatusOK, mockCompanyDetailRsp)
		default:
			serveMockJSON(w, http.StatusNotFound, mockDetailNotFoundRsp)
		}
	})
	mux.HandleFunc("/jobs/", func(w http.ResponseWriter, r *http.Request) {
		// "/jobs/facets" and "/jobs/search" have their own exact
		// patterns above, which ServeMux prefers over this subtree.
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
