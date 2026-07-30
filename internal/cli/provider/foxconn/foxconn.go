// Package foxconn implements the "openings-mcp foxconn" debug CLI, for
// manual checks against the live surface that internal/provider/foxconn
// documents.
package foxconn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	"github.com/amikai/openings-mcp/internal/provider/foxconn"
)

type options struct {
	timeout time.Duration
	format  string
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	workplace  string
	talentZone string
	keyword    string
}

type detailFlags struct {
	id string
}

// NewCommand returns a cobra.Command for foxconn.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "foxconn",
		Short:        "Hon Hai / Foxconn Taiwan careers CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	clihelp.FormatVar(rootCmd.PersistentFlags(), &opts.format)

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search job vacancies (server-side filters; no pagination)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), searchOptions{
				timeout:    opts.timeout,
				workplace:  sFlags.workplace,
				talentZone: sFlags.talentZone,
				keyword:    sFlags.keyword,
				format:     opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&sFlags.workplace, "workplace", "", "workplaceCode location filter (e.g. TA, CH, VM); see the 'codes' subcommand")
	searchCmd.Flags().StringVar(&sFlags.talentZone, "talent-zone", "", "talentZoneCode recruitment-track filter (e.g. MA, TALENTS, INTERN); see the 'codes' subcommand")
	searchCmd.Flags().StringVar(&sFlags.keyword, "keyword", "", "case-insensitive free-text search across title and body")

	dFlags := &detailFlags{}
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "fetch one vacancy's full detail by its opaque id",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dFlags.id == "" {
				return errors.New("--id is required (take it from a search result's ID, not the job_no)")
			}
			return runDetail(cmd.Context(), opts.timeout, dFlags.id, opts.format)
		},
	}
	detailCmd.Flags().StringVar(&dFlags.id, "id", "", "opaque job id from a search result (not the job_no)")

	codesCmd := &cobra.Command{
		Use:          "codes",
		Short:        "list the valid --workplace and --talent-zone filter codes (static, no network)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCodes(opts.format)
		},
	}

	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(detailCmd)
	rootCmd.AddCommand(codesCmd)

	return rootCmd
}

