// Command freehire is a debug CLI for freehire.me's public jobs API.
//
//	go run ./cmd/freehire search --q golang --work-mode remote --reality fresh
//	go run ./cmd/freehire search --company-slug stripe --seniority staff
//	go run ./cmd/freehire search --cities London --posted-within-days 7 --sort posted_at
//	go run ./cmd/freehire search --skills go,kubernetes --skills-mode and
//	go run ./cmd/freehire search --countries de --source-exclude adzuna
//	go run ./cmd/freehire facets --q golang --facets skills,countries
//	go run ./cmd/freehire facets --seniority senior --disjunctive
//	go run ./cmd/freehire companies --q adria
//	go run ./cmd/freehire companies --collections yc --yc-flags top_company
//	go run ./cmd/freehire company --company-slug stripe
//	go run ./cmd/freehire cities --q lond --country gb
//	go run ./cmd/freehire detail --slug staff-software-engineer-link-stripe-32iy7vks
//
// Every filter runs server-side, and every flag maps to one upstream
// query parameter of the same name. Paging is --limit/--offset, as
// upstream declares it, rather than a page number.
//
// Two upstream behaviours make a wrong filter hard to spot. A wrong
// VALUE matches nothing and is not an error, so a zero-result search
// usually means a bad slug: resolve one with the facets, companies, or
// cities subcommand. A wrong PARAMETER NAME is dropped and the whole
// catalogue comes back, which this CLI reports as an error rather than
// printing as a result. See internal/provider/freehire/openapi.yaml for
// the filter grammar and the geography OR-group.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/freehire"
)

// apiBaseURL is freehire's public API origin — the single production
// server in the provider's openapi.yaml.
const apiBaseURL = "https://freehire.me/api/v1"

// unsetInt marks an integer filter the user did not give. Zero is a
// legitimate value for salary and experience bounds, so it cannot double
// as the sentinel.
const unsetInt = -1

func main() {
	os.Exit(run())
}

// filterFlags are the filters /jobs/search and /jobs/facets share. Every
// repeatable upstream facet is a comma-separated list here; enum-valued
// ones are still validated against the generated types before the
// request goes out.
type filterFlags struct {
	q                  *string
	regions            *string
	countries          *string
	cities             *string
	workMode           *string
	category           *string
	role               *string
	seniority          *string
	skills             *string
	skillsMode         *string
	isTech             *string
	aiArchetype        *string
	collections        *string
	reality            *string
	source             *string
	companySlug        *string
	employmentType     *string
	relocation         *string
	englishLevel       *string
	educationLevel     *string
	postingLanguage    *string
	domains            *string
	companyType        *string
	companySize        *string
	salaryCurrency     *string
	salaryPeriod       *string
	visaSponsorship    *string
	salaryMin          *int
	salaryMax          *int
	experienceYearsMin *int
	postedWithinDays   *int
}

