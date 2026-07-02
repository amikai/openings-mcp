package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	cake "github.com/amikai/job-mcp/internal/provider/cake"
)

var tagRE = regexp.MustCompile(`<[^>]+>`)

// main issues a single SearchJobs request built entirely from flags, then
// fetches GetJobDetail for every job the search returned.
func main() {
	fs := ff.NewFlagSet("cake")
	var (
		timeout        = fs.DurationLong("timeout", 60*time.Second, "request timeout")
		keyword        = fs.StringLong("keyword", "", "free-text keyword search (empty browses all jobs)")
		sortBy         = fs.StringEnumLong("sort", usageWithChoices("Sort order", choices(cake.JobSearchRequestSortBy("").AllValues())), choices(cake.JobSearchRequestSortBy("").AllValues())...)
		page           = fs.IntLong("page", 0, "1-based page number (0 = unset, server default)")
		perPage        = fs.IntLong("per-page", 10, "jobs per page (0 = unset, server default 20)")
		locations      = fs.StringSetLong("location", "Location name as shown on cake.me, e.g. Taiwan (repeatable)")
		professions    = fs.StringSetLong("profession", "Profession slug, e.g. it_back-end-engineer (repeatable)")
		jobTypes       = fs.StringSetLong("job-type", usageWithChoices("Employment type (repeatable)", choices(cake.JobSearchFiltersJobTypesItem("").AllValues())))
		seniorities    = fs.StringSetLong("seniority", usageWithChoices("Seniority level (repeatable)", choices(cake.JobSearchFiltersSeniorityLevelsItem("").AllValues())))
		years          = fs.StringSetLong("years", usageWithChoices("Years of experience bucket (repeatable)", choices(cake.JobSearchFiltersYearOfSeniorityItem("").AllValues())))
		managements    = fs.StringSetLong("management", usageWithChoices("Number of people managed (repeatable)", choices(cake.JobSearchFiltersNumberOfManagementItem("").AllValues())))
		remotes        = fs.StringSetLong("remote", usageWithChoices("Remote-work policy (repeatable)", choices(cake.JobSearchFiltersRemoteItem("").AllValues())))
		inclusivities  = fs.StringSetLong("inclusivity", usageWithChoices("Inclusive-hiring trait (repeatable)", choices(cake.JobSearchFiltersInclusivityTraitsItem("").AllValues())))
		langs          = fs.StringSetLong("lang", "Job description language, e.g. English, Chinese (repeatable)")
		salaryType     = fs.StringEnumLong("salary-type", usageWithChoices("Salary period", choices(cake.JobSearchFiltersSalaryType("").AllValues())), enumChoices(cake.JobSearchFiltersSalaryType("").AllValues())...)
		salaryCurrency = fs.StringEnumLong("salary-currency", usageWithChoices("Salary currency", choices(cake.JobSearchFiltersSalaryCurrency("").AllValues())), enumChoices(cake.JobSearchFiltersSalaryCurrency("").AllValues())...)
		salaryMin      = fs.IntLong("salary-min", 0, "minimum salary (0 = unset)")
		salaryMax      = fs.IntLong("salary-max", 0, "maximum salary (0 = unset)")
		companySizes   = fs.StringSetLong("company-size", usageWithChoices("Company size bucket (repeatable)", choices(cake.JobSearchFiltersPageNumberOfEmployeesItem("").AllValues())))
		sectors        = fs.StringSetLong("sector", "Company sector slug, e.g. tech_software (repeatable)")
		techLabels     = fs.StringSetLong("tech-label", "Technology the company uses, e.g. go (repeatable)")
	)
	if err := ff.Parse(fs, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Flags(fs))
		if errors.Is(err, ff.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
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
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client, err := cake.NewClient("https://api.cake.me")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	searchRes, err := client.SearchJobs(ctx, &req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	search, ok := searchRes.(*cake.JobSearchResponse)
	if !ok {
		fmt.Fprintf(os.Stderr, "search returned %T\n", searchRes)
		os.Exit(1)
	}

	details := make(map[string]*cake.JobDetail, len(search.Data))
	for _, job := range search.Data {
		detailRes, err := client.GetJobDetail(ctx, cake.GetJobDetailParams{Path: job.Path})
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.Path, err)
			os.Exit(1)
		}
		detail, ok := detailRes.(*cake.JobDetail)
		if !ok {
			fmt.Fprintf(os.Stderr, "job detail %s returned %T\n", job.Path, detailRes)
			os.Exit(1)
		}
		details[job.Path] = detail
	}

	writeReport(os.Stdout, f.keyword, search, details)
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

	var err error
	if req.Filters.JobTypes, err = toEnums[cake.JobSearchFiltersJobTypesItem](f.jobTypes, "--job-type"); err != nil {
		return req, err
	}
	if req.Filters.SeniorityLevels, err = toEnums[cake.JobSearchFiltersSeniorityLevelsItem](f.seniorities, "--seniority"); err != nil {
		return req, err
	}
	if req.Filters.YearOfSeniority, err = toEnums[cake.JobSearchFiltersYearOfSeniorityItem](f.years, "--years"); err != nil {
		return req, err
	}
	if req.Filters.NumberOfManagement, err = toEnums[cake.JobSearchFiltersNumberOfManagementItem](f.managements, "--management"); err != nil {
		return req, err
	}
	if req.Filters.Remote, err = toEnums[cake.JobSearchFiltersRemoteItem](f.remotes, "--remote"); err != nil {
		return req, err
	}
	if req.Filters.InclusivityTraits, err = toEnums[cake.JobSearchFiltersInclusivityTraitsItem](f.inclusivities, "--inclusivity"); err != nil {
		return req, err
	}

	if f.salaryType != "" || f.salaryCurrency != "" || f.salaryMin != 0 || f.salaryMax != 0 {
		salary := cake.JobSearchFiltersSalary{}
		if f.salaryType != "" {
			salary.Type = cake.NewOptJobSearchFiltersSalaryType(cake.JobSearchFiltersSalaryType(f.salaryType))
		}
		if f.salaryCurrency != "" {
			salary.Currency = cake.NewOptJobSearchFiltersSalaryCurrency(cake.JobSearchFiltersSalaryCurrency(f.salaryCurrency))
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
		page := cake.JobSearchFiltersPage{Sectors: f.sectors, TechLabels: f.techLabels}
		if page.NumberOfEmployees, err = toEnums[cake.JobSearchFiltersPageNumberOfEmployeesItem](f.companySizes, "--company-size"); err != nil {
			return req, err
		}
		req.Filters.Page = cake.NewOptJobSearchFiltersPage(page)
	}

	return req, nil
}

// toEnums validates each value against T's enum and converts it. A nil or
// empty input returns nil, leaving the filter unset.
func toEnums[T interface {
	~string
	AllValues() []T
}](values []string, flag string) ([]T, error) {
	if len(values) == 0 {
		return nil, nil
	}
	var zero T
	all := zero.AllValues()
	valid := make(map[string]bool, len(all))
	for _, v := range all {
		valid[string(v)] = true
	}
	out := make([]T, 0, len(values))
	for _, v := range values {
		if !valid[v] {
			return nil, fmt.Errorf("%s: unknown value %q, one of: %s", flag, v, strings.Join(choices(all), " | "))
		}
		out = append(out, T(v))
	}
	return out, nil
}

// choices converts a generated enum's AllValues into flag choice strings.
func choices[T ~string](values []T) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, string(v))
	}
	return out
}

