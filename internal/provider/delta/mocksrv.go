package delta

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/areas_rsp.json
var mockAreasRsp []byte

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/jobs_area_filtered_rsp.json
var mockJobsAreaFilteredRsp []byte

//go:embed testdata/jobs_keyword_filtered_rsp.json
var mockJobsKeywordFilteredRsp []byte

//go:embed testdata/jobs_empty_rsp.json
var mockJobsEmptyRsp []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_notfound_rsp.json
var mockJobDetailNotFoundRsp []byte

// NewMockServer returns an httptest.Server serving canned Delta Electronics
// careers fixture responses, so tests never hit the live site.
// All fixtures were captured from the live site (see testdata/*.hurl).
// The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/OAGateWay/RWSV2/api/Index/GetAreaList", func(w http.ResponseWriter, r *http.Request) {
		serveMockJSON(mockAreasRsp)(w, r)
	})

	mux.HandleFunc("/OAGateWay/RWSV2/api/Index/SearchJobList", func(w http.ResponseWriter, r *http.Request) {
		areaID := r.URL.Query().Get("AreaID")
		keyword := r.URL.Query().Get("AddJobName")

		if keyword == "NONEXISTENT_KEYWORD_XYZ" {
			serveMockJSON(mockJobsEmptyRsp)(w, r)
			return
		}
		if keyword == "軟體" {
			serveMockJSON(mockJobsKeywordFilteredRsp)(w, r)
			return
		}
		if areaID == "A" {
			serveMockJSON(mockJobsAreaFilteredRsp)(w, r)
			return
		}
		serveMockJSON(mockJobsRsp)(w, r)
	})

	mux.HandleFunc("/OAGateWay/RWSV2/api/JobDetails/GetJobDetails", func(w http.ResponseWriter, r *http.Request) {
		empAddID := r.URL.Query().Get("EmpAddID")
		if empAddID == "C20260814001" {
			serveMockJSON(mockJobDetailRsp)(w, r)
			return
		}
		serveMockJSON(mockJobDetailNotFoundRsp)(w, r)
	})

	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)
	}
}
