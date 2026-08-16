// Command delta is a debug CLI for Delta Electronics' global recruitment
// portal (https://rws.deltaww.com).
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

	"github.com/amikai/openings-mcp/internal/provider/delta"
)

// apiBaseURL is Delta's careers portal origin.
const _apiBaseURL = "https://rws.deltaww.com"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("delta")
	timeout := rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
	format := rootFlags.StringEnumLong("format", "output format", "text", "json")
	rootCmd := &ff.Command{
		Name:  "delta",
		Usage: "delta [FLAGS] <search|detail|areas> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		area    = searchFS.StringLong("area", "", "area code filter (e.g. A, B, C, or A;B); see the 'areas' subcommand")
		keyword = searchFS.StringLong("keyword", "", "job title keyword filter (case-insensitive substring)")
		lang    = searchFS.StringLong("lang", "en-US", "language tag (e.g. en-US, zh-TW, zh-CN)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "delta search [--area CODE] [--keyword TEXT] [--lang LANG] [--format text|json]",
		ShortHelp: "search open job vacancies (server-side filters)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				timeout: *timeout,
				area:    *area,
				keyword: *keyword,
				lang:    *lang,
				format:  *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	var (
		id         = detailFS.StringLong("id", "", "EmpAddID from a search result (e.g. C20260814001)")
		detailLang = detailFS.StringLong("lang", "en-US", "language tag (e.g. en-US, zh-TW, zh-CN)")
	)
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "delta detail --id ID [--lang LANG] [--format text|json]",
		ShortHelp: "fetch full job details by EmpAddID",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			if *id == "" {
				return errors.New("--id is required (take it from a search result's EmpAddID)")
			}
			return runDetail(ctx, *timeout, *id, *detailLang, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

	areasFS := ff.NewFlagSet("areas").SetParent(rootFlags)
	areasLang := areasFS.StringLong("lang", "en-US", "language tag (e.g. en-US, zh-TW, zh-CN)")
	areasCmd := &ff.Command{
		Name:      "areas",
		Usage:     "delta areas [--lang LANG] [--format text|json]",
		ShortHelp: "list available area codes and their display names",
		Flags:     areasFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("areas takes no positional arguments, got %v", args)
			}
			return runAreas(ctx, *timeout, *areasLang, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, areasCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, detail, or areas) is required")
		return 1
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

type jobSummaryJSON struct {
	EmpAddID     string `json:"empAddId"`
	JobCode      string `json:"jobCode"`
	JobName      string `json:"jobName"`
	AreaName     string `json:"areaName,omitempty"`
	JobPlaceName string `json:"jobPlaceName,omitempty"`
	Education    string `json:"education,omitempty"`
	Experience   string `json:"experience,omitempty"`
	NeedNum      int    `json:"needNum,omitempty"`
	ModifyDate   string `json:"modifyDate,omitempty"`
}

type searchResultJSON struct {
	Total int              `json:"total"`
	Jobs  []jobSummaryJSON `json:"jobs"`
}

func summarize(j delta.JobSummary) jobSummaryJSON {
	s := jobSummaryJSON{
		EmpAddID: j.EmpAddID,
		JobCode:  j.JobCode,
		JobName:  j.JobName,
	}
	if v, ok := j.AreaName.Get(); ok {
		s.AreaName = v
	}
	if v, ok := j.JobPlaceName.Get(); ok {
		s.JobPlaceName = v
	}
	if v, ok := j.Education.Get(); ok {
		s.Education = v
	}
	if v, ok := j.Experience.Get(); ok {
		s.Experience = v
	}
	if v, ok := j.NeedNum.Get(); ok {
		s.NeedNum = v
	}
	if v, ok := j.ModifyDate.Get(); ok {
		s.ModifyDate = v
	}
	return s
}

func printSummary(s jobSummaryJSON) {
	switch {
	case s.AreaName != "" && s.JobPlaceName != "" && s.AreaName != s.JobPlaceName:
		fmt.Printf("Location: %s (%s)\n", s.AreaName, s.JobPlaceName)
	case s.AreaName != "":
		fmt.Printf("Location: %s\n", s.AreaName)
	case s.JobPlaceName != "":
		fmt.Printf("Location: %s\n", s.JobPlaceName)
	}
	if s.Education != "" {
		fmt.Printf("Education: %s\n", s.Education)
	}
	if s.Experience != "" {
		fmt.Printf("Experience: %s\n", s.Experience)
	}
	if s.NeedNum > 0 {
		fmt.Printf("Headcount: %d\n", s.NeedNum)
	}
	if s.ModifyDate != "" {
		fmt.Printf("Updated: %s\n", s.ModifyDate)
	}
	fmt.Printf("Job Code: %s\n", s.JobCode)
	fmt.Printf("ID: %s\n", s.EmpAddID)
}

type searchFlags struct {
	timeout time.Duration
	area    string
	keyword string
	lang    string
	format  string
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := delta.NewClient(_apiBaseURL)
	if err != nil {
		return err
	}

	params := delta.SearchJobListParams{
		AreaID:     f.area,
		AddJobName: f.keyword,
		Lang:       f.lang,
	}

	jobs, err := client.SearchJobList(ctx, params)
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

	fmt.Printf("Delta Electronics Jobs Report\n")
	fmt.Printf("Found %d jobs\n\n", len(out))
	for i, s := range out {
		fmt.Printf("%d. %s\n", i+1, s.JobName)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

func runDetail(ctx context.Context, timeout time.Duration, id, lang, format string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := delta.NewClient(_apiBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetJobDetails(ctx, delta.GetJobDetailsParams{
		EmpAddID: id,
		Resumeid: "",
		Lang:     lang,
	})
	if err != nil {
		return err
	}

	if len(res.JobDetails) == 0 {
		return fmt.Errorf("job vacancy with ID %q not found", id)
	}

	detail := res.JobDetails[0]

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Printf("Job Title: %s\n", detail.JobName)
	fmt.Printf("Requisition ID: %s\n", detail.EmpAddID)
	fmt.Printf("Job Code: %s\n", detail.JobCode)
	if v, ok := detail.AreaName.Get(); ok && v != "" {
		fmt.Printf("Area: %s\n", v)
	}
	if v, ok := detail.JobPlace.Get(); ok && v != "" {
		fmt.Printf("Workplace: %s\n", v)
	}
	if v, ok := detail.Education.Get(); ok && v != "" {
		fmt.Printf("Education: %s\n", v)
	}
	if v, ok := detail.JobExperience.Get(); ok && v != "" {
		fmt.Printf("Experience: %s\n", v)
	}
	if v, ok := detail.LanguageAbility.Get(); ok && v != "" {
		fmt.Printf("Language Ability: %s\n", v)
	}
	if v, ok := detail.NeedNum.Get(); ok && v > 0 {
		fmt.Printf("Headcount: %d\n", v)
	}
	if v, ok := detail.BeginDate.Get(); ok && v != "" {
		fmt.Printf("Posting Date: %s\n", v)
	}
	if v, ok := detail.JobResponsibility.Get(); ok && v != "" {
		fmt.Printf("\nResponsibilities:\n%s\n", v)
	}
	if v, ok := detail.JobSkill.Get(); ok && v != "" {
		fmt.Printf("\nSkills:\n%s\n", v)
	}

	if len(res.RelatedJob) > 0 {
		fmt.Printf("\nRelated Jobs (%d):\n", len(res.RelatedJob))
		for _, r := range res.RelatedJob {
			fmt.Printf("- %s (ID: %s)\n", r.JobName, r.EmpAddID)
		}
	}

	return nil
}

func runAreas(ctx context.Context, timeout time.Duration, lang, format string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := delta.NewClient(_apiBaseURL)
	if err != nil {
		return err
	}

	areas, err := client.GetAreaList(ctx, delta.GetAreaListParams{
		Lang: lang,
	})
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(areas)
	}

	fmt.Printf("%-8s %s\n", "CODE", "NAME")
	fmt.Printf("%-8s %s\n", "----", "----")
	for _, a := range areas {
		fmt.Printf("%-8s %s\n", a.Value, a.Text)
	}
	return nil
}
