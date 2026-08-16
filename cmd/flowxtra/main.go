// Command flowxtra is a debug CLI for Flowxtra's public cross-tenant
// jobs API — one board-wide feed covering every company hosted on the
// Flowxtra ATS.
//
//	go run ./cmd/flowxtra search --query sales --location Spain
//	go run ./cmd/flowxtra search --workplace Remote --page 2
//	go run ./cmd/flowxtra detail --id M88PB
//
// Search narrowing is server-side (see the surface notes in
// internal/provider/flowxtra/openapi.yaml); detail exchanges a has_id
// from a search result for the full posting.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/flowxtra"
)

// apiBaseURL is the single production server in the provider's
// openapi.yaml (paths carry the /central and /candidate prefixes).
const apiBaseURL = "https://app.flowxtra.com/api"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("flowxtra")
	var (
		baseURL = rootFlags.StringLong("base-url", apiBaseURL, "Flowxtra API base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "flowxtra",
		Usage: "flowxtra [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		query     = searchFS.StringLong("query", "", "job-title search text (server-side LIKE)")
		location  = searchFS.StringLong("location", "", `company city, state, or country search, e.g. "Spain"`)
		workplace = searchFS.StringLong("workplace", "", "exact workplace: On-site, Hybrid, or Remote")
		company   = searchFS.StringLong("company", "", "company-name search text (server-side LIKE)")
		page      = searchFS.IntLong("page", 1, "1-based page number")
		perPage   = searchFS.IntLong("per-page", 20, "page size")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "flowxtra search [--query TEXT] [--location TEXT] [--workplace TYPE] [--company TEXT] [--page N] [--per-page N] [--format text|json]",
		ShortHelp: "search live jobs across every company on Flowxtra (server-side narrowing)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			if *page < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", *page)
			}
			if *perPage < 1 {
				return fmt.Errorf("--per-page must be >= 1, got %d", *perPage)
			}
			params := flowxtra.ListJobsParams{
				Page:    flowxtra.NewOptInt(*page),
				PerPage: flowxtra.NewOptInt(*perPage),
			}
			if *query != "" {
				params.SearchKey = flowxtra.NewOptString(*query)
			}
			if *location != "" {
				params.Location = flowxtra.NewOptString(*location)
			}
			if *workplace != "" {
				wp := flowxtra.ListJobsWorkplace(*workplace)
				if err := wp.Validate(); err != nil {
					return fmt.Errorf("invalid --workplace %q: %w", *workplace, err)
				}
				params.Workplace = flowxtra.NewOptListJobsWorkplace(wp)
			}
			if *company != "" {
				params.CompanyName = flowxtra.NewOptString(*company)
			}
			return runSearch(ctx, *baseURL, *timeout, *format, params)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	hasID := detailFS.StringLong("id", "", "public hashed job id (has_id from a search result)")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "flowxtra detail --id HAS-ID [--format text|json]",
		ShortHelp: "print one job in full by its has_id",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			if *hasID == "" {
				return errors.New("--id is required (take it from a search result's has_id)")
			}
			return runDetail(ctx, *baseURL, *timeout, *format, *hasID)
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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search or detail) is required")
		return 1
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

// jobSummaryJSON is the --format json shape for one search result.
type jobSummaryJSON struct {
	HasID     string `json:"has_id"`
	Title     string `json:"title"`
	Company   string `json:"company"`
	Location  string `json:"location,omitempty"`
	Workplace string `json:"workplace"`
	Salary    string `json:"salary,omitempty"`
	PostedAt  string `json:"posted_at"`
	URL       string `json:"url"`
}

type searchResultJSON struct {
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	LastPage int              `json:"last_page"`
	Jobs     []jobSummaryJSON `json:"jobs"`
}

// formatLocation joins the non-empty company location parts.
func formatLocation(city, state, country string) string {
	cleaned := make([]string, 0, 3)
	for _, part := range []string{city, state, country} {
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, ", ")
}

