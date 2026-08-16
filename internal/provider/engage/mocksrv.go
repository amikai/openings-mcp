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
var _mockBoardRsp []byte

//go:embed testdata/board_cap_rsp.html
var _mockBoardCapRsp []byte

//go:embed testdata/board_minimal_rsp.html
var _mockBoardMinimalRsp []byte

//go:embed testdata/board_not_found_rsp.html
var _mockBoardNotFoundRsp []byte

//go:embed testdata/job_detail_rsp.html
var _mockJobDetailRsp []byte

//go:embed testdata/job_detail_no_jsonld_rsp.html
var _mockJobDetailNoJSONLDRsp []byte

//go:embed testdata/job_not_found_rsp.html
var _mockJobNotFoundRsp []byte

//go:embed testdata/companies_rsp.json
var _mockCompaniesRsp []byte

//go:embed testdata/companies_page2_rsp.json
var _mockCompaniesPage2Rsp []byte

// NewMockServer returns an httptest.Server replaying captured en-gage.net
// responses, so tests never hit the live host. The caller owns the server
// and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/"+MockSlug+"/", serveHTML(http.StatusOK, _mockBoardRsp))
	mux.HandleFunc("/"+MockCapSlug+"/", serveHTML(http.StatusOK, _mockBoardCapRsp))
	mux.HandleFunc("/"+MockMinimalSlug+"/", serveHTML(http.StatusOK, _mockBoardMinimalRsp))
	mux.HandleFunc("/"+MockUnknownSlug+"/", serveHTML(http.StatusNotFound, _mockBoardNotFoundRsp))

	mux.HandleFunc("/"+MockSlug+"/work_"+MockWorkID+"/", serveHTML(http.StatusOK, _mockJobDetailRsp))
	mux.HandleFunc("/"+MockSlug+"/work_"+MockUnknownWorkID+"/", serveHTML(http.StatusNotFound, _mockJobNotFoundRsp))
	mux.HandleFunc("/"+MockNoJSONLDSlug+"/work_"+MockNoJSONLDWorkID+"/", serveHTML(http.StatusOK, _mockJobDetailNoJSONLDRsp))

	mux.HandleFunc("/user/api/search/result_work_list/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("page") == "2" && q.Get("p_t") == MockCompaniesTotal:
			serveJSON(http.StatusOK, _mockCompaniesPage2Rsp)(w, r)
		case q.Get("page") == "2":
			// Live behavior for a page-2 request missing the correct p_t:
			// a stateless error, not a 4xx.
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write([]byte(`{"result":"error","error_message":"","searchResult":[]}`))
		default:
			serveJSON(http.StatusOK, _mockCompaniesRsp)(w, r)
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
