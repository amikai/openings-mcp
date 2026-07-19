// Command apple provides a small diagnostic CLI for the Apple Jobs API.
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

	"github.com/amikai/openings-mcp/internal/provider/apple"
)

const (
	defaultBaseURL = "https://jobs.apple.com"
	jsonFormat     = "json"
)

func main() {
	rootFlags := ff.NewFlagSet("apple")
	var (
		baseURL = rootFlags.StringLong("base-url", defaultBaseURL, "Apple Jobs API base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "apple",
		Usage: "apple [FLAGS] <search|detail|filters> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword        = searchFS.StringLong("keyword", "", "keyword query (required)")
		country        = searchFS.StringLong("country", "", "ISO 3166-1 alpha-3 country code, e.g. TWN or USA (required unless --location is set)")
		locations      = searchFS.StringListLong("location", "case-sensitive location code at any granularity, e.g. TPEI or state953, OR'd with --country (repeatable)")
		sort           = searchFS.StringEnumLong("sort", "result order", "relevance", "newest", "teamAsc", "teamDesc", "locationAsc", "locationDesc")
		page           = searchFS.IntLong("page", 1, "1-based page of 20 results")
		homeOffice     = searchFS.BoolLong("home-office", "only remote-eligible postings")
		filterKeywords = searchFS.StringListLong("filter-keyword", "extra keyword filter chip (repeatable)")
		teams          = searchFS.StringListLong("team", "team filter as TEAM/SUBTEAM codes, e.g. HRDWR/CAM (repeatable)")
		products       = searchFS.StringListLong("product", "product code, e.g. IPHN (repeatable)")
		languages      = searchFS.StringListLong("language", "language code, e.g. en_US (repeatable)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "apple search --keyword TEXT [--country ISO3] [--location CODE] [--sort ORDER] [--page N] [--home-office] [--filter-keyword TEXT] [--team TEAM/SUB] [--product CODE] [--language CODE]",
		ShortHelp: "search jobs.apple.com listings",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			return runSearch(ctx, searchFlags{
				baseURL:        *baseURL,
				timeout:        *timeout,
				format:         *format,
				keyword:        *keyword,
				country:        *country,
				locations:      *locations,
				sort:           *sort,
				page:           *page,
				homeOffice:     *homeOffice,
				filterKeywords: *filterKeywords,
				teams:          *teams,
				products:       *products,
				languages:      *languages,
			}, os.Stdout)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("job-id", "", "numeric position ID returned by search (required)")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "apple detail --job-id ID",
		ShortHelp: "fetch one Apple job posting",
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
		Usage:     "apple filters",
		ShortHelp: "list team and product filter codes for search",
		Flags:     filtersFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("filters takes no positional arguments, got %v", args)
			}
			return runFilters(ctx, filterFlags{
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

type filterFlags struct {
	baseURL string
	format  string
	timeout time.Duration
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
