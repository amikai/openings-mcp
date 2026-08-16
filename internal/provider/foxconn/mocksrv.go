package foxconn

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_workplace_rsp.json
var _mockJobsWorkplaceRsp []byte

//go:embed testdata/jobs_keyword_rsp.json
var _mockJobsKeywordRsp []byte

//go:embed testdata/jobs_empty_rsp.json
var _mockJobsEmptyRsp []byte

//go:embed testdata/job_detail_rsp.json
var _mockJobDetailRsp []byte

//go:embed testdata/job_not_found_rsp.json
var _mockJobNotFoundRsp []byte

//go:embed testdata/workplace_codes_rsp.json
var _mockWorkplaceCodesRsp []byte

//go:embed testdata/talent_zone_codes_rsp.json
var _mockTalentZoneCodesRsp []byte

// MockDetailID is the opaque detail id captured in job_detail_rsp.json.
const MockDetailID = "08de75d7bd7611a790b20d5b47c2f1bb"

// MockNotFoundID is an all-zero id matching the 404 fixture in
// job_not_found_rsp.json: the detail endpoint answers HTTP 404 with an RFC
// 7807 problem+json body for an unknown id.
const MockNotFoundID = "00000000000000000000000000000000"

// NewMockServer returns an httptest.Server serving canned Foxconn Taiwan
// careers API fixtures, so tests never hit the live API. All fixtures were
// captured from the live board (see testdata/*.hurl). The caller owns the
// server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	// List: dispatch on the same query params the live API filters on. An
	// unknown workplaceCode (or any query the fixtures don't cover) falls
	// through to the empty array, mirroring the API's no-404 behavior.
	mux.HandleFunc("/hh_recruit_tw_api/portal_api/JobVacancies", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("workplaceCode") == "CH":
			serveMockJSON(_mockJobsWorkplaceRsp)(w, r)
		case q.Get("keywords") == "ADAS":
			serveMockJSON(_mockJobsKeywordRsp)(w, r)
		default:
			serveMockJSON(_mockJobsEmptyRsp)(w, r)
		}
	})

	mux.HandleFunc("/hh_recruit_tw_api/portal_api/JobVacancies/"+MockDetailID, serveMockJSON(_mockJobDetailRsp))

	mux.HandleFunc("/hh_recruit_tw_api/portal_api/JobVacancies/"+MockNotFoundID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		w.Write(_mockJobNotFoundRsp)
	})

	mux.HandleFunc("/hh_recruit_tw_api/portal_api/Labels/Workplace/Codes", serveMockJSON(_mockWorkplaceCodesRsp))
	mux.HandleFunc("/hh_recruit_tw_api/portal_api/Labels/TalentZone/Codes", serveMockJSON(_mockTalentZoneCodesRsp))

	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Matches the live API's header; the generated decoder strips the
		// charset parameter before matching against the spec.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)
	}
}
