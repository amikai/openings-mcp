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

	leverprovider "github.com/amikai/openings-mcp/internal/provider/lever"
)

const leverAPIBaseURL = "https://api.lever.co"

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
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated Lever sites (company name and site slug)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
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
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
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
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
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

type searchResultJSON struct {
	Postings []postingJSON `json:"postings"`
}

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

func runSearch(ctx context.Context, f searchFlags) error {
	s, err := normalizeSite(f.site)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := leverprovider.NewClient(leverAPIBaseURL)
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

type getFlags struct {
	site      string
	timeout   time.Duration
	postingID string
	format    string
}

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

	client, err := leverprovider.NewClient(leverAPIBaseURL)
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

func setLocations(r *postingJSON, locations ...string) {
	if len(locations) == 0 {
		return
	}
	r.Location = locations[0]
	if len(locations) > 1 {
		r.Locations = locations
	}
}

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
