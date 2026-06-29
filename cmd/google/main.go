package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	google "github.com/amikai/job-mcp/internal/provider/google"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	keyword, err := keywordFromInput(os.Args[1:], os.Stdin)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := google.NewClient(http.DefaultClient)
	search, err := client.Jobs(ctx, defaultSearchParams(keyword))
	if err != nil {
		return err
	}

	jobs := jobsForDetail(search.Jobs)
	details := make(map[string]*google.JobDetailResponse, len(jobs))
	for _, job := range jobs {
		detail, err := client.JobDetail(ctx, job.Path)
		if err != nil {
			return err
		}
		details[job.ID] = detail
	}

	fmt.Print(formatReport(keyword, search, jobs, details))
	return nil
}

func keywordFromInput(args []string, stdin *os.File) (string, error) {
	if len(args) > 0 {
		return strings.TrimSpace(strings.Join(args, " ")), nil
	}
	fmt.Fprint(os.Stderr, "Keyword: ")
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("keyword is required")
	}
	keyword := strings.TrimSpace(scanner.Text())
	if keyword == "" {
		return "", fmt.Errorf("keyword is required")
	}
	return keyword, nil
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

func formatReport(keyword string, search *google.JobsResponse, jobs []google.Job, details map[string]*google.JobDetailResponse) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Google Jobs Report\n")
	fmt.Fprintf(&sb, "Keyword: %s\n", keyword)
	fmt.Fprintf(&sb, "Filters: full-time, newest first\n")
	fmt.Fprintf(&sb, "Found %d jobs; showing %d\n\n", len(search.Jobs), len(jobs))

	for i, job := range jobs {
		fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, job.ID, job.Title)
		fmt.Fprintf(&sb, "URL: https://www.google.com/about/careers/applications/jobs/results/%s\n", job.Path)
		if job.Company != "" {
			fmt.Fprintf(&sb, "Company: %s\n", job.Company)
		}
		if job.Location != "" {
			fmt.Fprintf(&sb, "Location: %s\n", job.Location)
		}
		if detail := details[job.ID]; detail != nil {
			writeGoogleDetail(&sb, detail)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func writeGoogleDetail(sb *strings.Builder, detail *google.JobDetailResponse) {
	if detail.About != "" {
		fmt.Fprintf(sb, "About: %s\n", detail.About)
	}
	if detail.Qualifications != "" {
		fmt.Fprintf(sb, "Qualifications: %s\n", detail.Qualifications)
	}
	if detail.Responsibilities != "" {
		fmt.Fprintf(sb, "Responsibilities: %s\n", detail.Responsibilities)
	}
}