func registerFilterFlags(fs *ff.FlagSet) *filterFlags {
	return &filterFlags{
		q:                  fs.StringLong("q", "", "full-text query over title, company, and description"),
		regions:            fs.StringLong("regions", "", "comma-separated macro-regions: global, north_america, latam, eu, uk, mena, africa, apac, cis, none. ORs with --countries and --cities, so a second geography widens the search"),
		countries:          fs.StringLong("countries", "", "comma-separated lowercase ISO 3166-1 alpha-2 codes, e.g. gb,de. ORs with --regions and --cities"),
		cities:             fs.StringLong("cities", "", "comma-separated canonical city names, e.g. London. Resolve with the cities subcommand; ORs with --regions and --countries"),
		workMode:           fs.StringLong("work-mode", "", "comma-separated work formats: remote, hybrid, onsite"),
		category:           fs.StringLong("category", "", "comma-separated role-category slugs, e.g. backend,devops"),
		role:               fs.StringLong("role", "", "comma-separated fine-grained role slugs, e.g. backend,android_developer"),
		seniority:          fs.StringLong("seniority", "", "comma-separated seniority levels: intern, junior, middle, senior, lead, staff, principal, c_level"),
		skills:             fs.StringLong("skills", "", "comma-separated canonical skill slugs, e.g. go,rust"),
		skillsMode:         fs.StringEnumLong("skills-mode", "set to and to require every listed skill instead of any", "", "and"),
		isTech:             fs.StringLong("is-tech", "", "comma-separated: tech, non_tech"),
		aiArchetype:        fs.StringLong("ai-archetype", "", "comma-separated AI archetypes: rag_app_builder, agent_builder, cloud_ml_platform_engineer, ml_trainer_researcher, fullstack_ai_engineer, devops_infra_engineer"),
		collections:        fs.StringLong("collections", "", "comma-separated curated collection slugs of the company, e.g. yc,unicorn"),
		reality:            fs.StringLong("reality", "", "comma-separated posting-reality classes: fresh, stale, likely-evergreen"),
		source:             fs.StringLong("source", "", "comma-separated origin slugs, e.g. greenhouse,lever"),
		companySlug:        fs.StringLong("company-slug", "", "comma-separated catalogue company slugs, e.g. stripe"),
		employmentType:     fs.StringLong("employment-type", "", "comma-separated: full_time, part_time, contract, internship, fellowship"),
		relocation:         fs.StringLong("relocation", "", "comma-separated: not_supported, supported, required"),
		englishLevel:       fs.StringLong("english-level", "", "comma-separated: none, a1, a2, b1, b2, c1, c2, native"),
		educationLevel:     fs.StringLong("education-level", "", "comma-separated: none, bachelor, master, phd"),
		postingLanguage:    fs.StringLong("posting-language", "", "comma-separated ISO 639-1 codes, e.g. en,de"),
		domains:            fs.StringLong("domains", "", "comma-separated business domains, e.g. fintech,devtools"),
		companyType:        fs.StringLong("company-type", "", "comma-separated: product, startup, outsource, outstaff, agency, inhouse, government"),
		companySize:        fs.StringLong("company-size", "", "comma-separated: 1-10, 11-50, 51-200, 201-500, 501-1000, 1000+"),
		salaryCurrency:     fs.StringLong("salary-currency", "", "comma-separated ISO 4217 codes, e.g. USD,EUR"),
		salaryPeriod:       fs.StringLong("salary-period", "", "comma-separated: year, month, day, hour"),
		visaSponsorship:    fs.StringEnumLong("visa-sponsorship", "filter on the posting's STATED sponsorship; postings that say nothing match neither setting", "", "true", "false"),
		salaryMin:          fs.IntLong("salary-min", unsetInt, "lower bound on the posting's minimum salary, in its own currency and period; negative leaves it unset"),
		salaryMax:          fs.IntLong("salary-max", unsetInt, "upper bound on the posting's maximum salary; negative leaves it unset"),
		experienceYearsMin: fs.IntLong("experience-years-min", unsetInt, "lower bound on years of experience asked for; negative leaves it unset"),
		postedWithinDays:   fs.IntLong("posted-within-days", unsetInt, "restrict to postings published within the last N days; negative leaves it unset"),
	}
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
		Usage: "freehire [FLAGS] <search|facets|companies|company|cities|detail> [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	searchFilters := registerFilterFlags(searchFS)
	var (
		sortField          = searchFS.StringEnumLong("sort", "sort field; omit for relevance", "", "created_at", "posted_at", "salary_min", "salary_max")
		order              = searchFS.StringEnumLong("order", "sort direction, with --sort", "", "asc", "desc")
		limit              = searchFS.IntLong("limit", unsetInt, "page size, 1-100; unset lets upstream default to 10")
		offset             = searchFS.IntLong("offset", unsetInt, "rows to skip; offset + limit may not exceed 10000")
		regionsExclude     = searchFS.StringLong("regions-exclude", "", "comma-separated regions to exclude; excludes AND together")
		countriesExclude   = searchFS.StringLong("countries-exclude", "", "comma-separated countries to exclude")
		workModeExclude    = searchFS.StringLong("work-mode-exclude", "", "comma-separated work formats to exclude, e.g. onsite")
		skillsExclude      = searchFS.StringLong("skills-exclude", "", "comma-separated skill slugs to exclude")
		sourceExclude      = searchFS.StringLong("source-exclude", "", "comma-separated source slugs to exclude")
		companySlugExclude = searchFS.StringLong("company-slug-exclude", "", "comma-separated company slugs to exclude")
	)
	searchCmd := &ff.Command{
		Name: "search",
		Usage: "freehire search [--q TEXT] [FILTERS] [--sort FIELD] [--order asc|desc] [--limit N] [--offset N] [--format text|json]\n" +
			"  filters: --regions --countries --cities --work-mode --category --role --seniority --skills --skills-mode\n" +
			"           --is-tech --ai-archetype --collections --reality --source --company-slug --employment-type\n" +
			"           --relocation --english-level --education-level --posting-language --domains --company-type\n" +
			"           --company-size --salary-currency --salary-period --visa-sponsorship --salary-min --salary-max\n" +
			"           --experience-years-min --posted-within-days\n" +
			"  excludes: --regions-exclude --countries-exclude --work-mode-exclude --skills-exclude --source-exclude\n" +
			"            --company-slug-exclude",
		ShortHelp: "search IT jobs across freehire's catalogue (GET /jobs/search)",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			if *limit != unsetInt && (*limit < 1 || *limit > 100) {
				return fmt.Errorf("--limit must be between 1 and 100, got %d", *limit)
			}
			return runSearch(ctx, clientFlags{*baseURL, *timeout, *format}, searchOpts{
				filters:            searchFilters,
				sortField:          *sortField,
				order:              *order,
				limit:              *limit,
				offset:             *offset,
				regionsExclude:     *regionsExclude,
				countriesExclude:   *countriesExclude,
				workModeExclude:    *workModeExclude,
				skillsExclude:      *skillsExclude,
				sourceExclude:      *sourceExclude,
				companySlugExclude: *companySlugExclude,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	facetsFS := ff.NewFlagSet("facets").SetParent(rootFlags)
	facetsFilters := registerFilterFlags(facetsFS)
	var (
		facets      = facetsFS.StringLong("facets", "", "comma-separated facet names to count, e.g. skills,seniority; omit for all of them")
		disjunctive = facetsFS.BoolLong("disjunctive", "count each facet under the filter MINUS its own selection, so a selected facet still shows its other values; cannot be combined with --facets")
	)
	facetsCmd := &ff.Command{
		Name: "facets",
		Usage: "freehire facets [--q TEXT] [FILTERS] [--facets NAMES] [--disjunctive] [--format text|json]\n" +
			"  filters: the same set search takes, minus sort, paging, and the excludes",
		ShortHelp: "print live filter vocabulary and counts (GET /jobs/facets)",
		Flags:     facetsFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("facets takes no positional arguments, got %v (did you forget a flag name?)", args)
			}
			return runFacets(ctx, clientFlags{*baseURL, *timeout, *format}, facetsFilters, *facets, *disjunctive)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, facetsCmd)

	companiesFS := ff.NewFlagSet("companies").SetParent(rootFlags)
	var (
		companiesQ             = companiesFS.StringLong("q", "", "company name to match, case-insensitive")
		companiesLimit         = companiesFS.IntLong("limit", unsetInt, "page size, 1-100; unset lets upstream default to 10")
		companiesOffset        = companiesFS.IntLong("offset", unsetInt, "companies to skip")
		companiesSort          = companiesFS.StringEnumLong("sort", "set to rating to order by average feedback rating", "", "rating")
		companiesCollections   = companiesFS.StringLong("collections", "", "comma-separated curated collection slugs, e.g. yc,unicorn")
		companiesRegions       = companiesFS.StringLong("regions", "", "comma-separated regions the company posts roles in")
		companiesCountries     = companiesFS.StringLong("countries", "", "comma-separated countries the company posts roles in")
		companiesRemoteRegions = companiesFS.StringLong("remote-regions", "", "comma-separated regions the company hires REMOTELY in")
		companiesIndustries    = companiesFS.StringLong("industries", "", "comma-separated curated industry slugs, e.g. fintech,developer-tools")
		companiesDomains       = companiesFS.StringLong("domains", "", "comma-separated job-derived domain slugs")
		companiesCompanyType   = companiesFS.StringLong("company-type", "", "comma-separated: product, startup, outsource, outstaff, agency, inhouse, government")
		companiesCompanySize   = companiesFS.StringLong("company-size", "", "comma-separated: 1-10, 11-50, 51-200, 201-500, 501-1000, 1000+")
		companiesMaturity      = companiesFS.StringLong("maturity", "", "comma-separated curated maturity stages, e.g. enterprise")
		companiesYcBatch       = companiesFS.StringLong("yc-batch", "", "comma-separated YC batches written out in full, e.g. 'Summer 2009'; the S09/W21 form matches nothing")
		companiesYcStatus      = companiesFS.StringLong("yc-status", "", "comma-separated YC statuses, e.g. Active")
		companiesYcStage       = companiesFS.StringLong("yc-stage", "", "comma-separated YC funding stages, e.g. Growth")
		companiesYcFlags       = companiesFS.StringLong("yc-flags", "", "comma-separated YC directory flags, e.g. top_company,hiring")
	)
	companiesCmd := &ff.Command{
		Name: "companies",
		Usage: "freehire companies [--q NAME] [FILTERS] [--sort rating] [--limit N] [--offset N] [--format text|json]\n" +
			"  filters: --collections --regions --countries --remote-regions --industries --domains --company-type\n" +
			"           --company-size --maturity --yc-batch --yc-status --yc-stage --yc-flags",
		ShortHelp: "search companies with open roles (GET /companies)",
		Flags:     companiesFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("companies takes no positional arguments, got %v (did you mean --q %q?)", args, args[0])
			}
			if *companiesLimit != unsetInt && (*companiesLimit < 1 || *companiesLimit > 100) {
				return fmt.Errorf("--limit must be between 1 and 100, got %d", *companiesLimit)
			}
			return runCompanies(ctx, clientFlags{*baseURL, *timeout, *format}, companiesOpts{
				q:             *companiesQ,
				limit:         *companiesLimit,
				offset:        *companiesOffset,
				sortField:     *companiesSort,
				collections:   *companiesCollections,
				regions:       *companiesRegions,
				countries:     *companiesCountries,
				remoteRegions: *companiesRemoteRegions,
				industries:    *companiesIndustries,
				domains:       *companiesDomains,
				companyType:   *companiesCompanyType,
				companySize:   *companiesCompanySize,
				maturity:      *companiesMaturity,
				ycBatch:       *companiesYcBatch,
				ycStatus:      *companiesYcStatus,
				ycStage:       *companiesYcStage,
				ycFlags:       *companiesYcFlags,
			})
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companiesCmd)

	companyFS := ff.NewFlagSet("company").SetParent(rootFlags)
	var (
		companySlug   = companyFS.StringLong("company-slug", "", "catalogue company slug, e.g. stripe")
		companyLimit  = companyFS.IntLong("limit", unsetInt, "open jobs to return alongside the profile; unset lets upstream default to 20")
		companyOffset = companyFS.IntLong("offset", unsetInt, "jobs to skip")
	)
	companyCmd := &ff.Command{
		Name:      "company",
		Usage:     "freehire company --company-slug SLUG [--limit N] [--offset N] [--format text|json]",
		ShortHelp: "print one company's profile and a page of its open jobs (GET /companies/{slug})",
		Flags:     companyFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("company takes no positional arguments, got %v (did you mean --company-slug %q?)", args, args[0])
			}
			if *companySlug == "" {
				return errors.New("company requires --company-slug; resolve one with the companies subcommand")
			}
			return runCompany(ctx, clientFlags{*baseURL, *timeout, *format}, *companySlug, *companyLimit, *companyOffset)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, companyCmd)

	citiesFS := ff.NewFlagSet("cities").SetParent(rootFlags)
	var (
		citiesQ       = citiesFS.StringLong("q", "", "city name prefix or substring, case-insensitive")
		citiesCountry = citiesFS.StringLong("country", "", "restrict matches to one lowercase ISO 3166-1 alpha-2 code, e.g. gb")
	)
	citiesCmd := &ff.Command{
		Name:      "cities",
		Usage:     "freehire cities [--q NAME] [--country CC] [--format text|json]",
		ShortHelp: "resolve a city name to the value the --cities filter takes (GET /geo/cities)",
		Flags:     citiesFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("cities takes no positional arguments, got %v (did you mean --q %q?)", args, args[0])
			}
			return runCities(ctx, clientFlags{*baseURL, *timeout, *format}, *citiesQ, *citiesCountry)
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, citiesCmd)

	detailFS := ff.NewFlagSet("detail").SetParent(rootFlags)
	slug := detailFS.StringLong("slug", "", "job public_slug from a search result")
	detailCmd := &ff.Command{
		Name:      "detail",
		Usage:     "freehire detail --slug JOB-SLUG [--format text|json]",
		ShortHelp: "print one job in full by its public_slug (GET /jobs/{slug})",
		Flags:     detailFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("detail takes no positional arguments, got %v (did you mean --slug %q?)", args, args[0])
			}
			if *slug == "" {
				return errors.New("detail requires --slug")
			}
			return runDetail(ctx, clientFlags{*baseURL, *timeout, *format}, *slug)
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
		fmt.Fprintln(os.Stderr, "err: a subcommand (search, facets, companies, company, cities, or detail) is required")
		return 1
	}
	if err := rootCmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	return 0
}

