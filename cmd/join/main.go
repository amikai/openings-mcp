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

	join "github.com/amikai/openings-mcp/internal/provider/join"
)

// apiBaseURL is join.com's own origin, host to both the public GraphQL
// endpoint and the SSR pages this client scrapes for detail (see
// internal/provider/join/API.md).
const apiBaseURL = "https://join.com"

func main() {
	rootFlags := ff.NewFlagSet("join")
	var (
		company = rootFlags.StringLong("company", "", "confirmed join.com company slug, e.g. routinelabs (see 'join companies' for the full list)")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "join",
		Usage: "join --company COMPANY [FLAGS] <companies|search|get> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "join companies [--format text|json]",
		ShortHelp: "list confirmed join.com companies (company name and slug)",
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
	var (
		keyword  = searchFS.StringLong("keyword", "", "case-insensitive substring filter on job titles (empty lists every job)")
		location = searchFS.StringLong("location", "", "case-insensitive substring filter on city names")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "join --company COMPANY search [--keyword TEXT] [--location TEXT] [--format text|json]",
		ShortHelp: "list a company's jobs as summaries (client-side filters; upstream has no server-side search)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{company: *company, timeout: *timeout, keyword: *keyword, location: *location, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	getFS := ff.NewFlagSet("get").SetParent(rootFlags)
	idParam := getFS.StringLong("id", "", "job idParam from a search result (not the numeric id)")
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "join --company COMPANY get --id ID-PARAM [--format text|json]",
		ShortHelp: "print one job in full (scraped from the SSR detail page)",
		Flags:     getFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runGet(ctx, getFlags{company: *company, timeout: *timeout, idParam: *idParam, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, getCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, or get) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// normalizeCompany requires --company to be a curated company and returns
// the roster's canonical slug — same policy as cmd/greenhouse's --board.
func normalizeCompany(company string) (join.RosterCompany, error) {
	if company == "" {
		return join.RosterCompany{}, errors.New("--company is required")
	}
	c, ok := join.CompaniesBySlug[strings.ToLower(company)]
	if !ok {
		return join.RosterCompany{}, fmt.Errorf("company %q not found; run 'join companies' to see supported companies", company)
	}
	return c, nil
}

// runCompanies lists every confirmed join.com company embedded in the CLI
// (internal/provider/join/companies.yaml), sorted by company name. It makes
// no network call.
func runCompanies(format string) error {
	cs := join.Companies

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

// jobSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs, no description (join.com's search
// endpoint never populates one — see API.md).
type jobSummaryJSON struct {
	IdParam  string `json:"idParam"`
	Title    string `json:"title"`
	Location string `json:"location,omitempty"`
	Category string `json:"category,omitempty"`
	PostedAt string `json:"postedAt,omitempty"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j join.Job) jobSummaryJSON {
	s := jobSummaryJSON{
		IdParam:  j.IdParam,
		Title:    j.Title,
		Location: j.City,
		Category: j.Category,
	}
	if !j.CreatedAt.IsZero() {
		s.PostedAt = j.CreatedAt.Format("2006-01-02")
	}
	return s
}

// matches applies the client-side search filters: case-insensitive
// substring on title (keyword) and city (location), ANDed. join.com's
// public API has no server-side keyword search, so this is the whole
// search — see API.md's "Why dump-style, not server-side search".
func matches(s jobSummaryJSON, keyword, location string) bool {
	return containsFold(s.Title, keyword) && containsFold(s.Location, location)
}

func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// printSummary prints one job's compact text block (everything below the
// title line).
func printSummary(s jobSummaryJSON) {
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.Category != "" {
		fmt.Printf("Category: %s\n", s.Category)
	}
	if s.PostedAt != "" {
		fmt.Printf("Posted: %s\n", s.PostedAt)
	}
	fmt.Printf("ID: %s\n", s.IdParam)
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	company  string
	timeout  time.Duration
	keyword  string
	location string
	format   string
}

// runSearch fetches the company's whole job dump (join.com has no
// server-side keyword search; Client.Jobs already loops every page) then
// filters client-side and prints summaries.
func runSearch(ctx context.Context, f searchFlags) error {
	c, err := normalizeCompany(f.company)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := join.NewClient(apiBaseURL, nil)
	jobs, err := client.Jobs(ctx, c.CompanyID)
	if err != nil {
		return err
	}

	matched := make([]jobSummaryJSON, 0, len(jobs))
	for _, j := range jobs {
		s := summarize(j)
		if matches(s, f.keyword, f.location) {
			matched = append(matched, s)
		}
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(jobs), Jobs: matched})
	}

	fmt.Printf("JOIN Jobs Report (company: %s)\n", c.Slug)
	fmt.Printf("Found %d jobs; showing %d\n\n", len(jobs), len(matched))
	for i, s := range matched {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

// getFlags carries the parsed "get" subcommand flags into runGet.
type getFlags struct {
	company string
	timeout time.Duration
	idParam string
	format  string
}

// runGet fetches one job's full posting by scraping its SSR detail page —
// join.com's public GraphQL API never populates a description (see
// API.md), so there is no API-based detail call to make instead.
func runGet(ctx context.Context, f getFlags) error {
	if f.idParam == "" {
		return errors.New("--id is required (take it from a search result's ID)")
	}
	c, err := normalizeCompany(f.company)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := join.NewClient(apiBaseURL, nil)
	d, err := client.JobDetail(ctx, c.Slug, f.idParam)
	if err != nil {
		if errors.Is(err, join.ErrNotFound) {
			return fmt.Errorf("job %q not found for company %q", f.idParam, c.Slug)
		}
		return err
	}
	return printDetail(d, c, f.format)
}

// printDetail renders one full job. JSON mode encodes the parsed JobDetail
// as-is — detail is for seeing the whole record.
func printDetail(d *join.JobDetail, c join.RosterCompany, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Title)
	fmt.Printf("Company: %s\n", c.Name)
	if d.City != "" {
		fmt.Printf("Location: %s\n", d.City)
	}
	if !d.CreatedAt.IsZero() {
		fmt.Printf("Posted: %s\n", d.CreatedAt.Format("2006-01-02"))
	}
	fmt.Printf("URL: %s\n", c.CareersURL()+"/"+d.IdParam)
	if d.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", d.Description)
	}
	return nil
}
