package ultipro

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
	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/provider/ultipro"
)

const pageSize = 20

type rootOptions struct {
	company string
	timeout time.Duration
	format  string
}

// NewCommand returns a cobra.Command for ultipro.
func NewCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:          "ultipro",
		Short:        "Search UltiPro jobs and view position details",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&opts.company, "company", "", `curated company name, company code, or career-board URL, e.g. "TechnoServe", "TEC1006TESER", or a recruiting.ultipro.com/.../JobBoard/... URL`)
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", 60*time.Second, "request timeout")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "text", "output format (text|json)")

	companiesCmd := &cobra.Command{
		Use:          "companies",
		Short:        "list curated UltiPro companies (company name and company code)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runCompanies(opts.format)
		},
	}

	var (
		searchKeyword      string
		searchLocation     string
		searchCategory     string
		searchLocationType string
		searchPage         int
	)
	searchCmd := &cobra.Command{
		Use:          "search",
		Short:        "search postings for a company (server-side keyword/location/category/location-type)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runSearch(cmd.Context(), searchFlags{
				company:      opts.company,
				timeout:      opts.timeout,
				keyword:      searchKeyword,
				location:     searchLocation,
				category:     searchCategory,
				locationType: searchLocationType,
				page:         searchPage,
				format:       opts.format,
			})
		},
	}
	searchCmd.Flags().StringVar(&searchKeyword, "keyword", "", "free-text keyword search")
	searchCmd.Flags().StringVar(&searchLocation, "location", "", "physical-location catalog id or display label")
	searchCmd.Flags().StringVar(&searchCategory, "category", "", "job-category catalog id or display label")
	searchCmd.Flags().StringVar(&searchLocationType, "location-type", "", "job location type (hybrid|onsite|remote)")
	searchCmd.Flags().IntVar(&searchPage, "page", 1, "one-based page number")

	var detailJobID string
	detailCmd := &cobra.Command{
		Use:          "detail",
		Short:        "print one posting in full (description and location)",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.format != "text" && opts.format != "json" {
				return fmt.Errorf("invalid format %q (must be text or json)", opts.format)
			}
			return runDetail(cmd.Context(), detailFlags{
				company:       opts.company,
				timeout:       opts.timeout,
				opportunityID: detailJobID,
				format:        opts.format,
			})
		},
	}
	detailCmd.Flags().StringVar(&detailJobID, "id", "", "opportunityId (job_id) from a search result")

	cmd.AddCommand(companiesCmd, searchCmd, detailCmd)
	return cmd
}

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
