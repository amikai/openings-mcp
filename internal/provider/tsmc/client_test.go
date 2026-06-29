package tsmc

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
	mux.HandleFunc("/zh_TW/careers/SearchJobs/", serveTestdata("testdata/search_jobs_rsp.html"))
	mux.HandleFunc("/zh_TW/careers/SearchJobs", serveTestdata("testdata/search_jobs_rsp.html"))
	mux.HandleFunc("/zh_TW/careers/JobDetail", serveTestdata("testdata/job_detail_rsp.html"))
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
	c := NewClient(Config{BaseURL: srv.URL})

	got, err := c.Jobs(t.Context(), &JobsRequest{
		Keyword:         "engineer",
		Locations:       []string{LocTaiwan},
		Categories:      []string{CatRD},
		JobTypes:        []string{JobTypeEngineer},
		EmploymentTypes: []string{EmployRegular},
	})
	require.NoError(t, err)

	want := &JobsResponse{Total: 22, Jobs: wantJobs}
	assert.Equal(t, want, got)
}

func TestJobDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := NewClient(Config{BaseURL: srv.URL})

	got, err := c.JobDetail(t.Context(), "21826")
	require.NoError(t, err)

	assert.Equal(t, wantDetail, got)
}
