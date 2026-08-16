package himalayas

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/browse_rsp.json
var mockBrowseRsp []byte

//go:embed testdata/search_rsp.json
var mockSearchRsp []byte

//go:embed testdata/search_filtered_rsp.json
var mockSearchFilteredRsp []byte

//go:embed testdata/search_company_rsp.json
var mockSearchCompanyRsp []byte

//go:embed testdata/search_invalid_country_rsp.json
var mockSearchInvalidCountryRsp []byte

//go:embed testdata/search_unknown_company_rsp.json
var mockSearchUnknownCompanyRsp []byte

// MockUnknownCompany is a company slug deliberately absent from Himalayas,
// matching the quirk captured in testdata/search_unknown_company_rsp.json:
// an unrecognized slug is rejected as a 400 "Invalid company", not answered
// with an empty 200 result.
const MockUnknownCompany = "this-company-does-not-exist-xyz"

// NewMockServer returns an httptest.Server serving canned Himalayas Remote
// Jobs API fixture responses, so tests never hit the live API. All fixtures
// were captured live on 2026-07-18 (see testdata/*.hurl). The caller owns
// the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/jobs/api", serveMockJSON(mockBrowseRsp))

	mux.HandleFunc("/jobs/api/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("country") == "Narnia":
			serveMockError(w, mockSearchInvalidCountryRsp)
		case q.Get("company") == MockUnknownCompany:
			serveMockError(w, mockSearchUnknownCompanyRsp)
		case q.Get("company") == "leland":
			serveMockJSON(mockSearchCompanyRsp)(w, r)
		case q.Get("country") == "US":
			serveMockJSON(mockSearchFilteredRsp)(w, r)
		default:
			serveMockJSON(mockSearchRsp)(w, r)
		}
	})

	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func serveMockError(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	w.Write(data)
}
