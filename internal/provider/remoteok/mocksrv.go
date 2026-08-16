package remoteok

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_rsp.json
var _mockJobsRsp []byte

//go:embed testdata/jobs_tags_rsp.json
var _mockJobsTagsRsp []byte

//go:embed testdata/jobs_tags_empty_rsp.json
var _mockJobsTagsEmptyRsp []byte

// MockUnknownTag makes the mock server answer with the legal element
// alone, the way the real feed responds to a tag no job carries.
const MockUnknownTag = "doesnotexisttag12345"

// NewMockServer returns an httptest.Server that mimics the Remote OK
// feed with canned fixture responses, so tests never hit the real site.
// Any request with a tags filter gets the golang-filtered fixture,
// except MockUnknownTag which gets the legal-only fixture. The caller
// owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("tags") {
		case "":
			w.Write(_mockJobsRsp)
		case MockUnknownTag:
			w.Write(_mockJobsTagsEmptyRsp)
		default:
			w.Write(_mockJobsTagsRsp)
		}
	})
	return httptest.NewServer(mux)
}
