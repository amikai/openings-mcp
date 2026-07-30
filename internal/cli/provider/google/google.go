package google

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	google "github.com/amikai/openings-mcp/internal/provider/google"
)

var (
	targetLevels    = []string{"EARLY", "MID", "ADVANCED", "INTERN_AND_APPRENTICE", "DIRECTOR_PLUS"}
	degrees         = []string{"PURSUING_DEGREE", "ASSOCIATE", "BACHELORS", "MASTERS", "PHD"}
	employmentTypes = []string{"FULL_TIME", "PART_TIME", "TEMPORARY", "INTERN"}
	companies       = []string{"DeepMind", "GFiber", "Google", "Verily Life Sciences", "Waymo", "Wing", "YouTube"}
	sortOrders      = []string{"relevance", "date"}
)

type searchFlags struct {
	baseURL        string
	timeout        time.Duration
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

// NewCommand returns a cobra.Command for google.
func NewCommand() *cobra.Command {
	var flags searchFlags

	cmd := &cobra.Command{
		Use:          "google",
		Short:        "Search Google Careers and fetch full posting details",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), flags)
		},
	}

	cmd.Flags().StringVar(&flags.baseURL, "base-url", google.DefaultBaseURL, "Google Careers site base URL")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.Flags().StringVar(&flags.query, "query", "", "free-text search query")
	cmd.Flags().StringVar(&flags.location, "location", "", "location filter (city, region, or country)")
	cmd.Flags().BoolVar(&flags.hasRemote, "has-remote", false, "only jobs marked Remote eligible")
	cmd.Flags().StringVar(&flags.targetLevel, "target-level", "", usageWithChoices("Experience level", targetLevels))
	cmd.Flags().StringVar(&flags.skills, "skills", "", "free-text skills and qualifications filter")
	cmd.Flags().StringVar(&flags.degree, "degree", "", usageWithChoices("Minimum education level", degrees))
	cmd.Flags().StringVar(&flags.employmentType, "employment-type", "", usageWithChoices("Job type", employmentTypes))
	cmd.Flags().StringVar(&flags.company, "company", "", usageWithChoices("Organization", companies))
	cmd.Flags().StringVar(&flags.sortBy, "sort-by", "", usageWithChoices("Sort order", sortOrders))
	cmd.Flags().IntVar(&flags.page, "page", 1, "1-based page number; 20 results per page")

	return cmd
}

func run(ctx context.Context, f searchFlags) error {
	req := buildJobsRequest(f)

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := google.NewClient(f.baseURL, nil)
	search, err := client.Jobs(ctx, req)
	if err != nil {
		return err
	}

	jobs := jobsForDetail(search.Jobs)
	details := make(map[string]*google.JobDetailResponse, len(jobs))
	for _, job := range jobs {
		detail, err := client.JobDetail(ctx, job.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "job detail %s: %v\n", job.ID, err)
			return err
		}
		details[job.ID] = detail
	}

	writeReport(os.Stdout, reportData{
		query:   f.query,
		baseURL: f.baseURL,
		search:  search,
		jobs:    jobs,
		details: details,
	})
	return nil
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

func usageWithChoices(base string, choices []string) string {
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}

func jobsForDetail(jobs []google.Job) []google.Job {
	if len(jobs) > 10 {
		return jobs[:10]
	}
	return jobs
}

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
