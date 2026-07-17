package jobindex

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobs(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{Keyword: "backend", Page: 1})
	require.NoError(t, err)
	require.NotEmpty(t, got.Jobs)
	assert.Greater(t, got.TotalCount, 0)
	assert.Equal(t, 1, got.Page)

	first := got.Jobs[0]
	assert.Equal(t, "h1683131", first.ID)
	assert.Equal(t, "Senior Backend Engineer", first.Title)
	assert.Equal(t, "Whiteaway Group", first.Company)
	assert.Equal(t, "Aarhus N", first.Location)
	assert.Equal(t, "2026-07-15", first.PostedDate)
	assert.Equal(t, "ASAP", first.Deadline)
	assert.Equal(t, "https://www.jobindex.dk/vis-job/h1683131", first.URL)
}

func TestJobsFiltered(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{
		Keyword:    "python",
		JobAgeDays: 7,
		Sort:       SortDate,
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.Jobs)
	assert.Equal(t, "h1675903", got.Jobs[0].ID)
}

func TestJobsAreaPath(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{
		Keyword: "backend",
		Area:    "storkoebenhavn",
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.Jobs)
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "h1683131")
	require.NoError(t, err)
	assert.Equal(t, "h1683131", got.ID)
	assert.Equal(t, "Senior Backend Engineer", got.Title)
	assert.Equal(t, "Whiteaway Group", got.Company)
	assert.Equal(t, "Aarhus N", got.Location)
	assert.Equal(t, "2026-07-15", got.PostedDate)
	assert.Contains(t, got.Description, "Whiteaway Group")
	assert.Equal(t, "https://career.whiteawaygroup.com/ad/senior-backend-engineer/t0ta4w/en", got.ApplyURL)
	assert.Equal(t, "https://www.jobindex.dk/vis-job/h1683131", got.URL)
}

func TestJobDetailFromURL(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "https://www.jobindex.dk/vis-job/h1683131")
	require.NoError(t, err)
	assert.Equal(t, "h1683131", got.ID)
}

func TestJobDetailEmptyID(t *testing.T) {
	c := NewClient("https://example.invalid", http.DefaultClient)
	_, err := c.JobDetail(t.Context(), "")
	assert.Error(t, err)
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), "missing999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInvalidSort(t *testing.T) {
	c := NewClient("https://example.invalid", http.DefaultClient)
	_, err := c.Jobs(t.Context(), &JobsRequest{Sort: "nope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sort")
}
