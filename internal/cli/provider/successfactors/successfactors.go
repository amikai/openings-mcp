// Package successfactors provides CLI command construction for SuccessFactors.
package successfactors

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

	"github.com/amikai/openings-mcp/internal/provider/successfactors"
)

type rootOptions struct {
	company string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for successfactors.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "successfactors",
		Short:        "successfactors --company COMPANY [FLAGS] <companies|search|facets|detail> [FLAGS]",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.company, "company", "", `curated company name or career-site host, e.g. "SAP" or "jobs.sap.com"`)
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated SuccessFactors companies (company name and career-site host)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runCompanies(opts.format)
		},
	}

	var (
		keyword      string
		location     string
		department   string
		careerStatus string
		country      string
		filters      []string
		startRow     int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search postings for a company (server-side filters)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runSearch(cmd.Context(), searchFlags{
				company:      opts.company,
				timeout:      opts.timeout,
				keyword:      keyword,
				location:     location,
				department:   department,
				careerStatus: careerStatus,
				country:      country,
				filters:      filters,
				startRow:     startRow,
				format:       opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&keyword, "keyword", "", "free-text keyword search across title and description")
	searchCmd.Flags().StringVar(&location, "location", "", "free-text fuzzy location match")
	searchCmd.Flags().StringVar(&department, "department", "", "department facet raw value from 'facets' (not the translated label)")
	searchCmd.Flags().StringVar(&careerStatus, "career-status", "", "career-status facet raw value from 'facets'")
	searchCmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 country code, e.g. DE")
	searchCmd.Flags().StringSliceVar(&filters, "filter", nil, "tenant facet as name=value (repeatable; run 'facets' for valid names and values)")
	searchCmd.Flags().IntVar(&startRow, "start-row", 0, "zero-based result offset")

	var facetsKeyword string
	facetsCmd := &cobra.Command{
		Use:          "facets",
		Short:        "list this company's filter dimensions and live option counts",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runFacets(cmd.Context(), facetsFlags{
				company: opts.company,
				timeout: opts.timeout,
				keyword: facetsKeyword,
				format:  opts.format,
			})
		},
	}
	facetsCmd.Flags().StringVar(&facetsKeyword, "keyword", "", "narrow facet counts to this keyword, same as search --keyword")

	var jobID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one posting in full (description and best-effort location/employer/posted date)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runDetail(cmd.Context(), detailFlags{
				company: opts.company,
				timeout: opts.timeout,
				jobID:   jobID,
				format:  opts.format,
			})
		},
	}
	detailCmd.Flags().StringVar(&jobID, "id", "", "numeric job id from a search result")

	cmd.AddCommand(companiesCmd, searchCmd, facetsCmd, detailCmd)
	return cmd
}

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
