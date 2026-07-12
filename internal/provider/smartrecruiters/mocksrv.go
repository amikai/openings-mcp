package smartrecruiters

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/postings_rsp.json
var mockPostingsRsp []byte

//go:embed testdata/postings_filtered_rsp.json
var mockPostingsFilteredRsp []byte

//go:embed testdata/postings_unknown_company_rsp.json
var mockPostingsUnknownCompanyRsp []byte

//go:embed testdata/posting_detail_rsp.json
var mockPostingDetailRsp []byte

//go:embed testdata/posting_not_found_rsp.json
var mockPostingNotFoundRsp []byte

//go:embed testdata/departments_rsp.json
var mockDepartmentsRsp []byte

// MockUnknownCompany is a companyIdentifier deliberately absent from any
// roster, matching the quirk captured in testdata/postings_unknown_company_rsp.json:
// SmartRecruiters answers HTTP 200 with empty content rather than a 404.
const MockUnknownCompany = "this-company-does-not-exist-xyz"

// NewMockServer returns an httptest.Server serving canned SmartRecruiters
// Posting API fixture responses, so tests never hit the live API. All
// fixtures were captured from Equinox's live board (see testdata/*.hurl).
// The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/companies/equinox/postings", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "trainer" {
			serveMockJSON(mockPostingsFilteredRsp)(w, r)
			return
		}
		serveMockJSON(mockPostingsRsp)(w, r)
	})

	mux.HandleFunc("/v1/companies/"+MockUnknownCompany+"/postings", serveMockJSON(mockPostingsUnknownCompanyRsp))

	mux.HandleFunc("/v1/companies/equinox/postings/744000137225639", serveMockJSON(mockPostingDetailRsp))

	mux.HandleFunc("/v1/companies/equinox/postings/000000000000", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write(mockPostingNotFoundRsp)
	})

	mux.HandleFunc("/v1/companies/equinox/departments", serveMockJSON(mockDepartmentsRsp))

	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Matches the live API's header; the generated decoder strips the
		// charset parameter before matching against the spec.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)
	}
}
