// Command mediatek is a debug CLI for MediaTek Careers' public search API and
// server-rendered job detail pages.
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

	"github.com/amikai/openings-mcp/internal/provider/mediatek"
)

const defaultBaseURL = "https://careers.mediatek.com"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("mediatek")
	baseURL := rootFlags.StringLong("base-url", defaultBaseURL, "MediaTek Careers base URL")
	timeout := rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
	format := rootFlags.StringEnumLong("format", "output format", "text", "json")
	rootCmd := &ff.Command{
		Name:  "mediatek",
		Usage: "mediatek [FLAGS] <search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFlags := ff.NewFlagSet("search").SetParent(rootFlags)
	keyword := searchFlags.StringLong("keyword", "", "free-text keyword query; AND/OR joins are supported")
	category := searchFlags.StringSetLong("category", "category label (repeatable)")
	experience := searchFlags.StringSetLong("experience", "work-experience label (repeatable)")
	location := searchFlags.StringSetLong("location", "location label (repeatable)")
	program := searchFlags.StringSetLong("program", "program label (repeatable)")
	page := searchFlags.IntLong("page", 1, "1-based page number")
	limit := searchFlags.IntLong("limit", 6, "page size (1-100)")
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "mediatek search [--keyword TEXT] [--category LABEL] [--experience LABEL] [--location LABEL] [--program LABEL] [--page N] [--limit N] [--format text|json]",
		ShortHelp: "search MediaTek Careers jobs",
		Flags:     searchFlags,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			if *page < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", *page)
			}
			if *limit < 1 || *limit > 100 {
				return fmt.Errorf("--limit must be between 1 and 100, got %d", *limit)
			}
			request, err := searchRequest(*keyword, *category, *experience, *location, *program, *page, *limit)
			if err != nil {
				return err
			}
			return runSearch(ctx, *baseURL, *timeout, *format, request)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFlags := ff.NewFlagSet("detail").SetParent(rootFlags)
	jobID := detailFlags.StringLong("job-id", "", "MediaTek job ID from search, e.g. MTK120260629001")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "mediatek detail --job-id ID [--format text|json]",
		ShortHelp: "fetch one MediaTek job detail page",
		Flags:     detailFlags,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v", args)
			}
			if *jobID == "" {
				return errors.New("--job-id is required")
			}
			return runDetail(ctx, *baseURL, *timeout, *format, *jobID)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

	if err := rootCmd.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd.GetSelected()))
		if errors.Is(err, ff.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	if rootCmd.GetSelected() == rootCmd {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd))
		fmt.Fprintln(os.Stderr, "err: a subcommand (search or detail) is required")
		return 1
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

func searchRequest(keyword string, categories, experiences, locations, programs []string, page, limit int) (mediatek.SearchRequest, error) {
	var err error
	request := mediatek.SearchRequest{Keyword: keyword, Page: page, Limit: limit}
	if request.Categories, err = resolveLabels(categories, mediatek.CategoryOptions, "category"); err != nil {
		return mediatek.SearchRequest{}, err
	}
	if request.WorkExperiences, err = resolveLabels(experiences, mediatek.WorkExperienceOptions, "experience"); err != nil {
		return mediatek.SearchRequest{}, err
	}
	if request.Locations, err = resolveLabels(locations, mediatek.LocationOptions, "location"); err != nil {
		return mediatek.SearchRequest{}, err
	}
	if request.Programs, err = resolveLabels(programs, mediatek.ProgramOptions, "program"); err != nil {
		return mediatek.SearchRequest{}, err
	}
	return request, nil
}

func resolveLabels(labels []string, options []mediatek.FilterOption, kind string) ([]string, error) {
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

func runSearch(ctx context.Context, baseURL string, timeout time.Duration, format string, request mediatek.SearchRequest) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, err := mediatek.NewClient(baseURL, nil).Search(ctx, request)
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(response)
	}

	fmt.Printf("total=%d page=%d/%d jobs=%d\n\n", response.Pagination.TotalItems, response.Pagination.CurrentPage, response.Pagination.TotalPages, len(response.Jobs))
	for i, job := range response.Jobs {
		fmt.Printf("%d. [%s] %s\n", i+1, job.ID, job.Title)
		fmt.Printf("   url: %s\n", mediatek.JobURL(baseURL, job.ID))
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

	detail, err := mediatek.NewClient(baseURL, nil).JobDetail(ctx, jobID)
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
