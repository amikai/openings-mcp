package main

import (
	"strings"
	"testing"

	google "github.com/amikai/job-mcp/internal/provider/google"
)

func TestFormatReportIncludesEveryGoogleJobDetail(t *testing.T) {
	search := &google.SearchResponse{
		Jobs: []google.Job{
			{ID: "104030745835512518", Path: "104030745835512518-model-ux-designer", Title: "Model UX Designer", Company: "Google", Location: "Mountain View, CA, USA"},
			{ID: "126340255522398918", Path: "126340255522398918-senior-engineer-gdc", Title: "Senior Engineer, GDC", Company: "Google", Location: "Sunnyvale, CA, USA"},
		},
	}
	details := map[string]*google.JobDetail{
		"104030745835512518": {ID: "104030745835512518", Path: "104030745835512518-model-ux-designer", Title: "Model UX Designer", Company: "Google", Location: "Mountain View, CA, USA", About: "Design AI product UX.", Qualifications: "Bachelor's degree.", Responsibilities: "Create model UX flows."},
		"126340255522398918": {ID: "126340255522398918", Path: "126340255522398918-senior-engineer-gdc", Title: "Senior Engineer, GDC", Company: "Google", Location: "Sunnyvale, CA, USA", About: "Build backend services."},
	}

	got := formatReport("software engineer", search, jobsForDetail(search.Jobs), details)

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
	params := defaultSearchParams("software engineer")
	if params.Query != "software engineer" {
		t.Fatalf("Query = %q", params.Query)
	}
	if params.SortBy != "date" {
		t.Fatalf("SortBy = %q", params.SortBy)
	}
	if len(params.EmploymentType) != 1 || params.EmploymentType[0] != "FULL_TIME" {
		t.Fatalf("EmploymentType = %#v", params.EmploymentType)
	}
	if params.Page != 1 {
		t.Fatalf("Page = %d", params.Page)
	}
}