type clientFlags struct {
	baseURL string
	timeout time.Duration
	format  string
}

type searchOpts struct {
	filters            *filterFlags
	sortField          string
	order              string
	limit              int
	offset             int
	regionsExclude     string
	countriesExclude   string
	workModeExclude    string
	skillsExclude      string
	sourceExclude      string
	companySlugExclude string
}

type companiesOpts struct {
	q             string
	limit         int
	offset        int
	sortField     string
	collections   string
	regions       string
	countries     string
	remoteRegions string
	industries    string
	domains       string
	companyType   string
	companySize   string
	maturity      string
	ycBatch       string
	ycStatus      string
	ycStage       string
	ycFlags       string
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// enumSlug is the shape ogen gives every enum in the spec: a string type
// that validates itself against the spec's value list.
type enumSlug interface {
	~string
	Validate() error
}

func enumOne[T enumSlug](s, flag string) (T, error) {
	item := T(s)
	if err := item.Validate(); err != nil {
		return item, fmt.Errorf("invalid --%s %q: %w", flag, s, err)
	}
	return item, nil
}

func enumCSV[T enumSlug](s, flag string) ([]T, error) {
	raw := splitCSV(s)
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]T, 0, len(raw))
	for _, p := range raw {
		item, err := enumOne[T](p, flag)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func optInt(v int) freehire.OptInt {
	if v < 0 {
		return freehire.OptInt{}
	}
	return freehire.NewOptInt(v)
}

func optString(v string) freehire.OptString {
	if v == "" {
		return freehire.OptString{}
	}
	return freehire.NewOptString(v)
}

// optTristateBool reads the "", "true", "false" flag domain. An unset
// flag must stay absent: upstream treats visa_sponsorship=false as a
// stated value, not as "unknown".
func optTristateBool(v string) freehire.OptBool {
	switch v {
	case "true":
		return freehire.NewOptBool(true)
	case "false":
		return freehire.NewOptBool(false)
	default:
		return freehire.OptBool{}
	}
}

// enumFilters holds the shared filters upstream types as enums. Both
// /jobs/search and /jobs/facets name the same component for each, so one
// conversion serves both.
type enumFilters struct {
	regions        []freehire.RegionsItem
	workMode       []freehire.WorkModeItem
	seniority      []freehire.SeniorityItem
	isTech         []freehire.IsTechItem
	aiArchetype    []freehire.AIArchetypeItem
	reality        []freehire.RealityItem
	employmentType []freehire.EmploymentTypeItem
	relocation     []freehire.RelocationItem
	englishLevel   []freehire.EnglishLevelItem
	educationLevel []freehire.EducationLevelItem
	domains        []freehire.DomainsItem
	companyType    []freehire.CompanyTypeItem
	companySize    []freehire.CompanySizeItem
	salaryPeriod   []freehire.SalaryPeriodItem
	skillsMode     freehire.OptSkillsMode
}

func parseFilterEnums(f *filterFlags) (enumFilters, error) {
	var e enumFilters
	var err error
	if e.regions, err = enumCSV[freehire.RegionsItem](*f.regions, "regions"); err != nil {
		return e, err
	}
	if e.workMode, err = enumCSV[freehire.WorkModeItem](*f.workMode, "work-mode"); err != nil {
		return e, err
	}
	if e.seniority, err = enumCSV[freehire.SeniorityItem](*f.seniority, "seniority"); err != nil {
		return e, err
	}
	if e.isTech, err = enumCSV[freehire.IsTechItem](*f.isTech, "is-tech"); err != nil {
		return e, err
	}
	if e.aiArchetype, err = enumCSV[freehire.AIArchetypeItem](*f.aiArchetype, "ai-archetype"); err != nil {
		return e, err
	}
	if e.reality, err = enumCSV[freehire.RealityItem](*f.reality, "reality"); err != nil {
		return e, err
	}
	if e.employmentType, err = enumCSV[freehire.EmploymentTypeItem](*f.employmentType, "employment-type"); err != nil {
		return e, err
	}
	if e.relocation, err = enumCSV[freehire.RelocationItem](*f.relocation, "relocation"); err != nil {
		return e, err
	}
	if e.englishLevel, err = enumCSV[freehire.EnglishLevelItem](*f.englishLevel, "english-level"); err != nil {
		return e, err
	}
	if e.educationLevel, err = enumCSV[freehire.EducationLevelItem](*f.educationLevel, "education-level"); err != nil {
		return e, err
	}
	if e.domains, err = enumCSV[freehire.DomainsItem](*f.domains, "domains"); err != nil {
		return e, err
	}
	if e.companyType, err = enumCSV[freehire.CompanyTypeItem](*f.companyType, "company-type"); err != nil {
		return e, err
	}
	if e.companySize, err = enumCSV[freehire.CompanySizeItem](*f.companySize, "company-size"); err != nil {
		return e, err
	}
	if e.salaryPeriod, err = enumCSV[freehire.SalaryPeriodItem](*f.salaryPeriod, "salary-period"); err != nil {
		return e, err
	}
	if *f.skillsMode != "" {
		mode, err := enumOne[freehire.SkillsMode](*f.skillsMode, "skills-mode")
		if err != nil {
			return e, err
		}
		e.skillsMode = freehire.NewOptSkillsMode(mode)
	}
	return e, nil
}

func searchParams(o searchOpts) (freehire.SearchJobsParams, error) {
	f := o.filters
	e, err := parseFilterEnums(f)
	if err != nil {
		return freehire.SearchJobsParams{}, err
	}
	params := freehire.SearchJobsParams{
		Q:                  optString(*f.q),
		Limit:              optInt(o.limit),
		Offset:             optInt(o.offset),
		Regions:            e.regions,
		Countries:          splitCSV(*f.countries),
		Cities:             splitCSV(*f.cities),
		WorkMode:           e.workMode,
		Category:           splitCSV(*f.category),
		Role:               splitCSV(*f.role),
		Seniority:          e.seniority,
		Skills:             splitCSV(*f.skills),
		SkillsMode:         e.skillsMode,
		IsTech:             e.isTech,
		AiArchetype:        e.aiArchetype,
		Collections:        splitCSV(*f.collections),
		Reality:            e.reality,
		Source:             splitCSV(*f.source),
		CompanySlug:        splitCSV(*f.companySlug),
		EmploymentType:     e.employmentType,
		Relocation:         e.relocation,
		EnglishLevel:       e.englishLevel,
		EducationLevel:     e.educationLevel,
		PostingLanguage:    splitCSV(*f.postingLanguage),
		Domains:            e.domains,
		CompanyType:        e.companyType,
		CompanySize:        e.companySize,
		SalaryCurrency:     splitCSV(*f.salaryCurrency),
		SalaryPeriod:       e.salaryPeriod,
		VisaSponsorship:    optTristateBool(*f.visaSponsorship),
		SalaryMin:          optInt(*f.salaryMin),
		SalaryMax:          optInt(*f.salaryMax),
		ExperienceYearsMin: optInt(*f.experienceYearsMin),
		PostedWithinDays:   optInt(*f.postedWithinDays),
		RegionsExclude:     splitCSV(o.regionsExclude),
		CountriesExclude:   splitCSV(o.countriesExclude),
		WorkModeExclude:    splitCSV(o.workModeExclude),
		SkillsExclude:      splitCSV(o.skillsExclude),
		SourceExclude:      splitCSV(o.sourceExclude),
		CompanySlugExclude: splitCSV(o.companySlugExclude),
	}
	if o.sortField != "" {
		field, err := enumOne[freehire.Sort](o.sortField, "sort")
		if err != nil {
			return freehire.SearchJobsParams{}, err
		}
		params.Sort = freehire.NewOptSort(field)
	}
	if o.order != "" {
		order, err := enumOne[freehire.Order](o.order, "order")
		if err != nil {
			return freehire.SearchJobsParams{}, err
		}
		params.Order = freehire.NewOptOrder(order)
	}
	return params, nil
}

func facetsParams(f *filterFlags, facets string, disjunctive bool) (freehire.GetJobFacetsParams, error) {
	e, err := parseFilterEnums(f)
	if err != nil {
		return freehire.GetJobFacetsParams{}, err
	}
	return freehire.GetJobFacetsParams{
		Q:                  optString(*f.q),
		Facets:             optString(facets),
		Disjunctive:        freehire.NewOptBool(disjunctive),
		Regions:            e.regions,
		Countries:          splitCSV(*f.countries),
		Cities:             splitCSV(*f.cities),
		WorkMode:           e.workMode,
		Category:           splitCSV(*f.category),
		Role:               splitCSV(*f.role),
		Seniority:          e.seniority,
		Skills:             splitCSV(*f.skills),
		SkillsMode:         e.skillsMode,
		IsTech:             e.isTech,
		AiArchetype:        e.aiArchetype,
		Collections:        splitCSV(*f.collections),
		Reality:            e.reality,
		Source:             splitCSV(*f.source),
		CompanySlug:        splitCSV(*f.companySlug),
		EmploymentType:     e.employmentType,
		Relocation:         e.relocation,
		EnglishLevel:       e.englishLevel,
		EducationLevel:     e.educationLevel,
		PostingLanguage:    splitCSV(*f.postingLanguage),
		Domains:            e.domains,
		CompanyType:        e.companyType,
		CompanySize:        e.companySize,
		SalaryCurrency:     splitCSV(*f.salaryCurrency),
		SalaryPeriod:       e.salaryPeriod,
		VisaSponsorship:    optTristateBool(*f.visaSponsorship),
		SalaryMin:          optInt(*f.salaryMin),
		SalaryMax:          optInt(*f.salaryMax),
		ExperienceYearsMin: optInt(*f.experienceYearsMin),
		PostedWithinDays:   optInt(*f.postedWithinDays),
	}, nil
}

func companiesParams(o companiesOpts) (freehire.SearchCompaniesParams, error) {
	companyType, err := enumCSV[freehire.SearchCompaniesCompanyTypeItem](o.companyType, "company-type")
	if err != nil {
		return freehire.SearchCompaniesParams{}, err
	}
	companySize, err := enumCSV[freehire.SearchCompaniesCompanySizeItem](o.companySize, "company-size")
	if err != nil {
		return freehire.SearchCompaniesParams{}, err
	}
	params := freehire.SearchCompaniesParams{
		Q:             optString(o.q),
		Limit:         optInt(o.limit),
		Offset:        optInt(o.offset),
		Collections:   splitCSV(o.collections),
		Regions:       splitCSV(o.regions),
		Countries:     splitCSV(o.countries),
		RemoteRegions: splitCSV(o.remoteRegions),
		Industries:    splitCSV(o.industries),
		Domains:       splitCSV(o.domains),
		CompanyType:   companyType,
		CompanySize:   companySize,
		Maturity:      splitCSV(o.maturity),
		YcBatch:       splitCSV(o.ycBatch),
		YcStatus:      splitCSV(o.ycStatus),
		YcStage:       splitCSV(o.ycStage),
		YcFlags:       splitCSV(o.ycFlags),
	}
	if o.sortField != "" {
		field, err := enumOne[freehire.SearchCompaniesSort](o.sortField, "sort")
		if err != nil {
			return freehire.SearchCompaniesParams{}, err
		}
		params.Sort = freehire.NewOptSearchCompaniesSort(field)
	}
	return params, nil
}

// ignoredParams turns upstream's dropped-parameter report into an error.
// A parameter no filter reads is ignored rather than refused, and the
// answer is the whole catalogue — which prints exactly like a search that
// legitimately matched everything.
func ignoredParams(ignored []freehire.IgnoredParam) error {
	if len(ignored) == 0 {
		return nil
	}
	names := make([]string, 0, len(ignored))
	for _, p := range ignored {
		if hint, ok := p.DidYouMean.Get(); ok {
			names = append(names, fmt.Sprintf("%s (did you mean %s?)", p.Param, hint))
			continue
		}
		names = append(names, p.Param)
	}
	return fmt.Errorf("freehire dropped %d parameter(s) and answered with an unfiltered result: %s", len(ignored), strings.Join(names, ", "))
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

// passthrough re-encodes one of the spec's open-ended objects as a plain
// map. The spec names their fields but still allows extras, so
// re-encoding forwards every key upstream sent.
func passthrough(v json.Marshaler) map[string]any {
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || len(out) == 0 {
		return nil
	}
	return out
}

func renderHTML(html string) string {
	if html == "" {
		return ""
	}
	text, err := html2text.FromString(html, html2text.Options{})
	if err != nil {
		return html
	}
	return text
}

type jobJSON struct {
	Slug              string         `json:"slug"`
	Title             string         `json:"title"`
	Company           string         `json:"company"`
	CompanySlug       string         `json:"company_slug,omitempty"`
	Location          string         `json:"location,omitempty"`
	Regions           []string       `json:"regions,omitempty"`
	Countries         []string       `json:"countries,omitempty"`
	Cities            []string       `json:"cities,omitempty"`
	WorkMode          string         `json:"work_mode,omitempty"`
	Skills            []string       `json:"skills,omitempty"`
	Collections       []string       `json:"collections,omitempty"`
	IsTech            string         `json:"is_tech,omitempty"`
	Source            string         `json:"source,omitempty"`
	ExternalID        string         `json:"external_id,omitempty"`
	ManuallyAdded     *bool          `json:"manually_added,omitempty"`
	PostedAt          string         `json:"posted_at,omitempty"`
	CreatedAt         string         `json:"created_at,omitempty"`
	UpdatedAt         string         `json:"updated_at,omitempty"`
	LastSeenAt        string         `json:"last_seen_at,omitempty"`
	ClosedAt          string         `json:"closed_at,omitempty"`
	Enrichment        map[string]any `json:"enrichment,omitempty"`
	EnrichedAt        string         `json:"enriched_at,omitempty"`
	EnrichmentVersion *int           `json:"enrichment_version,omitempty"`
	Reality           map[string]any `json:"reality,omitempty"`
	Ghost             map[string]any `json:"ghost,omitempty"`
	ViewCount         *int           `json:"view_count,omitempty"`
	AppliedCount      *int           `json:"applied_count,omitempty"`
	UpvoteCount       *int           `json:"upvote_count,omitempty"`
	DownvoteCount     *int           `json:"downvote_count,omitempty"`
	Description       string         `json:"description,omitempty"`
	URL               string         `json:"url,omitempty"`
}

func intPtr(v interface{ Get() (int, bool) }) *int {
	if n, ok := v.Get(); ok {
		return &n
	}
	return nil
}

func summarize(j freehire.Job) jobJSON {
	out := jobJSON{
		Slug:              j.PublicSlug,
		Title:             j.Title,
		Company:           j.Company,
		CompanySlug:       j.CompanySlug.Or(""),
		Location:          j.Location.Or(""),
		Regions:           j.Regions,
		Countries:         j.Countries,
		Cities:            j.Cities,
		WorkMode:          string(j.WorkMode.Or("")),
		Skills:            j.Skills,
		Collections:       j.Collections,
		IsTech:            string(j.IsTech.Or("")),
		Source:            j.Source.Or(""),
		ExternalID:        j.ExternalID.Or(""),
		PostedAt:          optDate(j.PostedAt),
		CreatedAt:         optDate(j.CreatedAt),
		UpdatedAt:         optDate(j.UpdatedAt),
		LastSeenAt:        optDate(j.LastSeenAt),
		ClosedAt:          optDate(j.ClosedAt),
		EnrichedAt:        optDate(j.EnrichedAt),
		EnrichmentVersion: intPtr(j.EnrichmentVersion),
		ViewCount:         intPtr(j.ViewCount),
		AppliedCount:      intPtr(j.AppliedCount),
		UpvoteCount:       intPtr(j.UpvoteCount),
		DownvoteCount:     intPtr(j.DownvoteCount),
		Description:       renderHTML(j.Description.Or("")),
	}
	if u, ok := j.URL.Get(); ok {
		out.URL = u.String()
	}
	if added, ok := j.ManuallyAdded.Get(); ok {
		out.ManuallyAdded = &added
	}
	if e, ok := j.Enrichment.Get(); ok {
		out.Enrichment = passthrough(&e)
	}
	if r, ok := j.Reality.Get(); ok {
		out.Reality = passthrough(&r)
	}
	if g, ok := j.Ghost.Get(); ok {
		out.Ghost = passthrough(&g)
	}
	return out
}

func newClient(ctx context.Context, f clientFlags) (*freehire.Client, context.Context, context.CancelFunc, error) {
	hc := &http.Client{Transport: freehire.Transport{}}
	client, err := freehire.NewClient(f.baseURL, freehire.WithClient(hc))
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	return client, ctx, cancel, nil
}

func encodeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printJobLine(i int, j jobJSON) {
	fmt.Printf("%d. %s\n", i+1, j.Title)
	if j.CompanySlug != "" {
		fmt.Printf("Company: %s (%s)\n", j.Company, j.CompanySlug)
	} else {
		fmt.Printf("Company: %s\n", j.Company)
	}
	switch {
	case j.Location != "" && j.WorkMode != "":
		fmt.Printf("Location: %s (%s)\n", j.Location, j.WorkMode)
	case j.Location != "":
		fmt.Printf("Location: %s\n", j.Location)
	case j.WorkMode != "":
		fmt.Printf("Work mode: %s\n", j.WorkMode)
	}
	if j.Source != "" {
		fmt.Printf("Source: %s\n", j.Source)
	}
	if j.PostedAt != "" {
		fmt.Printf("Posted: %s\n", j.PostedAt)
	}
	if class, ok := j.Reality["class"].(string); ok && class != "" {
		fmt.Printf("Reality: %s\n", class)
	}
	if level, ok := j.Ghost["level"].(string); ok && level != "" {
		fmt.Printf("Ghost: %s\n", level)
	}
	fmt.Printf("Slug: %s\n", j.Slug)
	if j.URL != "" {
		fmt.Printf("URL: %s\n", j.URL)
	}
	fmt.Println()
}

func runSearch(ctx context.Context, cf clientFlags, o searchOpts) error {
	params, err := searchParams(o)
	if err != nil {
		return err
	}
	client, ctx, cancel, err := newClient(ctx, cf)
	if err != nil {
		return err
	}
	defer cancel()

	res, err := client.SearchJobs(ctx, params)
	if err != nil {
		return err
	}
	meta := res.Meta.Or(freehire.PaginationMeta{})
	if err := ignoredParams(meta.IgnoredParams); err != nil {
		return err
	}

	jobs := make([]jobJSON, len(res.Data))
	for i, j := range res.Data {
		jobs[i] = summarize(j)
	}

	if cf.format == "json" {
		return encodeJSON(struct {
			Total  *int      `json:"total,omitempty"`
			Limit  *int      `json:"limit,omitempty"`
			Offset *int      `json:"offset,omitempty"`
			Jobs   []jobJSON `json:"jobs"`
		}{intPtr(meta.Total), intPtr(meta.Limit), intPtr(meta.Offset), jobs})
	}

	fmt.Printf("freehire jobs (total: %d, offset %d, limit %d)\n\n", meta.Total.Or(0), meta.Offset.Or(0), meta.Limit.Or(0))
	for i, j := range jobs {
		printJobLine(i, j)
	}
	return nil
}

func runFacets(ctx context.Context, cf clientFlags, f *filterFlags, facets string, disjunctive bool) error {
	params, err := facetsParams(f, facets, disjunctive)
	if err != nil {
		return err
	}
	client, ctx, cancel, err := newClient(ctx, cf)
	if err != nil {
		return err
	}
	defer cancel()

	res, err := client.GetJobFacets(ctx, params)
	if err != nil {
		return err
	}
	if meta, ok := res.Meta.Get(); ok {
		if err := ignoredParams(meta.IgnoredParams); err != nil {
			return err
		}
	}

	out := struct {
		Total  int                           `json:"total"`
		Facets map[string]map[string]int     `json:"facets"`
		Stats  map[string]map[string]float64 `json:"stats,omitempty"`
	}{Total: res.Data.Total, Facets: make(map[string]map[string]int, len(res.Data.Facets))}
	for name, vals := range res.Data.Facets {
		out.Facets[name] = vals
	}
	if stats, ok := res.Data.Stats.Get(); ok {
		out.Stats = make(map[string]map[string]float64, len(stats))
		for name, vals := range stats {
			out.Stats[name] = vals
		}
	}

	if cf.format == "json" {
		return encodeJSON(out)
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
	for name, vals := range out.Stats {
		fmt.Printf("%s range: min %g, max %g\n", name, vals["min"], vals["max"])
	}
	return nil
}

func runCompanies(ctx context.Context, cf clientFlags, o companiesOpts) error {
	params, err := companiesParams(o)
	if err != nil {
		return err
	}
	client, ctx, cancel, err := newClient(ctx, cf)
	if err != nil {
		return err
	}
	defer cancel()

	res, err := client.SearchCompanies(ctx, params)
	if err != nil {
		return err
	}
	meta := res.Meta.Or(freehire.PaginationMeta{})
	if err := ignoredParams(meta.IgnoredParams); err != nil {
		return err
	}

	companies := make([]map[string]any, 0, len(res.Data))
	for _, c := range res.Data {
		companies = append(companies, passthrough(&c))
	}

	if cf.format == "json" {
		return encodeJSON(struct {
			Total     *int             `json:"total,omitempty"`
			Companies []map[string]any `json:"companies"`
		}{intPtr(meta.Total), companies})
	}

	fmt.Printf("freehire companies (total: %d)\n\n", meta.Total.Or(0))
	for i, c := range res.Data {
		fmt.Printf("%d. %s\n", i+1, c.Name)
		fmt.Printf("Slug: %s\n", c.Slug)
		fmt.Printf("Open jobs: %d\n", c.JobCount.Or(0))
		if tagline, ok := c.Tagline.Get(); ok && tagline != "" {
			fmt.Printf("Tagline: %s\n", tagline)
		}
		if len(c.Industries) > 0 {
			fmt.Printf("Industries: %s\n", strings.Join(c.Industries, ", "))
		}
		if len(c.Collections) > 0 {
			fmt.Printf("Collections: %s\n", strings.Join(c.Collections, ", "))
		}
		fmt.Println()
	}
	return nil
}

func runCompany(ctx context.Context, cf clientFlags, slug string, limit, offset int) error {
	client, ctx, cancel, err := newClient(ctx, cf)
	if err != nil {
		return err
	}
	defer cancel()

	res, err := client.GetCompany(ctx, freehire.GetCompanyParams{
		Slug:   slug,
		Limit:  optInt(limit),
		Offset: optInt(offset),
	})
	if err != nil {
		return fmt.Errorf("company %q: %w", slug, err)
	}

	var profile map[string]any
	if c, ok := res.Data.Company.Get(); ok {
		profile = passthrough(&c)
	}
	jobs := make([]jobJSON, len(res.Data.Jobs))
	for i, j := range res.Data.Jobs {
		jobs[i] = summarize(j)
	}

	if cf.format == "json" {
		return encodeJSON(struct {
			Company           map[string]any `json:"company"`
			Jobs              []jobJSON      `json:"jobs"`
			ReferralAvailable *bool          `json:"referral_available,omitempty"`
		}{profile, jobs, boolPtr(res.Data.ReferralAvailable)})
	}

	keys := make([]string, 0, len(profile))
	for k := range profile {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("freehire company %s\n\n", slug)
	for _, k := range keys {
		// company_info alone is kilobytes of nested objects, so text mode
		// names the profile fields and leaves reading them to --format json.
		v, err := json.Marshal(profile[k])
		if err != nil {
			continue
		}
		if len(v) > 200 {
			fmt.Printf("%s: <%d bytes; use --format json>\n", k, len(v))
			continue
		}
		fmt.Printf("%s: %s\n", k, v)
	}
	fmt.Printf("\nOpen jobs on this page: %d\n\n", len(jobs))
	for i, j := range jobs {
		printJobLine(i, j)
	}
	return nil
}

func boolPtr(v freehire.OptBool) *bool {
	if b, ok := v.Get(); ok {
		return &b
	}
	return nil
}

func runCities(ctx context.Context, cf clientFlags, q, country string) error {
	client, ctx, cancel, err := newClient(ctx, cf)
	if err != nil {
		return err
	}
	defer cancel()

	res, err := client.SearchCities(ctx, freehire.SearchCitiesParams{
		Q:       optString(q),
		Country: optString(country),
	})
	if err != nil {
		return err
	}

	if cf.format == "json" {
		return encodeJSON(res)
	}

	fmt.Printf("freehire cities matching %q (%d)\n\n", q, len(res.Data))
	for _, city := range res.Data {
		fmt.Printf("%s (%s)\n", city.Value, city.Country)
	}
	return nil
}

func runDetail(ctx context.Context, cf clientFlags, slug string) error {
	client, ctx, cancel, err := newClient(ctx, cf)
	if err != nil {
		return err
	}
	defer cancel()

	res, err := client.GetJob(ctx, freehire.GetJobParams{Slug: slug})
	if err != nil {
		return fmt.Errorf("job %q: %w", slug, err)
	}
	return printDetail(res.Data, cf.format)
}

func printDetail(j freehire.Job, format string) error {
	d := summarize(j)
	if format == "json" {
		return encodeJSON(d)
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
	if d.ClosedAt != "" {
		fmt.Printf("Closed: %s\n", d.ClosedAt)
	}
	if class, ok := d.Reality["class"].(string); ok && class != "" {
		fmt.Printf("Reality: %s\n", class)
	}
	if level, ok := d.Ghost["level"].(string); ok && level != "" {
		fmt.Printf("Ghost: %s\n", level)
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
