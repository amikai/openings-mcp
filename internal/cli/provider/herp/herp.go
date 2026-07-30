package herp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/herp"
)

const baseURL = "https://herp.careers"

type options struct {
	company string
	timeout time.Duration
	format  string
}

type searchFlags struct {
	keyword string
}

type getFlags struct {
	jobID string
}

// NewCommand returns a cobra.Command for herp.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "herp",
		Short:        "HERP Career postings CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.company, "company", "", "HERP Career company slug, e.g. notainc (see 'herp companies' for the curated list)")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated HERP Career companies (company name and slug)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompanies(opts.format)
		},
	}

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "list a company's jobs as summaries (client-side keyword filter)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), searchOptions{company: opts.company, timeout: opts.timeout, keyword: sFlags.keyword, format: opts.format})
		},
	}
	searchCmd.Flags().StringVar(&sFlags.keyword, "keyword", "", "case-insensitive substring filter on job titles (empty lists every job)")

	gFlags := &getFlags{}
	getCmd := &cobra.Command{
		Use:          "get",
		Short:        "print one job in full (every section HERP Career publishes)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd.Context(), getOptions{company: opts.company, timeout: opts.timeout, jobID: gFlags.jobID, format: opts.format})
		},
	}
	getCmd.Flags().StringVar(&gFlags.jobID, "id", "", "job posting ID from search results")

	rootCmd.AddCommand(companiesCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(getCmd)

	return rootCmd
}

func runCompanies(format string) error {
	cs := herp.Companies

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

func fetchBoard(ctx context.Context, company string, timeout time.Duration) (*herp.CompanyBoard, error) {
	if company == "" {
		return nil, errors.New("--company is required")
	}
	slug := strings.ToLower(company)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := herp.NewClient(baseURL)
	if err != nil {
		return nil, err
	}

	res, err := client.GetCompany(ctx, herp.GetCompanyParams{Slug: slug})
	if err != nil {
		return nil, err
	}
	switch r := res.(type) {
	case *herp.CompanyResponse:
		return &r.Company, nil
	case *herp.ErrorResponse:
		return nil, fmt.Errorf("company %q is not listed on HERP Career", company)
	default:
		return nil, fmt.Errorf("unexpected response type %T", res)
	}
}

type jobSummaryJSON struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Roles       string `json:"roles,omitempty"`
	Location    string `json:"location,omitempty"`
	Salary      string `json:"salary,omitempty"`
	PublishedAt string `json:"publishedAt"`
	URL         string `json:"url"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(board *herp.CompanyBoard, j *herp.Job) jobSummaryJSON {
	roles := make([]string, 0, len(j.JobRoles))
	for _, r := range j.JobRoles {
		roles = append(roles, r.Name)
	}

	var salary string
	if j.Salary.Set && !j.Salary.Null {
		salary, _, _ = strings.Cut(j.Salary.Value.Text, "\n")
	}

	published, _, _ := strings.Cut(j.JobPublishedAt, "T")

	return jobSummaryJSON{
		ID:          j.ID,
		Title:       j.Name,
		Roles:       strings.Join(roles, ", "),
		Location:    strings.Join(strings.Fields(j.Location), " "),
		Salary:      salary,
		PublishedAt: published,
		URL:         jobURL(board, j.ID),
	}
}

func jobURL(board *herp.CompanyBoard, id string) string {
	if board.CompanyIsApplicationEnabled.Or(true) {
		return fmt.Sprintf("%s/careers/companies/%s/jobs/%s", baseURL, board.CompanySlug, id)
	}
	return fmt.Sprintf("%s/v1/%s/%s", baseURL, board.CompanySlug, id)
}

type searchOptions struct {
	company string
	timeout time.Duration
	keyword string
	format  string
}

func runSearch(ctx context.Context, f searchOptions) error {
	board, err := fetchBoard(ctx, f.company, f.timeout)
	if err != nil {
		return err
	}

	matched := make([]jobSummaryJSON, 0, len(board.Jobs))
	for _, j := range board.Jobs {
		if f.keyword != "" && !strings.Contains(strings.ToLower(j.Name), strings.ToLower(f.keyword)) {
			continue
		}
		matched = append(matched, summarize(board, &j))
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(board.Jobs), Jobs: matched})
	}

	fmt.Printf("HERP Career Jobs Report for %s (%s)\n", board.CompanyName, board.CompanySlug)
	fmt.Printf("Found %d jobs; showing %d\n\n", len(board.Jobs), len(matched))
	for i, s := range matched {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		fmt.Printf("  ID: %s\n", s.ID)
		if s.Roles != "" {
			fmt.Printf("  Roles: %s\n", s.Roles)
		}
		if s.Location != "" {
			fmt.Printf("  Location: %s\n", s.Location)
		}
		if s.Salary != "" {
			fmt.Printf("  Salary: %s\n", s.Salary)
		}
		fmt.Printf("  Posted: %s\n", s.PublishedAt)
		fmt.Printf("  URL: %s\n\n", s.URL)
	}
	return nil
}

type getOptions struct {
	company string
	timeout time.Duration
	jobID   string
	format  string
}

func runGet(ctx context.Context, f getOptions) error {
	if f.jobID == "" {
		return errors.New("--id is required")
	}
	board, err := fetchBoard(ctx, f.company, f.timeout)
	if err != nil {
		return err
	}
	for _, j := range board.Jobs {
		if j.ID == f.jobID {
			return printJob(board, &j, f.format)
		}
	}
	return fmt.Errorf("job %q not found on company %q", f.jobID, f.company)
}

func printJob(board *herp.CompanyBoard, j *herp.Job, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(j)
	}

	s := summarize(board, j)
	fmt.Println(s.Title)
	if j.Title != "" {
		fmt.Println(j.Title)
	}
	fmt.Printf("Company: %s\n", board.CompanyName)
	fmt.Printf("ID: %s\n", s.ID)
	fmt.Printf("Employment: %s\n", j.FormOfEmployment)
	if s.Roles != "" {
		fmt.Printf("Roles: %s\n", s.Roles)
	}
	fmt.Printf("Posted: %s\n", s.PublishedAt)
	fmt.Printf("URL: %s\n", s.URL)

	for _, sec := range []struct {
		label string
		body  string
	}{
		{label: "仕事概要", body: j.Summary},
		{label: "必須スキル", body: j.RequiredSkills.Or("")},
		{label: "歓迎スキル", body: j.PreferredSkills.Or("")},
		{label: "求める人物像", body: j.Personality.Or("")},
		{label: "給与", body: salaryText(j)},
		{label: "勤務地", body: j.Location},
		{label: "勤務体系", body: j.WorkingConditions.Or("")},
		{label: "試用期間", body: j.Trial.Or("")},
		{label: "福利厚生", body: j.Welfare.Or("")},
	} {
		if sec.body == "" {
			continue
		}
		fmt.Printf("\n%s:\n%s\n", sec.label, sec.body)
	}
	return nil
}

func salaryText(j *herp.Job) string {
	if !j.Salary.Set || j.Salary.Null {
		return ""
	}
	return j.Salary.Value.Text
}
