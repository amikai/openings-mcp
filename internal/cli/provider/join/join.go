package join

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	joinprovider "github.com/amikai/openings-mcp/internal/provider/join"
)

const apiBaseURL = "https://join.com"

type options struct {
	company string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for join.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "join",
		Short:        "join --company COMPANY [FLAGS] <companies|search|get> [FLAGS]",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.company, "company", "", "confirmed join.com company slug, e.g. routinelabs (see 'join companies' for the full list)")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list confirmed join.com companies (company name and slug)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runCompanies(opts.format)
		},
	}

	var (
		searchKeyword  string
		searchLocation string
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "list a company's jobs as summaries (client-side filters; upstream has no server-side search)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runSearch(cmd.Context(), searchFlags{
				company:  opts.company,
				timeout:  opts.timeout,
				keyword:  searchKeyword,
				location: searchLocation,
				format:   opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "case-insensitive substring filter on job titles (empty lists every job)")
	searchCmd.Flags().StringVar(&searchLocation, "location", "", "case-insensitive substring filter on city names")

	var getIDParam string
	getCmd := &cobra.Command{
		Use:          "get",
		Short:        "print one job in full (scraped from the SSR detail page)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runGet(cmd.Context(), getFlags{
				company: opts.company,
				timeout: opts.timeout,
				idParam: getIDParam,
				format:  opts.format,
			})
		},
	}
	getCmd.Flags().StringVar(&getIDParam, "id", "", "job idParam from a search result (not the numeric id)")

	rootCmd.AddCommand(companiesCmd, searchCmd, getCmd)
	return rootCmd
}

func normalizeCompany(company string) (joinprovider.RosterCompany, error) {
	if company == "" {
		return joinprovider.RosterCompany{}, errors.New("--company is required")
	}
	c, ok := joinprovider.CompaniesBySlug[strings.ToLower(company)]
	if !ok {
		return joinprovider.RosterCompany{}, fmt.Errorf("company %q not found; run 'join companies' to see supported companies", company)
	}
	return c, nil
}

func runCompanies(format string) error {
	cs := joinprovider.Companies

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

func summarize(j joinprovider.Job) jobSummaryJSON {
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

func matches(s jobSummaryJSON, keyword, location string) bool {
	return containsFold(s.Title, keyword) && containsFold(s.Location, location)
}

func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

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

type searchFlags struct {
	company  string
	timeout  time.Duration
	keyword  string
	location string
	format   string
}

func runSearch(ctx context.Context, f searchFlags) error {
	c, err := normalizeCompany(f.company)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := joinprovider.NewClient(apiBaseURL, nil)
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

type getFlags struct {
	company string
	timeout time.Duration
	idParam string
	format  string
}

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

	client := joinprovider.NewClient(apiBaseURL, nil)
	d, err := client.JobDetail(ctx, c.Slug, f.idParam)
	if err != nil {
		if errors.Is(err, joinprovider.ErrNotFound) {
			return fmt.Errorf("job %q not found for company %q", f.idParam, c.Slug)
		}
		return err
	}
	return printDetail(d, c, f.format)
}

func printDetail(d *joinprovider.JobDetail, c joinprovider.RosterCompany, format string) error {
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
