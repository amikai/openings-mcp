package openingsmcp

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/freehire"
	"github.com/go-faster/jx"
	"github.com/jaytaylor/html2text"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// freehirePageSize matches the package-wide pageSize convention in
// internal/ats; the upstream API accepts up to 100 but 20 keeps tool
// results economical.
const freehirePageSize = 20

// freehireMaxPage is the last page reachable inside freehire's result
// window at freehirePageSize rows per page.
const freehireMaxPage = freehire.MaxResultWindow / freehirePageSize

// freehireDescriptionChars bounds the description each search row
// carries. GET /agent/jobs/search ships every row's full posting
// whatever description_format asks for and offers no way to opt out, so
// a 20-row page pulls tens of kilobytes either way; keeping an opening
// worth triaging beats discarding all of it, and freehire_get_job_detail
// still has the rest.
const freehireDescriptionChars = 400

// freehireDefaultFacetValues caps how many values each facet
// contributes to a tool result by default. The unfiltered upstream
// response is ~86 KB across two dozen facets, with cities, role, and
// skills carrying 1200 values each.
const freehireDefaultFacetValues = 25

var freehireSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"query": {
			"type": "string",
			"description": "Full-text keywords over title, company, and description."
		},
		"company": {
			"type": "string",
			"description": "Catalogue company_slug, comma-separated for several, e.g. 'stripe'. Take it from a search result's company_slug or from freehire_search_companies — a display name such as 'Altinity, Inc.' matches nothing. Optional; omit to search the whole board. A company name by itself is not a source selection — use search_jobs_by_company for a company's own careers site."
		},
		"skills": {
			"type": "string",
			"description": "Comma-separated canonical skill slugs from freehire_get_job_facets, e.g. 'go,rust'."
		},
		"seniority": {
			"type": "array",
			"description": "Seniority levels, OR'd together.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["intern", "junior", "middle", "senior", "lead", "staff", "principal", "c_level"]
			}
		},
		"work_mode": {
			"type": "array",
			"description": "Work arrangements, OR'd together.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["remote", "hybrid", "onsite"]
			}
		},
		"regions": {
			"type": "array",
			"description": "Geographic regions, OR'd together.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["global", "north_america", "latam", "eu", "uk", "mena", "africa", "apac", "cis"]
			}
		},
		"country": {
			"type": "string",
			"description": "Comma-separated lowercase ISO country codes from freehire_get_job_facets, e.g. 'us,de'."
		},
		"source": {
			"type": "string",
			"description": "Comma-separated ingest-source slugs from freehire_get_job_facets, e.g. 'greenhouse,lever'. Narrows to postings freehire crawled from that ATS."
		},
		"category": {
			"type": "string",
			"description": "Comma-separated role-category slugs from freehire_get_job_facets, e.g. 'backend,devops'."
		},
		"salary_min": {
			"type": "integer",
			"description": "Lower bound on the posting's salary range. Most postings carry no salary at all and drop out when this is set.",
			"minimum": 0
		},
		"salary_max": {
			"type": "integer",
			"description": "Upper bound on the posting's salary range. Same caveat as salary_min.",
			"minimum": 0
		},
		"semantic_ratio": {
			"type": "number",
			"description": "How far to blend semantic matching into the keyword ranking, 0 for pure keyword. Affects ranking only, never the result count.",
			"minimum": 0,
			"maximum": 1
		},
		"sort": {
			"type": "string",
			"description": "Sort field. Omit for relevance; use 'posted_at' with order 'desc' for newest first.",
			"enum": ["created_at", "posted_at", "salary_min", "salary_max"]
		},
		"order": {
			"type": "string",
			"description": "Sort direction; only meaningful together with sort.",
			"enum": ["asc", "desc"]
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; 20 results per page. Pages past 500 are unreachable — freehire refuses offsets beyond 10000 rows.",
			"minimum": 1,
			"maximum": 500,
			"default": 1
		}
	},
	"additionalProperties": false
}`)

var freehireSearchInputSchema = mustSchema(freehireSearchInputRawSchema)

var freehireFacetsInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"query": {
			"type": "string",
			"description": "Optional full-text query. Facet values and counts are for jobs matching this query."
		},
		"company": {
			"type": "string",
			"description": "Catalogue company_slug, comma-separated for several, e.g. 'stripe'."
		},
		"skills": {
			"type": "string",
			"description": "Comma-separated canonical skill slugs, e.g. 'go,rust'."
		},
		"seniority": {
			"type": "array",
			"description": "Seniority levels, OR'd together.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["intern", "junior", "middle", "senior", "lead", "staff", "principal", "c_level"]
			}
		},
		"work_mode": {
			"type": "array",
			"description": "Work arrangements, OR'd together.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["remote", "hybrid", "onsite"]
			}
		},
		"regions": {
			"type": "array",
			"description": "Geographic regions, OR'd together.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["global", "north_america", "latam", "eu", "uk", "mena", "africa", "apac", "cis"]
			}
		},
		"country": {
			"type": "string",
			"description": "Comma-separated lowercase ISO country codes, e.g. 'us,de'."
		},
		"source": {
			"type": "string",
			"description": "Comma-separated ingest-source slugs, e.g. 'greenhouse,lever'."
		},
		"category": {
			"type": "string",
			"description": "Comma-separated role-category slugs, e.g. 'backend,devops'."
		},
		"salary_min": {
			"type": "integer",
			"description": "Lower bound on the posting's salary range.",
			"minimum": 0
		},
		"salary_max": {
			"type": "integer",
			"description": "Upper bound on the posting's salary range.",
			"minimum": 0
		},
		"facets": {
			"type": "string",
			"description": "Comma-separated facet names to return, e.g. 'skills,countries'. Omit for every facet. These are the ones freehire_search_jobs takes a filter for: skills, countries, source, category, seniority, work_mode, regions. The response carries others (cities, role, and more) that no filter accepts — informational only."
		},
		"max_values": {
			"type": "integer",
			"description": "Most common values to return per facet. Raise it only when the value you need is missing; cities, role, and skills each carry over a thousand values.",
			"minimum": 1,
			"maximum": 1000,
			"default": 25
		}
	},
	"additionalProperties": false
}`)

