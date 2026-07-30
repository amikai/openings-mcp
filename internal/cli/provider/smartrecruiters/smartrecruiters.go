// Package smartrecruiters provides CLI command construction for SmartRecruiters.
package smartrecruiters

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

	smartrecruiters "github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)


type rootOptions struct {
	company string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for smartrecruiters.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "smartrecruiters",
		Short:        "smartrecruiters --company COMPANY [FLAGS] <companies|search|get> [FLAGS]",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.company, "company", "", `SmartRecruiters companyIdentifier from the career site URL, e.g. "Equinox" in jobs.smartrecruiters.com/Equinox`)
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated SmartRecruiters companies (company name and companyIdentifier)",
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
		keyword    string
		country    string
		region     string
		city       string
		department string
		limit      int
		offset     int
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
				company:    opts.company,
				timeout:    opts.timeout,
				keyword:    keyword,
				country:    country,
				region:     region,
				city:       city,
				department: department,
				limit:      limit,
				offset:     offset,
				format:     opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&keyword, "keyword", "", "free-text keyword search across posting titles")
	searchCmd.Flags().StringVar(&country, "country", "", "lowercase ISO country code, e.g. us")
	searchCmd.Flags().StringVar(&region, "region", "", "state/region code, e.g. TX")
	searchCmd.Flags().StringVar(&city, "city", "", "city name")
	searchCmd.Flags().StringVar(&department, "department", "", "department.id value from a search result (not the display label)")
	searchCmd.Flags().IntVar(&limit, "limit", 20, "page size (upstream caps at 100)")
	searchCmd.Flags().IntVar(&offset, "offset", 0, "zero-based result offset")

	var postingID string
	getCmd := &cobra.Command{
		Use:          "get",
		Short:        "print one posting in full (description sections and public URL)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runGet(cmd.Context(), getFlags{
				company:   opts.company,
				timeout:   opts.timeout,
				postingID: postingID,
				format:    opts.format,
			})
		},
	}
	getCmd.Flags().StringVar(&postingID, "id", "", "posting id from a search result")

	cmd.AddCommand(companiesCmd, searchCmd, getCmd)
	return cmd
}

func normalizeCompany(company string) (string, error) {
	if company == "" {
		return "", errors.New("--company is required")
	}
	c, ok := smartrecruiters.CompaniesByIdentifier[strings.ToLower(company)]
	if !ok {
		return "", fmt.Errorf("company %q not found; run 'smartrecruiters companies' to see supported companies", company)
	}
	return c.CompanyIdentifier, nil
}

func runCompanies(format string) error {
	cs := smartrecruiters.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.CompanyIdentifier)
	}
	return nil
}

type postingSummaryJSON struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Location   string `json:"location,omitempty"`
	Department string `json:"department,omitempty"`
	PostedAt   string `json:"postedAt,omitempty"`
}

type searchResultJSON struct {
	Total int                  `json:"total"`
	Jobs  []postingSummaryJSON `json:"jobs"`
}

func summarize(p smartrecruiters.PostingItem) postingSummaryJSON {
	s := postingSummaryJSON{
		ID:       p.ID.Value,
		Title:    p.Name.Value,
		Location: p.Location.Value.FullLocation.Value,
	}
	if dep, ok := p.Department.Get(); ok {
		s.Department = dep.Label.Value
	}
	if v, ok := p.ReleasedDate.Get(); ok {
		s.PostedAt = v.Format("2006-01-02")
	}
	return s
}

func printSummary(s postingSummaryJSON) {
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.Department != "" {
		fmt.Printf("Department: %s\n", s.Department)
	}
	if s.PostedAt != "" {
		fmt.Printf("Posted: %s\n", s.PostedAt)
	}
	fmt.Printf("ID: %s\n", s.ID)
}

type searchFlags struct {
	company    string
	timeout    time.Duration
	keyword    string
	country    string
	region     string
	city       string
	department string
	limit      int
	offset     int
	format     string
}

