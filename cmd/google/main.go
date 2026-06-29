package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	google "github.com/amikai/job-mcp/internal/provider/google"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	keyword := strings.TrimSpace(scanner.Text())
	if keyword == "" {
		fmt.Fprintln(os.Stderr, "keyword is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := google.NewClient(http.DefaultClient)
	search, err := client.Jobs(ctx, defaultSearchParams(keyword))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	jobs := jobsForDetail(search.Jobs)
	details := make(map[string]*google.JobDetailResponse, len(jobs))
	for _, job := range jobs {
		detail, err := client.JobDetail(ctx, job.ID)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		details[job.ID] = detail
	}

	writeReport(os.Stdout, keyword, search, jobs, details)
}


func defaultSearchParams(keyword string) *google.JobsRequest {
	return &google.JobsRequest{
		Query:          keyword,
		EmploymentType: []string{"FULL_TIME"},
		SortBy:         "date",
		Page:           1,
	}
}

func jobsForDetail(jobs []google.Job) []google.Job {
	if len(jobs) > 10 {
		return jobs[:10]
	}
	return jobs
}

func writeReport(w io.Writer, keyword string, search *google.JobsResponse, jobs []google.Job, details map[string]*google.JobDetailResponse) {
	fmt.Fprintf(w, "Google Jobs Report\n")
	fmt.Fprintf(w, "Keyword: %s\n", keyword)
	fmt.Fprintf(w, "Filters: full-time, newest first\n")
	fmt.Fprintf(w, "Found %d jobs; showing %d\n\n", len(search.Jobs), len(jobs))

	for i, job := range jobs {
		fmt.Fprintf(w, "%d. [%s] %s\n", i+1, job.ID, job.Title)
		fmt.Fprintf(w, "URL: https://www.google.com/about/careers/applications/jobs/results/%s\n", job.ID)
		if job.Company != "" {
			fmt.Fprintf(w, "Company: %s\n", job.Company)
		}
		if job.Location != "" {
			fmt.Fprintf(w, "Location: %s\n", job.Location)
		}
		if detail := details[job.ID]; detail != nil {
			writeDetail(w, detail)
		}
		fmt.Fprintln(w)
	}
}

func writeDetail(w io.Writer, detail *google.JobDetailResponse) {
	if detail.About != "" {
		fmt.Fprintf(w, "About: %s\n", detail.About)
	}
	if detail.Qualifications != "" {
		fmt.Fprintf(w, "Qualifications: %s\n", detail.Qualifications)
	}
	if detail.Responsibilities != "" {
		fmt.Fprintf(w, "Responsibilities: %s\n", detail.Responsibilities)
	}
}
