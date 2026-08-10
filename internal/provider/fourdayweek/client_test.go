package fourdayweek

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{Limit: NewOptInt(2)})
	require.NoError(t, err)

	list, ok := res.(*JobList)
	require.True(t, ok, "expected a JobList, got %T", res)

	assert.Equal(t, 1, list.Page)
	assert.Equal(t, 2, list.Limit)
	assert.Equal(t, 22207, list.Total)
	assert.True(t, list.HasMore)
	require.Len(t, list.Data, 2)

	first := list.Data[0]
	assert.Equal(t, "material-project-manager-at-general-atomics-6b4b2cac", first.Slug)
	assert.Equal(t, "Material Project Manager", first.Title.Value)
	assert.Equal(t, JobWorkArrangementOnsite, first.WorkArrangement)
	// schedule_type stays a plain string: the upstream documents the domain
	// in prose but leaves the field open.
	assert.Equal(t, "9_day_fortnight", first.ScheduleType.Value)
	// Descriptions are Markdown, never HTML.
	assert.Contains(t, first.Description.Value, "## ")
	assert.NotContains(t, first.Description.Value, "<p")

	require.Len(t, first.Locations, 1)
	assert.Equal(t, "Poway", first.Locations[0].City.Value)
	assert.Equal(t, "United States", first.Locations[0].Country.Value)
	assert.Equal(t, "onsite", first.Locations[0].WorkArrangement.Value)
	assert.True(t, first.Locations[0].IsPrimary.Value)

	assert.Equal(t, "general-atomics", first.Company.Value.Slug)
	assert.Equal(t, "onsite", first.Company.Value.RemoteLevel.Value)
	assert.False(t, first.Company.Value.HiresWorldwide.Value)
}

func TestSearchJobsFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{
		WorkArrangement: NewOptString("remote"),
		Country:         NewOptString("Germany"),
		Schedule:        NewOptString("4_day_week"),
		Limit:           NewOptInt(2),
	})
	require.NoError(t, err)

	list, ok := res.(*JobList)
	require.True(t, ok, "expected a JobList, got %T", res)

	// The filters narrow the board server-side, from 22207 to 3.
	assert.Equal(t, 3, list.Total)
	require.Len(t, list.Data, 2)
	for _, job := range list.Data {
		assert.Equal(t, JobWorkArrangementRemote, job.WorkArrangement)
		assert.Equal(t, "4_day_week", job.ScheduleType.Value)
	}

	// country matches any location regardless of that location's own
	// arrangement, so the filter pair does not mean "remote-workable from
	// Germany": this job's German location is an onsite Berlin office and
	// its remote-allowed location is France.
	leaked := list.Data[1]
	assert.Equal(t, "Netherlands", leaked.Company.Value.Country.Value)
	require.Len(t, leaked.Locations, 2)
	assert.Equal(t, "France", leaked.Locations[0].Country.Value)
	assert.Equal(t, "remote", leaked.Locations[0].WorkArrangement.Value)
	assert.Equal(t, "Germany", leaked.Locations[1].Country.Value)
	assert.Equal(t, "onsite", leaked.Locations[1].WorkArrangement.Value)
}

func TestSearchJobsInvalidSort(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{Sort: NewOptSearchJobsSort("bogus")})
	require.NoError(t, err)

	// An unrecognized sort is a hard 400, not a fallback to the default.
	errRes, ok := res.(*ErrorResponse)
	require.True(t, ok, "expected an ErrorResponse, got %T", res)
	assert.Equal(t, 400, errRes.Error.Code)
	assert.Equal(t, "invalid sort", errRes.Error.Message)
}

func TestGetJob(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJob(t.Context(), GetJobParams{Slug: MockJobSlug})
	require.NoError(t, err)

	job, ok := res.(*Job)
	require.True(t, ok, "expected a Job, got %T", res)

	assert.Equal(t, MockJobSlug, job.Slug)
	assert.Equal(t, "Senior Infrastructure Engineer", job.Title.Value)
	assert.Equal(t, JobWorkArrangementRemote, job.WorkArrangement)
	assert.Equal(t, JobContractTypePermanent, job.ContractType.Value)
	assert.Equal(t, 32, job.HoursPerWeekMin.Value)

	// Fully remote with no office: locations is absent, not empty — which is
	// why it cannot be a required field.
	assert.Empty(t, job.Locations)
	assert.Equal(t, []string{"UTC+0"}, job.Timezones)

	// Salary is in cents, unlike the whole-dollar salary_min/salary_max
	// query filters: 16459500 is $164,595/year.
	assert.Equal(t, int64(16459500), job.SalaryMin.Value)
	assert.Equal(t, int64(21274400), job.SalaryMax.Value)
	assert.Equal(t, "USD", job.SalaryCurrency.Value)
	assert.Equal(t, JobSalaryPeriodYear, job.SalaryPeriod.Value)

	assert.Equal(t, "asynchronous-communication", job.Skills[0].Slug)
	assert.Equal(t, "buffer", job.Company.Value.Slug)
	assert.Equal(t, "fully-remote", job.Company.Value.RemoteLevel.Value)
	assert.True(t, job.Company.Value.HiresWorldwide.Value)
}

func TestGetJobNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJob(t.Context(), GetJobParams{Slug: MockUnknownJobSlug})
	require.NoError(t, err)

	errRes, ok := res.(*ErrorResponse)
	require.True(t, ok, "expected an ErrorResponse, got %T", res)
	assert.Equal(t, 404, errRes.Error.Code)
	assert.Equal(t, "Job not found", errRes.Error.Message)
}