func runSearch(ctx context.Context, f searchFlags) error {
	company, err := normalizeCompany(f.company)
	if err != nil {
		return err
	}
	if f.limit < 1 || f.limit > 100 {
		return fmt.Errorf("--limit must be between 1 and 100, got %d", f.limit)
	}
	if f.offset < 0 {
		return fmt.Errorf("--offset must be >= 0, got %d", f.offset)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := smartrecruiters.NewClient(smartrecruiters.DefaultBaseURL)
	if err != nil {
		return err
	}

	params := smartrecruiters.ListPostingsParams{
		CompanyIdentifier: company,
		Limit:             smartrecruiters.NewOptInt(f.limit),
		Offset:            smartrecruiters.NewOptInt(f.offset),
	}
	if f.keyword != "" {
		params.Q = smartrecruiters.NewOptString(f.keyword)
	}
	if f.country != "" {
		params.Country = smartrecruiters.NewOptString(f.country)
	}
	if f.region != "" {
		params.Region = smartrecruiters.NewOptString(f.region)
	}
	if f.city != "" {
		params.City = smartrecruiters.NewOptString(f.city)
	}
	if f.department != "" {
		params.Department = smartrecruiters.NewOptString(f.department)
	}

	res, err := client.ListPostings(ctx, params)
	if err != nil {
		return err
	}

	jobs := make([]postingSummaryJSON, len(res.Content))
	for i, p := range res.Content {
		jobs[i] = summarize(p)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: res.TotalFound, Jobs: jobs})
	}

	fmt.Printf("SmartRecruiters Jobs Report (company: %s)\n", company)
	fmt.Printf("Found %d jobs; showing %d\n\n", res.TotalFound, len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

type getFlags struct {
	company   string
	timeout   time.Duration
	postingID string
	format    string
}

func runGet(ctx context.Context, f getFlags) error {
	company, err := normalizeCompany(f.company)
	if err != nil {
		return err
	}
	if f.postingID == "" {
		return errors.New("--id is required (take it from a search result's ID)")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := smartrecruiters.NewClient(smartrecruiters.DefaultBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetPosting(ctx, smartrecruiters.GetPostingParams{
		CompanyIdentifier: company,
		PostingId:         f.postingID,
	})
	if err != nil {
		return err
	}

	switch d := res.(type) {
	case *smartrecruiters.Posting:
		return printDetail(d, f.format)
	case *smartrecruiters.PostingErrorResponse:
		return fmt.Errorf("posting %q not found for company %q", f.postingID, company)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}

func printDetail(d *smartrecruiters.Posting, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Name.Value)
	if name := d.Company.Value.Name.Value; name != "" {
		fmt.Printf("Company: %s\n", name)
	}
	if loc := d.Location.Value.FullLocation.Value; loc != "" {
		fmt.Printf("Location: %s\n", loc)
	}
	if v, ok := d.ReleasedDate.Get(); ok {
		fmt.Printf("Posted: %s\n", v.Format("2006-01-02"))
	}
	if v, ok := d.PostingUrl.Get(); ok && v != "" {
		fmt.Printf("URL: %s\n", v)
	}

	if sections, ok := d.JobAd.Value.Sections.Get(); ok {
		printSection("Company Description", sections.CompanyDescription)
		printSection("Job Description", sections.JobDescription)
		printSection("Qualifications", sections.Qualifications)
		printSection("Additional Information", sections.AdditionalInformation)
	}
	return nil
}

func printSection(fallbackTitle string, opt smartrecruiters.OptJobAdSection) {
	sec, ok := opt.Get()
	if !ok || sec.Text.Value == "" {
		return
	}
	title := sec.Title.Value
	if title == "" {
		title = fallbackTitle
	}
	rendered, err := html2text.FromString(sec.Text.Value, html2text.Options{})
	if err != nil {
		rendered = sec.Text.Value
	}
	fmt.Printf("\n%s:\n%s\n", title, rendered)
}
