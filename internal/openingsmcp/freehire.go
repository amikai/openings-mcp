package openingsmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/freehire"
	"github.com/jaytaylor/html2text"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// freehireGeoWarning rides on every geography parameter. Upstream joins
// regions, countries and cities into a single OR-group, so naming a
// second place widens the search — the opposite of what a caller adding
// a filter expects.
const freehireGeoWarning = "WARNING: regions, countries and cities form ONE OR-group upstream, so adding a second geography WIDENS the search instead of narrowing it — 'countries=gb' plus 'cities=London' means United Kingdom OR London. To search one place, name only that place. There is no AND for these three."

// freehireFilterProps are the filters GET /jobs/search and
// GET /jobs/facets both accept, spelled with the upstream parameter
// names. Facets takes no sort, paging, or exclude parameter, so those
// live in freehireSearchOnlyProps.
//
// Closed vocabularies are arrays with an enum, which lets the tool
// schema reject a bad value before the request goes out. Open or
// generated vocabularies are comma-separated strings: enumerating the
// ~1200 values of cities, role, or skills would bloat every tool
// listing and still imply the list is exhaustive when it is not.
const freehireFilterProps = `
		"q": {
			"type": "string",
			"description": "Full-text query over title, company, and description."
		},
		"regions": {
			"type": "array",
			"description": "Macro-regions. Country codes are NOT regions — use countries. 'none' selects postings with no resolved geography. ` + freehireGeoWarning + `",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["global", "north_america", "latam", "eu", "uk", "mena", "africa", "apac", "cis", "none"]
			}
		},
		"countries": {
			"type": "string",
			"description": "Comma-separated lowercase ISO 3166-1 alpha-2 codes, e.g. 'gb,de'. ` + freehireGeoWarning + `"
		},
		"cities": {
			"type": "string",
			"description": "Comma-separated canonical city display names, e.g. 'London,Berlin'. Resolve each one with freehire_search_cities first — the facet holds display names and a near miss matches nothing rather than erroring. ` + freehireGeoWarning + `"
		},
		"work_mode": {
			"type": "array",
			"description": "Work formats, OR'd together. Absent on a posting whose format could not be resolved, so this narrows to postings that stated it.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["remote", "hybrid", "onsite"]
			}
		},
		"category": {
			"type": "string",
			"description": "Comma-separated role-category slugs from freehire_get_job_facets, e.g. 'backend,devops'."
		},
		"role": {
			"type": "string",
			"description": "Comma-separated fine-grained role slugs from freehire_get_job_facets, e.g. 'backend,android_developer'. Much narrower than category; the vocabulary is large and generated, so always read it from the facets tool rather than guessing."
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
		"skills": {
			"type": "string",
			"description": "Comma-separated canonical skill slugs from freehire_get_job_facets, e.g. 'go,rust'. Never invent one — an unknown slug matches nothing rather than erroring."
		},
		"skills_mode": {
			"type": "string",
			"description": "Set to 'and' to require EVERY listed skill instead of any of them. Omit for the default OR.",
			"enum": ["and"]
		},
		"is_tech": {
			"type": "array",
			"description": "Technical vs non-technical, derived from title and category. Absent when unknown, so this filter never guesses.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["tech", "non_tech"]
			}
		},
		"ai_archetype": {
			"type": "array",
			"description": "AI skill-signature archetype, derived from the posting's skill set.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["rag_app_builder", "agent_builder", "cloud_ml_platform_engineer", "ml_trainer_researcher", "fullstack_ai_engineer", "devops_infra_engineer"]
			}
		},
		"collections": {
			"type": "string",
			"description": "Comma-separated curated collection slugs of the posting's company, e.g. 'yc,unicorn'. Read the live set from freehire_get_job_facets."
		},
		"reality": {
			"type": "array",
			"description": "Posting-reality class. 'fresh' is recently posted and not obviously recycled, 'stale' is old, 'likely-evergreen' reads as an always-open pipeline ad. A large share of the catalogue is stale, so reality=['fresh'] is the cheapest quality filter available.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["fresh", "stale", "likely-evergreen"]
			}
		},
		"source": {
			"type": "string",
			"description": "Comma-separated origin slugs — the ATS or board the posting was crawled from, e.g. 'greenhouse,lever'. Read the live set from freehire_get_job_facets. Pair with source_exclude to avoid double-counting a board you already query directly."
		},
		"company_slug": {
			"type": "string",
			"description": "Comma-separated catalogue company slugs, e.g. 'stripe'. Take one from a search hit's company_slug or from freehire_search_companies — a display name such as 'Altinity, Inc.' matches nothing, and this facet has no distribution in freehire_get_job_facets. A company name by itself is not a source selection; use search_jobs_by_company for a company's own careers site."
		},
		"employment_type": {
			"type": "array",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["full_time", "part_time", "contract", "internship", "fellowship"]
			}
		},
		"relocation": {
			"type": "array",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["not_supported", "supported", "required"]
			}
		},
		"english_level": {
			"type": "array",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["none", "a1", "a2", "b1", "b2", "c1", "c2", "native"]
			}
		},
		"education_level": {
			"type": "array",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["none", "bachelor", "master", "phd"]
			}
		},
		"posting_language": {
			"type": "string",
			"description": "Comma-separated ISO 639-1 codes for the language the posting is written in, e.g. 'en,de'."
		},
		"domains": {
			"type": "array",
			"description": "Business domain of the hiring company, as stated by the posting.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["fintech", "crypto", "ecommerce", "gambling", "gamedev", "media", "travel", "healthcare", "edtech", "govtech", "devtools", "cybersecurity", "ai", "hrtech", "adtech", "proptech", "logistics", "mobility", "climatetech", "other"]
			}
		},
		"company_type": {
			"type": "array",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["product", "startup", "outsource", "outstaff", "agency", "inhouse", "government"]
			}
		},
		"company_size": {
			"type": "array",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["1-10", "11-50", "51-200", "201-500", "501-1000", "1000+"]
			}
		},
		"salary_currency": {
			"type": "string",
			"description": "Comma-separated ISO 4217 codes, e.g. 'USD,EUR'."
		},
		"salary_period": {
			"type": "array",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["year", "month", "day", "hour"]
			}
		},
		"visa_sponsorship": {
			"type": "boolean",
			"description": "Whether the posting STATES it sponsors a visa. false is a real stated value, not 'unknown' — postings that say nothing match neither setting."
		},
		"salary_min": {
			"type": "integer",
			"description": "Lower bound on the posting's minimum salary, in the posting's OWN salary_currency and salary_period — the comparison does not normalize currencies, so pair this with salary_currency and salary_period for a meaningful range. Most postings carry no salary at all and drop out when this is set."
		},
		"salary_max": {
			"type": "integer",
			"description": "Upper bound on the posting's maximum salary. Same currency caveat as salary_min."
		},
		"experience_years_min": {
			"type": "integer",
			"description": "Lower bound on the years of experience the posting asks for."
		},
		"posted_within_days": {
			"type": "integer",
			"description": "Restrict to postings published within the last N days.",
			"minimum": 1
		}`

