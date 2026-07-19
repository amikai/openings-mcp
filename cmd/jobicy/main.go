// Command jobicy is a debug CLI for the Jobicy remote-jobs feed.
//
//	go run ./cmd/jobicy search --tag golang --geo usa --industry dev --count 5
//	go run ./cmd/jobicy locations
//	go run ./cmd/jobicy industries
//
// The feed has no detail endpoint: every search row already carries the
// complete HTML description, which --format json includes as
// jobDescription. Text output shows the plain-text excerpt instead.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/jobicy"
)

const defaultBaseURL = "https://jobicy.com"

func main() {
	rootFlags := ff.NewFlagSet("jobicy")
	var (
		baseURL = rootFlags.StringLong("base-url", defaultBaseURL, "Jobicy base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "jobicy",
		Usage: "jobicy [FLAGS] <search|locations|industries> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		count    = searchFS.IntLong("count", 20, "number of listings to return (1-100)")
		geo      = searchFS.StringLong("geo", "", "region geoSlug from the locations subcommand, e.g. usa")
		industry = searchFS.StringLong("industry", "", "category industrySlug from the industries subcommand, e.g. dev")
		tag      = searchFS.StringLong("tag", "", "free-text search over job title and description")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "jobicy search [--count N] [--geo SLUG] [--industry SLUG] [--tag TEXT] [--format text|json]",
		ShortHelp: "search the remote-jobs feed; JSON mirrors the upstream envelope",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			if *count < 1 || *count > 100 {
				return fmt.Errorf("--count must be in 1..100, got %d", *count)
			}
			return runSearch(ctx, searchFlags{
				baseURL:  *baseURL,
				timeout:  *timeout,
				format:   *format,
				count:    *count,
				geo:      *geo,
				industry: *industry,
				tag:      *tag,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	for _, tax := range []struct {
		name string
		get  jobicy.GetRemoteJobsGet
	}{
		{"locations", jobicy.GetRemoteJobsGetLocations},
		{"industries", jobicy.GetRemoteJobsGetIndustries},
	} {
		taxCmd := &ff.Command{
			Name:      tax.name,
			Usage:     "jobicy " + tax.name + " [--format text|json]",
			ShortHelp: "list the valid --" + map[string]string{"locations": "geo", "industries": "industry"}[tax.name] + " slugs",
			Flags:     ff.NewFlagSet(tax.name).SetParent(rootFlags),
			Exec: func(ctx context.Context, args []string) error {
				if len(args) > 0 {
					return fmt.Errorf("%s takes no positional arguments, got %v", tax.name, args)
				}
				return runTaxonomy(ctx, *baseURL, *timeout, *format, tax.get)
			},
		}
		rootCmd.Subcommands = append(rootCmd.Subcommands, taxCmd)
	}

	if err := rootCmd.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd.GetSelected()))
		if errors.Is(err, ff.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
	if rootCmd.GetSelected() == rootCmd {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd))
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, locations, or industries) is required")
		os.Exit(1)
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
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

	client, err := jobicy.NewClient(f.baseURL)
	if err != nil {
		return err
	}

	params := jobicy.GetRemoteJobsParams{Count: jobicy.NewOptInt(f.count)}
	if f.geo != "" {
		params.Geo = jobicy.NewOptString(f.geo)
	}
	if f.industry != "" {
		params.Industry = jobicy.NewOptString(f.industry)
	}
	if f.tag != "" {
		params.Tag = jobicy.NewOptString(f.tag)
	}

	res, err := client.GetRemoteJobs(ctx, params)
	if err != nil {
		if apiErr, ok := errors.AsType[*jobicy.ErrorResponseStatusCode](err); ok {
			return fmt.Errorf("jobicy: %d: %s", apiErr.StatusCode, apiErr.Response.Error)
		}
		return err
	}

	var jobs *jobicy.JobsResponse
	switch r := res.(type) {
	case *jobicy.GetRemoteJobsOK:
		v, ok := r.GetJobsResponse()
		if !ok {
			return fmt.Errorf("unexpected response variant %s", r.Type)
		}
		jobs = &v
	case *jobicy.JobsResponse: // zero-match 404 envelope
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

func runTaxonomy(ctx context.Context, baseURL string, timeout time.Duration, format string, get jobicy.GetRemoteJobsGet) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := jobicy.NewClient(baseURL)
	if err != nil {
		return err
	}

	res, err := client.GetRemoteJobs(ctx, jobicy.GetRemoteJobsParams{Get: jobicy.NewOptGetRemoteJobsGet(get)})
	if err != nil {
		return err
	}
	sum, ok := res.(*jobicy.GetRemoteJobsOK)
	if !ok {
		return fmt.Errorf("unexpected response type %T", res)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	switch get {
	case jobicy.GetRemoteJobsGetLocations:
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
	case jobicy.GetRemoteJobsGetIndustries:
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
