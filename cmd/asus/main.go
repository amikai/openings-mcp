// Command asus is a debug CLI for ASUS Careers (https://recruit.asus.com).
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

	"github.com/amikai/openings-mcp/internal/provider/asus"
)

const defaultBaseURL = "https://recruit.asus.com"

func main() {
	rootFlags := ff.NewFlagSet("asus")
	baseURL := rootFlags.StringLong("base-url", defaultBaseURL, "ASUS Careers base URL")
	timeout := rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
	format := rootFlags.StringEnumLong("format", "output format", "text", "json")
	rootCmd := &ff.Command{
		Name:  "asus",
		Usage: "asus [FLAGS] <search|detail|cities|codes> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword    = searchFS.StringLong("keyword", "", "free-text keyword search across job titles")
		category   = searchFS.StringEnumLong("category", "category prefix (choices: 研究發展, 業務/行銷, 工程技術, 管理/支援)", "", "研究發展", "業務/行銷", "工程技術", "管理/支援")
		location   = searchFS.StringLong("location", "", "ISO 3166-1 alpha-2 country code (e.g. TW, US, CN, JP; Slovenia is SL)")
		city       = searchFS.StringLong("city", "", "city code from cities subcommand (e.g. TPE, HSI)")
		experience = searchFS.StringEnumLong("experience", "experience level code (choices: 0: <2y, 1: 3-5y, 2: 6-10y, 3: 11-15y, 4: 16y+)", "", "0", "1", "2", "3", "4")
		page       = searchFS.IntLong("page", 1, "1-based page number")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "asus search [--keyword TEXT] [--category CODE] [--location CODE] [--city CODE] [--experience CODE] [--page NUM] [--format text|json]",
		ShortHelp: "search job vacancies",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			if *page < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", *page)
			}
			return runSearch(ctx, *baseURL, *timeout, *format, searchFlags{
				keyword:    *keyword,
				category:   *category,
				location:   *location,
				city:       *city,
				experience: *experience,
				page:       *page,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	id := detailFS.StringLong("id", "", "opaque job UUID (sn from search results)")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "asus detail --id ID [--format text|json]",
		ShortHelp: "fetch one vacancy's full detail by its opaque id",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v", args)
			}
			if *id == "" {
				return errors.New("--id is required (sn UUID from a search result)")
			}
			return runDetail(ctx, *baseURL, *timeout, *id, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, detailCmd)

	citiesFS := ff.NewFlagSet("cities").SetParent(rootFlags)
	country := citiesFS.StringLong("country", "TW", "ISO 3166-1 alpha-2 country code (e.g. TW, US, CN; Slovenia is SL)")
	citiesCmd := &ff.Command{
		Name:      "cities",
		Usage:     "asus cities [--country CODE] [--format text|json]",
		ShortHelp: "list available cities for a country code",
		Flags:     citiesFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("cities takes no positional arguments, got %v", args)
			}
			return runCities(ctx, *baseURL, *timeout, *country, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, citiesCmd)

	codesFS := ff.NewFlagSet("codes").SetParent(rootFlags)
	codesCmd := &ff.Command{
		Name:      "codes",
		Usage:     "asus codes [--format text|json]",
		ShortHelp: "list known filter options (categories, experience levels)",
		Flags:     codesFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("codes takes no positional arguments, got %v", args)
			}
			return runCodes(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, codesCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, detail, cities, or codes) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

type searchFlags struct {
	keyword    string
	category   string
	location   string
	city       string
	experience string
	page       int
}

func runSearch(ctx context.Context, baseURL string, timeout time.Duration, format string, f searchFlags) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := asus.NewClient(baseURL, nil)
	var categories []asus.Category
	if f.category != "" {
		categories = append(categories, asus.Category(f.category))
	}

	resp, err := client.Search(ctx, &asus.SearchRequest{
		Keyword:    f.keyword,
		Categories: categories,
		Location:   f.location,
		City:       f.city,
		Experience: asus.ExperienceLevel(f.experience),
		Page:       f.page,
	})
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	}

	fmt.Printf("Page %d of %d (Found %d jobs on this page)\n\n", resp.CurrentPage, resp.TotalPages, len(resp.Jobs))
	for i, j := range resp.Jobs {
		fmt.Printf("[%d] %s\n", i+1, j.Title)
		if j.JobNo != "" {
			fmt.Printf("    Job No:     %s\n", j.JobNo)
		}
		if j.Category != "" {
			fmt.Printf("    Category:   %s\n", j.Category)
		}
		if j.Location != "" {
			fmt.Printf("    Location:   %s\n", j.Location)
		}
		if j.Experience != "" {
			fmt.Printf("    Experience: %s\n", j.Experience)
		}
		if j.Education != "" {
			fmt.Printf("    Education:  %s\n", j.Education)
		}
		if j.ID != "" {
			fmt.Printf("    ID:         %s\n", j.ID)
		}
		if j.DetailURL != "" {
			fmt.Printf("    Detail URL: %s\n", j.DetailURL)
		}
		fmt.Println()
	}
	return nil
}

