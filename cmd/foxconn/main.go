// Command foxconn is a debug CLI for Hon Hai / Foxconn's Taiwan careers API
// (https://recruit.foxconn.com/isite-web-tw/main/jobsearch).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/foxconn"
)

// apiBaseURL is Foxconn's Taiwan careers origin — the single production
// server in the provider's openapi.yaml.
const apiBaseURL = "https://recruit.foxconn.com"

func main() {
	rootFlags := ff.NewFlagSet("foxconn")
	timeout := rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
	format := rootFlags.StringEnumLong("format", "output format", "text", "json")
	rootCmd := &ff.Command{
		Name:  "foxconn",
		Usage: "foxconn [FLAGS] <search|detail|codes> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		workplace  = searchFS.StringLong("workplace", "", "workplaceCode location filter (e.g. TA, CH, VM); see the 'codes' subcommand")
		talentZone = searchFS.StringLong("talent-zone", "", "talentZoneCode recruitment-track filter (e.g. MA, TALENTS, INTERN); see the 'codes' subcommand")
		keyword    = searchFS.StringLong("keyword", "", "case-insensitive free-text search across title and body")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "foxconn search [--workplace CODE] [--talent-zone CODE] [--keyword TEXT] [--format text|json]",
		ShortHelp: "search job vacancies (server-side filters; no pagination)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				timeout:    *timeout,
				workplace:  *workplace,
				talentZone: *talentZone,
				keyword:    *keyword,
				format:     *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	id := detailFS.StringLong("id", "", "opaque job id from a search result (not the job_no)")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "foxconn detail --id ID [--format text|json]",
		ShortHelp: "fetch one vacancy's full detail by its opaque id",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			if *id == "" {
				return errors.New("--id is required (take it from a search result's ID, not the job_no)")
			}
			return runDetail(ctx, *timeout, *id, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

	codesFS := ff.NewFlagSet("codes").SetParent(rootFlags)
	codesCmd := &ff.Command{
		Name:      "codes",
		Usage:     "foxconn codes [--format text|json]",
		ShortHelp: "list the valid --workplace and --talent-zone filter codes (static, no network)",
		Flags:     codesFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("codes takes no positional arguments, got %v", args)
			}
			return runCodes(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, codesCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, detail, or codes) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
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
	// loc_desc is a more specific place (e.g. 鄭州) when present.
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

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	timeout    time.Duration
	workplace  string
	talentZone string
	keyword    string
	format     string
}

// runSearch maps every flag onto the API's real server-side filters. The
// list endpoint has no pagination — it returns the full matching set in one
// response — so an unfiltered call returns the entire ~953-job board.
func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := foxconn.NewClient(apiBaseURL)
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

	client, err := foxconn.NewClient(apiBaseURL)
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

	// desc_1..desc_8 are free-text sections with no field-level label from
	// the API; print each that is present, in order.
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
