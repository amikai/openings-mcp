package realtek

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/jobs_filtered_rsp.json
var mockJobsFilteredRsp []byte

//go:embed testdata/types_rsp.json
var mockTypesRsp []byte

//go:embed testdata/locations_rsp.json
var mockLocationsRsp []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_notfound_rsp.json
var mockJobDetailNotFoundRsp []byte

// NewMockServer returns an httptest.Server serving canned Realtek
// recruitment site fixture responses, so tests never hit the live site.
// All fixtures were captured from the live site (see testdata/*.hurl).
// The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/Job/GetAllJobList", serveMockJSON(mockJobsRsp))

	mux.HandleFunc("/Job/GetFilterList", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.PostFormValue("keyword") == "verification" {
			serveMockJSON(mockJobsFilteredRsp)(w, r)
			return
		}
		serveMockJSON(mockJobsRsp)(w, r)
	})

	mux.HandleFunc("/Job/GetAllTypeList", serveMockJSON(mockTypesRsp))

	mux.HandleFunc("/Job/GetAllLocationList", serveMockJSON(mockLocationsRsp))

	mux.HandleFunc("/Job/GetVacancyDetail", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("JobOppId") == "18" {
			serveMockJSON(mockJobDetailRsp)(w, r)
			return
		}
		serveMockJSON(mockJobDetailNotFoundRsp)(w, r)
	})

	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
