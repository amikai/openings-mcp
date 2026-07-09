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

// maxConcurrentDetailFetches limits the burst sent to a third-party career site.
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

// parseFacets converts "--facet name=id" values to AppliedFacets. Repeated
// names are OR'd; different names are AND'd by the upstream API.
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

// runCompanies prints the embedded Workday tenant roster without a network call.
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

// runFacets reads the tenant's current facet tree from a one-result search;
// every /jobs response carries the tree even when its postings are unused.
func runFacets(ctx context.Context, tenant string, timeout time.Duration, searchText string, facetArgs []string, format string) error {
	if tenant == "" {
		return fmt.Errorf("--tenant is required")
	}
	_, ok := workday.CompaniesByTenant[strings.ToLower(tenant)]
	if !ok {
		return fmt.Errorf("tenant %q not found; run 'workday companies' to see supported tenants", tenant)
	}

	appliedFacets, err := parseFacets(facetArgs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := workday.NewTenantClient()
	if err != nil {
		return err
	}

	search, err := client.JobsByTenant(ctx, tenant, &workday.JobsRequest{
		AppliedFacets: appliedFacets,
		Limit:         1,
		Offset:        0,
		SearchText:    searchText,
	})
	if err != nil {
		return err
	}

	// A tenant may omit facets or send them as null.
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

// printFacetNode renders groups as "facetParameter (descriptor)" and leaves as
// "descriptor  id=...  count=...". It uses facetParameter rather than Values
// length to distinguish groups that temporarily have no values.
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

// jobResultJSON is the search summary merged with detail, or a fallback link
// and error when detail fetching fails.
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

// runSearch fetches detail for every result on one page. Incomplete postings
// remain visible with a fallback note instead of being dropped.
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
	client, err := workday.NewTenantClient()
	if err != nil {
		return err
	}

	search, err := client.JobsByTenant(ctx, tenant, &workday.JobsRequest{
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
			results[i] = fetchJobResult(gCtx, client, tenant, baseURL, job)
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

// fetchJobResult makes detail failures non-fatal: it returns a best-effort URL,
// aggregate location text, and the error. A posting without ExternalPath is
// returned with a no-detail note.
func fetchJobResult(ctx context.Context, client *workday.TenantClient, tenant, baseURL string, job workday.JobSummary) jobResultJSON {
	r := jobResultJSON{Title: job.Title.Value, PostedOn: job.PostedOn.Value}

	if job.ExternalPath.Value == "" {
		r.Error = "listing has no externalPath"
		setLocations(&r, job.LocationsText.Value)
		return r
	}

	location, titleSlug, ok := workday.JobDetailKeyFromPath(job.ExternalPath.Value)
	if !ok {
		r.Error = fmt.Sprintf("could not split externalPath %q", job.ExternalPath.Value)
		r.URL = fallbackURL(baseURL, job.ExternalPath.Value)
		setLocations(&r, job.LocationsText.Value)
		return r
	}

	detail, err := client.JobDetailByTenant(ctx, tenant, location, titleSlug)
	if err != nil {
		r.Error = err.Error()
		r.URL = fallbackURL(baseURL, job.ExternalPath.Value)
		setLocations(&r, job.LocationsText.Value)
		return r
	}

	info := detail.JobPostingInfo
	// Optional or empty detail fields must not erase good summary values.
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

// setLocations fills Location and adds Locations only when there is more than
// one entry.
func setLocations(r *jobResultJSON, locations ...string) {
	if len(locations) == 0 {
		return
	}
	r.Location = locations[0]
	if len(locations) > 1 {
		r.Locations = locations
	}
}

// fallbackURL builds a public job link when detail fetching fails, falling
// back to externalPath if the public site origin cannot be derived.
func fallbackURL(baseURL, externalPath string) string {
	site, err := workday.PublicSiteURL(baseURL)
	if err != nil {
		return externalPath
	}
	// Avoid joining the site segment directly to a path without a leading slash.
	if !strings.HasPrefix(externalPath, "/") {
		externalPath = "/" + externalPath
	}
	return site + externalPath
}