func runDetail(ctx context.Context, baseURL string, timeout time.Duration, id, format string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := asus.NewClient(baseURL, nil)
	detail, err := client.Detail(ctx, id)
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(detail)
	}

	fmt.Printf("Title:          %s\n", detail.Title)
	if detail.JobNo != "" {
		fmt.Printf("Job No:         %s\n", detail.JobNo)
	}
	if detail.Category != "" {
		fmt.Printf("Category:       %s\n", detail.Category)
	}
	if detail.Location != "" {
		fmt.Printf("Location:       %s\n", detail.Location)
	}
	if detail.Experience != "" {
		fmt.Printf("Experience:     %s\n", detail.Experience)
	}
	if detail.Education != "" {
		fmt.Printf("Education:      %s\n", detail.Education)
	}
	if detail.EmploymentType != "" {
		fmt.Printf("Job Type:       %s\n", detail.EmploymentType)
	}
	if detail.ApplyURL != "" {
		fmt.Printf("Apply URL:      %s\n", detail.ApplyURL)
	}
	if detail.Description != "" {
		fmt.Printf("\n--- Description ---\n%s\n", detail.Description)
	}
	if detail.Requirements != "" {
		fmt.Printf("\n--- Requirements ---\n%s\n", detail.Requirements)
	}
	return nil
}

func runCities(ctx context.Context, baseURL string, timeout time.Duration, country, format string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := asus.NewClient(baseURL, nil)
	cities, err := client.GetCities(ctx, country)
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cities)
	}

	fmt.Printf("Cities for %s (%d found):\n", country, len(cities))
	for _, c := range cities {
		fmt.Printf("  %-6s %s\n", c.Value, c.Text)
	}
	return nil
}

func runCodes(format string) error {
	type codeDoc struct {
		Categories map[string]string `json:"categories"`
		Experience map[string]string `json:"experience"`
	}
	doc := codeDoc{
		Categories: map[string]string{
			"研究發展": "Research and Development",
			"業務/行銷": "Marketing / Sales",
			"工程技術": "Technology / Engineering",
			"管理/支援": "Business Support / Administration",
		},
		Experience: map[string]string{
			"0": "2年以下 (Under 2 years)",
			"1": "3-5年 (3-5 years)",
			"2": "6-10年 (6-10 years)",
			"3": "11-15年 (11-15 years)",
			"4": "16年以上 (16 years and above)",
		},
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	}

	fmt.Println("--- Categories (--category) ---")
	for k, v := range doc.Categories {
		fmt.Printf("  %-10s %s\n", k, v)
	}
	fmt.Println("\n--- Experience Levels (--experience) ---")
	for k, v := range doc.Experience {
		fmt.Printf("  %-3s %s\n", k, v)
	}
	return nil
}
