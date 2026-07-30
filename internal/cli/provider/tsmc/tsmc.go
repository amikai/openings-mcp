package tsmc

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/tsmc"
)

type options struct {
	baseURL        string
	timeout        time.Duration
	keyword        string
	page           int
	perPage        int
	location       string
	category       string
	jobType        string
	employmentType string
}

// NewCommand returns a cobra.Command for tsmc.
func NewCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:          "tsmc",
		Short:        "Search TSMC jobs and view position details",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), *opts)
		},
	}

	cmd.Flags().StringVar(&opts.baseURL, "base-url", tsmc.DefaultBaseURL, "TSMC careers site base URL")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.Flags().StringVar(&opts.keyword, "keyword", "", "free-text keyword search")
	cmd.Flags().IntVar(&opts.page, "page", 1, "1-based page number")
	cmd.Flags().IntVar(&opts.perPage, "per-page", 10, "page size")
	cmd.Flags().StringVar(&opts.location, "location", "", usageWithChoices("Location", tsmc.LocationIDs))
	cmd.Flags().StringVar(&opts.category, "category", "", usageWithChoices("Job Category", tsmc.CategoryIDs))
	cmd.Flags().StringVar(&opts.jobType, "job-type", "", usageWithChoices("Job Type", tsmc.JobTypeIDs))
	cmd.Flags().StringVar(&opts.employmentType, "employment-type", "", usageWithChoices("Employment Type", tsmc.EmploymentTypeIDs))

	return cmd
}

func run(ctx context.Context, opts options) error {
	req := buildJobsRequest(searchFlags{
		keyword:        opts.keyword,
		location:       opts.location,
		category:       opts.category,
		jobType:        opts.jobType,
		employmentType: opts.employmentType,
		page:           opts.page,
		perPage:        opts.perPage,
	})

	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	client := tsmc.NewClient(opts.baseURL, nil)

	search, err := client.Jobs(ctx, req)
	if err != nil {
		return err
	}

	fmt.Printf("TSMC Jobs Report\n")
	fmt.Printf("Found %d jobs; showing %d\n\n", search.Total, len(search.Jobs))

	for i, job := range search.Jobs {
		fmt.Printf("%d. [%s] %s\n", i+1, job.ID, job.Title)
		if job.Slug != "" {
			fmt.Printf("URL: %s/zh_TW/careers/JobDetail/%s/%s\n", opts.baseURL, job.Slug, job.ID)
		}
		if job.Location != "" {
			fmt.Printf("Location: %s\n", job.Location)
		}
		if job.CareerArea != "" {
			fmt.Printf("Career Area: %s\n", job.CareerArea)
		}
		if job.EmploymentType != "" {
			fmt.Printf("Employment Type: %s\n", job.EmploymentType)
		}
		if job.Posted != "" {
			fmt.Printf("Posted: %s\n", job.Posted)
		}

		detail, err := client.JobDetail(ctx, job.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.ID, err)
			fmt.Println()
			continue
		}
		if detail.Responsibilities != "" {
			fmt.Printf("Responsibilities:\n%s\n", detail.Responsibilities)
		}
		if detail.Qualifications != "" {
			fmt.Printf("Qualifications:\n%s\n", detail.Qualifications)
		}
		fmt.Println()
	}
	return nil
}

// searchFlags carries the parsed flag values into buildJobsRequest.
type searchFlags struct {
	keyword        string
	location       string
	category       string
	jobType        string
	employmentType string
	page           int
	perPage        int
}

func buildJobsRequest(f searchFlags) *tsmc.JobsRequest {
	req := &tsmc.JobsRequest{
		Keyword: f.keyword,
		Page:    f.page,
		PerPage: f.perPage,
	}
	if f.location != "" {
		req.Locations = []string{tsmc.LocationIDs[f.location]}
	}
	if f.category != "" {
		req.Categories = []string{tsmc.CategoryIDs[f.category]}
	}
	if f.jobType != "" {
		req.JobTypes = []string{tsmc.JobTypeIDs[f.jobType]}
	}
	if f.employmentType != "" {
		req.EmploymentTypes = []string{tsmc.EmploymentTypeIDs[f.employmentType]}
	}
	return req
}

func labels[V any](table map[string]V) []string {
	return append([]string{""}, slices.Sorted(maps.Keys(table))...)
}

func usageWithChoices[V any](base string, table map[string]V) string {
	choices := labels(table)[1:]
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}
