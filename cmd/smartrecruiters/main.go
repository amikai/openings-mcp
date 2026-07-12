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

	smartrecruiters "github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

// apiBaseURL is SmartRecruiters' public Posting API origin — the single
// production server in the provider's openapi.yaml.
const apiBaseURL = "https://api.smartrecruiters.com/v1"

func main() {
	rootFlags := ff.NewFlagSet("smartrecruiters")
	var (
		company = rootFlags.StringLong("company", "", `SmartRecruiters companyIdentifier from the career site URL, e.g. "Equinox" in jobs.smartrecruiters.com/Equinox`)
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "smartrecruiters",
		Usage: "smartrecruiters --company COMPANY [FLAGS] <search|get> [FLAGS]",
		Flags: rootFlags,
	}

	searchFlags := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword    = searchFlags.StringLong("keyword", "", "free-text keyword search across posting titles")
		country    = searchFlags.StringLong("country", "", "lowercase ISO country code, e.g. us")
		region     = searchFlags.StringLong("region", "", "state/region code, e.g. TX")
		city       = searchFlags.StringLong("city", "", "city name")
		department = searchFlags.StringLong("department", "", "department.id value from a search result (not the display label)")
		limit      = searchFlags.IntLong("limit", 20, "page size (upstream caps at 100)")
		offset     = searchFlags.IntLong("offset", 0, "zero-based result offset")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "smartrecruiters --company COMPANY search [--keyword TEXT] [--country CC] [--region R] [--city CITY] [--department ID] [--limit N] [--offset N] [--format text|json]",
		ShortHelp: "search postings for a company (server-side filters)",
		Flags:     searchFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runSearch(ctx, *company, *timeout, *keyword, *country, *region, *city, *department, *limit, *offset, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	getFlags := ff.NewFlagSet("get").SetParent(rootFlags)
	postingID := getFlags.StringLong("id", "", "posting id from a search result")
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "smartrecruiters --company COMPANY get --id POSTING-ID [--format text|json]",
		ShortHelp: "print one posting in full (description sections and public URL)",
		Flags:     getFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runGet(ctx, *company, *timeout, *postingID, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, getCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search or get) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// postingSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs, no description. No public URL — the
// list endpoint only carries `ref` (the Posting API's own detail link);
// the human-clickable postingUrl only appears on 'get'.
type postingSummaryJSON struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Location   string `json:"location,omitempty"`
	Department string `json:"department,omitempty"`
	PostedAt   string `json:"postedAt,omitempty"`
}

type searchResultJSON struct {
	Total int                  `json:"total"`
	Jobs  []postingSummaryJSON `json:"jobs"`
}

func summarize(p smartrecruiters.PostingSummary) postingSummaryJSON {
	s := postingSummaryJSON{
		ID:       p.ID.Value,
		Title:    p.Name.Value,
		Location: p.Location.Value.FullLocation.Value,
	}
	if dep, ok := p.Department.Get(); ok {
		s.Department = dep.Label.Value
	}
	if v, ok := p.ReleasedDate.Get(); ok {
		s.PostedAt = v.Format("2006-01-02")
	}
	return s
}

// printSummary prints one job's compact text block (everything below the
// title line).
func printSummary(s postingSummaryJSON) {
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.Department != "" {
		fmt.Printf("Department: %s\n", s.Department)
	}
	if s.PostedAt != "" {
		fmt.Printf("Posted: %s\n", s.PostedAt)
	}
	fmt.Printf("ID: %s\n", s.ID)
}

// runSearch maps every flag directly onto the Posting API's real
// server-side filters — unlike Greenhouse's client-side dump-and-filter,
// SmartRecruiters does the narrowing upstream.
func runSearch(
	ctx context.Context,
	company string,
	timeout time.Duration,
	keyword, country, region, city, department string,
	limit, offset int,
	format string,
) error {
	if company == "" {
		return errors.New("--company is required")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := smartrecruiters.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	params := smartrecruiters.ListPostingsParams{
		CompanyIdentifier: company,
		Limit:             smartrecruiters.NewOptInt(limit),
		Offset:            smartrecruiters.NewOptInt(offset),
	}
	if keyword != "" {
		params.Q = smartrecruiters.NewOptString(keyword)
	}
	if country != "" {
		params.Country = smartrecruiters.NewOptString(country)
	}
	if region != "" {
		params.Region = smartrecruiters.NewOptString(region)
	}
	if city != "" {
		params.City = smartrecruiters.NewOptString(city)
	}
	if department != "" {
		params.Department = smartrecruiters.NewOptString(department)
	}

	res, err := client.ListPostings(ctx, params)
	if err != nil {
		return err
	}

	jobs := make([]postingSummaryJSON, len(res.Content))
	for i, p := range res.Content {
		jobs[i] = summarize(p)
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: res.TotalFound, Jobs: jobs})
	}

	fmt.Printf("SmartRecruiters Jobs Report (company: %s)\n", company)
	fmt.Printf("Found %d jobs; showing %d\n\n", res.TotalFound, len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

// runGet fetches one posting in full via the Posting API's detail
// endpoint, which — unlike the list endpoint — 404s for an unknown id
// rather than returning an empty result.
func runGet(ctx context.Context, company string, timeout time.Duration, postingID, format string) error {
	if company == "" {
		return errors.New("--company is required")
	}
	if postingID == "" {
		return errors.New("--id is required (take it from a search result's ID)")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := smartrecruiters.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetPosting(ctx, smartrecruiters.GetPostingParams{
		CompanyIdentifier: company,
		PostingId:         postingID,
	})
	if err != nil {
		return err
	}

	switch d := res.(type) {
	case *smartrecruiters.PostingDetail:
		return printDetail(d, format)
	case *smartrecruiters.PostingErrorResponse:
		return fmt.Errorf("posting %q not found for company %q", postingID, company)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}

// printDetail renders one full posting. JSON mode encodes the generated
// PostingDetail as-is — detail is for seeing the whole record.
func printDetail(d *smartrecruiters.PostingDetail, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Name.Value)
	if name := d.Company.Value.Name.Value; name != "" {
		fmt.Printf("Company: %s\n", name)
	}
	if loc := d.Location.Value.FullLocation.Value; loc != "" {
		fmt.Printf("Location: %s\n", loc)
	}
	if v, ok := d.ReleasedDate.Get(); ok {
		fmt.Printf("Posted: %s\n", v.Format("2006-01-02"))
	}
	if v, ok := d.PostingUrl.Get(); ok && v != "" {
		fmt.Printf("URL: %s\n", v)
	}

	if sections, ok := d.JobAd.Value.Sections.Get(); ok {
		printSection("Company Description", sections.CompanyDescription)
		printSection("Job Description", sections.JobDescription)
		printSection("Qualifications", sections.Qualifications)
		printSection("Additional Information", sections.AdditionalInformation)
	}
	return nil
}

// printSection renders one jobAd.sections entry, converting its HTML text
// to plain text. Falls back to the fallbackTitle when the section omits
// its own title, and to the raw HTML on a conversion failure rather than
// dropping the section.
func printSection(fallbackTitle string, opt smartrecruiters.OptJobAdSection) {
	sec, ok := opt.Get()
	if !ok || sec.Text.Value == "" {
		return
	}
	title := sec.Title.Value
	if title == "" {
		title = fallbackTitle
	}
	rendered, err := html2text.FromString(sec.Text.Value, html2text.Options{})
	if err != nil {
		rendered = sec.Text.Value
	}
	fmt.Printf("\n%s:\n%s\n", title, rendered)
}
