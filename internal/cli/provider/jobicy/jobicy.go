package jobicy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	jobicyprovider "github.com/amikai/openings-mcp/internal/provider/jobicy"
)

const defaultBaseURL = "https://jobicy.com"

type options struct {
	baseURL string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for jobicy.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "jobicy",
		Short:        "jobicy [FLAGS] <search|locations|industries> [FLAGS]",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", defaultBaseURL, "Jobicy base URL")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	var (
		searchCount    int
		searchGeo      string
		searchIndustry string
		searchTag      string
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search the remote-jobs feed; JSON mirrors the upstream envelope",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if searchCount < 1 || searchCount > 100 {
				return fmt.Errorf("--count must be in 1..100, got %d", searchCount)
			}
			return runSearch(cmd.Context(), searchFlags{
				baseURL:  opts.baseURL,
				timeout:  opts.timeout,
				format:   opts.format,
				count:    searchCount,
				geo:      searchGeo,
				industry: searchIndustry,
				tag:      searchTag,
			})
		},
	}
	searchCmd.Flags().IntVar(&searchCount, "count", 20, "number of listings to return (1-100)")
	searchCmd.Flags().StringVar(&searchGeo, "geo", "", "region geoSlug from the locations subcommand, e.g. usa")
	searchCmd.Flags().StringVar(&searchIndustry, "industry", "", "category industrySlug from the industries subcommand, e.g. dev")
	searchCmd.Flags().StringVar(&searchTag, "tag", "", "free-text search over job title and description")

	rootCmd.AddCommand(searchCmd)

	for _, tax := range []struct {
		name string
		get  jobicyprovider.GetRemoteJobsGet
	}{
		{"locations", jobicyprovider.GetRemoteJobsGetLocations},
		{"industries", jobicyprovider.GetRemoteJobsGetIndustries},
	} {
		taxName := tax.name
		taxGet := tax.get
		taxCmd := &cobra.Command{
			Use:          taxName,
			Short:        "list the valid --" + map[string]string{"locations": "geo", "industries": "industry"}[taxName] + " slugs",
			SilenceUsage: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) > 0 {
					return fmt.Errorf("%s takes no positional arguments, got %v", taxName, args)
				}
				if opts.format != "text" && opts.format != "json" {
					return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
				}
				return runTaxonomy(cmd.Context(), opts.baseURL, opts.timeout, opts.format, taxGet)
			},
		}
		rootCmd.AddCommand(taxCmd)
	}

	return rootCmd
}

type searchFlags struct {
	baseURL  string
	timeout  time.Duration
	format   string
	count    int
	geo      string
	industry string
	tag      string
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := jobicyprovider.NewClient(f.baseURL)
	if err != nil {
		return err
	}

	params := jobicyprovider.GetRemoteJobsParams{Count: jobicyprovider.NewOptInt(f.count)}
	if f.geo != "" {
		params.Geo = jobicyprovider.NewOptString(f.geo)
	}
	if f.industry != "" {
		params.Industry = jobicyprovider.NewOptString(f.industry)
	}
	if f.tag != "" {
		params.Tag = jobicyprovider.NewOptString(f.tag)
	}

	res, err := client.GetRemoteJobs(ctx, params)
	if err != nil {
		if apiErr, ok := errors.AsType[*jobicyprovider.ErrorResponseStatusCode](err); ok {
			return fmt.Errorf("jobicy: %d: %s", apiErr.StatusCode, apiErr.Response.Error)
		}
		return err
	}

	var jobs *jobicyprovider.JobsResponse
	switch r := res.(type) {
	case *jobicyprovider.GetRemoteJobsOK:
		v, ok := r.GetJobsResponse()
		if !ok {
			return fmt.Errorf("unexpected response variant %s", r.Type)
		}
		jobs = &v
	case *jobicyprovider.JobsResponse: // zero-match 404 envelope
		jobs = r
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jobs)
	}

	fmt.Printf("jobCount=%d lastUpdate=%s\n", jobs.JobCount, jobs.LastUpdate)
	if msg, ok := jobs.Message.Get(); ok {
		fmt.Printf("message: %s\n", msg)
	}
	fmt.Println()
	for i, j := range jobs.Jobs {
		fmt.Printf("%d. [%d] %s\n", i+1, j.ID, j.JobTitle)
		fmt.Printf("   company: %s\n", j.CompanyName)
		fmt.Printf("   geo: %s level: %s type: %v industry: %v\n", j.JobGeo, j.JobLevel, j.JobType, j.JobIndustry)
		if min, ok := j.SalaryMin.Get(); ok {
			salary := fmt.Sprintf("%v", min)
			if max, ok := j.SalaryMax.Get(); ok {
				salary += fmt.Sprintf("-%v", max)
			}
			fmt.Printf("   salary: %s %s %s\n", salary, j.SalaryCurrency.Or(""), j.SalaryPeriod.Or(""))
		}
		fmt.Printf("   pubDate: %s\n", j.PubDate)
		fmt.Printf("   url: %s\n", j.URL)
		fmt.Printf("   %s\n", j.JobExcerpt)
		fmt.Println()
	}
	return nil
}

func runTaxonomy(ctx context.Context, baseURL string, timeout time.Duration, format string, get jobicyprovider.GetRemoteJobsGet) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := jobicyprovider.NewClient(baseURL)
	if err != nil {
		return err
	}

	res, err := client.GetRemoteJobs(ctx, jobicyprovider.GetRemoteJobsParams{Get: jobicyprovider.NewOptGetRemoteJobsGet(get)})
	if err != nil {
		return err
	}
	sum, ok := res.(*jobicyprovider.GetRemoteJobsOK)
	if !ok {
		return fmt.Errorf("unexpected response type %T", res)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	switch get {
	case jobicyprovider.GetRemoteJobsGetLocations:
		v, ok := sum.GetLocationsResponse()
		if !ok {
			return fmt.Errorf("unexpected response variant %s", sum.Type)
		}
		if format == "json" {
			return enc.Encode(v)
		}
		for _, l := range v.Locations {
			fmt.Printf("%-24s %s\n", l.GeoSlug, l.GeoName)
		}
	case jobicyprovider.GetRemoteJobsGetIndustries:
		v, ok := sum.GetIndustriesResponse()
		if !ok {
			return fmt.Errorf("unexpected response variant %s", sum.Type)
		}
		if format == "json" {
			return enc.Encode(v)
		}
		for _, ind := range v.Industries {
			fmt.Printf("%-24s %s\n", ind.IndustrySlug, ind.IndustryName)
		}
	}
	return nil
}
