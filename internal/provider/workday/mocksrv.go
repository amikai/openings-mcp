package workday

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/nvidia_jobs_rsp.json
var MockNvidiaJobsRsp []byte

//go:embed testdata/nvidia_job_detail_rsp.json
var MockNvidiaJobDetailRsp []byte

//go:embed testdata/trendmicro_jobs_rsp.json
var MockTrendMicroJobsRsp []byte

//go:embed testdata/trendmicro_job_detail_rsp.json
var MockTrendMicroJobDetailRsp []byte

func NewMockServer(mockJobsRsp, mockJobDetailRsp []byte) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", serveMockJSON(mockJobsRsp))
	mux.HandleFunc("/job/{location}/{titleSlug}", serveMockJSON(mockJobDetailRsp))
	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
