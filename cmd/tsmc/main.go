package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/amikai/job-mcp/internal/provider/tsmc"
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

	client := tsmc.NewClient(tsmc.Config{})
	search, err := client.Jobs(ctx, &tsmc.JobRequest{
		Keyword:         keyword,
		Locations:       []string{tsmc.LocTaiwan},
		EmploymentTypes: []string{tsmc.EmployRegular},
	})
	if err != nil {
		return err
	}

	jobs := search.Jobs
	if len(jobs) > 10 {
		jobs = jobs[:10]
	}
	details := make(map[string]*tsmc.JobDetail, len(jobs))
	for _, job := range jobs {
		detail, err := client.JobDetail(ctx, job.ID)
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

func formatReport(keyword string, search *tsmc.SearchResponse, jobs []tsmc.Job, details map[string]*tsmc.JobDetail) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "TSMC Jobs Report\n")
	fmt.Fprintf(&sb, "Keyword: %s\n", keyword)
	fmt.Fprintf(&sb, "Filters: Taiwan, regular\n")
	fmt.Fprintf(&sb, "Found %d jobs; showing %d\n\n", search.Total, len(jobs))

	for i, job := range jobs {
		fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, job.ID, job.Title)
		fmt.Fprintf(&sb, "URL: https://careers.tsmc.com/zh_TW/careers/JobDetail/%s/%s\n", job.Slug, job.ID)
		if job.Location != "" {
			fmt.Fprintf(&sb, "Location: %s\n", job.Location)
		}
		if job.CareerArea != "" {
			fmt.Fprintf(&sb, "Career Area: %s\n", job.CareerArea)
		}
		if job.Posted != "" {
			fmt.Fprintf(&sb, "Posted: %s\n", job.Posted)
		}
		if detail := details[job.ID]; detail != nil {
			if detail.Responsibilities != "" {
				fmt.Fprintf(&sb, "Responsibilities: %s\n", detail.Responsibilities)
			}
			if detail.Qualifications != "" {
				fmt.Fprintf(&sb, "Qualifications: %s\n", detail.Qualifications)
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
