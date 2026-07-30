// Package ashby implements the "openings-mcp ashby" debug CLI, for manual
// checks against the live surface that internal/provider/ashby documents.
package ashby

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	ashby "github.com/amikai/openings-mcp/internal/provider/ashby"
)

type options struct {
	board   string
	timeout time.Duration
	format  string
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	board   string
	timeout time.Duration
	keyword string
	format  string
}

// getFlags carries the parsed "get" subcommand flags into runGet.
type getFlags struct {
	board   string
	timeout time.Duration
	jobID   string
	format  string
}

// NewCommand returns a cobra.Command for ashby.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "ashby",
		Short:        "Ashby jobs CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.board, "board", "", "confirmed Ashby board slug, e.g. openai")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	clihelp.FormatVar(rootCmd.PersistentFlags(), &opts.format)

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list confirmed Ashby boards (company name and board slug)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompanies(opts.format)
		},
	}

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "list a board's jobs as summaries (client-side keyword filter)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sFlags.board = opts.board
			sFlags.timeout = opts.timeout
			sFlags.format = opts.format
			return runSearch(cmd.Context(), *sFlags)
		},
	}
	searchCmd.Flags().StringVar(&sFlags.keyword, "keyword", "", "case-insensitive substring filter on job titles")

	gFlags := &getFlags{}
	getCmd := &cobra.Command{
		Use:          "get",
		Short:        "print one job in full (description and compensation)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			gFlags.board = opts.board
			gFlags.timeout = opts.timeout
			gFlags.format = opts.format
			return runGet(cmd.Context(), *gFlags)
		},
	}
	getCmd.Flags().StringVar(&gFlags.jobID, "id", "", "job posting UUID from search results")

	rootCmd.AddCommand(companiesCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(getCmd)

	return rootCmd
}

// runCompanies lists every confirmed Ashby board embedded in the CLI
// (internal/provider/ashby/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	cs := ashby.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.Board)
	}
	return nil
}

