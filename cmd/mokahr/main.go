// Command mokahr is a debug CLI for the MokaHR careers-site provider: list
// curated companies, search a site's postings, print one posting in full, and
// list a site's filterable locations and 职能 (job functions).
package main

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
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"github.com/peterbourgon/ff/v4/ffval"

	"github.com/amikai/openings-mcp/internal/provider/mokahr"
)

const _formatJSON = "json"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("mokahr")
	var (
		company = rootFlags.StringLong("company", "", `curated roster company name or "<org>/<site>" slug from the careers URL, e.g. "high-flyer/140576"`)
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "mokahr",
		Usage: "mokahr --company COMPANY [FLAGS] <companies|search|get|filters> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "mokahr companies [--format text|json]",
		ShortHelp: "list curated MokaHR companies (name, org/site slug, careers URL)",
		Flags:     companiesFlags,
		Exec: func(_ context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			return runCompanies(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword     = searchFS.StringLong("keyword", "", "title substring match (MokaHR matches job titles only, never descriptions)")
		locationIDs []int
		zhinengIDs  []int
		limit       = searchFS.IntLong("limit", 20, "page size, 1-50 (upstream rejects 60 with code 102)")
		offset      = searchFS.IntLong("offset", 0, "zero-based result offset")
	)
	// ff has no built-in repeatable-int flag, so these two go in through
	// AddFlag. A rejected static flag definition is a build-time bug.
	if _, err := searchFS.AddFlag(ff.FlagConfig{
		LongName: "location-id",
		Usage:    "location id from 'filters' output; repeatable, ORed",
		Value:    ffval.NewList(&locationIDs),
	}); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	if _, err := searchFS.AddFlag(ff.FlagConfig{
		LongName: "zhineng-id",
		Usage:    "职能 (job-function) id from 'filters' output; repeatable, ORed",
		Value:    ffval.NewList(&zhinengIDs),
	}); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "mokahr --company COMPANY search [--keyword TEXT] [--location-id ID]... [--zhineng-id ID]... [--limit N] [--offset N] [--format text|json]",
		ShortHelp: "search a site's postings (server-side filters)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				company:     *company,
				timeout:     *timeout,
				keyword:     *keyword,
				locationIDs: locationIDs,
				zhinengIDs:  zhinengIDs,
				limit:       *limit,
				offset:      *offset,
				format:      *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	getFS := ff.NewFlagSet("get").SetParent(rootFlags)
	jobID := getFS.StringLong("id", "", "job id (UUID) from a search result")
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "mokahr --company COMPANY get --id JOB-ID [--format text|json]",
		ShortHelp: "print one posting in full (description, department, commitment, URL)",
		Flags:     getFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runGet(ctx, getFlags{company: *company, timeout: *timeout, jobID: *jobID, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, getCmd)

	filtersFS := ff.NewFlagSet("filters").SetParent(rootFlags)
	filtersCmd := &ff.Command{
		Name:      "filters",
		Usage:     "mokahr --company COMPANY filters [--format text|json]",
		ShortHelp: "list a site's filterable locations and 职能 (job functions), with their ids",
		Flags:     filtersFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("filters takes no positional arguments, got %v", args)
			}
			return runFilters(ctx, filtersFlags{company: *company, timeout: *timeout, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, filtersCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, get, or filters) is required")
		return 1
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

// resolveCompany accepts a roster company name (case-insensitive) or an
// "<org>/<site>" slug — including one not in the roster, since MokaHR needs
// no credentials and any tenant's slug is guessable from its careers URL.
func resolveCompany(company string) (mokahr.Company, error) {
	if company == "" {
		return mokahr.Company{}, errors.New("--company is required")
	}
	if c, ok := mokahr.CompaniesBySlug[strings.ToLower(company)]; ok {
		return c, nil
	}
	for _, c := range mokahr.Companies {
		if strings.EqualFold(c.Name, company) {
			return c, nil
		}
	}
	org, site, ok := strings.Cut(company, "/")
	if ok && org != "" && site != "" {
		return mokahr.Company{Name: company, Org: org, Site: site}, nil
	}
	return mokahr.Company{}, fmt.Errorf("company %q not found; run 'mokahr companies' to see supported companies, or pass an \"<org>/<site>\" slug", company)
}

// runCompanies lists every curated MokaHR company embedded in the CLI
// (internal/provider/mokahr/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	if format == _formatJSON {
		return writeJSON(mokahr.Companies)
	}
	for _, c := range mokahr.Companies {
		fmt.Printf("%s (%s) — %s\n", c.Name, c.Slug(), c.CareersURL())
	}
	return nil
}

func newClient(timeout time.Duration) (*mokahr.JobsClient, error) {
	return mokahr.NewJobsClient(mokahr.DefaultBaseURL, &http.Client{Timeout: timeout})
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

func summarize(j mokahr.Job, org, site string) jobSummaryJSON {
	s := jobSummaryJSON{
		ID:       j.ID,
		Title:    j.Title,
		Location: locations(j.Locations.Or(nil)),
		OpenedAt: j.OpenedAt.Or(""),
		URL:      mokahr.JobURL(org, site, j.ID),
	}
	if z, ok := j.Zhineng.Get(); ok {
		s.Zhineng = z.Name.Or("")
	}
	return s
}

// locations joins every workplace as "city, province, country", skipping
// empty parts, and joins multiple workplaces with "; ".
func locations(ls []mokahr.Location) string {
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

	req := mokahr.ListJobsRequest{
		OrgId:       c.Org,
		SiteId:      c.Site,
		Limit:       mokahr.NewOptInt(f.limit),
		Offset:      mokahr.NewOptInt(f.offset),
		NeedStat:    mokahr.NewOptBool(true),
		LocationIds: f.locationIDs,
		ZhinengIds:  f.zhinengIDs,
	}
	if f.keyword != "" {
		req.Keyword = mokahr.NewOptString(f.keyword)
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

	if f.format == _formatJSON {
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

	d, err := client.GetJob(ctx, mokahr.GetJobRequest{OrgId: c.Org, SiteId: c.Site, JobId: f.jobID})
	if err != nil {
		var apiErr *mokahr.APIError
		if errors.As(err, &apiErr) && apiErr.Code == mokahr.CodeJobNotFound {
			return fmt.Errorf("job %q not found for %q (upstream %d: %s)", f.jobID, c.Slug(), apiErr.Code, apiErr.Msg)
		}
		return err
	}

	return printDetail(d, c, f.format)
}

// printDetail renders one full posting. JSON mode encodes the generated
// JobDetail as-is — detail is for seeing the whole record.
func printDetail(d *mokahr.JobDetail, c mokahr.Company, format string) error {
	if format == _formatJSON {
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
	fmt.Printf("URL: %s\n", mokahr.JobURL(c.Org, c.Site, d.ID))

	printSection(d.JobDescription)
	return nil
}

// printSection renders the job description, converting its HTML to plain
// text and falling back to the raw HTML on a conversion failure rather than
// dropping the section.
func printSection(opt mokahr.OptString) {
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

	aggs, err := client.ListFilterAggregations(ctx, mokahr.SiteRef{OrgId: c.Org, SiteId: c.Site})
	if err != nil {
		return err
	}

	if f.format == _formatJSON {
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
func printZhineng(f mokahr.ZhinengFacet, depth int) {
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
