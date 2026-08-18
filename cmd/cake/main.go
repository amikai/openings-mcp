package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	cake "github.com/amikai/openings-mcp/internal/provider/cake"
)

// main issues a single SearchJobs request built entirely from flags, then
// fetches GetJobDetail for every job the search returned.
func main() {
	os.Exit(run())
}

func run() int {
	fs := ff.NewFlagSet("cake")
	var (
		timeout        = fs.DurationLong("timeout", 60*time.Second, "request timeout")
		keyword        = fs.StringLong("keyword", "", "free-text keyword search (empty browses all jobs)")
		sortBy         = fs.StringEnumLong("sort", usageWithChoices("Sort order", choices(cake.JobSearchRequestSortBy("").AllValues())), choices(cake.JobSearchRequestSortBy("").AllValues())...)
		page           = fs.IntLong("page", 0, "1-based page number (0 = unset, server default)")
		perPage        = fs.IntLong("per-page", 10, "jobs per page (0 = unset, server default 20)")
		locations      = fs.StringSetLong("location", "Location name as shown on cake.me, e.g. Taiwan (repeatable)")
		professions    = fs.StringSetLong("profession", "Profession slug, e.g. it_back-end-engineer (repeatable)")
		jobTypes       = fs.StringSetLong("job-type", "Employment type, e.g. full_time, part_time (repeatable)")
		seniorities    = fs.StringSetLong("seniority", "Seniority level, e.g. mid_senior_level, entry_level (repeatable)")
		years          = fs.StringSetLong("years", "Years of experience bucket, e.g. 1_3, 3_5 (repeatable)")
		managements    = fs.StringSetLong("management", "Number of people managed, e.g. none, one_five (repeatable)")
		remotes        = fs.StringSetLong("remote", "Remote-work policy, e.g. full_remote_work, partial_remote_work (repeatable)")
		inclusivities  = fs.StringSetLong("inclusivity", "Inclusive-hiring trait, e.g. lgbtq, foreign_talents (repeatable)")
		langs          = fs.StringSetLong("lang", "Job description language, e.g. English, Chinese (repeatable)")
		salaryType     = fs.StringLong("salary-type", "", "Salary period, e.g. per_month, per_year")
		salaryCurrency = fs.StringLong("salary-currency", "", "Salary currency, e.g. TWD, USD")
		salaryMin      = fs.IntLong("salary-min", 0, "minimum salary (0 = unset)")
		salaryMax      = fs.IntLong("salary-max", 0, "maximum salary (0 = unset)")
		companySizes   = fs.StringSetLong("company-size", "Company size bucket, e.g. 51_200, 5001_ (repeatable)")
		sectors        = fs.StringSetLong("sector", "Company sector slug, e.g. tech_software (repeatable)")
		techLabels     = fs.StringSetLong("tech-label", "Technology the company uses, e.g. go (repeatable)")
	)
	if err := ff.Parse(fs, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Flags(fs))
		if errors.Is(err, ff.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}

	f := searchFlags{
		keyword:        *keyword,
		sort:           *sortBy,
		page:           *page,
		perPage:        *perPage,
		locations:      *locations,
		professions:    *professions,
		jobTypes:       *jobTypes,
		seniorities:    *seniorities,
		years:          *years,
		managements:    *managements,
		remotes:        *remotes,
		inclusivities:  *inclusivities,
		langs:          *langs,
		salaryType:     *salaryType,
		salaryCurrency: *salaryCurrency,
		salaryMin:      *salaryMin,
		salaryMax:      *salaryMax,
		companySizes:   *companySizes,
		sectors:        *sectors,
		techLabels:     *techLabels,
	}
	req, err := buildSearchRequest(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client, err := cake.NewClient("https://api.cake.me")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	search, err := client.SearchJobs(ctx, &req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	details := make(map[string]*cake.JobDetail, len(search.Data))
	for _, job := range search.Data {
		detail, err := client.GetJobDetail(ctx, cake.GetJobDetailParams{Path: job.Path})
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.Path, err)
			return 1
		}
		details[job.Path] = detail
	}

	writeReport(os.Stdout, f.keyword, search, details)
	return 0
}

