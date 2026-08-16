package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/oracle"
)

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("oracle")
	var (
		company = rootFlags.StringLong("company", "", "curated company name or Oracle Candidate Experience careers URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "combined discovery and API request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "oracle",
		Usage: "oracle --company COMPANY [FLAGS] <companies|search|facets|detail> [FLAGS]",
		Flags: rootFlags,
	}
	env := commandEnv{httpClient: http.DefaultClient, out: os.Stdout}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "oracle companies [--format text|json]",
		ShortHelp: "list curated Oracle companies (company name and careers URL)",
		Flags:     companiesFlags,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			return runCompanies(*format, env)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	searchFlagsSet := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		searchKeyword = searchFlagsSet.StringLong("keyword", "", "free-text keyword search")
		searchLimit   = searchFlagsSet.IntLong("limit", 20, "page size (1-100)")
		searchOffset  = searchFlagsSet.IntLong("offset", 0, "zero-based result offset")
		searchFilters = searchFlagsSet.StringListLong("filter", "facet filter as name=id (repeatable)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "oracle --company COMPANY search [--keyword TEXT] [--filter name=id]... [--limit N] [--offset N] [--format text|json]",
		ShortHelp: "search public requisitions with server-side keyword and facet filters",
		Flags:     searchFlagsSet,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				commonFlags: commonFlags{
					company: *company,
					timeout: *timeout,
					format:  *format,
				},
				keyword: *searchKeyword,
				limit:   *searchLimit,
				offset:  *searchOffset,
				filters: *searchFilters,
			}, env)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	facetsFlagsSet := ff.NewFlagSet("facets").SetParent(rootFlags)
	var (
		facetsKeyword = facetsFlagsSet.StringLong("keyword", "", "narrow facet counts with a keyword")
		facetsFilters = facetsFlagsSet.StringListLong("filter", "facet filter as name=id (repeatable)")
	)
	facetsCmd := &ff.Command{
		Name:      "facets",
		Usage:     "oracle --company COMPANY facets [--keyword TEXT] [--filter name=id]... [--format text|json]",
		ShortHelp: "list standard Oracle facets and their live option counts",
		Flags:     facetsFlagsSet,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("facets takes no positional arguments, got %v", args)
			}
			return runFacets(ctx, facetsFlags{
				commonFlags: commonFlags{
					company: *company,
					timeout: *timeout,
					format:  *format,
				},
				keyword: *facetsKeyword,
				filters: *facetsFilters,
			}, env)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, facetsCmd)

	detailFlagsSet := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFlagsSet.StringLong("id", "", "job id from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "oracle --company COMPANY detail --id JOB-ID [--format text|json]",
		ShortHelp: "print one public requisition and its description sections",
		Flags:     detailFlagsSet,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runDetail(ctx, detailFlags{
				commonFlags: commonFlags{
					company: *company,
					timeout: *timeout,
					format:  *format,
				},
				jobID: *jobID,
			}, env)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

	if err := rootCmd.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd.GetSelected()))
		if errors.Is(err, ff.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	if rootCmd.GetSelected() == rootCmd {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd))
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, facets, or detail) is required")
		return 1
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

type commandEnv struct {
	httpClient *http.Client
	out        io.Writer
}

type commonFlags struct {
	company string
	timeout time.Duration
	format  string
}

func (f commonFlags) context(parent context.Context) (context.Context, context.CancelFunc, error) {
	if f.timeout <= 0 {
		return nil, nil, fmt.Errorf("--timeout must be greater than zero, got %s", f.timeout)
	}
	ctx, cancel := context.WithTimeout(parent, f.timeout)
	return ctx, cancel, nil
}

func resolveCompany(
	ctx context.Context,
	company string,
	httpClient *http.Client,
) (oracle.Site, string, error) {
	company = strings.TrimSpace(company)
	if company == "" {
		return oracle.Site{}, "", errors.New("--company is required")
	}

	if strings.Contains(company, "://") {
		site, err := oracle.DiscoverSite(ctx, company, httpClient)
		if err != nil {
			return oracle.Site{}, "", err
		}
		return site, site.Site, nil
	}

	for _, candidate := range oracle.Companies {
		if !strings.EqualFold(candidate.Name, company) {
			continue
		}
		return oracle.Site{
			CareersURL: candidate.CareersURL(),
			APIBaseURL: candidate.APIBaseURL(),
			Site:       candidate.Site,
			SiteNumber: candidate.SiteNumber,
			Language:   "en",
		}, candidate.Name, nil
	}

	return oracle.Site{}, "", fmt.Errorf(
		"company %q not found; run 'oracle companies' to see supported companies, or pass an Oracle Candidate Experience careers URL",
		company,
	)
}

func runCompanies(format string, env commandEnv) error {
	if format == "json" {
		return writeJSON(env.out, oracle.Companies)
	}
	for _, company := range oracle.Companies {
		fmt.Fprintf(env.out, "%s (%s)\n", company.Name, company.CareersURL())
	}
	return nil
}

type searchFlags struct {
	commonFlags
	keyword string
	limit   int
	offset  int
	filters []string
}

