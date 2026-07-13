package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	lever "github.com/amikai/openings-mcp/internal/provider/lever"
)

// leverAPIBaseURL is the global-instance base URL. Every curated site in
// companies.yaml lives on the global instance, so the CLI never needs the
// EU server (https://api.eu.lever.co).
const leverAPIBaseURL = "https://api.lever.co"

func main() {
	rootFlags := ff.NewFlagSet("lever")
	var (
		site    = rootFlags.StringLong("site", "", "curated Lever site slug, e.g. leverdemo, palantir (see 'lever companies' for the full list)")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "lever",
		Usage: "lever --site SITE [FLAGS] <companies|search|get> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "lever companies [--format text|json]",
		ShortHelp: "list curated Lever sites (company name and site slug)",
		Flags:     companiesFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runCompanies(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		locations   = searchFS.StringListLong("location", "filter by location, repeatable (values OR'ed, case-sensitive)")
		commitments = searchFS.StringListLong("commitment", "filter by commitment, repeatable (values OR'ed, case-sensitive)")
		teams       = searchFS.StringListLong("team", "filter by team, repeatable (values OR'ed, case-sensitive)")
		departments = searchFS.StringListLong("department", "filter by department, repeatable (values OR'ed, case-sensitive)")
		level       = searchFS.StringLong("level", "", "filter by level")
		limit       = searchFS.IntLong("limit", 20, "page size")
		skip        = searchFS.IntLong("skip", 0, "number of postings to skip")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "lever --site SITE search [--location L ...] [--commitment C ...] [--team T ...] [--department D ...] [--level LVL] [--limit N] [--skip N] [--format text|json]",
		ShortHelp: "list postings for a site, with optional filters",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			return runSearch(ctx, searchFlags{
				site:        *site,
				timeout:     *timeout,
				locations:   *locations,
				commitments: *commitments,
				teams:       *teams,
				departments: *departments,
				level:       *level,
				limit:       *limit,
				skip:        *skip,
				format:      *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	getFS := ff.NewFlagSet("get").SetParent(rootFlags)
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "lever --site SITE get POSTING-ID [--format text|json]",
		ShortHelp: "fetch one posting by id",
		Flags:     getFS,
		Exec: func(ctx context.Context, args []string) error {
			var id string
			if len(args) > 0 {
				id = args[0]
			}
			return runGet(ctx, getFlags{site: *site, timeout: *timeout, postingID: id, format: *format})
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

// normalizeSite lowercases the --site value and requires it to be a
// curated site — same policy as cmd/workday's --tenant, even though
// Lever's URL shape wouldn't technically need the allowlist.
func normalizeSite(site string) (string, error) {
	if site == "" {
		return "", errors.New("--site is required")
	}
	s := strings.ToLower(site)
	if _, ok := lever.CompaniesBySite[s]; !ok {
		return "", fmt.Errorf("site %q not found; run 'lever companies' to see supported sites", site)
	}
	return s, nil
}

// runCompanies lists every curated Lever site embedded in the CLI
// (internal/provider/lever/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	cs := lever.Companies

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

	client, err := lever.NewClient(leverAPIBaseURL)
	if err != nil {
		return err
	}

	params := lever.ListPostingsParams{
		Site:       s,
		Mode:       lever.ListPostingsModeJSON,
		Skip:       lever.NewOptInt(f.skip),
		Limit:      lever.NewOptInt(f.limit),
		Location:   f.locations,
		Commitment: f.commitments,
		Team:       f.teams,
		Department: f.departments,
	}
	if f.level != "" {
		params.Level = lever.NewOptString(f.level)
	}

	postings, err := client.ListPostings(ctx, params)
	if err != nil {
		return err
	}

	results := make([]postingJSON, len(postings))
	for i, p := range postings {
		results[i] = toPostingJSON(p)
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

	client, err := lever.NewClient(leverAPIBaseURL)
	if err != nil {
		return err
	}

	p, err := client.GetPosting(ctx, lever.GetPostingParams{Site: s, PostingId: f.postingID})
	if err != nil {
		return err
	}

	r := toPostingJSON(*p)

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

func toPostingJSON(p lever.Posting) postingJSON {
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
func postingLocations(p lever.Posting) []string {
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
// mirrors cmd/workday's setLocations.
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
