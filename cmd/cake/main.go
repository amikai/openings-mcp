package main

import (
	"bufio"
	"context"
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
	"time"

	cake "github.com/amikai/job-mcp/internal/cake"
)

var tagRE = regexp.MustCompile(`<[^>]+>`)

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

	client, err := cake.NewClient("https://api.cake.me")
	if err != nil {
		return err
	}
	req := defaultSearchRequest(keyword)
	searchRes, err := client.SearchJobs(ctx, &req)
	if err != nil {
		return err
	}
	search, ok := searchRes.(*cake.JobSearchResponse)
	if !ok {
		return fmt.Errorf("search returned %T", searchRes)
	}

	jobs := jobsForDetail(search.Data)
	details := make(map[string]*cake.JobDetail, len(jobs))
	for _, job := range jobs {
		detailRes, err := client.GetJobDetail(ctx, cake.GetJobDetailParams{Path: job.Path})
		if err != nil {
			return fmt.Errorf("job detail %s: %w", job.Path, err)
		}
		detail, ok := detailRes.(*cake.JobDetail)
		if !ok {
			return fmt.Errorf("job detail %s returned %T", job.Path, detailRes)
		}
		details[job.Path] = detail
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

func defaultSearchRequest(keyword string) cake.JobSearchRequest {
	return cake.JobSearchRequest{
		Query:  keyword,
		SortBy: cake.JobSearchRequestSortByPopularity,
		Filters: cake.JobSearchRequestFilters{
			"job_types": []byte(`["full_time"]`),
			"remote":    []byte(`["no_remote_work"]`),
		},
	}
}

func jobsForDetail(jobs []cake.JobSearchItem) []cake.JobSearchItem {
	if len(jobs) > 10 {
		return jobs[:10]
	}
	return jobs
}

func formatReport(keyword string, search *cake.JobSearchResponse, jobs []cake.JobSearchItem, details map[string]*cake.JobDetail) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Cake Jobs Report\n")
	fmt.Fprintf(&sb, "Keyword: %s\n", keyword)
	fmt.Fprintf(&sb, "Filters: full-time, non-remote\n")
	fmt.Fprintf(&sb, "Found %d jobs (page %d/%d); showing %d\n\n", search.TotalEntries, search.CurrentPage, search.TotalPages, len(jobs))

	for i, job := range jobs {
		fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, job.Path, job.Title)
		if detail := details[job.Path]; detail != nil {
			writeCakeDetail(&sb, detail)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func writeCakeDetail(sb *strings.Builder, detail *cake.JobDetail) {
	fmt.Fprintf(sb, "URL: https://www.cake.me/companies/%s/jobs/%s\n", detail.PagePath, detail.Path)
	description := plainText(detail.Description)
	if description != "" {
		fmt.Fprintf(sb, "Description:\n%s\n", description)
	}
	requirements := plainText(detail.Requirements)
	if requirements != "" {
		fmt.Fprintf(sb, "Requirements: %s\n", requirements)
	}
}

func plainText(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Fields(s)
	return strings.Join(lines, " ")
}
