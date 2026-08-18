// Command freehire is a debug CLI for freehire.me's public jobs API.
//
//	go run ./cmd/freehire search --query golang --work-mode remote
//	go run ./cmd/freehire search --company stripe --seniority staff
//	go run ./cmd/freehire search --query golang --sort posted_at --order desc
//	go run ./cmd/freehire facets --query golang --facets skills,countries
//	go run ./cmd/freehire companies --query adria
//	go run ./cmd/freehire detail --slug staff-software-engineer-link-stripe-32iy7vks
//
// Every search filter runs server-side. --company takes a catalogue slug
// (company_slug on a search hit), not a display name. See
// internal/provider/freehire/openapi.yaml for filter domains and quirks.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-faster/jx"
	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/freehire"
)

// apiBaseURL is freehire's public API origin — the single production
// server in the provider's openapi.yaml (paths are under /jobs).
const apiBaseURL = "https://freehire.me/api/v1"

func main() {
	os.Exit(run())
}

func run() int {
	rootFlags := ff.NewFlagSet("freehire")
	var (
		baseURL = rootFlags.StringLong("base-url", apiBaseURL, "freehire API base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "freehire",
		Usage: "freehire [FLAGS] <search|facets|companies|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		query     = searchFS.StringLong("query", "", "full-text search over title, company, and description")
		company   = searchFS.StringLong("company", "", "catalogue company_slug, e.g. stripe")
		skills    = searchFS.StringLong("skills", "", "comma-separated skill slugs, e.g. go,rust")
		seniority = searchFS.StringLong("seniority", "", "comma-separated seniority slugs, e.g. senior,staff")
		workMode  = searchFS.StringLong("work-mode", "", "comma-separated work arrangements: remote, hybrid, onsite")
		region    = searchFS.StringLong("region", "", "comma-separated regions, e.g. eu,apac")
		country   = searchFS.StringLong("country", "", "comma-separated lowercase ISO country codes, e.g. us,de")
		source    = searchFS.StringLong("source", "", "comma-separated ingest sources, e.g. greenhouse")
		category  = searchFS.StringLong("category", "", "comma-separated role categories, e.g. backend,devops")
		salaryMin = searchFS.IntLong("salary-min", 0, "minimum salary; 0 leaves it unset")
		salaryMax = searchFS.IntLong("salary-max", 0, "maximum salary; 0 leaves it unset")
		semantic  = searchFS.Float64Long("semantic-ratio", -1, "semantic/keyword blend 0-1; <0 leaves it unset")
		sortField = searchFS.StringEnumLong("sort", "sort field; omit for relevance", "", "created_at", "posted_at", "salary_min", "salary_max")
		order     = searchFS.StringEnumLong("order", "sort direction, with --sort", "", "asc", "desc")
		page      = searchFS.IntLong("page", 1, "1-based page number")
		limit     = searchFS.IntLong("limit", 20, "page size, 1-100")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "freehire search [--query TEXT] [--company SLUG] [--skills SLUGS] [--seniority LEVELS] [--work-mode KINDS] [--region REGIONS] [--country CCS] [--source SOURCES] [--category CATS] [--salary-min N] [--salary-max N] [--semantic-ratio R] [--sort FIELD] [--order asc|desc] [--page N] [--limit N] [--format text|json]",
		ShortHelp: "search IT jobs across freehire's catalogue (server-side filters)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			if *page < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", *page)
			}
			if *limit < 1 || *limit > 100 {
				return fmt.Errorf("--limit must be between 1 and 100, got %d", *limit)
			}
			return runSearch(ctx, searchFlags{
				baseURL:   *baseURL,
				timeout:   *timeout,
				format:    *format,
				query:     *query,
				company:   *company,
				skills:    *skills,
				seniority: *seniority,
				workMode:  *workMode,
				region:    *region,
				country:   *country,
				source:    *source,
				category:  *category,
				salaryMin: *salaryMin,
				salaryMax: *salaryMax,
				semantic:  *semantic,
				sortField: *sortField,
				order:     *order,
				page:      *page,
				limit:     *limit,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	facetsFS := ff.NewFlagSet("facets").SetParent(rootFlags)
	var (
		facetsQuery     = facetsFS.StringLong("query", "", "full-text query that scopes the facet counts")
		facetsCompany   = facetsFS.StringLong("company", "", "catalogue company_slug, e.g. stripe")
		facetsSkills    = facetsFS.StringLong("skills", "", "comma-separated skill slugs, e.g. go,rust")
		facetsSeniority = facetsFS.StringLong("seniority", "", "comma-separated seniority slugs, e.g. senior,staff")
		facetsWorkMode  = facetsFS.StringLong("work-mode", "", "comma-separated work arrangements: remote, hybrid, onsite")
		facetsRegion    = facetsFS.StringLong("region", "", "comma-separated regions, e.g. eu,apac")
		facetsCountry   = facetsFS.StringLong("country", "", "comma-separated lowercase ISO country codes, e.g. us,de")
		facetsSource    = facetsFS.StringLong("source", "", "comma-separated ingest sources, e.g. greenhouse")
		facetsCategory  = facetsFS.StringLong("category", "", "comma-separated role categories, e.g. backend,devops")
		facetsSalaryMin = facetsFS.IntLong("salary-min", 0, "minimum salary; 0 leaves it unset")
		facetsSalaryMax = facetsFS.IntLong("salary-max", 0, "maximum salary; 0 leaves it unset")
	)
	facetsCmd := &ff.Command{
		Name:      "facets",
		Usage:     "freehire facets [--query TEXT] [--company SLUG] [--skills SLUGS] [--seniority LEVELS] [--work-mode KINDS] [--region REGIONS] [--country CCS] [--source SOURCES] [--category CATS] [--salary-min N] [--salary-max N] [--format text|json]",
		ShortHelp: "print live filter vocabulary and counts (GET /jobs/facets)",
		Flags:     facetsFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("facets takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runFacets(ctx, searchFlags{
				baseURL:   *baseURL,
				timeout:   *timeout,
				format:    *format,
				query:     *facetsQuery,
				company:   *facetsCompany,
				skills:    *facetsSkills,
				seniority: *facetsSeniority,
				workMode:  *facetsWorkMode,
				region:    *facetsRegion,
				country:   *facetsCountry,
				source:    *facetsSource,
				category:  *facetsCategory,
				salaryMin: *facetsSalaryMin,
				salaryMax: *facetsSalaryMax,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, facetsCmd)

	companiesFS := ff.NewFlagSet("companies").SetParent(rootFlags)
	var (
		companiesQuery = companiesFS.StringLong("query", "", "company name to resolve; matching is fuzzy")
		companiesPage  = companiesFS.IntLong("page", 1, "1-based page number")
		companiesLimit = companiesFS.IntLong("limit", 20, "page size, 1-100")
	)
	companiesCmd := &ff.Command{
		Name:      "companies",
		Usage:     "freehire companies --query NAME [--page N] [--limit N] [--format text|json]",
		ShortHelp: "resolve a company name to the company_slug --company takes",
		Flags:     companiesFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v (did you mean --query %q?)", args, args[0])
			}
			if *companiesQuery == "" {
				return errors.New("companies requires --query; without it the endpoint pages through every company freehire holds")
			}
			if *companiesPage < 1 {
				return fmt.Errorf("--page must be >= 1, got %d", *companiesPage)
			}
			if *companiesLimit < 1 || *companiesLimit > 100 {
				return fmt.Errorf("--limit must be between 1 and 100, got %d", *companiesLimit)
			}
			return runCompanies(ctx, companiesFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				query:   *companiesQuery,
				page:    *companiesPage,
				limit:   *companiesLimit,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	slug := detailFS.StringLong("slug", "", "job public_slug from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "freehire detail --slug JOB-SLUG [--format text|json]",
		ShortHelp: "print one job in full by its public_slug",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --slug %q?)", args, args[0])
			}
			if *slug == "" {
				return errors.New("detail requires --slug")
			}
			return runDetail(ctx, detailFlags{
				baseURL: *baseURL,
				timeout: *timeout,
				format:  *format,
				slug:    *slug,
			})
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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, facets, companies, or detail) is required")
		return 1
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

type searchFlags struct {
	baseURL   string
	timeout   time.Duration
	format    string
	query     string
	company   string
	skills    string
	seniority string
	workMode  string
	region    string
	country   string
	source    string
	category  string
	salaryMin int
	salaryMax int
	semantic  float64
	sortField string
	order     string
	page      int
	limit     int
}

type jobSummaryJSON struct {
	Slug        string         `json:"slug"`
	Title       string         `json:"title"`
	Company     string         `json:"company"`
	CompanySlug string         `json:"company_slug,omitempty"`
	Location    string         `json:"location,omitempty"`
	Regions     []string       `json:"regions,omitempty"`
	Countries   []string       `json:"countries,omitempty"`
	WorkMode    string         `json:"work_mode,omitempty"`
	Source      string         `json:"source,omitempty"`
	Skills      []string       `json:"skills,omitempty"`
	PostedAt    string         `json:"posted_at,omitempty"`
	CreatedAt   string         `json:"created_at,omitempty"`
	ClosedAt    string         `json:"closed_at,omitempty"`
	Enrichment  map[string]any `json:"enrichment,omitempty"`
	Reality     map[string]any `json:"reality,omitempty"`
	URL         string         `json:"url,omitempty"`
}

type searchResultJSON struct {
	Total  int              `json:"total"`
	Offset int              `json:"offset"`
	Limit  int              `json:"limit"`
	Jobs   []jobSummaryJSON `json:"jobs"`
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseEnum[T any](s string, parse func(string) (T, error)) (T, error) {
	item, err := parse(s)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("%q: %w", s, err)
	}
	return item, nil
}

func parseEnumCSV[T any](s string, parse func(string) (T, error)) ([]T, error) {
	raw := splitCSV(s)
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]T, 0, len(raw))
	for _, p := range raw {
		item, err := parse(p)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", p, err)
		}
		out = append(out, item)
	}
	return out, nil
}

