package delta

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/areas_rsp.json
var _mockAreasRsp []byte

//go:embed testdata/jobs_rsp.json
var _mockJobsRsp []byte

//go:embed testdata/jobs_area_filtered_rsp.json
var _mockJobsAreaFilteredRsp []byte

//go:embed testdata/jobs_keyword_filtered_rsp.json
var _mockJobsKeywordFilteredRsp []byte

//go:embed testdata/jobs_empty_rsp.json
var _mockJobsEmptyRsp []byte

//go:embed testdata/job_detail_rsp.json
var _mockJobDetailRsp []byte

//go:embed testdata/job_detail_notfound_rsp.json
var _mockJobDetailNotFoundRsp []byte

// NewMockServer returns an httptest.Server serving canned Delta Electronics
// careers fixture responses, so tests never hit the live site.
// All fixtures were captured from the live site (see testdata/*.hurl).
// The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/OAGateWay/RWSV2/api/Index/GetAreaList", func(w http.ResponseWriter, r *http.Request) {
		serveMockJSON(_mockAreasRsp)(w, r)
	})

	mux.HandleFunc("/OAGateWay/RWSV2/api/Index/SearchJobList", func(w http.ResponseWriter, r *http.Request) {
		areaID := r.URL.Query().Get("AreaID")
		keyword := r.URL.Query().Get("AddJobName")

		if keyword == "NONEXISTENT_KEYWORD_XYZ" {
			serveMockJSON(_mockJobsEmptyRsp)(w, r)
			return
		}
		if keyword == "軟體" {
			serveMockJSON(_mockJobsKeywordFilteredRsp)(w, r)
			return
		}
		if areaID == "A" {
			serveMockJSON(_mockJobsAreaFilteredRsp)(w, r)
			return
		}
		serveMockJSON(_mockJobsRsp)(w, r)
	})

	mux.HandleFunc("/OAGateWay/RWSV2/api/JobDetails/GetJobDetails", func(w http.ResponseWriter, r *http.Request) {
		empAddID := r.URL.Query().Get("EmpAddID")
		if empAddID == "C20260814001" {
			serveMockJSON(_mockJobDetailRsp)(w, r)
			return
		}
		serveMockJSON(_mockJobDetailNotFoundRsp)(w, r)
	})

	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)
	}
}
