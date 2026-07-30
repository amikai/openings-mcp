package job104

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/job104"
)

type searchFlags struct {
	keyword    string
	area       string
	ro         string
	order      string
	edu        []string
	remoteWork string
	s9         []string
	jobexp     []string
	page       int
	timeout    time.Duration
}

// NewCommand returns a cobra.Command for 104.
func NewCommand() *cobra.Command {
	var flags searchFlags

	cmd := &cobra.Command{
		Use:          "104",
		Short:        "Search 104 Job Bank and fetch full posting details",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), flags)
		},
	}

	cmd.Flags().DurationVar(&flags.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.Flags().StringVar(&flags.keyword, "keyword", "", "free-text keyword search")
	cmd.Flags().StringVar(&flags.area, "area", "", usageWithChoices("Area", labels(job104.AreaIDs)))
	cmd.Flags().StringVar(&flags.ro, "ro", "", usageWithChoices("Job type", labels(job104.RoIDs)))
	cmd.Flags().StringVar(&flags.order, "order", "", usageWithChoices("Sort order", labels(job104.OrderIDs)))
	cmd.Flags().IntVar(&flags.page, "page", 0, "1-based page number (0 = unset, server default)")
	cmd.Flags().StringSliceVar(&flags.edu, "edu", nil, usageWithChoices("Education (repeatable)", labels(job104.EduIDs)))
	cmd.Flags().StringVar(&flags.remoteWork, "remote-work", "", usageWithChoices("Remote work", labels(job104.RemoteWorkIDs)))
	cmd.Flags().StringSliceVar(&flags.s9, "s9", nil, usageWithChoices("Shift type (repeatable)", labels(job104.S9IDs)))
	cmd.Flags().StringSliceVar(&flags.jobexp, "jobexp", nil, usageWithChoices("Years of experience (repeatable)", labels(job104.JobExpIDs)))

	return cmd
}

func run(ctx context.Context, f searchFlags) error {
	params, err := buildSearchParams(f)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := job104.NewClient(job104.DefaultBaseURL, job104.WithClient(&http.Client{Transport: job104.BrowserTransport{}}))
	if err != nil {
		return err
	}

	search, err := client.SearchJobs(ctx, params)
	if err != nil {
		return err
	}

	p := search.Metadata.Pagination
	fmt.Printf("104 Jobs Report\n")
	fmt.Printf("Found %d jobs (page %d/%d); showing %d\n\n", p.Total.Value, p.CurrentPage.Value, p.LastPage.Value, len(search.Data))

	for i, job := range search.Data {
		code := job104.JobCodeFromURL(job.Link.Job)
		fmt.Printf("%d. [%s] %s\n", i+1, code, job.JobName.Value)
		fmt.Printf("Company: %s\n", job.CustName.Value)
		if job.JobAddrNoDesc.Value != "" {
			fmt.Printf("Location: %s\n", job.JobAddrNoDesc.Value)
		}
		fmt.Printf("jobRo=%d remoteWorkType=%d\n", job.JobRo.Value, job.RemoteWorkType.Value)
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
	return nil
}

func buildSearchParams(f searchFlags) (job104.SearchJobsParams, error) {
	params := job104.SearchJobsParams{}
	if f.keyword != "" {
		params.Keyword = job104.NewOptString(f.keyword)
	}
	if f.area != "" {
		val, ok := job104.AreaIDs[f.area]
		if !ok {
			return params, fmt.Errorf("--area: unknown value %q", f.area)
		}
		params.Area = job104.NewOptSearchJobsArea(val)
	}
	if f.ro != "" {
		val, ok := job104.RoIDs[f.ro]
		if !ok {
			return params, fmt.Errorf("--ro: unknown value %q", f.ro)
		}
		params.Ro = job104.NewOptSearchJobsRo(val)
	}
	if f.order != "" {
		val, ok := job104.OrderIDs[f.order]
		if !ok {
			return params, fmt.Errorf("--order: unknown value %q", f.order)
		}
		params.Order = job104.NewOptSearchJobsOrder(val)
	}
	if f.page != 0 {
		params.Page = job104.NewOptInt(f.page)
	}
	if len(f.edu) > 0 {
		items, err := lookupList(job104.EduIDs, f.edu, "--edu")
		if err != nil {
			return params, err
		}
		params.Edu = items
	}
	if f.remoteWork != "" {
		val, ok := job104.RemoteWorkIDs[f.remoteWork]
		if !ok {
			return params, fmt.Errorf("--remote-work: unknown value %q", f.remoteWork)
		}
		params.RemoteWork = job104.NewOptSearchJobsRemoteWork(val)
	}
	if len(f.s9) > 0 {
		items, err := lookupList(job104.S9IDs, f.s9, "--s9")
		if err != nil {
			return params, err
		}
		params.S9 = items
	}
	if len(f.jobexp) > 0 {
		items, err := lookupList(job104.JobExpIDs, f.jobexp, "--jobexp")
		if err != nil {
			return params, err
		}
		params.Jobexp = items
	}
	return params, nil
}

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

func labels[T any](table map[string]T) []string {
	return slices.Sorted(maps.Keys(table))
}

func usageWithChoices(base string, choices []string) string {
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}
