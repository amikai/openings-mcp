package icims

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/search_rsp.html
var mockSearchRsp []byte

//go:embed testdata/search_filtered_rsp.html
var mockSearchFilteredRsp []byte

//go:embed testdata/search_location_rsp.html
var mockSearchLocationRsp []byte

//go:embed testdata/search_location_lorton_rsp.html
var mockSearchLocationLortonRsp []byte

//go:embed testdata/search_no_results_rsp.html
var mockSearchNoResultsRsp []byte

//go:embed testdata/search_posted_rsp.html
var mockSearchPostedRsp []byte

//go:embed testdata/search_unknown_company_rsp.html
var mockSearchUnknownCompanyRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_not_found_rsp.html
var mockJobDetailNotFoundRsp []byte

// NewMockServer returns an httptest.Server that replays captured iCIMS HTML
// fixtures. The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// searchLocation repeats with OR semantics, mirroring the live portal.
		locs := strings.ToLower(strings.Join(q["searchLocation"], " "))
		hasAustin := strings.Contains(locs, "austin")
		hasLorton := strings.Contains(locs, "lorton")
		switch {
		case q.Get("searchKeyword") == "zzzznonexistentkeyword12345":
			serveHTML(mockSearchNoResultsRsp)(w, r)
		case strings.Contains(strings.ToLower(q.Get("searchKeyword")), "posted"):
			// Peraton Lorton capture — one card with a posted-date span.
			serveHTML(mockSearchPostedRsp)(w, r)
		case hasAustin && hasLorton:
			// The union of both locations is the whole three-job board.
			serveHTML(mockSearchRsp)(w, r)
		case hasAustin:
			// Encoded value (12781-12827-Austin) — Austin-only jobs 1977, 1922.
			serveHTML(mockSearchLocationRsp)(w, r)
		case hasLorton:
			// Encoded value (12781-12830-Lorton) — Lorton-only job 1925.
			serveHTML(mockSearchLocationLortonRsp)(w, r)
		case strings.Contains(strings.ToLower(q.Get("searchKeyword")), "product"):
			serveHTML(mockSearchFilteredRsp)(w, r)
		default:
			serveHTML(mockSearchRsp)(w, r)
		}
	})
	// Unknown-tenant fixture is served from a separate host in live tests;
	// expose it under /unknown/jobs/search for unit coverage of 404 handling.
	mux.HandleFunc("/unknown/jobs/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(mockSearchUnknownCompanyRsp)
	})
	mux.HandleFunc("/jobs/999999999/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write(mockJobDetailNotFoundRsp)
	})
	mux.HandleFunc("/jobs/1977/", serveHTML(mockJobDetailRsp))
	return httptest.NewServer(mux)
}

func serveHTML(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}
}
