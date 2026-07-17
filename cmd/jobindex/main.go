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
		keyword = searchFS.StringLong("keyword", "", "Jobindex q= free-text (required for useful results)")
		area    = searchFS.StringLong("area", "", "area path slug, e.g. storkoebenhavn")
		jobage  = searchFS.IntLong("jobage", 0, "Jobindex jobage= days (1, 7, 14, 30); 0 = all")
		sort    = searchFS.StringEnumLong("sort", "Jobindex sort=", "score", "date")
		page    = searchFS.IntLong("page", 1, "Jobindex page= (1-based)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "jobindex search --keyword TEXT [--area SLUG] [--jobage N] [--sort score|date] [--page N] [--format text|json]",
		ShortHelp: "search Jobindex.dk; JSON mirrors upstream Stash searchResponse",
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
	tid := detailFS.StringLong("tid", "", "Jobindex tid from search, e.g. h1683131")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "jobindex detail --tid TID [--format text|json]",
		ShortHelp: "scrape /vis-job/{tid}; JSON uses upstream-aligned field names",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --tid %q?)", args, args[0])
			}
			if *tid == "" {
				return errors.New("--tid is required")
			}
			return runDetail(ctx, detailFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				tid:     *tid,
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

	fmt.Printf("hitcount=%d page=%d total_pages=%d\n\n", resp.Hitcount, resp.Page, resp.TotalPages)
	for i, r := range resp.Results {
		tid, _ := r["tid"].(string)
		headline, _ := r["headline"].(string)
		area, _ := r["area"].(string)
		firstdate, _ := r["firstdate"].(string)
		jobURL, _ := r["url"].(string)
		companyName := ""
		if c, ok := r["company"].(map[string]any); ok {
			companyName, _ = c["name"].(string)
		}
		if companyName == "" {
			companyName, _ = r["companytext"].(string)
		}
		fmt.Printf("%d. [%s] %s\n", i+1, tid, headline)
		if companyName != "" {
			fmt.Printf("   company: %s\n", companyName)
		}
		if area != "" {
			fmt.Printf("   area: %s\n", area)
		}
		if firstdate != "" {
			fmt.Printf("   firstdate: %s\n", firstdate)
		}
		if jobURL != "" {
			fmt.Printf("   url: %s\n", jobURL)
		}
		fmt.Println()
	}
	return nil
}

type detailFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	tid     string
}

func runDetail(ctx context.Context, f detailFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := jobindex.NewClient(f.baseURL, nil)
	d, err := client.JobDetail(ctx, f.tid)
	if err != nil {
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Printf("[%s] %s\n", d.Tid, d.Headline)
	if d.Company != nil {
		if name, _ := d.Company["name"].(string); name != "" {
			fmt.Printf("company.name: %s\n", name)
		}
	}
	if d.Area != "" {
		fmt.Printf("area: %s\n", d.Area)
	}
	if d.Firstdate != "" {
		fmt.Printf("firstdate: %s\n", d.Firstdate)
	}
	if d.ApplyDeadline != "" {
		fmt.Printf("apply_deadline: %s\n", d.ApplyDeadline)
	}
	if d.URL != "" {
		fmt.Printf("url: %s\n", d.URL)
	}
	if d.Description != "" {
		fmt.Printf("\n%s\n", d.Description)
	}
	return nil
}
