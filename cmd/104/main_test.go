package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/stretchr/testify/assert"
)

func TestFormatReportIncludesEveryJobDetail(t *testing.T) {
	search := &job104.JobsResponse{}
	search.Metadata.Pagination.Total = 2
	search.Metadata.Pagination.CurrentPage = 1
	search.Metadata.Pagination.LastPage = 1
	search.Data = []job104.Job{
		{JobName: "Go Engineer", CustName: "Acme", JobAddrNoDesc: "Taipei"},
		{JobName: "Backend Engineer", CustName: "Beta", JobAddrNoDesc: "Remote"},
	}
	search.Data[0].Link.Job = "https://www.104.com.tw/job/abc123"
	search.Data[1].Link.Job = "https://www.104.com.tw/job/def456"

	details := map[string]*job104.JobDetailResponse{
		"abc123": detail("Go Engineer", "Build Go services"),
		"def456": detail("Backend Engineer", "Build APIs"),
	}

	var buf bytes.Buffer
	writeReport(&buf, "Golang", search, jobsForDetail(search.Data), details)
	got := buf.String()

	for _, want := range []string{
		"104 Jobs Report",
		"Keyword: Golang",
		"Filters: full-time, non-remote",
		"Found 2 jobs (page 1/1); showing 2",
		"[abc123] Go Engineer",
		"Build Go services",
		"[def456] Backend Engineer",
		"Build APIs",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("report missing %q:\n%s", want, got)
		}
	}
}

func detail(title, description string) *job104.JobDetailResponse {
	d := &job104.JobDetailResponse{}
	d.Data.Header.JobName = title
	d.Data.JobDetail.JobDescription = description
	return d
}

func TestJobCodeFromURL(t *testing.T) {
	got := jobCodeFromURL("https://www.104.com.tw/job/abc123?jobsource=foo")
	if got != "abc123" {
		t.Fatalf("jobCodeFromURL() = %q", got)
	}
}

func TestJobsForDetailLimitsToTen(t *testing.T) {
	jobs := make([]job104.Job, 12)
	got := jobsForDetail(jobs)
	if len(got) != 10 {
		t.Fatalf("jobsForDetail returned %d jobs, want 10", len(got))
	}
}

func TestJobsForDetailSkipsRemoteJobs(t *testing.T) {
	jobs := []job104.Job{
		{JobName: "Onsite"},
		{JobName: "Partial", RemoteWorkType: 1},
		{JobName: "Full Remote", RemoteWorkType: 2},
		{JobName: "Another Onsite"},
	}

	got := jobsForDetail(jobs)
	want := []job104.Job{
		{JobName: "Onsite"},
		{JobName: "Another Onsite"},
	}
	assert.Equal(t, want, got)
}

func TestDefaultSearchParamsUseFullTime(t *testing.T) {
	got := defaultSearchParams("Golang")
	ro := 0
	want := &job104.JobsRequest{
		Keyword: "Golang",
		RO:      &ro,
	}
	assert.Equal(t, want, got)
}