func searchParams(f searchFlags) (freehire.AgentSearchJobsParams, error) {
	offset := (f.page - 1) * f.limit
	if offset+f.limit > freehire.MaxResultWindow {
		return freehire.AgentSearchJobsParams{}, fmt.Errorf("page is past the %d-result window (offset %d + limit %d)", freehire.MaxResultWindow, offset, f.limit)
	}
	seniority, err := parseEnumCSV(f.seniority, func(s string) (freehire.AgentSearchJobsSeniorityItem, error) {
		item := freehire.AgentSearchJobsSeniorityItem(s)
		return item, item.Validate()
	})
	if err != nil {
		return freehire.AgentSearchJobsParams{}, fmt.Errorf("invalid --seniority: %w", err)
	}
	workMode, err := parseEnumCSV(f.workMode, func(s string) (freehire.AgentSearchJobsWorkModeItem, error) {
		item := freehire.AgentSearchJobsWorkModeItem(s)
		return item, item.Validate()
	})
	if err != nil {
		return freehire.AgentSearchJobsParams{}, fmt.Errorf("invalid --work-mode: %w", err)
	}
	regions, err := parseEnumCSV(f.region, func(s string) (freehire.AgentSearchJobsRegionsItem, error) {
		item := freehire.AgentSearchJobsRegionsItem(s)
		return item, item.Validate()
	})
	if err != nil {
		return freehire.AgentSearchJobsParams{}, fmt.Errorf("invalid --region: %w", err)
	}
	params := freehire.AgentSearchJobsParams{
		Limit:  freehire.NewOptInt(f.limit),
		Offset: freehire.NewOptInt(offset),
		// Every row carries a description whatever we ask for, so take
		// the cheaper plain text over the stored markup.
		DescriptionFormat: freehire.NewOptAgentSearchJobsDescriptionFormat(freehire.AgentSearchJobsDescriptionFormatText),
		Skills:            splitCSV(f.skills),
		Seniority:         seniority,
		WorkMode:          workMode,
		Regions:           regions,
		Countries:         splitCSV(f.country),
		Source:            splitCSV(f.source),
		Category:          splitCSV(f.category),
		CompanySlug:       splitCSV(f.company),
	}
	if f.query != "" {
		params.Q = freehire.NewOptString(f.query)
	}
	if f.salaryMin > 0 {
		params.SalaryMin = freehire.NewOptInt(f.salaryMin)
	}
	if f.salaryMax > 0 {
		params.SalaryMax = freehire.NewOptInt(f.salaryMax)
	}
	if f.semantic >= 0 {
		params.SemanticRatio = freehire.NewOptFloat64(f.semantic)
	}
	if f.sortField != "" {
		field, err := parseEnum(f.sortField, func(s string) (freehire.AgentSearchJobsSort, error) {
			item := freehire.AgentSearchJobsSort(s)
			return item, item.Validate()
		})
		if err != nil {
			return freehire.AgentSearchJobsParams{}, fmt.Errorf("invalid --sort: %w", err)
		}
		params.Sort = freehire.NewOptAgentSearchJobsSort(field)
	}
	if f.order != "" {
		order, err := parseEnum(f.order, func(s string) (freehire.AgentSearchJobsOrder, error) {
			item := freehire.AgentSearchJobsOrder(s)
			return item, item.Validate()
		})
		if err != nil {
			return freehire.AgentSearchJobsParams{}, fmt.Errorf("invalid --order: %w", err)
		}
		params.Order = freehire.NewOptAgentSearchJobsOrder(order)
	}
	return params, nil
}

