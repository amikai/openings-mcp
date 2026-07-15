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

	"github.com/amikai/openings-mcp/internal/provider/icims"
)

func main() {
	rootFlags := ff.NewFlagSet("icims")
	var (
		company = rootFlags.StringLong("company", "", `curated company name or career-portal host, e.g. "Peraton" or "careers-peraton.icims.com"`)
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "icims",
		Usage: "icims --company COMPANY [FLAGS] <companies|search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "icims companies [--format text|json]",
		ShortHelp: "list curated iCIMS companies (company name and career-portal host)",
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
		keyword  = searchFS.StringLong("keyword", "", "free-text keyword search across title and description")
		location = searchFS.StringLong("location", "", "free-text location match")
		page     = searchFS.IntLong("page", 0, "zero-based upstream pr page index")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "icims --company COMPANY search [--keyword TEXT] [--location TEXT] [--page N] [--format text|json]",
		ShortHelp: "search postings for a company (server-side keyword/location)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				company:  *company,
				timeout:  *timeout,
				keyword:  *keyword,
				location: *location,
				page:     *page,
				format:   *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("id", "", "numeric job id from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "icims --company COMPANY detail --id JOB-ID [--format text|json]",
		ShortHelp: "print one posting in full (JSON-LD description and location)",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runDetail(ctx, detailFlags{company: *company, timeout: *timeout, jobID: *jobID, format: *format})
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

func resolveCompany(company string) (icims.Company, error) {
	if company == "" {
		return icims.Company{}, errors.New("--company is required")
	}
	if c, ok := icims.CompaniesByHost[strings.ToLower(company)]; ok {
		return c, nil
	}
	for _, c := range icims.Companies {
		if strings.EqualFold(c.Name, company) {
			return c, nil
		}
	}
	// Allow any *.icims.com host for live debugging beyond the seed roster.
	host := strings.ToLower(strings.TrimSpace(company))
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	if strings.HasSuffix(host, ".icims.com") {
		return icims.Company{Name: host, Host: host}, nil
	}
	return icims.Company{}, fmt.Errorf("company %q not found; run 'icims companies' to see supported companies", company)
}

func runCompanies(format string) error {
	cs := icims.Companies
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}
	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.Host)
	}
	return nil
}

type searchFlags struct {
	company  string
	timeout  time.Duration
	keyword  string
	location string
	page     int
	format   string
}

func runSearch(ctx context.Context, f searchFlags) error {
	c, err := resolveCompany(f.company)
	if err != nil {
		return err
	}
	if f.page < 0 {
		return fmt.Errorf("--page must be >= 0, got %d", f.page)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	req := icims.SearchRequest{Keyword: f.keyword, Page: f.page}
	if f.location != "" {
		req.Locations = []string{f.location}
	}
	client := icims.NewClient("https://"+c.Host, nil)
	res, err := client.Search(ctx, &req)
	if err != nil {
		return err
	}

	type jobJSON struct {
		ID       string `json:"id"`
		Slug     string `json:"slug,omitempty"`
		Title    string `json:"title"`
		Location string `json:"location,omitempty"`
	}
	jobs := make([]jobJSON, len(res.Jobs))
	for i, j := range res.Jobs {
		jobs[i] = jobJSON{ID: j.ID, Slug: j.Slug, Title: j.Title, Location: j.Location}
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"total_pages": res.TotalPages,
			"page_size":   res.PageSize,
			"jobs":        jobs,
		})
	}

	fmt.Printf("iCIMS Jobs Report (company: %s)\n", c.Name)
	fmt.Printf("Upstream page size %d; total pages %d; showing %d\n\n", res.PageSize, res.TotalPages, len(jobs))
	for i, j := range jobs {
		fmt.Printf("%d. %s\n", i+1, j.Title)
		if j.Location != "" {
			fmt.Printf("Location: %s\n", j.Location)
		}
		fmt.Printf("ID: %s\n\n", j.ID)
	}
	return nil
}

type detailFlags struct {
	company string
	timeout time.Duration
	jobID   string
	format  string
}

func runDetail(ctx context.Context, f detailFlags) error {
	c, err := resolveCompany(f.company)
	if err != nil {
		return err
	}
	if strings.TrimSpace(f.jobID) == "" {
		return errors.New("--id is required")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := icims.NewClient("https://"+c.Host, nil)
	d, err := client.JobDetail(ctx, f.jobID)
	if err != nil {
		return err
	}

	desc := d.DescriptionHTML
	if desc != "" {
		if text, err := html2text.FromString(desc, html2text.Options{}); err == nil {
			desc = text
		}
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"id":              d.ID,
			"title":           d.Title,
			"location":        d.Location,
			"employer":        d.Employer,
			"posted_at":       d.PostedAtRaw,
			"employment_type": d.EmploymentType,
			"category":        d.Category,
			"url":             d.URL,
			"description":     desc,
		})
	}

	fmt.Printf("%s\n", d.Title)
	if d.Location != "" {
		fmt.Printf("Location: %s\n", d.Location)
	}
	if d.Employer != "" {
		fmt.Printf("Employer: %s\n", d.Employer)
	}
	if d.PostedAtRaw != "" {
		fmt.Printf("Posted: %s\n", d.PostedAtRaw)
	}
	if d.EmploymentType != "" {
		fmt.Printf("Type: %s\n", d.EmploymentType)
	}
	fmt.Printf("ID: %s\n", d.ID)
	fmt.Printf("URL: %s\n\n", icims.JobURL(c.Host, d.ID))
	fmt.Println(desc)
	return nil
}
