// Package linkedin implements the "openings-mcp linkedin" debug CLI, for
// manual checks against the live surface that internal/provider/linkedin
// documents.
package linkedin

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

	linkedinprovider "github.com/amikai/openings-mcp/internal/provider/linkedin"
)

type options struct {
	baseURL       string
	timeout       time.Duration
	keywords      string
	location      string
	workplaceType string
	jobType       string
	companyIDs    string
	postedWithin  time.Duration
	start         int
	fetchDetails  int
}

// NewCommand returns a cobra.Command for linkedin.
func NewCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:          "linkedin",
		Short:        "Search LinkedIn jobs and optionally fetch job details",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("linkedin takes no positional arguments, got %v", args)
			}
			return run(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.baseURL, "base-url", linkedinprovider.DefaultBaseURL, "LinkedIn base URL")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.Flags().StringVar(&opts.keywords, "keywords", "", "free-text search query")
	cmd.Flags().StringVar(&opts.location, "location", "", "location filter (LinkedIn searches globally)")
	cmd.Flags().StringVar(&opts.workplaceType, "workplace-type", "", usageWithChoices("Workplace type", linkedinprovider.WorkplaceTypeIDs))
	cmd.Flags().StringVar(&opts.jobType, "job-type", "", usageWithChoices("Job type", linkedinprovider.JobTypeIDs))
	cmd.Flags().StringVar(&opts.companyIDs, "company-ids", "", "comma-separated LinkedIn numeric company IDs")
	cmd.Flags().DurationVar(&opts.postedWithin, "posted-within", 0, "only jobs posted within this duration, e.g. 24h")
	cmd.Flags().IntVar(&opts.start, "start", linkedinprovider.DefaultStart, "zero-based result offset for pagination")
	cmd.Flags().IntVar(&opts.fetchDetails, "fetch-details", 0, "fetch full JobDetail for this many results (0 = none)")

	return cmd
}

func run(ctx context.Context, opts *options) error {
	req := buildJobsRequest(searchFlags{
		keywords:      opts.keywords,
		location:      opts.location,
		workplaceType: opts.workplaceType,
		jobType:       opts.jobType,
		companyIDs:    opts.companyIDs,
		postedWithin:  opts.postedWithin,
		start:         opts.start,
	})

	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()

	client := linkedinprovider.NewClient(opts.baseURL, nil)
	search, err := client.Jobs(ctx, req)
	if err != nil {
		return err
	}

	jobs := jobsForDetail(search.Jobs, opts.fetchDetails)
	details := make(map[string]*linkedinprovider.JobDetailResponse, len(jobs))
	for _, job := range jobs {
		detail, err := client.JobDetail(ctx, job.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.ID, err)
			continue
		}
		details[job.ID] = detail
	}

	writeReport(os.Stdout, reportData{
		keywords: opts.keywords,
		baseURL:  opts.baseURL,
		search:   search,
		details:  details,
	})
	return nil
}

// searchFlags carries the parsed flag values into buildJobsRequest.
type searchFlags struct {
	keywords      string
	location      string
	workplaceType string
	jobType       string
	companyIDs    string
	postedWithin  time.Duration
	start         int
}

func buildJobsRequest(f searchFlags) *linkedinprovider.JobsRequest {
	req := &linkedinprovider.JobsRequest{
		Keywords: f.keywords,
		Location: f.location,
		Start:    f.start,
	}
	if f.workplaceType != "" {
		req.WorkplaceType = linkedinprovider.WorkplaceTypeIDs[f.workplaceType]
	}
	if f.jobType != "" {
		req.JobType = linkedinprovider.JobTypeIDs[f.jobType]
	}
	if f.companyIDs != "" {
		for id := range strings.SplitSeq(f.companyIDs, ",") {
			if id = strings.TrimSpace(id); id != "" {
				req.CompanyIDs = append(req.CompanyIDs, id)
			}
		}
	}
	if f.postedWithin > 0 {
		req.PostedWithinSeconds = int(f.postedWithin.Seconds())
	}
	return req
}

func jobsForDetail(jobs []linkedinprovider.Job, n int) []linkedinprovider.Job {
	if n <= 0 {
		return nil
	}
	n = min(n, len(jobs))
	return jobs[:n]
}

// reportData carries the data writeReport renders.
type reportData struct {
	keywords string
	baseURL  string
	search   *linkedinprovider.JobsResponse
	details  map[string]*linkedinprovider.JobDetailResponse
}

func writeReport(w io.Writer, d reportData) {
	fmt.Fprintf(w, "LinkedIn Jobs Report\n")
	fmt.Fprintf(w, "Keywords: %s\n", d.keywords)
	fmt.Fprintf(w, "Found %d jobs\n\n", len(d.search.Jobs))

	for i, job := range d.search.Jobs {
		fmt.Fprintf(w, "%d. [%s] %s\n", i+1, job.ID, job.Title)
		fmt.Fprintf(w, "URL: %s/jobs/view/%s\n", d.baseURL, job.ID)
		if job.Company != "" {
			fmt.Fprintf(w, "Company: %s\n", job.Company)
		}
		if job.Location != "" {
			fmt.Fprintf(w, "Location: %s\n", job.Location)
		}
		if job.PostedDate != "" {
			fmt.Fprintf(w, "Posted: %s\n", job.PostedDate)
		}
		if job.Remote {
			fmt.Fprintln(w, "Looks remote")
		}
		if detail := d.details[job.ID]; detail != nil {
			writeDetail(w, detail)
		}
		fmt.Fprintln(w)
	}
}

func writeDetail(w io.Writer, detail *linkedinprovider.JobDetailResponse) {
	if detail.SeniorityLevel != "" {
		fmt.Fprintf(w, "Seniority level: %s\n", detail.SeniorityLevel)
	}
	if detail.EmploymentType != "" {
		fmt.Fprintf(w, "Employment type: %s\n", detail.EmploymentType)
	}
	if detail.Industries != "" {
		fmt.Fprintf(w, "Industries: %s\n", detail.Industries)
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
