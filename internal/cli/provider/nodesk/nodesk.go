package nodesk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	nodeskprovider "github.com/amikai/openings-mcp/internal/provider/nodesk"
)

type options struct {
	algoliaBaseURL string
	siteBaseURL    string
	timeout        time.Duration
	format         string
}

func (o options) newClient() *nodeskprovider.Client {
	return nodeskprovider.NewClient(o.algoliaBaseURL, o.siteBaseURL, nil)
}

// NewCommand returns a cobra.Command for nodesk.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "nodesk",
		Short:        "Search NoDesk jobs, view job details, and list search facets",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.algoliaBaseURL, "algolia-base-url", nodeskprovider.DefaultAlgoliaBaseURL, "Algolia DSN base URL")
	rootCmd.PersistentFlags().StringVar(&opts.siteBaseURL, "site-base-url", nodeskprovider.DefaultSiteBaseURL, "NoDesk site base URL")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	var (
		searchQuery       string
		searchPage        int
		searchHitsPerPage int
		searchFilter      string
		searchRegion      string
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "server-side search over the jobPosts index",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if searchPage < 0 {
				return fmt.Errorf("--page must be >= 0, got %d", searchPage)
			}
			if searchHitsPerPage < 1 {
				return fmt.Errorf("--hits-per-page must be >= 1, got %d", searchHitsPerPage)
			}
			return runSearch(cmd.Context(), searchFlags{
				options: *opts,
				opts: nodeskprovider.SearchOptions{
					Query:       searchQuery,
					Page:        searchPage,
					HitsPerPage: searchHitsPerPage,
					Filter:      searchFilter,
					Region:      searchRegion,
				},
			})
		},
	}

	searchCmd.Flags().StringVar(&searchQuery, "query", "", "full-text search; empty lists the whole board")
	searchCmd.Flags().IntVar(&searchPage, "page", 0, "zero-based result page")
	searchCmd.Flags().IntVar(&searchHitsPerPage, "hits-per-page", 20, "results per page (the index clamps values above 100)")
	searchCmd.Flags().StringVar(&searchFilter, "filter", "", `category path from 'nodesk facets', e.g. "remote-jobs/engineering"`)
	searchCmd.Flags().StringVar(&searchRegion, "region", "", `region label from 'nodesk facets', e.g. "Remote - Europe"`)

	var detailJobID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "fetch one job page and print its JSON-LD detail",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if detailJobID == "" {
				return errors.New("--id is required (take it from a search result's ID)")
			}
			return runDetail(cmd.Context(), *opts, detailJobID)
		},
	}

	detailCmd.Flags().StringVar(&detailJobID, "id", "", "job ID (permalink slug) from a search result")

	facetsCmd := &cobra.Command{
		Use:          "facets",
		Short:        "list every live --filter path and --region label with its job count",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runFacets(cmd.Context(), *opts)
		},
	}

	rootCmd.AddCommand(searchCmd, detailCmd, facetsCmd)
	return rootCmd
}

type searchFlags struct {
	options
	opts nodeskprovider.SearchOptions
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	res, err := f.newClient().Search(ctx, f.opts)
	if err != nil {
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Printf("Matched %d jobs across %d pages; showing page %d (%d jobs)\n\n",
		res.NbHits, res.NbPages, res.Page, len(res.Jobs))
	for i, j := range res.Jobs {
		fmt.Printf("%d. %s\n", i+1, j.Title)
		fmt.Printf("Company: %s\n", j.Company)
		if j.Role != "" {
			fmt.Printf("Role: %s\n", j.Role)
		}
		if len(j.Types) > 0 {
			fmt.Printf("Types: %s\n", strings.Join(j.Types, ", "))
		}
		if len(j.Regions) > 0 {
			fmt.Printf("Regions: %s\n", strings.Join(j.Regions, ", "))
		}
		if len(j.Keywords) > 0 {
			fmt.Printf("Keywords: %s\n", strings.Join(j.Keywords, ", "))
		}
		if j.BaseSalary != "" {
			fmt.Printf("Salary: %s\n", j.BaseSalary)
		}
		if !j.PublishedAt.IsZero() {
			fmt.Printf("Published: %s (%s)\n", j.PublishedAt.Format("2006-01-02"), j.DateLabel)
		}
		fmt.Printf("ID: %s\n", j.ID)
		fmt.Println()
	}
	return nil
}

func runDetail(ctx context.Context, f options, jobID string) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	d, err := f.newClient().Detail(ctx, jobID)
	if err != nil {
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Title)
	fmt.Printf("Company: %s\n", d.Company)
	for _, l := range d.CompanyLinks {
		fmt.Printf("Company link: %s\n", l)
	}
	if len(d.Types) > 0 {
		fmt.Printf("Types: %s\n", strings.Join(d.Types, ", "))
	}
	if len(d.Locations) > 0 {
		fmt.Printf("Locations: %s (JSON-LD; see search Regions for the reliable signal)\n", strings.Join(d.Locations, ", "))
	}
	if d.Salary != nil {
		fmt.Printf("Salary: %s %.0f – %.0f per %s\n", d.Salary.Currency, d.Salary.Min, d.Salary.Max, d.Salary.Unit)
	}
	fmt.Printf("Posted: %s\n", d.DatePosted.Format("2006-01-02"))
	if !d.ValidThrough.IsZero() {
		fmt.Printf("Valid through: %s\n", d.ValidThrough.Format("2006-01-02"))
	}
	fmt.Printf("URL: %s\n", d.URL)
	if d.ApplyURL != "" {
		fmt.Printf("Apply: %s\n", d.ApplyURL)
	}

	rendered, err := html2text.FromString(d.DescriptionHTML, html2text.Options{})
	if err != nil {
		rendered = d.DescriptionHTML
	}
	fmt.Printf("\nDescription:\n%s\n", rendered)
	return nil
}

func runFacets(ctx context.Context, f options) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	facets, err := f.newClient().Facets(ctx)
	if err != nil {
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(facets)
	}

	fmt.Println("Filters (searchFilter):")
	printCounts(facets.SearchFilters)
	fmt.Println("\nRegions (applicantLocationRegions):")
	printCounts(facets.Regions)
	return nil
}

func printCounts(counts map[string]int) {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		fmt.Printf("%5d  %s\n", counts[k], k)
	}
}
