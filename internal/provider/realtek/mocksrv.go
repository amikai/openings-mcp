package realtek

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var _mockJobsRsp []byte

//go:embed testdata/jobs_filtered_rsp.json
var _mockJobsFilteredRsp []byte

//go:embed testdata/types_rsp.json
var _mockTypesRsp []byte

//go:embed testdata/locations_rsp.json
var _mockLocationsRsp []byte

//go:embed testdata/job_detail_rsp.json
var _mockJobDetailRsp []byte

//go:embed testdata/job_detail_notfound_rsp.json
var _mockJobDetailNotFoundRsp []byte

// NewMockServer returns an httptest.Server serving canned Realtek
// recruitment site fixture responses, so tests never hit the live site.
// All fixtures were captured from the live site (see testdata/*.hurl).
// The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/Job/GetAllJobList", serveMockJSON(_mockJobsRsp))

	mux.HandleFunc("/Job/GetFilterList", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.PostFormValue("keyword") == "verification" {
			serveMockJSON(_mockJobsFilteredRsp)(w, r)
			return
		}
		serveMockJSON(_mockJobsRsp)(w, r)
	})

	mux.HandleFunc("/Job/GetAllTypeList", serveMockJSON(_mockTypesRsp))

	mux.HandleFunc("/Job/GetAllLocationList", serveMockJSON(_mockLocationsRsp))

	mux.HandleFunc("/Job/GetVacancyDetail", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("JobOppId") == "18" {
			serveMockJSON(_mockJobDetailRsp)(w, r)
			return
		}
		serveMockJSON(_mockJobDetailNotFoundRsp)(w, r)
	})

	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
