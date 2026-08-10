package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	workable "github.com/amikai/openings-mcp/internal/provider/workable"
)

// apiBaseURL is the origin behind Workable-hosted careers pages — the single
// production server in the provider's openapi.yaml (paths carry the
// /api/v3 and /api/v2 prefixes).
const apiBaseURL = "https://apply.workable.com"

func main() {
	rootFlags := ff.NewFlagSet("workable")
	var (
		company = rootFlags.StringLong("company", "", `Workable account subdomain from the careers URL, e.g. "blueground" in apply.workable.com/blueground`)
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "workable",
		Usage: "workable --company COMPANY [FLAGS] <companies|search|get|filters> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "workable companies [--format text|json]",
		ShortHelp: "list curated Workable companies (company name and account subdomain)",
		Flags:     companiesFlags,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			return runCompanies(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword    = searchFS.StringLong("keyword", "", "server-side keyword search over title and posting body text")
		country    = searchFS.StringLong("country", "", `country name as the filters facets list it, e.g. "Greece"`)
		region     = searchFS.StringLong("region", "", `state/region name, e.g. "Attica"`)
		city       = searchFS.StringLong("city", "", "city name")
		department = searchFS.IntLong("department", 0, "numeric department id from 'workable filters' (not the display name)")
		workplace  = searchFS.StringLong("workplace", "", "on_site, hybrid, or remote")
		worktype   = searchFS.StringLong("worktype", "", "full, part, contract, or temporary")
		remote     = searchFS.StringLong("remote", "", "true or false")
		token      = searchFS.StringLong("token", "", "nextPage cursor from the previous page (page size is a fixed 10)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "workable --company COMPANY search [--keyword TEXT] [--country C] [--region R] [--city CITY] [--department ID] [--workplace W] [--worktype W] [--remote true|false] [--token CURSOR] [--format text|json]",
		ShortHelp: "search jobs for a company (server-side filters, cursor pagination)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				company:    *company,
				timeout:    *timeout,
				keyword:    *keyword,
				country:    *country,
				region:     *region,
				city:       *city,
				department: *department,
				workplace:  *workplace,
				worktype:   *worktype,
				remote:     *remote,
				token:      *token,
				format:     *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	getFS := ff.NewFlagSet("get").SetParent(rootFlags)
	shortcode := getFS.StringLong("shortcode", "", "job shortcode from a search result")
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "workable --company COMPANY get --shortcode SHORTCODE [--format text|json]",
		ShortHelp: "print one job in full (description, requirements, benefits, public URL)",
		Flags:     getFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v (did you mean --shortcode %q?)", args, args[0])
			}
			return runGet(ctx, getFlags{company: *company, timeout: *timeout, shortcode: *shortcode, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, getCmd)

	filtersFlags := ff.NewFlagSet("filters").SetParent(rootFlags)
	filtersCmd := &ff.Command{
		Name:      "filters",
		Usage:     "workable --company COMPANY filters [--format text|json]",
		ShortHelp: "list a company's search facets (locations, department ids, worktypes, workplaces)",
		Flags:     filtersFlags,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("filters takes no positional arguments, got %v", args)
			}
			return runFilters(ctx, *company, *timeout, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, filtersCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, get, or filters) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// normalizeCompany requires --company to be a curated company — same policy
// as cmd/smartrecruiters — and returns the roster's account subdomain.
func normalizeCompany(company string) (string, error) {
	if company == "" {
		return "", errors.New("--company is required")
	}
	c, ok := workable.CompaniesByAccount[strings.ToLower(company)]
	if !ok {
		return "", fmt.Errorf("company %q not found; run 'workable companies' to see supported companies", company)
	}
	return c.Account, nil
}

// runCompanies lists every curated Workable company embedded in the CLI
// (internal/provider/workable/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	cs := workable.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.Account)
	}
	return nil
}

// jobSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs, no posting body.
type jobSummaryJSON struct {
	Shortcode  string `json:"shortcode"`
	Title      string `json:"title"`
	Location   string `json:"location,omitempty"`
	Department string `json:"department,omitempty"`
	Workplace  string `json:"workplace,omitempty"`
	PostedAt   string `json:"postedAt,omitempty"`
	URL        string `json:"url"`
}

type searchResultJSON struct {
	Total     int              `json:"total"`
	Jobs      []jobSummaryJSON `json:"jobs"`
	NextToken string           `json:"nextToken,omitempty"`
}

// jobURL builds the human-clickable posting page; no API response field
// carries it.
func jobURL(account, shortcode string) string {
	return fmt.Sprintf("https://apply.workable.com/%s/j/%s/", account, shortcode)
}

func summarize(account string, j workable.JobSummary) jobSummaryJSON {
	s := jobSummaryJSON{
		Shortcode: j.Shortcode,
		Title:     j.Title,
		Workplace: string(j.Workplace.Value),
		URL:       jobURL(account, j.Shortcode),
	}
	if loc, ok := j.Location.Get(); ok {
		if d, ok := loc.Display.Get(); ok && d != "" {
			s.Location = d
		} else {
			parts := []string{}
			for _, p := range []string{loc.City.Or(""), loc.Region.Or(""), loc.Country.Or("")} {
				if p != "" {
					parts = append(parts, p)
				}
			}
			s.Location = strings.Join(parts, ", ")
		}
	}
	if len(j.Department) > 0 {
		s.Department = strings.Join(j.Department, ", ")
	}
	if v, ok := j.Published.Get(); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			s.PostedAt = t.UTC().Format("2006-01-02")
		} else {
			s.PostedAt = v
		}
	}
	return s
}

