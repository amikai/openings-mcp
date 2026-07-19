// Command nodesk is a debug CLI for NoDesk's public Algolia search index
// and job detail pages.
//
//	go run ./cmd/nodesk search --query golang
//	go run ./cmd/nodesk search --filter remote-jobs/engineering --region "Remote - Europe"
//	go run ./cmd/nodesk detail --id sticker-mule-software-engineer
//	go run ./cmd/nodesk facets
//
// Search is server-side (full-text query, zero-based --page, facet
// filters); detail fetches the job page and parses its JobPosting
// JSON-LD block. facets lists every live --filter path and --region
// label with its job count.
package main

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
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/nodesk"
)

func main() {
	rootFlags := ff.NewFlagSet("nodesk")
	var (
		algoliaBaseURL = rootFlags.StringLong("algolia-base-url", nodesk.DefaultAlgoliaBaseURL, "Algolia DSN base URL")
		siteBaseURL    = rootFlags.StringLong("site-base-url", nodesk.DefaultSiteBaseURL, "NoDesk site base URL")
		timeout        = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format         = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "nodesk",
		Usage: "nodesk [FLAGS] <search|detail|facets> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		query       = searchFS.StringLong("query", "", "full-text search; empty lists the whole board")
		page        = searchFS.IntLong("page", 0, "zero-based result page")
		hitsPerPage = searchFS.IntLong("hits-per-page", 20, "results per page (the index clamps values above 100)")
		filter      = searchFS.StringLong("filter", "", `category path from 'nodesk facets', e.g. "remote-jobs/engineering"`)
		region      = searchFS.StringLong("region", "", `region label from 'nodesk facets', e.g. "Remote - Europe"`)
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "nodesk search [--query TEXT] [--page N] [--hits-per-page N] [--filter PATH] [--region LABEL] [--format text|json]",
		ShortHelp: "server-side search over the jobPosts index",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			if *page < 0 {
				return fmt.Errorf("--page must be >= 0, got %d", *page)
			}
			if *hitsPerPage < 1 {
				return fmt.Errorf("--hits-per-page must be >= 1, got %d", *hitsPerPage)
			}
			return runSearch(ctx, searchFlags{
				clientFlags: clientFlags{algoliaBaseURL: *algoliaBaseURL, siteBaseURL: *siteBaseURL, timeout: *timeout, format: *format},
				opts: nodesk.SearchOptions{
					Query:       *query,
					Page:        *page,
					HitsPerPage: *hitsPerPage,
					Filter:      *filter,
					Region:      *region,
				},
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("id", "", "job ID (permalink slug) from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "nodesk detail --id JOB-ID [--format text|json]",
		ShortHelp: "fetch one job page and print its JSON-LD detail",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			if *jobID == "" {
				return errors.New("--id is required (take it from a search result's ID)")
			}
			return runDetail(ctx, clientFlags{algoliaBaseURL: *algoliaBaseURL, siteBaseURL: *siteBaseURL, timeout: *timeout, format: *format}, *jobID)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

	facetsFS := ff.NewFlagSet("facets").SetParent(rootFlags)
	facetsCmd := &ff.Command{
		Name:      "facets",
		Usage:     "nodesk facets [--format text|json]",
		ShortHelp: "list every live --filter path and --region label with its job count",
		Flags:     facetsFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("facets takes no positional arguments, got %v", args)
			}
			return runFacets(ctx, clientFlags{algoliaBaseURL: *algoliaBaseURL, siteBaseURL: *siteBaseURL, timeout: *timeout, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, facetsCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, detail, or facets) is required")
		os.Exit(1)
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// clientFlags carries the root flags every subcommand needs.
type clientFlags struct {
	algoliaBaseURL string
	siteBaseURL    string
	timeout        time.Duration
	format         string
}

func (f clientFlags) newClient() *nodesk.Client {
	return nodesk.NewClient(f.algoliaBaseURL, f.siteBaseURL, nil)
}

type searchFlags struct {
	clientFlags
	opts nodesk.SearchOptions
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

func runDetail(ctx context.Context, f clientFlags, jobID string) error {
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

func runFacets(ctx context.Context, f clientFlags) error {
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

// printCounts lists a facet's values by descending job count, ties
// alphabetically.
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
