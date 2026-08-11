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
	rootFlags := ff.NewFlagSet("adp_myjobs")
	var (
		company = rootFlags.StringLong("company", "", "MyJobs slug, e.g. guitarcenterexternal")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "adp_myjobs",
		Usage: "adp_myjobs --company SLUG [FLAGS] <companies|search|get> [FLAGS]",
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
	keyword := searchFS.StringLong("keyword", "", "case-insensitive substring filter on job titles")
	limit := searchFS.IntLong("limit", 20, "max jobs to print after filtering")
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "adp_myjobs --company SLUG search [--keyword TEXT] [--limit N]",
		ShortHelp: "list jobs from a MyJobs board (full dump + local filter)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			return runSearch(ctx, *company, *keyword, *limit, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

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

func runSearch(ctx context.Context, company, keyword string, limit int, format string) error {
	slug, err := requireSlug(company)
	if err != nil {
		return err
	}
	if limit <= 0 {
		limit = 20
	}
	page, err := newClient().ListJobRequisitions(ctx, slug, adp_myjobs.ListParams{
		Search: strings.TrimSpace(keyword),
		Top:    limit,
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
