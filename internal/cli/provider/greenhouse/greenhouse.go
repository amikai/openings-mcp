package greenhouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	greenhouse "github.com/amikai/openings-mcp/internal/provider/greenhouse"
)


type options struct {
	board   string
	timeout time.Duration
	format  string
}

type searchFlags struct {
	keyword  string
	location string
}

type getFlags struct {
	jobID int
}

// NewCommand returns a cobra.Command for greenhouse.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "greenhouse",
		Short:        "Greenhouse Job Board API CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.board, "board", "", "confirmed Greenhouse board token, e.g. stripe (see 'greenhouse companies' for the full list)")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list confirmed Greenhouse boards (company name and board token)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompanies(opts.format)
		},
	}

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "list a board's jobs as summaries (client-side filters)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), searchOptions{
				board:    opts.board,
				timeout:  opts.timeout,
				keyword:  sFlags.keyword,
				location: sFlags.location,
				format:   opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&sFlags.keyword, "keyword", "", "case-insensitive substring filter on job titles (empty lists every job)")
	searchCmd.Flags().StringVar(&sFlags.location, "location", "", "case-insensitive substring filter on location names")

	gFlags := &getFlags{}
	getCmd := &cobra.Command{
		Use:          "get",
		Short:        "print one job in full (description and pay ranges)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd.Context(), getOptions{
				board:   opts.board,
				timeout: opts.timeout,
				jobID:   gFlags.jobID,
				format:  opts.format,
			})
		},
	}
	getCmd.Flags().IntVar(&gFlags.jobID, "id", 0, "job posting id from search results")

	rootCmd.AddCommand(companiesCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(getCmd)

	return rootCmd
}

type jobSummaryJSON struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Location  string `json:"location,omitempty"`
	PostedAt  string `json:"postedAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	URL       string `json:"url,omitempty"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j greenhouse.JobSummary) jobSummaryJSON {
	s := jobSummaryJSON{
		ID:       j.ID.Value,
		Title:    j.Title.Value,
		Location: j.Location.Value.Name.Value,
	}
	if v, ok := j.AbsoluteURL.Get(); ok {
		s.URL = v.String()
	}
	if v, ok := j.FirstPublished.Get(); ok {
		s.PostedAt = v.Format("2006-01-02")
	}
	if v, ok := j.UpdatedAt.Get(); ok {
		s.UpdatedAt = v.Format("2006-01-02")
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

func formatCents(cents int) string {
	if cents%100 == 0 {
		return strconv.Itoa(cents / 100)
	}
	return strconv.FormatFloat(float64(cents)/100, 'f', 2, 64)
}

func payRangeLine(r greenhouse.PayInputRange) string {
	span := fmt.Sprintf("%s – %s %s",
		formatCents(r.MinCents.Value), formatCents(r.MaxCents.Value), r.CurrencyType.Value)
	if t := r.Title.Value; t != "" {
		return t + ": " + span
	}
	return span
}

func renderDescription(content string) string {
	decoded := html.UnescapeString(content)
	if text, err := html2text.FromString(decoded, html2text.Options{}); err == nil {
		return text
	}
	return decoded
}

func printSummary(s jobSummaryJSON) {
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.PostedAt != "" {
		fmt.Printf("Posted: %s\n", s.PostedAt)
	}
	if s.URL != "" {
		fmt.Printf("URL: %s\n", s.URL)
	}
	fmt.Printf("ID: %d\n", s.ID)
}

func runCompanies(format string) error {
	cs := greenhouse.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.BoardToken)
	}
	return nil
}

func normalizeBoard(board string) (string, error) {
	if board == "" {
		return "", errors.New("--board is required")
	}
	slug := strings.ToLower(board)
	if _, ok := greenhouse.CompaniesByBoardToken[slug]; !ok {
		return "", fmt.Errorf("board %q not found; run 'greenhouse companies' to see supported boards", board)
	}
	return slug, nil
}

type searchOptions struct {
	board    string
	timeout  time.Duration
	keyword  string
	location string
	format   string
}

func runSearch(ctx context.Context, f searchOptions) error {
	slug, err := normalizeBoard(f.board)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := greenhouse.NewClient(greenhouse.DefaultBaseURL)
	if err != nil {
		return err
	}

	res, err := client.ListJobs(ctx, greenhouse.ListJobsParams{BoardToken: slug})
	if err != nil {
		return err
	}
	var resp *greenhouse.JobListResponse
	switch r := res.(type) {
	case *greenhouse.JobListResponse:
		resp = r
	case *greenhouse.ListJobsNotFound:
		return fmt.Errorf("board %q not found upstream", f.board)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}

	matched := make([]jobSummaryJSON, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		s := summarize(j)
		if matches(s, f.keyword, f.location) {
			matched = append(matched, s)
		}
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(resp.Jobs), Jobs: matched})
	}

	fmt.Printf("Greenhouse Jobs Report (board: %s)\n", slug)
	fmt.Printf("Found %d jobs; showing %d\n\n", len(resp.Jobs), len(matched))
	for i, s := range matched {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

type getOptions struct {
	board   string
	timeout time.Duration
	jobID   int
	format  string
}

func runGet(ctx context.Context, f getOptions) error {
	if f.jobID == 0 {
		return errors.New("--id is required (take it from a search result's ID)")
	}
	slug, err := normalizeBoard(f.board)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := greenhouse.NewClient(greenhouse.DefaultBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetJob(ctx, greenhouse.GetJobParams{
		BoardToken:      slug,
		JobID:           f.jobID,
		PayTransparency: greenhouse.NewOptBool(true),
	})
	if err != nil {
		return err
	}
	switch r := res.(type) {
	case *greenhouse.JobDetail:
		return printDetail(r, f.format)
	case *greenhouse.GetJobNotFound:
		return fmt.Errorf("job %d not found on board %q", f.jobID, f.board)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}

func printDetail(d *greenhouse.JobDetail, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Title.Value)
	if d.CompanyName.Value != "" {
		fmt.Printf("Company: %s\n", d.CompanyName.Value)
	}
	if name := d.Location.Value.Name.Value; name != "" {
		fmt.Printf("Location: %s\n", name)
	}
	if v, ok := d.FirstPublished.Get(); ok {
		fmt.Printf("Posted: %s\n", v.Format("2006-01-02"))
	}
	if v, ok := d.AbsoluteURL.Get(); ok {
		fmt.Printf("URL: %s\n", v.String())
	}
	if len(d.PayInputRanges) > 0 {
		fmt.Println("Pay ranges:")
		for _, r := range d.PayInputRanges {
			fmt.Printf("  %s\n", payRangeLine(r))
			if b := r.Blurb.Value; b != "" {
				fmt.Printf("    %s\n", strings.TrimSpace(renderDescription(b)))
			}
		}
	}
	if d.Content.Value != "" {
		fmt.Printf("\nDescription:\n%s\n", renderDescription(d.Content.Value))
	}
	return nil
}
