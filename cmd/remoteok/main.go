package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/remoteok"
)

const defaultBaseURL = "https://remoteok.com"

func main() {
	rootFlags := ff.NewFlagSet("remoteok")
	var (
		baseURL = rootFlags.StringLong("base-url", defaultBaseURL, "Remote OK base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "remoteok",
		Usage: "remoteok [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		searchTags = searchFS.StringListLong("tag", "server-side tag filter; repeatable, tags are AND-ed")
		keyword    = searchFS.StringLong("keyword", "", "client-side substring filter on position, company, and tags")
		limit      = searchFS.IntLong("limit", 20, "max jobs to print")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "remoteok search [--tag TAG]... [--keyword TEXT] [--limit N] [--format text|json]",
		ShortHelp: "fetch the feed (~100 most recent jobs per tag set)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			if *limit < 1 {
				return fmt.Errorf("--limit must be >= 1, got %d", *limit)
			}
			return runSearch(ctx, searchFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				tags:    *searchTags,
				keyword: *keyword,
				limit:   *limit,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	var (
		id         = detailFS.StringLong("id", "", "job id from search, e.g. 1134996")
		detailTags = detailFS.StringListLong("tag", "tag filter used when the job was found; scopes the feed re-fetch")
	)
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "remoteok detail --id ID [--tag TAG]... [--format text|json]",
		ShortHelp: "re-fetch the feed and print one job in full",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			if *id == "" {
				return errors.New("--id is required")
			}
			return runDetail(ctx, detailFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				id:      *id,
				tags:    *detailTags,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search or detail) is required")
		os.Exit(1)
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

func fetchJobs(ctx context.Context, baseURL string, tags []string) ([]remoteok.Job, error) {
	client, err := remoteok.NewClient(baseURL)
	if err != nil {
		return nil, err
	}
	feed, err := client.GetJobs(ctx, remoteok.GetJobsParams{Tags: tags})
	if err != nil {
		return nil, err
	}
	jobs := make([]remoteok.Job, 0, len(feed))
	for _, el := range feed {
		if job, ok := el.GetJob(); ok {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

type searchFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	tags    []string
	keyword string
	limit   int
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	jobs, err := fetchJobs(ctx, f.baseURL, f.tags)
	if err != nil {
		return err
	}
	total := len(jobs)
	if f.keyword != "" {
		kw := strings.ToLower(f.keyword)
		jobs = slices.DeleteFunc(jobs, func(j remoteok.Job) bool {
			return !strings.Contains(strings.ToLower(j.Position.Value), kw) &&
				!strings.Contains(strings.ToLower(j.Company.Value), kw) &&
				!slices.ContainsFunc(j.Tags, func(t string) bool {
					return strings.Contains(strings.ToLower(t), kw)
				})
		})
	}
	matched := len(jobs)
	jobs = jobs[:min(f.limit, len(jobs))]

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jobs)
	}

	fmt.Printf("feed=%d matched=%d shown=%d\n\n", total, matched, len(jobs))
	for i, j := range jobs {
		fmt.Printf("%d. [%s] %s\n", i+1, j.ID, j.Position.Value)
		fmt.Printf("   company: %s\n", j.Company.Value)
		if j.Location.Value != "" {
			fmt.Printf("   location: %s\n", j.Location.Value)
		}
		if len(j.Tags) > 0 {
			fmt.Printf("   tags: %s\n", strings.Join(j.Tags, ", "))
		}
		if j.Date.Value != "" {
			fmt.Printf("   date: %s\n", j.Date.Value)
		}
		if j.SalaryMin.Value != 0 || j.SalaryMax.Value != 0 {
			fmt.Printf("   salary: %d-%d\n", j.SalaryMin.Value, j.SalaryMax.Value)
		}
		fmt.Printf("   url: %s\n", j.URL.Value)
		fmt.Println()
	}
	return nil
}

type detailFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	id      string
	tags    []string
}

func runDetail(ctx context.Context, f detailFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	jobs, err := fetchJobs(ctx, f.baseURL, f.tags)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(jobs, func(j remoteok.Job) bool { return j.ID == f.id })
	if i < 0 {
		return fmt.Errorf("job %s not in the current feed window of %d jobs; pass the --tag filter it was found with", f.id, len(jobs))
	}
	j := jobs[i]

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(j)
	}

	fmt.Printf("[%s] %s\n", j.ID, j.Position.Value)
	fmt.Printf("company: %s\n", j.Company.Value)
	if j.Location.Value != "" {
		fmt.Printf("location: %s\n", j.Location.Value)
	}
	if len(j.Tags) > 0 {
		fmt.Printf("tags: %s\n", strings.Join(j.Tags, ", "))
	}
	if j.Date.Value != "" {
		fmt.Printf("date: %s\n", j.Date.Value)
	}
	if j.SalaryMin.Value != 0 || j.SalaryMax.Value != 0 {
		fmt.Printf("salary: %d-%d\n", j.SalaryMin.Value, j.SalaryMax.Value)
	}
	if v, ok := j.Original.Get(); ok {
		fmt.Printf("original: %t\n", v)
	}
	fmt.Printf("url: %s\n", j.URL.Value)
	if j.Description.Value != "" {
		fmt.Printf("\n%s\n", j.Description.Value)
	}
	return nil
}
