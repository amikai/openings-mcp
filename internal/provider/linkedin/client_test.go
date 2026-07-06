package linkedin

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{
		Keywords: "software engineer",
		Location: "Taiwan",
	})
	require.NoError(t, err)

	assert.Equal(t, &JobsResponse{Jobs: wantJobs}, got)
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "4422697744")
	require.NoError(t, err)

	description := got.Description
	got.Description = ""

	assert.Equal(t, wantDetail, got)
	assert.Contains(t, description, "BoostDraft is a software engineering company")
}

// A cold JobDetail call (no prior Jobs on the same client) must prime the
// session first, mirroring LinkedIn 999-authwalling a cookieless jobs/view
// request that succeeds once it carries cookies from a prior request.
func TestJobDetailWarmsColdSession(t *testing.T) {
	var searched bool
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/search", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "primed"})
		searched = true
		w.Write([]byte("<html></html>"))
	})
	mux.HandleFunc("/jobs/view/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("JSESSIONID"); err != nil {
			w.WriteHeader(999)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(mockJobDetailRsp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	c := NewClient(srv.URL, &http.Client{Jar: jar})

	got, err := c.JobDetail(t.Context(), "4422697744")
	require.NoError(t, err)
	assert.True(t, searched, "cold JobDetail should prime the session via /jobs/search")
	assert.Equal(t, "4422697744", got.ID)
}

func TestJobDetailEmptyID(t *testing.T) {
	c := NewClient("https://example.invalid", http.DefaultClient)
	_, err := c.JobDetail(t.Context(), "")
	assert.Error(t, err)
}

// LinkedIn's real jobs/view/{id} endpoint returns this non-standard status
// for bot-suspected requests with no session cookies (observed while building
// this package's fixtures); getHTML must surface a clear error rather than
// try to parse the authwall redirect page as job HTML.
func TestGetHTMLBotBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(999)
		w.Write([]byte(`<html><head><script>window.location.href="/authwall";</script></head></html>`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.Client())
	_, err := c.getHTML(t.Context(), srv.URL+"/jobs/view/1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "999")
}
