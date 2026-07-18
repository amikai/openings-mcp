package join

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Jobs(t.Context(), MockCompanyID)
	require.NoError(t, err)
	require.Len(t, got, 3)

	first := got[0]
	assert.Equal(t, "16397229-senior-software-engineer-backend-llm-infrastructure", first.IdParam)
	assert.Equal(t, "Senior Software Engineer (Backend/LLM Infrastructure)", first.Title)
	assert.Equal(t, "ONLINE", first.Status)
	assert.Equal(t, "ONSITE", first.WorkplaceType)
	assert.Equal(t, "Berlin", first.City)
	assert.Equal(t, "Germany", first.Country)
	assert.Equal(t, "Software Development", first.Category)
	assert.Equal(t, "Employee", first.EmploymentType)
	assert.False(t, first.CreatedAt.IsZero())
}

func TestJobsEmptyCompany(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Jobs(t.Context(), MockEmptyCompanyID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), MockJobSlug, MockJobIdParam)
	require.NoError(t, err)
	assert.Equal(t, "Senior Software Engineer (Backend/LLM Infrastructure)", got.Title)
	assert.Equal(t, MockJobIdParam, got.IdParam)
	assert.Equal(t, "ONSITE", got.WorkplaceType)
	assert.Contains(t, got.Description, "Routine Labs")
	assert.Contains(t, got.Description, "## Tasks")
	assert.Contains(t, got.Description, "## Skills")
}

func TestJobDetailRemoteAnywhere(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), MockRemoteJobSlug, MockRemoteJobIdParam)
	require.NoError(t, err)
	assert.Equal(t, "REMOTE", got.WorkplaceType)
	assert.Equal(t, "ANYWHERE", got.RemoteType)
	// city/country still carry the employer's base location even though
	// the role has no location restriction — a caller must not treat
	// City alone as "this job is on-site there" (see API.md).
	assert.Equal(t, "Berlin", got.City)
	assert.NotEmpty(t, got.Country)
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), "join", "00000000-nonexistent-job")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestJobDetailEmptyIdParam(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), "join", "")
	require.Error(t, err)
}
