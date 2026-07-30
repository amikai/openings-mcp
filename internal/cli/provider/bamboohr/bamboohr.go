// Package bamboohr provides CLI command for BambooHR careers sites.
package bamboohr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	"github.com/amikai/openings-mcp/internal/provider/bamboohr"
)

type options struct {
	company string
	timeout time.Duration
	format  string
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	company string
	timeout time.Duration
	keyword string
	format  string
}

// getFlags carries the parsed "get" subcommand flags into runGet.
type getFlags struct {
	company string
	timeout time.Duration
	jobID   string
	format  string
}

// NewCommand returns a cobra.Command for bamboohr.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "bamboohr",
		Short:        "BambooHR careers sites CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.company, "company", "", "curated BambooHR subdomain slug, e.g. concept2")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	clihelp.FormatVar(rootCmd.PersistentFlags(), &opts.format)

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated BambooHR careers sites (company name and subdomain slug)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompanies(opts.format)
		},
	}

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "list a careers site's jobs as summaries (client-side keyword filter)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sFlags.company = opts.company
			sFlags.timeout = opts.timeout
			sFlags.format = opts.format
			return runSearch(cmd.Context(), *sFlags)
		},
	}
	searchCmd.Flags().StringVar(&sFlags.keyword, "keyword", "", "case-insensitive substring filter on job titles")

	gFlags := &getFlags{}
	getCmd := &cobra.Command{
		Use:          "get",
		Short:        "print one job in full (description and compensation)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			gFlags.company = opts.company
			gFlags.timeout = opts.timeout
			gFlags.format = opts.format
			return runGet(cmd.Context(), *gFlags)
		},
	}
	getCmd.Flags().StringVar(&gFlags.jobID, "id", "", "job opening id from search results")

	rootCmd.AddCommand(companiesCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(getCmd)

	return rootCmd
}

// runCompanies lists every curated BambooHR careers site embedded in the
// CLI (internal/provider/bamboohr/companies.yaml), sorted by company name.
// It makes no network call.
func runCompanies(format string) error {
	cs := bamboohr.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.Slug)
	}
	return nil
}

// newTenantClient validates the slug against the embedded roster and
// returns a client bound to the tenant's origin. Redirect-following is
// disabled so an unknown tenant surfaces as the API's 302 instead of a
// decode error on the marketing site's HTML.
func newTenantClient(company string) (*bamboohr.Client, string, error) {
	if company == "" {
		return nil, "", errors.New("--company is required")
	}
	slug := strings.ToLower(company)
	if _, ok := bamboohr.CompaniesBySlug[slug]; !ok {
		return nil, "", fmt.Errorf("company %q not found; run 'bamboohr companies' to see supported companies", company)
	}
	hc := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	client, err := bamboohr.NewClient("https://"+slug+".bamboohr.com", bamboohr.WithClient(hc))
	if err != nil {
		return nil, "", err
	}
	return client, slug, nil
}

