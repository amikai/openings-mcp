package jobindex

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsPreservesUpstreamKeys(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{Keyword: "backend", Page: 1})
	require.NoError(t, err)
	require.NotEmpty(t, got.Results)
	assert.Greater(t, got.Hitcount, 0)
	assert.Equal(t, 1, got.Page)
	// total_pages comes from Stash, not a local fallback.
	assert.Greater(t, got.TotalPages, 0)

	first := got.Results[0]
	// Upstream names, not renamed card schema.
	assert.Equal(t, "h1683131", first["tid"])
	assert.Equal(t, "Senior Backend Engineer", first["headline"])
	assert.Equal(t, "Aarhus N", first["area"])
	assert.Equal(t, "2026-07-15", first["firstdate"])
	assert.Equal(t, true, first["apply_deadline_asap"])
	assert.Equal(t, "2026-08-12", first["lastdate"])
	// share_url / url stay as upstream, not rewritten to a synthetic path only.
	assert.Equal(t, "https://www.jobindex.dk/vis-job/h1683131", first["share_url"])
	assert.Contains(t, first["url"], "jobindex.dk/c?t=h1683131")

	company, ok := first["company"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Whiteaway Group", company["name"])
	assert.Equal(t, "https://whiteawaygroup.career.emply.com/career-site", company["homeurl"])

	// Card HTML must not be present.
	_, hasHTML := first["html"]
	assert.False(t, hasHTML)
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
	require.NotEmpty(t, got.Results)
	assert.Equal(t, "h1675903", got.Results[0]["tid"])
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
	require.NotEmpty(t, got.Results)
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "h1683131")
	require.NoError(t, err)
	assert.Equal(t, "h1683131", got.Tid)
	assert.Equal(t, "Senior Backend Engineer", got.Headline)
	require.NotNil(t, got.Company)
	assert.Equal(t, "Whiteaway Group", got.Company["name"])
	assert.Equal(t, "Aarhus N", got.Area)
	assert.Equal(t, "2026-07-15", got.Firstdate)
	assert.Contains(t, got.Description, "Whiteaway Group")
	assert.Equal(t, "https://career.whiteawaygroup.com/ad/senior-backend-engineer/t0ta4w/en", got.ApplyURL)
	assert.Equal(t, "https://www.jobindex.dk/vis-job/h1683131", got.ShareURL)
	// Must not invent ASAP when the HTML page has no deadline label.
	assert.Empty(t, got.ApplyDeadline)
}

func TestJobDetailFromURL(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "https://www.jobindex.dk/vis-job/h1683131")
	require.NoError(t, err)
	assert.Equal(t, "h1683131", got.Tid)
}

func TestJobDetailRobot(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "r13911770")
	require.NoError(t, err)
	assert.Equal(t, "r13911770", got.Tid)
	assert.Equal(t, "Senior Software Developer", got.Headline)
	require.NotNil(t, got.Company)
	assert.NotEmpty(t, got.Company["name"])
	assert.Equal(t, "København", got.Area)
	assert.NotEmpty(t, got.Description)
	assert.NotEmpty(t, got.ApplyURL)
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
