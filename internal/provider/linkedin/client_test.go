package linkedin

import (
	"net/http"
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
