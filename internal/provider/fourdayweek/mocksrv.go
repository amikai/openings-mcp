package fourdayweek

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/search_rsp.json
var mockSearchRsp []byte

//go:embed testdata/search_filtered_rsp.json
var mockSearchFilteredRsp []byte

//go:embed testdata/search_invalid_sort_rsp.json
var mockSearchInvalidSortRsp []byte

//go:embed testdata/detail_rsp.json
var mockDetailRsp []byte

//go:embed testdata/detail_not_found_rsp.json
var mockDetailNotFoundRsp []byte

// MockJobSlug is the slug served by NewMockServer's detail endpoint, matching
// testdata/detail_rsp.json. It is a fully remote job, so it exercises the
// case where a listing carries no locations array at all.
const MockJobSlug = "senior-infrastructure-engineer-at-buffer-38679a55"

// MockUnknownJobSlug is a slug deliberately absent from 4dayweek.io, matching
// testdata/detail_not_found_rsp.json.
const MockUnknownJobSlug = "no-such-job-xyz-000"

// NewMockServer returns an httptest.Server serving canned 4dayweek.io API
// fixture responses, so tests never hit the live API. All fixtures were
// captured live on 2026-07-31 (see testdata/*.hurl). The caller owns the
// server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v2/jobs", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("sort") != "" && q.Get("sort") != "date" && q.Get("sort") != "salary":
			serveMockJSON(w, http.StatusBadRequest, mockSearchInvalidSortRsp)
		case q.Get("work_arrangement") == "remote" && q.Get("country") == "Germany":
			serveMockJSON(w, http.StatusOK, mockSearchFilteredRsp)
		default:
			serveMockJSON(w, http.StatusOK, mockSearchRsp)
		}
	})

	mux.HandleFunc("/api/v2/jobs/", func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/api/v2/jobs/") {
		case MockJobSlug:
			serveMockJSON(w, http.StatusOK, mockDetailRsp)
		default:
			serveMockJSON(w, http.StatusNotFound, mockDetailNotFoundRsp)
		}
	})

	return httptest.NewServer(mux)
}

func serveMockJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(data)
}
