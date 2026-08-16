package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/adp_myjobs"
)

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("adp_myjobs")
	var (
		company = rootFlags.StringLong("company", "", "MyJobs slug, e.g. guitarcenterexternal")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "adp_myjobs",
		Usage: "adp_myjobs --company SLUG [FLAGS] <companies|filters|search|get> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "adp_myjobs companies [--format text|json]",
		ShortHelp: "list curated ADP MyJobs career sites",
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
	keyword := searchFS.StringLong("keyword", "", "server-side $search over the posting")
	field := searchFS.StringLong("field", "", "custom filter as FIELDn=VALUE; run the filters subcommand for the codes and values")
	limit := searchFS.IntLong("limit", 20, "max jobs to print after filtering")
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "adp_myjobs --company SLUG search [--keyword TEXT] [--field FIELDn=VALUE] [--limit N]",
		ShortHelp: "list jobs from a MyJobs board (server-side keyword and custom filters)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			return runSearch(ctx, *company, *keyword, *field, *limit, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	filtersFS := ff.NewFlagSet("filters").SetParent(rootFlags)
	filtersCmd := &ff.Command{
		Name:      "filters",
		Usage:     "adp_myjobs --company SLUG filters",
		ShortHelp: "print the board's custom filter dimensions and values",
		Flags:     filtersFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("filters takes no positional arguments, got %v", args)
			}
			return runFilters(ctx, *company, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, filtersCmd)

	getFS := ff.NewFlagSet("get").SetParent(rootFlags)
	jobID := getFS.StringLong("id", "", "job reqId from search results")
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "adp_myjobs --company SLUG get --id ID",
		ShortHelp: "print one job with plain-text description",
		Flags:     getFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v", args)
			}
			return runGet(ctx, *company, *jobID, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, getCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, filters, search, or get) is required")
		return 1
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

func runCompanies(format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(adp_myjobs.Companies)
	}
	for _, c := range adp_myjobs.Companies {
		fmt.Printf("%s (%s)\n", c.Name, c.Slug)
	}
	return nil
}

func newClient() *adp_myjobs.Client {
	return adp_myjobs.NewClient(adp_myjobs.Config{
		HTTPClient: http.DefaultClient,
	})
}

func runFilters(ctx context.Context, company, format string) error {
	slug, err := requireSlug(company)
	if err != nil {
		return err
	}
	catalog, err := newClient().GetCustomFilters(ctx, slug)
	if err != nil {
		return err
	}
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(catalog)
	}
	for _, c := range catalog.FilterList {
		fmt.Printf("%s\t%s\t(%d values)\n", c.Category, c.CategoryLabel, len(c.FilterList))
		for _, v := range c.FilterList {
			fmt.Printf("\t%s\n", v.Value)
		}
	}
	return nil
}

func runSearch(ctx context.Context, company, keyword, field string, limit int, format string) error {
	slug, err := requireSlug(company)
	if err != nil {
		return err
	}
	if limit <= 0 {
		limit = 20
	}
	filters, err := parseFields(field)
	if err != nil {
		return err
	}
	page, err := newClient().ListJobRequisitions(ctx, slug, adp_myjobs.ListParams{
		Search:        strings.TrimSpace(keyword),
		CustomFilters: filters,
		Top:           limit,
	})
	if err != nil {
		return err
	}
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"total": page.Count, "shown": len(page.JobRequisitions), "jobs": page.JobRequisitions})
	}
	fmt.Printf("# %s total=%d shown=%d\n", slug, page.Count, len(page.JobRequisitions))
	for _, j := range page.JobRequisitions {
		fmt.Printf("%s\t%s\t%s\n", j.ReqIDString(), j.Title(), j.PrimaryLocation())
	}
	return nil
}

func runGet(ctx context.Context, company, jobID, format string) error {
	slug, err := requireSlug(company)
	if err != nil {
		return err
	}
	if jobID == "" {
		return errors.New("--id is required")
	}
	j, err := newClient().GetJobRequisition(ctx, slug, jobID)
	if err != nil {
		return err
	}
	html := j.JobDescription
	if j.JobQualifications != "" {
		html += "\n" + j.JobQualifications
	}
	desc, _ := html2text.FromString(html, html2text.Options{})
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"id": j.ReqIDString(), "title": j.Title(), "location": j.PrimaryLocation(),
			"url": adp_myjobs.ApplyURL(slug, j.ReqIDString()), "description": desc,
		})
	}
	fmt.Printf("%s\n%s\n%s\n\n%s\n", j.Title(), j.PrimaryLocation(), adp_myjobs.ApplyURL(slug, j.ReqIDString()), desc)
	return nil
}

// parseFields turns a "FIELDn=VALUE" flag into one upstream clause. The slot
// code is passed through as given: it is positional per tenant, so this debug
// CLI cannot validate it, and an unconfigured code is answered with the whole
// board rather than an error.
func parseFields(raw string) ([]adp_myjobs.CustomFilter, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	code, value, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(code) == "" || strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("--field wants FIELDn=VALUE, got %q", raw)
	}
	return []adp_myjobs.CustomFilter{{Field: strings.TrimSpace(code), Value: strings.TrimSpace(value)}}, nil
}

func requireSlug(company string) (string, error) {
	if company == "" {
		return "", errors.New("--company is required")
	}
	slug := strings.ToLower(company)
	if _, ok := adp_myjobs.CompaniesBySlug[slug]; !ok {
		return "", fmt.Errorf("company %q not in roster; run 'adp_myjobs companies'", company)
	}
	return slug, nil
}
