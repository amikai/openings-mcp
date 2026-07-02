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

	"github.com/amikai/job-mcp/internal/provider/tsmc"
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

	client := tsmc.NewClient(http.DefaultClient)
	search, err := client.Jobs(ctx, &tsmc.JobsRequest{
		Keyword:         keyword,
		Locations:       []string{tsmc.LocTaiwan},
		EmploymentTypes: []string{tsmc.EmployRegular},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	jobs := search.Jobs
	if len(jobs) > 10 {
		jobs = jobs[:10]
	}
	details := make(map[string]*tsmc.JobDetailResponse, len(jobs))
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

func writeReport(w io.Writer, keyword string, search *tsmc.JobsResponse, jobs []tsmc.Job, details map[string]*tsmc.JobDetailResponse) {
	fmt.Fprintf(w, "TSMC Jobs Report\n")
	fmt.Fprintf(w, "Keyword: %s\n", keyword)
	fmt.Fprintf(w, "Filters: Taiwan, regular\n")
	fmt.Fprintf(w, "Found %d jobs; showing %d\n\n", search.Total, len(jobs))

	for i, job := range jobs {
		fmt.Fprintf(w, "%d. [%s] %s\n", i+1, job.ID, job.Title)
		fmt.Fprintf(w, "URL: https://careers.tsmc.com/zh_TW/careers/JobDetail/%s/%s\n", job.Slug, job.ID)
		if job.Location != "" {
			fmt.Fprintf(w, "Location: %s\n", job.Location)
		}
		if job.CareerArea != "" {
			fmt.Fprintf(w, "Career Area: %s\n", job.CareerArea)
		}
		if job.Posted != "" {
			fmt.Fprintf(w, "Posted: %s\n", job.Posted)
		}
		if detail := details[job.ID]; detail != nil {
			if detail.Responsibilities != "" {
				fmt.Fprintf(w, "Responsibilities: %s\n", detail.Responsibilities)
			}
			if detail.Qualifications != "" {
				fmt.Fprintf(w, "Qualifications: %s\n", detail.Qualifications)
			}
		}
		fmt.Fprintln(w)
	}
}
