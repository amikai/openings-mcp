package openingsmcp

import (
	"context"
	"fmt"

	"github.com/amikai/openings-mcp/internal/provider/indeed"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var indeedSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text search query: role titles, skills, or technologies."
		},
		"location": {
			"type": "string",
			"description": "Free-text location, e.g. 'Taipei'. Must correspond to country (e.g. don't pair 'Taipei' with country 'United States') or results are wrong or empty."
		},
		"country": {
			"type": "string",
			"description": "Country name selecting Indeed's country catalogue and site domain, e.g. 'Taiwan', 'United States', 'Japan'. Defaults to Taiwan when omitted."
		},
		"radius_miles": {
			"type": "integer",
			"description": "Search radius in miles around location. Defaults to 25.",
			"minimum": 0
		},
		"cursor": {
			"type": "string",
			"description": "Pagination cursor from a previous search's next_cursor. Omit for the first page."
		},
		"hours_old": {
			"type": "integer",
			"description": "Only jobs posted within this many hours. Mutually exclusive with job_type/remote/easy_apply: when set, those are ignored.",
			"minimum": 1
		},
		"job_type": {
			"type": "string",
			"description": "Job type filter. Ignored if hours_old or easy_apply is set.",
			"enum": ["Full-time", "Part-time", "Contract", "Internship"]
		},
		"remote": {
			"type": "boolean",
			"description": "Only remote jobs. Ignored if hours_old or easy_apply is set."
		},
		"easy_apply": {
			"type": "boolean",
			"description": "Only Easy Apply jobs. Takes precedence over job_type/remote; ignored if hours_old is set."
		}
	},
	"additionalProperties": false
}`)

// indeedSearchInputSchema is hand-written JSON kept aligned with the
// jobSearch GraphQL fields query.go builds: human labels instead of the
// site's raw filter codes (job_type maps back via indeed.JobTypeIDs).
var indeedSearchInputSchema = mustSchema(indeedSearchInputRawSchema)

type indeedSearchInput struct {
	Keyword     string `json:"keyword,omitempty"`
	Location    string `json:"location,omitempty"`
	Country     string `json:"country,omitempty"`
	RadiusMiles int    `json:"radius_miles,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	HoursOld    int    `json:"hours_old,omitempty"`
	JobType     string `json:"job_type,omitempty"`
	Remote      bool   `json:"remote,omitempty"`
	EasyApply   bool   `json:"easy_apply,omitempty"`
}

func indeedMCPToHTTPRequest(in *indeedSearchInput) (*indeed.JobsRequest, error) {
	req := &indeed.JobsRequest{
		Keywords:    in.Keyword,
		Location:    in.Location,
		Country:     in.Country,
		RadiusMiles: in.RadiusMiles,
		Cursor:      in.Cursor,
		HoursOld:    in.HoursOld,
		Remote:      in.Remote,
		EasyApply:   in.EasyApply,
	}
	if in.JobType != "" {
		id, ok := indeed.JobTypeIDs[in.JobType]
		if !ok {
			return nil, fmt.Errorf("invalid job_type %q", in.JobType)
		}
		req.JobType = id
	}
	return req, nil
}

type indeedCompensation struct {
	MinAmount int    `json:"min_amount,omitempty"`
	MaxAmount int    `json:"max_amount,omitempty"`
	Currency  string `json:"currency,omitempty"`
	Interval  string `json:"interval,omitempty"`
}

func indeedCompensationFromHTTP(c *indeed.Compensation) *indeedCompensation {
	if c == nil {
		return nil
	}
	return &indeedCompensation{MinAmount: c.MinAmount, MaxAmount: c.MaxAmount, Currency: c.Currency, Interval: c.Interval}
}

