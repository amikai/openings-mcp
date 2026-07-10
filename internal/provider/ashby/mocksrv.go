package ashby

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

// MockBoardName is the board slug the mock server serves fixtures for — the
// real board the testdata was captured from (see testdata/board_req.hurl).
const MockBoardName = "browserbase"

// MockNullsBoardName serves a fixture whose jobs carry null isRemote and
// workplaceType — fields the official docs claim are always present but many
// real boards null out (see testdata/board_req.hurl).
const MockNullsBoardName = "weaviate"

// MockNonRosterBoard is a board name deliberately absent from
// companies.yaml, so ats-layer tests can exercise non-roster behavior. It
// serves the same fixture as MockBoardName.
const MockNonRosterBoard = "somestartup"

//go:embed testdata/board_rsp.json
var mockBoardRsp []byte

//go:embed testdata/board_comp_rsp.json
var mockBoardCompRsp []byte

//go:embed testdata/board_nulls_rsp.json
var mockBoardNullsRsp []byte

// NewMockServer returns an httptest.Server serving canned Ashby job-board
// fixture responses captured from a real board (see testdata/board_req.hurl),
// so tests never hit the live API. The compensation fixture is served when
// the request sets includeCompensation=true; unknown boards get the same
// plain-text 404 the real API returns. The caller owns the server and must
// Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/posting-api/job-board/"+MockBoardName, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("includeCompensation") == "true" {
			serveMockJSON(mockBoardCompRsp)(w, r)
			return
		}
		serveMockJSON(mockBoardRsp)(w, r)
	})
	mux.HandleFunc("/posting-api/job-board/"+MockNullsBoardName, serveMockJSON(mockBoardNullsRsp))
	mux.HandleFunc("/posting-api/job-board/"+MockNonRosterBoard, serveMockJSON(mockBoardRsp))
	mux.HandleFunc("/posting-api/job-board/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	})
	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
