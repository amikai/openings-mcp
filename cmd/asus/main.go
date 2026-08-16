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
	os.Exit(run())
}

func run() int {
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
		category   = searchFS.StringLong("category", "", "category prefix from the codes subcommand (e.g. 研究發展); an unrecognized one is ignored by the board")
		location   = searchFS.StringLong("location", "", "ISO 3166-1 alpha-2 country code (e.g. TW, US, CN, JP; Slovenia is SL)")
		city       = searchFS.StringLong("city", "", "city code from cities subcommand (e.g. TPE, HSI)")
		experience = searchFS.StringLong("experience", "", "experience level code from the codes subcommand (e.g. 0); an unrecognized one is ignored by the board")
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
		ShortHelp: "list the search form's filter options (categories, countries, experience levels)",
		Flags:     codesFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("codes takes no positional arguments, got %v", args)
			}
			return runCodes(ctx, *baseURL, *timeout, *format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, codesCmd)

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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, detail, cities, or codes) is required")
		return 1
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
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
	var categories []string
	if f.category != "" {
		categories = append(categories, f.category)
	}

	resp, err := client.Search(ctx, &asus.SearchRequest{
		Keyword:    f.keyword,
		Categories: categories,
		Location:   f.location,
		City:       f.city,
		Experience: f.experience,
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

// runCodes prints the filter options the /Jobs search form itself offers,
// which is where the valid --category, --location and --experience values
// come from. They are locale-bound: the same board serves English category
// values to an en-US session.
func runCodes(ctx context.Context, baseURL string, timeout time.Duration, format string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := asus.NewClient(baseURL, nil).Search(ctx, nil)
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string][]asus.FilterOption{
			"categories":  resp.Categories,
			"countries":   resp.Countries,
			"experiences": resp.Experiences,
		})
	}

	for _, group := range []struct {
		flag    string
		options []asus.FilterOption
	}{
		{"--category", resp.Categories},
		{"--location", resp.Countries},
		{"--experience", resp.Experiences},
	} {
		fmt.Printf("--- %s (%d) ---\n", group.flag, len(group.options))
		for _, o := range group.options {
			if o.Label == o.Value {
				fmt.Printf("  %s\n", o.Value)
				continue
			}
			fmt.Printf("  %-6s %s\n", o.Value, o.Label)
		}
		fmt.Println()
	}
	return nil
}
