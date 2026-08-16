package mediatek

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
)

//go:embed testdata/jobs_rsp.json
var mockJobsResponse []byte

//go:embed testdata/jobs_keyword_rsp.json
var mockKeywordResponse []byte

//go:embed testdata/jobs_empty_rsp.json
var mockEmptyResponse []byte

//go:embed testdata/job_detail_rsp.html
var mockDetailResponse []byte

//go:embed testdata/job_detail_not_found_rsp.html
var mockNotFoundDetailResponse []byte

// NewMockServer returns a fixture-replaying MediaTek careers server. The
// caller owns the server and must close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/trpc/job.getJobs", func(w http.ResponseWriter, r *http.Request) {
		input, err := url.QueryUnescape(r.URL.Query().Get("input"))
		if err != nil {
			http.Error(w, "bad input", http.StatusBadRequest)
			return
		}
		var envelope struct {
			JSON struct {
				JobQueryInfo struct {
					Keywords []string `json:"keywords"`
				} `json:"jobQueryInfo"`
				Filters struct {
					Categorys []string `json:"categorys"`
					Locations []string `json:"locations"`
				} `json:"filters"`
			} `json:"json"`
		}
		if json.Unmarshal([]byte(input), &envelope) != nil {
			http.Error(w, "bad input", http.StatusBadRequest)
			return
		}
		var fixture []byte
		switch {
		case len(envelope.JSON.JobQueryInfo.Keywords) == 1 && envelope.JSON.JobQueryInfo.Keywords[0] == "AI":
			fixture = mockKeywordResponse
		case len(envelope.JSON.JobQueryInfo.Keywords) == 1 && envelope.JSON.JobQueryInfo.Keywords[0] == "__mtk_no_such_job_20260720__":
			fixture = mockEmptyResponse
		case len(envelope.JSON.Filters.Categorys) == 1 && envelope.JSON.Filters.Categorys[0] == "9020" && len(envelope.JSON.Filters.Locations) == 1 && envelope.JSON.Filters.Locations[0] == "0000009256":
			fixture = mockJobsResponse
		default:
			http.Error(w, "unknown fixture", http.StatusBadRequest)
			return
		}
		serveMockJSON(w, http.StatusOK, fixture)
	})
	mux.HandleFunc("/en/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/en/jobs/MTK120220511000" {
			serveMockHTML(w, http.StatusOK, mockDetailResponse)
			return
		}
		serveMockHTML(w, http.StatusInternalServerError, mockNotFoundDetailResponse)
	})
	return httptest.NewServer(mux)
}

func serveMockJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func serveMockHTML(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
