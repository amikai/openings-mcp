package engage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/engage"
)

// apiBaseURL is en-gage.net's own origin — every tenant is hosted at
// en-gage.net/<slug>/.
const apiBaseURL = "https://en-gage.net"

type options struct {
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for engage.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "engage",
		Short:        "en-gage.net hosted ATS postings CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated engage companies (company name and slug)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompanies(opts.format)
		},
	}

	searchCmd := &cobra.Command{
		Use:          "search SLUG",
		Short:        "print a tenant board's jobs",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), searchFlags{slug: args[0], timeout: opts.timeout, format: opts.format})
		},
	}

	detailCmd := &cobra.Command{
		Use:          "detail SLUG WORK-ID",
		Short:        "print one posting in full",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetail(cmd.Context(), detailFlags{slug: args[0], workID: args[1], timeout: opts.timeout, format: opts.format})
		},
	}

	rootCmd.AddCommand(companiesCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(detailCmd)

	return rootCmd
}

func normalizeSlug(slug string) (string, error) {
	if slug == "" {
		return "", errors.New("a company slug is required")
	}
	c, ok := engage.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return "", fmt.Errorf("company slug %q not found; run 'engage companies' to see supported companies", slug)
	}
	return c.Slug, nil
}

func runCompanies(format string) error {
	cs := engage.Companies

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

type searchFlags struct {
	slug    string
	timeout time.Duration
	format  string
}

type jobSummaryJSON struct {
	WorkID         string `json:"workId"`
	Title          string `json:"title"`
	Salary         string `json:"salary,omitempty"`
	Area           string `json:"area,omitempty"`
	EmploymentType string `json:"employmentType,omitempty"`
}

type searchResultJSON struct {
	AnyAtCap bool             `json:"anyAtCap"`
	Jobs     []jobSummaryJSON `json:"jobs"`
}

func summarize(j engage.Job) jobSummaryJSON {
	return jobSummaryJSON{
		WorkID:         j.WorkID,
		Title:          j.Title,
		Salary:         j.Salary,
		Area:           j.Area,
		EmploymentType: j.EmploymentType,
	}
}

func runSearch(ctx context.Context, f searchFlags) error {
	slug, err := normalizeSlug(f.slug)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := engage.NewClient(apiBaseURL, nil)
	board, err := client.Board(ctx, slug)
	if err != nil {
		return err
	}

	jobs := board.Jobs()
	summaries := make([]jobSummaryJSON, len(jobs))
	for i, j := range jobs {
		summaries[i] = summarize(j)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{AnyAtCap: board.AnyAtCap(), Jobs: summaries})
	}

	fmt.Printf("engage Jobs Report (company: %s)\n", slug)
	fmt.Printf("Found %d jobs\n", len(jobs))
	if board.AnyAtCap() {
		fmt.Printf("WARNING: one or more employment categories hit the %d-job cap; the listing below is truncated and the true count is higher.\n", engage.CategoryCap)
	}
	fmt.Println()
	for i, s := range summaries {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		if s.EmploymentType != "" {
			fmt.Printf("Employment Type: %s\n", s.EmploymentType)
		}
		if s.Area != "" {
			fmt.Printf("Area: %s\n", s.Area)
		}
		if s.Salary != "" {
			fmt.Printf("Salary: %s\n", s.Salary)
		}
		fmt.Printf("Work ID: %s\n", s.WorkID)
		fmt.Println()
	}
	return nil
}

type detailFlags struct {
	slug    string
	workID  string
	timeout time.Duration
	format  string
}

func runDetail(ctx context.Context, f detailFlags) error {
	slug, err := normalizeSlug(f.slug)
	if err != nil {
		return err
	}
	if f.workID == "" {
		return errors.New("a work id is required (take it from a search result's Work ID)")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := engage.NewClient(apiBaseURL, nil)
	detail, err := client.Job(ctx, slug, f.workID)
	if err != nil {
		return err
	}

	return printDetail(detail, f.format)
}

func printDetail(d *engage.JobDetail, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Title)
	if d.Organization.Name != "" {
		fmt.Printf("Company: %s\n", d.Organization.Name)
	}
	if d.EmploymentType != "" {
		fmt.Printf("Employment Type: %s\n", d.EmploymentType)
	}
	loc := formatLocation(d.Location)
	if loc != "" {
		fmt.Printf("Location: %s\n", loc)
	}
	if !d.DatePosted.IsZero() {
		fmt.Printf("Posted: %s\n", d.DatePosted.Format("2006-01-02"))
	}
	for _, s := range d.Salaries {
		fmt.Printf("Salary: %s %s%s\n", s.MinValue, salaryMaxSuffix(s), unitSuffix(s.UnitText))
	}
	if d.Organization.SameAs != "" {
		fmt.Printf("Company URL: %s\n", d.Organization.SameAs)
	}

	for _, sec := range d.Sections {
		fmt.Printf("\n%s:\n%s\n", sec.Heading, sec.Text)
	}
	return nil
}

func salaryMaxSuffix(s engage.Salary) string {
	if s.MaxValue == "" {
		return ""
	}
	return "-" + s.MaxValue
}

func unitSuffix(unit string) string {
	if unit == "" {
		return ""
	}
	return " (" + unit + ")"
}

func formatLocation(l engage.Location) string {
	parts := make([]string, 0, 4)
	for _, p := range []string{l.Region, l.Locality, l.Street} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}
