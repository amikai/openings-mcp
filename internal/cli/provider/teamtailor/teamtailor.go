package teamtailor

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

	"github.com/amikai/openings-mcp/internal/provider/teamtailor"
)

const formatJSON = "json"

type rootOptions struct {
	host    string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for teamtailor.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "teamtailor",
		Short:        "Search Teamtailor jobs and view position details",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.host, "host", "", "curated Teamtailor career-site host, e.g. career.teamtailor.com")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated Teamtailor companies and career-site hosts",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runCompanies(opts.format)
		},
	}

	var searchKeyword string
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "list one career site's jobs with a client-side title filter",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runSearch(cmd.Context(), searchFlags{host: opts.host, timeout: opts.timeout, keyword: searchKeyword, format: opts.format})
		},
	}
	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "case-insensitive substring filter on job titles")

	var getJobID string
	getCmd := &cobra.Command{
		Use:          "get",
		Short:        "print one job in full",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runGet(cmd.Context(), getFlags{host: opts.host, timeout: opts.timeout, jobID: getJobID, format: opts.format})
		},
	}
	getCmd.Flags().StringVar(&getJobID, "id", "", "JSON Feed item id from a search result")

	cmd.AddCommand(companiesCmd, searchCmd, getCmd)
	return cmd
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
