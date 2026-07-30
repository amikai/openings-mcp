package quanta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	quantaprovider "github.com/amikai/openings-mcp/internal/provider/quanta"
)

const apiBaseURL = "https://hr.quantatw.com"

type options struct {
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for quanta.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "quanta",
		Short:        "Search Quanta Computer jobs and view position details",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	var (
		searchKeyword     string
		searchLocationIDs []string
		searchCategoryIDs []string
		searchLimit       int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "fetch the dump and filter it client-side (the site's query params are no-ops)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if searchLimit < 1 {
				return fmt.Errorf("--limit must be >= 1, got %d", searchLimit)
			}
			return runSearch(cmd.Context(), searchFlags{
				timeout: opts.timeout,
				format:  opts.format,
				opts: quantaprovider.FilterOptions{
					Keyword:     searchKeyword,
					LocationIDs: searchLocationIDs,
					CategoryIDs: searchCategoryIDs,
				},
				limit: searchLimit,
			})
		},
	}

	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "case-insensitive substring over category, title, location, requirements, and keywords")
	searchCmd.Flags().StringSliceVar(&searchLocationIDs, "location-id", nil, "locati value from a search result; repeatable, matches any")
	searchCmd.Flags().StringSliceVar(&searchCategoryIDs, "category-id", nil, "capoid value from a search result; repeatable, matches any")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "max results to print (filtering is client-side; the site has no paging)")

	var detailSerial string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one job in full (resolved from the dump; there is no detail endpoint)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if detailSerial == "" {
				return errors.New("--serial is required (take it from a search result's serial)")
			}
			return runDetail(cmd.Context(), detailFlags{timeout: opts.timeout, format: opts.format, serial: detailSerial})
		},
	}

	detailCmd.Flags().StringVar(&detailSerial, "serial", "", "job serial from a search result (the id used in ?serial= share links)")

	rootCmd.AddCommand(searchCmd, detailCmd)
	return rootCmd
}

func fetchJobs(ctx context.Context, timeout time.Duration) ([]quantaprovider.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := quantaprovider.NewClient(apiBaseURL)
	if err != nil {
		return nil, err
	}
	res, err := client.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	return res.GetJobResult(), nil
}

type jobSummaryJSON struct {
	Serial   string `json:"serial"`
	JobCode  string `json:"jobCode"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Location string `json:"location,omitempty"`
	Salary   string `json:"salary,omitempty"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j quantaprovider.Job) jobSummaryJSON {
	return jobSummaryJSON{
		Serial:   j.GetSerial(),
		JobCode:  j.GetJobCode(),
		Title:    j.GetTitle(),
		Category: j.GetCategoryName(),
		Location: j.GetLocationName(),
		Salary:   j.GetSalary(),
	}
}

type searchFlags struct {
	timeout time.Duration
	format  string
	opts    quantaprovider.FilterOptions
	limit   int
}

func runSearch(ctx context.Context, f searchFlags) error {
	jobs, err := fetchJobs(ctx, f.timeout)
	if err != nil {
		return err
	}

	matched := quantaprovider.FilterJobs(jobs, f.opts)
	shown := matched
	if len(shown) > f.limit {
		shown = shown[:f.limit]
	}

	summaries := make([]jobSummaryJSON, len(shown))
	for i, j := range shown {
		summaries[i] = summarize(j)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(matched), Jobs: summaries})
	}

	fmt.Printf("Quanta Jobs Report (dump size: %d)\n", len(jobs))
	fmt.Printf("Matched %d jobs; showing %d\n\n", len(matched), len(summaries))
	for i, s := range summaries {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		fmt.Printf("Category: %s\n", s.Category)
		if s.Location != "" {
			fmt.Printf("Location: %s\n", s.Location)
		}
		if s.Salary != "" {
			fmt.Printf("Salary: %s\n", s.Salary)
		}
		fmt.Printf("Job Code: %s\n", s.JobCode)
		fmt.Printf("Serial: %s\n", s.Serial)
		fmt.Println()
	}
	return nil
}

type detailFlags struct {
	timeout time.Duration
	format  string
	serial  string
}

func runDetail(ctx context.Context, f detailFlags) error {
	jobs, err := fetchJobs(ctx, f.timeout)
	if err != nil {
		return err
	}

	j, ok := quantaprovider.FindBySerial(jobs, f.serial)
	if !ok {
		return fmt.Errorf("serial %q not found in the current dump (%d jobs); it may have expired", f.serial, len(jobs))
	}
	return printDetail(j, f.format)
}

func printDetail(j quantaprovider.Job, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(j)
	}

	fmt.Println(j.GetTitle())
	fmt.Printf("Category: %s\n", j.GetCategoryName())
	if loc := j.GetLocationName(); loc != "" {
		fmt.Printf("Location: %s\n", loc)
	}
	if salary := j.GetSalary(); salary != "" {
		fmt.Printf("Salary: %s\n", salary)
	}
	fmt.Printf("Job Code: %s\n", j.GetJobCode())
	fmt.Printf("Serial: %s\n", j.GetSerial())
	if keywords := j.GetKeywords(); keywords != "" {
		fmt.Printf("Keywords: %s\n", keywords)
	}

	if req, ok := j.GetRequirements().Get(); ok && req != "" {
		fmt.Printf("\nRequirements:\n%s\n", req)
	}
	fmt.Printf("\nDescription:\n%s\n", j.GetDescription())
	return nil
}
