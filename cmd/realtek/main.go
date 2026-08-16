// Command realtek is a debug CLI for Realtek's recruitment site
// (https://recruit.realtek.com).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/realtek"
)

// apiBaseURL is Realtek's recruitment site origin — the single production
// server in the provider's openapi.yaml.
const apiBaseURL = "https://recruit.realtek.com"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("realtek")
	timeout := rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
	format := rootFlags.StringEnumLong("format", "output format", "text", "json")
	rootCmd := &ff.Command{
		Name:  "realtek",
		Usage: "realtek [FLAGS] <search|detail|types|locations> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword  = searchFS.StringLong("keyword", "", "substring keyword match against title/requirement")
		location = searchFS.StringLong("location", "", "location display name from the 'locations' subcommand (not the id)")
		typeID   = searchFS.StringLong("type-id", "", "job category id from the 'types' subcommand")
		xp       = searchFS.IntLong("xp", -1, "minimum years of experience (N returns Exp >= N; 0 returns only no-experience jobs); -1 means no limit")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "realtek search [--keyword TEXT] [--location NAME] [--type-id ID] [--xp N] [--format text|json]",
		ShortHelp: "search open vacancies (server-side filters)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				timeout:  *timeout,
				keyword:  *keyword,
				location: *location,
				typeID:   *typeID,
				xp:       *xp,
				format:   *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobOppID := detailFS.StringLong("job-opp-id", "", "JobOppId from a search result (the id used in /Job/JobDetail?jobid= links)")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "realtek detail --job-opp-id ID [--format text|json]",
		ShortHelp: "fetch one vacancy's detail by JobOppId",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --job-opp-id %q?)", args, args[0])
			}
			if *jobOppID == "" {
				return errors.New("--job-opp-id is required (take it from a search result's JobOppId)")
			}
			return runDetail(ctx, *timeout, *jobOppID, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

	typesFS := ff.NewFlagSet("types").SetParent(rootFlags)
	typesCmd := &ff.Command{
		Name:      "types",
		Usage:     "realtek types [--format text|json]",
		ShortHelp: "list job category ids/names used to populate --type-id",
		Flags:     typesFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("types takes no positional arguments, got %v", args)
			}
			return runTypes(ctx, *timeout, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, typesCmd)

	locationsFS := ff.NewFlagSet("locations").SetParent(rootFlags)
	locationsCmd := &ff.Command{
		Name:      "locations",
		Usage:     "realtek locations [--format text|json]",
		ShortHelp: "list location ids/names; --location takes the display name, not the id",
		Flags:     locationsFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("locations takes no positional arguments, got %v", args)
			}
			return runLocations(ctx, *timeout, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, locationsCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, detail, types, or locations) is required")
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
	JobOppId string `json:"jobOppId"`
	Title    string `json:"title"`
	Type     string `json:"type,omitempty"`
	Degree   string `json:"degree,omitempty"`
	Exp      string `json:"exp,omitempty"`
	Location string `json:"location,omitempty"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j realtek.JobSummary) jobSummaryJSON {
	return jobSummaryJSON{
		JobOppId: j.JobOppId,
		Title:    j.JobTitle,
		Type:     j.JobType,
		Degree:   j.Degree,
		Exp:      j.Exp,
		Location: j.Location,
	}
}

// printSummary prints one job's compact text block (everything below the
// title line).
func printSummary(s jobSummaryJSON) {
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.Type != "" {
		fmt.Printf("Type: %s\n", s.Type)
	}
	if s.Degree != "" {
		fmt.Printf("Degree: %s\n", s.Degree)
	}
	if s.Exp != "" {
		fmt.Printf("Experience: %s years\n", s.Exp)
	}
	fmt.Printf("JobOppId: %s\n", s.JobOppId)
}

// searchFlags carries the parsed "search" subcommand flags into runSearch.
type searchFlags struct {
	timeout  time.Duration
	keyword  string
	location string
	typeID   string
	xp       int
	format   string
}

// runSearch calls GetFilterList when any filter is set, or the unfiltered
// GetAllJobList dump otherwise (GetFilterList's own defaults, keyword
// omitted and xp -1, make it equivalent to the dump, but the dump avoids
// an unnecessary form-encoded POST). Both unfiltered paths are capped at
// 200 rows by the server; see openapi.yaml.
func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := realtek.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	var res *realtek.JobListResponse
	if f.keyword == "" && f.location == "" && f.typeID == "" && f.xp == -1 {
		res, err = client.ListJobs(ctx)
	} else {
		req := &realtek.FilterJobsReq{
			Xp: realtek.NewOptString(strconv.Itoa(f.xp)),
		}
		if f.keyword != "" {
			req.Keyword = realtek.NewOptString(f.keyword)
		}
		if f.location != "" {
			req.JobLocation = realtek.NewOptString(f.location)
		}
		if f.typeID != "" {
			req.JobTypeID = realtek.NewOptString(f.typeID)
		}
		res, err = client.FilterJobs(ctx, req)
	}
	if err != nil {
		return err
	}

	jobs := make([]jobSummaryJSON, len(res.Data))
	for i, j := range res.Data {
		jobs[i] = summarize(j)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: len(jobs), Jobs: jobs})
	}

	fmt.Printf("Realtek Jobs Report\n")
	fmt.Printf("Found %d jobs\n\n", len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

func runDetail(ctx context.Context, timeout time.Duration, jobOppID, format string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := realtek.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetVacancyDetail(ctx, realtek.GetVacancyDetailParams{JobOppId: jobOppID})
	if err != nil {
		return err
	}

	title, ok := res.Data.JobTitle.Get()
	if !ok {
		return fmt.Errorf("vacancy %q not found", jobOppID)
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res.Data)
	}

	fmt.Println(title)
	if degree, ok := res.Data.Degree.Get(); ok && degree != "" {
		fmt.Printf("Degree: %s\n", degree)
	}
	if exp, ok := res.Data.Exp.Get(); ok && exp != "" {
		fmt.Printf("Experience: %s years\n", exp)
	}
	if loc, ok := res.Data.Location.Get(); ok && loc != "" {
		fmt.Printf("Location: %s\n", loc)
	}
	if req, ok := res.Data.Requirement.Get(); ok && req != "" {
		fmt.Printf("\nRequirement:\n%s\n", req)
	}
	return nil
}

func runTypes(ctx context.Context, timeout time.Duration, format string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := realtek.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.ListJobTypes(ctx)
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res.Data)
	}

	for _, t := range res.Data {
		fmt.Printf("%s\t%s\n", t.JobTypeId, t.JobType)
	}
	return nil
}

func runLocations(ctx context.Context, timeout time.Duration, format string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := realtek.NewClient(apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.ListJobLocations(ctx)
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res.Data)
	}

	for _, l := range res.Data {
		fmt.Printf("%s\t%s\n", l.JobLocationId, l.JobLocation)
	}
	return nil
}