func runSearch(parent context.Context, flags searchFlags, env commandEnv) error {
	filters, err := parseFilters(flags.filters)
	if err != nil {
		return err
	}
	if flags.limit < 1 || flags.limit > 100 {
		return fmt.Errorf("--limit must be between 1 and 100, got %d", flags.limit)
	}
	if flags.offset < 0 {
		return fmt.Errorf("--offset must be >= 0, got %d", flags.offset)
	}
	ctx, cancel, err := flags.context(parent)
	if err != nil {
		return err
	}
	defer cancel()

	site, company, err := resolveCompany(ctx, flags.company, env.httpClient)
	if err != nil {
		return err
	}
	client, err := oracle.NewSiteClient(site, env.httpClient)
	if err != nil {
		return err
	}
	result, err := client.Search(ctx, oracle.SearchRequest{
		Keyword: flags.keyword,
		Limit:   flags.limit,
		Offset:  flags.offset,
		Filters: filters,
	})
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return writeJSON(env.out, result)
	}

	fmt.Fprintf(
		env.out,
		"Oracle Recruiting Cloud Jobs (company: %s, site: %s, site number: %s)\n",
		company,
		site.Site,
		site.SiteNumber,
	)
	fmt.Fprintf(env.out, "Found %d jobs; showing %d\n\n", result.Total, len(result.Jobs))
	for i, job := range result.Jobs {
		fmt.Fprintf(env.out, "%d. %s\n", i+1, job.Title)
		if job.PrimaryLocation != "" {
			fmt.Fprintf(env.out, "Location: %s\n", job.PrimaryLocation)
		}
		if job.WorkplaceType != "" {
			fmt.Fprintf(env.out, "Workplace: %s\n", job.WorkplaceType)
		}
		if !job.PostedAt.IsZero() {
			fmt.Fprintf(env.out, "Posted: %s\n", job.PostedAt.Format("2006-01-02"))
		}
		fmt.Fprintf(env.out, "ID: %s\n", job.ID)
		fmt.Fprintf(env.out, "URL: %s\n\n", job.URL)
	}
	return nil
}

type facetsFlags struct {
	commonFlags
	keyword string
	filters []string
}

func runFacets(parent context.Context, flags facetsFlags, env commandEnv) error {
	filters, err := parseFilters(flags.filters)
	if err != nil {
		return err
	}
	ctx, cancel, err := flags.context(parent)
	if err != nil {
		return err
	}
	defer cancel()

	site, _, err := resolveCompany(ctx, flags.company, env.httpClient)
	if err != nil {
		return err
	}
	client, err := oracle.NewSiteClient(site, env.httpClient)
	if err != nil {
		return err
	}
	result, err := client.Search(ctx, oracle.SearchRequest{
		Keyword: flags.keyword,
		Limit:   1,
		Facets:  oracle.AllFacets(),
		Filters: filters,
	})
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return writeJSON(env.out, result.Facets)
	}

	for _, facet := range oracle.AllFacets() {
		options := result.Facets[facet]
		if len(options) == 0 {
			continue
		}
		fmt.Fprintf(env.out, "%s:\n", facet)
		for _, option := range options {
			fmt.Fprintf(env.out, "  %s (%s): %d\n", option.Name, option.ID, option.Count)
		}
	}
	return nil
}

type detailFlags struct {
	commonFlags
	jobID string
}

func runDetail(parent context.Context, flags detailFlags, env commandEnv) error {
	if strings.TrimSpace(flags.jobID) == "" {
		return errors.New("--id is required (take it from a search result's ID)")
	}
	ctx, cancel, err := flags.context(parent)
	if err != nil {
		return err
	}
	defer cancel()

	site, _, err := resolveCompany(ctx, flags.company, env.httpClient)
	if err != nil {
		return err
	}
	client, err := oracle.NewSiteClient(site, env.httpClient)
	if err != nil {
		return err
	}
	job, err := client.Detail(ctx, flags.jobID)
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return writeJSON(env.out, job)
	}

	fmt.Fprintln(env.out, job.Title)
	if job.PrimaryLocation != "" {
		fmt.Fprintf(env.out, "Location: %s\n", job.PrimaryLocation)
	}
	if job.WorkplaceType != "" {
		fmt.Fprintf(env.out, "Workplace: %s\n", job.WorkplaceType)
	}
	if !job.PostedAt.IsZero() {
		fmt.Fprintf(env.out, "Posted: %s\n", job.PostedAt.Format("2006-01-02"))
	}
	fmt.Fprintf(env.out, "ID: %s\n", job.ID)
	fmt.Fprintf(env.out, "URL: %s\n", job.URL)

	printHTMLSection(env.out, "Description", job.DescriptionHTML)
	printHTMLSection(env.out, "Company", job.CorporateDescriptionHTML)
	printHTMLSection(env.out, "Responsibilities", job.ResponsibilitiesHTML)
	printHTMLSection(env.out, "Qualifications", job.QualificationsHTML)
	return nil
}

func parseFilters(values []string) (map[oracle.Facet][]string, error) {
	filters := make(map[oracle.Facet][]string)
	for _, raw := range values {
		name, value, ok := strings.Cut(raw, "=")
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if !ok || name == "" || value == "" {
			return nil, fmt.Errorf("--filter %q must be name=id", raw)
		}
		facet, err := oracle.ParseFacet(name)
		if err != nil {
			return nil, fmt.Errorf("--filter %q: %w", raw, err)
		}
		filters[facet] = append(filters[facet], value)
	}
	return filters, nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printHTMLSection(w io.Writer, title, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	text, err := html2text.FromString(value, html2text.Options{})
	if err != nil {
		text = value
	}
	fmt.Fprintf(w, "\n%s:\n%s\n", title, strings.TrimSpace(text))
}
