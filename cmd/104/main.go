package main

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	job104 "github.com/amikai/job-mcp/internal/provider/104"
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

	client := job104.NewClient(job104.Config{})
	search, err := client.Jobs(ctx, defaultSearchParams(keyword))
	if err != nil {
		return err
	}

	jobs := jobsForDetail(search.Data)
	details := make(map[string]*job104.JobDetailResponse, len(jobs))
	for _, job := range jobs {
		code := jobCodeFromURL(job.Link.Job)
		if code == "" {
			continue
		}
		detail, err := client.JobDetail(ctx, code)
		if err != nil {
			return fmt.Errorf("job detail %s: %w", code, err)
		}
		details[code] = detail
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

func defaultSearchParams(keyword string) *job104.JobRequest {
	fullTime := 0
	return &job104.JobRequest{
		Keyword: keyword,
		RO:      &fullTime,
	}
}

func jobCodeFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil {
		raw = u.Path
	}
	raw = strings.TrimRight(raw, "/")
	parts := strings.Split(raw, "/")
	return parts[len(parts)-1]
}

func jobsForDetail(jobs []job104.Job) []job104.Job {
	limited := make([]job104.Job, 0, min(len(jobs), 10))
	for _, job := range jobs {
		if job.RemoteWorkType != 0 {
			continue
		}
		limited = append(limited, job)
		if len(limited) == 10 {
			break
		}
	}
	return limited
}

func formatReport(keyword string, search *job104.SearchJobResponse, jobs []job104.Job, details map[string]*job104.JobDetailResponse) string {
	var sb strings.Builder
	p := search.Metadata.Pagination
	fmt.Fprintf(&sb, "104 Jobs Report\n")
	fmt.Fprintf(&sb, "Keyword: %s\n", keyword)
	fmt.Fprintf(&sb, "Filters: full-time, non-remote\n")
	fmt.Fprintf(&sb, "Found %d jobs (page %d/%d); showing %d\n\n", p.Total, p.CurrentPage, p.LastPage, len(jobs))

	for i, job := range jobs {
		code := jobCodeFromURL(job.Link.Job)
		fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, code, job.JobName)
		fmt.Fprintf(&sb, "Company: %s\n", job.CustName)
		if job.JobAddrNoDesc != "" {
			fmt.Fprintf(&sb, "Location: %s\n", job.JobAddrNoDesc)
		}
		if detail := details[code]; detail != nil {
			write104Detail(&sb, detail)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func write104Detail(sb *strings.Builder, detail *job104.JobDetailResponse) {
	d := detail.Data
	jd := d.JobDetail
	if jd.Salary != "" {
		fmt.Fprintf(sb, "Salary: %s\n", jd.Salary)
	}
	if d.Condition.WorkExp != "" || d.Condition.Edu != "" {
		fmt.Fprintf(sb, "Experience: %s | Education: %s\n", d.Condition.WorkExp, d.Condition.Edu)
	}
	if jd.JobDescription != "" {
		fmt.Fprintf(sb, "Description:\n%s\n", strings.TrimSpace(jd.JobDescription))
	}
}
