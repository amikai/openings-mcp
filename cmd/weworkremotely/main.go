// Command weworkremotely is a debug CLI for We Work Remotely's public RSS feeds.
//
//	go run ./cmd/weworkremotely search --keyword golang --category "Full-Stack Programming"
//	go run ./cmd/weworkremotely detail --id lawnstarter-data-governance-platform-manager
//	go run ./cmd/weworkremotely categories
//
// A recognized --category fetches only that one feed; otherwise search
// fetches and merges all 10 category feeds. detail resolves the id against
// a fresh full-dump fetch — there is no per-job endpoint in use (see
// internal/provider/weworkremotely's package doc for why).
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

	"github.com/amikai/openings-mcp/internal/provider/weworkremotely"
)

const defaultBaseURL = "https://weworkremotely.com"

func main() {
	rootFlags := ff.NewFlagSet("weworkremotely")
	var (
		baseURL = rootFlags.StringLong("base-url", defaultBaseURL, "We Work Remotely base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "weworkremotely",
		Usage: "weworkremotely [FLAGS] <search|detail|categories> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword  = searchFS.StringLong("keyword", "", "case-insensitive substring over title, skills, and description")
		category = searchFS.StringLong("category", "", `category display name, e.g. "Full-Stack Programming" (see 'weworkremotely categories'); a recognized value fetches only that feed`)
		company  = searchFS.StringLong("company", "", "case-insensitive company name substring")
		jobType  = searchFS.StringLong("type", "", "exact job type, e.g. Full-Time or Contract")
		region   = searchFS.StringLong("region", "", "case-insensitive substring over region, country, and state")
		limit    = searchFS.IntLong("limit", 20, "max results to print (filtering is client-side)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "weworkremotely search [--keyword TEXT] [--category NAME] [--company TEXT] [--type TYPE] [--region TEXT] [--limit N] [--format text|json]",
		ShortHelp: "fetch one or all category feeds and filter client-side",
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
				opts: weworkremotely.FilterOptions{
					Keyword:  *keyword,
					Category: *category,
					Company:  *company,
					Type:     *jobType,
					Region:   *region,
				},
				limit: *limit,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("id", "", "job ID (URL slug) from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "weworkremotely detail --id JOB-ID [--format text|json]",
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

	categoriesFS := ff.NewFlagSet("categories").SetParent(rootFlags)
	categoriesCmd := &ff.Command{
		Name:      "categories",
		Usage:     "weworkremotely categories",
		ShortHelp: "list the fixed set of category feeds (name and slug)",
		Flags:     categoriesFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("categories takes no positional arguments, got %v", args)
			}
			for _, c := range weworkremotely.Categories {
				fmt.Printf("%s (%s)\n", c.Name, c.Slug)
			}
			return nil
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, categoriesCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, detail, or categories) is required")
		os.Exit(1)
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

func newClient(baseURL string) *weworkremotely.Client {
	return weworkremotely.NewClient(baseURL, nil)
}

// jobSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs, no HTML description.
type jobSummaryJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Company  string `json:"company"`
	Category string `json:"category"`
	Type     string `json:"type,omitempty"`
	Region   string `json:"region,omitempty"`
	PostedAt string `json:"postedAt,omitempty"`
	URL      string `json:"url"`
}

func summarize(j weworkremotely.Job) jobSummaryJSON {
	return jobSummaryJSON{
		ID:       j.ID,
		Title:    j.Title,
		Company:  j.Company,
		Category: j.Category,
		Type:     j.Type,
		Region:   j.Region,
		PostedAt: j.PostedAt.Format(time.RFC3339),
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
	opts    weworkremotely.FilterOptions
	limit   int
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	matched, err := newClient(f.baseURL).Search(ctx, f.opts)
	if len(matched) == 0 && err != nil {
		return err
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: some category feeds failed, results may be incomplete: %v\n", err)
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
		fmt.Printf("Category: %s (%s)\n", s.Category, s.Type)
		if s.Region != "" {
			fmt.Printf("Region: %s\n", s.Region)
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
func printDetail(j weworkremotely.Job, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(j)
	}

	fmt.Println(j.Title)
	fmt.Printf("Company: %s\n", j.Company)
	fmt.Printf("Category: %s (%s)\n", j.Category, j.Type)
	if j.Region != "" {
		fmt.Printf("Region: %s\n", j.Region)
	}
	if j.Country != "" {
		fmt.Printf("Country: %s\n", j.Country)
	}
	if j.State != "" {
		fmt.Printf("State: %s\n", j.State)
	}
	if j.Skills != "" {
		fmt.Printf("Skills: %s\n", j.Skills)
	}
	fmt.Printf("Posted: %s\n", j.PostedAt.Format(time.RFC3339))
	if !j.ExpiresAt.IsZero() {
		fmt.Printf("Expires: %s\n", j.ExpiresAt.Format(time.RFC3339))
	}
	fmt.Printf("URL: %s\n", j.URL)

	rendered, err := html2text.FromString(j.Description, html2text.Options{})
	if err != nil {
		rendered = j.Description
	}
	fmt.Printf("\nDescription:\n%s\n", rendered)
	return nil
}
