package ashby

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

// MockBoardName is the board slug the mock server serves fixtures for — the
// real board the testdata was captured from (see testdata/board_req.sh).
const MockBoardName = "browserbase"

//go:embed testdata/board_rsp.json
var mockBoardRsp []byte

//go:embed testdata/board_comp_rsp.json
var mockBoardCompRsp []byte

// NewMockServer returns an httptest.Server serving canned Ashby job-board
// fixture responses captured from a real board (see testdata/board_req.sh),
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
