package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/amikai/job-mcp/internal/provider/synopsys"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	keyword := strings.TrimSpace(scanner.Text())
	if keyword == "" {
		fmt.Fprintln(os.Stderr, "keyword is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := synopsys.NewClient(http.DefaultClient)

	results, err := c.Jobs(ctx, &synopsys.JobsRequest{
		Keywords:       keyword,
		RecordsPerPage: 15,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "Synopsys Jobs — keyword: %q\nFound %d jobs (page %d/%d); showing %d\n\n",
		keyword, results.TotalResults, results.CurrentPage, results.TotalPages, len(results.Jobs))

	for i, job := range results.Jobs {
		detail, err := c.JobDetail(ctx, job.City, job.Slug, job.JobID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.JobID, err)
			os.Exit(1)
		}
		fmt.Printf("%d. [%s] %s\n", i+1, job.DisplayID, job.Title)
		fmt.Printf("   Location: %s\n", job.Location)
		fmt.Printf("   Posted: %s\n", job.Posted)
		fmt.Printf("   URL: %s\n", job.URL())
		fmt.Printf("   Description:\n%s\n\n", indent(detail.Description, "   "))
	}
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
