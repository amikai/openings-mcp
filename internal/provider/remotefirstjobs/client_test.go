package remotefirstjobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	return client
}

func TestSearchJobs(t *testing.T) {
	client := newTestClient(t)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{})
	require.NoError(t, err)
	result, ok := res.(*SearchJobsResult)
	require.True(t, ok)

	assert.Equal(t, 0, result.Page)
	assert.Equal(t, 100, result.JobsCount)
	require.Len(t, result.Jobs, 100)
	assert.Contains(t, result.README.Value, "Remote First Jobs API")

	first := result.Jobs[0]
	assert.Equal(t, "senior-product-manager-ai-platform-837752", first.ID)
	assert.Equal(t, "https://remotefirstjobs.com/companies/campminder/jobs/senior-product-manager-ai-platform-837752", first.URL)
	assert.Equal(t, "CampMinder", first.CompanyName)
	assert.Equal(t, NewOptNilString("https://cdn.remotefirstjobs.com/logo/campminder-e075-0.webp"), first.CompanyLogo)
	assert.Equal(t, "Senior Product Manager, AI Platform", first.Title)
	assert.Equal(t, "product", first.Category)
	assert.Equal(t, "senior", first.Seniority)
	assert.Contains(t, first.Description, "Campminder")
	assert.Equal(t, NewOptNilInt(180000), first.SalaryMin)
	assert.Equal(t, NewOptNilInt(220000), first.SalaryMax)
	assert.Equal(t, []string{"United States"}, first.Locations)
	// No timezone offset — plain string on purpose, see openapi.yaml.
	assert.Equal(t, "2026-07-17T21:33:30", first.PublishedAt)

	// locations may be empty — 84 of 500 sampled jobs, see openapi.yaml.
	assert.Equal(t, "associate-copywriting-837749", result.Jobs[3].ID)
	assert.Empty(t, result.Jobs[3].Locations)
}

func TestSearchJobsQueryAndPage(t *testing.T) {
	client := newTestClient(t)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{
		Query: NewOptString("golang"),
		Page:  NewOptInt(1),
	})
	require.NoError(t, err)
	result, ok := res.(*SearchJobsResult)
	require.True(t, ok)

	assert.Equal(t, 1, result.Page)
	require.Len(t, result.Jobs, 100)
	assert.Equal(t, "staff-software-engineer-agentic-ai-systems-moveworks-822985", result.Jobs[0].ID)
}

// TestSearchJobsNoResults guards the zero-hit quirk: HTTP 200 with jobs
// null, not an empty array and not an error.
func TestSearchJobsNoResults(t *testing.T) {
	client := newTestClient(t)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{
		Query: NewOptString("zzzzqqqqxxxx"),
	})
	require.NoError(t, err)
	result, ok := res.(*SearchJobsResult)
	require.True(t, ok)

	assert.Equal(t, 0, result.JobsCount)
	assert.Nil(t, result.Jobs)
}

func TestSearchJobsInvalidCategory(t *testing.T) {
	client := newTestClient(t)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{
		Category: NewOptString("doesnotexist"),
	})
	require.NoError(t, err)
	apiErr, ok := res.(*Error)
	require.True(t, ok)

	assert.Equal(t, "invalid_argument", apiErr.Kind)
	assert.Contains(t, apiErr.Message, "invalid category")
}

func TestFindJob(t *testing.T) {
	client := newTestClient(t)

	// Found on the first (default) page — no narrowing needed.
	job, err := client.FindJob(t.Context(), "associate-copywriting-837749", FindOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Associate, Copywriting", job.Title)
	assert.Equal(t, "United States Department of Defense", job.CompanyName)

	// Found on page 1 of the narrowed search that surfaced it (the mock
	// serves the query fixture only for query=golang&page=1, so this
	// exercises the multi-page scan).
	job, err = client.FindJob(t.Context(), "staff-software-engineer-agentic-ai-systems-moveworks-822985", FindOptions{Query: "golang"})
	require.NoError(t, err)
	assert.NotEmpty(t, job.Title)
}

func TestFindJobNotFound(t *testing.T) {
	client := newTestClient(t)

	_, err := client.FindJob(t.Context(), "no-such-job-000000", FindOptions{})
	require.ErrorContains(t, err, `job "no-such-job-000000" not found`)
}

// TestFindJobSurfacesAPIError proves a 400 during the scan comes back as
// the API's own error, not a generic decode failure.
func TestFindJobSurfacesAPIError(t *testing.T) {
	client := newTestClient(t)

	_, err := client.FindJob(t.Context(), "anything", FindOptions{Category: "doesnotexist"})
	require.ErrorContains(t, err, "invalid category")
}
