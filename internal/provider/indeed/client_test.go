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
	assert.Equal(t, "Taiwan", first.Country)
	assert.Contains(t, first.JobTypes, "Full-time")
	assert.Nil(t, first.Compensation)

	// RangeType variants in jobs_rsp.json: unmarshaling any of these used
	// to abort the whole search with "unexpected concrete type ... AtLeast".
	require.NotNil(t, got.Jobs[1].Compensation)
	assert.Equal(t, 22.5, got.Jobs[1].Compensation.MinAmount)
	assert.Equal(t, 27.5, got.Jobs[1].Compensation.MaxAmount)
	assert.Equal(t, "HOUR", got.Jobs[1].Compensation.Interval)

	require.NotNil(t, got.Jobs[2].Compensation)
	assert.Equal(t, 15.0, got.Jobs[2].Compensation.MinAmount)
	assert.Equal(t, 0.0, got.Jobs[2].Compensation.MaxAmount)

	require.NotNil(t, got.Jobs[3].Compensation)
	assert.Equal(t, 17.5, got.Jobs[3].Compensation.MinAmount)
	assert.Equal(t, 17.5, got.Jobs[3].Compensation.MaxAmount)

	require.NotNil(t, got.Jobs[4].Compensation)
	assert.Equal(t, 0.0, got.Jobs[4].Compensation.MinAmount)
	assert.Equal(t, 30.0, got.Jobs[4].Compensation.MaxAmount)
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
	assert.Equal(t, DefaultCountryName, got.Jobs[0].Country)
}

func TestJobsUnknownCountry(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/graphql", srv.Client())

	_, err := c.Jobs(t.Context(), &JobsRequest{Keywords: "x", Country: "Narnia"})
	assert.Error(t, err)
}

func TestJobsUnsupportedIndeedCountry(t *testing.T) {
	// Formerly accepted via the jobspy table; live API rejects as invalid site.
	_, ok := CountryByName("Slovenia")
	assert.False(t, ok)
	_, ok = CountryByName("Bangladesh")
	assert.False(t, ok)
}

func TestJobsZeroRadius(t *testing.T) {
	// Explicit 0 must not be rewritten to the 25-mile default. The mock does
	// not assert request variables, but the client path must accept *int(0).
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL+"/graphql", srv.Client())
	zero := 0
	got, err := c.Jobs(t.Context(), &JobsRequest{
		Keywords:    "software engineer",
		Location:    "Taipei",
		Country:     "Taiwan",
		RadiusMiles: &zero,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, got.Jobs)
}

func TestJobDetailRequiresCountry(t *testing.T) {
	c := NewClient("https://example.invalid", nil)
	_, err := c.JobDetail(t.Context(), "", "9d503ca7fe211430")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "country is required")
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