// formatSalary renders the salary fields as one line, e.g.
// "EUR 21000/year" or "EUR 1000-2000/month"; empty when unset.
func formatSalary(currency string, minSalary, maxSalary, salary flowxtra.NilFloat64, rate string) string {
	amount := ""
	switch {
	case !salary.Null:
		amount = fmt.Sprintf("%g", salary.Value)
	case !minSalary.Null && !maxSalary.Null:
		amount = fmt.Sprintf("%g-%g", minSalary.Value, maxSalary.Value)
	case !minSalary.Null:
		amount = fmt.Sprintf("%g", minSalary.Value)
	case !maxSalary.Null:
		amount = fmt.Sprintf("up to %g", maxSalary.Value)
	default:
		return ""
	}
	out := currency + " " + amount
	if rate != "" {
		out += "/" + rate
	}
	return out
}

func summarize(j flowxtra.JobSummary) jobSummaryJSON {
	return jobSummaryJSON{
		HasID:     j.HasID,
		Title:     j.Title,
		Company:   j.NameCompany,
		Location:  formatLocation(j.CityCompany, j.StateCompany, j.CountryCompany),
		Workplace: j.Workplace,
		Salary:    formatSalary(j.Currency, j.MinSalary, j.MaxSalary, j.Salary, j.RateSalary),
		PostedAt:  j.DateShare.Format(time.DateOnly),
		URL:       j.UrlJobApplay,
	}
}

func runSearch(ctx context.Context, baseURL string, timeout time.Duration, format string, params flowxtra.ListJobsParams) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := flowxtra.NewClient(baseURL)
	if err != nil {
		return err
	}
	res, err := client.ListJobs(ctx, params)
	if err != nil {
		return err
	}

	jobs := make([]jobSummaryJSON, len(res.Data.Data))
	for i, j := range res.Data.Data {
		jobs[i] = summarize(j)
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{
			Total:    res.Data.Total,
			Page:     res.Data.CurrentPage,
			LastPage: res.Data.LastPage,
			Jobs:     jobs,
		})
	}

	fmt.Printf("Flowxtra Jobs Report (total: %d, page %d of %d)\n\n", res.Data.Total, res.Data.CurrentPage, res.Data.LastPage)
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		fmt.Printf("Company: %s\n", s.Company)
		if s.Location != "" {
			fmt.Printf("Location: %s (%s)\n", s.Location, s.Workplace)
		} else {
			fmt.Printf("Workplace: %s\n", s.Workplace)
		}
		if s.Salary != "" {
			fmt.Printf("Salary: %s\n", s.Salary)
		}
		fmt.Printf("Posted: %s\n", s.PostedAt)
		fmt.Printf("ID: %s\n", s.HasID)
		fmt.Println()
	}
	return nil
}

func runDetail(ctx context.Context, baseURL string, timeout time.Duration, format, hasID string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := flowxtra.NewClient(baseURL)
	if err != nil {
		return err
	}
	res, err := client.GetJobDetail(ctx, flowxtra.GetJobDetailParams{HasId: hasID})
	if err != nil {
		return err
	}

	envelope, ok := res.(*flowxtra.JobDetailEnvelope)
	if !ok {
		return fmt.Errorf("job %q not found (it may have expired)", hasID)
	}
	job := envelope.Data

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(job)
	}

	fmt.Println(job.Title)
	fmt.Printf("Company: %s\n", job.Company.Name)
	if website, ok := job.Company.Website.Get(); ok && website != "" {
		fmt.Printf("Website: %s\n", website)
	}
	if office, ok := job.CompanyOffice.Get(); ok {
		state, country := "", ""
		if v, ok := office.State.Get(); ok {
			state = v.Name
		}
		if v, ok := office.Country.Get(); ok {
			country = v.Name
		}
		if loc := formatLocation("", state, country); loc != "" {
			fmt.Printf("Location: %s (%s)\n", loc, job.Workplace)
		}
	}
	if seniority, ok := job.Seniority.Get(); ok && seniority != "" {
		fmt.Printf("Seniority: %s\n", seniority)
	}
	for _, et := range job.JobTypeJob {
		fmt.Printf("Employment: %s\n", et.Name)
	}
	if salary := formatSalary(job.Currency, job.MinSalary, job.MaxSalary, job.Salary, job.RateSalary); salary != "" {
		fmt.Printf("Salary: %s\n", salary)
	}
	fmt.Printf("Apply: %s\n", job.UrlJobApplay)

	rendered, err := html2text.FromString(job.Description, html2text.Options{})
	if err != nil {
		rendered = job.Description
	}
	fmt.Printf("\nDescription:\n%s\n", rendered)
	return nil
}
