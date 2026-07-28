package flowxtra

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.ListJobs(t.Context(), ListJobsParams{PerPage: NewOptInt(3)})
	require.NoError(t, err)

	assert.True(t, res.Success)
	assert.Equal(t, "200", res.Message)
	assert.Equal(t, 1, res.Data.CurrentPage)
	assert.Equal(t, 39, res.Data.LastPage)
	assert.Equal(t, 3, res.Data.PerPage)
	assert.Equal(t, 116, res.Data.Total)
	require.Len(t, res.Data.Data, 3)

	first := res.Data.Data[0]
	assert.Equal(t, "M88PB", first.HasID)
	assert.Equal(t, "Operario/a de envasado", first.Title)
	assert.Equal(t, "Arogreen", first.NameCompany)
	assert.Equal(t, "On-site", first.Workplace)
	assert.Equal(t, "EUR", first.Currency)
	// date_share is RFC3339 with fractional seconds; see openapi.yaml.
	assert.True(t, first.DateShare.Equal(time.Date(2026, time.July, 23, 16, 22, 27, 0, time.UTC)))
	assert.Equal(t, NewNilFloat64(21000), first.MinSalary)
	assert.True(t, first.MaxSalary.Null)
	assert.True(t, first.Salary.Null)
	assert.Equal(t, "year", first.RateSalary)
	// city_company may be empty while state/country are set.
	assert.Empty(t, first.CityCompany)
	assert.Equal(t, "Barcelona", first.StateCompany)
	assert.Equal(t, "Spain", first.CountryCompany)
	require.Len(t, first.EmploymentTypes, 1)
	assert.Equal(t, "Full-time", first.EmploymentTypes[0].Name)
	// List rows always carry the flowxtra.com apply URL (the detail
	// endpoint serves the company's own career-page URL instead).
	assert.Equal(t, "https://flowxtra.com/apply/M88PB", first.UrlJobApplay)

	assert.Equal(t, "Remote", res.Data.Data[1].Workplace)
}

// TestListJobsFiltered guards the server-side narrowing path: a
// search-key request returns a smaller board, not the full dump.
func TestListJobsFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.ListJobs(t.Context(), ListJobsParams{
		SearchKey: NewOptString("sales"),
		PerPage:   NewOptInt(3),
	})
	require.NoError(t, err)

	assert.Equal(t, 4, res.Data.Total)
	require.NotEmpty(t, res.Data.Data)
	first := res.Data.Data[0]
	assert.Equal(t, "sales", first.Title)
	assert.Equal(t, "3S Spring", first.NameCompany)
	assert.Equal(t, "Remote", first.Workplace)
}

func TestGetJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobDetail(t.Context(), GetJobDetailParams{HasId: "M88PB"})
	require.NoError(t, err)

	envelope, ok := res.(*JobDetailEnvelope)
	require.True(t, ok, "expected *JobDetailEnvelope, got %T", res)
	assert.True(t, envelope.Success)

	job := envelope.Data
	assert.Equal(t, "M88PB", job.HasID)
	assert.Equal(t, "Operario/a de envasado", job.Title)
	assert.Equal(t, "On-site", job.Workplace)
	assert.Equal(t, NewOptNilString("Midlevel"), job.Seniority)
	assert.Contains(t, job.Description, "Arogreen")
	assert.Equal(t, "EUR", job.Currency)
	// date_expired is present-but-null on open-ended postings.
	assert.True(t, job.DateExpired.Null)
	assert.Equal(t, NewNilFloat64(21000), job.MinSalary)
	// Detail serves the company's own career-page apply URL.
	assert.Equal(t, "https://arogreen.postular.link/job/operarioa-de-envasado/M88PB", job.UrlJobApplay)

	assert.Equal(t, "Arogreen", job.Company.Name)
	assert.Equal(t, NewOptNilString("https://www.arogreen.com"), job.Company.Website)
	assert.Equal(t, NewOptNilString("postular.link"), job.Company.CareerDomain)

	require.Len(t, job.JobTypeJob, 1)
	assert.Equal(t, "Full-time", job.JobTypeJob[0].Name)

	require.True(t, job.CompanyOffice.Set)
	office := job.CompanyOffice.Value
	assert.Equal(t, NewOptNilString("Flassaders 18, Santa Perpètua de Mogoda"), office.Address)
	require.True(t, office.Country.Set)
	assert.Equal(t, "Spain", office.Country.Value.Name)
	require.True(t, office.State.Set)
	assert.Equal(t, "Barcelona", office.State.Value.Name)
}

// TestGetJobDetailNotFound guards the clean JSON 404 for unknown ids.
func TestGetJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobDetail(t.Context(), GetJobDetailParams{HasId: "ZZZZZ99"})
	require.NoError(t, err)

	notFound, ok := res.(*NotFoundError)
	require.True(t, ok, "expected *NotFoundError, got %T", res)
	assert.False(t, notFound.Success)
	assert.Equal(t, "Central job not found", notFound.Message)
}
