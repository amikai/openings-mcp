package asus

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/jobs_rsp.html
var _mockJobsRsp []byte

//go:embed testdata/jobs_en_rsp.html
var _mockJobsEnRsp []byte

//go:embed testdata/jobs_filtered_rsp.html
var _mockJobsFilteredRsp []byte

//go:embed testdata/jobs_empty_rsp.html
var _mockJobsEmptyRsp []byte

//go:embed testdata/job_detail_rsp.html
var _mockJobDetailRsp []byte

//go:embed testdata/job_not_found_rsp.html
var _mockJobNotFoundRsp []byte

//go:embed testdata/cities_rsp.json
var _mockCitiesRsp []byte

// NewMockServer returns an httptest.Server mimicking the ASUS recruitment site.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/Jobs/Detail", func(w http.ResponseWriter, r *http.Request) {
		sn := r.URL.Query().Get("sn")
		if sn == "762c08de-1daa-4aa8-9668-d8a746ce24a8" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write(_mockJobDetailRsp)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(_mockJobNotFoundRsp)
	})

	mux.HandleFunc("/Jobs/GetCities", func(w http.ResponseWriter, r *http.Request) {
		country := r.URL.Query().Get("countryTw")
		if country == "TW" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write(_mockCitiesRsp)
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
			w.Write(_mockJobsEmptyRsp)
			return
		}
		if isEnglishLocale(r) {
			w.WriteHeader(http.StatusOK)
			w.Write(_mockJobsEnRsp)
			return
		}
		if kw != "" || r.URL.Query().Get("Location") != "" || len(r.URL.Query()["REQ_TYPEs_Prefix"]) > 0 {
			w.WriteHeader(http.StatusOK)
			w.Write(_mockJobsFilteredRsp)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(_mockJobsRsp)
	})

	return httptest.NewServer(mux)
}

// isEnglishLocale reports whether the session picked en-US at
// /Home/SetLanguage, which decides the locale of the labels — and so of the
// category filter values — the board renders. The real site keys this off a
// culture cookie whose name embeds a deployment-specific GUID
// ("hrisweb.<guid>"), so match on the value rather than pinning that name.
func isEnglishLocale(r *http.Request) bool {
	for _, c := range r.Cookies() {
		if strings.Contains(c.Value, "en-US") {
			return true
		}
	}
	return false
}