var freehireFacetsInputSchema = mustSchema(freehireFacetsInputRawSchema)

var freehireCompaniesInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"query": {
			"type": "string",
			"description": "Company name to resolve. Matching is fuzzy and tolerates typos — 'strpie' finds Stripe — so an approximate name is fine.",
			"minLength": 1
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; 20 companies per page.",
			"minimum": 1,
			"default": 1
		}
	},
	"required": ["query"],
	"additionalProperties": false
}`)

var freehireCompaniesInputSchema = mustSchema(freehireCompaniesInputRawSchema)

// freehireFilters are the narrowing fields freehire_search_jobs and
// freehire_get_job_facets share; both map onto the same query
// parameters upstream.
type freehireFilters struct {
	Query     string   `json:"query,omitempty"`
	Company   string   `json:"company,omitempty"`
	Skills    string   `json:"skills,omitempty"`
	Seniority []string `json:"seniority,omitempty"`
	WorkMode  []string `json:"work_mode,omitempty"`
	Regions   []string `json:"regions,omitempty"`
	Country   string   `json:"country,omitempty"`
	Source    string   `json:"source,omitempty"`
	Category  string   `json:"category,omitempty"`
	SalaryMin *int     `json:"salary_min,omitempty"`
	SalaryMax *int     `json:"salary_max,omitempty"`
}

type freehireSearchInput struct {
	freehireFilters
	SemanticRatio *float64 `json:"semantic_ratio,omitempty"`
	Sort          string   `json:"sort,omitempty"`
	Order         string   `json:"order,omitempty"`
	Page          int      `json:"page,omitempty"`
}

type freehireJobSummary struct {
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	CompanySlug string   `json:"company_slug,omitempty" jsonschema:"Catalogue slug; pass it as freehire_search_jobs's company param to see the rest of this company's postings."`
	Location    string   `json:"location,omitempty"`
	Regions     []string `json:"regions,omitempty"`
	Countries   []string `json:"countries,omitempty"`
	WorkMode    string   `json:"work_mode,omitempty"`
	Source      string   `json:"source,omitempty"`
	Skills      []string `json:"skills,omitempty"`
	PostedAt    string   `json:"posted_at,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty" jsonschema:"When freehire first ingested the posting; posted_at is the employer's own date."`
	ClosedAt    string   `json:"closed_at,omitempty" jsonschema:"Set once freehire has seen the posting come down."`

	Enrichment map[string]any `json:"enrichment,omitempty" jsonschema:"freehire's derived fields — summary, category, seniority, salary, and more. Its keys vary by posting."`
	Reality    map[string]any `json:"reality,omitempty" jsonschema:"freehire's freshness signals — class fresh/stale, age_days, repost_count, mass_posting_count, fake_freshness. Use it to judge whether a posting is still worth applying to."`

	Description string `json:"description,omitempty" jsonschema:"Opening of the posting as plain text, truncated; freehire_get_job_detail returns it in full."`
	JobID       string `json:"job_id" jsonschema:"public_slug; pass to freehire_get_job_detail's job_id param."`
	URL         string `json:"url,omitempty" jsonschema:"Employer's own apply URL."`
}

type freehireSearchOutput struct {
	Total    *int                 `json:"total,omitempty" jsonschema:"Total matching jobs; absent when the API omits pagination metadata."`
	Page     int                  `json:"page"`
	LastPage *int                 `json:"last_page,omitempty" jsonschema:"Deepest reachable page, capped at 500 by the API's 10000-result window; absent when total is unknown."`
	Data     []freehireJobSummary `json:"data"`
}

type freehireFacetsInput struct {
	freehireFilters
	Facets    string `json:"facets,omitempty"`
	MaxValues int    `json:"max_values,omitempty"`
}

type freehireFacetValues struct {
	Values      map[string]int `json:"values"`
	TotalValues int            `json:"total_values" jsonschema:"Distinct values upstream; more than the entries in values means max_values kept only the most common ones."`
}

type freehireFacetsOutput struct {
	Total  int                            `json:"total"`
	Facets map[string]freehireFacetValues `json:"facets"`
	Stats  map[string]map[string]float64  `json:"stats,omitempty"`
}

type freehireCompaniesInput struct {
	Query string `json:"query"`
	Page  int    `json:"page,omitempty"`
}

type freehireCompany struct {
	Slug     string `json:"slug" jsonschema:"Pass this as freehire_search_jobs's company param."`
	Name     string `json:"name"`
	JobCount *int   `json:"job_count,omitempty" jsonschema:"Open postings freehire holds for this company; the tiebreaker when two companies share a name."`

	Details map[string]any `json:"details,omitempty" jsonschema:"Everything else freehire holds — tagline, industries, hq_country, collections. Keys vary by company."`
}

type freehireCompaniesOutput struct {
	Total    *int              `json:"total,omitempty" jsonschema:"Total matching companies; absent when the API omits pagination metadata."`
	Page     int               `json:"page"`
	LastPage *int              `json:"last_page,omitempty" jsonschema:"Deepest page for this query; absent when total is unknown."`
	Data     []freehireCompany `json:"data"`
}

type freehireDetailInput struct {
	JobID string `json:"job_id" jsonschema:"public_slug from a freehire_search_jobs result."`
}

type freehireDetailOutput struct {
	JobID       string   `json:"job_id" jsonschema:"public_slug, the same id freehire_search_jobs returns."`
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	CompanySlug string   `json:"company_slug,omitempty" jsonschema:"Catalogue slug; pass it as freehire_search_jobs's company param to see the rest of this company's postings."`
	Location    string   `json:"location,omitempty"`
	Regions     []string `json:"regions,omitempty"`
	Countries   []string `json:"countries,omitempty"`
	WorkMode    string   `json:"work_mode,omitempty"`
	Source      string   `json:"source,omitempty"`
	Skills      []string `json:"skills,omitempty"`
	PostedAt    string   `json:"posted_at,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty" jsonschema:"When freehire first ingested the posting; posted_at is the employer's own date."`
	ClosedAt    string   `json:"closed_at,omitempty" jsonschema:"Set once freehire has seen the posting come down."`

	Enrichment map[string]any `json:"enrichment,omitempty" jsonschema:"freehire's derived fields — summary, category, seniority, salary, and more. Its keys vary by posting."`
	Reality    map[string]any `json:"reality,omitempty" jsonschema:"freehire's freshness signals — class fresh/stale, age_days, repost_count, mass_posting_count, fake_freshness. Use it to judge whether a posting is still worth applying to."`

	Description string `json:"description" jsonschema:"Full job description as plain text."`
	ApplyURL    string `json:"apply_url,omitempty" jsonschema:"The company's own apply URL. Authoritative — the rest of this record is freehire's crawled snapshot."`
}

func freehireSplitCSV(s string) []string {
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

// freehireEnumSlug is the shape ogen gives every enum in the spec: a
// string type that validates itself against the spec's value list.
type freehireEnumSlug interface {
	~string
	Validate() error
}

// freehireEnumOne converts one slug to its generated enum type. The
// tool input schemas enum the same values, so Validate only fires when
// a schema and openapi.yaml drift apart.
func freehireEnumOne[T freehireEnumSlug](slug, field string) (T, error) {
	item := T(slug)
	if err := item.Validate(); err != nil {
		return item, fmt.Errorf("invalid %s %q: %w", field, slug, err)
	}
	return item, nil
}

func freehireEnums[T freehireEnumSlug](slugs []string, field string) ([]T, error) {
	if len(slugs) == 0 {
		return nil, nil
	}
	out := make([]T, 0, len(slugs))
	for _, slug := range slugs {
		item, err := freehireEnumOne[T](slug, field)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// freehireEnumFilters holds the shared filters that upstream types as
// enums rather than free-form strings.
type freehireEnumFilters struct {
	seniority []freehire.AgentSearchJobsSeniorityItem
	workMode  []freehire.AgentSearchJobsWorkModeItem
	regions   []freehire.AgentSearchJobsRegionsItem
}

func freehireParseFilters(in freehireFilters) (freehireEnumFilters, error) {
	seniority, err := freehireEnums[freehire.AgentSearchJobsSeniorityItem](in.Seniority, "seniority")
	if err != nil {
		return freehireEnumFilters{}, err
	}
	workMode, err := freehireEnums[freehire.AgentSearchJobsWorkModeItem](in.WorkMode, "work_mode")
	if err != nil {
		return freehireEnumFilters{}, err
	}
	regions, err := freehireEnums[freehire.AgentSearchJobsRegionsItem](in.Regions, "regions")
	if err != nil {
		return freehireEnumFilters{}, err
	}
	return freehireEnumFilters{seniority: seniority, workMode: workMode, regions: regions}, nil
}

// freehireDescriptionPreview collapses a description to one truncated
// line. Search asks for description_format=text, so there is no markup
// to strip here — unlike the detail tool, which gets the stored HTML.
func freehireDescriptionPreview(desc string) string {
	desc = strings.Join(strings.Fields(desc), " ")
	runes := []rune(desc)
	if len(runes) <= freehireDescriptionChars {
		return desc
	}
	return strings.TrimRight(string(runes[:freehireDescriptionChars]), " ") + "…"
}

func freehireJobURL(j freehire.Job) string {
	if u, ok := j.URL.Get(); ok {
		return u.String()
	}
	return ""
}

// freehireOptTime covers both OptDateTime and OptNilDateTime, which ogen
// gives the same Get.
type freehireOptTime interface {
	Get() (time.Time, bool)
}

func freehireDate(v freehireOptTime) string {
	if t, ok := v.Get(); ok {
		return t.UTC().Format(time.DateOnly)
	}
	return ""
}

// freehireRawObject unwraps the free-form objects the spec declares with
// additionalProperties, which ogen leaves as raw JSON. Their keys vary by
// posting, so they pass through as-is rather than through a struct that
// would have to guess the shape.
func freehireRawObject(m map[string]jx.Raw) map[string]any {
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

// freehirePage normalizes the 1-based page number both tool inputs
// accept, so the offset arithmetic and the echoed page agree.
func freehirePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func freehireMCPToHTTPRequest(in *freehireSearchInput, page int) (freehire.AgentSearchJobsParams, error) {
	// Checked before multiplying: (page-1)*freehirePageSize overflows
	// int64 for a large enough page, and a wrapped negative offset
	// reaches the API as a silent first page.
	if page > freehireMaxPage {
		return freehire.AgentSearchJobsParams{}, fmt.Errorf("page %d is past page %d, the deepest freehire pages to (%d results)", page, freehireMaxPage, freehire.MaxResultWindow)
	}
	f, err := freehireParseFilters(in.freehireFilters)
	if err != nil {
		return freehire.AgentSearchJobsParams{}, err
	}
	params := freehire.AgentSearchJobsParams{
		Limit:  freehire.NewOptInt(freehirePageSize),
		Offset: freehire.NewOptInt((page - 1) * freehirePageSize),
		// Every row carries a description whatever we ask for, so take
		// the cheaper plain text over the stored markup.
		DescriptionFormat: freehire.NewOptAgentSearchJobsDescriptionFormat(freehire.AgentSearchJobsDescriptionFormatText),
		Skills:            freehireSplitCSV(in.Skills),
		Seniority:         f.seniority,
		WorkMode:          f.workMode,
		Regions:           f.regions,
		Countries:         freehireSplitCSV(in.Country),
		Source:            freehireSplitCSV(in.Source),
		Category:          freehireSplitCSV(in.Category),
		CompanySlug:       freehireSplitCSV(in.Company),
	}
	if in.Query != "" {
		params.Q = freehire.NewOptString(in.Query)
	}
	if in.SalaryMin != nil {
		params.SalaryMin = freehire.NewOptInt(*in.SalaryMin)
	}
	if in.SalaryMax != nil {
		params.SalaryMax = freehire.NewOptInt(*in.SalaryMax)
	}
	if in.SemanticRatio != nil {
		params.SemanticRatio = freehire.NewOptFloat64(*in.SemanticRatio)
	}
	if in.Sort != "" {
		field, err := freehireEnumOne[freehire.AgentSearchJobsSort](in.Sort, "sort")
		if err != nil {
			return freehire.AgentSearchJobsParams{}, err
		}
		params.Sort = freehire.NewOptAgentSearchJobsSort(field)
	}
	if in.Order != "" {
		order, err := freehireEnumOne[freehire.AgentSearchJobsOrder](in.Order, "order")
		if err != nil {
			return freehire.AgentSearchJobsParams{}, err
		}
		params.Order = freehire.NewOptAgentSearchJobsOrder(order)
	}
	return params, nil
}

func freehireMCPToFacetsRequest(in *freehireFacetsInput) (freehire.GetJobFacetsParams, error) {
	f, err := freehireParseFilters(in.freehireFilters)
	if err != nil {
		return freehire.GetJobFacetsParams{}, err
	}
	params := freehire.GetJobFacetsParams{
		Skills:      freehireSplitCSV(in.Skills),
		Seniority:   freehireSlugsOf(f.seniority),
		WorkMode:    freehireSlugsOf(f.workMode),
		Regions:     freehireSlugsOf(f.regions),
		Countries:   freehireSplitCSV(in.Country),
		Source:      freehireSplitCSV(in.Source),
		Category:    freehireSplitCSV(in.Category),
		CompanySlug: freehireSplitCSV(in.Company),
	}
	if in.Query != "" {
		params.Q = freehire.NewOptString(in.Query)
	}
	if in.SalaryMin != nil {
		params.SalaryMin = freehire.NewOptInt(*in.SalaryMin)
	}
	if in.SalaryMax != nil {
		params.SalaryMax = freehire.NewOptInt(*in.SalaryMax)
	}
	return params, nil
}

// freehireSlugsOf unwraps generated enum values for GetJobFacetsParams,
// whose spec leaves the same fields as free-form strings.
func freehireSlugsOf[T ~string](items []T) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = string(item)
	}
	return out
}

// freehireTopFacetValues keeps the maxValues most common values of one
// facet, so a tool result stays readable next to facets that carry over
// a thousand values.
func freehireTopFacetValues(vals map[string]int, maxValues int) freehireFacetValues {
	out := freehireFacetValues{TotalValues: len(vals)}
	if len(vals) <= maxValues {
		out.Values = maps.Clone(vals)
		if out.Values == nil {
			out.Values = map[string]int{}
		}
		return out
	}
	names := make([]string, 0, len(vals))
	for name := range vals {
		names = append(names, name)
	}
	// Ties break on name so the same query keeps returning the same
	// values across calls.
	slices.SortFunc(names, func(a, b string) int {
		if c := cmp.Compare(vals[b], vals[a]); c != 0 {
			return c
		}
		return cmp.Compare(a, b)
	})
	out.Values = make(map[string]int, maxValues)
	for _, name := range names[:maxValues] {
		out.Values[name] = vals[name]
	}
	return out
}

func freehireHTTPToMCPFacets(res *freehire.FacetsEnvelope, want []string, maxValues int) *freehireFacetsOutput {
	keep := make(map[string]bool, len(want))
	for _, name := range want {
		keep[name] = true
	}
	out := &freehireFacetsOutput{
		Total:  res.Data.Total,
		Facets: make(map[string]freehireFacetValues, len(res.Data.Facets)),
	}
	for name, vals := range res.Data.Facets {
		if len(keep) > 0 && !keep[name] {
			continue
		}
		out.Facets[name] = freehireTopFacetValues(vals, maxValues)
	}
	if stats, ok := res.Data.Stats.Get(); ok {
		out.Stats = make(map[string]map[string]float64, len(stats))
		for name, vals := range stats {
			out.Stats[name] = map[string]float64(vals)
		}
	}
	return out
}

func freehireLastPage(total int) int {
	if total <= 0 {
		return 1
	}
	return min((total+freehirePageSize-1)/freehirePageSize, freehireMaxPage)
}

func freehireHTTPToMCPResponse(res *freehire.JobListEnvelope, page int) *freehireSearchOutput {
	out := &freehireSearchOutput{
		Page: page,
		Data: make([]freehireJobSummary, 0, len(res.Data)),
	}
	// meta is optional in the spec. Defaulting a missing total to 0
	// would read as "no matches" beside a full page of results, so
	// leave both counts out instead.
	if meta, ok := res.Meta.Get(); ok {
		if total, ok := meta.Total.Get(); ok {
			last := freehireLastPage(total)
			out.Total, out.LastPage = &total, &last
		}
	}
	for _, j := range res.Data {
		out.Data = append(out.Data, freehireJobSummary{
			Title:       j.Title,
			Company:     j.Company,
			CompanySlug: j.CompanySlug.Or(""),
			Location:    j.Location.Or(""),
			Regions:     j.Regions,
			Countries:   j.Countries,
			WorkMode:    j.WorkMode.Or(""),
			Source:      j.Source.Or(""),
			Skills:      j.Skills,
			PostedAt:    freehireDate(j.PostedAt),
			CreatedAt:   freehireDate(j.CreatedAt),
			ClosedAt:    freehireDate(j.ClosedAt),
			Enrichment:  freehireRawObject(j.Enrichment.Or(nil)),
			Reality:     freehireRawObject(j.Reality.Or(nil)),
			Description: freehireDescriptionPreview(j.Description.Or("")),
			JobID:       j.PublicSlug,
			URL:         freehireJobURL(j),
		})
	}
	return out
}

func freehireHTTPToMCPCompanies(res *freehire.SearchCompaniesOK, page int) *freehireCompaniesOutput {
	out := &freehireCompaniesOutput{
		Page: page,
		Data: make([]freehireCompany, 0, len(res.Data)),
	}
	if meta, ok := res.Meta.Get(); ok {
		if total, ok := meta.Total.Get(); ok {
			last := max((total+freehirePageSize-1)/freehirePageSize, 1)
			out.Total, out.LastPage = &total, &last
		}
	}
	for _, c := range res.Data {
		company := freehireCompany{
			Slug:    c.Slug,
			Name:    c.Name,
			Details: freehireRawObject(c.AdditionalProps),
		}
		if n, ok := c.JobCount.Get(); ok {
			company.JobCount = &n
		}
		out.Data = append(out.Data, company)
	}
	return out
}

func freehireHTTPToMCPDetail(job freehire.Job) *freehireDetailOutput {
	desc := job.Description.Or("")
	if text, err := html2text.FromString(desc, html2text.Options{}); err == nil {
		desc = text
	}
	return &freehireDetailOutput{
		JobID:       job.PublicSlug,
		Title:       job.Title,
		Company:     job.Company,
		CompanySlug: job.CompanySlug.Or(""),
		Location:    job.Location.Or(""),
		Regions:     job.Regions,
		Countries:   job.Countries,
		WorkMode:    job.WorkMode.Or(""),
		Source:      job.Source.Or(""),
		Skills:      job.Skills,
		PostedAt:    freehireDate(job.PostedAt),
		CreatedAt:   freehireDate(job.CreatedAt),
		ClosedAt:    freehireDate(job.ClosedAt),
		Enrichment:  freehireRawObject(job.Enrichment.Or(nil)),
		Reality:     freehireRawObject(job.Reality.Or(nil)),
		Description: desc,
		ApplyURL:    freehireJobURL(job),
	}
}

// RegisterFreehire registers the freehire.me search, facets, company,
// and job-detail tools.
func RegisterFreehire(s *mcp.Server, c *freehire.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "freehire_search_jobs",
		Description: "Search IT/tech jobs on freehire.me, a catalogue of postings crawled from company ATS boards. Filters (company, skills, seniority, work mode, region, country, source, category) and sorting run server-side. Call freehire_get_job_facets before using open-vocabulary filters such as skills, country, source, or category. Complements search_jobs_by_company rather than replacing it — use this when the user names freehire or wants cross-company IT search with those facets.",
		Annotations: &mcp.ToolAnnotations{Title: "Search freehire.me jobs", ReadOnlyHint: true},
		InputSchema: freehireSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *freehireSearchInput) (*mcp.CallToolResult, *freehireSearchOutput, error) {
		page := freehirePage(in.Page)
		params, err := freehireMCPToHTTPRequest(in, page)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.AgentSearchJobs(ctx, params)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, freehireHTTPToMCPResponse(res, page), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "freehire_get_job_facets",
		Description: "Get live filter vocabulary and counts from freehire.me (GET /jobs/facets). Call before freehire_search_jobs when a filter value is uncertain: skills, countries, source, or category. Narrow the answer with the facets parameter — the full vocabulary runs to thousands of values. There is no company facet; use freehire_search_companies for company_slug.",
		Annotations: &mcp.ToolAnnotations{Title: "Get freehire.me search facets", ReadOnlyHint: true},
		InputSchema: freehireFacetsInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *freehireFacetsInput) (*mcp.CallToolResult, *freehireFacetsOutput, error) {
		params, err := freehireMCPToFacetsRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.GetJobFacets(ctx, params)
		if err != nil {
			return errorResult(err), nil, nil
		}
		maxValues := in.MaxValues
		if maxValues < 1 {
			maxValues = freehireDefaultFacetValues
		}
		return nil, freehireHTTPToMCPFacets(res, freehireSplitCSV(in.Facets), maxValues), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "freehire_search_companies",
		Description: "Resolve a company name to the company_slug that freehire_search_jobs's company filter takes. Matching is fuzzy, so an approximate or misspelled name works. Names are not slugs — 'Altinity, Inc.' is altinity-inc — and freehire_get_job_facets has no company facet, so this is the only way to look one up. Several companies can share a name; compare job_count and details before picking.",
		Annotations: &mcp.ToolAnnotations{Title: "Find freehire.me company slugs", ReadOnlyHint: true},
		InputSchema: freehireCompaniesInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *freehireCompaniesInput) (*mcp.CallToolResult, *freehireCompaniesOutput, error) {
		if in.Query == "" {
			return errorResult(fmt.Errorf("query is required (the company name to resolve)")), nil, nil
		}
		page := freehirePage(in.Page)
		res, err := c.SearchCompanies(ctx, freehire.SearchCompaniesParams{
			Q:      freehire.NewOptString(in.Query),
			Limit:  freehire.NewOptInt(freehirePageSize),
			Offset: freehire.NewOptInt((page - 1) * freehirePageSize),
		})
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, freehireHTTPToMCPCompanies(res, page), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "freehire_get_job_detail",
		Description: "Get the full description for a freehire.me job by its job_id (public_slug).",
		Annotations: &mcp.ToolAnnotations{Title: "Get freehire.me job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *freehireDetailInput) (*mcp.CallToolResult, *freehireDetailOutput, error) {
		if in.JobID == "" {
			return errorResult(fmt.Errorf("job_id is required (take it from a freehire_search_jobs result)")), nil, nil
		}
		res, err := c.GetJob(ctx, freehire.GetJobParams{Slug: in.JobID})
		if err != nil {
			return errorResult(fmt.Errorf("job %q not found", in.JobID)), nil, nil
		}
		return nil, freehireHTTPToMCPDetail(res.Data), nil
	})
}
