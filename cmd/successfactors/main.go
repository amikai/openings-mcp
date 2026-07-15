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

	"github.com/amikai/openings-mcp/internal/provider/successfactors"
)

func main() {
	rootFlags := ff.NewFlagSet("successfactors")
	var (
		company = rootFlags.StringLong("company", "", `curated company name or career-site host, e.g. "SAP" or "jobs.sap.com"`)
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "successfactors",
		Usage: "successfactors --company COMPANY [FLAGS] <companies|search|facets|detail> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "successfactors companies [--format text|json]",
		ShortHelp: "list curated SuccessFactors companies (company name and career-site host)",
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
		keyword      = searchFS.StringLong("keyword", "", "free-text keyword search across title and description")
		location     = searchFS.StringLong("location", "", "free-text fuzzy location match")
		department   = searchFS.StringLong("department", "", "department facet raw value from 'facets' (not the translated label)")
		careerStatus = searchFS.StringLong("career-status", "", "career-status facet raw value from 'facets'")
		country      = searchFS.StringLong("country", "", "ISO 3166-1 alpha-2 country code, e.g. DE")
		filters      = searchFS.StringListLong("filter", "tenant facet as name=value (repeatable; run 'facets' for valid names and values)")
		startRow     = searchFS.IntLong("start-row", 0, "zero-based result offset")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "successfactors --company COMPANY search [--keyword TEXT] [--location TEXT] [--filter NAME=VALUE]... [--department VALUE] [--career-status VALUE] [--country CC] [--start-row N] [--format text|json]",
		ShortHelp: "search postings for a company (server-side filters)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				company:      *company,
				timeout:      *timeout,
				keyword:      *keyword,
				location:     *location,
				department:   *department,
				careerStatus: *careerStatus,
				country:      *country,
				filters:      *filters,
				startRow:     *startRow,
				format:       *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	facetsFS := ff.NewFlagSet("facets").SetParent(rootFlags)
	facetsKeyword := facetsFS.StringLong("keyword", "", "narrow facet counts to this keyword, same as search --keyword")
	facetsCmd := &ff.Command{
		Name:      "facets",
		Usage:     "successfactors --company COMPANY facets [--keyword TEXT] [--format text|json]",
		ShortHelp: "list this company's filter dimensions and live option counts",
		Flags:     facetsFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("facets takes no positional arguments, got %v", args)
			}
			return runFacets(ctx, facetsFlags{company: *company, timeout: *timeout, keyword: *facetsKeyword, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, facetsCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("id", "", "numeric job id from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "successfactors --company COMPANY detail --id JOB-ID [--format text|json]",
		ShortHelp: "print one posting in full (description and best-effort location/employer/posted date)",
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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, facets, or detail) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// resolveCompany accepts a curated company name or its career-site host
// directly, and returns the Company record — the CLI's --company mirrors
// what a caller could otherwise only get by reading companies.yaml.
func resolveCompany(company string) (successfactors.Company, error) {
	if company == "" {
		return successfactors.Company{}, errors.New("--company is required")
	}
	if c, ok := successfactors.CompaniesByHost[strings.ToLower(company)]; ok {
		return c, nil
	}
	for _, c := range successfactors.Companies {
		if strings.EqualFold(c.Name, company) {
			return c, nil
		}
	}
	return successfactors.Company{}, fmt.Errorf("company %q not found; run 'successfactors companies' to see supported companies", company)
}

func runCompanies(format string) error {
	cs := successfactors.Companies

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

type jobSummaryJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location,omitempty"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	company      string
	timeout      time.Duration
	keyword      string
	location     string
	department   string
	careerStatus string
	country      string
	filters      []string
	startRow     int
	format       string
}

func runSearch(ctx context.Context, f searchFlags) error {
	c, err := resolveCompany(f.company)
	if err != nil {
		return err
	}
	if f.startRow < 0 {
		return fmt.Errorf("--start-row must be >= 0, got %d", f.startRow)
	}
	filters, err := buildSearchFilters(f)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := successfactors.NewClient("https://"+c.Host, nil)
	res, err := client.Search(ctx, &successfactors.SearchRequest{
		Query:          f.keyword,
		LocationSearch: f.location,
		Filters:        filters,
		StartRow:       f.startRow,
	})
	if err != nil {
		return err
	}

	jobs := make([]jobSummaryJSON, len(res.Jobs))
	for i, j := range res.Jobs {
		jobs[i] = jobSummaryJSON{ID: j.ID, Title: j.Title, Location: j.Location}
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: res.TotalCount, Jobs: jobs})
	}

	fmt.Printf("SuccessFactors Jobs Report (company: %s)\n", c.Name)
	fmt.Printf("Found %d jobs; showing %d\n\n", res.TotalCount, len(jobs))
	for i, j := range jobs {
		fmt.Printf("%d. %s\n", i+1, j.Title)
		if j.Location != "" {
			fmt.Printf("Location: %s\n", j.Location)
		}
		fmt.Printf("ID: %s\n\n", j.ID)
	}
	return nil
}

func buildSearchFilters(f searchFlags) (map[string]string, error) {
	filters := make(map[string]string, len(f.filters)+3)
	for _, raw := range f.filters {
		name, value, ok := strings.Cut(raw, "=")
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if !ok || name == "" || value == "" {
			return nil, fmt.Errorf("--filter %q must be name=value", raw)
		}
		if _, exists := filters[name]; exists {
			return nil, fmt.Errorf("filter %q was specified more than once", name)
		}
		filters[name] = value
	}

	legacy := []struct {
		name  string
		value string
		flag  string
	}{
		{name: "department", value: f.department, flag: "--department"},
		{name: "customfield3", value: f.careerStatus, flag: "--career-status"},
		{name: "country", value: f.country, flag: "--country"},
	}
	for _, filter := range legacy {
		if filter.value == "" {
			continue
		}
		if _, exists := filters[filter.name]; exists {
			return nil, fmt.Errorf("filter %q conflicts with %q", filter.name, filter.flag)
		}
		filters[filter.name] = filter.value
	}
	return filters, nil
}

// facetsFlags carries the parsed "facets" subcommand flags into runFacets.
type facetsFlags struct {
	company string
	timeout time.Duration
	keyword string
	format  string
}

func runFacets(ctx context.Context, f facetsFlags) error {
	c, err := resolveCompany(f.company)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := successfactors.NewClient("https://"+c.Host, nil)
	res, err := client.FacetValues(ctx, &successfactors.SearchRequest{Query: f.keyword})
	if err != nil {
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res.Facets)
	}

	if len(res.Facets) == 0 {
		fmt.Println("no filter dimensions configured for this query")
		return nil
	}
	for dimension, options := range res.Facets {
		fmt.Printf("%s:\n", dimension)
		for _, o := range options {
			label := o.Translated
			if label == "" {
				label = o.Name
			}
			fmt.Printf("  %s (%s): %d\n", label, o.Name, o.Count)
		}
	}
	return nil
}

// detailFlags carries the parsed "detail" subcommand flags into runDetail.
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
	if f.jobID == "" {
		return errors.New("--id is required (take it from a search result's ID)")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := successfactors.NewClient("https://"+c.Host, nil)
	d, err := client.JobDetail(ctx, f.jobID)
	if err != nil {
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Title)
	if d.Employer != "" {
		fmt.Printf("Company: %s\n", d.Employer)
	}
	if d.Location != "" {
		fmt.Printf("Location: %s\n", d.Location)
	}
	if d.PostedAtRaw != "" {
		fmt.Printf("Posted: %s\n", d.PostedAtRaw)
	}
	fmt.Printf("URL: https://%s/job/%s/%s/\n", c.Host, f.jobID, f.jobID)

	if d.DescriptionHTML != "" {
		text, err := html2text.FromString(d.DescriptionHTML, html2text.Options{})
		if err != nil {
			text = d.DescriptionHTML
		}
		fmt.Printf("\n%s\n", text)
	}
	return nil
}
