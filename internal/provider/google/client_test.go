package google

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/results/", serveTestdata("testdata/job_detail_rsp.html"))
	mux.HandleFunc("/jobs/results", serveTestdata("testdata/search_jobs_rsp.html"))
	return httptest.NewServer(mux)
}

func serveTestdata(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}

func TestJobs(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.Jobs(t.Context(), &JobsRequest{
		Query:          "software engineer",
		Locations:      []string{"Taiwan"},
		EmploymentType: []string{"FULL_TIME"},
		SortBy:         "date",
		Page:           1,
	})
	require.NoError(t, err)

	want := &JobsResponse{Jobs: wantJobs}
	assert.Equal(t, want, got)
}

func TestJobDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.JobDetail(t.Context(), "106863362666570438-software-engineer-gpu-system-software")
	require.NoError(t, err)

	assert.Equal(t, wantDetail, got)
}
