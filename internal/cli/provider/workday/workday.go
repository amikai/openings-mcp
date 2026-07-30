// Package workday implements the "openings-mcp workday" debug CLI, for
// manual checks against the live surface that internal/provider/workday
// documents.
package workday

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
	"golang.org/x/sync/errgroup"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	workday "github.com/amikai/openings-mcp/internal/provider/workday"
)

// maxConcurrentDetailFetches caps how many job-detail requests runSearch
// fires at once — fetchJobResult never returns an error, so the only reason
// to bound it is being a considerate caller of someone else's career site
// rather than firing --limit-many requests in a single burst.
const maxConcurrentDetailFetches = 5

type rootOptions struct {
	tenant  string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for workday.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "workday",
		Short:        "Search Workday jobs and view position details",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.tenant, "tenant", "", "confirmed Workday tenant slug, e.g. 3m, att (see 'workday companies' for the full list)")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	clihelp.FormatVar(cmd.PersistentFlags(), &opts.format)

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list confirmed Workday tenants (company name and tenant slug)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompanies(opts.format)
		},
	}

	var (
		facetsSearchText string
		facetsFacetArgs  []string
	)
	facetsCmd := &cobra.Command{
		Use:          "facets",
		Short:        "discover a tenant's current facet tree (categories, locations, ...)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFacets(cmd.Context(), facetsFlags{
				tenant:     opts.tenant,
				timeout:    opts.timeout,
				searchText: facetsSearchText,
				facetArgs:  facetsFacetArgs,
				format:     opts.format,
			})
		},
	}
	facetsCmd.Flags().StringVar(&facetsSearchText, "search-text", "", "free-text keyword search to narrow the facet tree")
	facetsCmd.Flags().StringArrayVar(&facetsFacetArgs, "facet", nil, "facet filter as name=id, repeatable")

	var (
		searchText      string
		limit           int
		offset          int
		searchFacetArgs []string
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search jobs and fetch full detail for each result",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), searchFlags{
				tenant:     opts.tenant,
				timeout:    opts.timeout,
				searchText: searchText,
				limit:      limit,
				offset:     offset,
				facetArgs:  searchFacetArgs,
				format:     opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&searchText, "search-text", "", "free-text keyword search")
	searchCmd.Flags().IntVar(&limit, "limit", 20, "page size")
	searchCmd.Flags().IntVar(&offset, "offset", 0, "zero-based result offset")
	searchCmd.Flags().StringArrayVar(&searchFacetArgs, "facet", nil, "facet filter as name=id, repeatable")

	cmd.AddCommand(companiesCmd, facetsCmd, searchCmd)
	return cmd
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

// facetsFlags carries the parsed "facets" subcommand flags into runFacets.
type facetsFlags struct {
	tenant     string
	timeout    time.Duration
	searchText string
	facetArgs  []string
	format     string
}

// runFacets discovers a tenant's current facet tree via a search whose only
// job is to read back JobsResponse.Facets — Limit is 1 because the actual
// jobPostings aren't used here (see openapi.yaml's note that every /jobs
// response, filtered or not, carries the full current facet tree).
func runFacets(ctx context.Context, f facetsFlags) error {
	if f.tenant == "" {
		return errors.New("--tenant is required")
	}
	_, ok := workday.CompaniesByTenant[strings.ToLower(f.tenant)]
	if !ok {
		return fmt.Errorf("tenant %q not found; run 'workday companies' to see supported tenants", f.tenant)
	}

	appliedFacets, err := parseFacets(f.facetArgs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := workday.NewTenantClient()
	if err != nil {
		return err
	}

	search, err := client.JobsByTenant(ctx, f.tenant, &workday.JobsRequest{
		AppliedFacets: appliedFacets,
		Limit:         1,
		Offset:        0,
		SearchText:    f.searchText,
	})
	if err != nil {
		return err
	}

	facets, _ := search.Facets.Get()

	if f.format == "json" {
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
	if param, ok := node.FacetParameter.Get(); ok {
		label := param
		if descriptor, ok := node.Descriptor.Get(); ok {
			label = fmt.Sprintf("%s (%s)", label, descriptor)
		}
		fmt.Println(indent + label)
		for _, child := range node.Values {
			printFacetNode(child, depth+1)
		}
		return
	}
	descriptor, _ := node.Descriptor.Get()
	count, _ := node.Count.Get()
	fmt.Printf("%s%s  id=%s  count=%d\n", indent, descriptor, node.ID.Value, count)
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

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	tenant     string
	timeout    time.Duration
	searchText string
	limit      int
	offset     int
	facetArgs  []string
	format     string
}

// runSearch searches jobs, then fetches full detail for every result
// (mirrors the nvidia CLI's report behavior: a posting with no ExternalPath is
// listed with a "no detail available" note rather than silently dropped, so
// "showing N" always matches the page's posting count) — one page per
// invocation, no auto-pagination.
func runSearch(ctx context.Context, f searchFlags) error {
	if f.tenant == "" {
		return errors.New("--tenant is required")
	}
	company, ok := workday.CompaniesByTenant[strings.ToLower(f.tenant)]
	if !ok {
		return fmt.Errorf("tenant %q not found; run 'workday companies' to see supported tenants", f.tenant)
	}

	appliedFacets, err := parseFacets(f.facetArgs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	baseURL := company.BaseURL()
	client, err := workday.NewTenantClient()
	if err != nil {
		return err
	}

	search, err := client.JobsByTenant(ctx, f.tenant, &workday.JobsRequest{
		AppliedFacets: appliedFacets,
		Limit:         f.limit,
		Offset:        f.offset,
		SearchText:    f.searchText,
	})
	if err != nil {
		return err
	}

	results := make([]jobResultJSON, len(search.JobPostings))
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentDetailFetches)
	for i, job := range search.JobPostings {
		g.Go(func() error {
			results[i] = fetchJobResult(gCtx, client, f.tenant, baseURL, job)
			return nil
		})
	}
	_ = g.Wait()

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: search.Total.Value, Jobs: results})
	}

	fmt.Printf("Workday Jobs Report\n")
	fmt.Printf("Found %d jobs; showing %d\n\n", search.Total.Value, len(results))
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
// description, so one bad job doesn't abort the whole search — mirrors the
// nvidia CLI's existing per-job fallback behavior. A summary with no
// ExternalPath (an incomplete/transient Workday posting) can't be fetched at
// all, so it's returned with a "no detail available" note rather than dropped.
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
	if info.Title.Value != "" {
		r.Title = info.Title.Value
	}
	if info.PostedOn.Set {
		r.PostedOn = info.PostedOn.Value
	}
	r.JobReqId = info.JobReqId.Value
	r.URL = fallbackURL(baseURL, job.ExternalPath.Value)
	if info.ExternalUrl.Set {
		r.URL = info.ExternalUrl.Value
	}

	itemized := make([]string, 0, 1+len(info.AdditionalLocations))
	if info.Location.Set {
		itemized = append(itemized, info.Location.Value)
	}
	itemized = append(itemized, info.AdditionalLocations...)
	setLocations(&r, itemized...)

	description, err := html2text.FromString(info.JobDescription.Value, html2text.Options{})
	if err != nil {
		description = info.JobDescription.Value
	}
	r.Description = description

	return r
}

// setLocations fills both the singular Location (first entry, for quick
// access) and the full Locations array (only when there's more than one, to
// avoid a redundant one-element array alongside the singular field) —
// mirrors the nvidia CLI's printLocations singular/plural distinction.
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
	if !strings.HasPrefix(externalPath, "/") {
		externalPath = "/" + externalPath
	}
	return site + externalPath
}
