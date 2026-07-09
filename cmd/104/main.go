package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/job104"
)

// main searches with the flags and fetches each returned job's detail.
func main() {
	fs := ff.NewFlagSet("104")
	var (
		timeout    = fs.DurationLong("timeout", 60*time.Second, "request timeout")
		keyword    = fs.StringLong("keyword", "", "free-text keyword search")
		area       = fs.StringEnumLong("area", usageWithChoices("Area", labels(job104.AreaIDs)), enumChoices(job104.AreaIDs)...)
		ro         = fs.StringEnumLong("ro", usageWithChoices("Job type", labels(job104.RoIDs)), enumChoices(job104.RoIDs)...)
		order      = fs.StringEnumLong("order", usageWithChoices("Sort order", labels(job104.OrderIDs)), enumChoices(job104.OrderIDs)...)
		page       = fs.IntLong("page", 0, "1-based page number (0 = unset, server default)")
		edu        = fs.StringSetLong("edu", usageWithChoices("Education (repeatable)", labels(job104.EduIDs)))
		remoteWork = fs.StringEnumLong("remote-work", usageWithChoices("Remote work", labels(job104.RemoteWorkIDs)), enumChoices(job104.RemoteWorkIDs)...)
		s9         = fs.StringSetLong("s9", usageWithChoices("Shift type (repeatable)", labels(job104.S9IDs)))
	)
	if err := ff.Parse(fs, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Flags(fs))
		if errors.Is(err, ff.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}

	params, err := buildSearchParams(*keyword, *area, *ro, *order, *edu, *remoteWork, *s9, *page)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client, err := job104.NewClient("https://www.104.com.tw", job104.WithClient(&http.Client{Transport: job104.BrowserTransport{}}))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	search, err := client.SearchJobs(ctx, params)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	p := search.Metadata.Pagination
	fmt.Printf("104 Jobs Report\n")
	fmt.Printf("Found %d jobs (page %d/%d); showing %d\n\n", p.Total, p.CurrentPage, p.LastPage, len(search.Data))

	for i, job := range search.Data {
		code := job104.JobCodeFromURL(job.Link.Job)
		fmt.Printf("%d. [%s] %s\n", i+1, code, job.JobName)
		fmt.Printf("Company: %s\n", job.CustName)
		if job.JobAddrNoDesc != "" {
			fmt.Printf("Location: %s\n", job.JobAddrNoDesc)
		}
		// These server-side filters are soft: individual results may not match
		// them, so expose the raw values instead of assuming they did.
		fmt.Printf("jobRo=%d remoteWorkType=%d\n", job.JobRo, job.RemoteWorkType)
		if code == "" {
			fmt.Println()
			continue
		}
		detail, err := client.GetJobDetail(ctx, job104.GetJobDetailParams{JobCode: code})
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", code, err)
			fmt.Println()
			continue
		}
		writeDetail(os.Stdout, detail)
		fmt.Println()
	}
}

// buildSearchParams resolves flag labels through the lookup tables. Empty
// labels and page 0 leave their request fields unset.
func buildSearchParams(keyword, area, ro, order string, edu []string, remoteWork string, s9 []string, page int) (job104.SearchJobsParams, error) {
	params := job104.SearchJobsParams{}
	if keyword != "" {
		params.Keyword = job104.NewOptString(keyword)
	}
	if area != "" {
		params.Area = job104.NewOptSearchJobsArea(job104.AreaIDs[area])
	}
	if ro != "" {
		params.Ro = job104.NewOptSearchJobsRo(job104.RoIDs[ro])
	}
	if order != "" {
		params.Order = job104.NewOptSearchJobsOrder(job104.OrderIDs[order])
	}
	if page != 0 {
		params.Page = job104.NewOptInt(page)
	}
	if len(edu) > 0 {
		items, err := lookupList(job104.EduIDs, edu, "--edu")
		if err != nil {
			return params, err
		}
		params.Edu = items
	}
	if remoteWork != "" {
		params.RemoteWork = job104.NewOptSearchJobsRemoteWork(job104.RemoteWorkIDs[remoteWork])
	}
	if len(s9) > 0 {
		items, err := lookupList(job104.S9IDs, s9, "--s9")
		if err != nil {
			return params, err
		}
		params.S9 = items
	}
	return params, nil
}

// lookupList resolves the repeatable flag values in table.
func lookupList[T any](table map[string]T, values []string, flag string) ([]T, error) {
	out := make([]T, 0, len(values))
	for _, v := range values {
		item, ok := table[v]
		if !ok {
			return nil, fmt.Errorf("%s: unknown value %q (see job104 label tables)", flag, v)
		}
		out = append(out, item)
	}
	return out, nil
}

func writeDetail(w io.Writer, detail *job104.JobDetailResponse) {
	d := detail.Data
	jd := d.JobDetail
	if jd.Salary.Set {
		fmt.Fprintf(w, "Salary: %s\n", jd.Salary.Value)
	}
	if d.Condition.WorkExp.Set || d.Condition.Edu.Set {
		fmt.Fprintf(w, "Experience: %s | Education: %s\n", d.Condition.WorkExp.Value, d.Condition.Edu.Value)
	}
	if jd.JobDescription.Set && jd.JobDescription.Value != "" {
		fmt.Fprintf(w, "Description:\n%s\n", strings.TrimSpace(jd.JobDescription.Value))
	}
}

// labels returns the sorted keys in table.
func labels[T any](table map[string]T) []string {
	l := make([]string, 0, len(table))
	for label := range table {
		l = append(l, label)
	}
	sort.Strings(l)
	return l
}

// Include an empty sentinel so ff leaves the enum unset instead of choosing
// the first real value.
func enumChoices[T any](table map[string]T) []string {
	return append([]string{""}, labels(table)...)
}

// usageWithChoices adds the valid values to flag help.
func usageWithChoices(base string, choices []string) string {
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}