// jobSummaryJSON is the --format json shape for one search result.
type jobSummaryJSON struct {
	ID       string `json:"id"`
	JobNo    string `json:"jobNo"`
	Title    string `json:"title"`
	Location string `json:"location,omitempty"`
	JobType  string `json:"jobType,omitempty"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j foxconn.JobVacancy) jobSummaryJSON {
	s := jobSummaryJSON{
		ID:       j.ID,
		JobNo:    j.JobNo,
		Title:    j.JobName,
		Location: j.LocName,
	}
	if d, ok := j.LocDesc.Get(); ok && d != "" {
		s.Location = fmt.Sprintf("%s (%s)", j.LocName, d)
	}
	if t, ok := j.JobTypeName.Get(); ok {
		s.JobType = t
	}
	return s
}

// printSummary prints one job's compact text block (everything below the
// title line).
func printSummary(s jobSummaryJSON) {
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.JobType != "" {
		fmt.Printf("Track: %s\n", s.JobType)
	}
	fmt.Printf("Job No: %s\n", s.JobNo)
	fmt.Printf("ID: %s\n", s.ID)
}

type searchOptions struct {
	timeout    time.Duration
	workplace  string
	talentZone string
	keyword    string
	format     string
}

// runSearch maps every flag onto the API's real server-side filters. The
// list endpoint has no pagination — it returns the full matching set in one
// response — so an unfiltered call returns the entire ~953-job board.
func runSearch(ctx context.Context, f searchOptions) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := foxconn.NewClient(foxconn.DefaultBaseURL)
	if err != nil {
		return err
	}

	params := foxconn.ListJobVacanciesParams{}
	if f.workplace != "" {
		params.WorkplaceCode = foxconn.NewOptString(f.workplace)
	}
	if f.talentZone != "" {
		params.TalentZoneCode = foxconn.NewOptString(f.talentZone)
	}
	if f.keyword != "" {
		params.Keywords = foxconn.NewOptString(f.keyword)
	}

	jobs, err := client.ListJobVacancies(ctx, params)
	if err != nil {
		return err
	}

	out := make([]jobSummaryJSON, len(jobs))
	for i, j := range jobs {
		out[i] = summarize(j)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(out), Jobs: out})
	}

	fmt.Printf("Foxconn (Hon Hai) Jobs Report\n")
	fmt.Printf("Found %d jobs\n\n", len(out))
	for i, s := range out {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

// runDetail fetches one vacancy in full. Unlike the list endpoint, the
// detail endpoint 404s for an unknown id (an RFC 7807 problem+json body,
// decoded here as *foxconn.ProblemDetails).
func runDetail(ctx context.Context, timeout time.Duration, id, format string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := foxconn.NewClient(foxconn.DefaultBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetJobVacancy(ctx, foxconn.GetJobVacancyParams{ID: id})
	if err != nil {
		return err
	}

	switch d := res.(type) {
	case *foxconn.JobVacancy:
		return printDetail(d, format)
	case *foxconn.ProblemDetails:
		return fmt.Errorf("vacancy %q not found", id)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}

// printDetail renders one full vacancy. JSON mode encodes the generated
// JobVacancy as-is — detail is for seeing the whole record.
func printDetail(d *foxconn.JobVacancy, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.JobName)
	fmt.Printf("Job No: %s\n", d.JobNo)
	loc := d.LocName
	if desc, ok := d.LocDesc.Get(); ok && desc != "" {
		loc = fmt.Sprintf("%s (%s)", d.LocName, desc)
	}
	fmt.Printf("Location: %s\n", loc)
	if v, ok := d.EduLevelNameAndDesc.Get(); ok && v != "" {
		fmt.Printf("Education: %s\n", v)
	}
	if v, ok := d.TreatDesc.Get(); ok && v != "" {
		fmt.Printf("Compensation: %s\n", v)
	}
	if v, ok := d.ExpectDate.Get(); ok && v != "" {
		fmt.Printf("Expected start: %s\n", v)
	}

	printSection("Description 1", d.Desc1)
	printSection("Description 2", d.Desc2)
	printSection("Requirements", d.Desc3)
	printSection("Responsibilities", d.Desc4)
	printSection("Description 5", d.Desc5)
	printSection("Description 6", d.Desc6)
	printSection("Description 7", d.Desc7)
	printSection("Description 8", d.Desc8)
	return nil
}

// printSection prints one desc_* free-text block when it is present and
// non-empty.
func printSection(label string, opt foxconn.OptNilString) {
	v, ok := opt.Get()
	if !ok || v == "" {
		return
	}
	fmt.Printf("\n%s:\n%s\n", label, v)
}

// codesJSON is the --format json shape for the codes subcommand.
type codesJSON struct {
	WorkplaceCodes  []foxconn.Code `json:"workplaceCodes"`
	TalentZoneCodes []foxconn.Code `json:"talentZoneCodes"`
}

// runCodes prints the static workplace and talent-zone filter enums
// embedded in the CLI (internal/provider/foxconn/codes.go). It makes no
// network call.
func runCodes(format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(codesJSON{
			WorkplaceCodes:  foxconn.WorkplaceCodes,
			TalentZoneCodes: foxconn.TalentZoneCodes,
		})
	}

	fmt.Println("Workplace codes (--workplace):")
	for _, c := range foxconn.WorkplaceCodes {
		fmt.Printf("  %-12s %s\n", c.Code, c.Name)
	}
	fmt.Println("\nTalent-zone codes (--talent-zone):")
	for _, c := range foxconn.TalentZoneCodes {
		fmt.Printf("  %-12s %s\n", c.Code, c.Name)
	}
	return nil
}