// printSummary prints one job's compact text block (everything below the
// title line).
func printSummary(s jobSummaryJSON) {
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.Workplace != "" {
		fmt.Printf("Workplace: %s\n", s.Workplace)
	}
	if s.Department != "" {
		fmt.Printf("Department: %s\n", s.Department)
	}
	if s.PostedAt != "" {
		fmt.Printf("Posted: %s\n", s.PostedAt)
	}
	fmt.Printf("Shortcode: %s\n", s.Shortcode)
	fmt.Printf("URL: %s\n", s.URL)
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	company    string
	timeout    time.Duration
	keyword    string
	country    string
	region     string
	city       string
	department int
	workplace  string
	worktype   string
	remote     string
	token      string
	format     string
}

// runSearch maps every flag onto the job board API's real server-side
// filters. Pagination is cursor-only: rerun with --token set to the
// previous output's next-page cursor.
func runSearch(ctx context.Context, f searchFlags) error {
	account, err := normalizeCompany(f.company)
	if err != nil {
		return err
	}
	if f.workplace != "" && !slices.Contains([]string{"on_site", "hybrid", "remote"}, f.workplace) {
		return fmt.Errorf("--workplace must be on_site, hybrid, or remote, got %q", f.workplace)
	}
	if f.remote != "" && f.remote != "true" && f.remote != "false" {
		return fmt.Errorf("--remote must be true or false, got %q", f.remote)
	}
	if f.department < 0 {
		return fmt.Errorf("--department must be a positive facet id, got %d", f.department)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := workable.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	req := workable.SearchRequest{}
	if f.keyword != "" {
		req.Query = workable.NewOptString(f.keyword)
	}
	if f.country != "" || f.region != "" || f.city != "" {
		loc := workable.LocationFilter{}
		if f.country != "" {
			loc.Country = workable.NewOptString(f.country)
		}
		if f.region != "" {
			loc.Region = workable.NewOptString(f.region)
		}
		if f.city != "" {
			loc.City = workable.NewOptString(f.city)
		}
		req.Location = []workable.LocationFilter{loc}
	}
	if f.department > 0 {
		req.Department = []int{f.department}
	}
	if f.workplace != "" {
		req.Workplace = []workable.SearchRequestWorkplaceItem{workable.SearchRequestWorkplaceItem(f.workplace)}
	}
	if f.worktype != "" {
		req.Worktype = []string{f.worktype}
	}
	if f.remote != "" {
		req.Remote = []string{f.remote}
	}
	if f.token != "" {
		req.Token = workable.NewOptString(f.token)
	}

	res, err := client.SearchJobs(ctx, &req, workable.SearchJobsParams{Account: account})
	if err != nil {
		return err
	}

	page, ok := res.(*workable.SearchResponse)
	if !ok {
		return fmt.Errorf("company %q not found on Workable (account removed?)", account)
	}

	jobs := make([]jobSummaryJSON, len(page.Results))
	for i, j := range page.Results {
		jobs[i] = summarize(account, j)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: page.Total, Jobs: jobs, NextToken: page.NextPage.Value})
	}

	fmt.Printf("Workable Jobs Report (company: %s)\n", account)
	fmt.Printf("Found %d jobs; showing %d\n\n", page.Total, len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	if v, ok := page.NextPage.Get(); ok {
		fmt.Printf("Next page: --token %s\n", v)
	}
	return nil
}

