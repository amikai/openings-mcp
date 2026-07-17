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
			"description": "Jobindex q= free-text search (role, skill, company, city).",
			"minLength": 1
		},
		"area": {
			"type": "string",
			"description": "Optional Jobindex area path slug (e.g. storkoebenhavn, midtjylland, fyn).",
			"minLength": 1
		},
		"job_age_days": {
			"type": "integer",
			"description": "Jobindex jobage= max posting age in days (1, 7, 14, 30). Omit for all ages.",
			"minimum": 1
		},
		"sort": {
			"type": "string",
			"description": "Jobindex sort= parameter. Defaults to score.",
			"enum": ["score", "date"],
			"default": "score"
		},
		"page": {
			"type": "integer",
			"description": "Jobindex page= (1-based). About 20 results per page.",
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

// jobindexSearchOutput mirrors Jobindex Stash searchResponse field names
// (hitcount, total_pages, results). page is the requested page (not a Stash
// field). results are upstream result objects with only card "html" stripped.
type jobindexSearchOutput struct {
	Hitcount   int              `json:"hitcount"`
	TotalPages int              `json:"total_pages,omitempty"`
	Page       int              `json:"page"`
	Results    []map[string]any `json:"results" jsonschema:"Upstream search result objects (tid, headline, company, area, firstdate, lastdate, apply_deadline_asap, share_url, url, …). Per-result html card markup is omitted."`
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
	// Tid is the Jobindex tid from search results (e.g. h1683131), or a
	// /vis-job/ or /jobannonce/ URL.
	Tid string `json:"tid" jsonschema:"Jobindex tid from search results (e.g. h1683131), or a /vis-job/ or /jobannonce/ URL."`
}

// jobindexDetailOutput uses the same key names as search Stash where concepts
// match. Values are scraped from /vis-job HTML (no JSON detail API).
type jobindexDetailOutput struct {
	Tid            string         `json:"tid"`
	Headline       string         `json:"headline"`
	Company        map[string]any `json:"company,omitempty"`
	Area           string         `json:"area,omitempty"`
	Firstdate      string         `json:"firstdate,omitempty"`
	ShareURL       string         `json:"share_url,omitempty"`
	ApplyURL       string         `json:"apply_url,omitempty"`
	Description    string         `json:"description,omitempty" jsonschema:"Appetizer text from the vis-job page (og:description / body). Full JD may be on the employer site."`
	EmploymentType string         `json:"employment_type,omitempty"`
	Hours          string         `json:"hours,omitempty"`
	ApplyDeadline  string         `json:"apply_deadline,omitempty" jsonschema:"Only when the page labels a deadline; never synthesized."`
}

func jobindexHTTPToMCPDetail(d *jobindex.JobDetail) *jobindexDetailOutput {
	return &jobindexDetailOutput{
		Tid:            d.Tid,
		Headline:       d.Headline,
		Company:        d.Company,
		Area:           d.Area,
		Firstdate:      d.Firstdate,
		ShareURL:       d.ShareURL,
		ApplyURL:       d.ApplyURL,
		Description:    d.Description,
		EmploymentType: d.EmploymentType,
		Hours:          d.Hours,
		ApplyDeadline:  d.ApplyDeadline,
	}
}

// RegisterJobindex registers Jobindex search and detail tools.
func RegisterJobindex(s *mcp.Server, c *jobindex.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "jobindex_search_jobs",
		Description: "Search Jobindex.dk. Returns the upstream Stash searchResponse shape " +
			"(hitcount, total_pages, results with tid/headline/company/…). " +
			"Only per-result card html markup is stripped. ~20 results per page.",
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
		Name: "jobindex_get_job_detail",
		Description: "Fetch a Jobindex /vis-job/{tid} page and return scraped fields using " +
			"upstream-aligned names (tid, headline, company, area, share_url, apply_url). " +
			"There is no JSON detail API; apply_deadline is only set when the page labels it.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Jobindex job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *jobindexDetailInput) (*mcp.CallToolResult, *jobindexDetailOutput, error) {
		res, err := c.JobDetail(ctx, in.Tid)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, jobindexHTTPToMCPDetail(res), nil
	})
}
