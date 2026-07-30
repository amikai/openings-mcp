// Package mynavi implements the "openings-mcp mynavi" debug CLI, for manual
// checks against the live surface that internal/provider/mynavi documents.
package mynavi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/cli/clihelp"
	mynaviprovider "github.com/amikai/openings-mcp/internal/provider/mynavi"
)

type options struct {
	baseURL string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for mynavi.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "mynavi",
		Short:        "Search Mynavi Tenshoku jobs and view position details",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", mynaviprovider.DefaultBaseURL, "Mynavi Tenshoku base URL")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	clihelp.FormatVar(rootCmd.PersistentFlags(), &opts.format)

	var (
		searchKeywords  string
		searchMinSalary int
		searchPage      int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search tenshoku.mynavi.jp job listings",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if searchPage < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", searchPage)
			}
			return runSearch(cmd.Context(), searchFlags{
				baseURL:   opts.baseURL,
				timeout:   opts.timeout,
				format:    opts.format,
				keywords:  searchKeywords,
				minSalary: searchMinSalary,
				page:      searchPage,
			})
		},
	}

	searchCmd.Flags().StringVar(&searchKeywords, "keywords", "", "free text; space-separated terms AND together (Japanese OK)")
	searchCmd.Flags().IntVar(&searchMinSalary, "min-salary", 0, "first-year income (初年度年収) floor in units of 10,000 JPY, e.g. 700; only the site's fixed steps are valid; 0 = no filter")
	searchCmd.Flags().IntVar(&searchPage, "page", 1, "1-based page of 50 results")

	var detailJobID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "fetch one posting's full JobPosting data",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if detailJobID == "" {
				return errors.New("--job-id is required")
			}
			return runDetail(cmd.Context(), detailFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				jobID:   detailJobID,
			})
		},
	}

	detailCmd.Flags().StringVar(&detailJobID, "job-id", "", "job ID from search, e.g. 348855-1-29-1")

	rootCmd.AddCommand(searchCmd, detailCmd)
	return rootCmd
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

	client := mynaviprovider.NewClient(f.baseURL, nil)
	resp, err := client.Jobs(ctx, &mynaviprovider.JobsRequest{
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

	client := mynaviprovider.NewClient(f.baseURL, nil)
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
