package rippling

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var _mockJobsRsp []byte

//go:embed testdata/job_detail_rsp.json
var _mockJobDetailRsp []byte

//go:embed testdata/job_not_found_rsp.json
var _mockJobNotFoundRsp []byte

//go:embed testdata/jobs_unknown_board_rsp.json
var _mockJobsUnknownBoardRsp []byte

// MockNonRosterBoard is a board slug deliberately absent from
// companies.yaml, so ats-layer tests can exercise non-roster behavior.
const MockNonRosterBoard = "somestartup"

// NewMockServer returns an httptest.Server serving canned Rippling Job
// Board API fixture responses, so tests never hit a live board. All
// fixtures were captured from Pythian's live board (see testdata/*.hurl);
// MockNonRosterBoard replays the same fixtures under a non-roster slug.
// The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	for _, board := range []string{"pythian", MockNonRosterBoard} {
		mux.HandleFunc("/board/"+board+"/jobs", serveMockJSON(http.StatusOK, _mockJobsRsp))
		mux.HandleFunc("/board/"+board+"/jobs/144f31c4-38a4-4666-97b4-2c88a3f123da", serveMockJSON(http.StatusOK, _mockJobDetailRsp))
		mux.HandleFunc("/board/"+board+"/jobs/1b2c3d4e-5f60-4789-8abc-def012345678", serveMockJSON(http.StatusNotFound, _mockJobNotFoundRsp))
	}

	mux.HandleFunc("/board/this-board-does-not-exist-xyz/jobs", serveMockJSON(http.StatusNotFound, _mockJobsUnknownBoardRsp))

	return httptest.NewServer(mux)
}

func serveMockJSON(status int, data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(data)
	}
}