func facetsParams(f searchFlags) (freehire.GetJobFacetsParams, error) {
	seniority, err := parseEnumCSV(f.seniority, func(s string) (freehire.AgentSearchJobsSeniorityItem, error) {
		item := freehire.AgentSearchJobsSeniorityItem(s)
		return item, item.Validate()
	})
	if err != nil {
		return freehire.GetJobFacetsParams{}, fmt.Errorf("invalid --seniority: %w", err)
	}
	workMode, err := parseEnumCSV(f.workMode, func(s string) (freehire.AgentSearchJobsWorkModeItem, error) {
		item := freehire.AgentSearchJobsWorkModeItem(s)
		return item, item.Validate()
	})
	if err != nil {
		return freehire.GetJobFacetsParams{}, fmt.Errorf("invalid --work-mode: %w", err)
	}
	regions, err := parseEnumCSV(f.region, func(s string) (freehire.AgentSearchJobsRegionsItem, error) {
		item := freehire.AgentSearchJobsRegionsItem(s)
		return item, item.Validate()
	})
	if err != nil {
		return freehire.GetJobFacetsParams{}, fmt.Errorf("invalid --region: %w", err)
	}
	params := freehire.GetJobFacetsParams{
		Skills:      splitCSV(f.skills),
		Seniority:   slugsOf(seniority),
		WorkMode:    slugsOf(workMode),
		Regions:     slugsOf(regions),
		Countries:   splitCSV(f.country),
		Source:      splitCSV(f.source),
		Category:    splitCSV(f.category),
		CompanySlug: splitCSV(f.company),
	}
	if f.query != "" {
		params.Q = freehire.NewOptString(f.query)
	}
	if f.salaryMin > 0 {
		params.SalaryMin = freehire.NewOptInt(f.salaryMin)
	}
	if f.salaryMax > 0 {
		params.SalaryMax = freehire.NewOptInt(f.salaryMax)
	}
	return params, nil
}

