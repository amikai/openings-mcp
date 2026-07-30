package weworkremotely

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/weworkremotely"
)

type rootOptions struct {
	baseURL string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for weworkremotely.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "weworkremotely",
		Short:        "Search We Work Remotely jobs and view details",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", weworkremotely.DefaultBaseURL, "We Work Remotely base URL")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	var (
		searchKeyword  string
		searchCategory string
		searchCompany  string
		searchJobType  string
		searchRegion   string
		searchLimit    int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "fetch one or all category feeds and filter client-side",
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
				opts: weworkremotely.FilterOptions{
					Keyword:  searchKeyword,
					Category: searchCategory,
					Company:  searchCompany,
					Type:     searchJobType,
					Region:   searchRegion,
				},
				limit: searchLimit,
			})
		},
	}
	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "case-insensitive substring over title, skills, and description")
	searchCmd.Flags().StringVar(&searchCategory, "category", "", `category display name, e.g. "Full-Stack Programming" (see 'weworkremotely categories'); a recognized value fetches only that feed`)
	searchCmd.Flags().StringVar(&searchCompany, "company", "", "case-insensitive company name substring")
	searchCmd.Flags().StringVar(&searchJobType, "type", "", "exact job type, e.g. Full-Time or Contract")
	searchCmd.Flags().StringVar(&searchRegion, "region", "", "case-insensitive substring over region, country, and state")
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
	detailCmd.Flags().StringVar(&detailJobID, "id", "", "job ID (URL slug) from a search result")

	categoriesCmd := &cobra.Command{
		Use:          "categories",
		Short:        "list the fixed set of category feeds (name and slug)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, c := range weworkremotely.Categories {
				fmt.Printf("%s (%s)\n", c.Name, c.Slug)
			}
			return nil
		},
	}

	cmd.AddCommand(searchCmd, detailCmd, categoriesCmd)
	return cmd
}

func newClient(baseURL string) *weworkremotely.Client {
	return weworkremotely.NewClient(baseURL, nil)
}

type jobSummaryJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Company  string `json:"company"`
	Category string `json:"category"`
	Type     string `json:"type,omitempty"`
	Region   string `json:"region,omitempty"`
	PostedAt string `json:"postedAt,omitempty"`
	URL      string `json:"url"`
}

func summarize(j weworkremotely.Job) jobSummaryJSON {
	return jobSummaryJSON{
		ID:       j.ID,
		Title:    j.Title,
		Company:  j.Company,
		Category: j.Category,
		Type:     j.Type,
		Region:   j.Region,
		PostedAt: j.PostedAt.Format(time.RFC3339),
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
	opts    weworkremotely.FilterOptions
	limit   int
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	matched, err := newClient(f.baseURL).Search(ctx, f.opts)
	if len(matched) == 0 && err != nil {
		return err
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: some category feeds failed, results may be incomplete: %v\n", err)
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
		fmt.Printf("Category: %s (%s)\n", s.Category, s.Type)
		if s.Region != "" {
			fmt.Printf("Region: %s\n", s.Region)
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

func printDetail(j weworkremotely.Job, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(j)
	}

	fmt.Println(j.Title)
	fmt.Printf("Company: %s\n", j.Company)
	fmt.Printf("Category: %s (%s)\n", j.Category, j.Type)
	if j.Region != "" {
		fmt.Printf("Region: %s\n", j.Region)
	}
	if j.Country != "" {
		fmt.Printf("Country: %s\n", j.Country)
	}
	if j.State != "" {
		fmt.Printf("State: %s\n", j.State)
	}
	if j.Skills != "" {
		fmt.Printf("Skills: %s\n", j.Skills)
	}
	fmt.Printf("Posted: %s\n", j.PostedAt.Format(time.RFC3339))
	if !j.ExpiresAt.IsZero() {
		fmt.Printf("Expires: %s\n", j.ExpiresAt.Format(time.RFC3339))
	}
	fmt.Printf("URL: %s\n", j.URL)

	rendered, err := html2text.FromString(j.Description, html2text.Options{})
	if err != nil {
		rendered = j.Description
	}
	fmt.Printf("\nDescription:\n%s\n", rendered)
	return nil
}
