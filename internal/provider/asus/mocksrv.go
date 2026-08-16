package asus

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.html
var mockJobsRsp []byte

//go:embed testdata/jobs_filtered_rsp.html
var mockJobsFilteredRsp []byte

//go:embed testdata/jobs_empty_rsp.html
var mockJobsEmptyRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

//go:embed testdata/job_not_found_rsp.html
var mockJobNotFoundRsp []byte

//go:embed testdata/cities_rsp.json
var mockCitiesRsp []byte

// NewMockServer returns an httptest.Server mimicking the ASUS recruitment site.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/Jobs/Detail", func(w http.ResponseWriter, r *http.Request) {
		sn := r.URL.Query().Get("sn")
		if sn == "762c08de-1daa-4aa8-9668-d8a746ce24a8" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write(mockJobDetailRsp)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(mockJobNotFoundRsp)
	})

	mux.HandleFunc("/Jobs/GetCities", func(w http.ResponseWriter, r *http.Request) {
		country := r.URL.Query().Get("countryTw")
		if country == "TW" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write(mockCitiesRsp)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("[]"))
	})

	mux.HandleFunc("/Jobs", func(w http.ResponseWriter, r *http.Request) {
		kw := r.URL.Query().Get("Keyword")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if kw == "xyz999foobar" {
			w.WriteHeader(http.StatusOK)
			w.Write(mockJobsEmptyRsp)
			return
		}
		if kw != "" || r.URL.Query().Get("Location") != "" || len(r.URL.Query()["REQ_TYPEs_Prefix"]) > 0 {
			w.WriteHeader(http.StatusOK)
			w.Write(mockJobsFilteredRsp)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(mockJobsRsp)
	})

	return httptest.NewServer(mux)
}
