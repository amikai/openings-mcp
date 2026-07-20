package foxconn

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListJobVacancies covers the happy path: a workplaceCode-filtered
// list. The five always-present fields decode as plain (non-optional)
// values, and job_type/job_type_name ARE populated on the list endpoint
// (the opposite of the detail endpoint — see TestGetJobVacancy).
func TestListJobVacancies(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	jobs, err := client.ListJobVacancies(t.Context(), ListJobVacanciesParams{
		WorkplaceCode: NewOptString("CH"),
	})
	require.NoError(t, err)
	require.Len(t, jobs, 2)

	first := jobs[0]
	assert.Equal(t, "8c1889c0bda64b18b9725e0a94aa2eae", first.ID)
	assert.Equal(t, "PP21081600019", first.JobNo)
	assert.Equal(t, "【A事業群】 設備採購工程師/主管", first.JobName)
	assert.Equal(t, "CH", first.Loc)
	assert.Equal(t, "大陸", first.LocName)

	// job_type/job_type_name are non-null on the list endpoint.
	jt, ok := first.JobType.Get()
	require.True(t, ok, "want job_type populated on the list endpoint")
	assert.Equal(t, "TALENTS,MA", jt)
	jtn, ok := first.JobTypeName.Get()
	require.True(t, ok)
	assert.Equal(t, "新幹班,一般招募(社招/顧問)", jtn)

	// The second job carries the optional loc_desc; the first does not.
	_, ok = first.LocDesc.Get()
	assert.False(t, ok, "want first job's loc_desc absent/null")
	locDesc, ok := jobs[1].LocDesc.Get()
	require.True(t, ok)
	assert.Equal(t, "鄭州", locDesc)
}

// TestListJobVacanciesKeyword proves keywords= is a real server-side filter
// that narrows the full ~953-job board down to a handful, not an ignored
// parameter.
func TestListJobVacanciesKeyword(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	jobs, err := client.ListJobVacancies(t.Context(), ListJobVacanciesParams{
		Keywords: NewOptString("ADAS"),
	})
	require.NoError(t, err)
	require.Len(t, jobs, 6)

	for _, j := range jobs {
		assert.NotEmpty(t, j.JobNo)
	}
}

// TestListJobVacanciesEmpty guards the no-404 quirk: an unknown/invalid
// filter value is HTTP 200 with an empty array, not an error.
func TestListJobVacanciesEmpty(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	jobs, err := client.ListJobVacancies(t.Context(), ListJobVacanciesParams{
		WorkplaceCode: NewOptString("ZZZINVALID"),
	})
	require.NoError(t, err)
	assert.Empty(t, jobs)
}

// TestGetJobVacancy covers the detail happy path and the documented
// list/detail null flip: job_type/job_type_name are null here even though
// they were populated on the list endpoint for this same job, while many
// detail-only fields (edu_level, treat_desc, expect_date, ...) are now set.
func TestGetJobVacancy(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobVacancy(t.Context(), GetJobVacancyParams{ID: MockDetailID})
	require.NoError(t, err)

	got, ok := res.(*JobVacancy)
	require.True(t, ok, "want *JobVacancy, got %T", res)

	assert.Equal(t, MockDetailID, got.ID)
	assert.Equal(t, "PP26022700001", got.JobNo)

	// The list/detail null flip: both come back null on detail.
	_, ok = got.JobType.Get()
	assert.False(t, ok, "want job_type null on the detail endpoint")
	_, ok = got.JobTypeName.Get()
	assert.False(t, ok, "want job_type_name null on the detail endpoint")

	// Detail-only fields the list endpoint never populates.
	edu, ok := got.EduLevel.Get()
	require.True(t, ok, "want edu_level populated on the detail endpoint")
	assert.Equal(t, "C", edu)
	eduDisplay, ok := got.EduLevelNameAndDesc.Get()
	require.True(t, ok)
	assert.Equal(t, "大學(含)以上", eduDisplay)
	treat, ok := got.TreatDesc.Get()
	require.True(t, ok)
	assert.Equal(t, "面議", treat)
	expect, ok := got.ExpectDate.Get()
	require.True(t, ok)
	assert.Equal(t, "2026-02-27T00:00:00+08:00", expect)
	req, ok := got.Desc3.Get()
	require.True(t, ok)
	assert.Contains(t, req, "CAN/LIN")
}

// TestGetJobVacancyNotFound guards the RFC 7807 problem+json 404: an
// unknown detail id returns a ProblemDetails, not a JobVacancy — the
// opposite of the list endpoint's empty-array-no-404 behavior. traceId is
// per-request and unstable, so it is deliberately not asserted.
func TestGetJobVacancyNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobVacancy(t.Context(), GetJobVacancyParams{ID: MockNotFoundID})
	require.NoError(t, err)

	got, ok := res.(*ProblemDetails)
	require.True(t, ok, "want *ProblemDetails, got %T", res)

	assert.Equal(t, NewOptInt(404), got.Status)
	assert.Equal(t, NewOptString("Not Found"), got.Title)
}
