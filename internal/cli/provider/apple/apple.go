// Package apple provides CLI command for Apple Jobs.
package apple

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

	"github.com/amikai/openings-mcp/internal/provider/apple"
)

const (
	defaultBaseURL = "https://jobs.apple.com"
	jsonFormat     = "json"
)

type options struct {
	baseURL string
	timeout time.Duration
	format  string
}

type searchFlags struct {
	baseURL        string
	format         string
	keyword        string
	country        string
	sort           string
	locations      []string
	filterKeywords []string
	teams          []string
	products       []string
	languages      []string
	timeout        time.Duration
	page           int
	homeOffice     bool
}

type detailFlags struct {
	baseURL string
	format  string
	jobID   string
	timeout time.Duration
}

type filterFlags struct {
	baseURL string
	format  string
	timeout time.Duration
}

// NewCommand returns a cobra.Command for apple.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "apple",
		Short:        "Apple Jobs diagnostic CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", defaultBaseURL, "Apple Jobs API base URL")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search jobs.apple.com listings",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sFlags.baseURL = opts.baseURL
			sFlags.timeout = opts.timeout
			sFlags.format = opts.format
			return runSearch(cmd.Context(), *sFlags, os.Stdout)
		},
	}

	searchCmd.Flags().StringVar(&sFlags.keyword, "keyword", "", "keyword query (required)")
	searchCmd.Flags().StringVar(&sFlags.country, "country", "", "ISO 3166-1 alpha-3 country code, e.g. TWN or USA")
	searchCmd.Flags().StringSliceVar(&sFlags.locations, "location", nil, "location code (repeatable)")
	searchCmd.Flags().StringVar(&sFlags.sort, "sort", "relevance", "result order")
	searchCmd.Flags().IntVar(&sFlags.page, "page", 1, "1-based page of 20 results")
	searchCmd.Flags().BoolVar(&sFlags.homeOffice, "home-office", false, "only remote-eligible postings")
	searchCmd.Flags().StringSliceVar(&sFlags.filterKeywords, "filter-keyword", nil, "extra keyword filter chip (repeatable)")
	searchCmd.Flags().StringSliceVar(&sFlags.teams, "team", nil, "team filter as TEAM/SUBTEAM codes (repeatable)")
	searchCmd.Flags().StringSliceVar(&sFlags.products, "product", nil, "product code (repeatable)")
	searchCmd.Flags().StringSliceVar(&sFlags.languages, "language", nil, "language code (repeatable)")

	dFlags := &detailFlags{}
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "fetch one Apple job posting",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dFlags.baseURL = opts.baseURL
			dFlags.timeout = opts.timeout
			dFlags.format = opts.format
			return runDetail(cmd.Context(), *dFlags, os.Stdout)
		},
	}

	detailCmd.Flags().StringVar(&dFlags.jobID, "job-id", "", "numeric position ID returned by search (required)")

	filtersCmd := &cobra.Command{
		Use:          "filters",
		Short:        "list team and product filter codes for search",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFilters(cmd.Context(), filterFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
			}, os.Stdout)
		},
	}

	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(detailCmd)
	rootCmd.AddCommand(filtersCmd)

	return rootCmd
}

func runSearch(ctx context.Context, flags searchFlags, out io.Writer) error {
	if strings.TrimSpace(flags.keyword) == "" {
		return errors.New("--keyword is required")
	}
	if strings.TrimSpace(flags.country) == "" && len(flags.locations) == 0 {
		return errors.New("--country or --location is required")
	}
	if flags.page < 1 {
		return fmt.Errorf("--page must be >= 1, got %d", flags.page)
	}
	teams, err := teamFilters(flags.teams)
	if err != nil {
		return err
	}

	client, err := apple.NewJobsClient(flags.baseURL, nil)
	if err != nil {
		return fmt.Errorf("create apple client: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	response, err := client.SearchJobs(ctx, apple.SearchRequest{
		Keyword:     flags.keyword,
		CountryCode: flags.country,
		Locations:   flags.locations,
		Sort:        apple.Sort(flags.sort),
		Page:        flags.page,
		HomeOffice:  flags.homeOffice,
		Keywords:    flags.filterKeywords,
		Teams:       teams,
		Products:    flags.products,
		Languages:   flags.languages,
	})
	if err != nil {
		return fmt.Errorf("search apple jobs: %w", err)
	}
	return writeSearch(out, flags.format, flags.page, response)
}

func teamFilters(values []string) ([]apple.TeamFilter, error) {
	teams := make([]apple.TeamFilter, 0, len(values))
	for _, value := range values {
		team, err := apple.ParseTeamFilter(value)
		if err != nil {
			return nil, fmt.Errorf("--team: %w", err)
		}
		teams = append(teams, team)
	}
	return teams, nil
}