type indeedJobSummary struct {
	Key        string   `json:"key" jsonschema:"Opaque Indeed job key; pass to indeed_get_job_detail's job_key param."`
	Title      string   `json:"title"`
	Company    string   `json:"company,omitempty"`
	CompanyURL string   `json:"company_url,omitempty"`
	Location   string   `json:"location,omitempty"`
	URL        string   `json:"url,omitempty" jsonschema:"Public job posting URL."`
	PostedDate string   `json:"posted_date,omitempty"`
	JobTypes   []string `json:"job_types,omitempty" jsonschema:"Indeed's own labels, e.g. 'Full-time', 'Permanent'; not filtered to a fixed enum."`

	Compensation *indeedCompensation `json:"compensation,omitempty"`
}

type indeedSearchOutput struct {
	Data       []indeedJobSummary `json:"data"`
	NextCursor string             `json:"next_cursor,omitempty" jsonschema:"Pass to indeed_search_jobs's cursor param to fetch the next page; absent means no more results."`
}

func indeedHTTPToMCPResponse(resp *indeed.JobsResponse) *indeedSearchOutput {
	out := &indeedSearchOutput{Data: make([]indeedJobSummary, 0, len(resp.Jobs)), NextCursor: resp.NextCursor}
	for _, j := range resp.Jobs {
		out.Data = append(out.Data, indeedJobSummary{
			Key:          j.Key,
			Title:        j.Title,
			Company:      j.Company,
			CompanyURL:   j.CompanyURL,
			Location:     j.Location,
			URL:          j.JobURL,
			PostedDate:   j.PostedDate,
			JobTypes:     j.JobTypes,
			Compensation: indeedCompensationFromHTTP(j.Compensation),
		})
	}
	return out
}

type indeedDetailInput struct {
	JobKey  string `json:"job_key" jsonschema:"Opaque Indeed job key (key from indeed_search_jobs results)."`
	Country string `json:"country,omitempty" jsonschema:"Country used to resolve the job's site domain for its URL; should match the country used in the original search. Defaults to Taiwan."`
}

type indeedLocation struct {
	Country       string `json:"country,omitempty"`
	CountryCode   string `json:"country_code,omitempty"`
	State         string `json:"state,omitempty" jsonschema:"Indeed's own region code, e.g. 'TPE'."`
	City          string `json:"city,omitempty"`
	PostalCode    string `json:"postal_code,omitempty" jsonschema:"Usually absent; Indeed rarely discloses it."`
	StreetAddress string `json:"street_address,omitempty" jsonschema:"Usually absent; Indeed rarely discloses it."`
	Formatted     string `json:"formatted,omitempty" jsonschema:"Indeed's own human-readable rendering."`
}

func indeedLocationFromHTTP(l indeed.Location) *indeedLocation {
	if l == (indeed.Location{}) {
		return nil
	}
	return &indeedLocation{
		Country:       l.Country,
		CountryCode:   l.CountryCode,
		State:         l.State,
		City:          l.City,
		PostalCode:    l.PostalCode,
		StreetAddress: l.StreetAddress,
		Formatted:     l.Formatted,
	}
}

