package indeed

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/graphql", srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{
		Keywords: "software engineer",
		Location: "Taipei",
		Country:  "Taiwan",
		Limit:    5,
	})
	require.NoError(t, err)
	require.Len(t, got.Jobs, 5)
	assert.NotEmpty(t, got.NextCursor)

	first := got.Jobs[0]
	assert.Equal(t, "9d503ca7fe211430", first.Key)
	assert.Equal(t, "Senior Staff Engineer System Application Engineering", first.Title)
	assert.Equal(t, "Infineon Technologies", first.Company)
	assert.Equal(t, "https://tw.indeed.com/cmp/Infineon-Technologies", first.CompanyURL)
	assert.Equal(t, "https://tw.indeed.com/viewjob?jk=9d503ca7fe211430", first.JobURL)
	assert.Equal(t, "2026-06-04", first.PostedDate)
	assert.Contains(t, first.JobTypes, "Full-time")
	assert.Nil(t, first.Compensation)
}

func TestJobsFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/graphql", srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{
		Keywords: "software engineer",
		Location: "Taipei",
		Country:  "Taiwan",
		HoursOld: 24,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, got.Jobs)
}

func TestJobsDefaultsCountry(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/graphql", srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{Keywords: "software engineer"})
	require.NoError(t, err)
	assert.NotEmpty(t, got.Jobs)
}

func TestJobsUnknownCountry(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/graphql", srv.Client())

	_, err := c.Jobs(t.Context(), &JobsRequest{Keywords: "x", Country: "Narnia"})
	assert.Error(t, err)
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/graphql", srv.Client())

	got, err := c.JobDetail(t.Context(), "Taiwan", "9d503ca7fe211430")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "9d503ca7fe211430", got.Key)
	assert.Equal(t, "Senior Staff Engineer System Application Engineering", got.Title)
	assert.Equal(t, "Infineon Technologies", got.Company)
	assert.Equal(t, "https://tw.indeed.com/viewjob?jk=9d503ca7fe211430", got.JobURL)
	assert.Equal(t, "https://www.infineon.com/", got.CompanyWebsite)
	assert.Equal(t, "10,000+", got.CompanyEmployees)
	assert.Contains(t, got.CompanyLogo, "squarelogo")
	assert.Contains(t, got.ApplyURL, "jobs.infineon.com")
	assert.Contains(t, got.Description, "Your Role")

	assert.Equal(t, Location{Country: "台灣", CountryCode: "TW", State: "TPE", City: "台北市", Formatted: "台北市"}, got.Location)
	assert.Equal(t, "Infineon Technologies", got.Source)
	assert.Equal(t, "2026-07-14", got.DateIndexed)
	assert.Equal(t, []string{"Neubiberg"}, got.CompanyAddresses)
	assert.Equal(t, "Jochen Hanebeck", got.CompanyCEO)
	assert.Contains(t, got.CompanyCEOPhoto, "ceophoto")
	assert.Contains(t, got.CompanyBannerImage, "headerimage")
	assert.Empty(t, got.DetailedSalary)
	assert.Empty(t, got.WorkSchedule)
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/graphql", srv.Client())

	got, err := c.JobDetail(t.Context(), "Taiwan", MockNotFoundJobKey)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestJobDetailEmptyKey(t *testing.T) {
	c := NewClient("https://example.invalid", nil)
	_, err := c.JobDetail(t.Context(), "Taiwan", "")
	assert.Error(t, err)
}
