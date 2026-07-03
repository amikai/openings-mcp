package main

import (
	"bytes"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSearchParamsUnfilteredByDefault(t *testing.T) {
	got, err := buildSearchParams("Golang", "", "", "", nil, "", nil, 0)
	require.NoError(t, err)
	want := job104.SearchJobsParams{Keyword: job104.NewOptString("Golang")}
	assert.Equal(t, want, got)
}

func TestBuildSearchParamsResolvesLabels(t *testing.T) {
	got, err := buildSearchParams("Golang", "Taipei", "Full-time", "Newest", []string{"University", "Master"}, "Partial", []string{"Day", "Night"}, 2)
	require.NoError(t, err)

	want := job104.SearchJobsParams{
		Keyword:    job104.NewOptString("Golang"),
		Area:       job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
		Ro:         job104.NewOptSearchJobsRo(job104.SearchJobsRo1),
		Order:      job104.NewOptSearchJobsOrder(job104.SearchJobsOrder2),
		Page:       job104.NewOptInt(2),
		Edu:        []job104.SearchJobsEduItem{job104.SearchJobsEduItem4, job104.SearchJobsEduItem5},
		RemoteWork: job104.NewOptSearchJobsRemoteWork(job104.SearchJobsRemoteWork2),
		S9:         []job104.SearchJobsS9Item{job104.SearchJobsS9Item1, job104.SearchJobsS9Item2},
	}
	assert.Equal(t, want, got)
}

func TestBuildSearchParamsUnknownEduLabel(t *testing.T) {
	_, err := buildSearchParams("", "", "", "", []string{"Bogus"}, "", nil, 0)
	require.ErrorContains(t, err, "--edu")
}

func TestBuildSearchParamsUnknownS9Label(t *testing.T) {
	_, err := buildSearchParams("", "", "", "", nil, "", []string{"Bogus"}, 0)
	require.ErrorContains(t, err, "--s9")
}

func TestBuildSearchParamsPageZeroLeavesPageUnset(t *testing.T) {
	got, err := buildSearchParams("", "", "", "", nil, "", nil, 0)
	require.NoError(t, err)
	assert.False(t, got.Page.Set)
}

func TestWriteDetail(t *testing.T) {
	d := detail("Go Engineer", "Build Go services")
	d.Data.JobDetail.Salary = job104.NewOptString("60k-80k")
	d.Data.Condition.WorkExp = job104.NewOptString("3 years")
	d.Data.Condition.Edu = job104.NewOptString("Bachelor")

	var buf bytes.Buffer
	writeDetail(&buf, d)
	got := buf.String()

	for _, want := range []string{
		"Salary: 60k-80k",
		"Experience: 3 years | Education: Bachelor",
		"Build Go services",
	} {
		assert.Contains(t, got, want)
	}
}

func detail(title, description string) *job104.JobDetailResponse {
	d := &job104.JobDetailResponse{}
	d.Data.Header.JobName = title
	d.Data.JobDetail.JobDescription = job104.NewOptString(description)
	return d
}
