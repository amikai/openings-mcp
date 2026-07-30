package eightfold

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/spf13/cobra"

	eightfold "github.com/amikai/openings-mcp/internal/provider/eightfold"
)

type options struct {
	company string
	timeout time.Duration
	format  string
}

type searchFlags struct {
	keyword  string
	location string
	filters  []string
	start    int
}

type detailFlags struct {
	positionID string
}

// NewCommand returns a cobra.Command for eightfold.
func NewCommand() *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:          "eightfold",
		Short:        "Eightfold ATS postings CLI",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&opts.company, "company", "", `Eightfold tenant slug from companies, e.g. "morganstanley"`)
	rootCmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	rootCmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated Eightfold companies (company name and tenant)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompanies(opts.format)
		},
	}

	sFlags := &searchFlags{}
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search postings for a company (server-side query, location, and facet filters)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), searchOptions{
				company:  opts.company,
				timeout:  opts.timeout,
				keyword:  sFlags.keyword,
				location: sFlags.location,
				filters:  sFlags.filters,
				start:    sFlags.start,
				format:   opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&sFlags.keyword, "keyword", "", "free-text keyword search across posting titles and descriptions")
	searchCmd.Flags().StringVar(&sFlags.location, "location", "", "free-text fuzzy location match")
	searchCmd.Flags().StringSliceVar(&sFlags.filters, "filter", nil, "facet filter as name=value (repeatable)")
	searchCmd.Flags().IntVar(&sFlags.start, "start", 0, "zero-based result offset")

	filtersCmd := &cobra.Command{
		Use:          "filters",
		Short:        "list the company's facet filter names and values",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFilters(cmd.Context(), opts.company, opts.timeout, opts.format)
		},
	}

	dFlags := &detailFlags{}
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one posting in full (description and public URL)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetail(cmd.Context(), detailOptions{
				company:    opts.company,
				timeout:    opts.timeout,
				positionID: dFlags.positionID,
				format:     opts.format,
			})
		},
	}
	detailCmd.Flags().StringVar(&dFlags.positionID, "id", "", "position id from a search result")

	rootCmd.AddCommand(companiesCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(filtersCmd)
	rootCmd.AddCommand(detailCmd)

	return rootCmd
}

func resolveCompany(company string) (eightfold.RosterCompany, error) {
	if company == "" {
		return eightfold.RosterCompany{}, errors.New("--company is required")
	}
	c, ok := eightfold.CompaniesByTenant[strings.ToLower(company)]
	if !ok {
		return eightfold.RosterCompany{}, fmt.Errorf("company %q not found; run 'eightfold companies' to see supported companies", company)
	}
	return c, nil
}

func baseURL(c eightfold.RosterCompany) string {
	return fmt.Sprintf("https://%s.eightfold.ai", c.Tenant)
}

func httpClient() *http.Client {
	return &http.Client{Transport: eightfold.BrowserTransport{}}
}

func runCompanies(format string) error {
	cs := eightfold.Companies

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}

	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.Tenant)
	}
	return nil
}

func parseFilters(raw []string) (map[string][]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string][]string, len(raw))
	for _, f := range raw {
		name, value, ok := strings.Cut(f, "=")
		if !ok || name == "" || value == "" {
			return nil, fmt.Errorf("--filter %q must be name=value", f)
		}
		out[name] = append(out[name], value)
	}
	return out, nil
}

type positionSummaryJSON struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Location   string `json:"location,omitempty"`
	Department string `json:"department,omitempty"`
	PostedAt   string `json:"postedAt,omitempty"`
	URL        string `json:"url,omitempty"`
}

type searchResultJSON struct {
	Total int                   `json:"total"`
	Jobs  []positionSummaryJSON `json:"jobs"`
}

func summarize(p eightfold.Position, tenantURL string) positionSummaryJSON {
	s := positionSummaryJSON{
		ID:         strconv.FormatInt(p.ID, 10),
		Title:      p.Name,
		Department: p.Department.Value,
		PostedAt:   time.Unix(int64(p.PostedTs), 0).UTC().Format("2006-01-02"),
		URL:        tenantURL + p.PositionUrl,
	}
	if len(p.Locations) > 0 {
		s.Location = strings.Join(p.Locations, "; ")
	}
	return s
}

func printSummary(s positionSummaryJSON) {
	if s.Location != "" {
		fmt.Printf("Location: %s\n", s.Location)
	}
	if s.Department != "" {
		fmt.Printf("Department: %s\n", s.Department)
	}
	if s.PostedAt != "" {
		fmt.Printf("Posted: %s\n", s.PostedAt)
	}
	fmt.Printf("ID: %s\n", s.ID)
}

type searchOptions struct {
	company  string
	timeout  time.Duration
	keyword  string
	location string
	filters  []string
	start    int
	format   string
}

