package main

import (
	"bytes"
	"strings"
	"testing"

	google "github.com/amikai/job-mcp/internal/provider/google"
	"github.com/stretchr/testify/assert"
)

func TestFormatReportIncludesEveryGoogleJobDetail(t *testing.T) {
	search := &google.JobsResponse{
		Jobs: []google.Job{
			{ID: "104030745835512518", Title: "Model UX Designer", Company: "Google", Location: "Mountain View, CA, USA"},
			{ID: "126340255522398918", Title: "Senior Engineer, GDC", Company: "Google", Location: "Sunnyvale, CA, USA", Remote: true},
		},
	}
	details := map[string]*google.JobDetailResponse{
		"104030745835512518": {ID: "104030745835512518", Title: "Model UX Designer", Company: "Google", Location: "Mountain View, CA, USA", About: "Design AI product UX.", Qualifications: "Bachelor's degree.", Responsibilities: "Create model UX flows."},
		"126340255522398918": {ID: "126340255522398918", Title: "Senior Engineer, GDC", Company: "Google", Location: "Sunnyvale, CA, USA", About: "Build backend services."},
	}

	var buf bytes.Buffer
	writeReport(&buf, "software engineer", "https://www.google.com/about/careers/applications", search, jobsForDetail(search.Jobs), details)
	got := buf.String()

	for _, want := range []string{
		"Google Jobs Report",
		"Query: software engineer",
		"Found 2 jobs; showing 2",
		"[104030745835512518] Model UX Designer",
		"URL: https://www.google.com/about/careers/applications/jobs/results/104030745835512518",
		"Company: Google",
		"Location: Mountain View, CA, USA",
		"About: Design AI product UX.",
		"Qualifications: Bachelor's degree.",
		"Responsibilities: Create model UX flows.",
		"[126340255522398918] Senior Engineer, GDC",
		"Remote eligible",
		"Build backend services.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("report missing %q:\n%s", want, got)
		}
	}
}

func TestJobsForDetailLimitsGoogleJobsToTen(t *testing.T) {
	jobs := make([]google.Job, 12)
	got := jobsForDetail(jobs)
	if len(got) != 10 {
		t.Fatalf("jobsForDetail returned %d jobs, want 10", len(got))
	}
}

func TestBuildJobsRequest(t *testing.T) {
	got := buildJobsRequest("software engineer", "Taiwan", true, "MID", "Python", "MASTERS", "FULL_TIME", "Google", "date", 2)

	want := &google.JobsRequest{
		Query:          "software engineer",
		Locations:      []string{"Taiwan"},
		HasRemote:      true,
		TargetLevels:   []string{"MID"},
		Skills:         "Python",
		Degrees:        []string{"MASTERS"},
		EmploymentType: []string{"FULL_TIME"},
		Companies:      []string{"Google"},
		SortBy:         "date",
		Page:           2,
	}
	assert.Equal(t, want, got)
}

func TestBuildJobsRequestLeavesUnsetFiltersOut(t *testing.T) {
	got := buildJobsRequest("software engineer", "", false, "", "", "", "", "", "", 1)

	want := &google.JobsRequest{
		Query: "software engineer",
		Page:  1,
	}
	assert.Equal(t, want, got)
}
