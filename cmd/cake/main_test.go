package main

import (
	"bytes"
	"strings"
	"testing"

	cake "github.com/amikai/job-mcp/internal/provider/cake"
	"github.com/stretchr/testify/assert"
)

func TestFormatReportIncludesEveryJobDetail(t *testing.T) {
	search := &cake.JobSearchResponse{
		TotalEntries: 2,
		TotalPages:   1,
		PerPage:      20,
		CurrentPage:  1,
		Data: []cake.JobSearchItem{
			{Path: "go-engineer", Title: "Go Engineer", Description: "Go preview"},
			{Path: "backend-engineer", Title: "Backend Engineer", Description: "Backend preview"},
		},
	}
	details := map[string]*cake.JobDetail{
		"go-engineer":      {Path: "go-engineer", Title: "Go Engineer", Description: "<p>Build Go services</p>", Requirements: "<p>Go</p>"},
		"backend-engineer": {Path: "backend-engineer", Title: "Backend Engineer", Description: "<p>Build APIs</p>", Requirements: ""},
	}

	var buf bytes.Buffer
	writeReport(&buf, "Golang", search, jobsForDetail(search.Data), details)
	got := buf.String()

	for _, want := range []string{
		"Cake Jobs Report",
		"Keyword: Golang",
		"Filters: full-time, non-remote",
		"Found 2 jobs (page 1/1); showing 2",
		"[go-engineer] Go Engineer",
		"Build Go services",
		"Requirements: Go",
		"[backend-engineer] Backend Engineer",
		"Build APIs",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("report missing %q:\n%s", want, got)
		}
	}
}

func TestJobsForDetailLimitsToTen(t *testing.T) {
	jobs := make([]cake.JobSearchItem, 12)
	got := jobsForDetail(jobs)
	if len(got) != 10 {
		t.Fatalf("jobsForDetail returned %d jobs, want 10", len(got))
	}
}

func TestDefaultSearchRequestUsesFullTimeAndNoRemote(t *testing.T) {
	got := defaultSearchRequest("Golang")
	want := cake.JobSearchRequest{
		Query:  "Golang",
		SortBy: cake.JobSearchRequestSortByPopularity,
		Filters: cake.JobSearchRequestFilters{
			"job_types": []byte(`["full_time"]`),
			"remote":    []byte(`["no_remote_work"]`),
		},
	}
	assert.Equal(t, want, got)
}
