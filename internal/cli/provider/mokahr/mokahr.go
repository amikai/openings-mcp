// Package mokahr implements the "openings-mcp mokahr" debug CLI, for manual
// checks against the live surface that internal/provider/mokahr documents.
package mokahr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	mokahrprovider "github.com/amikai/openings-mcp/internal/provider/mokahr"
)

const formatJSON = "json"

type options struct {
	company string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for mokahr.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "mokahr",
		Short:        "mokahr --company COMPANY [FLAGS] <companies|search|get|filters> [FLAGS]",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.company, "company", "", `curated roster company name or "<org>/<site>" slug from the careers URL, e.g. "high-flyer/140576"`)
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	clihelp.FormatVar(rootCmd.PersistentFlags(), &opts.format)

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated MokaHR companies (name, org/site slug, careers URL)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			return runCompanies(opts.format)
		},
	}

	var (
		searchKeyword     string
		searchLocationIDs []int
		searchZhinengIDs  []int
		searchLimit       int
		searchOffset      int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search a site's postings (server-side filters)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(cmd.Context(), searchFlags{
				company:     opts.company,
				timeout:     opts.timeout,
				keyword:     searchKeyword,
				locationIDs: searchLocationIDs,
				zhinengIDs:  searchZhinengIDs,
				limit:       searchLimit,
				offset:      searchOffset,
				format:      opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "title substring match (MokaHR matches job titles only, never descriptions)")
	searchCmd.Flags().IntSliceVar(&searchLocationIDs, "location-id", nil, "location id from 'filters' output; repeatable, ORed")
	searchCmd.Flags().IntSliceVar(&searchZhinengIDs, "zhineng-id", nil, "职能 (job-function) id from 'filters' output; repeatable, ORed")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "page size, 1-50 (upstream rejects 60 with code 102)")
	searchCmd.Flags().IntVar(&searchOffset, "offset", 0, "zero-based result offset")

	var getJobID string
	getCmd := &cobra.Command{
		Use:          "get",
		Short:        "print one posting in full (description, department, commitment, URL)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runGet(cmd.Context(), getFlags{
				company: opts.company,
				timeout: opts.timeout,
				jobID:   getJobID,
				format:  opts.format,
			})
		},
	}
	getCmd.Flags().StringVar(&getJobID, "id", "", "job id (UUID) from a search result")

	filtersCmd := &cobra.Command{
		Use:          "filters",
		Short:        "list a site's filterable locations and 职能 (job functions), with their ids",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("filters takes no positional arguments, got %v", args)
			}
			return runFilters(cmd.Context(), filtersFlags{
				company: opts.company,
				timeout: opts.timeout,
				format:  opts.format,
			})
		},
	}

	rootCmd.AddCommand(companiesCmd, searchCmd, getCmd, filtersCmd)
	return rootCmd
}

// resolveCompany accepts a roster company name (case-insensitive) or an
// "<org>/<site>" slug — including one not in the roster, since MokaHR needs
// no credentials and any tenant's slug is guessable from its careers URL.
func resolveCompany(company string) (mokahrprovider.Company, error) {
	if company == "" {
		return mokahrprovider.Company{}, errors.New("--company is required")
	}
	if c, ok := mokahrprovider.CompaniesBySlug[strings.ToLower(company)]; ok {
		return c, nil
	}
	for _, c := range mokahrprovider.Companies {
		if strings.EqualFold(c.Name, company) {
			return c, nil
		}
	}
	org, site, ok := strings.Cut(company, "/")
	if ok && org != "" && site != "" {
		return mokahrprovider.Company{Name: company, Org: org, Site: site}, nil
	}
	return mokahrprovider.Company{}, fmt.Errorf("company %q not found; run 'mokahr companies' to see supported companies, or pass an \"<org>/<site>\" slug", company)
}

// runCompanies lists every curated MokaHR company embedded in the CLI
// (internal/provider/mokahr/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	if format == formatJSON {
		return writeJSON(mokahrprovider.Companies)
	}
	for _, c := range mokahrprovider.Companies {
		fmt.Printf("%s (%s) — %s\n", c.Name, c.Slug(), c.CareersURL())
	}
	return nil
}

func newClient(timeout time.Duration) (*mokahrprovider.JobsClient, error) {
	return mokahrprovider.NewJobsClient(mokahrprovider.DefaultBaseURL, &http.Client{Timeout: timeout})
}