func slugsOf[T ~string](items []T) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = string(item)
	}
	return out
}

type optTime interface {
	Get() (time.Time, bool)
}

func optDate(v optTime) string {
	if t, ok := v.Get(); ok {
		return t.UTC().Format(time.DateOnly)
	}
	return ""
}

// rawObject unwraps the spec's free-form objects, which ogen leaves as
// raw JSON because their keys vary by posting.
func rawObject(m map[string]jx.Raw) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, raw := range m {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			continue
		}
		out[k] = v
	}
	return out
}

func summarize(j freehire.Job) jobSummaryJSON {
	return jobSummaryJSON{
		Slug:        j.PublicSlug,
		Title:       j.Title,
		Company:     j.Company,
		CompanySlug: j.CompanySlug.Or(""),
		Location:    j.Location.Or(""),
		WorkMode:    j.WorkMode.Or(""),
		Source:      j.Source.Or(""),
		Skills:      j.Skills,
		Regions:     j.Regions,
		Countries:   j.Countries,
		PostedAt:    optDate(j.PostedAt),
		CreatedAt:   optDate(j.CreatedAt),
		ClosedAt:    optDate(j.ClosedAt),
		Enrichment:  rawObject(j.Enrichment.Or(nil)),
		Reality:     rawObject(j.Reality.Or(nil)),
		URL:         jobURL(j),
	}
}

