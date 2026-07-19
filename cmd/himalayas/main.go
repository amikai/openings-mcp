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

	himalayas "github.com/amikai/openings-mcp/internal/provider/himalayas"
)

// apiBaseURL is Himalayas' public site origin — the single production
// server in the provider's openapi.yaml (paths carry the /jobs/api prefix).
const apiBaseURL = "https://himalayas.app"

var sortValues = []string{"relevant", "recent", "salaryAsc", "salaryDesc", "nameAToZ", "nameZToA", "jobs"}

func main() {
	rootFlags := ff.NewFlagSet("himalayas")
	var (
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "himalayas",
		Usage: "himalayas [FLAGS] <browse|search> [FLAGS]",
		Flags: rootFlags,
	}

	browseFS := ff.NewFlagSet("browse").SetParent(rootFlags)
	var (
		limit  = browseFS.IntLong("limit", 20, "page size, 1-20 (upstream caps at 20; larger values are rejected)")
		offset = browseFS.IntLong("offset", 0, "zero-based result offset")
	)
	browseCmd := &ff.Command{
		Name:      "browse",
		Usage:     "himalayas browse [--limit N] [--offset N] [--format text|json]",
		ShortHelp: "page through the full unfiltered remote jobs feed",
		Flags:     browseFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("browse takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runBrowse(ctx, browseFlags{timeout: *timeout, limit: *limit, offset: *offset, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, browseCmd)

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword          = searchFS.StringLong("keyword", "", "free-text search query (fuzzy-relevance ranked)")
		country          = searchFS.StringLong("country", "", "country filter: ISO alpha-2 code or country name, e.g. US")
		worldwide        = searchFS.BoolLong("worldwide", "limit results to worldwide-friendly jobs")
		excludeWorldwide = searchFS.BoolLong("exclude-worldwide", "exclude worldwide matches when --country is set")
		seniority        = searchFS.StringLong("seniority", "", "comma-separated seniority filters: Entry-level, Mid-level, Senior, Manager, Director, Executive")
		employmentType   = searchFS.StringLong("employment-type", "", "comma-separated employment type filters: Full Time, Part Time, Contractor, Temporary, Intern, Volunteer, Other")
		company          = searchFS.StringLong("company", "", "canonical Himalayas company slug (himalayas.app/companies/<slug>); comma-separated values allowed")
		timezone         = searchFS.StringLong("timezone", "", "timezone filter, e.g. UTC-5 or UTC+05:30")
		sortOrder        = searchFS.StringEnumLong("sort", "sort order", sortValues...)
		page             = searchFS.IntLong("page", 1, "1-based results page (fixed 20 jobs per page)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "himalayas search [--keyword TEXT] [--country CC] [--worldwide] [--seniority LEVELS] [--employment-type TYPES] [--company SLUG] [--timezone TZ] [--sort ORDER] [--page N] [--format text|json]",
		ShortHelp: "search remote jobs with server-side filters",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				timeout:          *timeout,
				keyword:          *keyword,
				country:          *country,
				worldwide:        *worldwide,
				excludeWorldwide: *excludeWorldwide,
				seniority:        *seniority,
				employmentType:   *employmentType,
				company:          *company,
				timezone:         *timezone,
				sort:             *sortOrder,
				page:             *page,
				format:           *format,
			})
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
		fmt.Fprintln(os.Stderr, "err: a subcommand (browse or search) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// jobSummaryJSON is the --format json shape for one job: the compact
// fields a listing needs, no description. The guid doubles as the job's
// public himalayas.app posting URL.
type jobSummaryJSON struct {
	GUID           string   `json:"guid"`
	Title          string   `json:"title"`
	Company        string   `json:"company"`
	CompanySlug    string   `json:"companySlug"`
	EmploymentType string   `json:"employmentType"`
	Seniority      []string `json:"seniority,omitempty"`
	Salary         string   `json:"salary,omitempty"`
	Locations      []string `json:"locations,omitempty"`
	PostedAt       string   `json:"postedAt"`
}

type resultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j himalayas.Job) jobSummaryJSON {
	s := jobSummaryJSON{
		GUID:           j.GUID,
		Title:          j.Title,
		Company:        j.CompanyName,
		CompanySlug:    j.CompanySlug,
		EmploymentType: string(j.EmploymentType),
		Salary:         formatSalary(j),
		Locations:      j.LocationRestrictions,
		PostedAt:       time.Unix(j.PubDate, 0).UTC().Format("2006-01-02"),
	}
	for _, lvl := range j.Seniority {
		s.Seniority = append(s.Seniority, string(lvl))
	}
	return s
}

// formatSalary renders the disclosed salary range, e.g. "USD 120000-180000
// annual". A null currency means no salary line at all, since the currency
// is required to render either bound meaningfully.
func formatSalary(j himalayas.Job) string {
	if j.Currency.Null {
		return ""
	}
	minSalary, hasMin := salaryValue(j.MinSalary)
	maxSalary, hasMax := salaryValue(j.MaxSalary)
	var bounds string
	switch {
	case hasMin && hasMax:
		bounds = fmt.Sprintf("%.0f-%.0f", minSalary, maxSalary)
	case hasMin:
		bounds = fmt.Sprintf("from %.0f", minSalary)
	case hasMax:
		bounds = fmt.Sprintf("up to %.0f", maxSalary)
	default:
		return ""
	}
	return fmt.Sprintf("%s %s %s", j.Currency.Value, bounds, j.SalaryPeriod)
}

func salaryValue(v himalayas.OptNilFloat64) (float64, bool) {
	if !v.Set || v.Null {
		return 0, false
	}
	return v.Value, true
}

// printSummary prints one job's compact text block (everything below the
// title line).
func printSummary(s jobSummaryJSON) {
	fmt.Printf("Company: %s (%s)\n", s.Company, s.CompanySlug)
	fmt.Printf("Type: %s", s.EmploymentType)
	if len(s.Seniority) > 0 {
		fmt.Printf(" (%s)", strings.Join(s.Seniority, ", "))
	}
	fmt.Println()
	if s.Salary != "" {
		fmt.Printf("Salary: %s\n", s.Salary)
	}
	if len(s.Locations) > 0 {
		fmt.Printf("Locations: %s\n", strings.Join(s.Locations, ", "))
	} else {
		fmt.Println("Locations: Worldwide")
	}
	fmt.Printf("Posted: %s\n", s.PostedAt)
	fmt.Printf("URL: %s\n", s.GUID)
}

func printResult(total int, jobs []jobSummaryJSON, format, heading string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resultJSON{Total: total, Jobs: jobs})
	}

	fmt.Println(heading)
	fmt.Printf("Found %d jobs; showing %d\n\n", total, len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

// browseFlags carries the parsed "browse" subcommand flags into runBrowse.
type browseFlags struct {
	timeout time.Duration
	limit   int
	offset  int
	format  string
}

func runBrowse(ctx context.Context, f browseFlags) error {
	if f.limit < 1 || f.limit > 20 {
		return fmt.Errorf("--limit must be between 1 and 20, got %d", f.limit)
	}
	if f.offset < 0 {
		return fmt.Errorf("--offset must be >= 0, got %d", f.offset)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := himalayas.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.BrowseJobs(ctx, himalayas.BrowseJobsParams{
		Limit:  himalayas.NewOptInt(f.limit),
		Offset: himalayas.NewOptInt(f.offset),
	})
	if err != nil {
		return err
	}

	jobs := make([]jobSummaryJSON, len(res.Jobs))
	for i, j := range res.Jobs {
		jobs[i] = summarize(j)
	}
	return printResult(res.TotalCount, jobs, f.format, "Himalayas Remote Jobs Feed")
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	timeout          time.Duration
	keyword          string
	country          string
	worldwide        bool
	excludeWorldwide bool
	seniority        string
	employmentType   string
	company          string
	timezone         string
	sort             string
	page             int
	format           string
}

// runSearch maps every flag directly onto the search endpoint's real
// server-side filters. Note the API rejects unrecognized --country,
// --timezone, and --company values as 400 errors rather than returning
// empty results.
func runSearch(ctx context.Context, f searchFlags) error {
	if f.page < 1 {
		return fmt.Errorf("--page must be >= 1, got %d", f.page)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := himalayas.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	params := himalayas.SearchJobsParams{
		Sort: himalayas.NewOptSearchJobsSort(himalayas.SearchJobsSort(f.sort)),
		Page: himalayas.NewOptInt(f.page),
	}
	if f.keyword != "" {
		params.Q = himalayas.NewOptString(f.keyword)
	}
	if f.country != "" {
		params.Country = himalayas.NewOptString(f.country)
	}
	if f.worldwide {
		params.Worldwide = himalayas.NewOptBool(true)
	}
	if f.excludeWorldwide {
		params.ExcludeWorldwide = himalayas.NewOptBool(true)
	}
	if f.seniority != "" {
		params.Seniority = himalayas.NewOptString(f.seniority)
	}
	if f.employmentType != "" {
		params.EmploymentType = himalayas.NewOptString(f.employmentType)
	}
	if f.company != "" {
		params.Company = himalayas.NewOptString(f.company)
	}
	if f.timezone != "" {
		params.Timezone = himalayas.NewOptString(f.timezone)
	}

	res, err := client.SearchJobs(ctx, params)
	if err != nil {
		return err
	}

	switch d := res.(type) {
	case *himalayas.JobsResponse:
		jobs := make([]jobSummaryJSON, len(d.Jobs))
		for i, j := range d.Jobs {
			jobs[i] = summarize(j)
		}
		return printResult(d.TotalCount, jobs, f.format, "Himalayas Remote Jobs Search")
	case *himalayas.SearchError:
		return fmt.Errorf("himalayas rejected the search: %s", d.Errors)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}
