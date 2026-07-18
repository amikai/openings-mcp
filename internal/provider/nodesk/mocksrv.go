package nodesk

import (
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
)

//go:embed testdata/search_rsp.json
var mockSearchRsp []byte

//go:embed testdata/search_all_rsp.json
var mockSearchAllRsp []byte

//go:embed testdata/search_filtered_rsp.json
var mockSearchFilteredRsp []byte

//go:embed testdata/search_no_results_rsp.json
var mockSearchNoResultsRsp []byte

//go:embed testdata/facets_rsp.json
var mockFacetsRsp []byte

//go:embed testdata/search_missing_referer_rsp.json
var mockMissingRefererRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_notfound_rsp.html
var mockJobDetailNotFoundRsp []byte

// filteredFacetFilters is the exact facetFilters value the filtered
// fixture was captured with; the mock only serves that fixture for a
// byte-identical encoding, so a client encoding regression fails tests.
const filteredFacetFilters = `["searchFilter:remote-jobs/engineering","applicantLocationRegions:Remote - Europe"]`

// NewMockServer returns an httptest.Server that mimics both NoDesk hosts
// with canned fixture captures — the Algolia query endpoint (dispatched
// on the request's params, enforcing the real API's Referer lock) and the
// site's job detail pages. Point a client's algoliaBaseURL and
// siteBaseURL at it. The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /1/indexes/jobPosts/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") == "" {
			serveJSON(w, http.StatusForbidden, mockMissingRefererRsp)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var q struct {
			Params string `json:"params"`
		}
		if err := json.Unmarshal(body, &q); err != nil {
			http.Error(w, "malformed query body", http.StatusBadRequest)
			return
		}
		params, err := url.ParseQuery(q.Params)
		if err != nil {
			http.Error(w, "malformed params", http.StatusBadRequest)
			return
		}

		switch {
		case params.Has("facets"):
			serveJSON(w, http.StatusOK, mockFacetsRsp)
		case params.Has("facetFilters"):
			if params.Get("facetFilters") != filteredFacetFilters {
				http.Error(w, "unexpected facetFilters encoding: "+params.Get("facetFilters"), http.StatusBadRequest)
				return
			}
			serveJSON(w, http.StatusOK, mockSearchFilteredRsp)
		case params.Get("query") == "golang":
			serveJSON(w, http.StatusOK, mockSearchRsp)
		case params.Get("query") == "":
			serveJSON(w, http.StatusOK, mockSearchAllRsp)
		default:
			serveJSON(w, http.StatusOK, mockSearchNoResultsRsp)
		}
	})

	mux.HandleFunc("GET /remote-jobs/sticker-mule-software-engineer/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(mockJobDetailRsp)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write(mockJobDetailNotFoundRsp)
	})

	return httptest.NewServer(mux)
}

func serveJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	w.Write(body)
}
