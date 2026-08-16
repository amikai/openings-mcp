package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/amazon"
)

const _defaultBaseURL = "https://www.amazon.jobs"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("amazon")
	var (
		baseURL = rootFlags.StringLong("base-url", _defaultBaseURL, "Amazon Jobs base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCommand := &ff.Command{
		Name:  "amazon",
		Usage: "amazon [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFlags := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword            = searchFlags.StringLong("keyword", "", "free-text query (empty browses all jobs)")
		countries          = searchFlags.StringSetLong("country", "ISO-3 country code, e.g. TWN (repeatable)")
		cities             = searchFlags.StringSetLong("city", "normalized city name, e.g. Taipei City (repeatable)")
		jobCategories      = searchFlags.StringSetLong("job-category", "job category display name, e.g. Software Development (repeatable)")
		businessCategories = searchFlags.StringSetLong("business-category", "Amazon business-category slug, e.g. aws (repeatable)")
		scheduleTypes      = searchFlags.StringSetLong("schedule-type", "schedule type, e.g. Full-Time (repeatable)")
		sort               = searchFlags.StringEnumLong("sort", "result order", "relevant", "recent")
		offset             = searchFlags.IntLong("offset", 0, "zero-based result offset")
		limit              = searchFlags.IntLong("limit", 10, "page size (1-100)")
	)
	searchCommand := &ff.Command{
		Name:      "search",
		Usage:     "amazon search [--keyword TEXT] [--country ISO3] [--city CITY] [--job-category NAME] [--business-category SLUG] [--schedule-type TYPE] [--sort relevant|recent] [--offset N] [--limit N] [--format text|json]",
		ShortHelp: "search Amazon jobs with server-side filters",
		Flags:     searchFlags,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			return runSearch(ctx, searchOptions{
				baseURL:            *baseURL,
				timeout:            *timeout,
				format:             *format,
				keyword:            *keyword,
				countries:          *countries,
				cities:             *cities,
				jobCategories:      *jobCategories,
				businessCategories: *businessCategories,
				scheduleTypes:      *scheduleTypes,
				sort:               *sort,
				offset:             *offset,
				limit:              *limit,
			})
		},
	}
	rootCommand.Subcommands = append(rootCommand.Subcommands, searchCommand)

	detailFlags := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFlags.StringLong("id", "", "numeric job id from a search result")
	detailCommand := &ff.Command{
		Name:      "detail",
		Usage:     "amazon detail --id JOB-ID [--format text|json]",
		ShortHelp: "print one Amazon job with its full description and qualifications",
		Flags:     detailFlags,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v", args)
			}
			return runDetail(ctx, detailOptions{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				jobID:   *jobID,
			})
		},
	}
	rootCommand.Subcommands = append(rootCommand.Subcommands, detailCommand)

	if err := rootCommand.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCommand.GetSelected()))
		if errors.Is(err, ff.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	if rootCommand.GetSelected() == rootCommand {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCommand))
		fmt.Fprintln(os.Stderr, "err: a subcommand (search or detail) is required")
		return 1
	}
	if err := rootCommand.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

type searchOptions struct {
	baseURL            string
	timeout            time.Duration
	format             string
	keyword            string
	countries          []string
	cities             []string
	jobCategories      []string
	businessCategories []string
	scheduleTypes      []string
	sort               string
	offset             int
	limit              int
}

func (o searchOptions) request() amazon.SearchRequest {
	return amazon.SearchRequest{
		Query:              o.keyword,
		Countries:          o.countries,
		Cities:             o.cities,
		JobCategories:      o.jobCategories,
		BusinessCategories: o.businessCategories,
		ScheduleTypes:      o.scheduleTypes,
		Sort:               amazon.SearchJobsSort(o.sort),
		Offset:             o.offset,
		Limit:              o.limit,
	}
}

type jobSummary struct {
	ID               string `json:"id"`
	URL              string `json:"url"`
	Title            string `json:"title"`
	Location         string `json:"location,omitempty"`
	Normalized       string `json:"normalized_location,omitempty"`
	CountryCode      string `json:"country_code,omitempty"`
	CompanyName      string `json:"company_name,omitempty"`
	JobCategory      string `json:"job_category,omitempty"`
	BusinessCategory string `json:"business_category,omitempty"`
	ScheduleType     string `json:"schedule_type,omitempty"`
	PostedDate       string `json:"posted_date,omitempty"`
	UpdatedTime      string `json:"updated_time,omitempty"`
	Description      string `json:"description,omitempty"`
}

