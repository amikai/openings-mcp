package rippling

import (
	"net/url"
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

	res, err := client.ListJobs(t.Context(), ListJobsParams{BoardSlug: "pythian"})
	require.NoError(t, err)

	got, ok := res.(*ListJobsOKApplicationJSON)
	require.True(t, ok, "want *ListJobsOKApplicationJSON, got %T", res)

	entries := []JobListEntry(*got)
	require.Len(t, entries, 33)

	// The list carries one entry per (job, workLocation) pair: 33 entries
	// but only 12 distinct jobs on this board.
	distinct := make(map[string]struct{})
	for _, e := range entries {
		distinct[e.UUID.Value] = struct{}{}
	}
	assert.Len(t, distinct, 12)

	want := JobListEntry{
		UUID: NewOptString("144f31c4-38a4-4666-97b4-2c88a3f123da"),
		Name: NewOptString("DevOps Engineer"),
		Department: NewOptLabeledValue(LabeledValue{
			ID:    NewOptString("Managed Services"),
			Label: NewOptString("Managed Services"),
		}),
		URL: mustOptURI("https://ats.rippling.com/pythian/jobs/144f31c4-38a4-4666-97b4-2c88a3f123da"),
		WorkLocation: NewOptLabeledValue(LabeledValue{
			ID:    NewOptString("Poland"),
			Label: NewOptString("Poland"),
		}),
	}
	assert.Equal(t, want, entries[0])
}

func TestListJobsUnknownBoard(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.ListJobs(t.Context(), ListJobsParams{BoardSlug: "this-board-does-not-exist-xyz"})
	require.NoError(t, err)

	got, ok := res.(*BoardNotFoundError)
	require.True(t, ok, "want *BoardNotFoundError, got %T", res)
	assert.Equal(t, NewOptString("RESOURCE_NOT_FOUND"), got.ErrorCode)
}

func TestGetJob(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJob(t.Context(), GetJobParams{
		BoardSlug: "pythian",
		JobUUID:   "144f31c4-38a4-4666-97b4-2c88a3f123da",
	})
	require.NoError(t, err)

	got, ok := res.(*JobDetail)
	require.True(t, ok, "want *JobDetail, got %T", res)

	assert.Equal(t, NewOptString("144f31c4-38a4-4666-97b4-2c88a3f123da"), got.UUID)
	assert.Equal(t, NewOptString("DevOps Engineer"), got.Name)
	assert.Equal(t, NewOptString("Pythian"), got.CompanyName)
	assert.Equal(t, mustOptDateTime("2026-06-25T09:07:41.686-07:00"), got.CreatedOn)
	assert.Equal(t, []string{"India", "United Kingdom", "Poland", "Spain", "Romania"}, got.WorkLocations)
	assert.Equal(t, NewOptDepartment(Department{
		Name:           NewOptString("SRE"),
		BaseDepartment: NewOptString("Managed Services"),
		DepartmentTree: []string{"Managed Services", "SRE"},
	}), got.Department)
	// label carries the machine enum, id the human label — the swap
	// documented on the EmploymentType schema.
	assert.Equal(t, NewOptEmploymentType(EmploymentType{
		Label: NewOptNilString("SALARIED_FT"),
		ID:    NewOptString("Employee (Full-Time)"),
	}), got.EmploymentType)
	assert.Equal(t, mustOptURI("https://ats.rippling.com/pythian/jobs/144f31c4-38a4-4666-97b4-2c88a3f123da"), got.URL)
	assert.Empty(t, got.PayRangeDetails)

	desc, ok := got.Description.Get()
	require.True(t, ok)
	assert.Contains(t, desc.Company.Value, "Pythian")
	assert.Contains(t, desc.Role.Value, "DevOps")
}

func TestGetJobNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJob(t.Context(), GetJobParams{
		BoardSlug: "pythian",
		JobUUID:   "1b2c3d4e-5f60-4789-8abc-def012345678",
	})
	require.NoError(t, err)

	got, ok := res.(*JobNotFoundError)
	require.True(t, ok, "want *JobNotFoundError, got %T", res)
	assert.Equal(t, NewOptBool(false), got.Ok)
}

func mustOptDateTime(s string) OptDateTime {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return NewOptDateTime(tm)
}

func mustOptURI(s string) OptURI {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return NewOptURI(*u)
}
