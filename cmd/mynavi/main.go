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

	"github.com/amikai/openings-mcp/internal/provider/mynavi"
)

const defaultBaseURL = "https://tenshoku.mynavi.jp"

func main() {
	rootFlags := ff.NewFlagSet("mynavi")
	var (
		baseURL = rootFlags.StringLong("base-url", defaultBaseURL, "Mynavi Tenshoku base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "mynavi",
		Usage: "mynavi [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keywords  = searchFS.StringLong("keywords", "", "free text; space-separated terms AND together (Japanese OK)")
		minSalary = searchFS.IntLong("min-salary", 0, "first-year income (初年度年収) floor in units of 10,000 JPY, e.g. 700; only the site's fixed steps are valid; 0 = no filter")
		page      = searchFS.IntLong("page", 1, "1-based page of 50 results")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "mynavi search [--keywords TEXT] [--min-salary N] [--page N] [--format text|json]",
		ShortHelp: "search tenshoku.mynavi.jp job listings",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			if *page < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", *page)
			}
			return runSearch(ctx, searchFlags{
				baseURL:   *baseURL,
				timeout:   *timeout,
				format:    *format,
				keywords:  *keywords,
				minSalary: *minSalary,
				page:      *page,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFS.StringLong("job-id", "", "job ID from search, e.g. 348855-1-29-1")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "mynavi detail --job-id ID [--format text|json]",
		ShortHelp: "fetch one posting's full JobPosting data",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --job-id %q?)", args, args[0])
			}
			if *jobID == "" {
				return errors.New("--job-id is required")
			}
			return runDetail(ctx, detailFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				jobID:   *jobID,
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
	baseURL   string
	timeout   time.Duration
	format    string
	keywords  string
	minSalary int
	page      int
}

func runSearch(ctx context.Context, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := mynavi.NewClient(f.baseURL, nil)
	resp, err := client.Jobs(ctx, &mynavi.JobsRequest{
		Keywords:  f.keywords,
		MinSalary: f.minSalary,
		Page:      f.page,
	})
	if err != nil {
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	fmt.Printf("total=%d page=%d jobs=%d\n\n", resp.Total, f.page, len(resp.Jobs))
	for i, job := range resp.Jobs {
		fmt.Printf("%d. [%s] %s\n", i+1, job.ID, job.Title)
		fmt.Printf("   company: %s\n", job.Company)
		if job.EmploymentStatus != "" {
			fmt.Printf("   employment: %s\n", job.EmploymentStatus)
		}
		if job.Location != "" {
			fmt.Printf("   location: %s\n", job.Location)
		}
		if job.Salary != "" {
			fmt.Printf("   salary: %s\n", job.Salary)
		}
		if job.FirstYearIncome != "" {
			fmt.Printf("   first_year_income: %s\n", job.FirstYearIncome)
		}
		if job.EndDate != "" {
			fmt.Printf("   end_date: %s\n", job.EndDate)
		}
		fmt.Println()
	}
	return nil
}

type detailFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	jobID   string
}

func runDetail(ctx context.Context, f detailFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := mynavi.NewClient(f.baseURL, nil)
	d, err := client.JobDetail(ctx, f.jobID)
	if err != nil {
		return err
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Printf("[%s] %s\n", d.ID, d.Title)
	fmt.Printf("company: %s\n", d.Company)
	if d.CompanyURL != "" {
		fmt.Printf("company_url: %s\n", d.CompanyURL)
	}
	if d.EmploymentType != "" {
		fmt.Printf("employment_type: %s\n", d.EmploymentType)
	}
	if d.Industry != "" {
		fmt.Printf("industry: %s\n", d.Industry)
	}
	if d.OccupationalCategory != "" {
		fmt.Printf("occupation: %s\n", d.OccupationalCategory)
	}
	if d.DatePosted != "" {
		fmt.Printf("posted: %s\n", d.DatePosted)
	}
	if d.ValidThrough != "" {
		fmt.Printf("valid_through: %s\n", d.ValidThrough)
	}
	if n := len(d.Locations); n > 0 {
		if n > 5 {
			fmt.Printf("locations: %d prefectures\n", n)
		} else {
			for _, loc := range d.Locations {
				fmt.Printf("location: %s %s\n", loc.Region, loc.Locality)
			}
		}
	}
	if d.SalaryMin != "" || d.SalaryMax != "" {
		fmt.Printf("salary: %s-%s %s/%s\n", d.SalaryMin, d.SalaryMax, d.SalaryCurrency, d.SalaryUnit)
	}
	if d.URL != "" {
		fmt.Printf("url: %s\n", d.URL)
	}
	if d.Description != "" {
		fmt.Printf("\n%s\n", d.Description)
	}
	return nil
}
