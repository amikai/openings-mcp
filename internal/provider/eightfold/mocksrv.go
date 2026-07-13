package eightfold

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/search_rsp.json
var mockSearchRsp []byte

//go:embed testdata/search_query_rsp.json
var mockSearchQueryRsp []byte

//go:embed testdata/search_filter_rsp.json
var mockSearchFilterRsp []byte

//go:embed testdata/position_details_rsp.json
var mockPositionDetailsRsp []byte

//go:embed testdata/position_details_not_found_rsp.json
var mockPositionNotFoundRsp []byte

// MockDomain is the tenant domain every fixture was captured against (see
// testdata/*.hurl). NewMockServer only answers this domain.
const MockDomain = "morganstanley.com"

// MockPositionID is the position_id embedded in search_rsp.json's first
// result and position_details_rsp.json, so a test can chain a search result
// straight into a detail fetch.
const MockPositionID = 549798858854

// NewMockServer returns an httptest.Server serving canned Eightfold PCSX
// fixture responses, so tests never hit the live API. All fixtures were
// captured from morganstanley.eightfold.ai (see testdata/*.hurl). The
// caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/pcsx/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("filter_businessarea") == "technology":
			serveMockJSON(mockSearchFilterRsp)(w, r)
		case q.Get("query") == "engineer" && q.Get("location") == "New York":
			serveMockJSON(mockSearchQueryRsp)(w, r)
		default:
			serveMockJSON(mockSearchRsp)(w, r)
		}
	})

	mux.HandleFunc("/api/pcsx/position_details", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("position_id") == "1" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write(mockPositionNotFoundRsp)
			return
		}
		serveMockJSON(mockPositionDetailsRsp)(w, r)
	})

	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
