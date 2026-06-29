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
			{ID: "126340255522398918", Title: "Senior Engineer, GDC", Company: "Google", Location: "Sunnyvale, CA, USA"},
		},
	}
	details := map[string]*google.JobDetailResponse{
		"104030745835512518": {ID: "104030745835512518", Title: "Model UX Designer", Company: "Google", Location: "Mountain View, CA, USA", About: "Design AI product UX.", Qualifications: "Bachelor's degree.", Responsibilities: "Create model UX flows."},
		"126340255522398918": {ID: "126340255522398918", Title: "Senior Engineer, GDC", Company: "Google", Location: "Sunnyvale, CA, USA", About: "Build backend services."},
	}

	var buf bytes.Buffer
	writeReport(&buf, "software engineer", search, jobsForDetail(search.Jobs), details)
	got := buf.String()

	for _, want := range []string{
		"Google Jobs Report",
		"Keyword: software engineer",
		"Filters: full-time, newest first",
		"Found 2 jobs; showing 2",
		"[104030745835512518] Model UX Designer",
		"Company: Google",
		"Location: Mountain View, CA, USA",
		"About: Design AI product UX.",
		"Qualifications: Bachelor's degree.",
		"Responsibilities: Create model UX flows.",
		"[126340255522398918] Senior Engineer, GDC",
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

func TestDefaultSearchParamsUsesFullTimeNewest(t *testing.T) {
	got := defaultSearchParams("software engineer")
	want := &google.JobsRequest{
		Query:          "software engineer",
		SortBy:         "date",
		EmploymentType: []string{"FULL_TIME"},
		Page:           1,
	}
	assert.Equal(t, want, got)
}
