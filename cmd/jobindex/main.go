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

	"github.com/amikai/openings-mcp/internal/provider/jobindex"
)

const defaultBaseURL = "https://www.jobindex.dk"

func main() {
	rootFlags := ff.NewFlagSet("jobindex")
	var (
		baseURL = rootFlags.StringLong("base-url", defaultBaseURL, "Jobindex base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "jobindex",
		Usage: "jobindex [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword = searchFS.StringLong("keyword", "", "free-text keyword (required for useful results)")
		area    = searchFS.StringLong("area", "", "area path slug, e.g. storkoebenhavn")
		jobage  = searchFS.IntLong("jobage", 0, "max posting age in days (1, 7, 14, 30); 0 = all")
		sort    = searchFS.StringEnumLong("sort", "sort order", "score", "date")
		page    = searchFS.IntLong("page", 1, "1-based page number")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "jobindex search --keyword TEXT [--area SLUG] [--jobage N] [--sort score|date] [--page N] [--format text|json]",
		ShortHelp: "search Jobindex.dk listings",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			if *keyword == "" {
				return errors.New("--keyword is required")
			}
			if *page < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", *page)
			}
			return runSearch(ctx, searchFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				keyword: *keyword,
				area:    *area,
				jobage:  *jobage,
				sort:    *sort,
				page:    *page,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	id := detailFS.StringLong("id", "", "job tid from search, e.g. h1683131")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "jobindex detail --id TID [--format text|json]",
		ShortHelp: "fetch one Jobindex posting via /vis-job/{id}",
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

type searchFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	keyword string
	area    string
	jobage  int
	sort    string
	page    int
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := jobindex.NewClient(f.baseURL, nil)
	resp, err := client.Jobs(ctx, &jobindex.JobsRequest{
		Keyword:    f.keyword,
		Area:       f.area,
		Page:       f.page,
		JobAgeDays: f.jobage,
		Sort:       f.sort,
	})
	if err != nil {
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	fmt.Printf("Jobindex search: %q — %d total, page %d/%d\n\n", f.keyword, resp.TotalCount, resp.Page, resp.TotalPages)
	for i, j := range resp.Jobs {
		fmt.Printf("%d. [%s] %s\n", i+1, j.ID, j.Title)
		if j.Company != "" {
			fmt.Printf("   Company: %s\n", j.Company)
		}
		if j.Location != "" {
			fmt.Printf("   Location: %s\n", j.Location)
		}
		if j.PostedDate != "" {
			fmt.Printf("   Posted: %s\n", j.PostedDate)
		}
		if j.Deadline != "" {
			fmt.Printf("   Deadline: %s\n", j.Deadline)
		}
		fmt.Printf("   URL: %s\n\n", j.URL)
	}
	return nil
}

type detailFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	id      string
}

func runDetail(ctx context.Context, f detailFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := jobindex.NewClient(f.baseURL, nil)
	d, err := client.JobDetail(ctx, f.id)
	if err != nil {
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Printf("[%s] %s\n", d.ID, d.Title)
	if d.Company != "" {
		fmt.Printf("Company: %s\n", d.Company)
	}
	if d.Location != "" {
		fmt.Printf("Location: %s\n", d.Location)
	}
	if d.PostedDate != "" {
		fmt.Printf("Posted: %s\n", d.PostedDate)
	}
	if d.Deadline != "" {
		fmt.Printf("Deadline: %s\n", d.Deadline)
	}
	if d.EmploymentType != "" {
		fmt.Printf("Employment: %s\n", d.EmploymentType)
	}
	if d.Hours != "" {
		fmt.Printf("Hours: %s\n", d.Hours)
	}
	fmt.Printf("URL: %s\n", d.URL)
	if d.ApplyURL != "" {
		fmt.Printf("Apply: %s\n", d.ApplyURL)
	}
	if d.Description != "" {
		fmt.Printf("\n%s\n", d.Description)
	}
	return nil
}
