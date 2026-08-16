// Command engage is a debug CLI for the engage provider
// (internal/provider/engage), en-gage.net's hosted ATS. It talks directly
// to the client used by the MCP server, for exercising a tenant board or
// posting without going through the MCP protocol.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/engage"
)

// apiBaseURL is en-gage.net's own origin — every tenant is hosted at
// en-gage.net/<slug>/.
const apiBaseURL = "https://en-gage.net"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("engage")
	var (
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "engage",
		Usage: "engage [FLAGS] <companies|search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "engage companies [--format text|json]",
		ShortHelp: "list curated engage companies (company name and slug)",
		Flags:     companiesFlags,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			return runCompanies(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "engage search SLUG [--format text|json]",
		ShortHelp: "print a tenant board's jobs",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("search takes exactly one positional argument (SLUG), got %v", args)
			}
			return runSearch(ctx, searchFlags{slug: args[0], timeout: *timeout, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "engage detail SLUG WORK-ID [--format text|json]",
		ShortHelp: "print one posting in full",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("detail takes exactly two positional arguments (SLUG WORK-ID), got %v", args)
			}
			return runDetail(ctx, detailFlags{slug: args[0], workID: args[1], timeout: *timeout, format: *format})
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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, or detail) is required")
		return 1
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

// normalizeSlug requires slug to be a curated company — same policy as
// cmd/greenhouse's --board and cmd/lever's --site — and returns the
// roster's canonical slug rather than whatever casing the caller typed.
func normalizeSlug(slug string) (string, error) {
	if slug == "" {
		return "", errors.New("a company slug is required")
	}
	c, ok := engage.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return "", fmt.Errorf("company slug %q not found; run 'engage companies' to see supported companies", slug)
	}
	return c.Slug, nil
}

// runCompanies lists every curated engage company embedded in the CLI
// (internal/provider/engage/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	cs := engage.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.Slug)
	}
	return nil
}

// searchFlags carries the parsed "search" subcommand arguments into
// runSearch.
type searchFlags struct {
	slug    string
	timeout time.Duration
	format  string
}

// jobSummaryJSON is the --format json shape for one board listing.
type jobSummaryJSON struct {
	WorkID         string `json:"workId"`
	Title          string `json:"title"`
	Salary         string `json:"salary,omitempty"`
	Area           string `json:"area,omitempty"`
	EmploymentType string `json:"employmentType,omitempty"`
}

type searchResultJSON struct {
	AnyAtCap bool             `json:"anyAtCap"`
	Jobs     []jobSummaryJSON `json:"jobs"`
}

func summarize(j engage.Job) jobSummaryJSON {
	return jobSummaryJSON{
		WorkID:         j.WorkID,
		Title:          j.Title,
		Salary:         j.Salary,
		Area:           j.Area,
		EmploymentType: j.EmploymentType,
	}
}

// runSearch fetches one tenant board and prints its jobs. When any
// employment-type category came back at [engage.CategoryCap], the board is
// a lower bound on the tenant's true job count, so this is one of the two
// places (the other is doc.go) that surfaces the cap explicitly rather than
// presenting the listing as exhaustive.
func runSearch(ctx context.Context, f searchFlags) error {
	slug, err := normalizeSlug(f.slug)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := engage.NewClient(apiBaseURL, nil)
	board, err := client.Board(ctx, slug)
	if err != nil {
		return err
	}

	jobs := board.Jobs()
	summaries := make([]jobSummaryJSON, len(jobs))
	for i, j := range jobs {
		summaries[i] = summarize(j)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{AnyAtCap: board.AnyAtCap(), Jobs: summaries})
	}

	fmt.Printf("engage Jobs Report (company: %s)\n", slug)
	fmt.Printf("Found %d jobs\n", len(jobs))
	if board.AnyAtCap() {
		fmt.Printf("WARNING: one or more employment categories hit the %d-job cap; the listing below is truncated and the true count is higher.\n", engage.CategoryCap)
	}
	fmt.Println()
	for i, s := range summaries {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		if s.EmploymentType != "" {
			fmt.Printf("Employment Type: %s\n", s.EmploymentType)
		}
		if s.Area != "" {
			fmt.Printf("Area: %s\n", s.Area)
		}
		if s.Salary != "" {
			fmt.Printf("Salary: %s\n", s.Salary)
		}
		fmt.Printf("Work ID: %s\n", s.WorkID)
		fmt.Println()
	}
	return nil
}

// detailFlags carries the parsed "detail" subcommand arguments into
// runDetail.
type detailFlags struct {
	slug    string
	workID  string
	timeout time.Duration
	format  string
}

// runDetail fetches one posting in full.
func runDetail(ctx context.Context, f detailFlags) error {
	slug, err := normalizeSlug(f.slug)
	if err != nil {
		return err
	}
	if f.workID == "" {
		return errors.New("a work id is required (take it from a search result's Work ID)")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := engage.NewClient(apiBaseURL, nil)
	detail, err := client.Job(ctx, slug, f.workID)
	if err != nil {
		return err
	}

	return printDetail(detail, f.format)
}

// printDetail renders one full posting.
func printDetail(d *engage.JobDetail, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Title)
	if d.Organization.Name != "" {
		fmt.Printf("Company: %s\n", d.Organization.Name)
	}
	if d.EmploymentType != "" {
		fmt.Printf("Employment Type: %s\n", d.EmploymentType)
	}
	loc := formatLocation(d.Location)
	if loc != "" {
		fmt.Printf("Location: %s\n", loc)
	}
	if !d.DatePosted.IsZero() {
		fmt.Printf("Posted: %s\n", d.DatePosted.Format("2006-01-02"))
	}
	for _, s := range d.Salaries {
		fmt.Printf("Salary: %s %s%s\n", s.MinValue, salaryMaxSuffix(s), unitSuffix(s.UnitText))
	}
	if d.Organization.SameAs != "" {
		fmt.Printf("Company URL: %s\n", d.Organization.SameAs)
	}

	for _, sec := range d.Sections {
		fmt.Printf("\n%s:\n%s\n", sec.Heading, sec.Text)
	}
	return nil
}

func salaryMaxSuffix(s engage.Salary) string {
	if s.MaxValue == "" {
		return ""
	}
	return "-" + s.MaxValue
}

func unitSuffix(unit string) string {
	if unit == "" {
		return ""
	}
	return " (" + unit + ")"
}

// formatLocation joins the JobDetail's address fields into one display
// line, skipping empty parts.
func formatLocation(l engage.Location) string {
	parts := make([]string, 0, 4)
	for _, p := range []string{l.Region, l.Locality, l.Street} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}
