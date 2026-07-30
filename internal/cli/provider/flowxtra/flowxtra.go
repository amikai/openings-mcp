package flowxtra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/flowxtra"
)

// apiBaseURL is the single production server in the provider's openapi.yaml.
const apiBaseURL = "https://app.flowxtra.com/api"

type options struct {
	baseURL string
	timeout time.Duration
	format  string
}

type searchFlags struct {
	query     string
	location  string
	workplace string
	company   string
	page      int
	perPage   int
}

type detailFlags struct {
	hasID string
}

// NewCommand returns a cobra.Command for flowxtra.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "flowxtra",
		Short:        "Flowxtra ATS cross-tenant postings CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", apiBaseURL, "Flowxtra API base URL")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search live jobs across every company on Flowxtra (server-side narrowing)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sFlags.page < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", sFlags.page)
			}
			if sFlags.perPage < 1 {
				return fmt.Errorf("--per-page must be >= 1, got %d", sFlags.perPage)
			}
			params := flowxtra.ListJobsParams{
				Page:    flowxtra.NewOptInt(sFlags.page),
				PerPage: flowxtra.NewOptInt(sFlags.perPage),
			}
			if sFlags.query != "" {
				params.SearchKey = flowxtra.NewOptString(sFlags.query)
			}
			if sFlags.location != "" {
				params.Location = flowxtra.NewOptString(sFlags.location)
			}
			if sFlags.workplace != "" {
				wp := flowxtra.ListJobsWorkplace(sFlags.workplace)
				if err := wp.Validate(); err != nil {
					return fmt.Errorf("invalid --workplace %q: %w", sFlags.workplace, err)
				}
				params.Workplace = flowxtra.NewOptListJobsWorkplace(wp)
			}
			if sFlags.company != "" {
				params.CompanyName = flowxtra.NewOptString(sFlags.company)
			}
			return runSearch(cmd.Context(), opts.baseURL, opts.timeout, opts.format, params)
		},
	}
	searchCmd.Flags().StringVar(&sFlags.query, "query", "", "job-title search text (server-side LIKE)")
	searchCmd.Flags().StringVar(&sFlags.location, "location", "", `company city, state, or country search, e.g. "Spain"`)
	searchCmd.Flags().StringVar(&sFlags.workplace, "workplace", "", "exact workplace: On-site, Hybrid, or Remote")
	searchCmd.Flags().StringVar(&sFlags.company, "company", "", "company-name search text (server-side LIKE)")
	searchCmd.Flags().IntVar(&sFlags.page, "page", 1, "1-based page number")
	searchCmd.Flags().IntVar(&sFlags.perPage, "per-page", 20, "page size")

	dFlags := &detailFlags{}
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one job in full by its has_id",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dFlags.hasID == "" {
				return errors.New("--id is required (take it from a search result's has_id)")
			}
			return runDetail(cmd.Context(), opts.baseURL, opts.timeout, opts.format, dFlags.hasID)
		},
	}
	detailCmd.Flags().StringVar(&dFlags.hasID, "id", "", "public hashed job id (has_id from a search result)")

	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(detailCmd)

	return rootCmd
}

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

func formatLocation(city, state, country string) string {
	cleaned := make([]string, 0, 3)
	for _, part := range []string{city, state, country} {
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, ", ")
}

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
