package synopsys

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/search-jobs/results", serveTestdata("testdata/jobs_rsp.json"))
	mux.HandleFunc("/job/", serveTestdata("testdata/job_detail_rsp.html"))
	return httptest.NewServer(mux)
}

func serveTestdata(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if strings.HasSuffix(path, ".json") {
			w.Header().Set("Content-Type", "application/json")
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		w.Write(data)
	}
}

func TestJobs(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.Jobs(t.Context(), &JobsRequest{Keywords: "software engineer"})
	require.NoError(t, err)

	assert.Equal(t, &JobsResponse{
		TotalResults: 604,
		TotalPages:   300,
		CurrentPage:  1,
		HasJobs:      true,
		HasContent:   true,
		Jobs: []Job{
			{Title: "Staff Software Engineer", Location: "Bengaluru, India", Category: "Engineering", Posted: "03/31/2026", DisplayID: "16567", JobID: "93498496944", City: "bengaluru", Slug: "staff-software-engineer"},
			{Title: "Staff Software Engineer", Location: "Bengaluru, India", Category: "Engineering", Posted: "03/31/2026", DisplayID: "16566", JobID: "93498496928", City: "bengaluru", Slug: "staff-software-engineer"},
		},
	}, got)
}

// hasJobs=true with zero parseable cards means the results markup changed
// (or a challenge slipped through); that must not look like an empty search.
func TestJobsErrorsWhenHasJobsButNoCards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"filters":"","results":"<section id=\"search-results\"></section>","hasJobs":true,"hasContent":true}`))
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	_, err := c.Jobs(t.Context(), &JobsRequest{Keywords: "software"})
	require.Error(t, err)
}

func TestJobDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.JobDetail(t.Context(), "bengaluru", "staff-software-engineer", "93498496944")
	require.NoError(t, err)

	assert.Equal(t, wantJobDetail, got)
}