// searchFlags carries the parsed flag values into buildSearchRequest.
type searchFlags struct {
	keyword        string
	sort           string
	page           int
	perPage        int
	locations      []string
	professions    []string
	jobTypes       []string
	seniorities    []string
	years          []string
	managements    []string
	remotes        []string
	inclusivities  []string
	langs          []string
	salaryType     string
	salaryCurrency string
	salaryMin      int
	salaryMax      int
	companySizes   []string
	sectors        []string
	techLabels     []string
}

// buildSearchRequest converts flag values into the API request. Enum flags
// are validated against the generated enum types; empty or zero values leave
// the corresponding field unset (unfiltered).
func buildSearchRequest(f searchFlags) (cake.JobSearchRequest, error) {
	req := cake.JobSearchRequest{
		Query:  f.keyword,
		SortBy: cake.JobSearchRequestSortBy(f.sort),
	}
	if f.page != 0 {
		req.Page = cake.NewOptInt(f.page)
	}
	if f.perPage != 0 {
		req.PerPage = cake.NewOptInt(f.perPage)
	}

	req.Filters.Locations = f.locations
	req.Filters.Professions = f.professions
	req.Filters.LangNames = f.langs
	req.Filters.JobTypes = f.jobTypes
	req.Filters.SeniorityLevels = f.seniorities
	req.Filters.YearOfSeniority = f.years
	req.Filters.NumberOfManagement = f.managements
	req.Filters.Remote = f.remotes
	req.Filters.InclusivityTraits = f.inclusivities

	if f.salaryType != "" || f.salaryCurrency != "" || f.salaryMin != 0 || f.salaryMax != 0 {
		salary := cake.JobSearchFiltersSalary{}
		if f.salaryType != "" {
			salary.Type = cake.NewOptString(f.salaryType)
		}
		if f.salaryCurrency != "" {
			salary.Currency = cake.NewOptString(f.salaryCurrency)
		}
		if f.salaryMin != 0 {
			salary.Min = cake.NewOptInt(f.salaryMin)
		}
		if f.salaryMax != 0 {
			salary.Max = cake.NewOptInt(f.salaryMax)
		}
		req.Filters.Salary = cake.NewOptJobSearchFiltersSalary(salary)
	}

	if len(f.companySizes) > 0 || len(f.sectors) > 0 || len(f.techLabels) > 0 {
		page := cake.JobSearchFiltersPage{
			NumberOfEmployees: f.companySizes,
			Sectors:           f.sectors,
			TechLabels:        f.techLabels,
		}
		req.Filters.Page = cake.NewOptJobSearchFiltersPage(page)
	}

	return req, nil
}

// choices converts a generated enum's AllValues into flag choice strings.
func choices[T ~string](values []T) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, string(v))
	}
	return out
}

// usageWithChoices appends a comma-separated "one of: ..." list to base.
func usageWithChoices(base string, choices []string) string {
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}

func writeReport(w io.Writer, keyword string, search *cake.JobSearchResponse, details map[string]*cake.JobDetail) {
	fmt.Fprintf(w, "Cake Jobs Report\n")
	fmt.Fprintf(w, "Keyword: %s\n", keyword)
	fmt.Fprintf(w, "Found %d jobs (page %d/%d); showing %d\n\n", search.TotalEntries.Value, search.CurrentPage.Value, search.TotalPages.Value, len(search.Data))

	for i, job := range search.Data {
		fmt.Fprintf(w, "%d. [%s] %s\n", i+1, job.Path, job.Title.Value)
		if detail := details[job.Path]; detail != nil {
			writeDetail(w, detail)
		}
		fmt.Fprintln(w)
	}
}

func writeDetail(w io.Writer, detail *cake.JobDetail) {
	fmt.Fprintf(w, "URL: https://www.cake.me/companies/%s/jobs/%s\n", detail.PagePath.Value, detail.Path.Value)
	description, err := html2text.FromString(detail.Description.Value, html2text.Options{})
	if err != nil {
		description = detail.Description.Value
	}
	if description != "" {
		fmt.Fprintf(w, "Description:\n%s\n", description)
	}
	requirements, err := html2text.FromString(detail.Requirements.Value, html2text.Options{})
	if err != nil {
		requirements = detail.Requirements.Value
	}
	if requirements != "" {
		fmt.Fprintf(w, "Requirements: %s\n", requirements)
	}
}
