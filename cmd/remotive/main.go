// Command remotive is a debug CLI for Remotive's public remote-jobs API.
//
//	go run ./cmd/remotive search --keyword golang --category software-development
//	go run ./cmd/remotive detail --id 2091069
//	go run ./cmd/remotive categories
//
// Every invocation fetches the full dump once and filters it client-side —
// the API's documented query params are no-ops (see the deviation notes in
// internal/provider/remotive/openapi.yaml). Upstream blocks >2 requests
// per minute and asks for at most ~4 fetches a day, so keep invocations
// sparse.
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

	"github.com/amikai/openings-mcp/internal/provider/remotive"
)

// apiBaseURL is the single production server in the provider's
// openapi.yaml (paths carry the /remote-jobs prefix).
const _apiBaseURL = "https://remotive.com/api"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("remotive")
	var (
		baseURL = rootFlags.StringLong("base-url", _apiBaseURL, "Remotive API base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "remotive",
		Usage: "remotive [FLAGS] <search|detail|categories> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword  = searchFS.StringLong("keyword", "", "case-insensitive substring over title, tags, and description")
		category = searchFS.StringLong("category", "", `category name or slug, e.g. "software-development" (see 'remotive categories')`)
		company  = searchFS.StringLong("company", "", "case-insensitive company name substring")
		jobType  = searchFS.StringLong("job-type", "", "exact job_type, e.g. full_time, contract, part_time, freelance")
		location = searchFS.StringLong("location", "", `case-insensitive substring of candidate_required_location, e.g. "usa"`)
		limit    = searchFS.IntLong("limit", 20, "max results to print (filtering is client-side; the API has no paging)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "remotive search [--keyword TEXT] [--category NAME|SLUG] [--company TEXT] [--job-type TYPE] [--location TEXT] [--limit N] [--format text|json]",
		ShortHelp: "fetch the dump and filter it client-side (upstream query params are no-ops)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			if *limit < 1 {
				return fmt.Errorf("--limit must be >= 1, got %d", *limit)
			}
			return runSearch(ctx, searchFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				opts: remotive.FilterOptions{
					Keyword:  *keyword,
					Category: *category,
					Company:  *company,
					JobType:  *jobType,
					Location: *location,
				},
				limit: *limit,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.IntLong("id", 0, "Remotive job id from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "remotive detail --id JOB-ID [--format text|json]",
		ShortHelp: "print one job in full (resolved from the dump; there is no detail endpoint)",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			if *jobID == 0 {
				return errors.New("--id is required (take it from a search result's ID)")
			}
			return runDetail(ctx, detailFlags{baseURL: *baseURL, timeout: *timeout, format: *format, jobID: *jobID})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

	categoriesFS := ff.NewFlagSet("categories").SetParent(rootFlags)
	categoriesCmd := &ff.Command{
		Name:      "categories",
		Usage:     "remotive categories [--format text|json]",
		ShortHelp: "list job categories (name and slug)",
		Flags:     categoriesFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("categories takes no positional arguments, got %v", args)
			}
			return runCategories(ctx, *baseURL, *timeout, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, categoriesCmd)

	if err := rootCmd.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd.GetSelected()))
		if errors.Is(err, ff.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	if rootCmd.GetSelected() == rootCmd {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd))
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, detail, or categories) is required")
		return 1
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

// fetchJobs pulls the full dump — the only read the API supports.
func fetchJobs(ctx context.Context, baseURL string, timeout time.Duration) (*remotive.JobList, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := remotive.NewClient(baseURL)
	if err != nil {
		return nil, err
	}
	return client.ListJobs(ctx)
}

// jobSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs, no HTML description.
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

// searchFlags carries the parsed "search" subcommand flags into runSearch.
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

// detailFlags carries the parsed "detail" subcommand flags into runDetail.
type detailFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	jobID   int
}

// runDetail resolves one job from the dump by id — the API has no detail
// endpoint, so an id that has left the dump (or never existed) is simply
// not found.
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

// printDetail renders one full job. JSON mode encodes the generated Job
// as-is — detail is for seeing the whole record.
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
