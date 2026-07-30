package meta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	metaprovider "github.com/amikai/openings-mcp/internal/provider/meta"
)

const defaultBaseURL = "https://www.metacareers.com"

type options struct {
	baseURL string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for meta.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "meta",
		Short:        "meta [FLAGS] <search|detail|filters> [FLAGS]",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", defaultBaseURL, "Meta Careers base URL")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	var (
		searchQ                string
		searchTeams            []string
		searchSubTeams         []string
		searchOffices          []string
		searchRoles            []string
		searchDivisions        []string
		searchLeadershipLevels []string
		searchIsLeadership     bool
		searchRemoteOnly       bool
		searchSortByNew        bool
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search metacareers.com listings (all matches, no pagination)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runSearch(cmd.Context(), searchFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				request: metaprovider.SearchRequest{
					Q:                searchQ,
					Teams:            searchTeams,
					SubTeams:         searchSubTeams,
					Offices:          searchOffices,
					Divisions:        searchDivisions,
					Roles:            searchRoles,
					LeadershipLevels: searchLeadershipLevels,
					IsLeadership:     searchIsLeadership,
					IsRemoteOnly:     searchRemoteOnly,
					SortByNew:        searchSortByNew,
				},
			}, os.Stdout)
		},
	}
	searchCmd.Flags().StringVar(&searchQ, "q", "", "free-text keyword query")
	searchCmd.Flags().StringArrayVar(&searchTeams, "team", nil, `team display name, e.g. "Software Engineering" or "AR/VR" (repeatable)`)
	searchCmd.Flags().StringArrayVar(&searchSubTeams, "sub-team", nil, `sub-team display name, e.g. "Design" (repeatable)`)
	searchCmd.Flags().StringArrayVar(&searchOffices, "office", nil, `office display name or ID, e.g. "Singapore" or "menlo-park" (repeatable)`)
	searchCmd.Flags().StringArrayVar(&searchRoles, "role", nil, `employment type: "Full time employment", "Internship", or "Short term employment" (repeatable)`)
	searchCmd.Flags().StringArrayVar(&searchDivisions, "division", nil, `technology filter, e.g. "Facebook", "Instagram", "Meta Quest" (repeatable; the site's Technology filter submits under the divisions key)`)
	searchCmd.Flags().StringArrayVar(&searchLeadershipLevels, "leadership-level", nil, "leadership level filter (repeatable)")
	searchCmd.Flags().BoolVar(&searchIsLeadership, "leadership", false, "only leadership roles")
	searchCmd.Flags().BoolVar(&searchRemoteOnly, "remote-only", false, "only remote-eligible roles")
	searchCmd.Flags().BoolVar(&searchSortByNew, "sort-by-new", false, "order results by posting date instead of relevance")

	var detailJobID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "fetch one Meta job posting",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v", args)
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runDetail(cmd.Context(), detailFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				jobID:   detailJobID,
			}, os.Stdout)
		},
	}
	detailCmd.Flags().StringVar(&detailJobID, "job-id", "", "requisition ID returned by search (required)")

	filtersCmd := &cobra.Command{
		Use:          "filters",
		Short:        "list the current search filter values (teams, technologies, roles, offices)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("filters takes no positional arguments, got %v", args)
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runFilters(cmd.Context(), filtersFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
			}, os.Stdout)
		},
	}

	rootCmd.AddCommand(searchCmd, detailCmd, filtersCmd)
	return rootCmd
}

type searchFlags struct {
	baseURL string
	format  string
	timeout time.Duration
	request metaprovider.SearchRequest
}

func runSearch(ctx context.Context, flags searchFlags, out io.Writer) error {
	client := metaprovider.NewClient(flags.baseURL, nil)
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	response, err := client.SearchJobs(ctx, flags.request)
	if err != nil {
		return fmt.Errorf("search meta jobs: %w", err)
	}
	return writeSearch(out, flags.format, response)
}

func writeSearch(out io.Writer, format string, response *metaprovider.SearchResponse) error {
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
		fmt.Fprintf(out, "   url: %s\n\n", metaprovider.JobURL(job.ID))
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

	client := metaprovider.NewClient(flags.baseURL, nil)
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	detail, err := client.JobDetail(ctx, flags.jobID)
	if err != nil {
		return fmt.Errorf("get meta job detail: %w", err)
	}
	return writeDetail(out, flags.format, detail)
}

func writeDetail(out io.Writer, format string, detail *metaprovider.JobDetail) error {
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
	fmt.Fprintf(out, "url: %s\n", metaprovider.JobURL(detail.ID))

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
	client := metaprovider.NewClient(flags.baseURL, nil)
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	filters, err := client.SearchFilters(ctx)
	if err != nil {
		return fmt.Errorf("get meta search filters: %w", err)
	}
	return writeFilters(out, flags.format, filters)
}

func writeFilters(out io.Writer, format string, filters *metaprovider.SearchFilters) error {
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
