// Package avature provides CLI command for Avature career portals.
package avature

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/avature"
)

type options struct {
	company string
	timeout time.Duration
	format  string
}

type searchFlags struct {
	keyword string
	offset  int
}

type detailFlags struct {
	jobID string
}

// NewCommand returns a cobra.Command for avature.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "avature",
		Short:        "Avature career-portal debug CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.company, "company", "", `curated company name or portal slug (e.g. "Bloomberg" or "koch.avature.net/careers")`)
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated Avature portals (company name and portal slug)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompanies(opts.format)
		},
	}

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "fetch one listing page (server-side keyword search)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sFlags.offset < 0 {
				return fmt.Errorf("--offset must be >= 0, got %d", sFlags.offset)
			}
			return runSearch(cmd.Context(), opts.company, opts.timeout, sFlags.keyword, sFlags.offset, opts.format)
		},
	}
	searchCmd.Flags().StringVar(&sFlags.keyword, "keyword", "", "full-text query over titles and descriptions")
	searchCmd.Flags().IntVar(&sFlags.offset, "offset", 0, "zero-based result offset")

	dFlags := &detailFlags{}
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one posting in full (metadata fields and description)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetail(cmd.Context(), opts.company, opts.timeout, dFlags.jobID, opts.format)
		},
	}
	detailCmd.Flags().StringVar(&dFlags.jobID, "id", "", "numeric job id from a search result")

	rootCmd.AddCommand(companiesCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(detailCmd)

	return rootCmd
}

func resolvePortal(company string) (baseURL string, err error) {
	company = strings.TrimSpace(company)
	if company == "" {
		return "", errors.New("--company is required")
	}
	for _, c := range avature.Companies {
		if strings.EqualFold(c.Name, company) || strings.EqualFold(c.Slug(), company) {
			return c.URL, nil
		}
	}
	slug := strings.TrimPrefix(company, "https://")
	if host, portal, ok := strings.Cut(slug, "/"); ok && strings.Contains(host, ".") && portal != "" {
		return "https://" + slug, nil
	}
	return "", fmt.Errorf("company %q not found; run 'avature companies', or pass a portal slug like koch.avature.net/careers", company)
}

func runCompanies(format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(avature.Companies)
	}
	for _, c := range avature.Companies {
		fmt.Printf("%s (%s)\n", c.Name, c.Slug())
	}
	return nil
}

func runSearch(ctx context.Context, company string, timeout time.Duration, keyword string, offset int, format string) error {
	baseURL, err := resolvePortal(company)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := avature.NewClient(baseURL, nil).Search(ctx, &avature.SearchRequest{Search: keyword, Offset: offset})
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Printf("Avature Jobs Report (portal: %s)\n", strings.TrimPrefix(baseURL, "https://"))
	switch {
	case res.Total >= 0:
		fmt.Printf("Found %d jobs; showing %d from offset %d\n\n", res.Total, len(res.Jobs), offset)
	case res.HasNext:
		fmt.Printf("Showing %d jobs from offset %d (total hidden by portal; more pages exist)\n\n", len(res.Jobs), offset)
	default:
		fmt.Printf("Showing %d jobs from offset %d (total hidden by portal; last page)\n\n", len(res.Jobs), offset)
	}
	for i, j := range res.Jobs {
		fmt.Printf("%d. %s\n", i+1, j.Title)
		if j.Location != "" {
			fmt.Printf("Location: %s\n", j.Location)
		}
		fmt.Printf("ID: %s\n", j.ID)
		fmt.Printf("URL: %s\n\n", j.URL)
	}
	return nil
}

func runDetail(ctx context.Context, company string, timeout time.Duration, jobID, format string) error {
	baseURL, err := resolvePortal(company)
	if err != nil {
		return err
	}
	if jobID == "" {
		return errors.New("--id is required (take it from a search result's ID)")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	d, err := avature.NewClient(baseURL, nil).JobDetail(ctx, jobID)
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Title)
	for _, f := range d.Fields {
		fmt.Printf("%s: %s\n", f.Label, f.Value)
	}
	fmt.Printf("URL: %s\n", d.URL)
	if d.DescriptionHTML != "" {
		text, err := html2text.FromString(d.DescriptionHTML, html2text.Options{})
		if err != nil {
			text = d.DescriptionHTML
		}
		fmt.Printf("\n%s\n", text)
	}
	return nil
}
