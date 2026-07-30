package workingnomads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/workingnomads"
)

const defaultBaseURL = "https://www.workingnomads.com"

type rootOptions struct {
	baseURL string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for workingnomads.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "workingnomads",
		Short:        "Search Working Nomads jobs and view details",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", defaultBaseURL, "Working Nomads base URL")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	var (
		searchKeyword  string
		searchCategory string
		searchCompany  string
		searchLocation string
		searchLimit    int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "fetch the full jobs dump and filter client-side",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if searchLimit < 1 {
				return fmt.Errorf("--limit must be >= 1, got %d", searchLimit)
			}
			return runSearch(cmd.Context(), searchFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				opts: workingnomads.FilterOptions{
					Keyword:  searchKeyword,
					Category: searchCategory,
					Company:  searchCompany,
					Location: searchLocation,
				},
				limit: searchLimit,
			})
		},
	}
	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "case-insensitive substring over title, tags, and description")
	searchCmd.Flags().StringVar(&searchCategory, "category", "", "case-insensitive substring of category (e.g. Development, Design); no fixed enum")
	searchCmd.Flags().StringVar(&searchCompany, "company", "", "case-insensitive company name substring")
	searchCmd.Flags().StringVar(&searchLocation, "location", "", "case-insensitive substring over the free-text location field")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "max results to print (filtering is client-side)")

	var detailJobID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one job in full (resolved from a fresh full-dump fetch)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if detailJobID == "" {
				return errors.New("--id is required (take it from a search result's ID)")
			}
			return runDetail(cmd.Context(), detailFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				jobID:   detailJobID,
			})
		},
	}
	detailCmd.Flags().StringVar(&detailJobID, "id", "", "job ID (numeric, from a search result)")

	cmd.AddCommand(searchCmd, detailCmd)
	return cmd
}

func newClient(baseURL string) *workingnomads.Client {
	return workingnomads.NewClient(baseURL, nil)
}

type jobSummaryJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Company  string `json:"company"`
	Category string `json:"category"`
	Location string `json:"location,omitempty"`
	PostedAt string `json:"postedAt,omitempty"`
	URL      string `json:"url"`
}

func summarize(j workingnomads.Job) jobSummaryJSON {
	var postedAt string
	if !j.PostedAt.IsZero() {
		postedAt = j.PostedAt.Format(time.RFC3339)
	}
	return jobSummaryJSON{
		ID:       j.ID,
		Title:    j.Title,
		Company:  j.Company,
		Category: j.Category,
		Location: j.Location,
		PostedAt: postedAt,
		URL:      j.URL,
	}
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

type searchFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	opts    workingnomads.FilterOptions
	limit   int
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	matched, err := newClient(f.baseURL).Search(ctx, f.opts)
	if err != nil {
		return err
	}

	shown := matched
	if len(shown) > f.limit {
		shown = shown[:f.limit]
	}

	jobs := make([]jobSummaryJSON, len(shown))
	for i, j := range shown {
		jobs[i] = summarize(j)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(matched), Jobs: jobs})
	}

	fmt.Printf("Matched %d jobs; showing %d\n\n", len(matched), len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		fmt.Printf("Company: %s\n", s.Company)
		fmt.Printf("Category: %s\n", s.Category)
		if s.Location != "" {
			fmt.Printf("Location: %s\n", s.Location)
		}
		if s.PostedAt != "" {
			fmt.Printf("Posted: %s\n", s.PostedAt)
		}
		fmt.Printf("ID: %s\n", s.ID)
		fmt.Println()
	}
	return nil
}

type detailFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	jobID   string
}

func runDetail(ctx context.Context, f detailFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	j, err := newClient(f.baseURL).Detail(ctx, f.jobID)
	if err != nil {
		return err
	}
	return printDetail(*j, f.format)
}

func printDetail(j workingnomads.Job, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(j)
	}

	fmt.Println(j.Title)
	fmt.Printf("Company: %s\n", j.Company)
	fmt.Printf("Category: %s\n", j.Category)
	if j.Location != "" {
		fmt.Printf("Location: %s\n", j.Location)
	}
	if len(j.Tags) > 0 {
		fmt.Printf("Tags: %v\n", j.Tags)
	}
	if !j.PostedAt.IsZero() {
		fmt.Printf("Posted: %s\n", j.PostedAt.Format(time.RFC3339))
	}
	fmt.Printf("URL: %s\n", j.URL)

	rendered, err := html2text.FromString(j.Description, html2text.Options{})
	if err != nil {
		rendered = j.Description
	}
	fmt.Printf("\nDescription:\n%s\n", rendered)
	return nil
}
