package himalayas

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	himalayas "github.com/amikai/openings-mcp/internal/provider/himalayas"
)


var sortValues = []string{"relevant", "recent", "salaryAsc", "salaryDesc", "nameAToZ", "nameZToA", "jobs"}

type options struct {
	timeout time.Duration
	format  string
}

type browseFlags struct {
	limit  int
	offset int
}

type searchFlags struct {
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
}

// NewCommand returns a cobra.Command for himalayas.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "himalayas",
		Short:        "Himalayas remote jobs CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	bFlags := &browseFlags{}
	browseCmd := &cobra.Command{
		Use:          "browse",
		Short:        "page through the full unfiltered remote jobs feed",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrowse(cmd.Context(), browseOptions{
				timeout: opts.timeout,
				limit:   bFlags.limit,
				offset:  bFlags.offset,
				format:  opts.format,
			})
		},
	}
	browseCmd.Flags().IntVar(&bFlags.limit, "limit", 20, "page size, 1-20 (upstream caps at 20; larger values are rejected)")
	browseCmd.Flags().IntVar(&bFlags.offset, "offset", 0, "zero-based result offset")

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search remote jobs with server-side filters",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), searchOptions{
				timeout:          opts.timeout,
				keyword:          sFlags.keyword,
				country:          sFlags.country,
				worldwide:        sFlags.worldwide,
				excludeWorldwide: sFlags.excludeWorldwide,
				seniority:        sFlags.seniority,
				employmentType:   sFlags.employmentType,
				company:          sFlags.company,
				timezone:         sFlags.timezone,
				sort:             sFlags.sort,
				page:             sFlags.page,
				format:           opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&sFlags.keyword, "keyword", "", "free-text search query (fuzzy-relevance ranked)")
	searchCmd.Flags().StringVar(&sFlags.country, "country", "", "country filter: ISO alpha-2 code or country name, e.g. US")
	searchCmd.Flags().BoolVar(&sFlags.worldwide, "worldwide", false, "limit results to worldwide-friendly jobs")
	searchCmd.Flags().BoolVar(&sFlags.excludeWorldwide, "exclude-worldwide", false, "exclude worldwide matches when --country is set")
	searchCmd.Flags().StringVar(&sFlags.seniority, "seniority", "", "comma-separated seniority filters: Entry-level, Mid-level, Senior, Manager, Director, Executive")
	searchCmd.Flags().StringVar(&sFlags.employmentType, "employment-type", "", "comma-separated employment type filters: Full Time, Part Time, Contractor, Temporary, Intern, Volunteer, Other")
	searchCmd.Flags().StringVar(&sFlags.company, "company", "", "canonical Himalayas company slug (himalayas.app/companies/<slug>); comma-separated values allowed")
	searchCmd.Flags().StringVar(&sFlags.timezone, "timezone", "", "timezone filter, e.g. UTC-5 or UTC+05:30")
	searchCmd.Flags().StringVar(&sFlags.sort, "sort", "relevant", "sort order: "+strings.Join(sortValues, ", "))
	searchCmd.Flags().IntVar(&sFlags.page, "page", 1, "1-based results page (fixed 20 jobs per page)")

	rootCmd.AddCommand(browseCmd)
	rootCmd.AddCommand(searchCmd)

	return rootCmd
}

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

type browseOptions struct {
	timeout time.Duration
	limit   int
	offset  int
	format  string
}

func runBrowse(ctx context.Context, f browseOptions) error {
	if f.limit < 1 || f.limit > 20 {
		return fmt.Errorf("--limit must be between 1 and 20, got %d", f.limit)
	}
	if f.offset < 0 {
		return fmt.Errorf("--offset must be >= 0, got %d", f.offset)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := himalayas.NewClient(himalayas.DefaultBaseURL)
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

type searchOptions struct {
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

func runSearch(ctx context.Context, f searchOptions) error {
	if f.page < 1 {
		return fmt.Errorf("--page must be >= 1, got %d", f.page)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := himalayas.NewClient(himalayas.DefaultBaseURL)
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