func writeSearch(out io.Writer, format string, page int, response *apple.SearchResponse) error {
	if format == jsonFormat {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("encode search response: %w", err)
		}
		return nil
	}

	result := response.Res
	fmt.Fprintf(out, "total=%d page=%d jobs=%d\n\n", result.TotalRecords, page, len(result.SearchResults))
	for index, job := range result.SearchResults {
		fmt.Fprintf(out, "%d. [%s] %s\n", index+1, job.PositionId, job.PostingTitle)
		if job.Team.TeamName != "" {
			fmt.Fprintf(out, "   team: %s\n", job.Team.TeamName)
		}
		for _, location := range job.Locations {
			fmt.Fprintf(out, "   location: %s\n", locationLabel(location.Name, location.CountryName))
		}
		if job.PostingDate != "" {
			fmt.Fprintf(out, "   posted: %s\n", job.PostingDate)
		}
		fmt.Fprintf(out, "   url: %s\n\n", apple.JobURL(job.PositionId, job.TransformedPostingTitle))
	}
	return nil
}

func runFilters(ctx context.Context, flags filterFlags, out io.Writer) error {
	client, err := apple.NewJobsClient(flags.baseURL, nil)
	if err != nil {
		return fmt.Errorf("create apple client: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	teams, err := client.ListTeams(ctx)
	if err != nil {
		return fmt.Errorf("list apple teams: %w", err)
	}
	return writeFilters(out, flags.format, teams)
}

func writeFilters(out io.Writer, format string, teams *apple.TeamsResponse) error {
	if format == jsonFormat {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(map[string]any{"teams": teams.Res, "products": apple.Products}); err != nil {
			return fmt.Errorf("encode filters: %w", err)
		}
		return nil
	}

	fmt.Fprintln(out, "Teams (--team TEAM/SUB):")
	for _, group := range teams.Res {
		for _, subTeam := range group.Teams {
			fmt.Fprintf(out, "  %s/%s\t%s\n", subTeam.TeamCode, subTeam.Code, subTeam.DisplayName)
		}
	}
	fmt.Fprintln(out, "\nProducts (--product CODE):")
	for _, product := range apple.Products {
		fmt.Fprintf(out, "  %s\t%s\n", product.Code, product.Name)
	}
	return nil
}

func runDetail(ctx context.Context, flags detailFlags, out io.Writer) error {
	if strings.TrimSpace(flags.jobID) == "" {
		return errors.New("--job-id is required")
	}

	client, err := apple.NewJobsClient(flags.baseURL, nil)
	if err != nil {
		return fmt.Errorf("create apple client: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()

	response, err := client.JobDetail(ctx, flags.jobID)
	if err != nil {
		return fmt.Errorf("get apple job detail: %w", err)
	}
	return writeDetail(out, flags.format, response)
}

func writeDetail(out io.Writer, format string, response *apple.JobDetailResponse) error {
	if format == jsonFormat {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("encode detail response: %w", err)
		}
		return nil
	}

	job := response.Res
	fmt.Fprintf(out, "[%s] %s\n", job.PositionId, job.PostingTitle)
	for _, team := range job.TeamNames {
		fmt.Fprintf(out, "team: %s\n", team)
	}
	for _, location := range job.Locations {
		fmt.Fprintf(out, "location: %s\n", locationLabel(location.Name, location.CountryName))
	}
	if employmentType := job.EmploymentType.Or(""); employmentType != "" {
		fmt.Fprintf(out, "employment_type: %s\n", employmentType)
	}
	if job.PostingDate != "" {
		fmt.Fprintf(out, "posted: %s\n", job.PostingDate)
	}
	fmt.Fprintf(out, "url: %s\n", apple.JobURL(job.PositionId, job.TransformedPostingTitle))

	writeSection(out, "Summary", job.JobSummary.Or(""))
	writeSection(out, "Description", job.Description.Or(""))
	writeSection(out, "Responsibilities", job.Responsibilities.Or(""))
	writeSection(out, "Minimum qualifications", job.MinimumQualifications.Or(""))
	writeSection(out, "Preferred qualifications", job.PreferredQualifications.Or(""))
	return nil
}

func writeSection(out io.Writer, heading, body string) {
	if strings.TrimSpace(body) != "" {
		fmt.Fprintf(out, "\n%s\n%s\n", heading, body)
	}
}

func locationLabel(name, country string) string {
	if name == "" {
		return country
	}
	if country == "" || strings.EqualFold(name, country) {
		return name
	}
	return name + ", " + country
}
