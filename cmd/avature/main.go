// Command avature is a debug CLI for the Avature career-portal client:
// live keyword search, posting detail, and the curated portal roster.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/avature"
)

func main() {
	rootFlags := ff.NewFlagSet("avature")
	var (
		company = rootFlags.StringLong("company", "", `curated company name or portal slug (e.g. "Bloomberg" or "koch.avature.net/careers"); an uncurated <host>/<portal> works too`)
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "avature",
		Usage: "avature --company COMPANY [FLAGS] <companies|search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "avature companies [--format text|json]",
		ShortHelp: "list curated Avature portals (company name and portal slug)",
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
		keyword = searchFS.StringLong("keyword", "", "full-text query over titles and descriptions (not a location filter)")
		offset  = searchFS.IntLong("offset", 0, "zero-based result offset; page size is portal-configured")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "avature --company COMPANY search [--keyword TEXT] [--offset N] [--format text|json]",
		ShortHelp: "fetch one listing page (server-side keyword search)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			if *offset < 0 {
				return fmt.Errorf("--offset must be >= 0, got %d", *offset)
			}
			return runSearch(ctx, *company, *timeout, *keyword, *offset, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("id", "", "numeric job id from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "avature --company COMPANY detail --id JOB-ID [--format text|json]",
		ShortHelp: "print one posting in full (metadata fields and description)",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runDetail(ctx, *company, *timeout, *jobID, *format)
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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, or detail) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// resolvePortal accepts a curated company name or portal slug, or an
// uncurated "<host>/<portal>" (with or without the https:// prefix) so
// roster candidates can be probed before curation.
func resolvePortal(company string) (baseURL string, err error) {
	company = strings.TrimSpace(company)
	if company == "" {
		return "", errors.New("--company is required")
	}
	for _, c := range avature.Companies {
		if strings.EqualFold(c.Name, company) || strings.EqualFold(c.Slug(), company) {
			return c.URL, nil
		}
	}
	slug := strings.TrimPrefix(company, "https://")
	if host, portal, ok := strings.Cut(slug, "/"); ok && strings.Contains(host, ".") && portal != "" {
		return "https://" + slug, nil
	}
	return "", fmt.Errorf("company %q not found; run 'avature companies', or pass a portal slug like koch.avature.net/careers", company)
}

// runCompanies lists every curated Avature portal embedded in the CLI
// (internal/provider/avature/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(avature.Companies)
	}
	for _, c := range avature.Companies {
		fmt.Printf("%s (%s)\n", c.Name, c.Slug())
	}
	return nil
}

func runSearch(ctx context.Context, company string, timeout time.Duration, keyword string, offset int, format string) error {
	baseURL, err := resolvePortal(company)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := avature.NewClient(baseURL, nil).Search(ctx, &avature.SearchRequest{Search: keyword, Offset: offset})
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Printf("Avature Jobs Report (portal: %s)\n", strings.TrimPrefix(baseURL, "https://"))
	switch {
	case res.Total >= 0:
		fmt.Printf("Found %d jobs; showing %d from offset %d\n\n", res.Total, len(res.Jobs), offset)
	case res.HasNext:
		fmt.Printf("Showing %d jobs from offset %d (total hidden by portal; more pages exist)\n\n", len(res.Jobs), offset)
	default:
		fmt.Printf("Showing %d jobs from offset %d (total hidden by portal; last page)\n\n", len(res.Jobs), offset)
	}
	for i, j := range res.Jobs {
		fmt.Printf("%d. %s\n", i+1, j.Title)
		if j.Location != "" {
			fmt.Printf("Location: %s\n", j.Location)
		}
		fmt.Printf("ID: %s\n", j.ID)
		fmt.Printf("URL: %s\n\n", j.URL)
	}
	return nil
}

func runDetail(ctx context.Context, company string, timeout time.Duration, jobID, format string) error {
	baseURL, err := resolvePortal(company)
	if err != nil {
		return err
	}
	if jobID == "" {
		return errors.New("--id is required (take it from a search result's ID)")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	d, err := avature.NewClient(baseURL, nil).JobDetail(ctx, jobID)
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Title)
	for _, f := range d.Fields {
		fmt.Printf("%s: %s\n", f.Label, f.Value)
	}
	fmt.Printf("URL: %s\n", d.URL)
	if d.DescriptionHTML != "" {
		text, err := html2text.FromString(d.DescriptionHTML, html2text.Options{})
		if err != nil {
			text = d.DescriptionHTML
		}
		fmt.Printf("\n%s\n", text)
	}
	return nil
}
