package main

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
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	greenhouse "github.com/amikai/openings-mcp/internal/provider/greenhouse"
)

// apiBaseURL is Greenhouse's public Job Board API origin — the single
// production server in the provider's openapi.yaml.
const apiBaseURL = "https://boards-api.greenhouse.io/v1"

func main() {
	rootFlags := ff.NewFlagSet("greenhouse")
	var (
		board   = rootFlags.StringLong("board", "", "confirmed Greenhouse board token, e.g. stripe (see 'greenhouse companies' for the full list)")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "greenhouse",
		Usage: "greenhouse --board BOARD [FLAGS] <companies|search|get> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "greenhouse companies [--format text|json]",
		ShortHelp: "list confirmed Greenhouse boards (company name and board token)",
		Flags:     companiesFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runCompanies(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	searchFlags := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword  = searchFlags.StringLong("keyword", "", "case-insensitive substring filter on job titles (empty lists every job)")
		location = searchFlags.StringLong("location", "", "case-insensitive substring filter on location names")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "greenhouse --board BOARD search [--keyword TEXT] [--location TEXT] [--format text|json]",
		ShortHelp: "list a board's jobs as summaries (client-side filters)",
		Flags:     searchFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runSearch(ctx, *board, *timeout, *keyword, *location, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	getFlags := ff.NewFlagSet("get").SetParent(rootFlags)
	jobID := getFlags.IntLong("id", 0, "job posting id from search results")
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "greenhouse --board BOARD get --id JOB-ID [--format text|json]",
		ShortHelp: "print one job in full (description and pay ranges)",
		Flags:     getFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runGet(ctx, *board, *timeout, *jobID, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, getCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, or get) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// jobSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs, no description. It's a flat, stable
// projection of the generated greenhouse.JobSummary so the CLI's output
// doesn't change shape when the spec's generated types do.
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
	if j.AbsoluteURL.Set {
		s.URL = j.AbsoluteURL.Value.String()
	}
	if j.FirstPublished.Set {
		s.PostedAt = j.FirstPublished.Value.Format("2006-01-02")
	}
	if j.UpdatedAt.Set {
		s.UpdatedAt = j.UpdatedAt.Value.Format("2006-01-02")
	}
	return s
}

// matches applies the client-side search filters: case-insensitive
// substring on title (keyword) and location name (location), ANDed. The
// Job Board API has no server-side filtering, so this is the whole search.
func matches(s jobSummaryJSON, keyword, location string) bool {
	return containsFold(s.Title, keyword) && containsFold(s.Location, location)
}

func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// formatCents renders a pay_input_ranges amount: whole currency units when
// the cents divide evenly (the common case), two decimals otherwise.
func formatCents(cents int) string {
	if cents%100 == 0 {
		return strconv.Itoa(cents / 100)
	}
	return strconv.FormatFloat(float64(cents)/100, 'f', 2, 64)
}

// payRangeLine renders one pay range as "title: min – max CURRENCY". The
// currency comes from currency_type verbatim — no hard-coded "$", the
// roster has EUR boards.
func payRangeLine(r greenhouse.PayInputRange) string {
	span := fmt.Sprintf("%s – %s %s",
		formatCents(r.MinCents.Value), formatCents(r.MaxCents.Value), r.CurrencyType.Value)
	if t := r.Title.Value; t != "" {
		return t + ": " + span
	}
	return span
}

// renderDescription converts a job's content field to plain text. Greenhouse
// sends it HTML entity-encoded, so decode first, then strip tags; on a
// conversion failure fall back to the decoded HTML rather than dropping it.
func renderDescription(content string) string {
	decoded := html.UnescapeString(content)
	if text, err := html2text.FromString(decoded, html2text.Options{}); err == nil {
		return text
	}
	return decoded
}

// printSummary prints one job's compact text block (everything below the
// title line).
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

// runCompanies lists every confirmed Greenhouse board embedded in the CLI
// (internal/provider/greenhouse/companies.yaml), sorted by company name. It
// makes no network call.
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

// normalizeBoard lowercases the --board value and requires it to be a
// curated board — same policy as cmd/ashby's fetchBoard front half.
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

// runSearch fetches the board's whole job list (the API has no pagination
// and no server-side filters) WITHOUT content=true — summaries stay small —
// then filters client-side and prints summaries.
func runSearch(ctx context.Context, board string, timeout time.Duration, keyword, location, format string) error {
	slug, err := normalizeBoard(board)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := greenhouse.NewClient(apiBaseURL)
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
		// Theoretically unreachable for roster boards, but reported
		// rather than swallowed.
		return fmt.Errorf("board %q not found upstream", board)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}

	matched := make([]jobSummaryJSON, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		s := summarize(j)
		if matches(s, keyword, location) {
			matched = append(matched, s)
		}
	}

	if format == "json" {
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

// runGet fetches one job in full via Greenhouse's single-job endpoint —
// unlike Ashby there's no need to re-fetch the whole board — with
// pay_transparency=true so pay_input_ranges come back.
func runGet(ctx context.Context, board string, timeout time.Duration, jobID int, format string) error {
	if jobID == 0 {
		return errors.New("--id is required (take it from a search result's ID)")
	}
	slug, err := normalizeBoard(board)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := greenhouse.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetJob(ctx, greenhouse.GetJobParams{
		BoardToken:      slug,
		JobID:           jobID,
		PayTransparency: greenhouse.NewOptBool(true),
	})
	if err != nil {
		return err
	}
	switch r := res.(type) {
	case *greenhouse.JobDetail:
		return printDetail(r, format)
	case *greenhouse.GetJobNotFound:
		return fmt.Errorf("job %d not found on board %q", jobID, board)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}

// printDetail renders one full job. JSON mode encodes the generated
// JobDetail as-is — detail is for seeing the whole record.
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
	if d.FirstPublished.Set {
		fmt.Printf("Posted: %s\n", d.FirstPublished.Value.Format("2006-01-02"))
	}
	if d.AbsoluteURL.Set {
		fmt.Printf("URL: %s\n", d.AbsoluteURL.Value.String())
	}
	if len(d.PayInputRanges) > 0 {
		fmt.Println("Pay ranges:")
		for _, r := range d.PayInputRanges {
			fmt.Printf("  %s\n", payRangeLine(r))
			if b := r.Blurb.Value; b != "" {
				// Blurbs arrive as HTML fragments; render them like the
				// description so no raw tags leak into text output.
				fmt.Printf("    %s\n", strings.TrimSpace(renderDescription(b)))
			}
		}
	}
	if d.Content.Value != "" {
		fmt.Printf("\nDescription:\n%s\n", renderDescription(d.Content.Value))
	}
	return nil
}
