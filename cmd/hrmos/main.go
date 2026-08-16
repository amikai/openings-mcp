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

	"github.com/amikai/openings-mcp/internal/provider/hrmos"
)

const _defaultBaseURL = "https://hrmos.co"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("hrmos")
	var (
		baseURL = rootFlags.StringLong("base-url", _defaultBaseURL, "HRMOS 採用 base URL")
		company = rootFlags.StringLong("company", "", "HRMOS tenant slug, e.g. moneyforward (see 'hrmos companies' for the curated list; any live tenant slug also works)")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "hrmos",
		Usage: "hrmos --company SLUG [FLAGS] <companies|search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "hrmos companies [--format text|json]",
		ShortHelp: "list curated HRMOS companies (company name and slug)",
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
		location = searchFS.StringLong("location", "", "case-insensitive substring filter on the sg-tag-location address chip")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "hrmos --company SLUG search [--keyword TEXT] [--location TEXT] [--format text|json]",
		ShortHelp: "list a tenant's whole job dump (client-side filters; upstream has no server-side search)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				baseURL: *baseURL, company: *company, timeout: *timeout,
				keyword: *keyword, location: *location, format: *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("job-id", "", "job ID from a search result, e.g. 0000265")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "hrmos --company SLUG detail --job-id ID [--format text|json]",
		ShortHelp: "fetch one posting's full JobPosting data",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --job-id %q?)", args, args[0])
			}
			return runDetail(ctx, detailFlags{
				baseURL: *baseURL, company: *company, timeout: *timeout,
				jobID: *jobID, format: *format,
			})
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

// resolveSlug accepts either a curated roster slug or an arbitrary live
// HRMOS tenant slug — unlike join.com, a HRMOS slug needs no roster lookup
// to resolve (see internal/ats.HrmosAdapter.ParseCareersURL).
func resolveSlug(company string) (string, error) {
	if company == "" {
		return "", errors.New("--company is required")
	}
	if c, ok := hrmos.CompaniesBySlug[strings.ToLower(company)]; ok {
		return c.Slug, nil
	}
	return company, nil
}

// runCompanies lists every curated HRMOS company embedded in the CLI
// (internal/provider/hrmos/companies.yaml). It makes no network call.
func runCompanies(format string) error {
	cs := hrmos.Companies

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

type searchFlags struct {
	baseURL, company  string
	timeout           time.Duration
	keyword, location string
	format            string
}

// runSearch fetches the tenant's whole job dump (Client.AllJobs already
// loops every page) then filters client-side and prints summaries.
func runSearch(ctx context.Context, f searchFlags) error {
	slug, err := resolveSlug(f.company)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := hrmos.NewClient(f.baseURL, nil)
	resp, err := client.AllJobs(ctx, slug)
	if err != nil {
		return err
	}

	matched := make([]hrmos.Job, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		if containsFold(j.Title, f.keyword) && containsFold(j.Location, f.location) {
			matched = append(matched, j)
		}
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Total  int                `json:"total"`
			Jobs   []hrmos.Job        `json:"jobs"`
			Facets []hrmos.FacetGroup `json:"facets"`
		}{Total: resp.Total, Jobs: matched, Facets: resp.Facets})
	}

	fmt.Printf("HRMOS Jobs Report (tenant: %s)\n", slug)
	fmt.Printf("Total %d; showing %d\n", resp.Total, len(matched))
	if len(resp.Facets) > 0 {
		fmt.Println("Facets:")
		for _, g := range resp.Facets {
			fmt.Printf("  %s: %s\n", g.Label, strings.Join(g.Options, ", "))
		}
	}
	fmt.Println()
	for i, j := range matched {
		fmt.Printf("%d. [%s] %s\n", i+1, j.ID, j.Title)
		if j.Location != "" {
			fmt.Printf("   location: %s\n", j.Location)
		}
		fmt.Println()
	}
	return nil
}

func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

type detailFlags struct {
	baseURL, company string
	timeout          time.Duration
	jobID            string
	format           string
}

func runDetail(ctx context.Context, f detailFlags) error {
	if f.jobID == "" {
		return errors.New("--job-id is required (take it from a search result)")
	}
	slug, err := resolveSlug(f.company)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := hrmos.NewClient(f.baseURL, nil)
	d, err := client.JobDetail(ctx, slug, f.jobID)
	if err != nil {
		if errors.Is(err, hrmos.ErrNotFound) {
			return fmt.Errorf("job %q not found for tenant %q", f.jobID, slug)
		}
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Printf("[%s] %s\n", d.ID, d.Title)
	fmt.Printf("company: %s\n", d.Company)
	if d.EmploymentType != "" {
		fmt.Printf("employment_type: %s\n", d.EmploymentType)
	}
	if d.DatePosted != "" {
		fmt.Printf("posted: %s\n", d.DatePosted)
	}
	if d.ValidThrough != "" {
		fmt.Printf("valid_through: %s\n", d.ValidThrough)
	}
	for _, loc := range d.Locations {
		fmt.Printf("location: %s%s%s (%s)\n", loc.Region, loc.Locality, loc.Street, loc.PostalCode)
	}
	if d.SalaryMin != "" || d.SalaryMax != "" {
		fmt.Printf("salary: %s-%s %s/%s\n", d.SalaryMin, d.SalaryMax, d.SalaryCurrency, d.SalaryUnit)
	} else if d.SalaryNote != "" {
		fmt.Printf("salary: %s\n", d.SalaryNote)
	}
	fmt.Printf("url: %s\n", d.URL)
	if d.Description != "" {
		fmt.Printf("\n%s\n", d.Description)
	}
	return nil
}