// freehireSearchOnlyProps are the sort, paging, and exclude parameters
// GET /jobs/search takes and GET /jobs/facets does not.
const freehireSearchOnlyProps = `
		"sort": {
			"type": "string",
			"description": "Sort field. Omit for relevance on a text query, or posted_at descending on an empty one.",
			"enum": ["created_at", "posted_at", "salary_min", "salary_max"]
		},
		"order": {
			"type": "string",
			"description": "Sort direction. Ignored without a valid sort.",
			"enum": ["asc", "desc"]
		},
		"limit": {
			"type": "integer",
			"description": "Page size. Upstream clamps values above 100 rather than rejecting them, and defaults to 10.",
			"minimum": 1,
			"maximum": 100
		},
		"offset": {
			"type": "integer",
			"description": "Rows to skip. offset + limit may not exceed 10000; deeper paging is an upstream error.",
			"minimum": 0
		},
		"regions_exclude": {
			"type": "string",
			"description": "Comma-separated regions to exclude. Excludes AND together, unlike included geography."
		},
		"countries_exclude": {
			"type": "string",
			"description": "Comma-separated countries to exclude."
		},
		"work_mode_exclude": {
			"type": "string",
			"description": "Comma-separated work formats to exclude, e.g. 'onsite'."
		},
		"skills_exclude": {
			"type": "string",
			"description": "Comma-separated skill slugs to exclude."
		},
		"source_exclude": {
			"type": "string",
			"description": "Comma-separated source slugs to exclude."
		},
		"company_slug_exclude": {
			"type": "string",
			"description": "Comma-separated company slugs to exclude."
		}`

var freehireSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {` + freehireFilterProps + `,` + freehireSearchOnlyProps + `
	},
	"additionalProperties": false
}`)

var freehireSearchInputSchema = mustSchema(freehireSearchInputRawSchema)

var freehireFacetsInputRawSchema = []byte(`{
	"type": "object",
	"properties": {` + freehireFilterProps + `,
		"facets": {
			"type": "string",
			"description": "Comma-separated facet names to count, e.g. 'skills,seniority'. Omit for all of them, which runs to thousands of values across two dozen facets — cities, role, and skills carry about 1200 each. An unknown name is an error. Cannot be combined with disjunctive."
		},
		"disjunctive": {
			"type": "boolean",
			"description": "Count each facet under the whole filter MINUS its own selection, so a facet you already filtered on still shows what its other values would return — the numbers needed to decide which filter to relax. Without it that facet collapses to the single value you picked. Forces every facet to be counted, so it cannot be combined with facets."
		}
	},
	"additionalProperties": false
}`)

var freehireFacetsInputSchema = mustSchema(freehireFacetsInputRawSchema)

var freehireCompaniesInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"q": {
			"type": "string",
			"description": "Case-insensitive company name match. Optional — the filters below work on their own."
		},
		"limit": {
			"type": "integer",
			"description": "Page size. Upstream clamps values above 100 rather than rejecting them, and defaults to 10.",
			"minimum": 1,
			"maximum": 100
		},
		"offset": {
			"type": "integer",
			"description": "Companies to skip.",
			"minimum": 0
		},
		"sort": {
			"type": "string",
			"description": "Set to 'rating' to order by average feedback rating. Omit for the default order: open-role count descending, then name.",
			"enum": ["rating"]
		},
		"collections": {
			"type": "string",
			"description": "Comma-separated curated collection slugs, e.g. 'yc,unicorn'."
		},
		"regions": {
			"type": "string",
			"description": "Comma-separated regions the company posts roles in, e.g. 'eu,apac'."
		},
		"countries": {
			"type": "string",
			"description": "Comma-separated lowercase ISO 3166-1 alpha-2 codes the company posts roles in, e.g. 'gb,de'."
		},
		"remote_regions": {
			"type": "string",
			"description": "Comma-separated regions the company hires REMOTELY in, derived from its open postings — unlike regions, which counts any posting."
		},
		"industries": {
			"type": "string",
			"description": "Comma-separated curated company industry slugs, e.g. 'fintech,developer-tools'. There is no facet endpoint for this vocabulary; read real values off a known company with freehire_get_company_detail. An unrecognized slug matches nothing rather than erroring."
		},
		"domains": {
			"type": "string",
			"description": "Comma-separated job-derived domain slugs aggregated per company, e.g. 'fintech,devtools'."
		},
		"company_type": {
			"type": "array",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["product", "startup", "outsource", "outstaff", "agency", "inhouse", "government"]
			}
		},
		"company_size": {
			"type": "array",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["1-10", "11-50", "51-200", "201-500", "501-1000", "1000+"]
			}
		},
		"maturity": {
			"type": "string",
			"description": "Comma-separated curated company maturity stages, e.g. 'enterprise'. No facet endpoint covers this vocabulary; read real values off a known company with freehire_get_company_detail."
		},
		"yc_batch": {
			"type": "string",
			"description": "Comma-separated Y Combinator batches. Written out in full, e.g. 'Summer 2009' — NOT the abbreviated 'S09' or 'W21' form, which matches nothing. Read real values off a known YC company with freehire_get_company_detail."
		},
		"yc_status": {
			"type": "string",
			"description": "Comma-separated Y Combinator company statuses, e.g. 'Active'. Capitalized as shown."
		},
		"yc_stage": {
			"type": "string",
			"description": "Comma-separated Y Combinator funding stages, e.g. 'Growth'. Capitalized as shown."
		},
		"yc_flags": {
			"type": "string",
			"description": "Comma-separated Y Combinator directory flags, e.g. 'top_company,hiring'."
		}
	},
	"additionalProperties": false
}`)

