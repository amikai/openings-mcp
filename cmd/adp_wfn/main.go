// Command adp_wfn is a debug CLI for public ADP Workforce Now career centers.
// It talks to the provider client directly, bypassing the MCP server and the
// ATS adapter, so a board can be checked without a running server.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/adp_wfn"
)

func main() {
	rootFlags := ff.NewFlagSet("adp_wfn")
	var (
		company = rootFlags.StringLong("company", "", "roster slug, e.g. novae")
		cid     = rootFlags.StringLong("cid", "", "tenant GUID, for a board that is not on the roster")
		locale  = rootFlags.StringLong("locale", "", "locale to pin; discovered from the tenant when omitted")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "adp_wfn",
		Usage: "adp_wfn [--company SLUG | --cid GUID] [FLAGS] <companies|filters|search|get|whois> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	rootCmd.Subcommands = append(rootCmd.Subcommands, &ff.Command{
		Name:      "companies",
		Usage:     "adp_wfn companies [--format text|json]",
		ShortHelp: "list curated Workforce Now career centers",
		Flags:     companiesFlags,
		Exec: func(_ context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			return runCompanies(*format)
		},
	})

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword  = searchFS.StringLong("keyword", "", "relevance search over the description; first page only")
		location = searchFS.StringLong("location", "", "a published location value, a city, or a two-letter state; paired automatically")
		jobType  = searchFS.StringLong("job-type", "", "one job-type oid or label; must be published by the board (see filters)")
		page     = searchFS.IntLong("page", 1, "1-based page number")
	)
	rootCmd.Subcommands = append(rootCmd.Subcommands, &ff.Command{
		Name:      "search",
		Usage:     "adp_wfn --company SLUG search [--keyword TEXT] [--location VALUE] [--job-type OID] [--page N]",
		ShortHelp: "list jobs from a board",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			return runSearch(ctx, *company, *cid, *locale, *keyword, *location, *jobType, *page, *format)
		},
	})

	filtersFS := ff.NewFlagSet("filters").SetParent(rootFlags)
	rootCmd.Subcommands = append(rootCmd.Subcommands, &ff.Command{
		Name:      "filters",
		Usage:     "adp_wfn --company SLUG filters",
		ShortHelp: "print the board's published location and job-type values",
		Flags:     filtersFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("filters takes no positional arguments, got %v", args)
			}
			return runFilters(ctx, *company, *cid, *locale, *format)
		},
	})

	getFS := ff.NewFlagSet("get").SetParent(rootFlags)
	jobID := getFS.StringLong("id", "", "job id from search results")
	rootCmd.Subcommands = append(rootCmd.Subcommands, &ff.Command{
		Name:      "get",
		Usage:     "adp_wfn --company SLUG get --id ID",
		ShortHelp: "print one job with plain-text description",
		Flags:     getFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v", args)
			}
			return runGet(ctx, *company, *cid, *locale, *jobID, *format)
		},
	})

	whoisFS := ff.NewFlagSet("whois").SetParent(rootFlags)
	legacy := whoisFS.StringLong("legacy-slug", "", "resolve a retired posting.html client slug to its cid first")
	rootCmd.Subcommands = append(rootCmd.Subcommands, &ff.Command{
		Name:      "whois",
		Usage:     "adp_wfn --cid GUID whois | adp_wfn whois --legacy-slug SLUG",
		ShortHelp: "print a tenant's ADP-recorded name, client id, and locale",
		Flags:     whoisFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("whois takes no positional arguments, got %v", args)
			}
			return runWhois(ctx, *company, *cid, *legacy, *format)
		},
	})

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, filters, search, get, or whois) is required")
		os.Exit(1)
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

func newClient() (*adp_wfn.BoardClient, error) {
	return adp_wfn.NewBoardClient(adp_wfn.Config{HTTPClient: http.DefaultClient})
}

