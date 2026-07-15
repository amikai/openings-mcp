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

	"github.com/amikai/openings-mcp/internal/provider/teamtailor"
)

const formatJSON = "json"

func main() {
	rootFlags := ff.NewFlagSet("teamtailor")
	var (
		host    = rootFlags.StringLong("host", "", "curated Teamtailor career-site host, e.g. career.teamtailor.com")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "teamtailor",
		Usage: "teamtailor --host HOST [FLAGS] <companies|search|get> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "teamtailor companies [--format text|json]",
		ShortHelp: "list curated Teamtailor companies and career-site hosts",
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
	keyword := searchFS.StringLong("keyword", "", "case-insensitive substring filter on job titles")
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "teamtailor --host HOST search [--keyword TEXT] [--format text|json]",
		ShortHelp: "list one career site's jobs with a client-side title filter",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{host: *host, timeout: *timeout, keyword: *keyword, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	getFS := ff.NewFlagSet("get").SetParent(rootFlags)
	jobID := getFS.StringLong("id", "", "JSON Feed item id from a search result")
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "teamtailor --host HOST get --id ITEM-ID [--format text|json]",
		ShortHelp: "print one job in full",
		Flags:     getFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runGet(ctx, getFlags{host: *host, timeout: *timeout, jobID: *jobID, format: *format})
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

func normalizeHost(host string) (string, error) {
	if host == "" {
		return "", errors.New("--host is required")
	}
	key := strings.ToLower(host)
	c, ok := teamtailor.CompaniesByHost[key]
	if !ok {
		return "", fmt.Errorf("host %q not found; run 'teamtailor companies' to see supported hosts", host)
	}
	return c.Host, nil
}

func runCompanies(format string) error {
	if format == formatJSON {
		return writeJSON(teamtailor.Companies)
	}
	for _, c := range teamtailor.Companies {
		fmt.Printf("%s (%s)\n", c.Name, c.Host)
	}
	return nil
}

func fetchFeed(
	ctx context.Context,
	host string,
	timeout time.Duration,
) (*teamtailor.CareerFeed, error) {
	host, err := normalizeHost(host)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := teamtailor.NewClient("https://" + host)
	if err != nil {
		return nil, fmt.Errorf("create teamtailor client for %q: %w", host, err)
	}
	res, err := client.GetJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch teamtailor feed for %q: %w", host, err)
	}
	switch r := res.(type) {
	case *teamtailor.CareerFeed:
		return r, nil
	case *teamtailor.GetJobsNotFound:
		return nil, fmt.Errorf("teamtailor host %q not found upstream", host)
	default:
		return nil, fmt.Errorf("unexpected teamtailor response type %T", res)
	}
}

type jobSummaryJSON struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Location    string `json:"location,omitempty"`
	PublishedAt string `json:"publishedAt"`
	URL         string `json:"url"`
}

type searchResultJSON struct {
	Jobs  []jobSummaryJSON `json:"jobs"`
	Total int              `json:"total"`
}

type jobDetailJSON struct {
	jobSummaryJSON
	Company     string `json:"company"`
	Description string `json:"description"`
}

func summarize(j teamtailor.CareerItem) jobSummaryJSON {
	return jobSummaryJSON{
		ID:          j.ID.String(),
		Title:       j.Title,
		Location:    locations(j.Jobposting.JobLocation),
		PublishedAt: j.DatePublished.UTC().Format("2006-01-02"),
		URL:         j.URL,
	}
}

func locations(places []teamtailor.Place) string {
	seen := make(map[string]bool, len(places))
	parts := make([]string, 0, len(places))
	for _, p := range places {
		label := p.Address.AddressLocality
		if label == "" {
			label = p.Address.AddressRegion.Or(p.Address.AddressCountry)
		}
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		parts = append(parts, label)
	}
	return strings.Join(parts, "; ")
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	host    string
	timeout time.Duration
	keyword string
	format  string
}

func runSearch(ctx context.Context, f searchFlags) error {
	feed, err := fetchFeed(ctx, f.host, f.timeout)
	if err != nil {
		return err
	}

	needle := strings.ToLower(strings.TrimSpace(f.keyword))
	jobs := make([]jobSummaryJSON, 0, len(feed.Items))
	for _, j := range feed.Items {
		if needle != "" && !strings.Contains(strings.ToLower(j.Title), needle) {
			continue
		}
		jobs = append(jobs, summarize(j))
	}

	if f.format == formatJSON {
		return writeJSON(searchResultJSON{Total: len(jobs), Jobs: jobs})
	}

	fmt.Printf("Teamtailor Jobs — %s\n", feed.Title)
	fmt.Printf("Found %d jobs; showing %d\n\n", len(feed.Items), len(jobs))
	for i, j := range jobs {
		fmt.Printf("%d. %s\n", i+1, j.Title)
		printSummary(j)
		fmt.Println()
	}
	return nil
}

func printSummary(j jobSummaryJSON) {
	if j.Location != "" {
		fmt.Printf("Location: %s\n", j.Location)
	}
	fmt.Printf("Published: %s\n", j.PublishedAt)
	fmt.Printf("ID: %s\n", j.ID)
	fmt.Printf("URL: %s\n", j.URL)
}

// getFlags carries the parsed "get" subcommand flags into runGet.
type getFlags struct {
	host    string
	timeout time.Duration
	jobID   string
	format  string
}

func runGet(ctx context.Context, f getFlags) error {
	if f.jobID == "" {
		return errors.New("--id is required")
	}
	feed, err := fetchFeed(ctx, f.host, f.timeout)
	if err != nil {
		return err
	}

	for _, j := range feed.Items {
		if j.ID.String() != f.jobID {
			continue
		}
		description, err := html2text.FromString(j.ContentHTML, html2text.Options{})
		if err != nil {
			return fmt.Errorf("convert job description: %w", err)
		}
		detail := jobDetailJSON{
			jobSummaryJSON: summarize(j),
			Company:        feed.Title,
			Description:    description,
		}
		if f.format == formatJSON {
			return writeJSON(detail)
		}
		fmt.Println(detail.Title)
		printSummary(detail.jobSummaryJSON)
		fmt.Println()
		fmt.Println(detail.Description)
		return nil
	}
	return fmt.Errorf("job %q not found for host %q; pass an id exactly as returned by search", f.jobID, f.host)
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode JSON output: %w", err)
	}
	return nil
}
