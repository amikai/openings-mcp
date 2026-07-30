// Package cake implements the "openings-mcp cake" debug CLI, for manual
// checks against the live surface that internal/provider/cake documents.
package cake

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	cake "github.com/amikai/openings-mcp/internal/provider/cake"
)

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
	timeout        time.Duration
}

// NewCommand returns a cobra.Command for cake.
func NewCommand() *cobra.Command {
	var f searchFlags

	cmd := &cobra.Command{
		Use:          "cake",
		Short:        "Search Cake.me jobs and fetch details",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), f)
		},
	}

	cmd.Flags().DurationVar(&f.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.Flags().StringVar(&f.keyword, "keyword", "", "free-text keyword search (empty browses all jobs)")
	// sort_by is required by the API (see openapi.yaml), so it defaults to a
	// real value rather than to unset — an empty sort_by is a 422.
	clihelp.ChoiceVar(cmd.Flags(), &f.sort, "sort", string(cake.JobSearchRequestSortByPopularity),
		choices(cake.JobSearchRequestSortBy("").AllValues()), "Sort order")
	cmd.Flags().IntVar(&f.page, "page", 0, "1-based page number (0 = unset, server default)")
	cmd.Flags().IntVar(&f.perPage, "per-page", 10, "jobs per page (0 = unset, server default 20)")
	cmd.Flags().StringArrayVar(&f.locations, "location", nil, "Location name as shown on cake.me, e.g. Taiwan (repeatable)")
	cmd.Flags().StringArrayVar(&f.professions, "profession", nil, "Profession slug, e.g. it_back-end-engineer (repeatable)")
	cmd.Flags().StringArrayVar(&f.jobTypes, "job-type", nil, usageWithChoices("Employment type (repeatable)", choices(cake.JobSearchFiltersJobTypesItem("").AllValues())))
	cmd.Flags().StringArrayVar(&f.seniorities, "seniority", nil, usageWithChoices("Seniority level (repeatable)", choices(cake.JobSearchFiltersSeniorityLevelsItem("").AllValues())))
	cmd.Flags().StringArrayVar(&f.years, "years", nil, usageWithChoices("Years of experience bucket (repeatable)", choices(cake.JobSearchFiltersYearOfSeniorityItem("").AllValues())))
	cmd.Flags().StringArrayVar(&f.managements, "management", nil, usageWithChoices("Number of people managed (repeatable)", choices(cake.JobSearchFiltersNumberOfManagementItem("").AllValues())))
	cmd.Flags().StringArrayVar(&f.remotes, "remote", nil, usageWithChoices("Remote-work policy (repeatable)", choices(cake.JobSearchFiltersRemoteItem("").AllValues())))
	cmd.Flags().StringArrayVar(&f.inclusivities, "inclusivity", nil, usageWithChoices("Inclusive-hiring trait (repeatable)", choices(cake.JobSearchFiltersInclusivityTraitsItem("").AllValues())))
	cmd.Flags().StringArrayVar(&f.langs, "lang", nil, "Job description language, e.g. English, Chinese (repeatable)")
	clihelp.ChoiceVar(cmd.Flags(), &f.salaryType, "salary-type", "",
		clihelp.WithUnset(choices(cake.JobSearchFiltersSalaryType("").AllValues())), "Salary period")
	clihelp.ChoiceVar(cmd.Flags(), &f.salaryCurrency, "salary-currency", "",
		clihelp.WithUnset(choices(cake.JobSearchFiltersSalaryCurrency("").AllValues())), "Salary currency")
	cmd.Flags().IntVar(&f.salaryMin, "salary-min", 0, "minimum salary (0 = unset)")
	cmd.Flags().IntVar(&f.salaryMax, "salary-max", 0, "maximum salary (0 = unset)")
	cmd.Flags().StringArrayVar(&f.companySizes, "company-size", nil, usageWithChoices("Company size bucket (repeatable)", choices(cake.JobSearchFiltersPageNumberOfEmployeesItem("").AllValues())))
	cmd.Flags().StringArrayVar(&f.sectors, "sector", nil, "Company sector slug, e.g. tech_software (repeatable)")
	cmd.Flags().StringArrayVar(&f.techLabels, "tech-label", nil, "Technology the company uses, e.g. go (repeatable)")

	return cmd
}

func run(ctx context.Context, f searchFlags) error {
	req, err := buildSearchRequest(f)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := cake.NewClient(cake.DefaultBaseURL)
	if err != nil {
		return err
	}
	search, err := client.SearchJobs(ctx, &req)
	if err != nil {
		return err
	}

	details := make(map[string]*cake.JobDetail, len(search.Data))
	for _, job := range search.Data {
		detail, err := client.GetJobDetail(ctx, cake.GetJobDetailParams{Path: job.Path})
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.Path, err)
			return err
		}
		details[job.Path] = detail
	}

	writeReport(os.Stdout, f.keyword, search, details)
	return nil
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