// runFilters dumps the account's facets — most usefully the numeric
// department ids that search's --department flag requires.
func runFilters(ctx context.Context, company string, timeout time.Duration, format string) error {
	account, err := normalizeCompany(company)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := workable.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.ListJobFilters(ctx, workable.ListJobFiltersParams{Account: account})
	if err != nil {
		return err
	}

	facets, ok := res.(*workable.FiltersResponse)
	if !ok {
		return fmt.Errorf("company %q not found on Workable (account removed?)", account)
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(facets)
	}

	fmt.Printf("Facets (company: %s)\n\n", account)
	fmt.Println("Departments:")
	for _, d := range facets.Departments {
		fmt.Printf("  %d  %s (%d jobs)\n", d.ID, d.Name, d.Count.Value)
	}
	fmt.Println("Locations:")
	for _, l := range facets.Locations {
		label := l.Display.Or("")
		if label == "" {
			// display is account-inconsistent (see openapi.yaml); rebuild it
			// from the structured fields.
			parts := []string{}
			for _, p := range []string{l.Country.Or(""), l.Region.Or(""), l.City.Or("")} {
				if p != "" {
					parts = append(parts, p)
				}
			}
			label = strings.Join(parts, ", ")
		}
		fmt.Printf("  %s\n", label)
	}
	fmt.Printf("Worktypes: %s\n", strings.Join(facets.Worktypes, ", "))
	fmt.Printf("Workplaces: %s\n", strings.Join(facets.Workplaces, ", "))
	return nil
}

// getFlags carries the parsed "get" subcommand flags into runGet.
type getFlags struct {
	company   string
	timeout   time.Duration
	shortcode string
	format    string
}

// runGet fetches one job in full via the v2 detail endpoint, which — unlike
// search — 404s for an unknown shortcode.
func runGet(ctx context.Context, f getFlags) error {
	account, err := normalizeCompany(f.company)
	if err != nil {
		return err
	}
	if f.shortcode == "" {
		return errors.New("--shortcode is required (take it from a search result's Shortcode)")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := workable.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetJob(ctx, workable.GetJobParams{Account: account, Shortcode: f.shortcode})
	if err != nil {
		return err
	}

	switch d := res.(type) {
	case *workable.JobDetail:
		return printDetail(account, d, f.format)
	case *workable.NotFound:
		return fmt.Errorf("job %q not found for company %q", f.shortcode, account)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}

// printDetail renders one full job. JSON mode encodes the generated
// JobDetail as-is — detail is for seeing the whole record.
func printDetail(account string, d *workable.JobDetail, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Title)
	if c, ok := workable.CompaniesByAccount[account]; ok {
		fmt.Printf("Company: %s\n", c.Name)
	}
	if loc, ok := d.Location.Get(); ok {
		if label, ok := loc.Display.Get(); ok && label != "" {
			fmt.Printf("Location: %s\n", label)
		} else {
			parts := []string{}
			for _, p := range []string{loc.City.Or(""), loc.Region.Or(""), loc.Country.Or("")} {
				if p != "" {
					parts = append(parts, p)
				}
			}
			if len(parts) > 0 {
				fmt.Printf("Location: %s\n", strings.Join(parts, ", "))
			}
		}
	}
	if w, ok := d.Workplace.Get(); ok {
		fmt.Printf("Workplace: %s\n", w)
	}
	if v, ok := d.Published.Get(); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			fmt.Printf("Posted: %s\n", t.UTC().Format("2006-01-02"))
		}
	}
	fmt.Printf("URL: %s\n", jobURL(account, d.Shortcode))

	printSection("Description", d.Description)
	printSection("Requirements", d.Requirements)
	printSection("Benefits", d.Benefits)
	return nil
}

// printSection renders one of the three HTML body fields as plain text,
// skipping fields the posting leaves empty. Falls back to the raw HTML on a
// conversion failure rather than dropping the section.
func printSection(title string, opt workable.OptString) {
	html, ok := opt.Get()
	if !ok || html == "" {
		return
	}
	rendered, err := html2text.FromString(html, html2text.Options{})
	if err != nil {
		rendered = html
	}
	fmt.Printf("\n%s:\n%s\n", title, rendered)
}
