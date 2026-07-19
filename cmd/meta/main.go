// Command meta provides a small diagnostic CLI for the Meta Careers
// GraphQL surface.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/meta"
)

const defaultBaseURL = "https://www.metacareers.com"

func main() {
	rootFlags := ff.NewFlagSet("meta")
	var (
		baseURL = rootFlags.StringLong("base-url", defaultBaseURL, "Meta Careers base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "meta",
		Usage: "meta [FLAGS] <search|detail|filters> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		q                = searchFS.StringLong("q", "", "free-text keyword query")
		teams            = searchFS.StringSetLong("team", `team display name, e.g. "Software Engineering" or "AR/VR" (repeatable)`)
		subTeams         = searchFS.StringSetLong("sub-team", `sub-team display name, e.g. "Design" (repeatable)`)
		offices          = searchFS.StringSetLong("office", `office display name or ID, e.g. "Singapore" or "menlo-park" (repeatable)`)
		roles            = searchFS.StringSetLong("role", `employment type: "Full time employment", "Internship", or "Short term employment" (repeatable)`)
		divisions        = searchFS.StringSetLong("division", `technology filter, e.g. "Facebook", "Instagram", "Meta Quest" (repeatable; the site's Technology filter submits under the divisions key)`)
		leadershipLevels = searchFS.StringSetLong("leadership-level", "leadership level filter (repeatable)")
		isLeadership     = searchFS.BoolLong("leadership", "only leadership roles")
		remoteOnly       = searchFS.BoolLong("remote-only", "only remote-eligible roles")
		sortByNew        = searchFS.BoolLong("sort-by-new", "order results by posting date instead of relevance")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "meta search [--q TEXT] [--team NAME] [--office NAME] [--role NAME] [--remote-only] ...",
		ShortHelp: "search metacareers.com listings (all matches, no pagination)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			return runSearch(ctx, searchFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				request: meta.SearchRequest{
					Q:                *q,
					Teams:            *teams,
					SubTeams:         *subTeams,
					Offices:          *offices,
					Divisions:        *divisions,
					Roles:            *roles,
					LeadershipLevels: *leadershipLevels,
					IsLeadership:     *isLeadership,
					IsRemoteOnly:     *remoteOnly,
					SortByNew:        *sortByNew,
				},
			}, os.Stdout)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("job-id", "", "requisition ID returned by search (required)")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "meta detail --job-id ID",
		ShortHelp: "fetch one Meta job posting",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v", args)
			}
			return runDetail(ctx, detailFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				jobID:   *jobID,
			}, os.Stdout)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

	filtersFS := ff.NewFlagSet("filters").SetParent(rootFlags)
	filtersCmd := &ff.Command{
		Name:      "filters",
		Usage:     "meta filters",
		ShortHelp: "list the current search filter values (teams, technologies, roles, offices)",
		Flags:     filtersFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("filters takes no positional arguments, got %v", args)
			}
			return runFilters(ctx, filtersFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
			}, os.Stdout)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, filtersCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, detail, or filters) is required")
		os.Exit(1)
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

type searchFlags struct {
	baseURL string
	format  string
	timeout time.Duration
	request meta.SearchRequest
}

func runSearch(ctx context.Context, flags searchFlags, out io.Writer) error {
	client := meta.NewClient(flags.baseURL, nil)
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	response, err := client.SearchJobs(ctx, flags.request)
	if err != nil {
		return fmt.Errorf("search meta jobs: %w", err)
	}
	return writeSearch(out, flags.format, response)
}

func writeSearch(out io.Writer, format string, response *meta.SearchResponse) error {
	if format == "json" {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("encode search response: %w", err)
		}
		return nil
	}

	fmt.Fprintf(out, "jobs=%d featured=%d (site-wide, unrelated to filters)\n\n", len(response.AllJobs), len(response.FeaturedJobs))
	for index, job := range response.AllJobs {
		fmt.Fprintf(out, "%d. [%s] %s\n", index+1, job.ID, job.Title)
		if len(job.Teams) > 0 {
			fmt.Fprintf(out, "   teams: %s\n", strings.Join(job.Teams, "; "))
		}
		if len(job.SubTeams) > 0 {
			fmt.Fprintf(out, "   sub-teams: %s\n", strings.Join(job.SubTeams, "; "))
		}
		if len(job.Locations) > 0 {
			fmt.Fprintf(out, "   locations: %s\n", strings.Join(job.Locations, "; "))
		}
		fmt.Fprintf(out, "   url: %s\n\n", meta.JobURL(job.ID))
	}
	return nil
}

