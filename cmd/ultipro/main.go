package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/ultipro"
)

const pageSize = 20

func main() {
	rootFlags := ff.NewFlagSet("ultipro")
	var (
		company = rootFlags.StringLong("company", "", `curated company name, company code, or career-board URL, e.g. "TechnoServe", "TEC1006TESER", or a recruiting.ultipro.com/.../JobBoard/... URL`)
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "ultipro",
		Usage: "ultipro --company COMPANY [FLAGS] <companies|search|detail> [FLAGS]",
		Flags: rootFlags,
	}

	companiesFlags := ff.NewFlagSet("companies").SetParent(rootFlags)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "ultipro companies [--format text|json]",
		ShortHelp: "list curated UltiPro companies (company name and company code)",
		Flags:     companiesFlags,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v", args)
			}
			return runCompanies(*format)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword      = searchFS.StringLong("keyword", "", "free-text keyword search")
		location     = searchFS.StringLong("location", "", "physical-location catalog id or display label")
		category     = searchFS.StringLong("category", "", "job-category catalog id or display label")
		locationType = searchFS.StringEnumLong("location-type", "job location type", "", "hybrid", "onsite", "remote")
		page         = searchFS.IntLong("page", 1, "one-based page number")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "ultipro --company COMPANY search [--keyword TEXT] [--location ID|LABEL] [--category ID|LABEL] [--location-type hybrid|onsite|remote] [--page N] [--format text|json]",
		ShortHelp: "search postings for a company (server-side keyword/location/category/location-type)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runSearch(ctx, searchFlags{
				company:      *company,
				timeout:      *timeout,
				keyword:      *keyword,
				location:     *location,
				category:     *category,
				locationType: *locationType,
				page:         *page,
				format:       *format,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	opportunityID := detailFS.StringLong("id", "", "opportunityId (job_id) from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "ultipro --company COMPANY detail --id OPPORTUNITY-ID [--format text|json]",
		ShortHelp: "print one posting in full (description and location)",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --id %q?)", args, args[0])
			}
			return runDetail(ctx, detailFlags{company: *company, timeout: *timeout, opportunityID: *opportunityID, format: *format})
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
		fmt.Fprintln(os.Stderr, "err: a subcommand (companies, search, or detail) is required")
		os.Exit(1)
	}

	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}

// resolveCompany accepts a curated display name, a curated company code, or
// any recognized career-board URL (for live debugging beyond the seed
// roster — mirrors cmd/icims's any-host allowance).
func resolveCompany(company string) (name string, site ultipro.CareersSite, err error) {
	if company == "" {
		return "", ultipro.CareersSite{}, errors.New("--company is required")
	}
	if c, ok := ultipro.CompaniesByCode[strings.ToLower(company)]; ok {
		return c.Name, ultipro.CareersSite{Host: c.Host, CompanyCode: c.CompanyCode, BoardID: c.BoardID}, nil
	}
	for _, c := range ultipro.Companies {
		if strings.EqualFold(c.Name, company) {
			return c.Name, ultipro.CareersSite{Host: c.Host, CompanyCode: c.CompanyCode, BoardID: c.BoardID}, nil
		}
	}
	if u, perr := url.Parse(ensureScheme(company)); perr == nil {
		if s, ok := ultipro.ParseCareersURL(u); ok {
			return s.CompanyCode, s, nil
		}
	}
	return "", ultipro.CareersSite{}, fmt.Errorf("company %q not found; run 'ultipro companies' to see supported companies", company)
}

func ensureScheme(raw string) string {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return "https://" + raw
}

func runCompanies(format string) error {
	cs := ultipro.Companies
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cs)
	}
	for _, c := range cs {
		fmt.Printf("%s (%s)\n", c.Name, c.CompanyCode)
	}
	return nil
}

type searchFlags struct {
	company      string
	timeout      time.Duration
	keyword      string
	location     string
	category     string
	locationType string
	page         int
	format       string
}

// locationTypeCodes maps the CLI's enum flag to the LoadSearchResults
// fieldName-37 values (see openapi.yaml: 0=Hybrid, 1=On-site, 2=Remote).
var locationTypeCodes = map[string]string{"hybrid": "0", "onsite": "1", "remote": "2"}

