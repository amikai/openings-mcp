package workable

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/jobs_page2_rsp.json
var mockJobsPage2Rsp []byte

//go:embed testdata/jobs_filtered_rsp.json
var mockJobsFilteredRsp []byte

//go:embed testdata/jobs_filters_rsp.json
var mockJobsFiltersRsp []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailRsp []byte

//go:embed testdata/job_not_found_rsp.txt
var mockJobNotFoundRsp []byte

//go:embed testdata/jobs_unknown_company_rsp.txt
var mockJobsUnknownCompanyRsp []byte

// MockUnknownCompany is an account deliberately absent from any roster,
// matching the quirk captured in testdata/jobs_unknown_company_rsp.txt: an
// unknown account 404s with a text/plain body, not JSON.
const MockUnknownCompany = "openings-mcp-no-such-account"

// MockPage2Token is the nextPage cursor inside testdata/jobs_rsp.json; a
// search body carrying it is answered with testdata/jobs_page2_rsp.json.
const MockPage2Token = "WzE3ODEwNDk2MDAwMDAsNTg0NDk1Nl0="

// NewMockServer returns an httptest.Server serving canned Workable job board
// API fixture responses, so tests never hit the live API. All fixtures were
// captured from Blueground's live board (see testdata/*.hurl). The caller
// owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v3/accounts/blueground/jobs", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		switch {
		case body.Token == MockPage2Token:
			serveMockJSON(mockJobsPage2Rsp)(w, r)
		case body.Query == "engineer":
			serveMockJSON(mockJobsFilteredRsp)(w, r)
		default:
			serveMockJSON(mockJobsRsp)(w, r)
		}
	})

	mux.HandleFunc("/api/v3/accounts/blueground/jobs/filters", serveMockJSON(mockJobsFiltersRsp))

	mux.HandleFunc("/api/v3/accounts/"+MockUnknownCompany+"/jobs", serveMockText(http.StatusNotFound, mockJobsUnknownCompanyRsp))

	mux.HandleFunc("/api/v2/accounts/blueground/jobs/B02DA69C8F", serveMockJSON(mockJobDetailRsp))

	mux.HandleFunc("/api/v2/accounts/blueground/jobs/0000000000", serveMockText(http.StatusNotFound, mockJobNotFoundRsp))

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

func serveMockText(status int, data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		w.Write(data)
	}
}
