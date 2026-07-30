package icims

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	icimsprovider "github.com/amikai/openings-mcp/internal/provider/icims"
)

type options struct {
	company string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for icims.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "icims",
		Short:        "icims --company COMPANY [FLAGS] <companies|search|detail> [FLAGS]",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.company, "company", "", `curated company name or career-portal host, e.g. "Peraton" or "careers-peraton.icims.com"`)
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated iCIMS companies (company name and career-portal host)",
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
		searchPage     int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search postings for a company (server-side keyword/location)",
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
				page:     searchPage,
				format:   opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "free-text keyword search across title and description")
	searchCmd.Flags().StringVar(&searchLocation, "location", "", "free-text location match")
	searchCmd.Flags().IntVar(&searchPage, "page", 0, "zero-based upstream pr page index")

	var detailJobID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one posting in full (JSON-LD description and location)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runDetail(cmd.Context(), detailFlags{
				company: opts.company,
				timeout: opts.timeout,
				jobID:   detailJobID,
				format:  opts.format,
			})
		},
	}
	detailCmd.Flags().StringVar(&detailJobID, "id", "", "numeric job id from a search result")

	rootCmd.AddCommand(companiesCmd, searchCmd, detailCmd)
	return rootCmd
}

func resolveCompany(company string) (icimsprovider.Company, error) {
	if company == "" {
		return icimsprovider.Company{}, errors.New("--company is required")
	}
	if c, ok := icimsprovider.CompaniesByHost[strings.ToLower(company)]; ok {
		return c, nil
	}
	for _, c := range icimsprovider.Companies {
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
		return icimsprovider.Company{Name: host, Host: host}, nil
	}
	return icimsprovider.Company{}, fmt.Errorf("company %q not found; run 'icims companies' to see supported companies", company)
}

func runCompanies(format string) error {
	cs := icimsprovider.Companies
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

	req := icimsprovider.SearchRequest{Keyword: f.keyword, Page: f.page}
	if f.location != "" {
		req.Locations = []string{f.location}
	}
	client := icimsprovider.NewClient("https://"+c.Host, nil)
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

	client := icimsprovider.NewClient("https://"+c.Host, nil)
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
	fmt.Printf("URL: %s\n\n", icimsprovider.JobURL(c.Host, d.ID))
	fmt.Println(desc)
	return nil
}
