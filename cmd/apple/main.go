// Command apple provides a small diagnostic CLI for the Apple Jobs API.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/apple"
)

const defaultBaseURL = "https://jobs.apple.com"

func main() {
	rootFlags := ff.NewFlagSet("apple")
	var (
		baseURL = rootFlags.StringLong("base-url", defaultBaseURL, "Apple Jobs API base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "apple",
		Usage: "apple [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword = searchFS.StringLong("keyword", "", "keyword query (required)")
		country = searchFS.StringLong("country", "", "ISO 3166-1 alpha-3 country code, e.g. TWN or USA (required)")
		sort    = searchFS.StringEnumLong("sort", "result order", "relevance", "newest")
		page    = searchFS.IntLong("page", 1, "1-based page of 20 results")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "apple search --keyword TEXT --country ISO3 [--sort relevance|newest] [--page N]",
		ShortHelp: "search jobs.apple.com listings",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			return runSearch(ctx, searchFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				keyword: *keyword,
				country: *country,
				sort:    *sort,
				page:    *page,
			}, os.Stdout)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("job-id", "", "numeric position ID returned by search (required)")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "apple detail --job-id ID",
		ShortHelp: "fetch one Apple job posting",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v", args)
			}
			return runDetail(ctx, detailFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				jobID:   *jobID,
			}, os.Stdout)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

	if err := rootCmd.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd.GetSelected()))
		if errors.Is(err, ff.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
	if rootCmd.GetSelected() == rootCmd {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd))
		fmt.Fprintln(os.Stderr, "err: a subcommand (search or detail) is required")
		os.Exit(1)
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

type searchFlags struct {
	baseURL string
	format  string
	keyword string
	country string
	sort    string
	timeout time.Duration
	page    int
}

func runSearch(ctx context.Context, flags searchFlags, out io.Writer) error {
	if strings.TrimSpace(flags.keyword) == "" {
		return errors.New("--keyword is required")
	}
	if strings.TrimSpace(flags.country) == "" {
		return errors.New("--country is required")
	}
	if flags.page < 1 {
		return fmt.Errorf("--page must be >= 1, got %d", flags.page)
	}

	client, err := apple.NewJobsClient(flags.baseURL, nil)
	if err != nil {
		return fmt.Errorf("create apple client: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	response, err := client.SearchJobs(ctx, apple.SearchRequest{
		Keyword:     flags.keyword,
		CountryCode: flags.country,
		Sort:        apple.Sort(flags.sort),
		Page:        flags.page,
	})
	if err != nil {
		return fmt.Errorf("search apple jobs: %w", err)
	}
	return writeSearch(out, flags.format, flags.page, response)
}

func writeSearch(out io.Writer, format string, page int, response *apple.SearchResponse) error {
	if format == "json" {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("encode search response: %w", err)
		}
		return nil
	}

	result := response.Res
	fmt.Fprintf(out, "total=%d page=%d jobs=%d\n\n", result.TotalRecords, page, len(result.SearchResults))
	for index, job := range result.SearchResults {
		fmt.Fprintf(out, "%d. [%s] %s\n", index+1, job.PositionId, job.PostingTitle)
		if job.Team.TeamName != "" {
			fmt.Fprintf(out, "   team: %s\n", job.Team.TeamName)
		}
		for _, location := range job.Locations {
			fmt.Fprintf(out, "   location: %s\n", locationLabel(location.Name, location.CountryName))
		}
		if job.PostingDate != "" {
			fmt.Fprintf(out, "   posted: %s\n", job.PostingDate)
		}
		fmt.Fprintf(out, "   url: %s\n\n", apple.JobURL(job.PositionId, job.TransformedPostingTitle))
	}
	return nil
}

type detailFlags struct {
	baseURL string
	format  string
	jobID   string
	timeout time.Duration
}

func runDetail(ctx context.Context, flags detailFlags, out io.Writer) error {
	if strings.TrimSpace(flags.jobID) == "" {
		return errors.New("--job-id is required")
	}

	client, err := apple.NewJobsClient(flags.baseURL, nil)
	if err != nil {
		return fmt.Errorf("create apple client: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	response, err := client.JobDetail(ctx, flags.jobID)
	if err != nil {
		return fmt.Errorf("get apple job detail: %w", err)
	}
	return writeDetail(out, flags.format, response)
}

func writeDetail(out io.Writer, format string, response *apple.JobDetailResponse) error {
	if format == "json" {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("encode detail response: %w", err)
		}
		return nil
	}

	job := response.Res
	fmt.Fprintf(out, "[%s] %s\n", job.PositionId, job.PostingTitle)
	for _, team := range job.TeamNames {
		fmt.Fprintf(out, "team: %s\n", team)
	}
	for _, location := range job.Locations {
		fmt.Fprintf(out, "location: %s\n", locationLabel(location.Name, location.CountryName))
	}
	if employmentType := job.EmploymentType.Or(""); employmentType != "" {
		fmt.Fprintf(out, "employment_type: %s\n", employmentType)
	}
	if job.PostingDate != "" {
		fmt.Fprintf(out, "posted: %s\n", job.PostingDate)
	}
	fmt.Fprintf(out, "url: %s\n", apple.JobURL(job.PositionId, job.TransformedPostingTitle))

	writeSection(out, "Summary", job.JobSummary.Or(""))
	writeSection(out, "Description", job.Description.Or(""))
	writeSection(out, "Responsibilities", job.Responsibilities.Or(""))
	writeSection(out, "Minimum qualifications", job.MinimumQualifications.Or(""))
	writeSection(out, "Preferred qualifications", job.PreferredQualifications.Or(""))
	return nil
}

func writeSection(out io.Writer, heading, body string) {
	if strings.TrimSpace(body) != "" {
		fmt.Fprintf(out, "\n%s\n%s\n", heading, body)
	}
}

func locationLabel(name, country string) string {
	if name == "" {
		return country
	}
	if country == "" || strings.EqualFold(name, country) {
		return name
	}
	return name + ", " + country
}
