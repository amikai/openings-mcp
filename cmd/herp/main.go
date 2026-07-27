// Command herp is a debug CLI for the HERP Career client: list the curated
// roster, list one company's board, and print a single posting in full.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/herp"
)

const baseURL = "https://herp.careers"

func main() {
	rootFlags := ff.NewFlagSet("herp")
	var (
		company = rootFlags.StringLong("company", "", "HERP Career company slug, e.g. notainc (see 'herp companies' for the curated list)")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "herp",
		Usage: "herp --company COMPANY [FLAGS] <companies|search|get> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "herp companies [--format text|json]",
		ShortHelp: "list curated HERP Career companies (company name and slug)",
		Flags:     companiesFlags,
		Exec: func(_ context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			return runCompanies(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	keyword := searchFS.StringLong("keyword", "", "case-insensitive substring filter on job titles (empty lists every job)")
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "herp --company COMPANY search [--keyword TEXT] [--format text|json]",
		ShortHelp: "list a company's jobs as summaries (client-side keyword filter)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{company: *company, timeout: *timeout, keyword: *keyword, format: *format})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	getFS := ff.NewFlagSet("get").SetParent(rootFlags)
	jobID := getFS.StringLong("id", "", "job posting ID from search results")
	getCmd := &ff.Command{
		Name:      "get",
		Usage:     "herp --company COMPANY get --id ID [--format text|json]",
		ShortHelp: "print one job in full (every section HERP Career publishes)",
		Flags:     getFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("get takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runGet(ctx, getFlags{company: *company, timeout: *timeout, jobID: *jobID, format: *format})
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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, or get) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
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

// summarize takes the board rather than the caller's --company string: the
// slug the caller typed may differ in case, and herp.careers 404s on a
// mixed-case path, so every rendered URL uses board.CompanySlug.
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

// jobURL mirrors the ATS adapter: a company that opted out of HERP Career
// applications renders a media page with no apply action, so its postings are
// linked to the HERP Hire career page instead.
func jobURL(board *herp.CompanyBoard, id string) string {
	if board.CompanyIsApplicationEnabled.Or(true) {
		return fmt.Sprintf("%s/careers/companies/%s/jobs/%s", baseURL, board.CompanySlug, id)
	}
	return fmt.Sprintf("%s/v1/%s/%s", baseURL, board.CompanySlug, id)
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	company string
	timeout time.Duration
	keyword string
	format  string
}

func runSearch(ctx context.Context, f searchFlags) error {
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

// getFlags carries the parsed "get" subcommand flags into runGet.
type getFlags struct {
	company string
	timeout time.Duration
	jobID   string
	format  string
}

func runGet(ctx context.Context, f getFlags) error {
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
		{"仕事概要", j.Summary},
		{"必須スキル", j.RequiredSkills.Or("")},
		{"歓迎スキル", j.PreferredSkills.Or("")},
		{"求める人物像", j.Personality.Or("")},
		{"給与", salaryText(j)},
		{"勤務地", j.Location},
		{"勤務体系", j.WorkingConditions.Or("")},
		{"試用期間", j.Trial.Or("")},
		{"福利厚生", j.Welfare.Or("")},
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
