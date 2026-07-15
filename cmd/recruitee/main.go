package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	recruitee "github.com/amikai/openings-mcp/internal/provider/recruitee"
)

func main() {
	rootFlags := ff.NewFlagSet("recruitee")
	var (
		board   = rootFlags.StringLong("board", "", "confirmed Recruitee subdomain slug, e.g. bunq (see 'recruitee companies' for the full list)")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "recruitee",
		Usage: "recruitee --board BOARD [FLAGS] <companies|search|get> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "recruitee companies [--format text|json]",
		ShortHelp: "list confirmed Recruitee subdomains (company name and slug)",
		Flags:     companiesFlags,
		Exec: func(_ context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			return runCompanies(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	keyword := searchFS.StringLong("keyword", "", "case-insensitive substring filter on job titles (empty lists every job)")
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "recruitee --board BOARD search [--keyword TEXT] [--format text|json]",
		ShortHelp: "list a board's jobs as summaries (client-side keyword filter)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{board: *board, timeout: *timeout, keyword: *keyword, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	getFS := ff.NewFlagSet("get").SetParent(rootFlags)
	jobID := getFS.StringLong("id", "", "job posting ID from search results")
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "recruitee --board BOARD get --id ID [--format text|json]",
		ShortHelp: "print one job in full (description)",
		Flags:     getFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runGet(ctx, getFlags{board: *board, timeout: *timeout, jobID: *jobID, format: *format})
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

func runCompanies(format string) error {
	cs := recruitee.Companies

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

func fetchOffers(ctx context.Context, board string, timeout time.Duration) (*recruitee.OffersResponse, error) {
	if board == "" {
		return nil, errors.New("--board is required")
	}
	slug := strings.ToLower(board)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := recruitee.NewClient("https://" + slug + ".recruitee.com")
	if err != nil {
		return nil, err
	}

	res, err := client.GetOffers(ctx)
	if err != nil {
		return nil, err
	}
	switch r := res.(type) {
	case *recruitee.OffersResponse:
		return r, nil
	case *recruitee.GetOffersNotFound:
		return nil, fmt.Errorf("board %q not found upstream", board)
	default:
		return nil, fmt.Errorf("unexpected response type %T", res)
	}
}

type jobSummaryJSON struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Department  string `json:"department,omitempty"`
	Location    string `json:"location,omitempty"`
	PublishedAt string `json:"publishedAt"`
	URL         string `json:"url"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(slug string, o *recruitee.Offer) jobSummaryJSON {
	title := o.Title.Or("")
	dept := o.Department.Or("")
	loc := o.Location.Or("")

	jobURL := o.CareersURL.Or("")
	if jobURL == "" && !o.Slug.Null && o.Slug.Value != "" {
		jobURL = fmt.Sprintf("https://%s.recruitee.com/o/%s", slug, o.Slug.Value)
	}

	posted, _, _ := strings.Cut(o.PublishedAt.Or(o.CreatedAt.Or("")), " ")

	return jobSummaryJSON{
		ID:          o.ID,
		Title:       title,
		Department:  dept,
		Location:    loc,
		PublishedAt: posted,
		URL:         jobURL,
	}
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	board   string
	timeout time.Duration
	keyword string
	format  string
}

func runSearch(ctx context.Context, f searchFlags) error {
	resp, err := fetchOffers(ctx, f.board, f.timeout)
	if err != nil {
		return err
	}

	matched := make([]jobSummaryJSON, 0, len(resp.Offers))
	for _, o := range resp.Offers {
		title := o.Title.Or("")
		if f.keyword != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(f.keyword)) {
			continue
		}
		matched = append(matched, summarize(f.board, &o))
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(resp.Offers), Jobs: matched})
	}

	fmt.Printf("Recruitee Jobs Report for %q\n", f.board)
	fmt.Printf("Found %d jobs; showing %d\n\n", len(resp.Offers), len(matched))
	for i, s := range matched {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		fmt.Printf("  ID: %d\n", s.ID)
		if s.Department != "" {
			fmt.Printf("  Department: %s\n", s.Department)
		}
		if s.Location != "" {
			fmt.Printf("  Location: %s\n", s.Location)
		}
		fmt.Printf("  Posted: %s\n", s.PublishedAt)
		fmt.Printf("  URL: %s\n\n", s.URL)
	}
	return nil
}

// getFlags carries the parsed "get" subcommand flags into runGet.
type getFlags struct {
	board   string
	timeout time.Duration
	jobID   string
	format  string
}

func runGet(ctx context.Context, f getFlags) error {
	if f.jobID == "" {
		return errors.New("--id is required")
	}
	resp, err := fetchOffers(ctx, f.board, f.timeout)
	if err != nil {
		return err
	}
	for _, o := range resp.Offers {
		if strconv.Itoa(o.ID) == f.jobID {
			return printJob(f.board, &o, f.format)
		}
	}
	return fmt.Errorf("job %q not found on board %q", f.jobID, f.board)
}

func printJob(slug string, o *recruitee.Offer, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(o)
	}

	s := summarize(slug, o)
	fmt.Println(s.Title)
	fmt.Printf("ID: %d\n", s.ID)
	if s.Department != "" {
		fmt.Printf("Department: %s\n", s.Department)
	}
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	fmt.Printf("Posted: %s\n", s.PublishedAt)
	fmt.Printf("URL: %s\n", s.URL)
	fmt.Printf("Apply: %s\n", o.CareersApplyURL.Or(""))

	descHTML := o.Description.Or("")
	reqHTML := o.Requirements.Or("")
	fullHTML := descHTML
	if reqHTML != "" {
		fullHTML = fullHTML + "\n\n<h3>Requirements</h3>\n" + reqHTML
	}

	var desc string
	if fullHTML != "" {
		if text, err := html2text.FromString(fullHTML, html2text.Options{}); err == nil {
			desc = text
		}
	}
	desc = cmp.Or(desc, fullHTML)
	if desc != "" {
		fmt.Printf("\nDescription:\n%s\n", desc)
	}
	return nil
}
