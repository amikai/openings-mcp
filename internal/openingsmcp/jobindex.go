package openingsmcp

import (
	"context"

	"github.com/amikai/openings-mcp/internal/provider/jobindex"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var jobindexSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text search: role titles, skills, company names, or cities.",
			"minLength": 1
		},
		"area": {
			"type": "string",
			"description": "Optional region filter as a path slug (e.g. storkoebenhavn, midtjylland, fyn for Danish regions).",
			"minLength": 1
		},
		"job_age_days": {
			"type": "integer",
			"description": "Only jobs posted within this many days (commonly 1, 7, 14, or 30). Omit for no recency filter.",
			"minimum": 1
		},
		"sort": {
			"type": "string",
			"description": "Result order: relevance (score) or newest first (date). Defaults to score.",
			"enum": ["score", "date"],
			"default": "score"
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; about 20 results per page.",
			"minimum": 1,
			"default": 1
		}
	},
	"required": ["keyword"],
	"additionalProperties": false
}`)

var jobindexSearchInputSchema = mustSchema(jobindexSearchInputRawSchema)

type jobindexSearchInput struct {
	Keyword    string `json:"keyword"`
	Area       string `json:"area,omitempty"`
	JobAgeDays int    `json:"job_age_days,omitempty"`
	Sort       string `json:"sort,omitempty"`
	Page       int    `json:"page,omitempty"`
}

// jobindexSearchOutput is the client-facing search payload.
type jobindexSearchOutput struct {
	Hitcount   int              `json:"hitcount" jsonschema:"Total number of matching jobs."`
	TotalPages int              `json:"total_pages,omitempty" jsonschema:"Total pages when known."`
	Page       int              `json:"page" jsonschema:"Current 1-based page."`
	Results    []map[string]any `json:"results" jsonschema:"Matching jobs. Typical fields: tid (id for get_job_detail), headline (title), company{name}, area (location), posted_at and expired_at as ISO 8601 dates YYYY-MM-DD (published and listing end; expired_at is not always the application deadline), apply_deadline / apply_deadline_asap when present, and url (only link: open or apply)."`
}

func jobindexMCPToHTTPRequest(in *jobindexSearchInput) (*jobindex.JobsRequest, error) {
	// Sort is schema-enum'd; the provider client validates again in searchURL.
	return &jobindex.JobsRequest{
		Keyword:    in.Keyword,
		Area:       in.Area,
		Page:       in.Page,
		JobAgeDays: in.JobAgeDays,
		Sort:       in.Sort,
	}, nil
}

func jobindexHTTPToMCPResponse(resp *jobindex.SearchResponse) *jobindexSearchOutput {
	return &jobindexSearchOutput{
		Hitcount:   resp.Hitcount,
		TotalPages: resp.TotalPages,
		Page:       resp.Page,
		Results:    resp.Results,
	}
}

type jobindexDetailInput struct {
	Tid string `json:"tid" jsonschema:"Job id from jobindex_search_jobs (e.g. h1683131), or a public job page URL."`
}

// jobindexDetailOutput is the client-facing detail payload.
type jobindexDetailOutput struct {
	Tid            string         `json:"tid" jsonschema:"Job id."`
	Headline       string         `json:"headline" jsonschema:"Job title."`
	Company        map[string]any `json:"company,omitempty" jsonschema:"Employer; typically {name}."`
	Area           string         `json:"area,omitempty" jsonschema:"Location text."`
	PostedAt       string         `json:"posted_at,omitempty" jsonschema:"When the listing was published; ISO 8601 date (YYYY-MM-DD)."`
	URL            string         `json:"url,omitempty" jsonschema:"URL to open or apply for this job (employer apply link when known, otherwise the public listing page)."`
	Description    string         `json:"description,omitempty" jsonschema:"Short listing text; full description may only appear on the employer site."`
	EmploymentType string         `json:"employment_type,omitempty" jsonschema:"Employment type when shown on the listing."`
	Hours          string         `json:"hours,omitempty" jsonschema:"Working hours when shown on the listing."`
	ApplyDeadline  string         `json:"apply_deadline,omitempty" jsonschema:"Application deadline when the listing states one; never invented. May be an ISO 8601 datetime."`
}

func jobindexHTTPToMCPDetail(d *jobindex.JobDetail) *jobindexDetailOutput {
	return &jobindexDetailOutput{
		Tid:            d.Tid,
		Headline:       d.Headline,
		Company:        d.Company,
		Area:           d.Area,
		PostedAt:       d.PostedAt,
		URL:            d.URL,
		Description:    d.Description,
		EmploymentType: d.EmploymentType,
		Hours:          d.Hours,
		ApplyDeadline:  d.ApplyDeadline,
	}
}

// RegisterJobindex registers Jobindex search and detail tools.
func RegisterJobindex(s *mcp.Server, c *jobindex.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "jobindex_search_jobs",
		Description: "Search jobs on Jobindex (Denmark job board).",
		Annotations: &mcp.ToolAnnotations{Title: "Search Jobindex jobs", ReadOnlyHint: true},
		InputSchema: jobindexSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *jobindexSearchInput) (*mcp.CallToolResult, *jobindexSearchOutput, error) {
		req, err := jobindexMCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.Jobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, jobindexHTTPToMCPResponse(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "jobindex_get_job_detail",
		Description: "Get details for a job from jobindex_search_jobs (pass tid from a search result).",
		Annotations: &mcp.ToolAnnotations{Title: "Get Jobindex job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *jobindexDetailInput) (*mcp.CallToolResult, *jobindexDetailOutput, error) {
		res, err := c.JobDetail(ctx, in.Tid)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, jobindexHTTPToMCPDetail(res), nil
	})
}
