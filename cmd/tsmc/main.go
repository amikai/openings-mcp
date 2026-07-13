package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/tsmc"
)

// main issues a single JobsRequest built entirely from flags, then fetches
// JobDetail for every job the search returned.
func main() {
	fs := ff.NewFlagSet("tsmc")
	var (
		baseURL        = fs.StringLong("base-url", "https://careers.tsmc.com", "TSMC careers site base URL")
		timeout        = fs.DurationLong("timeout", 60*time.Second, "request timeout")
		keyword        = fs.StringLong("keyword", "", "free-text keyword search")
		page           = fs.IntLong("page", 1, "1-based page number")
		perPage        = fs.IntLong("per-page", 10, "page size")
		location       = fs.StringEnumLong("location", usageWithChoices("Location", tsmc.LocationIDs), labels(tsmc.LocationIDs)...)
		category       = fs.StringEnumLong("category", usageWithChoices("Job Category", tsmc.CategoryIDs), labels(tsmc.CategoryIDs)...)
		jobType        = fs.StringEnumLong("job-type", usageWithChoices("Job Type", tsmc.JobTypeIDs), labels(tsmc.JobTypeIDs)...)
		employmentType = fs.StringEnumLong("employment-type", usageWithChoices("Employment Type", tsmc.EmploymentTypeIDs), labels(tsmc.EmploymentTypeIDs)...)
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
		keyword:        *keyword,
		location:       *location,
		category:       *category,
		jobType:        *jobType,
		employmentType: *employmentType,
		page:           *page,
		perPage:        *perPage,
	})

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := tsmc.NewClient(*baseURL, nil)

	search, err := client.Jobs(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("TSMC Jobs Report\n")
	fmt.Printf("Found %d jobs; showing %d\n\n", search.Total, len(search.Jobs))

	for i, job := range search.Jobs {
		fmt.Printf("%d. [%s] %s\n", i+1, job.ID, job.Title)
		if job.Slug != "" {
			fmt.Printf("URL: %s/zh_TW/careers/JobDetail/%s/%s\n", *baseURL, job.Slug, job.ID)
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

// buildJobsRequest resolves each flag's human label to a form-field id via
// the ids.go lookup tables. Labels are already validated against the flag's
// enum at parse time, so a lookup miss here can't happen for a non-empty
// label. An empty label (flag not set) leaves that filter unset.
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

// labels returns the sorted keys of an ids.go lookup table, prefixed with
// "" so an ff.StringEnumLong flag can default to unset (no filter) instead
// of silently falling back to the first real label — ffval.Enum's zero
// Default only survives initialize() if it's itself in the Valid list.
func labels[V any](table map[string]V) []string {
	return append([]string{""}, slices.Sorted(maps.Keys(table))...)
}

// usageWithChoices appends a "one of: ..." list to base. ffhelp never
// introspects an ff.StringEnumLong's valid values on its own, so small
// enough choice sets are spelled out here to make -h self-documenting.
func usageWithChoices[V any](base string, table map[string]V) string {
	choices := labels(table)[1:] // drop the leading "" no-filter sentinel
	// " | " (not ", ") because some labels (e.g. "USA-Washington, D.C.")
	// contain commas themselves.
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}