// jobSummaryJSON is the --format json shape for one search result.
type jobSummaryJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location,omitempty"`
	Zhineng  string `json:"zhineng,omitempty"`
	OpenedAt string `json:"openedAt,omitempty"`
	URL      string `json:"url"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j mokahrprovider.Job, org, site string) jobSummaryJSON {
	s := jobSummaryJSON{
		ID:       j.ID,
		Title:    j.Title,
		Location: locations(j.Locations.Or(nil)),
		OpenedAt: j.OpenedAt.Or(""),
		URL:      mokahrprovider.JobURL(org, site, j.ID),
	}
	if z, ok := j.Zhineng.Get(); ok {
		s.Zhineng = z.Name.Or("")
	}
	return s
}

// locations joins every workplace as "city, province, country", skipping
// empty parts, and joins multiple workplaces with "; ".
func locations(ls []mokahrprovider.Location) string {
	parts := make([]string, 0, len(ls))
	for _, l := range ls {
		fields := []string{l.CityName.Or(""), l.ProvinceName.Or(""), l.Country.Or("")}
		nonEmpty := make([]string, 0, len(fields))
		for _, f := range fields {
			if f != "" {
				nonEmpty = append(nonEmpty, f)
			}
		}
		if len(nonEmpty) > 0 {
			parts = append(parts, strings.Join(nonEmpty, ", "))
		}
	}
	return strings.Join(parts, "; ")
}

func printSummary(s jobSummaryJSON) {
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.Zhineng != "" {
		fmt.Printf("职能: %s\n", s.Zhineng)
	}
	if s.OpenedAt != "" {
		fmt.Printf("Opened: %s\n", s.OpenedAt)
	}
	fmt.Printf("ID: %s\n", s.ID)
	fmt.Printf("URL: %s\n", s.URL)
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	company     string
	timeout     time.Duration
	keyword     string
	locationIDs []int
	zhinengIDs  []int
	limit       int
	offset      int
	format      string
}

func runSearch(ctx context.Context, f searchFlags) error {
	c, err := resolveCompany(f.company)
	if err != nil {
		return err
	}
	if f.limit < 1 || f.limit > 50 {
		return fmt.Errorf("--limit must be between 1 and 50, got %d", f.limit)
	}
	if f.offset < 0 {
		return fmt.Errorf("--offset must be >= 0, got %d", f.offset)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := newClient(f.timeout)
	if err != nil {
		return err
	}

	req := mokahrprovider.ListJobsRequest{
		OrgId:       c.Org,
		SiteId:      c.Site,
		Limit:       mokahrprovider.NewOptInt(f.limit),
		Offset:      mokahrprovider.NewOptInt(f.offset),
		NeedStat:    mokahrprovider.NewOptBool(true),
		LocationIds: f.locationIDs,
		ZhinengIds:  f.zhinengIDs,
	}
	if f.keyword != "" {
		req.Keyword = mokahrprovider.NewOptString(f.keyword)
	}

	res, err := client.ListJobs(ctx, req)
	if err != nil {
		return err
	}

	jobs := make([]jobSummaryJSON, len(res.Jobs))
	for i, j := range res.Jobs {
		jobs[i] = summarize(j, c.Org, c.Site)
	}
	total := res.JobStats.Value.Total.Or(len(jobs))

	if f.format == formatJSON {
		return writeJSON(searchResultJSON{Total: total, Jobs: jobs})
	}

	fmt.Printf("MokaHR Jobs Report (company: %s, site: %s)\n", c.Name, c.Slug())
	fmt.Printf("Found %d jobs; showing %d\n\n", total, len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

// getFlags carries the parsed "get" subcommand flags into runGet.
type getFlags struct {
	company string
	timeout time.Duration
	jobID   string
	format  string
}

func runGet(ctx context.Context, f getFlags) error {
	c, err := resolveCompany(f.company)
	if err != nil {
		return err
	}
	if f.jobID == "" {
		return errors.New("--id is required (take it from a search result's ID)")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := newClient(f.timeout)
	if err != nil {
		return err
	}

	d, err := client.GetJob(ctx, mokahrprovider.GetJobRequest{OrgId: c.Org, SiteId: c.Site, JobId: f.jobID})
	if err != nil {
		var apiErr *mokahrprovider.APIError
		if errors.As(err, &apiErr) && apiErr.Code == mokahrprovider.CodeJobNotFound {
			return fmt.Errorf("job %q not found for %q (upstream %d: %s)", f.jobID, c.Slug(), apiErr.Code, apiErr.Msg)
		}
		return err
	}

	return printDetail(d, c, f.format)
}

// printDetail renders one full posting. JSON mode encodes the generated
// JobDetail as-is — detail is for seeing the whole record.
func printDetail(d *mokahrprovider.JobDetail, c mokahrprovider.Company, format string) error {
	if format == formatJSON {
		return writeJSON(d)
	}

	fmt.Println(d.Title)
	fmt.Printf("ID: %s\n", d.ID)
	if dep, ok := d.Department.Get(); ok {
		fmt.Printf("Department: %s\n", dep.Name.Or(""))
	}
	if commitment := d.Commitment.Or(""); commitment != "" {
		fmt.Printf("Commitment: %s\n", commitment)
	}
	if loc := locations(d.Locations.Or(nil)); loc != "" {
		fmt.Printf("Location: %s\n", loc)
	}
	if opened := d.OpenedAt.Or(""); opened != "" {
		fmt.Printf("Opened: %s\n", opened)
	}
	if published := d.PublishedAt.Or(""); published != "" {
		fmt.Printf("Published: %s\n", published)
	}
	fmt.Printf("URL: %s\n", mokahrprovider.JobURL(c.Org, c.Site, d.ID))

	printSection(d.JobDescription)
	return nil
}

// printSection renders the job description, converting its HTML to plain
// text and falling back to the raw HTML on a conversion failure rather than
// dropping the section.
func printSection(opt mokahrprovider.OptString) {
	html, ok := opt.Get()
	if !ok || html == "" {
		return
	}
	rendered, err := html2text.FromString(html, html2text.Options{})
	if err != nil {
		rendered = html
	}
	fmt.Printf("\nDescription:\n%s\n", rendered)
}

// filtersFlags carries the parsed "filters" subcommand flags into runFilters.
type filtersFlags struct {
	company string
	timeout time.Duration
	format  string
}

func runFilters(ctx context.Context, f filtersFlags) error {
	c, err := resolveCompany(f.company)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := newClient(f.timeout)
	if err != nil {
		return err
	}

	aggs, err := client.ListFilterAggregations(ctx, mokahrprovider.SiteRef{OrgId: c.Org, SiteId: c.Site})
	if err != nil {
		return err
	}

	if f.format == formatJSON {
		return writeJSON(aggs)
	}

	sys, ok := aggs.SystemFieldsAggregations.Get()
	if !ok {
		fmt.Println("no filterable dimensions reported for this site")
		return nil
	}

	fmt.Printf("MokaHR Filters (company: %s, site: %s)\n\n", c.Name, c.Slug())

	if locAgg, ok := sys.LocationAggregation.Get(); ok {
		fmt.Println("Locations (--location-id):")
		for _, facet := range locAgg.LocationList {
			ids := make([]string, 0, len(facet.LocationRows))
			for _, row := range facet.LocationRows {
				if id, ok := row.ID.Get(); ok {
					ids = append(ids, strconv.Itoa(id))
				}
			}
			fmt.Printf("  %s: %s\n", facet.Label.Or(""), strings.Join(ids, ", "))
		}
		fmt.Println()
	}

	if zhinengAgg, ok := sys.ZhinengAggregation.Get(); ok {
		fmt.Println("职能 (--zhineng-id):")
		for _, facet := range zhinengAgg.ZhinengList {
			printZhineng(facet, 1)
		}
	}
	return nil
}

// printZhineng renders one 职能 facet and its children, indented by depth,
// so the id a caller passes to --zhineng-id is visible next to its label.
func printZhineng(f mokahrprovider.ZhinengFacet, depth int) {
	fmt.Printf("%s%s (id=%d, jobs=%d)\n", strings.Repeat("  ", depth), f.Label.Or(""), f.ID.Or(0), f.JobCount.Or(0))
	for _, child := range f.Children {
		printZhineng(child, depth+1)
	}
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode JSON output: %w", err)
	}
	return nil
}
