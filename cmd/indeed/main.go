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

	"github.com/amikai/openings-mcp/internal/provider/indeed"
)

// main issues a single JobsRequest built entirely from flags, then optionally
// fetches full JobDetail for the first -fetch-details results (default 0,
// i.e. none) via a second call per job.
func main() {
	fs := ff.NewFlagSet("indeed")
	var (
		apiURL      = fs.StringLong("api-url", "https://apis.indeed.com/graphql", "Indeed GraphQL API URL")
		timeout     = fs.DurationLong("timeout", 60*time.Second, "request timeout")
		keywords    = fs.StringLong("keywords", "", "free-text search query")
		location    = fs.StringLong("location", "", "free-text location, e.g. 'Taipei'")
		country     = fs.StringLong("country", indeed.DefaultCountryName, "country name selecting Indeed's indeed-co catalogue and site domain")
		radius      = fs.IntLong("radius", 25, "search radius in miles around location")
		limit       = fs.IntLong("limit", 25, "results per page, max 100")
		cursor      = fs.StringLong("cursor", "", "pagination cursor from a previous page's NextCursor")
		hoursOld    = fs.IntLong("hours-old", 0, "only jobs posted within this many hours")
		jobType     = fs.StringEnumLong("job-type", usageWithChoices("Job type", indeed.JobTypeIDs), labels(indeed.JobTypeIDs)...)
		remote      = fs.BoolLong("remote", "only remote jobs")
		easyApply   = fs.BoolLong("easy-apply", "only Easy Apply jobs")
		fetchDetail = fs.IntLong("fetch-details", 0, "fetch full JobDetail for this many results (0 = none)")
	)
	if err := ff.Parse(fs, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Flags(fs))
		if errors.Is(err, ff.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}

	radiusMiles := *radius
	req := &indeed.JobsRequest{
		Keywords:    *keywords,
		Location:    *location,
		Country:     *country,
		RadiusMiles: &radiusMiles,
		Limit:       *limit,
		Cursor:      *cursor,
		HoursOld:    *hoursOld,
		Remote:      *remote,
		EasyApply:   *easyApply,
	}
	if *jobType != "" {
		req.JobType = indeed.JobTypeIDs[*jobType]
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := indeed.NewClient(*apiURL, nil)
	search, err := client.Jobs(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	jobs := jobsForDetail(search.Jobs, *fetchDetail)
	details := make(map[string]*indeed.JobDetail, len(jobs))
	for _, job := range jobs {
		detail, err := client.JobDetail(ctx, *country, job.Key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.Key, err)
			continue
		}
		details[job.Key] = detail
	}

	writeReport(os.Stdout, reportData{
		keywords: *keywords,
		search:   search,
		details:  details,
	})
}

func jobsForDetail(jobs []indeed.Job, n int) []indeed.Job {
	if n <= 0 {
		return nil
	}
	n = min(n, len(jobs))
	return jobs[:n]
}

// reportData carries the data writeReport renders.
type reportData struct {
	keywords string
	search   *indeed.JobsResponse
	details  map[string]*indeed.JobDetail
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
			c := job.Compensation
			fmt.Fprintf(w, "Compensation: %g-%g %s (%s)\n", c.MinAmount, c.MaxAmount, c.Currency, c.Interval)
		}
		if detail := d.details[job.Key]; detail != nil {
			writeDetail(w, detail)
		}
		fmt.Fprintln(w)
	}
}

func writeDetail(w io.Writer, detail *indeed.JobDetail) {
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
	choices := labels(table)[1:] // drop the leading "" no-filter sentinel
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}