type detailFlags struct {
	baseURL string
	format  string
	jobID   string
	timeout time.Duration
}

func runDetail(ctx context.Context, flags detailFlags, out io.Writer) error {
	if strings.TrimSpace(flags.jobID) == "" {
		return errors.New("--job-id is required")
	}

	client := meta.NewClient(flags.baseURL, nil)
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	detail, err := client.JobDetail(ctx, flags.jobID)
	if err != nil {
		return fmt.Errorf("get meta job detail: %w", err)
	}
	return writeDetail(out, flags.format, detail)
}

func writeDetail(out io.Writer, format string, detail *meta.JobDetail) error {
	if format == "json" {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(detail); err != nil {
			return fmt.Errorf("encode detail response: %w", err)
		}
		return nil
	}

	fmt.Fprintf(out, "[%s] %s\n", detail.ID, detail.Title)
	if len(detail.Departments) > 0 {
		fmt.Fprintf(out, "teams: %s\n", strings.Join(detail.Departments, "; "))
	}
	if len(detail.InternalDepartments) > 0 {
		fmt.Fprintf(out, "sub-teams: %s\n", strings.Join(detail.InternalDepartments, "; "))
	}
	if len(detail.Locations) > 0 {
		fmt.Fprintf(out, "locations: %s\n", strings.Join(detail.Locations, "; "))
	}
	for _, comp := range detail.PublicCompensation {
		fmt.Fprintf(out, "compensation (%s): %s - %s bonus=%t equity=%t\n",
			comp.CountryCode, comp.Minimum, comp.Maximum, comp.HasBonus, comp.HasEquity)
	}
	fmt.Fprintf(out, "url: %s\n", meta.JobURL(detail.ID))

	writeSection(out, "Description (HTML)", detail.DescriptionHTML)
	writeList(out, "Responsibilities", detail.Responsibilities)
	writeList(out, "Minimum qualifications", detail.MinimumQualifications)
	writeList(out, "Preferred qualifications", detail.PreferredQualifications)
	return nil
}

type filtersFlags struct {
	baseURL string
	format  string
	timeout time.Duration
}

func runFilters(ctx context.Context, flags filtersFlags, out io.Writer) error {
	client := meta.NewClient(flags.baseURL, nil)
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	filters, err := client.SearchFilters(ctx)
	if err != nil {
		return fmt.Errorf("get meta search filters: %w", err)
	}
	return writeFilters(out, flags.format, filters)
}

func writeFilters(out io.Writer, format string, filters *meta.SearchFilters) error {
	if format == "json" {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(filters); err != nil {
			return fmt.Errorf("encode filters response: %w", err)
		}
		return nil
	}

	writeList(out, "Teams (--team)", filters.Teams)
	writeList(out, "Technologies (--division)", filters.Technologies)
	writeList(out, "Roles (--role)", filters.Roles)
	fmt.Fprintf(out, "\nOffices (--office; display name or ID)\n")
	for _, location := range filters.Locations {
		remote := ""
		if location.IsRemote {
			remote = " (remote)"
		}
		fmt.Fprintf(out, "- %s [%s]%s\n", location.DisplayName, location.ID, remote)
	}
	return nil
}

func writeSection(out io.Writer, heading, body string) {
	if strings.TrimSpace(body) != "" {
		fmt.Fprintf(out, "\n%s\n%s\n", heading, body)
	}
}

func writeList(out io.Writer, heading string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(out, "\n%s\n", heading)
	for _, item := range items {
		fmt.Fprintf(out, "- %s\n", item)
	}
}
