// Command fourdayweek is a debug CLI for 4dayweek.io's public jobs API.
//
//	go run ./cmd/fourdayweek search --work-arrangement remote --schedule 4_day_week
//	go run ./cmd/fourdayweek search --query golang --country Germany --salary-min 100000
//	go run ./cmd/fourdayweek detail --slug senior-infrastructure-engineer-at-buffer-38679a55
//
// Every search filter runs server-side. Two results are easy to misread:
// --country matches any of a job's locations regardless of that location's
// own arrangement, so pairing it with --work-arrangement remote does not
// mean "remote-workable from there"; and --salary-min/--salary-max take
// whole dollars while the salary the API returns is in cents (see the quirk
// notes in internal/provider/fourdayweek/openapi.yaml).
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

	"github.com/amikai/openings-mcp/internal/provider/fourdayweek"
)

// apiBaseURL is 4dayweek.io's site origin — the single production server in
// the provider's openapi.yaml (paths carry the /api/v2 prefix).
const apiBaseURL = "https://4dayweek.io"

var sortValues = []string{"date", "salary"}

func main() {
	rootFlags := ff.NewFlagSet("fourdayweek")
	var (
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "fourdayweek",
		Usage: "fourdayweek [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		query           = searchFS.StringLong("query", "", "full-text search query")
		category        = searchFS.StringLong("category", "", "comma-separated category slugs, e.g. engineering,data")
		level           = searchFS.StringLong("level", "", "comma-separated seniority slugs: entry, mid, senior, lead, executive")
		schedule        = searchFS.StringLong("schedule", "", "comma-separated schedule types, e.g. 4_day_week,9_day_fortnight")
		workArrangement = searchFS.StringLong("work-arrangement", "", "comma-separated arrangements: onsite, hybrid, remote")
		skills          = searchFS.StringLong("skills", "", "comma-separated skill slugs, e.g. python,go")
		country         = searchFS.StringLong("country", "", "country name (matches any location, not only remote-allowed ones), e.g. Germany")
		salaryMin       = searchFS.IntLong("salary-min", 0, "minimum salary in USD whole dollars (responses report cents)")
		salaryMax       = searchFS.IntLong("salary-max", 0, "maximum salary in USD whole dollars (responses report cents)")
		postedAfter     = searchFS.IntLong("posted-after", 0, "posted within the last N days, 1-365 (0 = unset)")
		sortOrder       = searchFS.StringEnumLong("sort", "sort order", sortValues...)
		page            = searchFS.IntLong("page", 1, "1-based results page")
		limit           = searchFS.IntLong("limit", 25, "page size, 1-100")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "fourdayweek search [--query TEXT] [--work-arrangement KINDS] [--schedule TYPES] [--category SLUGS] [--level LEVELS] [--skills SLUGS] [--country NAME] [--salary-min N] [--salary-max N] [--posted-after DAYS] [--sort date|salary] [--page N] [--limit N] [--format text|json]",
		ShortHelp: "search jobs with server-side filters",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				timeout:         *timeout,
				query:           *query,
				category:        *category,
				level:           *level,
				schedule:        *schedule,
				workArrangement: *workArrangement,
				skills:          *skills,
				country:         *country,
				salaryMin:       *salaryMin,
				salaryMax:       *salaryMax,
				postedAfter:     *postedAfter,
				sort:            *sortOrder,
				page:            *page,
				limit:           *limit,
				format:          *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	slug := detailFS.StringLong("slug", "", "job slug from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "fourdayweek detail --slug JOB-SLUG [--format text|json]",
		ShortHelp: "print one job in full",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --slug %q?)", args, args[0])
			}
			if *slug == "" {
				return errors.New("detail requires --slug")
			}
			return runDetail(ctx, detailFlags{timeout: *timeout, slug: *slug, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search or detail) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// jobSummaryJSON is the --format json shape for one job: the compact fields
// a listing needs, no description.
type jobSummaryJSON struct {
	Slug            string   `json:"slug"`
	Title           string   `json:"title"`
	Company         string   `json:"company,omitempty"`
	CompanySlug     string   `json:"companySlug,omitempty"`
	WorkArrangement string   `json:"workArrangement"`
	ScheduleType    string   `json:"scheduleType,omitempty"`
	Hours           string   `json:"hours,omitempty"`
	Level           string   `json:"level,omitempty"`
	ContractType    string   `json:"contractType,omitempty"`
	Salary          string   `json:"salary,omitempty"`
	Locations       []string `json:"locations,omitempty"`
	Timezones       []string `json:"timezones,omitempty"`
	PostedAt        string   `json:"postedAt"`
	URL             string   `json:"url"`
}

type resultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j fourdayweek.Job) jobSummaryJSON {
	s := jobSummaryJSON{
		Slug:            j.Slug,
		Title:           j.Title.Value,
		WorkArrangement: string(j.WorkArrangement),
		ScheduleType:    j.ScheduleType.Value,
		Hours:           formatHours(j),
		Level:           j.Level.Value,
		ContractType:    string(j.ContractType.Value),
		Salary:          formatSalary(j),
		Locations:       formatLocations(j),
		Timezones:       j.Timezones,
		PostedAt:        j.PostedAt.UTC().Format("2006-01-02"),
		URL:             j.URL.String(),
	}
	if j.Company.Set {
		s.Company = j.Company.Value.Name
		s.CompanySlug = j.Company.Value.Slug
	}
	return s
}

// formatHours renders the weekly hours range, collapsing an equal min and
// max to a single number.
func formatHours(j fourdayweek.Job) string {
	minHours, maxHours := j.HoursPerWeekMin, j.HoursPerWeekMax
	switch {
	case minHours.Set && maxHours.Set && minHours.Value == maxHours.Value:
		return fmt.Sprintf("%dh/week", minHours.Value)
	case minHours.Set && maxHours.Set:
		return fmt.Sprintf("%d-%dh/week", minHours.Value, maxHours.Value)
	case minHours.Set:
		return fmt.Sprintf("from %dh/week", minHours.Value)
	case maxHours.Set:
		return fmt.Sprintf("up to %dh/week", maxHours.Value)
	default:
		return ""
	}
}

// formatSalary renders the disclosed range in whole currency units, e.g.
// "USD 164595-212744 per year". The API reports the bounds in the currency's
// smallest unit, so they are divided by 100 here. A missing currency means
// no salary line at all, since neither bound renders meaningfully without it.
func formatSalary(j fourdayweek.Job) string {
	if !j.SalaryCurrency.Set {
		return ""
	}
	var bounds string
	switch {
	case j.SalaryMin.Set && j.SalaryMax.Set:
		bounds = fmt.Sprintf("%d-%d", j.SalaryMin.Value/100, j.SalaryMax.Value/100)
	case j.SalaryMin.Set:
		bounds = fmt.Sprintf("from %d", j.SalaryMin.Value/100)
	case j.SalaryMax.Set:
		bounds = fmt.Sprintf("up to %d", j.SalaryMax.Value/100)
	default:
		return ""
	}
	out := fmt.Sprintf("%s %s", j.SalaryCurrency.Value, bounds)
	if j.SalaryPeriod.Set {
		out += " per " + string(j.SalaryPeriod.Value)
	}
	return out
}

// formatLocations renders each location as "City, Country (arrangement)",
// keeping the per-location arrangement visible: a job filtered by --country
// can match on an onsite office while being remote-allowed somewhere else.
func formatLocations(j fourdayweek.Job) []string {
	out := make([]string, 0, len(j.Locations))
	for _, loc := range j.Locations {
		parts := make([]string, 0, 3)
		// An absent or null field decodes to the zero value, so one emptiness
		// check covers both.
		for _, p := range []fourdayweek.OptNilString{loc.City, loc.State, loc.Country} {
			if p.Value != "" {
				parts = append(parts, p.Value)
			}
		}
		if len(parts) == 0 {
			continue
		}
		place := strings.Join(parts, ", ")
		if loc.WorkArrangement.Value != "" {
			place += fmt.Sprintf(" (%s)", loc.WorkArrangement.Value)
		}
		out = append(out, place)
	}
	return out
}

// printSummary prints one job's compact text block (everything below the
// title line).
func printSummary(s jobSummaryJSON) {
	if s.Company != "" {
		fmt.Printf("Company: %s (%s)\n", s.Company, s.CompanySlug)
	}
	fmt.Printf("Arrangement: %s", s.WorkArrangement)
	if s.ScheduleType != "" {
		fmt.Printf(" | Schedule: %s", s.ScheduleType)
	}
	if s.Hours != "" {
		fmt.Printf(" | %s", s.Hours)
	}
	fmt.Println()
	if s.Level != "" || s.ContractType != "" {
		fmt.Printf("Level: %s | Contract: %s\n", orDash(s.Level), orDash(s.ContractType))
	}
	if s.Salary != "" {
		fmt.Printf("Salary: %s\n", s.Salary)
	}
	if len(s.Locations) > 0 {
		fmt.Printf("Locations: %s\n", strings.Join(s.Locations, "; "))
	}
	if len(s.Timezones) > 0 {
		fmt.Printf("Timezones: %s\n", strings.Join(s.Timezones, ", "))
	}
	fmt.Printf("Posted: %s\n", s.PostedAt)
	fmt.Printf("URL: %s\n", s.URL)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
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

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	timeout         time.Duration
	query           string
	category        string
	level           string
	schedule        string
	workArrangement string
	skills          string
	country         string
	salaryMin       int
	salaryMax       int
	postedAfter     int
	sort            string
	page            int
	limit           int
	format          string
}

// runSearch maps every flag directly onto the search endpoint's real
// server-side filters.
func runSearch(ctx context.Context, f searchFlags) error {
	if f.page < 1 {
		return fmt.Errorf("--page must be >= 1, got %d", f.page)
	}
	if f.limit < 1 || f.limit > 100 {
		return fmt.Errorf("--limit must be between 1 and 100, got %d", f.limit)
	}
	if f.postedAfter < 0 || f.postedAfter > 365 {
		return fmt.Errorf("--posted-after must be between 0 and 365, got %d", f.postedAfter)
	}
	if f.salaryMin < 0 || f.salaryMax < 0 {
		return fmt.Errorf("--salary-min and --salary-max must be >= 0, got %d and %d", f.salaryMin, f.salaryMax)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := fourdayweek.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	params := fourdayweek.SearchJobsParams{
		Page:  fourdayweek.NewOptInt(f.page),
		Limit: fourdayweek.NewOptInt(f.limit),
		Sort:  fourdayweek.NewOptSearchJobsSort(fourdayweek.SearchJobsSort(f.sort)),
	}
	if f.query != "" {
		params.Q = fourdayweek.NewOptString(f.query)
	}
	if f.category != "" {
		params.Category = fourdayweek.NewOptString(f.category)
	}
	if f.level != "" {
		params.Level = fourdayweek.NewOptString(f.level)
	}
	if f.schedule != "" {
		params.Schedule = fourdayweek.NewOptString(f.schedule)
	}
	if f.workArrangement != "" {
		params.WorkArrangement = fourdayweek.NewOptString(f.workArrangement)
	}
	if f.skills != "" {
		params.Skills = fourdayweek.NewOptString(f.skills)
	}
	if f.country != "" {
		params.Country = fourdayweek.NewOptString(f.country)
	}
	if f.salaryMin > 0 {
		params.SalaryMin = fourdayweek.NewOptInt(f.salaryMin)
	}
	if f.salaryMax > 0 {
		params.SalaryMax = fourdayweek.NewOptInt(f.salaryMax)
	}
	if f.postedAfter > 0 {
		params.PostedAfter = fourdayweek.NewOptInt(f.postedAfter)
	}

	res, err := client.SearchJobs(ctx, params)
	if err != nil {
		return err
	}

	switch d := res.(type) {
	case *fourdayweek.JobList:
		jobs := make([]jobSummaryJSON, len(d.Data))
		for i, j := range d.Data {
			jobs[i] = summarize(j)
		}
		return printResult(d.Total, jobs, f.format, "4dayweek.io Jobs")
	case *fourdayweek.ErrorResponse:
		return apiError(d)
	default:
		return fmt.Errorf("unexpected response %T", res)
	}
}

// detailFlags carries the parsed "detail" subcommand flags into runDetail.
type detailFlags struct {
	timeout time.Duration
	slug    string
	format  string
}

// runDetail prints one job in full. The endpoint returns the same shape a
// search result carries, so the only field this adds over a listing is the
// description.
func runDetail(ctx context.Context, f detailFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := fourdayweek.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetJob(ctx, fourdayweek.GetJobParams{Slug: f.slug})
	if err != nil {
		return err
	}

	switch d := res.(type) {
	case *fourdayweek.Job:
		return printDetail(*d, f.format)
	case *fourdayweek.ErrorResponse:
		return apiError(d)
	default:
		return fmt.Errorf("unexpected response %T", res)
	}
}

// detailJSON is the --format json shape for one job: the listing summary
// plus the description a search result already carries but omits here.
type detailJSON struct {
	jobSummaryJSON
	Description string   `json:"description,omitempty"`
	Skills      []string `json:"skills,omitempty"`
	Stack       []string `json:"stack,omitempty"`
	Tools       []string `json:"tools,omitempty"`
}

func printDetail(j fourdayweek.Job, format string) error {
	d := detailJSON{
		jobSummaryJSON: summarize(j),
		Description:    j.Description.Value,
		Skills:         tagNames(j.Skills),
		Stack:          tagNames(j.Stack),
		Tools:          tagNames(j.Tools),
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Title)
	printSummary(d.jobSummaryJSON)
	for _, tags := range []struct {
		label  string
		values []string
	}{{"Skills", d.Skills}, {"Stack", d.Stack}, {"Tools", d.Tools}} {
		if len(tags.values) > 0 {
			fmt.Printf("%s: %s\n", tags.label, strings.Join(tags.values, ", "))
		}
	}
	if d.Description != "" {
		// Descriptions are Markdown, so they print readably as-is.
		fmt.Printf("\n%s\n", d.Description)
	}
	return nil
}

func tagNames(tags []fourdayweek.Tag) []string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.Name
	}
	return out
}

func apiError(e *fourdayweek.ErrorResponse) error {
	if e.Error.Field.Value != "" {
		return fmt.Errorf("api error %d: %s (field %s)", e.Error.Code, e.Error.Message, e.Error.Field.Value)
	}
	return fmt.Errorf("api error %d: %s", e.Error.Code, e.Error.Message)
}