// indeedDetailOutput surfaces every field indeed.JobDetail carries. Unlike
// python-jobspy's cross-site JobPost, this type only ever holds Indeed data,
// so there's no shared-schema reason to trim it down.
type indeedDetailOutput struct {
	Key          string              `json:"key"`
	URL          string              `json:"url,omitempty" jsonschema:"Public job posting URL."`
	Title        string              `json:"title"`
	Company      string              `json:"company,omitempty"`
	CompanyURL   string              `json:"company_url,omitempty"`
	Location     *indeedLocation     `json:"location,omitempty"`
	PostedDate   string              `json:"posted_date,omitempty"`
	Description  string              `json:"description,omitempty" jsonschema:"Full job description as HTML, as Indeed sends it."`
	JobTypes     []string            `json:"job_types,omitempty"`
	Compensation *indeedCompensation `json:"compensation,omitempty"`

	Source      string `json:"source,omitempty" jsonschema:"Listing source: usually the employer's own name, or a third-party board's name when Indeed aggregated the posting."`
	DateIndexed string `json:"date_indexed,omitempty" jsonschema:"When Indeed indexed/last refreshed this posting; can be later than posted_date for reposted or refreshed listings."`

	CompanyWebsite     string   `json:"company_website,omitempty"`
	CompanyIndustry    string   `json:"company_industry,omitempty"`
	CompanyEmployees   string   `json:"company_employees,omitempty"`
	CompanyRevenue     string   `json:"company_revenue,omitempty"`
	CompanyDescription string   `json:"company_description,omitempty"`
	CompanyAddresses   []string `json:"company_addresses,omitempty" jsonschema:"Employer's disclosed office addresses, when available (rare)."`
	CompanyCEO         string   `json:"company_ceo,omitempty"`
	CompanyCEOPhoto    string   `json:"company_ceo_photo,omitempty"`
	CompanyBannerImage string   `json:"company_banner_image,omitempty" jsonschema:"Employer's profile banner image, distinct from the square logo."`

	ApplyURL       string `json:"apply_url,omitempty" jsonschema:"External ATS apply URL, when the poster configured direct apply instead of Indeed-native apply."`
	DetailedSalary string `json:"detailed_salary,omitempty" jsonschema:"Free-text salary detail beyond compensation, when the poster provided one; usually absent."`
	WorkSchedule   string `json:"work_schedule,omitempty" jsonschema:"Free-text work schedule (e.g. shift pattern), when disclosed; usually absent."`
}

func indeedHTTPToMCPDetail(d *indeed.JobDetail) *indeedDetailOutput {
	return &indeedDetailOutput{
		Key:                d.Key,
		URL:                d.JobURL,
		Title:              d.Title,
		Company:            d.Company,
		CompanyURL:         d.CompanyURL,
		Location:           indeedLocationFromHTTP(d.Location),
		PostedDate:         d.PostedDate,
		Description:        d.Description,
		JobTypes:           d.JobTypes,
		Compensation:       indeedCompensationFromHTTP(d.Compensation),
		Source:             d.Source,
		DateIndexed:        d.DateIndexed,
		CompanyWebsite:     d.CompanyWebsite,
		CompanyIndustry:    d.CompanyIndustry,
		CompanyEmployees:   d.CompanyEmployees,
		CompanyRevenue:     d.CompanyRevenue,
		CompanyDescription: d.CompanyDescription,
		CompanyAddresses:   d.CompanyAddresses,
		CompanyCEO:         d.CompanyCEO,
		CompanyCEOPhoto:    d.CompanyCEOPhoto,
		CompanyBannerImage: d.CompanyBannerImage,
		ApplyURL:           d.ApplyURL,
		DetailedSalary:     d.DetailedSalary,
		WorkSchedule:       d.WorkSchedule,
	}
}

// RegisterIndeed registers the Indeed search and job-detail tools.
func RegisterIndeed(s *mcp.Server, c *indeed.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "indeed_search_jobs",
		Description: "Search jobs on Indeed via its GraphQL job-search API.",
		Annotations: &mcp.ToolAnnotations{Title: "Search Indeed jobs", ReadOnlyHint: true},
		InputSchema: indeedSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *indeedSearchInput) (*mcp.CallToolResult, *indeedSearchOutput, error) {
		req, err := indeedMCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.Jobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, indeedHTTPToMCPResponse(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "indeed_get_job_detail",
		Description: "Get the full job description by Indeed job key (key from indeed_search_jobs results).",
		Annotations: &mcp.ToolAnnotations{Title: "Get Indeed job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *indeedDetailInput) (*mcp.CallToolResult, *indeedDetailOutput, error) {
		res, err := c.JobDetail(ctx, in.Country, in.JobKey)
		if err != nil {
			return errorResult(err), nil, nil
		}
		if res == nil {
			return errorResult(fmt.Errorf("job %q not found", in.JobKey)), nil, nil
		}
		return nil, indeedHTTPToMCPDetail(res), nil
	})
}
