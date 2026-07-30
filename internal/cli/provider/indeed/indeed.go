// Package indeed implements the "openings-mcp indeed" debug CLI, for manual
// checks against the live surface that internal/provider/indeed documents.
package indeed

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	indeedprovider "github.com/amikai/openings-mcp/internal/provider/indeed"
)

type options struct {
	apiURL      string
	timeout     time.Duration
	keywords    string
	location    string
	country     string
	radius      int
	limit       int
	cursor      string
	hoursOld    int
	jobType     string
	remote      bool
	easyApply   bool
	fetchDetail int
}

// NewCommand returns a cobra.Command for indeed.
func NewCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:          "indeed",
		Short:        "Search Indeed jobs and optionally fetch job details",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("indeed takes no positional arguments, got %v", args)
			}
			return run(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.apiURL, "api-url", "https://apis.indeed.com/graphql", "Indeed GraphQL API URL")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.Flags().StringVar(&opts.keywords, "keywords", "", "free-text search query")
	cmd.Flags().StringVar(&opts.location, "location", "", "free-text location, e.g. 'Taipei'")
	cmd.Flags().StringVar(&opts.country, "country", indeedprovider.DefaultCountryName, "country name selecting Indeed's indeed-co catalogue and site domain")
	cmd.Flags().IntVar(&opts.radius, "radius", 25, "search radius in miles around location")
	cmd.Flags().IntVar(&opts.limit, "limit", 25, "results per page, max 100")
	cmd.Flags().StringVar(&opts.cursor, "cursor", "", "pagination cursor from a previous page's NextCursor")
	cmd.Flags().IntVar(&opts.hoursOld, "hours-old", 0, "only jobs posted within this many hours")
	cmd.Flags().StringVar(&opts.jobType, "job-type", "", usageWithChoices("Job type", indeedprovider.JobTypeIDs))
	cmd.Flags().BoolVar(&opts.remote, "remote", false, "only remote jobs")
	cmd.Flags().BoolVar(&opts.easyApply, "easy-apply", false, "only Easy Apply jobs")
	cmd.Flags().IntVar(&opts.fetchDetail, "fetch-details", 0, "fetch full JobDetail for this many results (0 = none)")

	return cmd
}

func run(ctx context.Context, opts *options) error {
	radiusMiles := opts.radius
	req := &indeedprovider.JobsRequest{
		Keywords:    opts.keywords,
		Location:    opts.location,
		Country:     opts.country,
		RadiusMiles: &radiusMiles,
		Limit:       opts.limit,
		Cursor:      opts.cursor,
		HoursOld:    opts.hoursOld,
		Remote:      opts.remote,
		EasyApply:   opts.easyApply,
	}
	if opts.jobType != "" {
		req.JobType = indeedprovider.JobTypeIDs[opts.jobType]
	}

	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	client := indeedprovider.NewClient(opts.apiURL, nil)
	search, err := client.Jobs(ctx, req)
	if err != nil {
		return err
	}

	jobs := jobsForDetail(search.Jobs, opts.fetchDetail)
	details := make(map[string]*indeedprovider.JobDetail, len(jobs))
	for _, job := range jobs {
		detail, err := client.JobDetail(ctx, opts.country, job.Key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.Key, err)
			continue
		}
		details[job.Key] = detail
	}

	writeReport(os.Stdout, reportData{
		keywords: opts.keywords,
		search:   search,
		details:  details,
	})
	return nil
}

func jobsForDetail(jobs []indeedprovider.Job, n int) []indeedprovider.Job {
	if n <= 0 {
		return nil
	}
	n = min(n, len(jobs))
	return jobs[:n]
}

// reportData carries the data writeReport renders.
type reportData struct {
	keywords string
	search   *indeedprovider.JobsResponse
	details  map[string]*indeedprovider.JobDetail
}

