// Package amazon implements the "openings-mcp amazon" debug CLI, for manual
// checks against the live surface that internal/provider/amazon documents.
package amazon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	"github.com/amikai/openings-mcp/internal/provider/amazon"
)

type options struct {
	baseURL string
	timeout time.Duration
	format  string
}

type searchOptions struct {
	options
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

type detailOptions struct {
	options
	jobID string
}

// NewCommand returns a cobra.Command for amazon.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "amazon",
		Short:        "Search Amazon jobs and view position details",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", amazon.DefaultBaseURL, "Amazon Jobs base URL")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	clihelp.FormatVar(rootCmd.PersistentFlags(), &opts.format)

	sOpts := &searchOptions{options: *opts}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search Amazon jobs with server-side filters",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sOpts.options = *opts
			return runSearch(cmd.Context(), *sOpts)
		},
	}

	searchCmd.Flags().StringVar(&sOpts.keyword, "keyword", "", "free-text query (empty browses all jobs)")
	searchCmd.Flags().StringArrayVar(&sOpts.countries, "country", nil, "ISO-3 country code, e.g. TWN (repeatable)")
	searchCmd.Flags().StringArrayVar(&sOpts.cities, "city", nil, "normalized city name, e.g. Taipei City (repeatable)")
	searchCmd.Flags().StringArrayVar(&sOpts.jobCategories, "job-category", nil, "job category display name, e.g. Software Development (repeatable)")
	searchCmd.Flags().StringArrayVar(&sOpts.businessCategories, "business-category", nil, "Amazon business-category slug, e.g. aws (repeatable)")
	searchCmd.Flags().StringArrayVar(&sOpts.scheduleTypes, "schedule-type", nil, "schedule type, e.g. Full-Time (repeatable)")
	clihelp.ChoiceVar(searchCmd.Flags(), &sOpts.sort, "sort", "relevant", []string{"relevant", "recent"}, "result order")
	searchCmd.Flags().IntVar(&sOpts.offset, "offset", 0, "zero-based result offset")
	searchCmd.Flags().IntVar(&sOpts.limit, "limit", 10, "page size (1-100)")

	dOpts := &detailOptions{options: *opts}
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one Amazon job with its full description and qualifications",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dOpts.options = *opts
			return runDetail(cmd.Context(), *dOpts)
		},
	}

	detailCmd.Flags().StringVar(&dOpts.jobID, "id", "", "numeric job id from a search result")

	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(detailCmd)

	return rootCmd
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
