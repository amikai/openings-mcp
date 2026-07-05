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
	"golang.org/x/sync/errgroup"

	workday "github.com/amikai/openings-mcp/internal/provider/workday"
)

// maxConcurrentDetailFetches caps how many job-detail requests runSearch
// fires at once — fetchJobResult never returns an error, so the only reason
// to bound it is being a considerate caller of someone else's career site
// rather than firing --limit-many requests in a single burst.
const maxConcurrentDetailFetches = 5

func main() {
	rootFlags := ff.NewFlagSet("workday")
	var (
		tenant  = rootFlags.StringLong("tenant", "", "confirmed Workday tenant slug, e.g. 3m, att (see 'workday companies' for the full list)")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "workday",
		Usage: "workday --tenant TENANT [FLAGS] <companies|facets|search> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "workday companies [--format text|json]",
		ShortHelp: "list confirmed Workday tenants (company name and tenant slug)",
		Flags:     companiesFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runCompanies(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	facetsFlags := ff.NewFlagSet("facets").SetParent(rootFlags)
	var (
		facetsSearchText = facetsFlags.StringLong("search-text", "", "free-text keyword search to narrow the facet tree")
		facetsFacetArgs  = facetsFlags.StringListLong("facet", "facet filter as name=id, repeatable")
	)
	facetsCmd := &ff.Command{
		Name:      "facets",
		Usage:     "workday --tenant TENANT facets [--search-text TEXT] [--facet name=id ...] [--format text|json]",
		ShortHelp: "discover a tenant's current facet tree (categories, locations, ...)",
		Flags:     facetsFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runFacets(ctx, *tenant, *timeout, *facetsSearchText, *facetsFacetArgs, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, facetsCmd)

	searchFlags := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		searchText      = searchFlags.StringLong("search-text", "", "free-text keyword search")
		limit           = searchFlags.IntLong("limit", 20, "page size")
		offset          = searchFlags.IntLong("offset", 0, "zero-based result offset")
		searchFacetArgs = searchFlags.StringListLong("facet", "facet filter as name=id, repeatable")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "workday --tenant TENANT search [--search-text TEXT] [--limit N] [--offset N] [--facet name=id ...] [--format text|json]",
		ShortHelp: "search jobs and fetch full detail for each result",
		Flags:     searchFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runSearch(ctx, *tenant, *timeout, *searchText, *limit, *offset, *searchFacetArgs, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, facets, or search) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// parseFacets turns repeated "--facet name=id" flag values into an
// AppliedFacets map. Repeating the same name appends to that facet's id
// list (OR'd within a facet); different names key different facets (AND'd
// together) — matches AppliedFacets's map[string][]string shape 1:1.
func parseFacets(raw []string) (workday.AppliedFacets, error) {
	af := workday.AppliedFacets{}
	for _, f := range raw {
		name, id, ok := strings.Cut(f, "=")
		if !ok || name == "" || id == "" {
			return nil, fmt.Errorf("invalid --facet %q, want name=id", f)
		}
		af[name] = append(af[name], id)
	}
	return af, nil
}

// runCompanies lists every confirmed Workday tenant embedded in the CLI
// (internal/provider/workday/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	cs := workday.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.Tenant)
	}
	return nil
}

// runFacets discovers a tenant's current facet tree via a search whose only
// job is to read back JobsResponse.Facets — Limit is 1 because the actual
// jobPostings aren't used here (see openapi.yaml's note that every /jobs
// response, filtered or not, carries the full current facet tree).
func runFacets(ctx context.Context, tenant string, timeout time.Duration, searchText string, facetArgs []string, format string) error {
	if tenant == "" {
		return fmt.Errorf("--tenant is required")
	}
	company, ok := workday.CompaniesByTenant[strings.ToLower(tenant)]
	if !ok {
		return fmt.Errorf("tenant %q not found; run 'workday companies' to see supported tenants", tenant)
	}

	appliedFacets, err := parseFacets(facetArgs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := workday.NewClient(company.BaseURL())
	if err != nil {
		return err
	}

	search, err := client.SearchJobs(ctx, &workday.JobsRequest{
		AppliedFacets: appliedFacets,
		Limit:         1,
		Offset:        0,
		SearchText:    searchText,
	})
	if err != nil {
		return err
	}

	// Get returns a nil slice when the tenant omitted facets or sent null.
	facets, _ := search.Facets.Get()

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(facets)
	}

	for _, node := range facets {
		printFacetNode(node, 0)
	}
	return nil
}

// printFacetNode recursively prints one facet tree node. A node with a
// facetParameter is a group (printed as "facetParameter (descriptor)",
// descriptor omitted when unset — some top-level groups like
// locationMainGroup have none); a node without one is a leaf, printed as
// "descriptor  id=...  count=...". Grouping keys on facetParameter rather
// than len(Values) so a group whose Values are momentarily empty isn't
// mis-rendered as a leaf.
func printFacetNode(node workday.FacetNode, depth int) {
	indent := strings.Repeat("  ", depth)
	if node.FacetParameter.Set {
		label := node.FacetParameter.Value
		if node.Descriptor.Set {
			label = fmt.Sprintf("%s (%s)", label, node.Descriptor.Value)
		}
		fmt.Println(indent + label)
		for _, child := range node.Values {
			printFacetNode(child, depth+1)
		}
		return
	}
	fmt.Printf("%s%s  id=%s  count=%d\n", indent, node.Descriptor.Value, node.ID.Value, node.Count.Value)
}

// jobResultJSON is the --format json shape for one search result: the
// search summary merged with its fetched detail (or, if the detail fetch
// failed, a fallback link plus Error instead of Description).
type jobResultJSON struct {
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Location    string   `json:"location,omitempty"`
	Locations   []string `json:"locations,omitempty"`
	PostedOn    string   `json:"postedOn,omitempty"`
	Description string   `json:"description,omitempty"`
	JobReqId    string   `json:"jobReqId,omitempty"`
	Error       string   `json:"error,omitempty"`
}

type searchResultJSON struct {
	Total int             `json:"total"`
	Jobs  []jobResultJSON `json:"jobs"`
}

// runSearch searches jobs, then fetches full detail for every result
// (mirrors cmd/nvidia's report behavior: a posting with no ExternalPath is
// listed with a "no detail available" note rather than silently dropped, so
// "showing N" always matches the page's posting count) — one page per
// invocation, no auto-pagination.
func runSearch(ctx context.Context, tenant string, timeout time.Duration, searchText string, limit, offset int, facetArgs []string, format string) error {
	if tenant == "" {
		return fmt.Errorf("--tenant is required")
	}
	company, ok := workday.CompaniesByTenant[strings.ToLower(tenant)]
	if !ok {
		return fmt.Errorf("tenant %q not found; run 'workday companies' to see supported tenants", tenant)
	}

	appliedFacets, err := parseFacets(facetArgs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	baseURL := company.BaseURL()
	client, err := workday.NewClient(baseURL)
	if err != nil {
		return err
	}

	search, err := client.SearchJobs(ctx, &workday.JobsRequest{
		AppliedFacets: appliedFacets,
		Limit:         limit,
		Offset:        offset,
		SearchText:    searchText,
	})
	if err != nil {
		return err
	}

	results := make([]jobResultJSON, len(search.JobPostings))
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentDetailFetches)
	for i, job := range search.JobPostings {
		g.Go(func() error {
			results[i] = fetchJobResult(gCtx, client, baseURL, job)
			return nil
		})
	}
	_ = g.Wait() // fetchJobResult never returns an error; failures land in each result's Error field instead.

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: search.Total, Jobs: results})
	}

	fmt.Printf("Workday Jobs Report\n")
	fmt.Printf("Found %d jobs; showing %d\n\n", search.Total, len(results))
	for i, r := range results {
		fmt.Printf("%d. %s\n", i+1, r.Title)
		if r.PostedOn != "" {
			fmt.Printf("Posted: %s\n", r.PostedOn)
		}
		if r.URL != "" {
			fmt.Printf("URL: %s\n", r.URL)
		}
		printResultLocations(r)
		switch {
		case r.Error != "":
			fmt.Printf("(job detail unavailable: %s)\n", r.Error)
		case r.Description != "":
			fmt.Printf("Description:\n%s\n", r.Description)
		}
		fmt.Println()
	}
	return nil
}

