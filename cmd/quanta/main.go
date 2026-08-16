// Command quanta is a debug CLI for Quanta Computer's recruitment site
// (https://hr.quantatw.com/QuantaRecruit/Home/Job).
//
//	go run ./cmd/quanta search --keyword 伺服器
//	go run ./cmd/quanta detail --serial 106
//
// Every invocation fetches the full dump once and filters it client-side —
// the site's own query params are no-ops (see the deviation notes in
// internal/provider/quanta/openapi.yaml).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/quanta"
)

// apiBaseURL is the single production server in the provider's
// openapi.yaml.
const apiBaseURL = "https://hr.quantatw.com"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("quanta")
	timeout := rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
	format := rootFlags.StringEnumLong("format", "output format", "text", "json")
	rootCmd := &ff.Command{
		Name:  "quanta",
		Usage: "quanta [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword     = searchFS.StringLong("keyword", "", "case-insensitive substring over category, title, location, requirements, and keywords")
		locationIDs = searchFS.StringSetLong("location-id", "locati value from a search result; repeatable, matches any")
		categoryIDs = searchFS.StringSetLong("category-id", "capoid value from a search result; repeatable, matches any")
		limit       = searchFS.IntLong("limit", 20, "max results to print (filtering is client-side; the site has no paging)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "quanta search [--keyword TEXT] [--location-id ID]... [--category-id ID]... [--limit N] [--format text|json]",
		ShortHelp: "fetch the dump and filter it client-side (the site's query params are no-ops)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			if *limit < 1 {
				return fmt.Errorf("--limit must be >= 1, got %d", *limit)
			}
			return runSearch(ctx, searchFlags{
				timeout: *timeout,
				format:  *format,
				opts: quanta.FilterOptions{
					Keyword:     *keyword,
					LocationIDs: *locationIDs,
					CategoryIDs: *categoryIDs,
				},
				limit: *limit,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	serial := detailFS.StringLong("serial", "", "job serial from a search result (the id used in ?serial= share links)")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "quanta detail --serial SERIAL [--format text|json]",
		ShortHelp: "print one job in full (resolved from the dump; there is no detail endpoint)",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --serial %q?)", args, args[0])
			}
			if *serial == "" {
				return errors.New("--serial is required (take it from a search result's serial)")
			}
			return runDetail(ctx, detailFlags{timeout: *timeout, format: *format, serial: *serial})
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

// fetchJobs pulls the full dump — the only read the site supports.
func fetchJobs(ctx context.Context, timeout time.Duration) ([]quanta.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := quanta.NewClient(apiBaseURL)
	if err != nil {
		return nil, err
	}
	res, err := client.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	return res.GetJobResult(), nil
}

// jobSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs, no description or requirements.
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

func summarize(j quanta.Job) jobSummaryJSON {
	return jobSummaryJSON{
		Serial:   j.GetSerial(),
		JobCode:  j.GetJobCode(),
		Title:    j.GetTitle(),
		Category: j.GetCategoryName(),
		Location: j.GetLocationName(),
		Salary:   j.GetSalary(),
	}
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	timeout time.Duration
	format  string
	opts    quanta.FilterOptions
	limit   int
}

func runSearch(ctx context.Context, f searchFlags) error {
	jobs, err := fetchJobs(ctx, f.timeout)
	if err != nil {
		return err
	}

	matched := quanta.FilterJobs(jobs, f.opts)
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

// detailFlags carries the parsed "detail" subcommand flags into runDetail.
type detailFlags struct {
	timeout time.Duration
	format  string
	serial  string
}

// runDetail resolves one job from the dump by serial — the site has no
// detail endpoint, so a serial that has left the dump (or never existed)
// is simply not found.
func runDetail(ctx context.Context, f detailFlags) error {
	jobs, err := fetchJobs(ctx, f.timeout)
	if err != nil {
		return err
	}

	j, ok := quanta.FindBySerial(jobs, f.serial)
	if !ok {
		return fmt.Errorf("serial %q not found in the current dump (%d jobs); it may have expired", f.serial, len(jobs))
	}
	return printDetail(j, f.format)
}

// printDetail renders one full job. JSON mode encodes the generated Job
// as-is — detail is for seeing the whole record.
func printDetail(j quanta.Job, format string) error {
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
