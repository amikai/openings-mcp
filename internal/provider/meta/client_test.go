package meta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.SearchJobs(t.Context(), SearchRequest{})
	require.NoError(t, err)

	require.NotEmpty(t, got.AllJobs)
	require.NotEmpty(t, got.FeaturedJobs)
	first := got.AllJobs[0]
	assert.Equal(t, Job{
		ID:        "1063741453022215",
		Title:     "Instagram Product Designer, Brand-in-Product",
		Locations: []string{"Menlo Park, CA", "New York, NY", "San Francisco, CA"},
		Teams:     []string{"Design & User Experience", "Creative"},
		SubTeams:  []string{"Design"},
	}, first)
}

func TestSearchJobsFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.SearchJobs(t.Context(), SearchRequest{
		Q:       "engineer",
		Offices: []string{"Singapore"},
	})
	require.NoError(t, err)

	require.NotEmpty(t, got.AllJobs)
	for _, job := range got.AllJobs {
		assert.Contains(t, job.Locations, "Singapore")
	}
}

func TestSearchFilters(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.SearchFilters(t.Context())
	require.NoError(t, err)

	assert.Contains(t, got.Teams, "Software Engineering")
	assert.Contains(t, got.Technologies, "Meta Quest")
	assert.Contains(t, got.Roles, "Full time employment")
	require.NotEmpty(t, got.Locations)
	assert.Equal(t, Location{
		ID:          "aiken-dc",
		DisplayName: "Aiken, SC",
		IsRemote:    false,
		State:       "South Carolina",
		Country:     "United States",
	}, got.Locations[0])
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), MockJobID)
	require.NoError(t, err)

	assert.Equal(t, MockJobID, got.ID)
	assert.Equal(t, "Instagram Product Designer, Brand-in-Product", got.Title)
	assert.Equal(t, []string{"Menlo Park, CA", "New York, NY", "San Francisco, CA"}, got.Locations)
	assert.Equal(t, []string{"Design & User Experience", "Creative"}, got.Departments)
	assert.Equal(t, []string{"Design"}, got.InternalDepartments)
	assert.Contains(t, got.DescriptionHTML, "Product Designer")
	assert.Len(t, got.MinimumQualifications, 5)
	assert.Len(t, got.PreferredQualifications, 7)
	assert.Len(t, got.Responsibilities, 7)
	require.Len(t, got.PublicCompensation, 1)
	comp := got.PublicCompensation[0]
	assert.Equal(t, "$201,000/year", comp.Minimum)
	assert.Equal(t, "$278,000/year", comp.Maximum)
	assert.Equal(t, "US", comp.CountryCode)
	assert.True(t, comp.HasBonus)
	assert.True(t, comp.HasEquity)
	assert.NotEmpty(t, got.BoilerplateIntroHTML)
	assert.NotEmpty(t, got.EqualOpportunityMessageHTML)
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), MockNotFoundJobID)
	require.ErrorIs(t, err, ErrJobNotFound)
}

func TestJobDetailEmptyID(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), "")
	require.Error(t, err)
}

func TestJobURL(t *testing.T) {
	assert.Equal(t, "https://www.metacareers.com/jobs/1063741453022215/", JobURL("1063741453022215"))
}
