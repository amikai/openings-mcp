package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/amikai/job-mcp/internal/provider/synopsys"
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

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := synopsys.NewClient(synopsys.Config{})

	results, err := c.Jobs(ctx, &synopsys.JobRequest{
		Keywords:       keyword,
		RecordsPerPage: 15,
	})
	if err != nil {
		return err
	}

	p := results.TotalResults
	fmt.Fprintf(os.Stdout, "Synopsys Jobs — keyword: %q\nFound %d jobs (page %d/%d); showing %d\n\n",
		keyword, p, results.CurrentPage, results.TotalPages, len(results.Jobs))

	for i, job := range results.Jobs {
		detail, err := c.JobDetail(ctx, job.City, job.Slug, job.JobID)
		if err != nil {
			return fmt.Errorf("job detail %s: %w", job.JobID, err)
		}
		fmt.Printf("%d. [%s] %s\n", i+1, job.DisplayID, job.Title)
		fmt.Printf("   Location: %s\n", job.Location)
		fmt.Printf("   Posted: %s\n", job.Posted)
		fmt.Printf("   URL: %s\n", job.URL())
		fmt.Printf("   Description:\n%s\n\n", indent(detail.Description, "   "))
	}
	return nil
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
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