func writeReport(w io.Writer, d reportData) {
	fmt.Fprintf(w, "Indeed Jobs Report\n")
	fmt.Fprintf(w, "Keywords: %s\n", d.keywords)
	fmt.Fprintf(w, "Found %d jobs\n", len(d.search.Jobs))
	if d.search.NextCursor != "" {
		fmt.Fprintf(w, "Next cursor: %s\n", d.search.NextCursor)
	}
	fmt.Fprintln(w)

	for i, job := range d.search.Jobs {
		fmt.Fprintf(w, "%d. [%s] %s\n", i+1, job.Key, job.Title)
		fmt.Fprintf(w, "URL: %s\n", job.JobURL)
		if job.Company != "" {
			fmt.Fprintf(w, "Company: %s\n", job.Company)
		}
		if job.Location != "" {
			fmt.Fprintf(w, "Location: %s\n", job.Location)
		}
		if job.PostedDate != "" {
			fmt.Fprintf(w, "Posted: %s\n", job.PostedDate)
		}
		if len(job.JobTypes) > 0 {
			fmt.Fprintf(w, "Job types: %v\n", job.JobTypes)
		}
		if job.Compensation != nil {
			fmt.Fprintf(w, "Compensation: %s\n", formatCompensation(job.Compensation))
		}
		if detail := d.details[job.Key]; detail != nil {
			writeDetail(w, detail)
		}
		fmt.Fprintln(w)
	}
}

// formatCompensation renders one-sided and exact salaries without a
// spurious zero bound (e.g. AtLeast → ">= 20", AtMost → "<= 30", Exactly
// a single number, Range as "min-max").
func formatCompensation(c *indeedprovider.Compensation) string {
	var amount string
	switch {
	case c.MinAmount != 0 && c.MaxAmount != 0 && c.MinAmount == c.MaxAmount:
		amount = fmt.Sprintf("%g", c.MinAmount)
	case c.MinAmount != 0 && c.MaxAmount != 0:
		amount = fmt.Sprintf("%g-%g", c.MinAmount, c.MaxAmount)
	case c.MinAmount != 0:
		amount = fmt.Sprintf(">= %g", c.MinAmount)
	case c.MaxAmount != 0:
		amount = fmt.Sprintf("<= %g", c.MaxAmount)
	default:
		amount = "undisclosed"
	}
	return fmt.Sprintf("%s %s (%s)", amount, c.Currency, c.Interval)
}

func writeDetail(w io.Writer, detail *indeedprovider.JobDetail) {
	if detail.Source != "" {
		fmt.Fprintf(w, "Source: %s\n", detail.Source)
	}
	if detail.DateIndexed != "" {
		fmt.Fprintf(w, "Date indexed: %s\n", detail.DateIndexed)
	}
	if detail.CompanyIndustry != "" {
		fmt.Fprintf(w, "Industry: %s\n", detail.CompanyIndustry)
	}
	if detail.CompanyEmployees != "" {
		fmt.Fprintf(w, "Company size: %s\n", detail.CompanyEmployees)
	}
	if detail.CompanyRevenue != "" {
		fmt.Fprintf(w, "Company revenue: %s\n", detail.CompanyRevenue)
	}
	if len(detail.CompanyAddresses) > 0 {
		fmt.Fprintf(w, "Company addresses: %v\n", detail.CompanyAddresses)
	}
	if detail.CompanyCEO != "" {
		fmt.Fprintf(w, "Company CEO: %s\n", detail.CompanyCEO)
	}
	if detail.DetailedSalary != "" {
		fmt.Fprintf(w, "Detailed salary: %s\n", detail.DetailedSalary)
	}
	if detail.WorkSchedule != "" {
		fmt.Fprintf(w, "Work schedule: %s\n", detail.WorkSchedule)
	}
	if detail.ApplyURL != "" {
		fmt.Fprintf(w, "Apply URL: %s\n", detail.ApplyURL)
	}
	if detail.Description != "" {
		fmt.Fprintf(w, "Description: %s\n", detail.Description)
	}
}

// labels returns the sorted keys of a lookup table, prefixed with "" so an
// ff.StringEnumLong flag can default to unset (no filter) instead of
// silently falling back to the first real value — ffval.Enum's zero Default
// only survives initialize() if it's itself in the Valid list.
func labels(table map[string]string) []string {
	return append([]string{""}, slices.Sorted(maps.Keys(table))...)
}

// usageWithChoices appends a "one of: ..." list to base. ffhelp never
// introspects an ff.StringEnumLong's valid values on its own, so small
// enough choice sets are spelled out here to make -h self-documenting.
func usageWithChoices(base string, table map[string]string) string {
	choices := labels(table)[1:]
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}
