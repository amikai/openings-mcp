package jobindex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	jobindexprovider "github.com/amikai/openings-mcp/internal/provider/jobindex"
)

const defaultBaseURL = "https://www.jobindex.dk"

type options struct {
	baseURL string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for jobindex.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "jobindex",
		Short:        "jobindex [FLAGS] <search|detail> [FLAGS]",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", defaultBaseURL, "Jobindex base URL")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	var (
		searchKeyword string
		searchArea    string
		searchJobAge  int
		searchSort    string
		searchPage    int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search Jobindex.dk; JSON mirrors upstream Stash searchResponse",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if searchKeyword == "" {
				return errors.New("--keyword is required")
			}
			if searchPage < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", searchPage)
			}
			return runSearch(cmd.Context(), searchFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				keyword: searchKeyword,
				area:    searchArea,
				jobage:  searchJobAge,
				sort:    searchSort,
				page:    searchPage,
			})
		},
	}
	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "Jobindex q= free-text (required for useful results)")
	searchCmd.Flags().StringVar(&searchArea, "area", "", "area path slug, e.g. storkoebenhavn")
	searchCmd.Flags().IntVar(&searchJobAge, "jobage", 0, "Jobindex jobage= days (1, 7, 14, 30); 0 = all")
	searchCmd.Flags().StringVar(&searchSort, "sort", "score", "Jobindex sort= (score|date)")
	searchCmd.Flags().IntVar(&searchPage, "page", 1, "Jobindex page= (1-based)")

	var detailTID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "scrape /vis-job/{tid}; JSON uses upstream-aligned field names",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --tid %q?)", args, args[0])
			}
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if detailTID == "" {
				return errors.New("--tid is required")
			}
			return runDetail(cmd.Context(), detailFlags{
				baseURL: opts.baseURL,
				timeout: opts.timeout,
				format:  opts.format,
				tid:     detailTID,
			})
		},
	}
	detailCmd.Flags().StringVar(&detailTID, "tid", "", "Jobindex tid from search, e.g. h1683131")

	rootCmd.AddCommand(searchCmd, detailCmd)
	return rootCmd
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

	client := jobindexprovider.NewClient(f.baseURL, nil)
	resp, err := client.Jobs(ctx, &jobindexprovider.JobsRequest{
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
		postedAt, _ := r["posted_at"].(string)
		expiredAt, _ := r["expired_at"].(string)
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
		if postedAt != "" {
			fmt.Printf("   posted_at: %s\n", postedAt)
		}
		if expiredAt != "" {
			fmt.Printf("   expired_at: %s\n", expiredAt)
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

	client := jobindexprovider.NewClient(f.baseURL, nil)
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
	if d.PostedAt != "" {
		fmt.Printf("posted_at: %s\n", d.PostedAt)
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
