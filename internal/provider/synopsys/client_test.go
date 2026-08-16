package synopsys

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
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.Jobs(t.Context(), &JobsRequest{Keywords: "software engineer"})
	require.NoError(t, err)

	want := &JobsResponse{
		TotalResults: 604,
		TotalPages:   300,
		CurrentPage:  1,
		HasJobs:      true,
		HasContent:   true,
		Jobs: []Job{
			{Title: "Staff Software Engineer", Location: "Bengaluru, India", Category: "Engineering", Posted: "03/31/2026", DisplayID: "16567", JobID: "93498496944", City: "bengaluru", Slug: "staff-software-engineer"},
			{Title: "Staff Software Engineer", Location: "Bengaluru, India", Category: "Engineering", Posted: "03/31/2026", DisplayID: "16566", JobID: "93498496928", City: "bengaluru", Slug: "staff-software-engineer"},
		},
	}
	assert.Equal(t, want, got)
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
	srv := NewMockServer()
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.JobDetail(t.Context(), MockCity, MockSlug, MockJobID)
	require.NoError(t, err)

	assert.Equal(t, _wantJobDetail, got)
}

func TestResolveLocation(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})

	got, err := c.ResolveLocation(t.Context(), MockLocationTerm)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Bengaluru, Karnataka, India", got[0].Value)

	// An unrecognized place is HTTP 200 with an empty array, not an error.
	got, err = c.ResolveLocation(t.Context(), "not a place")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestJobURL(t *testing.T) {
	assert.Equal(t,
		"https://careers.synopsys.com/job/bengaluru/staff-software-engineer/44408/93498496944",
		Job{City: MockCity, Slug: MockSlug, JobID: MockJobID}.URL())
}