func printResultLocations(r jobResultJSON) {
	if len(r.Locations) > 0 {
		fmt.Println("Locations:")
		for _, l := range r.Locations {
			fmt.Printf("  - %s\n", l)
		}
		return
	}
	if r.Location != "" {
		fmt.Printf("Location: %s\n", r.Location)
	}
}

// fetchJobResult fetches full detail for one job summary. A detail-fetch
// failure is non-fatal: it falls back to a derived public site URL and the
// summary's aggregate LocationsText, and records the error instead of a
// description, so one bad job doesn't abort the whole search — mirrors
// cmd/nvidia's existing per-job fallback behavior. A summary with no
// ExternalPath (an incomplete/transient Workday posting) can't be fetched at
// all, so it's returned with a "no detail available" note rather than dropped.
func fetchJobResult(ctx context.Context, client *workday.Client, baseURL string, job workday.JobSummary) jobResultJSON {
	r := jobResultJSON{Title: job.Title.Value, PostedOn: job.PostedOn.Value}

	if job.ExternalPath.Value == "" {
		r.Error = "listing has no externalPath"
		setLocations(&r, job.LocationsText.Value)
		return r
	}

	location, titleSlug, ok := workday.SplitExternalPath(job.ExternalPath.Value)
	if !ok {
		r.Error = fmt.Sprintf("could not split externalPath %q", job.ExternalPath.Value)
		r.URL = fallbackURL(baseURL, job.ExternalPath.Value)
		setLocations(&r, job.LocationsText.Value)
		return r
	}

	detail, err := client.GetJobDetail(ctx, workday.GetJobDetailParams{Location: location, TitleSlug: titleSlug})
	if err != nil {
		r.Error = err.Error()
		r.URL = fallbackURL(baseURL, job.ExternalPath.Value)
		setLocations(&r, job.LocationsText.Value)
		return r
	}

	info := detail.JobPostingInfo
	// Overwrite the summary's title/postedOn only when the detail actually
	// carries a value — a detail response that omits postedOn (optional) or
	// returns an empty title must not blank out the good summary value.
	if info.Title != "" {
		r.Title = info.Title
	}
	if info.PostedOn.Set {
		r.PostedOn = info.PostedOn.Value
	}
	r.JobReqId = info.JobReqId.Value
	if info.ExternalUrl.Set {
		r.URL = info.ExternalUrl.Value
	} else {
		r.URL = fallbackURL(baseURL, job.ExternalPath.Value)
	}

	itemized := make([]string, 0, 1+len(info.AdditionalLocations))
	if info.Location.Set {
		itemized = append(itemized, info.Location.Value)
	}
	itemized = append(itemized, info.AdditionalLocations...)
	setLocations(&r, itemized...)

	description, err := html2text.FromString(info.JobDescription, html2text.Options{})
	if err != nil {
		description = info.JobDescription
	}
	r.Description = description

	return r
}

// setLocations fills both the singular Location (first entry, for quick
// access) and the full Locations array (only when there's more than one, to
// avoid a redundant one-element array alongside the singular field) —
// mirrors cmd/nvidia's printLocations singular/plural distinction.
func setLocations(r *jobResultJSON, locations ...string) {
	if len(locations) == 0 {
		return
	}
	r.Location = locations[0]
	if len(locations) > 1 {
		r.Locations = locations
	}
}

// fallbackURL builds a best-effort public job link when the detail fetch
// (which carries the authoritative externalUrl) fails. Falls back to
// externalPath alone if the base URL can't be resolved to a public site
// origin either.
func fallbackURL(baseURL, externalPath string) string {
	site, err := workday.PublicSiteURL(baseURL)
	if err != nil {
		return externalPath
	}
	// externalPath usually starts with "/", but SplitExternalPath treats a
	// missing leading slash as just another malformed shape that lands here —
	// don't let it glue the site segment and location together.
	if !strings.HasPrefix(externalPath, "/") {
		externalPath = "/" + externalPath
	}
	return site + externalPath
}
