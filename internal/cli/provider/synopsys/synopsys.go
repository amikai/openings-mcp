package synopsys

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/synopsys"
)

type options struct {
	keyword string
	timeout time.Duration
}

// NewCommand returns a cobra.Command for synopsys.
func NewCommand() *cobra.Command {
	opts := &options{}
	cmd := &cobra.Command{
		Use:          "synopsys",
		Short:        "Search Synopsys jobs",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), *opts)
		},
	}

	cmd.Flags().StringVar(&opts.keyword, "keyword", "", "free-text query (if empty, reads from stdin)")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 120*time.Second, "request timeout")

	return cmd
}

func run(ctx context.Context, opts options) error {
	keyword := opts.keyword
	if keyword == "" {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			keyword = strings.TrimSpace(scanner.Text())
		}
	}
	if keyword == "" {
		return errors.New("keyword is required")
	}

	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	c := synopsys.NewClient(http.DefaultClient)

	results, err := c.Jobs(ctx, &synopsys.JobsRequest{
		Keywords:       keyword,
		RecordsPerPage: 15,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Synopsys Jobs — keyword: %q\nFound %d jobs (page %d/%d); showing %d\n\n",
		keyword, results.TotalResults, results.CurrentPage, results.TotalPages, len(results.Jobs))

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