type searchOutput struct {
	Total  int          `json:"total"`
	Offset int          `json:"offset"`
	Jobs   []jobSummary `json:"jobs"`
}

func summarize(job amazon.Job) jobSummary {
	return jobSummary{
		ID:               job.IDIcims,
		URL:              amazon.JobURL(job.JobPath),
		Title:            job.Title,
		Location:         job.Location,
		Normalized:       job.NormalizedLocation,
		CountryCode:      job.CountryCode,
		CompanyName:      job.CompanyName,
		JobCategory:      job.JobCategory,
		BusinessCategory: job.BusinessCategory,
		ScheduleType:     job.JobScheduleType,
		PostedDate:       job.PostedDate,
		UpdatedTime:      job.UpdatedTime,
		Description:      job.DescriptionShort,
	}
}

func runSearch(ctx context.Context, options searchOptions) error {
	ctx, cancel := context.WithTimeout(ctx, options.timeout)
	defer cancel()

	client, err := amazon.NewClient(options.baseURL)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	result, err := client.Search(ctx, options.request())
	if err != nil {
		return err
	}

	jobs := make([]jobSummary, 0, len(result.Jobs))
	for _, job := range result.Jobs {
		jobs = append(jobs, summarize(job))
	}
	output := searchOutput{Total: result.Total, Offset: options.offset, Jobs: jobs}
	if options.format == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}

	fmt.Printf("Amazon Jobs Report\nFound %d jobs; showing %d from offset %d\n\n", output.Total, len(output.Jobs), output.Offset)
	for i, job := range output.Jobs {
		fmt.Printf("%d. %s\n", i+1, job.Title)
		fmt.Printf("ID: %s\nURL: %s\n", job.ID, job.URL)
		if job.Location != "" {
			fmt.Printf("Location: %s\n", job.Location)
		}
		if job.CompanyName != "" {
			fmt.Printf("Entity: %s\n", job.CompanyName)
		}
		if job.PostedDate != "" {
			fmt.Printf("Posted: %s\n", job.PostedDate)
		}
		fmt.Println()
	}
	return nil
}

type detailOptions struct {
	baseURL string
	timeout time.Duration
	format  string
	jobID   string
}

type detailOutput struct {
	jobSummary
	ApplyURL                string `json:"apply_url,omitempty"`
	Description             string `json:"description"`
	BasicQualifications     string `json:"basic_qualifications"`
	PreferredQualifications string `json:"preferred_qualifications"`
}

func toDetail(job amazon.Job) detailOutput {
	return detailOutput{
		jobSummary:              summarize(job),
		ApplyURL:                job.URLNextStep.String(),
		Description:             renderHTML(job.Description),
		BasicQualifications:     renderHTML(job.BasicQualifications),
		PreferredQualifications: renderHTML(job.PreferredQualifications),
	}
}

func runDetail(ctx context.Context, options detailOptions) error {
	if options.jobID == "" {
		return errors.New("--id is required")
	}
	ctx, cancel := context.WithTimeout(ctx, options.timeout)
	defer cancel()

	client, err := amazon.NewClient(options.baseURL)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	job, err := client.JobDetail(ctx, options.jobID)
	if err != nil {
		return err
	}
	detail := toDetail(*job)
	if options.format == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(detail)
	}

	fmt.Println(detail.Title)
	fmt.Printf("ID: %s\nURL: %s\n", detail.ID, detail.URL)
	if detail.ApplyURL != "" {
		fmt.Printf("Apply: %s\n", detail.ApplyURL)
	}
	if detail.Location != "" {
		fmt.Printf("Location: %s\n", detail.Location)
	}
	if detail.CompanyName != "" {
		fmt.Printf("Entity: %s\n", detail.CompanyName)
	}
	if detail.Description != "" {
		fmt.Printf("\nDescription\n%s\n", detail.Description)
	}
	if detail.BasicQualifications != "" {
		fmt.Printf("\nBasic qualifications\n%s\n", detail.BasicQualifications)
	}
	if detail.PreferredQualifications != "" {
		fmt.Printf("\nPreferred qualifications\n%s\n", detail.PreferredQualifications)
	}
	return nil
}

func renderHTML(value string) string {
	text, err := html2text.FromString(value, html2text.Options{})
	if err != nil {
		return value
	}
	return text
}
