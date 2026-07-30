// Package lever implements the "openings-mcp lever" debug CLI, for manual
// checks against the live surface that internal/provider/lever documents.
package lever

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	leverprovider "github.com/amikai/openings-mcp/internal/provider/lever"
)

type options struct {
	site    string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for lever.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "lever",
		Short:        "lever --site SITE [FLAGS] <companies|search|get> [FLAGS]",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.site, "site", "", "curated Lever site slug, e.g. leverdemo, palantir (see 'lever companies' for the full list)")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	clihelp.FormatVar(rootCmd.PersistentFlags(), &opts.format)

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated Lever sites (company name and site slug)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			return runCompanies(opts.format)
		},
	}

	var (
		locations   []string
		commitments []string
		teams       []string
		departments []string
		level       string
		limit       int
		skip        int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "list postings for a site, with optional filters",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			return runSearch(cmd.Context(), searchFlags{
				site:        opts.site,
				timeout:     opts.timeout,
				locations:   locations,
				commitments: commitments,
				teams:       teams,
				departments: departments,
				level:       level,
				limit:       limit,
				skip:        skip,
				format:      opts.format,
			})
		},
	}
	searchCmd.Flags().StringArrayVar(&locations, "location", nil, "filter by location, repeatable (values OR'ed, case-sensitive)")
	searchCmd.Flags().StringArrayVar(&commitments, "commitment", nil, "filter by commitment, repeatable (values OR'ed, case-sensitive)")
	searchCmd.Flags().StringArrayVar(&teams, "team", nil, "filter by team, repeatable (values OR'ed, case-sensitive)")
	searchCmd.Flags().StringArrayVar(&departments, "department", nil, "filter by department, repeatable (values OR'ed, case-sensitive)")
	searchCmd.Flags().StringVar(&level, "level", "", "filter by level")
	searchCmd.Flags().IntVar(&limit, "limit", 20, "page size")
	searchCmd.Flags().IntVar(&skip, "skip", 0, "number of postings to skip")

	getCmd := &cobra.Command{
		Use:          "get [POSTING-ID]",
		Short:        "fetch one posting by id",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var id string
			if len(args) > 0 {
				id = args[0]
			}
			return runGet(cmd.Context(), getFlags{
				site:      opts.site,
				timeout:   opts.timeout,
				postingID: id,
				format:    opts.format,
			})
		},
	}

	rootCmd.AddCommand(companiesCmd, searchCmd, getCmd)
	return rootCmd
}

// normalizeSite lowercases the --site value and requires it to be a
// curated site — same policy as the workday CLI's --tenant, even though
// Lever's URL shape wouldn't technically need the allowlist.
func normalizeSite(site string) (string, error) {
	if site == "" {
		return "", errors.New("--site is required")
	}
	s := strings.ToLower(site)
	if _, ok := leverprovider.CompaniesBySite[s]; !ok {
		return "", fmt.Errorf("site %q not found; run 'lever companies' to see supported sites", site)
	}
	return s, nil
}

// runCompanies lists every curated Lever site embedded in the CLI
// (internal/provider/lever/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	cs := leverprovider.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.Site)
	}
	return nil
}

// searchResultJSON wraps the postings array so future side-channel fields
// (e.g. a total count, if Lever ever exposes one) don't break consumers.
type searchResultJSON struct {
	Postings []postingJSON `json:"postings"`
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	site        string
	timeout     time.Duration
	locations   []string
	commitments []string
	teams       []string
	departments []string
	level       string
	limit       int
	skip        int
	format      string
}

// runSearch fetches one page of postings with the given filters. The list
// response already carries full posting content, so there are no
// per-result detail fetches — one API call per invocation.
func runSearch(ctx context.Context, f searchFlags) error {
	s, err := normalizeSite(f.site)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := leverprovider.NewClient(leverprovider.DefaultBaseURL)
	if err != nil {
		return err
	}

	params := leverprovider.ListPostingsParams{
		Site:       s,
		Mode:       leverprovider.ListPostingsModeJSON,
		Skip:       leverprovider.NewOptInt(f.skip),
		Limit:      leverprovider.NewOptInt(f.limit),
		Location:   f.locations,
		Commitment: f.commitments,
		Team:       f.teams,
		Department: f.departments,
	}
	if f.level != "" {
		params.Level = leverprovider.NewOptString(f.level)
	}

	postings, err := client.ListPostings(ctx, params)
	if err != nil {
		return err
	}

	results := make([]postingJSON, len(postings))
	for i := range postings {
		results[i] = toPostingJSON(&postings[i])
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Postings: results})
	}

	fmt.Printf("Lever Jobs Report (site: %s)\n", s)
	fmt.Printf("Showing %d postings\n\n", len(results))
	for i, r := range results {
		printPosting(i+1, r)
		fmt.Println()
	}
	return nil
}

