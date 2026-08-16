package engage

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

// MockSlug is the multi-category board captured in testdata/board_rsp.html:
// 中途採用 at [CategoryCap] plus a smaller アルバイト・パート採用 category.
const MockSlug = "nova_career"

// MockCapSlug is the single-category board captured in
// testdata/board_cap_rsp.html, at exactly [CategoryCap].
const MockCapSlug = "cookbiz_jobs"

// MockMinimalSlug is the board captured in testdata/board_minimal_rsp.html,
// the smallest board engage actually serves: exactly one job.
const MockMinimalSlug = "2918"

// MockUnknownSlug is a slug the mock server 404s.
const MockUnknownSlug = "definitely-not-a-real-slug-xyz"

// MockWorkID is the job captured in testdata/job_detail_rsp.html, on
// [MockSlug].
const MockWorkID = "17046487"

// MockUnknownWorkID is a work id the mock server 404s on [MockSlug].
const MockUnknownWorkID = "999999999"

// MockNoJSONLDSlug and MockNoJSONLDWorkID address the posting captured in
// testdata/job_detail_no_jsonld_rsp.html: a normally rendered detail page
// that ships no JSON-LD block, so only the HTML fallback can read it.
const (
	MockNoJSONLDSlug   = "aspark-tokyo"
	MockNoJSONLDWorkID = "17068421"
)

// MockCompaniesTotal is the totalCount embedded in testdata/companies_rsp.json,
// the p_t value a page-2 request must carry (see API.md's p_t/f_t stitch).
const MockCompaniesTotal = "1031581"

//go:embed testdata/board_rsp.html
var mockBoardRsp []byte

//go:embed testdata/board_cap_rsp.html
var mockBoardCapRsp []byte

//go:embed testdata/board_minimal_rsp.html
var mockBoardMinimalRsp []byte

//go:embed testdata/board_not_found_rsp.html
var mockBoardNotFoundRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_no_jsonld_rsp.html
var mockJobDetailNoJSONLDRsp []byte

//go:embed testdata/job_not_found_rsp.html
var mockJobNotFoundRsp []byte

//go:embed testdata/companies_rsp.json
var mockCompaniesRsp []byte

//go:embed testdata/companies_page2_rsp.json
var mockCompaniesPage2Rsp []byte

// NewMockServer returns an httptest.Server replaying captured en-gage.net
// responses, so tests never hit the live host. The caller owns the server
// and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/"+MockSlug+"/", serveHTML(http.StatusOK, mockBoardRsp))
	mux.HandleFunc("/"+MockCapSlug+"/", serveHTML(http.StatusOK, mockBoardCapRsp))
	mux.HandleFunc("/"+MockMinimalSlug+"/", serveHTML(http.StatusOK, mockBoardMinimalRsp))
	mux.HandleFunc("/"+MockUnknownSlug+"/", serveHTML(http.StatusNotFound, mockBoardNotFoundRsp))

	mux.HandleFunc("/"+MockSlug+"/work_"+MockWorkID+"/", serveHTML(http.StatusOK, mockJobDetailRsp))
	mux.HandleFunc("/"+MockSlug+"/work_"+MockUnknownWorkID+"/", serveHTML(http.StatusNotFound, mockJobNotFoundRsp))
	mux.HandleFunc("/"+MockNoJSONLDSlug+"/work_"+MockNoJSONLDWorkID+"/", serveHTML(http.StatusOK, mockJobDetailNoJSONLDRsp))

	mux.HandleFunc("/user/api/search/result_work_list/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("page") == "2" && q.Get("p_t") == MockCompaniesTotal:
			serveJSON(http.StatusOK, mockCompaniesPage2Rsp)(w, r)
		case q.Get("page") == "2":
			// Live behavior for a page-2 request missing the correct p_t:
			// a stateless error, not a 4xx.
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write([]byte(`{"result":"error","error_message":"","searchResult":[]}`))
		default:
			serveJSON(http.StatusOK, mockCompaniesRsp)(w, r)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	return httptest.NewServer(mux)
}

func serveHTML(status int, data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(data)
	}
}

func serveJSON(status int, data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(data)
	}
}
