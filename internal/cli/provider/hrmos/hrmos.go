package hrmos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/hrmos"
)

type options struct {
	baseURL string
	company string
	timeout time.Duration
	format  string
}

type searchFlags struct {
	keyword  string
	location string
}

type detailFlags struct {
	jobID string
}

// NewCommand returns a cobra.Command for hrmos.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "hrmos",
		Short:        "HRMOS 採用 postings CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", hrmos.DefaultBaseURL, "HRMOS 採用 base URL")
	rootCmd.PersistentFlags().StringVar(&opts.company, "company", "", "HRMOS tenant slug, e.g. moneyforward (see 'hrmos companies' for the curated list; any live tenant slug also works)")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated HRMOS companies (company name and slug)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompanies(opts.format)
		},
	}

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "list a tenant's whole job dump (client-side filters; upstream has no server-side search)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), searchOptions{
				baseURL:  opts.baseURL,
				company:  opts.company,
				timeout:  opts.timeout,
				keyword:  sFlags.keyword,
				location: sFlags.location,
				format:   opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&sFlags.keyword, "keyword", "", "case-insensitive substring filter on job titles (empty lists every job)")
	searchCmd.Flags().StringVar(&sFlags.location, "location", "", "case-insensitive substring filter on the sg-tag-location address chip")

	dFlags := &detailFlags{}
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "fetch one posting's full JobPosting data",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetail(cmd.Context(), detailOptions{
				baseURL: opts.baseURL,
				company: opts.company,
				timeout: opts.timeout,
				jobID:   dFlags.jobID,
				format:  opts.format,
			})
		},
	}
	detailCmd.Flags().StringVar(&dFlags.jobID, "job-id", "", "job ID from a search result, e.g. 0000265")

	rootCmd.AddCommand(companiesCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(detailCmd)

	return rootCmd
}

func resolveSlug(company string) (string, error) {
	if company == "" {
		return "", errors.New("--company is required")
	}
	if c, ok := hrmos.CompaniesBySlug[strings.ToLower(company)]; ok {
		return c.Slug, nil
	}
	return company, nil
}

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

type searchOptions struct {
	baseURL, company  string
	timeout           time.Duration
	keyword, location string
	format            string
}

func runSearch(ctx context.Context, f searchOptions) error {
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

type detailOptions struct {
	baseURL, company string
	timeout          time.Duration
	jobID            string
	format           string
}

func runDetail(ctx context.Context, f detailOptions) error {
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