// getFlags carries the parsed "get" subcommand flags into runGet.
type getFlags struct {
	site      string
	timeout   time.Duration
	postingID string
	format    string
}

// runGet fetches one posting by id and renders it unnumbered.
func runGet(ctx context.Context, f getFlags) error {
	s, err := normalizeSite(f.site)
	if err != nil {
		return err
	}
	if f.postingID == "" {
		return errors.New("a posting id argument is required (take it from a search result's id)")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := leverprovider.NewClient(leverprovider.DefaultBaseURL)
	if err != nil {
		return err
	}

	p, err := client.GetPosting(ctx, leverprovider.GetPostingParams{Site: s, PostingId: f.postingID})
	if err != nil {
		return err
	}

	r := toPostingJSON(p)

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}

	printPosting(0, r)
	return nil
}

// postingJSON is the --format json shape for one posting, and the input
// to text rendering: a flat, stable projection of the generated
// lever.Posting so the CLI's output doesn't change shape when the spec's
// generated types do.
type postingJSON struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	URL         string   `json:"url,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"` // 2006-01-02 (UTC)
	Location    string   `json:"location,omitempty"`
	Locations   []string `json:"locations,omitempty"`
	Team        string   `json:"team,omitempty"`
	Commitment  string   `json:"commitment,omitempty"`
	Description string   `json:"description,omitempty"`
}

func toPostingJSON(p *leverprovider.Posting) postingJSON {
	cats := p.Categories.Value
	r := postingJSON{
		ID:          p.ID,
		Title:       p.Text.Value,
		URL:         p.HostedUrl.Value,
		Team:        cats.Team.Value,
		Commitment:  cats.Commitment.Value,
		Description: p.DescriptionPlain.Value,
	}
	if v, ok := p.CreatedAt.Get(); ok {
		r.CreatedAt = time.UnixMilli(v).UTC().Format("2006-01-02")
	}
	setLocations(&r, postingLocations(p)...)
	return r
}

// postingLocations prefers the full allLocations list; the primary
// location is its first entry when present, so the fallback only matters
// for postings that carry a single location field.
func postingLocations(p *leverprovider.Posting) []string {
	cats := p.Categories.Value
	if len(cats.AllLocations) > 0 {
		return cats.AllLocations
	}
	if cats.Location.Set {
		return []string{cats.Location.Value}
	}
	return nil
}

// setLocations fills both the singular Location (first entry, for quick
// access) and the full Locations array (only when there's more than one,
// to avoid a redundant one-element array alongside the singular field) —
// mirrors internal/cli/provider/workday's setLocations.
func setLocations(r *postingJSON, locations ...string) {
	if len(locations) == 0 {
		return
	}
	r.Location = locations[0]
	if len(locations) > 1 {
		r.Locations = locations
	}
}

// printPosting renders one posting as text. index > 0 numbers the entry
// (search results); index 0 prints it unnumbered (get).
func printPosting(index int, p postingJSON) {
	if index > 0 {
		fmt.Printf("%d. %s\n", index, p.Title)
	} else {
		fmt.Println(p.Title)
	}
	if p.CreatedAt != "" {
		fmt.Printf("Created: %s\n", p.CreatedAt)
	}
	if p.URL != "" {
		fmt.Printf("URL: %s\n", p.URL)
	}
	if len(p.Locations) > 0 {
		fmt.Println("Locations:")
		for _, l := range p.Locations {
			fmt.Printf("  - %s\n", l)
		}
	} else if p.Location != "" {
		fmt.Printf("Location: %s\n", p.Location)
	}
	if p.Team != "" {
		fmt.Printf("Team: %s\n", p.Team)
	}
	if p.Commitment != "" {
		fmt.Printf("Commitment: %s\n", p.Commitment)
	}
	if p.Description != "" {
		fmt.Printf("Description:\n%s\n", p.Description)
	}
}