func jobURL(j freehire.Job) string {
	if u, ok := j.URL.Get(); ok {
		return u.String()
	}
	return ""
}

func runSearch(ctx context.Context, f searchFlags) error {
	params, err := searchParams(f)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := freehire.NewClient(f.baseURL)
	if err != nil {
		return err
	}
	res, err := client.AgentSearchJobs(ctx, params)
	if err != nil {
		return err
	}

	jobs := make([]jobSummaryJSON, len(res.Data))
	for i, j := range res.Data {
		jobs[i] = summarize(j)
	}
	meta := res.Meta.Or(freehire.PaginationMeta{})

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(searchResultJSON{
			Total:  meta.Total.Or(0),
			Offset: meta.Offset.Or(0),
			Limit:  meta.Limit.Or(f.limit),
			Jobs:   jobs,
		})
	}

	fmt.Printf("freehire Jobs (total: %d, offset %d, limit %d)\n\n", meta.Total.Or(0), meta.Offset.Or(0), meta.Limit.Or(f.limit))
	for i, s := range jobs {
		fmt.Printf("%d. %s\n", i+1, s.Title)
		if s.CompanySlug != "" {
			fmt.Printf("Company: %s (%s)\n", s.Company, s.CompanySlug)
		} else {
			fmt.Printf("Company: %s\n", s.Company)
		}
		if s.Location != "" {
			if s.WorkMode != "" {
				fmt.Printf("Location: %s (%s)\n", s.Location, s.WorkMode)
			} else {
				fmt.Printf("Location: %s\n", s.Location)
			}
		} else if s.WorkMode != "" {
			fmt.Printf("Work mode: %s\n", s.WorkMode)
		}
		if s.Source != "" {
			fmt.Printf("Source: %s\n", s.Source)
		}
		if s.PostedAt != "" {
			fmt.Printf("Posted: %s\n", s.PostedAt)
		}
		fmt.Printf("Slug: %s\n", s.Slug)
		if s.URL != "" {
			fmt.Printf("URL: %s\n", s.URL)
		}
		fmt.Println()
	}
	return nil
}

func runFacets(ctx context.Context, f searchFlags) error {
	params, err := facetsParams(f)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := freehire.NewClient(f.baseURL)
	if err != nil {
		return err
	}
	res, err := client.GetJobFacets(ctx, params)
	if err != nil {
		return err
	}

	facets := make(map[string]map[string]int, len(res.Data.Facets))
	for name, vals := range res.Data.Facets {
		facets[name] = map[string]int(vals)
	}
	out := struct {
		Total  int                       `json:"total"`
		Facets map[string]map[string]int `json:"facets"`
	}{Total: res.Data.Total, Facets: facets}

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Printf("freehire facets (total: %d)\n\n", out.Total)
	names := make([]string, 0, len(out.Facets))
	for name := range out.Facets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		vals := out.Facets[name]
		fmt.Printf("%s (%d)\n", name, len(vals))
		if len(vals) > 20 {
			fmt.Printf("  %d values; use --format json\n", len(vals))
			continue
		}
		keys := make([]string, 0, len(vals))
		for k := range vals {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s: %d\n", k, vals[k])
		}
	}
	return nil
}

type companiesFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	query   string
	page    int
	limit   int
}

type companyJSON struct {
	Slug     string         `json:"slug"`
	Name     string         `json:"name"`
	JobCount int            `json:"job_count"`
	Details  map[string]any `json:"details,omitempty"`
}

func runCompanies(ctx context.Context, f companiesFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := freehire.NewClient(f.baseURL)
	if err != nil {
		return err
	}
	res, err := client.SearchCompanies(ctx, freehire.SearchCompaniesParams{
		Q:      freehire.NewOptString(f.query),
		Limit:  freehire.NewOptInt(f.limit),
		Offset: freehire.NewOptInt((f.page - 1) * f.limit),
	})
	if err != nil {
		return err
	}

	companies := make([]companyJSON, 0, len(res.Data))
	for _, c := range res.Data {
		companies = append(companies, companyJSON{
			Slug:     c.Slug,
			Name:     c.Name,
			JobCount: c.JobCount.Or(0),
			Details:  rawObject(c.AdditionalProps),
		})
	}
	meta := res.Meta.Or(freehire.PaginationMeta{})

	if f.format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Total     int           `json:"total"`
			Companies []companyJSON `json:"companies"`
		}{meta.Total.Or(0), companies})
	}

	fmt.Printf("freehire companies matching %q (total: %d)\n\n", f.query, meta.Total.Or(0))
	for i, c := range companies {
		fmt.Printf("%d. %s\n", i+1, c.Name)
		fmt.Printf("Slug: %s\n", c.Slug)
		fmt.Printf("Open jobs: %d\n", c.JobCount)
		if tagline, ok := c.Details["tagline"].(string); ok && tagline != "" {
			fmt.Printf("Tagline: %s\n", tagline)
		}
		fmt.Println()
	}
	return nil
}

type detailFlags struct {
	baseURL string
	timeout time.Duration
	format  string
	slug    string
}

type detailJSON struct {
	jobSummaryJSON
	Description string `json:"description,omitempty"`
}

func renderHTML(html string) string {
	text, err := html2text.FromString(html, html2text.Options{})
	if err != nil {
		return html
	}
	return text
}

func runDetail(ctx context.Context, f detailFlags) error {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	client, err := freehire.NewClient(f.baseURL)
	if err != nil {
		return err
	}
	res, err := client.GetJob(ctx, freehire.GetJobParams{Slug: f.slug})
	if err != nil {
		return fmt.Errorf("job %q: %w", f.slug, err)
	}
	return printDetail(res.Data, f.format)
}

func printDetail(j freehire.Job, format string) error {
	d := detailJSON{
		jobSummaryJSON: summarize(j),
		Description:    renderHTML(j.Description.Or("")),
	}
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}
	fmt.Printf("%s\n", d.Title)
	fmt.Printf("Company: %s\n", d.Company)
	if d.Location != "" {
		fmt.Printf("Location: %s\n", d.Location)
	}
	if d.WorkMode != "" {
		fmt.Printf("Work mode: %s\n", d.WorkMode)
	}
	if d.Source != "" {
		fmt.Printf("Source: %s\n", d.Source)
	}
	if d.PostedAt != "" {
		fmt.Printf("Posted: %s\n", d.PostedAt)
	}
	fmt.Printf("Slug: %s\n", d.Slug)
	if d.URL != "" {
		fmt.Printf("URL: %s\n", d.URL)
	}
	if d.Description != "" {
		fmt.Printf("\n%s\n", d.Description)
	}
	return nil
}
