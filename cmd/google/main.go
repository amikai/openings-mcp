package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	google "github.com/amikai/openings-mcp/internal/provider/google"
)

// Enum values mirror openapi.yaml's searchJobs parameters. The site silently
// ignores unrecognized values, so the flags reject them up front.
var (
	targetLevels    = []string{"EARLY", "MID", "ADVANCED", "INTERN_AND_APPRENTICE", "DIRECTOR_PLUS"}
	degrees         = []string{"PURSUING_DEGREE", "ASSOCIATE", "BACHELORS", "MASTERS", "PHD"}
	employmentTypes = []string{"FULL_TIME", "PART_TIME", "TEMPORARY", "INTERN"}
	companies       = []string{"DeepMind", "GFiber", "Google", "Verily Life Sciences", "Waymo", "Wing", "YouTube"}
	sortOrders      = []string{"relevance", "date"}
)

// main issues a single JobsRequest built entirely from flags, then fetches
// JobDetail for the first ten jobs the search returned.
func main() {
	fs := ff.NewFlagSet("google")
	var (
		baseURL        = fs.StringLong("base-url", "https://www.google.com/about/careers/applications", "Google Careers site base URL")
		timeout        = fs.DurationLong("timeout", 60*time.Second, "request timeout")
		query          = fs.StringLong("query", "", "free-text search query")
		location       = fs.StringLong("location", "", "location filter (city, region, or country)")
		hasRemote      = fs.BoolLong("has-remote", "only jobs marked Remote eligible")
		targetLevel    = fs.StringEnumLong("target-level", usageWithChoices("Experience level", targetLevels), withUnset(targetLevels)...)
		skills         = fs.StringLong("skills", "", "free-text skills and qualifications filter")
		degree         = fs.StringEnumLong("degree", usageWithChoices("Minimum education level", degrees), withUnset(degrees)...)
		employmentType = fs.StringEnumLong("employment-type", usageWithChoices("Job type", employmentTypes), withUnset(employmentTypes)...)
		company        = fs.StringEnumLong("company", usageWithChoices("Organization", companies), withUnset(companies)...)
		sortBy         = fs.StringEnumLong("sort-by", usageWithChoices("Sort order", sortOrders), withUnset(sortOrders)...)
		page           = fs.IntLong("page", 1, "1-based page number; 20 results per page")
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
		query:          *query,
		location:       *location,
		hasRemote:      *hasRemote,
		targetLevel:    *targetLevel,
		skills:         *skills,
		degree:         *degree,
		employmentType: *employmentType,
		company:        *company,
		sortBy:         *sortBy,
		page:           *page,
	})

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := google.NewClient(*baseURL, nil)
	search, err := client.Jobs(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	jobs := jobsForDetail(search.Jobs)
	details := make(map[string]*google.JobDetailResponse, len(jobs))
	for _, job := range jobs {
		detail, err := client.JobDetail(ctx, job.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.ID, err)
			os.Exit(1)
		}
		details[job.ID] = detail
	}

	writeReport(os.Stdout, reportData{
		query:   *query,
		baseURL: *baseURL,
		search:  search,
		jobs:    jobs,
		details: details,
	})
}

// searchFlags carries the parsed flag values into buildJobsRequest.
type searchFlags struct {
	query          string
	location       string
	hasRemote      bool
	targetLevel    string
	skills         string
	degree         string
	employmentType string
	company        string
	sortBy         string
	page           int
}

func buildJobsRequest(f searchFlags) *google.JobsRequest {
	req := &google.JobsRequest{
		Query:     f.query,
		HasRemote: f.hasRemote,
		Skills:    f.skills,
		SortBy:    f.sortBy,
		Page:      f.page,
	}
	if f.location != "" {
		req.Locations = []string{f.location}
	}
	if f.targetLevel != "" {
		req.TargetLevels = []string{f.targetLevel}
	}
	if f.degree != "" {
		req.Degrees = []string{f.degree}
	}
	if f.employmentType != "" {
		req.EmploymentType = []string{f.employmentType}
	}
	if f.company != "" {
		req.Companies = []string{f.company}
	}
	return req
}

// withUnset prefixes choices with "" so an ff.StringEnumLong flag can default
// to unset (no filter) instead of silently falling back to the first real
// value — ffval.Enum's zero Default only survives initialize() if it's itself
// in the Valid list.
func withUnset(choices []string) []string {
	return append([]string{""}, choices...)
}

// usageWithChoices appends a "one of: ..." list to base. ffhelp never
// introspects an ff.StringEnumLong's valid values on its own, so small
// enough choice sets are spelled out here to make -h self-documenting.
func usageWithChoices(base string, choices []string) string {
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}

func jobsForDetail(jobs []google.Job) []google.Job {
	if len(jobs) > 10 {
		return jobs[:10]
	}
	return jobs
}

// reportData carries the data writeReport renders.
type reportData struct {
	query   string
	baseURL string
	search  *google.JobsResponse
	jobs    []google.Job
	details map[string]*google.JobDetailResponse
}

func writeReport(w io.Writer, d reportData) {
	fmt.Fprintf(w, "Google Jobs Report\n")
	fmt.Fprintf(w, "Query: %s\n", d.query)
	fmt.Fprintf(w, "Found %d jobs; showing %d\n\n", len(d.search.Jobs), len(d.jobs))

	for i, job := range d.jobs {
		fmt.Fprintf(w, "%d. [%s] %s\n", i+1, job.ID, job.Title)
		fmt.Fprintf(w, "URL: %s/jobs/results/%s\n", d.baseURL, job.ID)
		if job.Company != "" {
			fmt.Fprintf(w, "Company: %s\n", job.Company)
		}
		if job.Location != "" {
			fmt.Fprintf(w, "Location: %s\n", job.Location)
		}
		if job.Remote {
			fmt.Fprintln(w, "Remote eligible")
		}
		if detail := d.details[job.ID]; detail != nil {
			writeDetail(w, detail)
		}
		fmt.Fprintln(w)
	}
}

func writeDetail(w io.Writer, detail *google.JobDetailResponse) {
	if detail.About != "" {
		fmt.Fprintf(w, "About: %s\n", detail.About)
	}
	if detail.Qualifications != "" {
		fmt.Fprintf(w, "Qualifications: %s\n", detail.Qualifications)
	}
	if detail.Responsibilities != "" {
		fmt.Fprintf(w, "Responsibilities: %s\n", detail.Responsibilities)
	}
}
