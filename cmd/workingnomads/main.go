// Command workingnomads is a debug CLI for Working Nomads's public
// exposed_jobs feed.
//
//	go run ./cmd/workingnomads search --keyword golang --category Development
//	go run ./cmd/workingnomads detail --id 1734670
//
// There is no server-side search or per-job endpoint: search always
// fetches and filters the full dump client-side, and detail resolves the
// id against a fresh full-dump fetch (see internal/provider/workingnomads's
// package doc for why).
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

	"github.com/amikai/openings-mcp/internal/provider/workingnomads"
)

const defaultBaseURL = "https://www.workingnomads.com"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("workingnomads")
	var (
		baseURL = rootFlags.StringLong("base-url", defaultBaseURL, "Working Nomads base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "workingnomads",
		Usage: "workingnomads [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword  = searchFS.StringLong("keyword", "", "case-insensitive substring over title, tags, and description")
		category = searchFS.StringLong("category", "", "case-insensitive substring of category (e.g. Development, Design); no fixed enum")
		company  = searchFS.StringLong("company", "", "case-insensitive company name substring")
		location = searchFS.StringLong("location", "", "case-insensitive substring over the free-text location field")
		limit    = searchFS.IntLong("limit", 20, "max results to print (filtering is client-side)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "workingnomads search [--keyword TEXT] [--category TEXT] [--company TEXT] [--location TEXT] [--limit N] [--format text|json]",
		ShortHelp: "fetch the full jobs dump and filter client-side",
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
				opts: workingnomads.FilterOptions{
					Keyword:  *keyword,
					Category: *category,
					Company:  *company,
					Location: *location,
				},
				limit: *limit,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("id", "", "job ID (numeric, from a search result)")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "workingnomads detail --id JOB-ID [--format text|json]",
		ShortHelp: "print one job in full (resolved from a fresh full-dump fetch)",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			if *jobID == "" {
				return errors.New("--id is required (take it from a search result's ID)")
			}
			return runDetail(ctx, detailFlags{baseURL: *baseURL, timeout: *timeout, format: *format, jobID: *jobID})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search or detail) is required")
		return 1
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

func newClient(baseURL string) *workingnomads.Client {
	return workingnomads.NewClient(baseURL, nil)
}

// jobSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs, no HTML description.
type jobSummaryJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Company  string `json:"company"`
	Category string `json:"category"`
	Location string `json:"location,omitempty"`
	PostedAt string `json:"postedAt,omitempty"`
	URL      string `json:"url"`
}

func summarize(j workingnomads.Job) jobSummaryJSON {
	var postedAt string
	if !j.PostedAt.IsZero() {
		postedAt = j.PostedAt.Format(time.RFC3339)
	}
	return jobSummaryJSON{
		ID:       j.ID,
		Title:    j.Title,
		Company:  j.Company,
		Category: j.Category,
		Location: j.Location,
		PostedAt: postedAt,
		URL:      j.URL,
	}
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	opts    workingnomads.FilterOptions
	limit   int
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	matched, err := newClient(f.baseURL).Search(ctx, f.opts)
	if err != nil {
		return err
	}

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

	fmt.Printf("Matched %d jobs; showing %d\n\n", len(matched), len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		fmt.Printf("Company: %s\n", s.Company)
		fmt.Printf("Category: %s\n", s.Category)
		if s.Location != "" {
			fmt.Printf("Location: %s\n", s.Location)
		}
		if s.PostedAt != "" {
			fmt.Printf("Posted: %s\n", s.PostedAt)
		}
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
}

func runDetail(ctx context.Context, f detailFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	j, err := newClient(f.baseURL).Detail(ctx, f.jobID)
	if err != nil {
		return err
	}
	return printDetail(*j, f.format)
}

// printDetail renders one full job. JSON mode encodes the Job as-is —
// detail is for seeing the whole record.
func printDetail(j workingnomads.Job, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(j)
	}

	fmt.Println(j.Title)
	fmt.Printf("Company: %s\n", j.Company)
	fmt.Printf("Category: %s\n", j.Category)
	if j.Location != "" {
		fmt.Printf("Location: %s\n", j.Location)
	}
	if len(j.Tags) > 0 {
		fmt.Printf("Tags: %v\n", j.Tags)
	}
	if !j.PostedAt.IsZero() {
		fmt.Printf("Posted: %s\n", j.PostedAt.Format(time.RFC3339))
	}
	fmt.Printf("URL: %s\n", j.URL)

	rendered, err := html2text.FromString(j.Description, html2text.Options{})
	if err != nil {
		rendered = j.Description
	}
	fmt.Printf("\nDescription:\n%s\n", rendered)
	return nil
}
