// Command dayforce is a debug CLI for the Dayforce (Ceridian) candidate
// portal API, driving internal/provider/dayforce against the curated
// roster in companies.yaml.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	dayforce "github.com/amikai/openings-mcp/internal/provider/dayforce"
)

// apiBaseURL is the Dayforce candidate portal origin — the single server
// in the provider's openapi.yaml.
const _apiBaseURL = "https://jobs.dayforcehcm.com"

// pageSize is the fixed upstream page size; --page is converted to
// paginationStart against this constant.
const _pageSize = 25

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("dayforce")
	var (
		company = rootFlags.StringLong("company", "", "curated dayforce company slug, e.g. \"pca\" (see the companies subcommand)")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "dayforce",
		Usage: "dayforce --company SLUG [FLAGS] <companies|search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "dayforce companies [--format text|json]",
		ShortHelp: "list curated dayforce companies (slug, company name, namespace, board)",
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
		keyword  = searchFS.StringLong("keyword", "", "free-text keyword search")
		location = searchFS.StringLong("location", "", "free-text location string")
		page     = searchFS.IntLong("page", 1, "1-based page number; page size is fixed upstream")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "dayforce --company SLUG search [--keyword TEXT] [--location TEXT] [--page N] [--format text|json]",
		ShortHelp: "search postings for a company (server-side filters)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				company:  *company,
				timeout:  *timeout,
				keyword:  *keyword,
				location: *location,
				page:     *page,
				format:   *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	postingID := detailFS.IntLong("id", 0, "jobPostingId from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "dayforce --company SLUG detail --id POSTING-ID [--format text|json]",
		ShortHelp: "print one posting in full (description sections)",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %s?)", args, args[0])
			}
			return runDetail(ctx, detailFlags{company: *company, timeout: *timeout, postingID: *postingID, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, or detail) is required")
		return 1
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

// normalizeCompany requires --company to be a curated company, matching
// cmd/smartrecruiters's --company policy, and returns the roster entry
// (namespace, job board code/id, culture) so callers need not re-look it up.
func normalizeCompany(company string) (dayforce.Company, error) {
	if company == "" {
		return dayforce.Company{}, errors.New("--company is required")
	}
	c, ok := dayforce.CompaniesBySlug[company]
	if !ok {
		return dayforce.Company{}, fmt.Errorf("company %q not found; run 'dayforce companies' to see supported companies", company)
	}
	return c, nil
}

// runCompanies lists every curated Dayforce board embedded in the CLI
// (internal/provider/dayforce/companies.yaml), sorted by company name. It
// makes no network call.
func runCompanies(format string) error {
	cs := dayforce.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s) [namespace: %s, board: %s]\n", c.Name, c.Slug(), c.Namespace, c.JobBoardCode)
	}
	return nil
}

// postingSummaryJSON is the --format json shape for one search result.
type postingSummaryJSON struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location,omitempty"`
	Remote   bool   `json:"remote,omitempty"`
}

type searchResultJSON struct {
	MaxCount int                  `json:"maxCount"`
	Offset   int                  `json:"offset"`
	Jobs     []postingSummaryJSON `json:"jobs"`
}

func summarize(p dayforce.SearchPosting) postingSummaryJSON {
	s := postingSummaryJSON{
		ID:     p.JobPostingId,
		Title:  p.JobTitle,
		Remote: p.HasVirtualLocation,
	}
	if len(p.PostingLocations) > 0 {
		s.Location = p.PostingLocations[0].FormattedAddress
	}
	return s
}

// printSummary prints one job's compact text block (everything below the
// title line).
func printSummary(s postingSummaryJSON) {
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.Remote {
		fmt.Println("Remote: yes")
	}
	fmt.Printf("ID: %d\n", s.ID)
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	company  string
	timeout  time.Duration
	keyword  string
	location string
	page     int
	format   string
}