// enumChoices is choices prefixed with "" so an ff.StringEnum flag can
// default to unset (no filter) instead of silently falling back to the
// first real value — ffval.Enum's zero Default only survives initialize()
// if it's itself in the Valid list.
func enumChoices[T ~string](values []T) []string {
	return append([]string{""}, choices(values)...)
}

// usageWithChoices appends a comma-separated "one of: ..." list to base.
func usageWithChoices(base string, choices []string) string {
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}

func writeReport(w io.Writer, keyword string, search *cake.JobSearchResponse, details map[string]*cake.JobDetail) {
	fmt.Fprintf(w, "Cake Jobs Report\n")
	fmt.Fprintf(w, "Keyword: %s\n", keyword)
	fmt.Fprintf(w, "Found %d jobs (page %d/%d); showing %d\n\n", search.TotalEntries, search.CurrentPage, search.TotalPages, len(search.Data))

	for i, job := range search.Data {
		fmt.Fprintf(w, "%d. [%s] %s\n", i+1, job.Path, job.Title)
		if detail := details[job.Path]; detail != nil {
			writeDetail(w, detail)
		}
		fmt.Fprintln(w)
	}
}

func writeDetail(w io.Writer, detail *cake.JobDetail) {
	fmt.Fprintf(w, "URL: https://www.cake.me/companies/%s/jobs/%s\n", detail.PagePath, detail.Path)
	description := plainText(detail.Description)
	if description != "" {
		fmt.Fprintf(w, "Description:\n%s\n", description)
	}
	requirements := plainText(detail.Requirements)
	if requirements != "" {
		fmt.Fprintf(w, "Requirements: %s\n", requirements)
	}
}

func plainText(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Fields(s)
	return strings.Join(lines, " ")
}