// jobSummaryJSON is the --format json shape for one search result: the
// compact fields a listing needs. The list feed carries no posting date -
// that lives on the detail endpoint only.
type jobSummaryJSON struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Department       string `json:"department,omitempty"`
	Location         string `json:"location,omitempty"`
	WorkMode         string `json:"workMode,omitempty"`
	EmploymentStatus string `json:"employmentStatus,omitempty"`
	URL              string `json:"url"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(slug string, j *bamboohr.ListJob) jobSummaryJSON {
	return jobSummaryJSON{
		ID:               j.ID,
		Title:            j.JobOpeningName,
		Department:       j.DepartmentLabel.Or(""),
		Location:         listLocation(j),
		WorkMode:         bamboohr.WorkModeLabel(j.LocationType.Or("")),
		EmploymentStatus: j.EmploymentStatusLabel,
		URL:              postingURL(slug, j.ID),
	}
}

// postingURL builds the human-clickable posting page, the same URL the
// detail endpoint reports as jobOpeningShareUrl.
func postingURL(slug, id string) string {
	return fmt.Sprintf("https://%s.bamboohr.com/careers/%s", slug, id)
}

// listLocation renders a list row's location, preferring the structured
// `location` and falling back to `atsLocation` (which alone carries the
// country) when the former is all-null.
func listLocation(j *bamboohr.ListJob) string {
	if s := joinParts(j.Location.City.Or(""), j.Location.State.Or("")); s != "" {
		return s
	}
	return joinParts(j.AtsLocation.City.Or(""), j.AtsLocation.State.Or(""), j.AtsLocation.Country.Or(""))
}

func joinParts(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}

// runSearch fetches the whole board and prints summaries, optionally
// filtered by a case-insensitive substring match on the title. There is no
// pagination - the API returns everything in one response.
func runSearch(ctx context.Context, f searchFlags) error {
	client, slug, err := newTenantClient(f.company)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	res, err := client.ListJobs(ctx)
	if err != nil {
		return err
	}
	list, err := unwrapList(res, slug)
	if err != nil {
		return err
	}

	matched := make([]jobSummaryJSON, 0, len(list.Result))
	for _, j := range list.Result {
		if f.keyword != "" && !strings.Contains(strings.ToLower(j.JobOpeningName), strings.ToLower(f.keyword)) {
			continue
		}
		matched = append(matched, summarize(slug, &j))
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: list.Meta.TotalCount, Jobs: matched})
	}

	fmt.Printf("BambooHR Jobs Report\n")
	fmt.Printf("Found %d jobs; showing %d\n\n", list.Meta.TotalCount, len(matched))
	for i, s := range matched {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

func unwrapList(res bamboohr.ListJobsRes, slug string) (*bamboohr.ListResponse, error) {
	switch r := res.(type) {
	case *bamboohr.ListResponse:
		return r, nil
	case *bamboohr.ListJobsFound:
		return nil, fmt.Errorf("tenant %q not found upstream (redirected to the marketing site)", slug)
	default:
		return nil, fmt.Errorf("unexpected response type %T", res)
	}
}

// printSummary prints one job's compact text block.
func printSummary(s jobSummaryJSON) {
	if s.Department != "" {
		fmt.Printf("Department: %s\n", s.Department)
	}
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.WorkMode != "" {
		fmt.Printf("Workplace: %s\n", s.WorkMode)
	}
	if s.EmploymentStatus != "" {
		fmt.Printf("Employment: %s\n", s.EmploymentStatus)
	}
	fmt.Printf("URL: %s\n", s.URL)
	fmt.Printf("ID: %s\n", s.ID)
}

// runGet fetches one posting from the per-job detail endpoint and prints it
// in full.
func runGet(ctx context.Context, f getFlags) error {
	if f.jobID == "" {
		return errors.New("--id is required")
	}
	client, slug, err := newTenantClient(f.company)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	res, err := client.GetJobDetail(ctx, bamboohr.GetJobDetailParams{ID: f.jobID})
	if err != nil {
		return err
	}
	switch r := res.(type) {
	case *bamboohr.DetailResponse:
		return printJob(slug, f.jobID, &r.Result.JobOpening, f.format)
	case *bamboohr.NotFoundError:
		return fmt.Errorf("job %q not found for company %q", f.jobID, slug)
	case *bamboohr.GetJobDetailFound:
		return fmt.Errorf("tenant %q not found upstream (redirected to the marketing site)", slug)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}

func printJob(slug, id string, jo *bamboohr.JobOpening, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jo)
	}

	fmt.Println(jo.JobOpeningName)
	if v := jo.DepartmentLabel.Or(""); v != "" {
		fmt.Printf("Department: %s\n", v)
	}
	if loc := joinParts(jo.Location.City.Or(""), jo.Location.State.Or(""), jo.Location.AddressCountry.Or("")); loc != "" {
		fmt.Printf("Location: %s\n", loc)
	}
	if v := bamboohr.WorkModeLabel(jo.LocationType.Or("")); v != "" {
		fmt.Printf("Workplace: %s\n", v)
	}
	fmt.Printf("Employment: %s\n", jo.EmploymentStatusLabel)
	if v := jo.MinimumExperience.Or(""); v != "" {
		fmt.Printf("Experience: %s\n", v)
	}
	if v := jo.Compensation.Or(""); v != "" {
		fmt.Printf("Compensation: %s\n", v)
	}
	if v := jo.DatePosted.Or(""); v != "" {
		fmt.Printf("Posted: %s\n", v)
	}
	fmt.Printf("URL: %s\n", postingURL(slug, id))

	if text, err := html2text.FromString(jo.Description, html2text.Options{}); err == nil && text != "" {
		fmt.Printf("\nDescription:\n%s\n", text)
	}
	return nil
}
