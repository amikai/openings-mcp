package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/linkedin"
)

// main issues a single JobsRequest built entirely from flags. Job detail is
// fetched for at most -fetch-details jobs (default 0, i.e. none): LinkedIn's
// jobs/view/{id} endpoint is the most block-prone of the two, commonly
// 999-authwalling a cold request and rate-limiting a single IP after
// sustained use, so fetching every result's detail is opt-in, not default.
func main() {
	fs := ff.NewFlagSet("linkedin")
	var (
		baseURL       = fs.StringLong("base-url", "https://www.linkedin.com", "LinkedIn base URL")
		timeout       = fs.DurationLong("timeout", 60*time.Second, "request timeout")
		keywords      = fs.StringLong("keywords", "", "free-text search query")
		location      = fs.StringLong("location", "", "location filter (LinkedIn searches globally)")
		workplaceType = fs.StringEnumLong("workplace-type", usageWithChoices("Workplace type", linkedin.WorkplaceTypeIDs), labels(linkedin.WorkplaceTypeIDs)...)
		jobType       = fs.StringEnumLong("job-type", usageWithChoices("Job type", linkedin.JobTypeIDs), labels(linkedin.JobTypeIDs)...)
		companyIDs    = fs.StringLong("company-ids", "", "comma-separated LinkedIn numeric company IDs")
		postedWithin  = fs.DurationLong("posted-within", 0, "only jobs posted within this duration, e.g. 24h")
		start         = fs.IntLong("start", linkedin.DefaultStart, "zero-based result offset for pagination")
		fetchDetails  = fs.IntLong("fetch-details", 0, "fetch full JobDetail for this many results (0 = none; see main's doc comment on why this isn't on by default)")
	)
	if err := ff.Parse(fs, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Flags(fs))
		if errors.Is(err, ff.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}

	req := buildJobsRequest(searchFlags{
		keywords:      *keywords,
		location:      *location,
		workplaceType: *workplaceType,
		jobType:       *jobType,
		companyIDs:    *companyIDs,
		postedWithin:  *postedWithin,
		start:         *start,
	})

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := linkedin.NewClient(*baseURL, nil)
	search, err := client.Jobs(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	jobs := jobsForDetail(search.Jobs, *fetchDetails)
	details := make(map[string]*linkedin.JobDetailResponse, len(jobs))
	for _, job := range jobs {
		detail, err := client.JobDetail(ctx, job.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.ID, err)
			continue
		}
		details[job.ID] = detail
	}

	writeReport(os.Stdout, reportData{
		keywords: *keywords,
		baseURL:  *baseURL,
		search:   search,
		details:  details,
	})
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

func buildJobsRequest(f searchFlags) *linkedin.JobsRequest {
	req := &linkedin.JobsRequest{
		Keywords: f.keywords,
		Location: f.location,
		Start:    f.start,
	}
	if f.workplaceType != "" {
		req.WorkplaceType = linkedin.WorkplaceTypeIDs[f.workplaceType]
	}
	if f.jobType != "" {
		req.JobType = linkedin.JobTypeIDs[f.jobType]
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

func jobsForDetail(jobs []linkedin.Job, n int) []linkedin.Job {
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
	search   *linkedin.JobsResponse
	details  map[string]*linkedin.JobDetailResponse
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

func writeDetail(w io.Writer, detail *linkedin.JobDetailResponse) {
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
	choices := labels(table)[1:] // drop the leading "" no-filter sentinel
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}