// resolveTenant turns the --company / --cid pair into the values every
// subcommand needs. A roster company brings its own locale; a bare cid has to
// have one discovered, since guessing wrong returns an empty board rather than
// an error.
func resolveTenant(ctx context.Context, company, cid, locale string) (tenantCID, tenantCCID, tenantLocale string, err error) {
	switch {
	case company != "":
		c, ok := adp_wfn.CompaniesBySlug[strings.ToLower(company)]
		if !ok {
			return "", "", "", fmt.Errorf("company %q not in roster; run 'adp_wfn companies'", company)
		}
		tenantCID, tenantCCID, tenantLocale = c.CID, c.CCID, c.Locale
	case cid != "":
		tenantCID = cid
	default:
		return "", "", "", errors.New("one of --company or --cid is required")
	}
	if locale != "" {
		tenantLocale = locale
	}
	if tenantLocale == "" {
		client, err := newClient()
		if err != nil {
			return "", "", "", err
		}
		discovered, err := client.PrimaryLocale(ctx, tenantCID)
		if err != nil {
			return "", "", "", err
		}
		tenantLocale = discovered
	}
	return tenantCID, tenantCCID, tenantLocale, nil
}

func runCompanies(format string) error {
	if format == "json" {
		return encodeJSON(adp_wfn.Companies)
	}
	for _, c := range adp_wfn.Companies {
		fmt.Printf("%s\t%s\t%s\t%s\n", c.Slug, c.Name, c.Locale, c.CID)
	}
	return nil
}

func runFilters(ctx context.Context, company, cid, locale, format string) error {
	tenantCID, _, tenantLocale, err := resolveTenant(ctx, company, cid, locale)
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	catalog, err := client.SearchFilters(ctx, tenantCID, tenantLocale)
	if err != nil {
		return err
	}
	if format == "json" {
		return encodeJSON(catalog)
	}
	// Both columns are printed because which one goes on the wire differs by
	// dimension: locations send the value, job types send the oid.
	fmt.Println("# location (send the wire value)")
	for _, v := range catalog.Locations {
		fmt.Printf("\t%s\t-> %s\n", v.Label, v.Wire)
	}
	fmt.Println("# job_type (send the wire oid)")
	for _, v := range catalog.WorkerCategories {
		fmt.Printf("\t%s\t-> %s\n", v.Label, v.Wire)
	}
	if len(catalog.Locations) == 0 {
		fmt.Println("# this board publishes no location list, but still filters by city and state")
	}
	return nil
}

func runSearch(ctx context.Context, company, cid, locale, keyword, location, jobType string, page int, format string) error {
	tenantCID, tenantCCID, tenantLocale, err := resolveTenant(ctx, company, cid, locale)
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" && page > 1 {
		return fmt.Errorf("--keyword cannot be paged: upstream reorders relevance results between identical calls, "+
			"so page %d would overlap page 1 and omit rows. Drop --page, or narrow with --location or --job-type, which page soundly", page)
	}
	params := adp_wfn.ListParams{Locale: tenantLocale, Query: keyword, Page: page}

	if location = strings.TrimSpace(location); location != "" {
		wire, ok := locationPair(location)
		if !ok {
			return fmt.Errorf("--location %q is empty once trimmed", location)
		}
		params.Locations = []string{wire}
	}
	if jobType = strings.TrimSpace(jobType); jobType != "" {
		// An unpublished oid is answered with a large plausible-looking subset
		// rather than an error, so it is checked against the tenant's catalog
		// before it goes on the wire. The filters subcommand prints the label
		// on the left and the oid on the right, and pasting the left column is
		// the easy mistake, so labels resolve too.
		catalog, err := client.SearchFilters(ctx, tenantCID, tenantLocale)
		if err != nil {
			return err
		}
		wire, ok := matchCatalog(catalog.WorkerCategories, jobType)
		if !ok {
			return fmt.Errorf("--job-type %q is not published by this board; run the filters subcommand. "+
				"An unpublished value is not rejected upstream — it returns a subset that looks filtered", jobType)
		}
		params.WorkerCategories = []string{wire}
	}

	res, err := client.List(ctx, tenantCID, params)
	if err != nil {
		return err
	}
	if format == "json" {
		return encodeJSON(map[string]any{
			"total": res.TotalNumber, "total_trusted": res.TotalTrusted,
			"has_more": res.HasMore, "shown": len(res.Jobs), "jobs": res.Jobs,
		})
	}
	trust := "exact"
	if !res.TotalTrusted {
		trust = "relevance tally, not a row count"
	}
	fmt.Printf("# %s locale=%s shown=%d upstream_total=%d (%s)\n",
		tenantCID, tenantLocale, len(res.Jobs), res.TotalNumber, trust)
	for _, j := range res.Jobs {
		id := adp_wfn.ExternalJobID(j)
		fmt.Printf("%s\t%s\t%s\t%s\n", id, j.RequisitionTitle.Value, adp_wfn.PrimaryLocation(j),
			adp_wfn.JobURL(tenantCID, tenantCCID, id, tenantLocale))
	}
	return nil
}