// runSearch maps every flag onto the search endpoint's real server-side
// filters. --page is 1-based for the caller; the arithmetic against the
// fixed 25-row page size happens here, so the client only ever sees a raw
// paginationStart offset.
func runSearch(ctx context.Context, f searchFlags) error {
	company, err := normalizeCompany(f.company)
	if err != nil {
		return err
	}
	if f.page < 1 {
		return fmt.Errorf("--page must be >= 1, got %d", f.page)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := dayforce.NewBoardClient(_apiBaseURL, nil)
	if err != nil {
		return err
	}

	req := dayforce.SearchRequest{
		ClientNamespace: company.Namespace,
		JobBoardCode:    company.JobBoardCode,
		CultureCode:     company.Culture(),
		PaginationStart: dayforce.NewOptInt((f.page - 1) * _pageSize),
	}
	if f.keyword != "" {
		req.SearchText = dayforce.NewOptString(f.keyword)
	}
	if f.location != "" {
		// distanceUnit 0 is miles — the portal's own search body.
		const distanceUnitMiles = 0
		req.LocationString = dayforce.NewOptString(f.location)
		req.Distance = dayforce.NewOptFloat64(50)
		req.DistanceUnit = dayforce.NewOptInt(distanceUnitMiles)
	}

	res, err := client.Search(ctx, req)
	if err != nil {
		return err
	}

	jobs := make([]postingSummaryJSON, len(res.JobPostings))
	for i, p := range res.JobPostings {
		jobs[i] = summarize(p)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{MaxCount: res.MaxCount, Offset: res.Offset, Jobs: jobs})
	}

	fmt.Printf("Dayforce Jobs Report (company: %s)\n", company.Slug())
	fmt.Printf("Found %d jobs total; showing %d starting at offset %d\n\n", res.MaxCount, len(jobs), res.Offset)
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

// detailFlags carries the parsed "detail" subcommand flags into runDetail.
type detailFlags struct {
	company   string
	timeout   time.Duration
	postingID int
	format    string
}

// runDetail fetches one posting in full via the detail endpoint, which —
// unlike search — returns a ProblemDetails for an unknown id.
func runDetail(ctx context.Context, f detailFlags) error {
	company, err := normalizeCompany(f.company)
	if err != nil {
		return err
	}
	if f.postingID <= 0 {
		return errors.New("--id is required (take it from a search result's ID)")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := dayforce.NewBoardClient(_apiBaseURL, nil)
	if err != nil {
		return err
	}

	d, err := client.Job(ctx, company.Namespace, company.Culture(), company.JobBoardID, f.postingID)
	if err != nil {
		return err
	}

	return printDetail(d, f.format)
}

// printDetail renders one full posting. JSON mode encodes the generated
// JobPostingDetail as-is — detail is for seeing the whole record.
func printDetail(d *dayforce.JobPostingDetail, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.JobTitle)
	if len(d.PostingLocations) > 0 {
		fmt.Printf("Location: %s\n", d.PostingLocations[0].FormattedAddress)
	}
	if d.HasVirtualLocation {
		fmt.Println("Remote: yes")
	}
	fmt.Printf("Posted: %s\n", d.PostingStartTimestampUTC.Format("2006-01-02"))
	fmt.Printf("ID: %d\n", d.JobPostingId)

	printSection("Job Description Header", d.JobPostingContent.JobDescriptionHeader.Or(""))
	printSection("Job Description", d.JobPostingContent.JobDescription.Or(""))
	printSection("Job Description Footer", d.JobPostingContent.JobDescriptionFooter.Or(""))
	return nil
}

// printSection renders one jobPostingContent field, converting its HTML
// text to plain text. Falls back to the raw HTML on a conversion failure
// rather than dropping the section; skips it entirely when empty.
func printSection(title, html string) {
	if html == "" {
		return
	}
	rendered, err := html2text.FromString(html, html2text.Options{})
	if err != nil {
		rendered = html
	}
	fmt.Printf("\n%s:\n%s\n", title, rendered)
}
