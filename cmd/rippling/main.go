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

	rippling "github.com/amikai/openings-mcp/internal/provider/rippling"
)

// apiBaseURL is Rippling's public Job Board API origin — the single
// production server in the provider's openapi.yaml.
const apiBaseURL = "https://api.rippling.com/platform/api/ats/v1"

func main() {
	rootFlags := ff.NewFlagSet("rippling")
	var (
		board   = rootFlags.StringLong("board", "", "confirmed Rippling board slug, e.g. pythian (see 'rippling companies' for the full list)")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "rippling",
		Usage: "rippling --board BOARD [FLAGS] <companies|search|get> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "rippling companies [--format text|json]",
		ShortHelp: "list confirmed Rippling boards (company name and board slug)",
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
		keyword  = searchFS.StringLong("keyword", "", "case-insensitive substring filter on job titles (empty lists every job)")
		location = searchFS.StringLong("location", "", "case-insensitive substring filter on location names")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "rippling --board BOARD search [--keyword TEXT] [--location TEXT] [--format text|json]",
		ShortHelp: "list a board's jobs as summaries (client-side filters)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{board: *board, timeout: *timeout, keyword: *keyword, location: *location, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	getFS := ff.NewFlagSet("get").SetParent(rootFlags)
	jobUUID := getFS.StringLong("id", "", "job posting uuid from search results")
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "rippling --board BOARD get --id JOB-UUID [--format text|json]",
		ShortHelp: "print one job in full (description and pay ranges)",
		Flags:     getFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runGet(ctx, getFlags{board: *board, timeout: *timeout, jobUUID: *jobUUID, format: *format})
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
// projection of the generated rippling.JobListEntry so the CLI's output
// doesn't change shape when the spec's generated types do. One row per
// job — the upstream's per-location duplicate entries are merged here.
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

// summarizeDump merges the list's one-entry-per-(job, location) rows into
// one summary per job, preserving the board's ordering and each job's
// location order.
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

// matches applies the client-side search filters: case-insensitive
// substring on title (keyword) and any location name (location), ANDed.
// The Job Board API has no server-side filtering, so this is the whole
// search.
func matches(s jobSummaryJSON, keyword, location string) bool {
	return containsFold(s.Title, keyword) && containsFold(strings.Join(s.Locations, "; "), location)
}

func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// renderDescription converts the JD's HTML to plain text; on a conversion
// failure fall back to the raw HTML rather than dropping it.
func renderDescription(content string) string {
	if text, err := html2text.FromString(content, html2text.Options{}); err == nil {
		return text
	}
	return content
}

// payRangeLine renders one payRangeDetails band as
// "label: start – end CURRENCY/FREQUENCY". The label is the band's
// location, work mode, or role level.
func payRangeLine(r rippling.PayRangeDetail) string {
	span := fmt.Sprintf("%.0f – %.0f %s/%s",
		r.RangeStart.Value, r.RangeEnd.Value, r.Currency.Value, r.Frequency.Value)
	if l := r.Location.Value; l != "" {
		return l + ": " + span
	}
	return span
}

// printSummary prints one job's compact text block (everything below the
// title line).
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

// runCompanies lists every confirmed Rippling board embedded in the CLI
// (internal/provider/rippling/companies.yaml), sorted by company name. It
// makes no network call.
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

// normalizeBoard lowercases the --board value and requires it to be a
// curated board — same policy as cmd/greenhouse's normalizeBoard.
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

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	board    string
	timeout  time.Duration
	keyword  string
	location string
	format   string
}

// runSearch fetches the board's whole job list (the API has no pagination
// and no server-side filters), merges the per-location duplicate entries,
// then filters client-side and prints summaries.
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
		// Theoretically unreachable for roster boards, but reported
		// rather than swallowed.
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

// getFlags carries the parsed "get" subcommand flags into runGet.
type getFlags struct {
	board   string
	timeout time.Duration
	jobUUID string
	format  string
}

// runGet fetches one job in full via Rippling's single-job endpoint.
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

// printDetail renders one full job. JSON mode encodes the generated
// JobDetail as-is — detail is for seeing the whole record.
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
	// The human-readable label lives in id; label is the machine enum (the
	// swap documented on the EmploymentType schema).
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
