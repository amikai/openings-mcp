// Package realtek implements the "openings-mcp realtek" debug CLI, for
// manual checks against the live surface that internal/provider/realtek
// documents.
package realtek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	realtekprovider "github.com/amikai/openings-mcp/internal/provider/realtek"
)

type options struct {
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for realtek.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "realtek",
		Short:        "Search Realtek jobs, view position details, and list types/locations",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	clihelp.FormatVar(rootCmd.PersistentFlags(), &opts.format)

	var (
		searchKeyword  string
		searchLocation string
		searchTypeID   string
		searchXp       int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search open vacancies (server-side filters)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), searchFlags{
				timeout:  opts.timeout,
				keyword:  searchKeyword,
				location: searchLocation,
				typeID:   searchTypeID,
				xp:       searchXp,
				format:   opts.format,
			})
		},
	}

	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "substring keyword match against title/requirement")
	searchCmd.Flags().StringVar(&searchLocation, "location", "", "location display name from the 'locations' subcommand (not the id)")
	searchCmd.Flags().StringVar(&searchTypeID, "type-id", "", "job category id from the 'types' subcommand")
	searchCmd.Flags().IntVar(&searchXp, "xp", -1, "minimum years of experience (N returns Exp >= N; 0 returns only no-experience jobs); -1 means no limit")

	var detailJobOppID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "fetch one vacancy's detail by JobOppId",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if detailJobOppID == "" {
				return errors.New("--job-opp-id is required (take it from a search result's JobOppId)")
			}
			return runDetail(cmd.Context(), opts.timeout, detailJobOppID, opts.format)
		},
	}

	detailCmd.Flags().StringVar(&detailJobOppID, "job-opp-id", "", "JobOppId from a search result (the id used in /Job/JobDetail?jobid= links)")

	typesCmd := &cobra.Command{
		Use:          "types",
		Short:        "list job category ids/names used to populate --type-id",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTypes(cmd.Context(), opts.timeout, opts.format)
		},
	}

	locationsCmd := &cobra.Command{
		Use:          "locations",
		Short:        "list location ids/names; --location takes the display name, not the id",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLocations(cmd.Context(), opts.timeout, opts.format)
		},
	}

	rootCmd.AddCommand(searchCmd, detailCmd, typesCmd, locationsCmd)
	return rootCmd
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

func summarize(j realtekprovider.JobSummary) jobSummaryJSON {
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

	client, err := realtekprovider.NewClient(realtekprovider.DefaultBaseURL)
	if err != nil {
		return err
	}

	var res *realtekprovider.JobListResponse
	if f.keyword == "" && f.location == "" && f.typeID == "" && f.xp == -1 {
		res, err = client.ListJobs(ctx)
	} else {
		req := &realtekprovider.FilterJobsReq{
			Xp: realtekprovider.NewOptString(strconv.Itoa(f.xp)),
		}
		if f.keyword != "" {
			req.Keyword = realtekprovider.NewOptString(f.keyword)
		}
		if f.location != "" {
			req.JobLocation = realtekprovider.NewOptString(f.location)
		}
		if f.typeID != "" {
			req.JobTypeID = realtekprovider.NewOptString(f.typeID)
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

	client, err := realtekprovider.NewClient(realtekprovider.DefaultBaseURL)
	if err != nil {
		return err
	}

	res, err := client.GetVacancyDetail(ctx, realtekprovider.GetVacancyDetailParams{JobOppId: jobOppID})
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

	client, err := realtekprovider.NewClient(realtekprovider.DefaultBaseURL)
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

	client, err := realtekprovider.NewClient(realtekprovider.DefaultBaseURL)
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
