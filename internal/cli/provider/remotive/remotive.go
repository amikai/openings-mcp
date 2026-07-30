// Package remotive provides CLI command construction for Remotive.
package remotive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/remotive"
)

type rootOptions struct {
	baseURL string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for remotive.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "remotive",
		Short:        "remotive [FLAGS] <search|detail|categories> [FLAGS]",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", remotive.DefaultBaseURL, "Remotive API base URL")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	var (
		keyword  string
		category string
		company  string
		jobType  string
		location string
		limit    int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "fetch the dump and filter it client-side (upstream query params are no-ops)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if limit < 1 {
				return fmt.Errorf("--limit must be >= 1, got %d", limit)
			}
			return runSearch(cmd.Context(), searchFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				opts: remotive.FilterOptions{
					Keyword:  keyword,
					Category: category,
					Company:  company,
					JobType:  jobType,
					Location: location,
				},
				limit: limit,
			})
		},
	}
	searchCmd.Flags().StringVar(&keyword, "keyword", "", "case-insensitive substring over title, tags, and description")
	searchCmd.Flags().StringVar(&category, "category", "", `category name or slug, e.g. "software-development" (see 'remotive categories')`)
	searchCmd.Flags().StringVar(&company, "company", "", "case-insensitive company name substring")
	searchCmd.Flags().StringVar(&jobType, "job-type", "", "exact job_type, e.g. full_time, contract, part_time, freelance")
	searchCmd.Flags().StringVar(&location, "location", "", `case-insensitive substring of candidate_required_location, e.g. "usa"`)
	searchCmd.Flags().IntVar(&limit, "limit", 20, "max results to print (filtering is client-side; the API has no paging)")

	var jobID int
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one job in full (resolved from the dump; there is no detail endpoint)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if jobID == 0 {
				return errors.New("--id is required (take it from a search result's ID)")
			}
			return runDetail(cmd.Context(), detailFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				jobID:   jobID,
			})
		},
	}
	detailCmd.Flags().IntVar(&jobID, "id", 0, "Remotive job id from a search result")

	categoriesCmd := &cobra.Command{
		Use:          "categories",
		Short:        "list job categories (name and slug)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runCategories(cmd.Context(), opts.baseURL, opts.timeout, opts.format)
		},
	}

	cmd.AddCommand(searchCmd, detailCmd, categoriesCmd)
	return cmd
}

func fetchJobs(ctx context.Context, baseURL string, timeout time.Duration) (*remotive.JobList, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := remotive.NewClient(baseURL)
	if err != nil {
		return nil, err
	}
	return client.ListJobs(ctx)
}

type jobSummaryJSON struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Company  string `json:"company"`
	Category string `json:"category"`
	JobType  string `json:"job_type"`
	Location string `json:"location,omitempty"`
	Salary   string `json:"salary,omitempty"`
	PostedAt string `json:"postedAt,omitempty"`
	URL      string `json:"url"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j remotive.Job) jobSummaryJSON {
	return jobSummaryJSON{
		ID:       j.ID,
		Title:    j.Title,
		Company:  j.CompanyName,
		Category: j.Category,
		JobType:  j.JobType,
		Location: j.CandidateRequiredLocation,
		Salary:   j.Salary,
		PostedAt: j.PublicationDate,
		URL:      j.URL,
	}
}

type searchFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	opts    remotive.FilterOptions
	limit   int
}

func runSearch(ctx context.Context, f searchFlags) error {
	res, err := fetchJobs(ctx, f.baseURL, f.timeout)
	if err != nil {
		return err
	}

	matched := remotive.FilterJobs(res.Jobs, f.opts)
	shown := matched
	if len(shown) > f.limit {
		shown = shown[:f.limit]
	}

	jobs := make([]jobSummaryJSON, len(shown))
	for i, j := range shown {
		jobs[i] = summarize(j)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(matched), Jobs: jobs})
	}

	fmt.Printf("Remotive Jobs Report (dump size: %d)\n", res.JobCount)
	fmt.Printf("Matched %d jobs; showing %d\n\n", len(matched), len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		fmt.Printf("Company: %s\n", s.Company)
		fmt.Printf("Category: %s (%s)\n", s.Category, s.JobType)
		if s.Location != "" {
			fmt.Printf("Location: %s\n", s.Location)
		}
		if s.Salary != "" {
			fmt.Printf("Salary: %s\n", s.Salary)
		}
		if s.PostedAt != "" {
			fmt.Printf("Posted: %s\n", s.PostedAt)
		}
		fmt.Printf("ID: %d\n", s.ID)
		fmt.Println()
	}
	return nil
}

type detailFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	jobID   int
}

func runDetail(ctx context.Context, f detailFlags) error {
	res, err := fetchJobs(ctx, f.baseURL, f.timeout)
	if err != nil {
		return err
	}

	for _, j := range res.Jobs {
		if j.ID == f.jobID {
			return printDetail(j, f.format)
		}
	}
	return fmt.Errorf("job %d not found in the current dump (%d jobs); it may have expired", f.jobID, res.JobCount)
}

func printDetail(j remotive.Job, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(j)
	}

	fmt.Println(j.Title)
	fmt.Printf("Company: %s\n", j.CompanyName)
	fmt.Printf("Category: %s (%s)\n", j.Category, j.JobType)
	if j.CandidateRequiredLocation != "" {
		fmt.Printf("Location: %s\n", j.CandidateRequiredLocation)
	}
	if j.Salary != "" {
		fmt.Printf("Salary: %s\n", j.Salary)
	}
	fmt.Printf("Posted: %s\n", j.PublicationDate)
	if len(j.Tags) > 0 {
		fmt.Printf("Tags: %v\n", j.Tags)
	}
	fmt.Printf("URL: %s\n", j.URL)

	rendered, err := html2text.FromString(j.Description, html2text.Options{})
	if err != nil {
		rendered = j.Description
	}
	fmt.Printf("\nDescription:\n%s\n", rendered)
	return nil
}

func runCategories(ctx context.Context, baseURL string, timeout time.Duration, format string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := remotive.NewClient(baseURL)
	if err != nil {
		return err
	}
	res, err := client.ListCategories(ctx)
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res.Jobs)
	}

	for _, c := range res.Jobs {
		fmt.Printf("%s (%s)\n", c.Name, c.Slug)
	}
	return nil
}