var freehireCompaniesInputSchema = mustSchema(freehireCompaniesInputRawSchema)

var freehireCitiesInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"q": {
			"type": "string",
			"description": "Case-insensitive city name prefix or substring."
		},
		"country": {
			"type": "string",
			"description": "Restrict matches to one lowercase ISO 3166-1 alpha-2 country code, e.g. 'gb'. City names are not unique — London exists in both gb and ca."
		}
	},
	"additionalProperties": false
}`)

var freehireCitiesInputSchema = mustSchema(freehireCitiesInputRawSchema)

var freehireCompanyDetailInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"company_slug": {
			"type": "string",
			"description": "Catalogue company slug, e.g. 'stripe'. Take it from a freehire_search_companies result or a job's company_slug.",
			"minLength": 1
		},
		"limit": {
			"type": "integer",
			"description": "How many of the company's open jobs to return alongside its profile. Each carries its full description, so a large page is a large response. Upstream defaults to 20 here.",
			"minimum": 1,
			"maximum": 100
		},
		"offset": {
			"type": "integer",
			"description": "Jobs to skip.",
			"minimum": 0
		}
	},
	"required": ["company_slug"],
	"additionalProperties": false
}`)

var freehireCompanyDetailInputSchema = mustSchema(freehireCompanyDetailInputRawSchema)

// freehireFilters are the filters freehire_search_jobs and
// freehire_get_job_facets share. Field names match the upstream query
// parameters: a name that drifts from upstream's is silently dropped
// there rather than refused, so keeping them identical is the cheapest
// guard available.
type freehireFilters struct {
	Q                  string   `json:"q,omitempty"`
	Regions            []string `json:"regions,omitempty"`
	Countries          string   `json:"countries,omitempty"`
	Cities             string   `json:"cities,omitempty"`
	WorkMode           []string `json:"work_mode,omitempty"`
	Category           string   `json:"category,omitempty"`
	Role               string   `json:"role,omitempty"`
	Seniority          []string `json:"seniority,omitempty"`
	Skills             string   `json:"skills,omitempty"`
	SkillsMode         string   `json:"skills_mode,omitempty"`
	IsTech             []string `json:"is_tech,omitempty"`
	AIArchetype        []string `json:"ai_archetype,omitempty"`
	Collections        string   `json:"collections,omitempty"`
	Reality            []string `json:"reality,omitempty"`
	Source             string   `json:"source,omitempty"`
	CompanySlug        string   `json:"company_slug,omitempty"`
	EmploymentType     []string `json:"employment_type,omitempty"`
	Relocation         []string `json:"relocation,omitempty"`
	EnglishLevel       []string `json:"english_level,omitempty"`
	EducationLevel     []string `json:"education_level,omitempty"`
	PostingLanguage    string   `json:"posting_language,omitempty"`
	Domains            []string `json:"domains,omitempty"`
	CompanyType        []string `json:"company_type,omitempty"`
	CompanySize        []string `json:"company_size,omitempty"`
	SalaryCurrency     string   `json:"salary_currency,omitempty"`
	SalaryPeriod       []string `json:"salary_period,omitempty"`
	VisaSponsorship    *bool    `json:"visa_sponsorship,omitempty"`
	SalaryMin          *int     `json:"salary_min,omitempty"`
	SalaryMax          *int     `json:"salary_max,omitempty"`
	ExperienceYearsMin *int     `json:"experience_years_min,omitempty"`
	PostedWithinDays   *int     `json:"posted_within_days,omitempty"`
}

type freehireSearchInput struct {
	freehireFilters
	Sort               string `json:"sort,omitempty"`
	Order              string `json:"order,omitempty"`
	Limit              *int   `json:"limit,omitempty"`
	Offset             *int   `json:"offset,omitempty"`
	RegionsExclude     string `json:"regions_exclude,omitempty"`
	CountriesExclude   string `json:"countries_exclude,omitempty"`
	WorkModeExclude    string `json:"work_mode_exclude,omitempty"`
	SkillsExclude      string `json:"skills_exclude,omitempty"`
	SourceExclude      string `json:"source_exclude,omitempty"`
	CompanySlugExclude string `json:"company_slug_exclude,omitempty"`
}

type freehireFacetsInput struct {
	freehireFilters
	Facets      string `json:"facets,omitempty"`
	Disjunctive *bool  `json:"disjunctive,omitempty"`
}

type freehireCompaniesInput struct {
	Q             string   `json:"q,omitempty"`
	Limit         *int     `json:"limit,omitempty"`
	Offset        *int     `json:"offset,omitempty"`
	Sort          string   `json:"sort,omitempty"`
	Collections   string   `json:"collections,omitempty"`
	Regions       string   `json:"regions,omitempty"`
	Countries     string   `json:"countries,omitempty"`
	RemoteRegions string   `json:"remote_regions,omitempty"`
	Industries    string   `json:"industries,omitempty"`
	Domains       string   `json:"domains,omitempty"`
	CompanyType   []string `json:"company_type,omitempty"`
	CompanySize   []string `json:"company_size,omitempty"`
	Maturity      string   `json:"maturity,omitempty"`
	YcBatch       string   `json:"yc_batch,omitempty"`
	YcStatus      string   `json:"yc_status,omitempty"`
	YcStage       string   `json:"yc_stage,omitempty"`
	YcFlags       string   `json:"yc_flags,omitempty"`
}