func runSearch(ctx context.Context, f searchOptions) error {
	c, err := resolveCompany(f.company)
	if err != nil {
		return err
	}
	if f.start < 0 {
		return fmt.Errorf("--start must be >= 0, got %d", f.start)
	}
	parsedFilters, err := parseFilters(f.filters)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	params := eightfold.SearchParams{
		Domain: c.Domain,
		Start:  eightfold.NewOptInt(f.start),
	}
	if f.keyword != "" {
		params.Query = eightfold.NewOptString(f.keyword)
	}
	if f.location != "" {
		params.Location = eightfold.NewOptString(f.location)
	}

	base := baseURL(c)
	var res *eightfold.SearchResponse
	if len(parsedFilters) > 0 {
		res, err = eightfold.SearchFiltered(ctx, eightfold.FilteredSearch{
			HTTPClient: httpClient(),
			BaseURL:    base,
			Params:     params,
			Filters:    parsedFilters,
		})
		if err != nil {
			return err
		}
	} else {
		client, err := eightfold.NewClient(base, eightfold.WithClient(httpClient()))
		if err != nil {
			return err
		}
		res, err = client.Search(ctx, params)
		if err != nil {
			return err
		}
	}

	jobs := make([]positionSummaryJSON, len(res.Data.Positions))
	for i, p := range res.Data.Positions {
		jobs[i] = summarize(p, base)
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{Total: res.Data.Count, Jobs: jobs})
	}

	fmt.Printf("Eightfold Jobs Report (company: %s)\n", c.Name)
	fmt.Printf("Found %d jobs; showing %d\n\n", res.Data.Count, len(jobs))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		printSummary(s)
		fmt.Println()
	}
	return nil
}

func runFilters(ctx context.Context, company string, timeout time.Duration, format string) error {
	c, err := resolveCompany(company)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := eightfold.NewClient(baseURL(c), eightfold.WithClient(httpClient()))
	if err != nil {
		return err
	}
	return printFilters(ctx, client, c.Domain, format, os.Stdout)
}

func printFilters(ctx context.Context, client *eightfold.Client, domain, format string, w io.Writer) error {
	res, err := client.Search(ctx, eightfold.SearchParams{Domain: domain})
	if err != nil {
		return err
	}

	type filterJSON struct {
		Name   string   `json:"name"`
		Title  string   `json:"title"`
		Values []string `json:"values"`
	}
	out := make([]filterJSON, 0)
	for _, sf := range eightfold.MergedFacets(res.Data.FilterDef) {
		if sf.Options == nil {
			continue
		}
		values := make([]string, len(sf.Options))
		for i, o := range sf.Options {
			values[i] = o.Value
		}
		out = append(out, filterJSON{Name: sf.FilterName, Title: sf.Title, Values: values})
	}

	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	for _, f := range out {
		fmt.Fprintf(w, "%s (%s): %s\n", f.Name, f.Title, strings.Join(f.Values, ", "))
	}
	return nil
}

type detailOptions struct {
	company    string
	timeout    time.Duration
	positionID string
	format     string
}

func runDetail(ctx context.Context, f detailOptions) error {
	c, err := resolveCompany(f.company)
	if err != nil {
		return err
	}
	if f.positionID == "" {
		return errors.New("--id is required (take it from a search result's ID)")
	}
	id, err := strconv.ParseInt(f.positionID, 10, 64)
	if err != nil {
		return fmt.Errorf("--id must be numeric, got %q", f.positionID)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := eightfold.NewClient(baseURL(c), eightfold.WithClient(httpClient()))
	if err != nil {
		return err
	}

	res, err := client.PositionDetails(ctx, eightfold.PositionDetailsParams{
		PositionID: id,
		Domain:     c.Domain,
	})
	if err != nil {
		return err
	}

	switch d := res.(type) {
	case *eightfold.PositionDetailsResponse:
		return printDetail(d.Data, baseURL(c), f.format)
	case *eightfold.PositionNotFoundResponse:
		return fmt.Errorf("position %q not found for company %q", f.positionID, c.Name)
	default:
		return fmt.Errorf("unexpected response type %T", res)
	}
}

func printDetail(d eightfold.PositionDetail, tenantURL, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	fmt.Println(d.Name)
	if len(d.Locations) > 0 {
		fmt.Printf("Location: %s\n", strings.Join(d.Locations, "; "))
	}
	if d.Department.Value != "" {
		fmt.Printf("Department: %s\n", d.Department.Value)
	}
	fmt.Printf("Posted: %s\n", time.Unix(int64(d.PostedTs), 0).UTC().Format("2006-01-02"))
	if u := detailPublicURL(d, tenantURL); u != "" {
		fmt.Printf("URL: %s\n", u)
	}

	if d.JobDescription != "" {
		rendered, err := html2text.FromString(d.JobDescription, html2text.Options{})
		if err != nil {
			rendered = d.JobDescription
		}
		fmt.Printf("\nDescription:\n%s\n", rendered)
	}
	return nil
}

func detailPublicURL(d eightfold.PositionDetail, tenantURL string) string {
	if u, ok := d.PublicUrl.Get(); ok && u != "" {
		return u
	}
	if d.PositionUrl == "" {
		return ""
	}
	return strings.TrimRight(tenantURL, "/") + d.PositionUrl
}
