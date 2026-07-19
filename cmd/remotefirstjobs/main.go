// Command remotefirstjobs is a debug CLI for RemoteFirstJobs' public
// jobs API.
//
//	go run ./cmd/remotefirstjobs search --query golang --page 1
//	go run ./cmd/remotefirstjobs search --category software-development
//	go run ./cmd/remotefirstjobs detail --id senior-product-manager-ai-platform-837752
//
// Search runs server-side (full-text query, category, page 0-4 of 100
// jobs each). There is no per-job detail endpoint, so detail scans
// search pages for the id; pass the same --query/--category the id was
// found under to shorten the scan (see the quirk notes in
// internal/provider/remotefirstjobs/openapi.yaml).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/remotefirstjobs"
)

// apiBaseURL is the single production server in the provider's
// openapi.yaml (paths carry the /search-jobs suffix).
const apiBaseURL = "https://remotefirstjobs.com/api"

func main() {
	rootFlags := ff.NewFlagSet("remotefirstjobs")
	var (
		baseURL = rootFlags.StringLong("base-url", apiBaseURL, "RemoteFirstJobs API base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "remotefirstjobs",
		Usage: "remotefirstjobs [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		query    = searchFS.StringLong("query", "", `full-text search term, e.g. "python" or "react engineer"`)
		category = searchFS.StringLong("category", "", `category filter, e.g. "software-development" or "design" (see openapi.yaml for the full list)`)
		page     = searchFS.IntLong("page", 0, "page number, 0-4 (100 jobs per page)")
		limit    = searchFS.IntLong("limit", 20, "max results to print out of the fetched page")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "remotefirstjobs search [--query TEXT] [--category NAME] [--page 0-4] [--limit N] [--format text|json]",
		ShortHelp: "search jobs server-side (newest first, 24h publication delay)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			if *page < 0 || *page > 4 {
				return fmt.Errorf("--page must be between 0 and 4, got %d", *page)
			}
			if *limit < 1 {
				return fmt.Errorf("--limit must be >= 1, got %d", *limit)
			}
			return runSearch(ctx, searchFlags{
				baseURL:  *baseURL,
				timeout:  *timeout,
				format:   *format,
				query:    *query,
				category: *category,
				page:     *page,
				limit:    *limit,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	var (
		jobID          = detailFS.StringLong("id", "", "job id (slug) from a search result")
		detailQuery    = detailFS.StringLong("query", "", "the search term the id was found under (narrows the page scan)")
		detailCategory = detailFS.StringLong("category", "", "the job's category (narrows the page scan)")
	)
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "remotefirstjobs detail --id JOB-ID [--query TEXT] [--category NAME] [--format text|json]",
		ShortHelp: "print one job in full (resolved by scanning search pages; there is no detail endpoint)",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			if *jobID == "" {
				return errors.New("--id is required (take it from a search result's ID)")
			}
			return runDetail(ctx, detailFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				jobID:   *jobID,
				opts: remotefirstjobs.FindOptions{
					Query:    *detailQuery,
					Category: *detailCategory,
				},
			})
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
