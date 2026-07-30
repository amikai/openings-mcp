package mtk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	mtkprovider "github.com/amikai/openings-mcp/internal/provider/mtk"
)

const defaultBaseURL = "https://careers.mediatek.com"

type options struct {
	baseURL string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for mtk.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "mtk",
		Short:        "Search MediaTek Careers jobs and view position details",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", defaultBaseURL, "MediaTek Careers base URL")
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	var (
		searchKeyword     string
		searchCategories  []string
		searchExperiences []string
		searchLocations   []string
		searchPrograms    []string
		searchPage        int
		searchLimit       int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search MediaTek Careers jobs",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if searchPage < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", searchPage)
			}
			if searchLimit < 1 || searchLimit > 100 {
				return fmt.Errorf("--limit must be between 1 and 100, got %d", searchLimit)
			}
			request, err := searchRequest(searchKeyword, searchCategories, searchExperiences, searchLocations, searchPrograms, searchPage, searchLimit)
			if err != nil {
				return err
			}
			return runSearch(cmd.Context(), opts.baseURL, opts.timeout, opts.format, request)
		},
	}

	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "free-text keyword query; AND/OR joins are supported")
	searchCmd.Flags().StringSliceVar(&searchCategories, "category", nil, "category label (repeatable)")
	searchCmd.Flags().StringSliceVar(&searchExperiences, "experience", nil, "work-experience label (repeatable)")
	searchCmd.Flags().StringSliceVar(&searchLocations, "location", nil, "location label (repeatable)")
	searchCmd.Flags().StringSliceVar(&searchPrograms, "program", nil, "program label (repeatable)")
	searchCmd.Flags().IntVar(&searchPage, "page", 1, "1-based page number")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 6, "page size (1-100)")

	var detailJobID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "fetch one MediaTek job detail page",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			if detailJobID == "" {
				return errors.New("--job-id is required")
			}
			return runDetail(cmd.Context(), opts.baseURL, opts.timeout, opts.format, detailJobID)
		},
	}

	detailCmd.Flags().StringVar(&detailJobID, "job-id", "", "MediaTek job ID from search, e.g. MTK120260629001")

	rootCmd.AddCommand(searchCmd, detailCmd)
	return rootCmd
}

func searchRequest(keyword string, categories, experiences, locations, programs []string, page, limit int) (mtkprovider.SearchRequest, error) {
	var err error
	request := mtkprovider.SearchRequest{Keyword: keyword, Page: page, Limit: limit}
	if request.Categories, err = resolveLabels(categories, mtkprovider.CategoryOptions, "category"); err != nil {
		return mtkprovider.SearchRequest{}, err
	}
	if request.WorkExperiences, err = resolveLabels(experiences, mtkprovider.WorkExperienceOptions, "experience"); err != nil {
		return mtkprovider.SearchRequest{}, err
	}
	if request.Locations, err = resolveLabels(locations, mtkprovider.LocationOptions, "location"); err != nil {
		return mtkprovider.SearchRequest{}, err
	}
	if request.Programs, err = resolveLabels(programs, mtkprovider.ProgramOptions, "program"); err != nil {
		return mtkprovider.SearchRequest{}, err
	}
	return request, nil
}

func resolveLabels(labels []string, options []mtkprovider.FilterOption, kind string) ([]string, error) {
	result := make([]string, 0, len(labels))
	for _, label := range labels {
		for _, option := range options {
			if option.Label == label {
				result = append(result, option.Code)
				goto next
			}
		}
		return nil, fmt.Errorf("invalid %s %q", kind, label)
	next:
	}
	return result, nil
}

func runSearch(ctx context.Context, baseURL string, timeout time.Duration, format string, request mtkprovider.SearchRequest) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, err := mtkprovider.NewClient(baseURL, nil).Search(ctx, request)
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(response)
	}

	fmt.Printf("total=%d page=%d/%d jobs=%d\n\n", response.Pagination.TotalItems, response.Pagination.CurrentPage, response.Pagination.TotalPages, len(response.Jobs))
	for i, job := range response.Jobs {
		fmt.Printf("%d. [%s] %s\n", i+1, job.ID, job.Title)
		fmt.Printf("   url: %s\n", mtkprovider.JobURL(baseURL, job.ID))
		if job.Category != "" {
			fmt.Printf("   category: %s\n", job.Category)
		}
		if job.Location != "" {
			fmt.Printf("   location: %s\n", job.Location)
		}
		if job.WorkExperience != "" {
			fmt.Printf("   experience: %s\n", job.WorkExperience)
		}
		if job.PublishedDate != "" {
			fmt.Printf("   published: %s\n", job.PublishedDate)
		}
		fmt.Println()
	}
	return nil
}

func runDetail(ctx context.Context, baseURL string, timeout time.Duration, format, jobID string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	detail, err := mtkprovider.NewClient(baseURL, nil).JobDetail(ctx, jobID)
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(detail)
	}

	fmt.Printf("[%s] %s\n", detail.ID, detail.Title)
	fmt.Printf("url: %s\n", detail.URL)
	if detail.Category != "" {
		fmt.Printf("category: %s\n", detail.Category)
	}
	if detail.Location != "" {
		fmt.Printf("location: %s\n", detail.Location)
	}
	if detail.Experience != "" {
		fmt.Printf("experience: %s\n", detail.Experience)
	}
	if detail.Education != "" {
		fmt.Printf("education: %s\n", detail.Education)
	}
	if detail.Description != "" {
		fmt.Printf("\nJob Description:\n%s\n", detail.Description)
	}
	if detail.Qualifications != "" {
		fmt.Printf("\nMain Requirements and Qualifications:\n%s\n", detail.Qualifications)
	}
	return nil
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
