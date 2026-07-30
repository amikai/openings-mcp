// Package oracle implements the "openings-mcp oracle" debug CLI, for manual
// checks against the live surface that internal/provider/oracle documents.
package oracle

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
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	oracleprovider "github.com/amikai/openings-mcp/internal/provider/oracle"
)

type options struct {
	company string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for oracle.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "oracle",
		Short:        "Search Oracle Candidate Experience jobs, view requisitions, and list facets",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.company, "company", "", "curated company name or Oracle Candidate Experience careers URL")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "combined discovery and API request timeout")
	clihelp.FormatVar(rootCmd.PersistentFlags(), &opts.format)

	env := commandEnv{httpClient: http.DefaultClient, out: os.Stdout}

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated Oracle companies (company name and careers URL)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompanies(opts.format, env)
		},
	}

	var (
		searchKeyword string
		searchLimit   int
		searchOffset  int
		searchFilters []string
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search public requisitions with server-side keyword and facet filters",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), searchFlags{
				commonFlags: commonFlags{
					company: opts.company,
					timeout: opts.timeout,
					format:  opts.format,
				},
				keyword: searchKeyword,
				limit:   searchLimit,
				offset:  searchOffset,
				filters: searchFilters,
			}, env)
		},
	}
	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "free-text keyword search")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "page size (1-100)")
	searchCmd.Flags().IntVar(&searchOffset, "offset", 0, "zero-based result offset")
	searchCmd.Flags().StringArrayVar(&searchFilters, "filter", nil, "facet filter as name=id (repeatable)")

	var (
		facetsKeyword string
		facetsFilters []string
	)
	facetsCmd := &cobra.Command{
		Use:          "facets",
		Short:        "list standard Oracle facets and their live option counts",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFacets(cmd.Context(), facetsFlags{
				commonFlags: commonFlags{
					company: opts.company,
					timeout: opts.timeout,
					format:  opts.format,
				},
				keyword: facetsKeyword,
				filters: facetsFilters,
			}, env)
		},
	}
	facetsCmd.Flags().StringVar(&facetsKeyword, "keyword", "", "narrow facet counts with a keyword")
	facetsCmd.Flags().StringArrayVar(&facetsFilters, "filter", nil, "facet filter as name=id (repeatable)")

	var detailJobID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one public requisition and its description sections",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetail(cmd.Context(), detailFlags{
				commonFlags: commonFlags{
					company: opts.company,
					timeout: opts.timeout,
					format:  opts.format,
				},
				jobID: detailJobID,
			}, env)
		},
	}
	detailCmd.Flags().StringVar(&detailJobID, "id", "", "job id from a search result")

	rootCmd.AddCommand(companiesCmd, searchCmd, facetsCmd, detailCmd)
	return rootCmd
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
) (oracleprovider.Site, string, error) {
	company = strings.TrimSpace(company)
	if company == "" {
		return oracleprovider.Site{}, "", errors.New("--company is required")
	}

	if strings.Contains(company, "://") {
		site, err := oracleprovider.DiscoverSite(ctx, company, httpClient)
		if err != nil {
			return oracleprovider.Site{}, "", err
		}
		return site, site.Site, nil
	}

	for _, candidate := range oracleprovider.Companies {
		if !strings.EqualFold(candidate.Name, company) {
			continue
		}
		return oracleprovider.Site{
			CareersURL: candidate.CareersURL(),
			APIBaseURL: candidate.APIBaseURL(),
			Site:       candidate.Site,
			SiteNumber: candidate.SiteNumber,
			Language:   "en",
		}, candidate.Name, nil
	}

	return oracleprovider.Site{}, "", fmt.Errorf(
		"company %q not found; run 'oracle companies' to see supported companies, or pass an Oracle Candidate Experience careers URL",
		company,
	)
}

func runCompanies(format string, env commandEnv) error {
	if format == "json" {
		return writeJSON(env.out, oracleprovider.Companies)
	}
	for _, company := range oracleprovider.Companies {
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
	client, err := oracleprovider.NewSiteClient(site, env.httpClient)
	if err != nil {
		return err
	}
	result, err := client.Search(ctx, oracleprovider.SearchRequest{
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
	client, err := oracleprovider.NewSiteClient(site, env.httpClient)
	if err != nil {
		return err
	}
	result, err := client.Search(ctx, oracleprovider.SearchRequest{
		Keyword: flags.keyword,
		Limit:   1,
		Facets:  oracleprovider.AllFacets(),
		Filters: filters,
	})
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return writeJSON(env.out, result.Facets)
	}

	for _, facet := range oracleprovider.AllFacets() {
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
	client, err := oracleprovider.NewSiteClient(site, env.httpClient)
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

func parseFilters(values []string) (map[oracleprovider.Facet][]string, error) {
	filters := make(map[oracleprovider.Facet][]string)
	for _, raw := range values {
		name, value, ok := strings.Cut(raw, "=")
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if !ok || name == "" || value == "" {
			return nil, fmt.Errorf("--filter %q must be name=id", raw)
		}
		facet, err := oracleprovider.ParseFacet(name)
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