type freehireCitiesInput struct {
	Q       string `json:"q,omitempty"`
	Country string `json:"country,omitempty"`
}

type freehireCompanyDetailInput struct {
	CompanySlug string `json:"company_slug"`
	Limit       *int   `json:"limit,omitempty"`
	Offset      *int   `json:"offset,omitempty"`
}

type freehireDetailInput struct {
	JobID string `json:"job_id" jsonschema:"public_slug from a freehire_search_jobs result."`
}

// freehireJob mirrors the upstream Job schema, which the search and
// detail endpoints both serve. Only description differs between them:
// search sends a preview upstream truncates near 1000 characters, and
// the detail endpoint the full body.
type freehireJob struct {
	PublicSlug  string `json:"public_slug" jsonschema:"Stable identifier; pass it to freehire_get_job_detail's job_id param."`
	Title       string `json:"title"`
	Company     string `json:"company"`
	CompanySlug string `json:"company_slug,omitempty" jsonschema:"Catalogue slug; pass it as freehire_search_jobs's company_slug filter to see the rest of this company's postings."`
	URL         string `json:"url,omitempty" jsonschema:"The employer's own application URL — where an applicant actually goes. Authoritative; the rest of this record is freehire's crawled snapshot."`

	Location      string   `json:"location,omitempty" jsonschema:"The posting's own location string, verbatim and unnormalized. Filter on the geography parameters instead."`
	Regions       []string `json:"regions,omitempty"`
	Countries     []string `json:"countries,omitempty"`
	Cities        []string `json:"cities,omitempty"`
	WorkMode      string   `json:"work_mode,omitempty" jsonschema:"Absent when the posting's format could not be resolved."`
	Skills        []string `json:"skills,omitempty"`
	Collections   []string `json:"collections,omitempty" jsonschema:"Curated collection slugs of the posting's company."`
	IsTech        string   `json:"is_tech,omitempty" jsonschema:"Absent when unknown."`
	Source        string   `json:"source,omitempty" jsonschema:"Origin slug — the ATS or board this posting was crawled from."`
	ExternalID    string   `json:"external_id,omitempty" jsonschema:"The source's own identifier for the posting."`
	ManuallyAdded *bool    `json:"manually_added,omitempty" jsonschema:"True for a hand-curated posting rather than an automated crawl."`

	PostedAt   string `json:"posted_at,omitempty" jsonschema:"The employer's own date."`
	CreatedAt  string `json:"created_at,omitempty" jsonschema:"When freehire first saw the posting."`
	UpdatedAt  string `json:"updated_at,omitempty"`
	LastSeenAt string `json:"last_seen_at,omitempty" jsonschema:"When a re-crawl last confirmed the posting was still live."`
	ClosedAt   string `json:"closed_at,omitempty" jsonschema:"Set once the posting is no longer open. Only freehire_get_job_detail ever returns such a row."`

	Enrichment        map[string]any `json:"enrichment,omitempty" jsonschema:"freehire's model-extracted attributes — summary, employment_type, salary_min/max/currency/period, seniority, experience_years_min, english_level, education_level, category, domains, posting_language, company_type, company_size, relocation, visa_sponsorship. Every field is omitted when the posting did not state it, so its keys vary by posting. These are the values the matching search filters compare against."`
	EnrichedAt        string         `json:"enriched_at,omitempty"`
	EnrichmentVersion *int           `json:"enrichment_version,omitempty" jsonschema:"Bumped when freehire's enrichment contract changes."`
	Reality           map[string]any `json:"reality,omitempty" jsonschema:"Freshness signals, present on every posting: class (fresh/stale/likely-evergreen), age_days, repost_count, mass_posting_count, fake_freshness. A high repost_count or mass_posting_count marks a recycled or mass-blasted ad."`
	Ghost             map[string]any `json:"ghost,omitempty" jsonschema:"Hedged 'is anyone actually hiring for this' verdict — level, criteria, criteria_total, contributors, ats_checked_at. Raised only when evidence fires, so ABSENT MEANS 'no signal', never 'verified real'. Read criteria against criteria_total. freehire_search_jobs and freehire_get_job_detail carry it; freehire_get_company_detail's jobs do not, so its absence there is not evidence either way."`

	ViewCount     *int `json:"view_count,omitempty"`
	AppliedCount  *int `json:"applied_count,omitempty" jsonschema:"How many freehire users reported applying — a rough proxy for how contested the role is."`
	UpvoteCount   *int `json:"upvote_count,omitempty"`
	DownvoteCount *int `json:"downvote_count,omitempty"`

	Description string `json:"description,omitempty" jsonschema:"Plain text, converted from the stored HTML. freehire_search_jobs returns the preview upstream truncates near 1000 characters; freehire_get_job_detail returns the full body."`
}

// freehirePagination mirrors the upstream meta block. limit is what
// upstream actually applied after clamping, which can differ from what
// was asked for.
type freehirePagination struct {
	Total  *int `json:"total,omitempty" jsonschema:"Total matching rows. Exact on the job endpoints; a planner estimate on a completely unfiltered company search."`
	Limit  *int `json:"limit,omitempty" jsonschema:"The page size upstream actually applied, after clamping."`
	Offset *int `json:"offset,omitempty"`
}

type freehireSearchOutput struct {
	freehirePagination
	Data []freehireJob `json:"data"`
}

type freehireFacetsOutput struct {
	Total  int                           `json:"total"`
	Facets map[string]map[string]int     `json:"facets" jsonschema:"Facet name to a map of value to matching-posting count. The values are canonical — pass them verbatim to the matching freehire_search_jobs filter."`
	Stats  map[string]map[string]float64 `json:"stats,omitempty" jsonschema:"Numeric min/max ranges for the continuous facets: salary_min, salary_max, experience_years_min."`
}

type freehireCompaniesOutput struct {
	freehirePagination
	Data []map[string]any `json:"data" jsonschema:"Each row carries slug, name, job_count, tagline, industries, hq_country, collections, and feedback counts. Pass slug as freehire_search_jobs's company_slug filter, or to freehire_get_company_detail."`
}