func runGet(ctx context.Context, company, cid, locale, jobID, format string) error {
	if strings.TrimSpace(jobID) == "" {
		return errors.New("--id is required")
	}
	tenantCID, tenantCCID, tenantLocale, err := resolveTenant(ctx, company, cid, locale)
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	j, err := client.Job(ctx, tenantCID, jobID, tenantLocale)
	if err != nil {
		return err
	}
	desc, _ := html2text.FromString(j.RequisitionDescription.Value, html2text.Options{})
	id := adp_wfn.ExternalJobID(*j)
	url := adp_wfn.JobURL(tenantCID, tenantCCID, id, tenantLocale)
	if format == "json" {
		return encodeJSON(map[string]any{
			"id": id, "title": j.RequisitionTitle.Value, "location": adp_wfn.PrimaryLocation(*j),
			"salary": adp_wfn.SalaryLine(*j), "url": url, "description": desc,
		})
	}
	fmt.Printf("%s\n%s\n", j.RequisitionTitle.Value, adp_wfn.PrimaryLocation(*j))
	if salary := adp_wfn.SalaryLine(*j); salary != "" {
		fmt.Println(salary)
	}
	fmt.Printf("%s\n\n%s\n", url, desc)
	return nil
}

// runWhois answers "which company is this GUID", which is the question that
// stands between a discovered careers URL and a roster entry.
func runWhois(ctx context.Context, company, cid, legacySlug, format string) error {
	client, err := newClient()
	if err != nil {
		return err
	}
	if legacySlug = strings.TrimSpace(legacySlug); legacySlug != "" {
		resolved, ok := client.ResolveLegacySlug(legacySlug)
		if !ok {
			return fmt.Errorf("legacy slug %q did not resolve to a cid; not every slug does", legacySlug)
		}
		cid = resolved
	}
	tenantCID, _, tenantLocale, err := resolveTenant(ctx, company, cid, "")
	if err != nil {
		return err
	}
	info, err := client.TenantInfo(ctx, tenantCID)
	if err != nil {
		return err
	}
	if format == "json" {
		return encodeJSON(map[string]any{
			"cid": tenantCID, "client_name": info.ClientName,
			"client_id": info.ClientID, "locale": tenantLocale,
		})
	}
	// The name is ADP's own record and is printed verbatim; it is known to
	// truncate and to disagree with a company's trading name.
	fmt.Printf("cid\t%s\nclient_name\t%s\nclient_id\t%s\nlocale\t%s\n",
		tenantCID, info.ClientName, info.ClientID, tenantLocale)
	return nil
}

// locationPair mirrors what the adapter does with a free-text location: build
// a value,qualifier pair rather than forward a bare token, which upstream
// answers with the whole unfiltered board.
func locationPair(location string) (string, bool) {
	location = strings.TrimSpace(location)
	if location == "" {
		return "", false
	}
	if len(location) == 2 && strings.IndexFunc(location, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'))
	}) < 0 {
		return strings.ToUpper(location) + ",LOCATION_STATE", true
	}
	if value, qualifier, ok := strings.Cut(location, ","); ok {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(qualifier) != "" {
			return location, true
		}
	}
	return strings.Trim(location, " ,") + ",LOCATION_CITY", true
}

// matchCatalog resolves caller text against a published dimension on either
// the label or the wire value.
func matchCatalog(values []adp_wfn.FilterValue, raw string) (string, bool) {
	needle := strings.ToLower(strings.TrimSpace(raw))
	for _, v := range values {
		if strings.ToLower(v.Label) == needle || strings.ToLower(v.Wire) == needle {
			return v.Wire, true
		}
	}
	return "", false
}

func encodeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
