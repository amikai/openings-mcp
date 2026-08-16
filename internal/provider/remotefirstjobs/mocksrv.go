package remotefirstjobs

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/search_rsp.json
var mockSearchRsp []byte

//go:embed testdata/search_query_page_rsp.json
var mockSearchQueryPageRsp []byte

//go:embed testdata/search_no_results_rsp.json
var mockSearchNoResultsRsp []byte

//go:embed testdata/search_invalid_category_rsp.json
var mockSearchInvalidCategoryRsp []byte

//go:embed testdata/search_page_out_of_range_rsp.json
var mockSearchPageOutOfRangeRsp []byte

// NewMockServer returns an httptest.Server replaying canned
// RemoteFirstJobs API fixture responses, so tests never hit the live
// API. All fixtures were captured live on 2026-07-19 (see
// testdata/*.hurl). Routing mirrors the live behavior for exactly the
// captured parameter combinations; any other combination falls back to
// the default page-0 fixture. The caller owns the server and must Close
// it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/search-jobs", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch {
		case q.Get("category") == "doesnotexist":
			w.WriteHeader(http.StatusBadRequest)
			w.Write(mockSearchInvalidCategoryRsp)
		case q.Get("page") == "5":
			w.WriteHeader(http.StatusBadRequest)
			w.Write(mockSearchPageOutOfRangeRsp)
		case q.Get("query") == "zzzzqqqqxxxx":
			w.Write(mockSearchNoResultsRsp)
		case q.Get("query") == "golang" && q.Get("page") == "1":
			w.Write(mockSearchQueryPageRsp)
		default:
			w.Write(mockSearchRsp)
		}
	})
	return httptest.NewServer(mux)
}
