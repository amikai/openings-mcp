package weworkremotely

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/full_stack_rsp.xml
var mockFullStackRsp []byte

//go:embed testdata/design_rsp.xml
var mockDesignRsp []byte

//go:embed testdata/back_end_rsp.xml
var mockBackEndRsp []byte

// emptyFeed is served for every category not backed by a captured fixture,
// so [Client.AllJobs] can be exercised against the mock server without
// shipping a real capture for all 10 feeds.
const emptyFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss"><channel><title>empty</title></channel></rss>`

// NewMockServer returns an httptest.Server that mimics We Work Remotely's
// RSS feeds with canned fixture responses, so tests never hit the real
// site. Categories with a captured fixture serve it; every other known
// category serves an empty-but-valid feed. Any other path is HTTP 404. The
// caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	fixtures := map[string][]byte{
		"remote-full-stack-programming-jobs": mockFullStackRsp,
		"remote-design-jobs":                 mockDesignRsp,
		"remote-back-end-programming-jobs":   mockBackEndRsp,
	}

	mux := http.NewServeMux()
	for _, cat := range Categories {
		body, ok := fixtures[cat.Slug]
		if !ok {
			body = []byte(emptyFeed)
		}
		mux.HandleFunc("/categories/"+cat.Slug+".rss", serveRSS(body))
	}
	return httptest.NewServer(mux)
}

func serveRSS(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=UTF-8")
		w.Write(body)
	}
}
