// Package remoteok provides CLI command construction for Remote OK.
package remoteok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/remoteok"
)

const defaultBaseURL = "https://remoteok.com"

type rootOptions struct {
	baseURL string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for remoteok.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "remoteok",
		Short:        "remoteok [FLAGS] <search|detail> [FLAGS]",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", defaultBaseURL, "Remote OK base URL")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	var (
		searchTags []string
		keyword    string
		limit      int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "fetch the feed (~100 most recent jobs per tag set)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if limit < 1 {
				return fmt.Errorf("--limit must be >= 1, got %d", limit)
			}
			return runSearch(cmd.Context(), searchFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				tags:    searchTags,
				keyword: keyword,
				limit:   limit,
			})
		},
	}
	searchCmd.Flags().StringSliceVar(&searchTags, "tag", nil, "server-side tag filter; repeatable, tags are AND-ed")
	searchCmd.Flags().StringVar(&keyword, "keyword", "", "client-side substring filter on position, company, and tags")
	searchCmd.Flags().IntVar(&limit, "limit", 20, "max jobs to print")

	var (
		id         string
		detailTags []string
	)
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "re-fetch the feed and print one job in full",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if id == "" {
				return errors.New("--id is required")
			}
			return runDetail(cmd.Context(), detailFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				id:      id,
				tags:    detailTags,
			})
		},
	}
	detailCmd.Flags().StringVar(&id, "id", "", "job id from search, e.g. 1134996")
	detailCmd.Flags().StringSliceVar(&detailTags, "tag", nil, "tag filter used when the job was found; scopes the feed re-fetch")

	cmd.AddCommand(searchCmd, detailCmd)
	return cmd
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