type freehireCitiesOutput struct {
	Data []freehireCity `json:"data"`
}

type freehireCity struct {
	Value   string `json:"value" jsonschema:"Canonical city name — pass it verbatim to the cities filter."`
	Country string `json:"country" jsonschema:"Lowercase ISO 3166-1 alpha-2."`
}

type freehireCompanyDetailOutput struct {
	Company           map[string]any `json:"company" jsonschema:"The company profile as freehire holds it: tagline, company_info, industries, domains, collections, regions, countries, remote_regions, company_types, company_sizes, hq_country, year_founded, employee_count, organization_type, maturity, yc_batch, yc_status, yc_stage, yc_flags, and feedback counts. This is the only place the industries, maturity, and yc_* vocabularies can be read, so use it to learn real values before filtering freehire_search_companies on them."`
	Jobs              []freehireJob  `json:"jobs,omitempty" jsonschema:"A page of the company's open jobs with full descriptions. This list takes no filters and no sort; use freehire_search_jobs with company_slug to filter or sort them."`
	ReferralAvailable *bool          `json:"referral_available,omitempty" jsonschema:"Whether an employee referral offer exists for this company."`
}

func freehireSplitCSV(s string) []string {
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

// freehireEnumFilters holds the shared filters upstream types as enums.
// Both /jobs/search and /jobs/facets now name the same component for
// each, so one conversion serves both.
type freehireEnumFilters struct {
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

func freehireParseFilters(in freehireFilters) (freehireEnumFilters, error) {
	var f freehireEnumFilters
	var err error
	if f.regions, err = freehireEnums[freehire.RegionsItem](in.Regions, "regions"); err != nil {
		return f, err
	}
	if f.workMode, err = freehireEnums[freehire.WorkModeItem](in.WorkMode, "work_mode"); err != nil {
		return f, err
	}
	if f.seniority, err = freehireEnums[freehire.SeniorityItem](in.Seniority, "seniority"); err != nil {
		return f, err
	}
	if f.isTech, err = freehireEnums[freehire.IsTechItem](in.IsTech, "is_tech"); err != nil {
		return f, err
	}
	if f.aiArchetype, err = freehireEnums[freehire.AIArchetypeItem](in.AIArchetype, "ai_archetype"); err != nil {
		return f, err
	}
	if f.reality, err = freehireEnums[freehire.RealityItem](in.Reality, "reality"); err != nil {
		return f, err
	}
	if f.employmentType, err = freehireEnums[freehire.EmploymentTypeItem](in.EmploymentType, "employment_type"); err != nil {
		return f, err
	}
	if f.relocation, err = freehireEnums[freehire.RelocationItem](in.Relocation, "relocation"); err != nil {
		return f, err
	}
	if f.englishLevel, err = freehireEnums[freehire.EnglishLevelItem](in.EnglishLevel, "english_level"); err != nil {
		return f, err
	}
	if f.educationLevel, err = freehireEnums[freehire.EducationLevelItem](in.EducationLevel, "education_level"); err != nil {
		return f, err
	}
	if f.domains, err = freehireEnums[freehire.DomainsItem](in.Domains, "domains"); err != nil {
		return f, err
	}
	if f.companyType, err = freehireEnums[freehire.CompanyTypeItem](in.CompanyType, "company_type"); err != nil {
		return f, err
	}
	if f.companySize, err = freehireEnums[freehire.CompanySizeItem](in.CompanySize, "company_size"); err != nil {
		return f, err
	}
	if f.salaryPeriod, err = freehireEnums[freehire.SalaryPeriodItem](in.SalaryPeriod, "salary_period"); err != nil {
		return f, err
	}
	if in.SkillsMode != "" {
		mode, err := freehireEnumOne[freehire.SkillsMode](in.SkillsMode, "skills_mode")
		if err != nil {
			return f, err
		}
		f.skillsMode = freehire.NewOptSkillsMode(mode)
	}
	return f, nil
}

func freehireOptInt(v *int) freehire.OptInt {
	if v == nil {
		return freehire.OptInt{}
	}
	return freehire.NewOptInt(*v)
}

func freehireOptBool(v *bool) freehire.OptBool {
	if v == nil {
		return freehire.OptBool{}
	}
	return freehire.NewOptBool(*v)
}

func freehireOptString(v string) freehire.OptString {
	if v == "" {
		return freehire.OptString{}
	}
	return freehire.NewOptString(v)
}

func freehireSearchParams(in *freehireSearchInput) (freehire.SearchJobsParams, error) {
	f, err := freehireParseFilters(in.freehireFilters)
	if err != nil {
		return freehire.SearchJobsParams{}, err
	}
	params := freehire.SearchJobsParams{
		Q:                  freehireOptString(in.Q),
		Limit:              freehireOptInt(in.Limit),
		Offset:             freehireOptInt(in.Offset),
		Regions:            f.regions,
		Countries:          freehireSplitCSV(in.Countries),
		Cities:             freehireSplitCSV(in.Cities),
		WorkMode:           f.workMode,
		Category:           freehireSplitCSV(in.Category),
		Role:               freehireSplitCSV(in.Role),
		Seniority:          f.seniority,
		Skills:             freehireSplitCSV(in.Skills),
		SkillsMode:         f.skillsMode,
		IsTech:             f.isTech,
		AiArchetype:        f.aiArchetype,
		Collections:        freehireSplitCSV(in.Collections),
		Reality:            f.reality,
		Source:             freehireSplitCSV(in.Source),
		CompanySlug:        freehireSplitCSV(in.CompanySlug),
		EmploymentType:     f.employmentType,
		Relocation:         f.relocation,
		EnglishLevel:       f.englishLevel,
		EducationLevel:     f.educationLevel,
		PostingLanguage:    freehireSplitCSV(in.PostingLanguage),
		Domains:            f.domains,
		CompanyType:        f.companyType,
		CompanySize:        f.companySize,
		SalaryCurrency:     freehireSplitCSV(in.SalaryCurrency),
		SalaryPeriod:       f.salaryPeriod,
		VisaSponsorship:    freehireOptBool(in.VisaSponsorship),
		SalaryMin:          freehireOptInt(in.SalaryMin),
		SalaryMax:          freehireOptInt(in.SalaryMax),
		ExperienceYearsMin: freehireOptInt(in.ExperienceYearsMin),
		PostedWithinDays:   freehireOptInt(in.PostedWithinDays),
		RegionsExclude:     freehireSplitCSV(in.RegionsExclude),
		CountriesExclude:   freehireSplitCSV(in.CountriesExclude),
		WorkModeExclude:    freehireSplitCSV(in.WorkModeExclude),
		SkillsExclude:      freehireSplitCSV(in.SkillsExclude),
		SourceExclude:      freehireSplitCSV(in.SourceExclude),
		CompanySlugExclude: freehireSplitCSV(in.CompanySlugExclude),
	}
	if in.Sort != "" {
		field, err := freehireEnumOne[freehire.Sort](in.Sort, "sort")
		if err != nil {
			return freehire.SearchJobsParams{}, err
		}
		params.Sort = freehire.NewOptSort(field)
	}
	if in.Order != "" {
		order, err := freehireEnumOne[freehire.Order](in.Order, "order")
		if err != nil {
			return freehire.SearchJobsParams{}, err
		}
		params.Order = freehire.NewOptOrder(order)
	}
	return params, nil
}

func freehireFacetsParams(in *freehireFacetsInput) (freehire.GetJobFacetsParams, error) {
	f, err := freehireParseFilters(in.freehireFilters)
	if err != nil {
		return freehire.GetJobFacetsParams{}, err
	}
	return freehire.GetJobFacetsParams{
		Q:                  freehireOptString(in.Q),
		Facets:             freehireOptString(in.Facets),
		Disjunctive:        freehireOptBool(in.Disjunctive),
		Regions:            f.regions,
		Countries:          freehireSplitCSV(in.Countries),
		Cities:             freehireSplitCSV(in.Cities),
		WorkMode:           f.workMode,
		Category:           freehireSplitCSV(in.Category),
		Role:               freehireSplitCSV(in.Role),
		Seniority:          f.seniority,
		Skills:             freehireSplitCSV(in.Skills),
		SkillsMode:         f.skillsMode,
		IsTech:             f.isTech,
		AiArchetype:        f.aiArchetype,
		Collections:        freehireSplitCSV(in.Collections),
		Reality:            f.reality,
		Source:             freehireSplitCSV(in.Source),
		CompanySlug:        freehireSplitCSV(in.CompanySlug),
		EmploymentType:     f.employmentType,
		Relocation:         f.relocation,
		EnglishLevel:       f.englishLevel,
		EducationLevel:     f.educationLevel,
		PostingLanguage:    freehireSplitCSV(in.PostingLanguage),
		Domains:            f.domains,
		CompanyType:        f.companyType,
		CompanySize:        f.companySize,
		SalaryCurrency:     freehireSplitCSV(in.SalaryCurrency),
		SalaryPeriod:       f.salaryPeriod,
		VisaSponsorship:    freehireOptBool(in.VisaSponsorship),
		SalaryMin:          freehireOptInt(in.SalaryMin),
		SalaryMax:          freehireOptInt(in.SalaryMax),
		ExperienceYearsMin: freehireOptInt(in.ExperienceYearsMin),
		PostedWithinDays:   freehireOptInt(in.PostedWithinDays),
	}, nil
}

func freehireCompaniesParams(in *freehireCompaniesInput) (freehire.SearchCompaniesParams, error) {
	companyType, err := freehireEnums[freehire.SearchCompaniesCompanyTypeItem](in.CompanyType, "company_type")
	if err != nil {
		return freehire.SearchCompaniesParams{}, err
	}
	companySize, err := freehireEnums[freehire.SearchCompaniesCompanySizeItem](in.CompanySize, "company_size")
	if err != nil {
		return freehire.SearchCompaniesParams{}, err
	}
	params := freehire.SearchCompaniesParams{
		Q:             freehireOptString(in.Q),
		Limit:         freehireOptInt(in.Limit),
		Offset:        freehireOptInt(in.Offset),
		Collections:   freehireSplitCSV(in.Collections),
		Regions:       freehireSplitCSV(in.Regions),
		Countries:     freehireSplitCSV(in.Countries),
		RemoteRegions: freehireSplitCSV(in.RemoteRegions),
		Industries:    freehireSplitCSV(in.Industries),
		Domains:       freehireSplitCSV(in.Domains),
		CompanyType:   companyType,
		CompanySize:   companySize,
		Maturity:      freehireSplitCSV(in.Maturity),
		YcBatch:       freehireSplitCSV(in.YcBatch),
		YcStatus:      freehireSplitCSV(in.YcStatus),
		YcStage:       freehireSplitCSV(in.YcStage),
		YcFlags:       freehireSplitCSV(in.YcFlags),
	}
	if in.Sort != "" {
		sort, err := freehireEnumOne[freehire.SearchCompaniesSort](in.Sort, "sort")
		if err != nil {
			return freehire.SearchCompaniesParams{}, err
		}
		params.Sort = freehire.NewOptSearchCompaniesSort(sort)
	}
	return params, nil
}

// freehireIgnoredParams turns upstream's dropped-parameter report into
// an error. Upstream ignores a parameter no filter reads instead of
// refusing it, and answers with the unfiltered catalogue — a result
// indistinguishable from a genuine one. Every parameter name here is
// hardcoded and the tool schemas forbid extras, so a report can only
// mean this mapping has drifted from openapi.yaml, and returning the
// unfiltered page as if it were filtered is the one outcome worth
// failing loudly over.
func freehireIgnoredParams(ignored []freehire.IgnoredParam) error {
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
	return fmt.Errorf("freehire dropped %d parameter(s) and answered with an unfiltered result: %s — this openings-mcp build has drifted from freehire's openapi.yaml", len(ignored), strings.Join(names, ", "))
}

// freehirePassthrough re-encodes one of the spec's open-ended objects as
// a plain map. The spec names their fields but still allows extras, so
// re-encoding forwards every key upstream sent rather than only the ones
// this build knows about.
func freehirePassthrough(v json.Marshaler) map[string]any {
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

func freehireIntPtr(v interface{ Get() (int, bool) }) *int {
	if n, ok := v.Get(); ok {
		return &n
	}
	return nil
}

// freehireText converts a stored description to plain text. Upstream
// serves HTML on every endpoint, and the search preview is cut at a
// fixed length without closing its open tags, so the caller would
// otherwise have to read broken markup.
func freehireText(desc string) string {
	if desc == "" {
		return ""
	}
	if text, err := html2text.FromString(desc, html2text.Options{}); err == nil {
		return text
	}
	return desc
}

func freehireJobOf(j freehire.Job) freehireJob {
	out := freehireJob{
		PublicSlug:    j.PublicSlug,
		Title:         j.Title,
		Company:       j.Company,
		CompanySlug:   j.CompanySlug.Or(""),
		Location:      j.Location.Or(""),
		Regions:       j.Regions,
		Countries:     j.Countries,
		Cities:        j.Cities,
		WorkMode:      string(j.WorkMode.Or("")),
		Skills:        j.Skills,
		Collections:   j.Collections,
		IsTech:        string(j.IsTech.Or("")),
		Source:        j.Source.Or(""),
		ExternalID:    j.ExternalID.Or(""),
		PostedAt:      freehireDate(j.PostedAt),
		CreatedAt:     freehireDate(j.CreatedAt),
		UpdatedAt:     freehireDate(j.UpdatedAt),
		LastSeenAt:    freehireDate(j.LastSeenAt),
		ClosedAt:      freehireDate(j.ClosedAt),
		EnrichedAt:    freehireDate(j.EnrichedAt),
		ViewCount:     freehireIntPtr(j.ViewCount),
		AppliedCount:  freehireIntPtr(j.AppliedCount),
		UpvoteCount:   freehireIntPtr(j.UpvoteCount),
		DownvoteCount: freehireIntPtr(j.DownvoteCount),
		Description:   freehireText(j.Description.Or("")),
	}
	if u, ok := j.URL.Get(); ok {
		out.URL = u.String()
	}
	if added, ok := j.ManuallyAdded.Get(); ok {
		out.ManuallyAdded = &added
	}
	out.EnrichmentVersion = freehireIntPtr(j.EnrichmentVersion)
	if e, ok := j.Enrichment.Get(); ok {
		out.Enrichment = freehirePassthrough(&e)
	}
	if r, ok := j.Reality.Get(); ok {
		out.Reality = freehirePassthrough(&r)
	}
	if g, ok := j.Ghost.Get(); ok {
		out.Ghost = freehirePassthrough(&g)
	}
	return out
}

func freehireJobsOf(jobs []freehire.Job) []freehireJob {
	out := make([]freehireJob, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, freehireJobOf(j))
	}
	return out
}

func freehirePaginationOf(meta freehire.OptPaginationMeta) freehirePagination {
	var out freehirePagination
	m, ok := meta.Get()
	if !ok {
		return out
	}
	out.Total = freehireIntPtr(m.Total)
	out.Limit = freehireIntPtr(m.Limit)
	out.Offset = freehireIntPtr(m.Offset)
	return out
}

func freehireSearchOutputOf(res *freehire.JobListEnvelope) (*freehireSearchOutput, error) {
	if meta, ok := res.Meta.Get(); ok {
		if err := freehireIgnoredParams(meta.IgnoredParams); err != nil {
			return nil, err
		}
	}
	return &freehireSearchOutput{
		freehirePagination: freehirePaginationOf(res.Meta),
		Data:               freehireJobsOf(res.Data),
	}, nil
}

func freehireFacetsOutputOf(res *freehire.FacetsEnvelope) (*freehireFacetsOutput, error) {
	if meta, ok := res.Meta.Get(); ok {
		if err := freehireIgnoredParams(meta.IgnoredParams); err != nil {
			return nil, err
		}
	}
	out := &freehireFacetsOutput{
		Total:  res.Data.Total,
		Facets: make(map[string]map[string]int, len(res.Data.Facets)),
	}
	for name, vals := range res.Data.Facets {
		out.Facets[name] = vals
	}
	if stats, ok := res.Data.Stats.Get(); ok {
		out.Stats = make(map[string]map[string]float64, len(stats))
		for name, vals := range stats {
			out.Stats[name] = vals
		}
	}
	return out, nil
}

func freehireCompaniesOutputOf(res *freehire.SearchCompaniesOK) (*freehireCompaniesOutput, error) {
	if meta, ok := res.Meta.Get(); ok {
		if err := freehireIgnoredParams(meta.IgnoredParams); err != nil {
			return nil, err
		}
	}
	out := &freehireCompaniesOutput{
		freehirePagination: freehirePaginationOf(res.Meta),
		Data:               make([]map[string]any, 0, len(res.Data)),
	}
	for _, c := range res.Data {
		out.Data = append(out.Data, freehirePassthrough(&c))
	}
	return out, nil
}

func freehireCompanyDetailOutputOf(res *freehire.GetCompanyOK) *freehireCompanyDetailOutput {
	out := &freehireCompanyDetailOutput{Jobs: freehireJobsOf(res.Data.Jobs)}
	if c, ok := res.Data.Company.Get(); ok {
		out.Company = freehirePassthrough(&c)
	}
	if avail, ok := res.Data.ReferralAvailable.Get(); ok {
		out.ReferralAvailable = &avail
	}
	return out
}

// RegisterFreehire registers freehire.me's job search, facet, company,
// city, and job-detail tools.
func RegisterFreehire(s *mcp.Server, c *freehire.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "freehire_search_jobs",
		Description: "Search IT/tech jobs on freehire.me, a catalogue of over a million postings crawled from company ATS boards and job boards. Every filter and the sort run server-side. Resolve open-vocabulary values first: skills, category, role, source, collections, and countries come from freehire_get_job_facets; cities from freehire_search_cities; company_slug from freehire_search_companies. An unrecognized VALUE matches nothing rather than erroring, so a zero-result search usually means a bad value. Most of the catalogue is stale, so reality=['fresh'] is the cheapest quality filter. Complements search_jobs_by_company rather than replacing it — use this when the user names freehire or wants cross-company IT search.",
		Annotations: &mcp.ToolAnnotations{Title: "Search freehire.me jobs", ReadOnlyHint: true},
		InputSchema: freehireSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *freehireSearchInput) (*mcp.CallToolResult, *freehireSearchOutput, error) {
		params, err := freehireSearchParams(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.SearchJobs(ctx, params)
		if err != nil {
			return errorResult(err), nil, nil
		}
		out, err := freehireSearchOutputOf(res)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "freehire_get_job_facets",
		Description: "Get freehire.me's live filter vocabulary with counts. Call it before freehire_search_jobs whenever a value is uncertain — skills, category, role, source, collections, countries, and posting_language are open or generated vocabularies where a wrong value silently matches nothing. It takes the same filters as the search, so the counts describe the slice you are about to search. Narrow with facets= : the full answer runs to thousands of values across two dozen facets. Set disjunctive to see what a facet you already filtered on would return at its other values, which tells you which filter to relax. There is no company_slug distribution — use freehire_search_companies.",
		Annotations: &mcp.ToolAnnotations{Title: "Get freehire.me search facets", ReadOnlyHint: true},
		InputSchema: freehireFacetsInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *freehireFacetsInput) (*mcp.CallToolResult, *freehireFacetsOutput, error) {
		params, err := freehireFacetsParams(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.GetJobFacets(ctx, params)
		if err != nil {
			return errorResult(err), nil, nil
		}
		out, err := freehireFacetsOutputOf(res)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "freehire_search_companies",
		Description: "Search companies that currently have at least one open role on freehire.me, most active first. Use it to resolve a company name to the company_slug that freehire_search_jobs filters on — names are not slugs, 'Altinity, Inc.' is altinity-inc — or to find companies by collection, industry, geography, type, size, maturity, or Y Combinator batch. Several companies can share a name; compare job_count before picking. The industries, maturity, and yc_* vocabularies have no facet endpoint, so read real values off a known company with freehire_get_company_detail first.",
		Annotations: &mcp.ToolAnnotations{Title: "Search freehire.me companies", ReadOnlyHint: true},
		InputSchema: freehireCompaniesInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *freehireCompaniesInput) (*mcp.CallToolResult, *freehireCompaniesOutput, error) {
		params, err := freehireCompaniesParams(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.SearchCompanies(ctx, params)
		if err != nil {
			return errorResult(err), nil, nil
		}
		out, err := freehireCompaniesOutputOf(res)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "freehire_get_company_detail",
		Description: "Get one freehire.me company's full profile plus a page of its open jobs. The profile carries fields no other tool returns — company_info, year_founded, employee_count, organization_type, maturity, remote_regions, industries, and the yc_batch/yc_status/yc_stage/yc_flags directory values — which makes it the only way to learn what those vocabularies actually contain before filtering freehire_search_companies on them. The jobs it returns take no filters and no sort and carry their full descriptions, so use freehire_search_jobs with company_slug to filter, sort, or page through a company's postings.",
		Annotations: &mcp.ToolAnnotations{Title: "Get freehire.me company details", ReadOnlyHint: true},
		InputSchema: freehireCompanyDetailInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *freehireCompanyDetailInput) (*mcp.CallToolResult, *freehireCompanyDetailOutput, error) {
		if in.CompanySlug == "" {
			return errorResult(fmt.Errorf("company_slug is required (take it from a freehire_search_companies result)")), nil, nil
		}
		res, err := c.GetCompany(ctx, freehire.GetCompanyParams{
			Slug:   in.CompanySlug,
			Limit:  freehireOptInt(in.Limit),
			Offset: freehireOptInt(in.Offset),
		})
		if err != nil {
			return errorResult(fmt.Errorf("company %q not found", in.CompanySlug)), nil, nil
		}
		return nil, freehireCompanyDetailOutputOf(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "freehire_search_cities",
		Description: "Resolve a city name to the canonical value freehire_search_jobs's cities filter expects. The filter holds display names such as London or Berlin and matches nothing on a near miss, so resolve here first. City names are not unique — London exists in both gb and ca — and the cities filter carries no country qualifier, so pair it with countries only if you accept that the two widen the search together rather than narrowing it.",
		Annotations: &mcp.ToolAnnotations{Title: "Resolve freehire.me city names", ReadOnlyHint: true},
		InputSchema: freehireCitiesInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *freehireCitiesInput) (*mcp.CallToolResult, *freehireCitiesOutput, error) {
		res, err := c.SearchCities(ctx, freehire.SearchCitiesParams{
			Q:       freehireOptString(in.Q),
			Country: freehireOptString(in.Country),
		})
		if err != nil {
			return errorResult(err), nil, nil
		}
		out := &freehireCitiesOutput{Data: make([]freehireCity, 0, len(res.Data))}
		for _, city := range res.Data {
			out.Data = append(out.Data, freehireCity{Value: city.Value, Country: city.Country})
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "freehire_get_job_detail",
		Description: "Get one freehire.me posting in full by its public_slug: the whole description, plus the ghost signal. This is the only freehire tool that returns a posting that has already closed, which shows up as a set closed_at.",
		Annotations: &mcp.ToolAnnotations{Title: "Get freehire.me job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *freehireDetailInput) (*mcp.CallToolResult, *freehireJob, error) {
		if in.JobID == "" {
			return errorResult(fmt.Errorf("job_id is required (take it from a freehire_search_jobs result)")), nil, nil
		}
		res, err := c.GetJob(ctx, freehire.GetJobParams{Slug: in.JobID})
		if err != nil {
			return errorResult(fmt.Errorf("job %q not found", in.JobID)), nil, nil
		}
		job := freehireJobOf(res.Data)
		return nil, &job, nil
	})
}