// fetchBoard validates the board slug against the embedded roster, then
// fetches the board's entire job list with compensation included — the
// shared front half of search and get. The API's typed 404 is theoretically
// unreachable for roster boards but reported rather than swallowed.
func fetchBoard(ctx context.Context, board string, timeout time.Duration) (*ashby.JobBoardResponse, error) {
	if board == "" {
		return nil, errors.New("--board is required")
	}
	slug := strings.ToLower(board)
	if _, ok := ashby.CompaniesByBoard[slug]; !ok {
		return nil, fmt.Errorf("board %q not found; run 'ashby companies' to see supported boards", board)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := ashby.NewClient(ashby.DefaultBaseURL)
	if err != nil {
		return nil, err
	}

	res, err := client.GetJobBoard(ctx, ashby.GetJobBoardParams{
		JobBoardName:        slug,
		IncludeCompensation: ashby.NewOptBool(true),
	})
	if err != nil {
		return nil, err
	}
	switch r := res.(type) {
	case *ashby.JobBoardResponse:
		return r, nil
	case *ashby.GetJobBoardNotFound:
		return nil, fmt.Errorf("board %q not found upstream", board)
	default:
		return nil, fmt.Errorf("unexpected response type %T", res)
	}
}

// jobSummaryJSON is the --format json shape for one search result and the
// summary header of a get result: the compact fields a listing needs, no
// description.
type jobSummaryJSON struct {
	ID                 string   `json:"id,omitempty"`
	Title              string   `json:"title"`
	Department         string   `json:"department,omitempty"`
	Team               string   `json:"team,omitempty"`
	Location           string   `json:"location,omitempty"`
	SecondaryLocations []string `json:"secondaryLocations,omitempty"`
	WorkplaceType      string   `json:"workplaceType,omitempty"`
	IsRemote           *bool    `json:"isRemote,omitempty"`
	PublishedAt        string   `json:"publishedAt"`
	Compensation       string   `json:"compensation,omitempty"`
	URL                string   `json:"url"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j *ashby.JobPosting) jobSummaryJSON {
	s := jobSummaryJSON{
		ID:         j.ID.Value,
		Title:      j.Title.Value,
		Department: j.Department.Value,
		Team:       j.Team.Value,
		Location:   j.Location.Value,
		URL:        j.JobUrl.Value,
	}
	if v, ok := j.PublishedAt.Get(); ok {
		s.PublishedAt = v.Format("2006-01-02")
	}
	if !j.WorkplaceType.Null {
		s.WorkplaceType = string(j.WorkplaceType.Value)
	}
	if !j.IsRemote.Null {
		v := j.IsRemote.Value
		s.IsRemote = &v
	}
	for _, sl := range j.SecondaryLocations {
		if sl.Location.Set {
			s.SecondaryLocations = append(s.SecondaryLocations, sl.Location.Value)
		}
	}
	if j.Compensation.Set {
		if v, ok := j.Compensation.Value.CompensationTierSummary.Get(); ok {
			s.Compensation = v
		}
	}
	return s
}

// runSearch fetches the whole board and prints summaries, optionally
// filtered by a case-insensitive substring match on the title. There is no
// pagination — the API returns everything in one response.
func runSearch(ctx context.Context, f searchFlags) error {
	resp, err := fetchBoard(ctx, f.board, f.timeout)
	if err != nil {
		return err
	}

	matched := make([]jobSummaryJSON, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		if f.keyword != "" && !strings.Contains(strings.ToLower(j.Title.Value), strings.ToLower(f.keyword)) {
			continue
		}
		matched = append(matched, summarize(&j))
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(resp.Jobs), Jobs: matched})
	}

	fmt.Printf("Ashby Jobs Report\n")
	fmt.Printf("Found %d jobs; showing %d\n\n", len(resp.Jobs), len(matched))
	for i, s := range matched {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

// printSummary prints one job's compact text block (everything but the
// description). The locations block mirrors the workday CLI's singular/plural
// treatment: a bare "Location:" line when there's one, an itemized
// "Locations:" list when secondaries exist.
func printSummary(s jobSummaryJSON) {
	switch {
	case s.Department != "" && s.Team != "" && s.Team != s.Department:
		fmt.Printf("Department: %s / %s\n", s.Department, s.Team)
	case s.Department != "":
		fmt.Printf("Department: %s\n", s.Department)
	}
	if len(s.SecondaryLocations) > 0 {
		fmt.Println("Locations:")
		if s.Location != "" {
			fmt.Printf("  - %s\n", s.Location)
		}
		for _, l := range s.SecondaryLocations {
			fmt.Printf("  - %s\n", l)
		}
	} else if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.WorkplaceType != "" || (s.IsRemote != nil && *s.IsRemote) {
		workplace := s.WorkplaceType
		if workplace == "" {
			workplace = "(unspecified)"
		}
		if s.IsRemote != nil && *s.IsRemote {
			workplace += " (remote)"
		}
		fmt.Printf("Workplace: %s\n", workplace)
	}
	fmt.Printf("Posted: %s\n", s.PublishedAt)
	if s.Compensation != "" {
		fmt.Printf("Compensation: %s\n", s.Compensation)
	}
	fmt.Printf("URL: %s\n", s.URL)
	if s.ID != "" {
		fmt.Printf("ID: %s\n", s.ID)
	}
}

// runGet fetches the whole board (Ashby has no per-job endpoint) and prints
// the one job whose id matches, in full.
func runGet(ctx context.Context, f getFlags) error {
	if f.jobID == "" {
		return errors.New("--id is required")
	}
	resp, err := fetchBoard(ctx, f.board, f.timeout)
	if err != nil {
		return err
	}
	for _, j := range resp.Jobs {
		if j.ID.Value == f.jobID {
			return printJob(&j, f.format)
		}
	}
	return fmt.Errorf("job %q not found on board %q", f.jobID, f.board)
}

func printJob(j *ashby.JobPosting, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(j)
	}

	s := summarize(j)
	fmt.Println(s.Title)
	printSummary(s)
	fmt.Printf("Employment: %s\n", j.EmploymentType.Value)
	fmt.Printf("Apply: %s\n", j.ApplyUrl.Value)
	if j.Compensation.Set {
		printCompensation(j.Compensation.Value)
	}

	var desc string
	if v, ok := j.DescriptionHtml.Get(); ok {
		if text, err := html2text.FromString(v, html2text.Options{}); err == nil {
			desc = text
		}
	}
	desc = cmp.Or(desc, j.DescriptionPlain.Value)
	if desc != "" {
		fmt.Printf("\nDescription:\n%s\n", desc)
	}
	return nil
}

// printCompensation itemizes the tier/component detail beneath the one-line
// summary printSummary already showed. Jobs that publish no ranges send an
// empty tier list — print nothing rather than an empty header.
func printCompensation(c ashby.Compensation) {
	if len(c.CompensationTiers) == 0 {
		return
	}
	fmt.Println("Compensation:")
	for _, tier := range c.CompensationTiers {
		title := "(unnamed tier)"
		if v, ok := tier.Title.Get(); ok {
			title = v
		}
		fmt.Printf("  %s\n", title)
		for _, comp := range tier.Components {
			fmt.Printf("    - %s\n", componentLine(comp))
		}
	}
}

// componentLine renders one compensation component. The API's summary
// already carries the human-readable range and currency ("Estimated base
// salary $132K – $330K"), so it leads; compensationType and a non-NONE
// interval qualify it.
func componentLine(c ashby.CompensationComponent) string {
	line := cmp.Or(c.Summary.Value, c.CompensationType.Value)
	var quals []string
	if c.CompensationType.Value != "" && c.CompensationType.Value != line {
		quals = append(quals, c.CompensationType.Value)
	}
	if v, ok := c.Interval.Get(); ok && v != "NONE" {
		quals = append(quals, v)
	}
	if len(quals) > 0 {
		return fmt.Sprintf("%s (%s)", line, strings.Join(quals, ", "))
	}
	return line
}