func runSearch(ctx context.Context, f searchFlags) error {
	name, site, err := resolveCompany(f.company)
	if err != nil {
		return err
	}
	if f.page < 1 {
		return fmt.Errorf("--page must be >= 1, got %d", f.page)
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := ultipro.NewClient(site.BaseURL(), nil)

	req := ultipro.SearchRequest{
		Query: f.keyword,
		Top:   pageSize,
		Skip:  (f.page - 1) * pageSize,
	}
	if f.location != "" {
		id, err := resolveCatalogValue(ctx, client.Locations, f.location, "location")
		if err != nil {
			return err
		}
		req.Filters = append(req.Filters, ultipro.SearchFilter{FieldName: 4, Values: []string{id}})
	}
	if f.category != "" {
		id, err := resolveCatalogValue(ctx, client.Categories, f.category, "category")
		if err != nil {
			return err
		}
		req.Filters = append(req.Filters, ultipro.SearchFilter{FieldName: 5, Values: []string{id}})
	}
	if f.locationType != "" {
		req.Filters = append(req.Filters, ultipro.SearchFilter{FieldName: 37, Values: []string{locationTypeCodes[f.locationType]}})
	}

	res, err := client.Search(ctx, req)
	if err != nil {
		return err
	}

	type jobJSON struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Location string `json:"location,omitempty"`
		Category string `json:"category,omitempty"`
		PostedAt string `json:"posted_at,omitempty"`
	}
	jobs := make([]jobJSON, len(res.Opportunities))
	for i, o := range res.Opportunities {
		loc := ""
		if len(o.Locations) > 0 {
			loc = o.Locations[0].Display()
		}
		jobs[i] = jobJSON{ID: o.ID, Title: o.Title, Location: loc, Category: o.JobCategoryName, PostedAt: o.PostedDate}
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"total": res.TotalCount, "jobs": jobs})
	}

	fmt.Printf("UltiPro Jobs Report (company: %s)\n", name)
	fmt.Printf("Found %d jobs; showing %d\n\n", res.TotalCount, len(jobs))
	for i, j := range jobs {
		fmt.Printf("%d. %s\n", i+1, j.Title)
		if j.Location != "" {
			fmt.Printf("Location: %s\n", j.Location)
		}
		if j.Category != "" {
			fmt.Printf("Category: %s\n", j.Category)
		}
		if j.PostedAt != "" {
			fmt.Printf("Posted: %s\n", j.PostedAt)
		}
		fmt.Printf("ID: %s\n\n", j.ID)
	}
	return nil
}

// resolveCatalogValue accepts either a raw catalog id (passed through
// unchanged) or a display label (resolved via one catalog call, exact
// case-insensitive match).
func resolveCatalogValue(ctx context.Context, fetch func(context.Context) ([]ultipro.FilterCatalog, error), input, kind string) (string, error) {
	catalog, err := fetch(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch %s catalog: %w", kind, err)
	}
	for _, c := range catalog {
		if c.ID == input {
			return c.ID, nil
		}
	}
	for _, c := range catalog {
		if strings.EqualFold(c.Label, input) {
			return c.ID, nil
		}
	}
	const maxListed = 20
	labels := make([]string, 0, len(catalog))
	for _, c := range catalog {
		labels = append(labels, c.Label)
	}
	if len(labels) > maxListed {
		labels = labels[:maxListed]
	}
	return "", fmt.Errorf("%s %q not found; available: %s", kind, input, strings.Join(labels, ", "))
}

type detailFlags struct {
	company       string
	timeout       time.Duration
	opportunityID string
	format        string
}

func runDetail(ctx context.Context, f detailFlags) error {
	name, site, err := resolveCompany(f.company)
	if err != nil {
		return err
	}
	if strings.TrimSpace(f.opportunityID) == "" {
		return errors.New("--id is required")
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client := ultipro.NewClient(site.BaseURL(), nil)
	d, err := client.Detail(ctx, f.opportunityID)
	if err != nil {
		return err
	}

	desc := d.Description
	if desc != "" {
		if text, err := html2text.FromString(desc, html2text.Options{}); err == nil {
			desc = text
		}
	}
	loc := ""
	if len(d.Locations) > 0 {
		loc = d.Locations[0].Display()
	}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"id":                 d.ID,
			"title":              d.Title,
			"company":            name,
			"location":           loc,
			"requisition_number": d.RequisitionNumber,
			"category":           d.JobCategoryName,
			"posted_at":          d.PostedDate,
			"url":                site.CanonicalURL() + "OpportunityDetail?opportunityId=" + url.QueryEscape(d.ID),
			"description":        desc,
		})
	}

	fmt.Printf("%s\n", d.Title)
	fmt.Printf("Company: %s\n", name)
	if loc != "" {
		fmt.Printf("Location: %s\n", loc)
	}
	if d.JobCategoryName != "" {
		fmt.Printf("Category: %s\n", d.JobCategoryName)
	}
	if d.PostedDate != "" {
		fmt.Printf("Posted: %s\n", d.PostedDate)
	}
	fmt.Printf("ID: %s\n", d.ID)
	fmt.Printf("URL: %sOpportunityDetail?opportunityId=%s\n\n", site.CanonicalURL(), url.QueryEscape(d.ID))
	fmt.Println(desc)
	return nil
}
