// Package rippling provides CLI command construction for Rippling.
package rippling

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

	rippling "github.com/amikai/openings-mcp/internal/provider/rippling"
)

// apiBaseURL is Rippling's public Job Board API origin — the single
// production server in the provider's openapi.yaml.
const apiBaseURL = "https://api.rippling.com/platform/api/ats/v1"

type rootOptions struct {
	board   string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for rippling.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "rippling",
		Short:        "rippling --board BOARD [FLAGS] <companies|search|get> [FLAGS]",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.board, "board", "", "confirmed Rippling board slug, e.g. pythian (see 'rippling companies' for the full list)")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list confirmed Rippling boards (company name and board slug)",
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
		keyword  string
		location string
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "list a board's jobs as summaries (client-side filters)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runSearch(cmd.Context(), searchFlags{
				board:    opts.board,
				timeout:  opts.timeout,
				keyword:  keyword,
				location: location,
				format:   opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&keyword, "keyword", "", "case-insensitive substring filter on job titles (empty lists every job)")
	searchCmd.Flags().StringVar(&location, "location", "", "case-insensitive substring filter on location names")

	var jobUUID string
	getCmd := &cobra.Command{
		Use:          "get",
		Short:        "print one job in full (description and pay ranges)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runGet(cmd.Context(), getFlags{
				board:   opts.board,
				timeout: opts.timeout,
				jobUUID: jobUUID,
				format:  opts.format,
			})
		},
	}
	getCmd.Flags().StringVar(&jobUUID, "id", "", "job posting uuid from search results")

	cmd.AddCommand(companiesCmd, searchCmd, getCmd)
	return cmd
}

// jobSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs, no description.
type jobSummaryJSON struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Department string   `json:"department,omitempty"`
	Locations  []string `json:"locations,omitempty"`
	URL        string   `json:"url,omitempty"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarizeDump(entries []rippling.JobListEntry) []jobSummaryJSON {
	byID := make(map[string]int)
	jobs := make([]jobSummaryJSON, 0, len(entries))
	for _, e := range entries {
		loc := e.WorkLocation.Value.Label.Value
		if i, ok := byID[e.UUID.Value]; ok {
			jobs[i].Locations = append(jobs[i].Locations, loc)
			continue
		}
		byID[e.UUID.Value] = len(jobs)
		s := jobSummaryJSON{
			ID:         e.UUID.Value,
			Title:      e.Name.Value,
			Department: e.Department.Value.Label.Value,
		}
		if v, ok := e.URL.Get(); ok {
			s.URL = v.String()
		}
		if loc != "" {
			s.Locations = append(s.Locations, loc)
		}
		jobs = append(jobs, s)
	}
	return jobs
}

func matches(s jobSummaryJSON, keyword, location string) bool {
	return containsFold(s.Title, keyword) && containsFold(strings.Join(s.Locations, "; "), location)
}

func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func renderDescription(content string) string {
	if text, err := html2text.FromString(content, html2text.Options{}); err == nil {
		return text
	}
	return content
}

func payRangeLine(r rippling.PayRangeDetail) string {
	span := fmt.Sprintf("%.0f – %.0f %s/%s",
		r.RangeStart.Value, r.RangeEnd.Value, r.Currency.Value, r.Frequency.Value)
	if l := r.Location.Value; l != "" {
		return l + ": " + span
	}
	return span
}

func printSummary(s jobSummaryJSON) {
	if len(s.Locations) > 0 {
		fmt.Printf("Locations: %s\n", strings.Join(s.Locations, "; "))
	}
	if s.Department != "" {
		fmt.Printf("Department: %s\n", s.Department)
	}
	if s.URL != "" {
		fmt.Printf("URL: %s\n", s.URL)
	}
	fmt.Printf("ID: %s\n", s.ID)
}

func runCompanies(format string) error {
	cs := rippling.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.BoardSlug)
	}
	return nil
}

func normalizeBoard(board string) (string, error) {
	if board == "" {
		return "", errors.New("--board is required")
	}
	slug := strings.ToLower(board)
	if _, ok := rippling.CompaniesByBoardSlug[slug]; !ok {
		return "", fmt.Errorf("board %q not found; run 'rippling companies' to see supported boards", board)
	}
	return slug, nil
}

type searchFlags struct {
	board    string
	timeout  time.Duration
	keyword  string
	location string
	format   string
}

func runSearch(ctx context.Context, f searchFlags) error {
	slug, err := normalizeBoard(f.board)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := rippling.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.ListJobs(ctx, rippling.ListJobsParams{BoardSlug: slug})
	if err != nil {
		return err
	}
	var entries []rippling.JobListEntry
	switch r := res.(type) {
	case *rippling.ListJobsOKApplicationJSON:
		entries = []rippling.JobListEntry(*r)
	case *rippling.BoardNotFoundError:
		return fmt.Errorf("board %q not found upstream", f.board)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}

	jobs := summarizeDump(entries)
	matched := make([]jobSummaryJSON, 0, len(jobs))
	for _, s := range jobs {
		if matches(s, f.keyword, f.location) {
			matched = append(matched, s)
		}
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(jobs), Jobs: matched})
	}

	fmt.Printf("Rippling Jobs Report (board: %s)\n", slug)
	fmt.Printf("Found %d jobs; showing %d\n\n", len(jobs), len(matched))
	for i, s := range matched {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

type getFlags struct {
	board   string
	timeout time.Duration
	jobUUID string
	format  string
}

func runGet(ctx context.Context, f getFlags) error {
	if f.jobUUID == "" {
		return errors.New("--id is required (take it from a search result's ID)")
	}
	slug, err := normalizeBoard(f.board)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := rippling.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetJob(ctx, rippling.GetJobParams{
		BoardSlug: slug,
		JobUUID:   f.jobUUID,
	})
	if err != nil {
		return err
	}
	switch r := res.(type) {
	case *rippling.JobDetail:
		return printDetail(r, f.format)
	case *rippling.JobNotFoundError:
		return fmt.Errorf("job %q not found on board %q", f.jobUUID, f.board)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}

func printDetail(d *rippling.JobDetail, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Name.Value)
	if d.CompanyName.Value != "" {
		fmt.Printf("Company: %s\n", d.CompanyName.Value)
	}
	if len(d.WorkLocations) > 0 {
		fmt.Printf("Locations: %s\n", strings.Join(d.WorkLocations, "; "))
	}
	if dep, ok := d.Department.Get(); ok && dep.Name.Value != "" {
		fmt.Printf("Department: %s\n", strings.Join(dep.DepartmentTree, " > "))
	}
	if et, ok := d.EmploymentType.Get(); ok && et.ID.Value != "" {
		fmt.Printf("Employment type: %s\n", et.ID.Value)
	}
	if v, ok := d.CreatedOn.Get(); ok {
		fmt.Printf("Posted: %s\n", v.Format("2006-01-02"))
	}
	if v, ok := d.URL.Get(); ok {
		fmt.Printf("URL: %s\n", v.String())
	}
	if len(d.PayRangeDetails) > 0 {
		fmt.Println("Pay ranges:")
		for _, r := range d.PayRangeDetails {
			fmt.Printf("  %s\n", payRangeLine(r))
		}
	}
	if desc, ok := d.Description.Get(); ok {
		parts := make([]string, 0, 2)
		for _, p := range []string{desc.Company.Value, desc.Role.Value} {
			if strings.TrimSpace(p) != "" {
				parts = append(parts, p)
			}
		}
		if len(parts) > 0 {
			fmt.Printf("\nDescription:\n%s\n", renderDescription(strings.Join(parts, "\n\n")))
		}
	}
	return nil
}
