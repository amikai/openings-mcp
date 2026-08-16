package successfactors

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/search_rsp.html
var mockSearchRsp []byte

//go:embed testdata/search_filtered_rsp.html
var mockSearchFilteredRsp []byte

//go:embed testdata/search_no_results_rsp.html
var mockSearchNoResultsRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_not_found_rsp.html
var mockJobDetailNotFoundRsp []byte

//go:embed testdata/facet_values_rsp.json
var mockFacetValuesRsp []byte

// NewMockServer returns an httptest.Server that mimics a SuccessFactors
// Career Site Builder tenant with canned fixture responses, keyed off the
// request's job ID / query so tests never hit a live site. The caller owns
// the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/job/999999999/999999999/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/errorpage/?errortype=Exception", http.StatusFound)
	})
	mux.HandleFunc("/errorpage/", serveMockHTML(mockJobDetailNotFoundRsp))
	mux.HandleFunc("/job/1414343333/1414343333/", serveMockHTML(mockJobDetailRsp))
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("q") == "zzzznonexistentkeyword12345":
			serveMockHTML(mockSearchNoResultsRsp)(w, r)
		case q.Get("optionsFacetsDD_department") != "" || q.Get("optionsFacetsDD_country") != "":
			serveMockHTML(mockSearchFilteredRsp)(w, r)
		default:
			serveMockHTML(mockSearchRsp)(w, r)
		}
	})
	mux.HandleFunc("/services/jobs/options/facetValues/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(mockFacetValuesRsp)
	})
	return httptest.NewServer(mux)
}

func serveMockHTML(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}
