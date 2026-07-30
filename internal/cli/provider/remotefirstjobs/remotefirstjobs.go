// Package remotefirstjobs provides CLI command construction for RemoteFirstJobs.
package remotefirstjobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	"github.com/amikai/openings-mcp/internal/provider/remotefirstjobs"
)

type rootOptions struct {
	baseURL string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for remotefirstjobs.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "remotefirstjobs",
		Short:        "remotefirstjobs [FLAGS] <search|detail> [FLAGS]",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", remotefirstjobs.DefaultBaseURL, "RemoteFirstJobs API base URL")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	clihelp.FormatVar(cmd.PersistentFlags(), &opts.format)

	var (
		query    string
		category string
		page     int
		limit    int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search jobs server-side (newest first, 24h publication delay)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if page < 0 || page > 4 {
				return fmt.Errorf("--page must be between 0 and 4, got %d", page)
			}
			if limit < 1 {
				return fmt.Errorf("--limit must be >= 1, got %d", limit)
			}
			return runSearch(cmd.Context(), searchFlags{
				baseURL:  opts.baseURL,
				timeout:  opts.timeout,
				format:   opts.format,
				query:    query,
				category: category,
				page:     page,
				limit:    limit,
			})
		},
	}
	searchCmd.Flags().StringVar(&query, "query", "", `full-text search term, e.g. "python" or "react engineer"`)
	searchCmd.Flags().StringVar(&category, "category", "", `category filter, e.g. "software-development" or "design" (see openapi.yaml for the full list)`)
	searchCmd.Flags().IntVar(&page, "page", 0, "page number, 0-4 (100 jobs per page)")
	searchCmd.Flags().IntVar(&limit, "limit", 20, "max results to print out of the fetched page")

	var (
		jobID          string
		detailQuery    string
		detailCategory string
	)
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one job in full (resolved by scanning search pages; there is no detail endpoint)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if jobID == "" {
				return errors.New("--id is required (take it from a search result's ID)")
			}
			return runDetail(cmd.Context(), detailFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				jobID:   jobID,
				opts: remotefirstjobs.FindOptions{
					Query:    detailQuery,
					Category: detailCategory,
				},
			})
		},
	}
	detailCmd.Flags().StringVar(&jobID, "id", "", "job id (slug) from a search result")
	detailCmd.Flags().StringVar(&detailQuery, "query", "", "the search term the id was found under (narrows the page scan)")
	detailCmd.Flags().StringVar(&detailCategory, "category", "", "the job's category (narrows the page scan)")

	cmd.AddCommand(searchCmd, detailCmd)
	return cmd
}

// jobSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs, no HTML description.
type jobSummaryJSON struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Company   string   `json:"company"`
	Category  string   `json:"category"`
	Seniority string   `json:"seniority"`
	Locations []string `json:"locations,omitempty"`
	SalaryMin int      `json:"salaryMin,omitempty"`
	SalaryMax int      `json:"salaryMax,omitempty"`
	PostedAt  string   `json:"postedAt"`
	URL       string   `json:"url"`
}

type searchResultJSON struct {
	Page  int              `json:"page"`
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j remotefirstjobs.Job) jobSummaryJSON {
	return jobSummaryJSON{
		ID:        j.ID,
		Title:     j.Title,
		Company:   j.CompanyName,
		Category:  j.Category,
		Seniority: j.Seniority,
		Locations: j.Locations,
		SalaryMin: j.SalaryMin.Or(0),
		SalaryMax: j.SalaryMax.Or(0),
		PostedAt:  j.PublishedAt,
		URL:       j.URL,
	}
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	baseURL  string
	timeout  time.Duration
	format   string
	query    string
	category string
	page     int
	limit    int
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := remotefirstjobs.NewClient(f.baseURL)
	if err != nil {
		return err
	}

	params := remotefirstjobs.SearchJobsParams{
		Page: remotefirstjobs.NewOptInt(f.page),
	}
	if f.query != "" {
		params.Query = remotefirstjobs.NewOptString(f.query)
	}
	if f.category != "" {
		params.Category = remotefirstjobs.NewOptString(f.category)
	}

	res, err := client.SearchJobs(ctx, params)
	if err != nil {
		return err
	}
	result, ok := res.(*remotefirstjobs.SearchJobsResult)
	if !ok {
		return res.(*remotefirstjobs.Error)
	}

	shown := result.Jobs
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
		return enc.Encode(searchResultJSON{Page: result.Page, Total: result.JobsCount, Jobs: jobs})
	}

	fmt.Printf("RemoteFirstJobs Report (page %d: %d jobs; showing %d)\n\n", result.Page, result.JobsCount, len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		fmt.Printf("Company: %s\n", s.Company)
		fmt.Printf("Category: %s (%s)\n", s.Category, s.Seniority)
		if len(s.Locations) > 0 {
			fmt.Printf("Locations: %v\n", s.Locations)
		}
		if s.SalaryMin > 0 || s.SalaryMax > 0 {
			fmt.Printf("Salary: %d - %d\n", s.SalaryMin, s.SalaryMax)
		}
		fmt.Printf("Posted: %s\n", s.PostedAt)
		fmt.Printf("ID: %s\n", s.ID)
		fmt.Println()
	}
	return nil
}

// detailFlags carries the parsed "detail" subcommand flags into runDetail.
type detailFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	jobID   string
	opts    remotefirstjobs.FindOptions
}

func runDetail(ctx context.Context, f detailFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := remotefirstjobs.NewClient(f.baseURL)
	if err != nil {
		return err
	}

	job, err := client.FindJob(ctx, f.jobID, f.opts)
	if err != nil {
		return err
	}
	return printDetail(*job, f.format)
}

// printDetail renders one full job. JSON mode encodes the generated Job
// as-is — detail is for seeing the whole record.
func printDetail(j remotefirstjobs.Job, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(j)
	}

	fmt.Println(j.Title)
	fmt.Printf("Company: %s\n", j.CompanyName)
	fmt.Printf("Category: %s (%s)\n", j.Category, j.Seniority)
	if len(j.Locations) > 0 {
		fmt.Printf("Locations: %v\n", j.Locations)
	}
	if min, max := j.SalaryMin.Or(0), j.SalaryMax.Or(0); min > 0 || max > 0 {
		fmt.Printf("Salary: %d - %d\n", min, max)
	}
	fmt.Printf("Posted: %s\n", j.PublishedAt)
	fmt.Printf("URL: %s\n", j.URL)

	rendered, err := html2text.FromString(j.Description, html2text.Options{})
	if err != nil {
		rendered = j.Description
	}
	fmt.Printf("\nDescription:\n%s\n", rendered)
	return nil
}
