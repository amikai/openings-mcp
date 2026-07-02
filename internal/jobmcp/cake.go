package jobmcp

import (
	"context"
	"fmt"

	"github.com/amikai/job-mcp/internal/provider/cake"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// cakeSearchInputRawSchema is hand-written JSON kept aligned with
// openapi.yaml's JobSearchRequest/JobSearchFilters: a flat property list
// instead of the query/sort_by/filters nesting. Enum values are the API's
// own slugs, which the converter casts back to the generated types.
var cakeSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text keyword search."
		},
		"location": {
			"type": "string",
			"description": "Location name as shown on Cake.me, localized English or Chinese, e.g. \"Taiwan\", \"台灣\", \"Taipei City, Taiwan\"."
		},
		"job_type": {
			"type": "string",
			"description": "Employment type.",
			"enum": ["full_time", "part_time", "internship", "contract", "freelance", "temporary", "volunteer"]
		},
		"seniority": {
			"type": "array",
			"description": "Seniority levels, OR'd together.",
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["internship_level", "entry_level", "associate", "mid_senior_level", "director", "executive"]
			}
		},
		"remote": {
			"type": "string",
			"description": "Remote-work policy. Omit to include all.",
			"enum": ["no_remote_work", "partial_remote_work", "optional_remote_work", "full_remote_work"]
		},
		"sort": {
			"type": "string",
			"description": "Result order. Defaults to popularity.",
			"enum": ["popularity", "latest"]
		},
		"page": {
			"type": "integer",
			"description": "1-based page number.",
			"minimum": 1
		}
	},
	"required": ["keyword", "location"],
	"additionalProperties": false
}`)

var cakeSearchInputSchema = mustSchema(cakeSearchInputRawSchema)

type cakeSearchInput struct {
	Keyword   string   `json:"keyword"`  // required
	Location  string   `json:"location"` // required
	JobType   string   `json:"job_type,omitempty"`
	Seniority []string `json:"seniority,omitempty"`
	Remote    string   `json:"remote,omitempty"`
	Sort      string   `json:"sort,omitempty"`
	Page      int      `json:"page,omitempty"`
}

type cakeDetailInput struct {
	Path string `json:"path" jsonschema:"Cake.me job path (path from search results)."`
}

func cakeMCPToHTTPRequest(in *cakeSearchInput) (*cake.JobSearchRequest, error) {
	var req cake.JobSearchRequest
	// The schema already marks keyword and location required; this guards
	// direct callers and clients that skip schema validation.
	if in.Keyword == "" {
		return nil, fmt.Errorf("keyword is required")
	}
	req.Query = in.Keyword

	if in.Location == "" {
		return nil, fmt.Errorf("location is required")
	}
	req.Filters.Locations = []string{in.Location}

	// The Cake API rejects requests without sort_by, so default to
	// popularity when sort is omitted.
	req.SortBy = cake.JobSearchRequestSortByPopularity
	if in.Sort != "" {
		sort := cake.JobSearchRequestSortBy(in.Sort)
		if err := sort.Validate(); err != nil {
			return nil, fmt.Errorf("invalid sort %q: %w", in.Sort, err)
		}
		req.SortBy = sort
	}

	if in.JobType != "" {
		jobType := cake.JobSearchFiltersJobTypesItem(in.JobType)
		if err := jobType.Validate(); err != nil {
			return nil, fmt.Errorf("invalid job_type %q: %w", in.JobType, err)
		}
		req.Filters.JobTypes = []cake.JobSearchFiltersJobTypesItem{jobType}
	}

	for _, slug := range in.Seniority {
		seniority := cake.JobSearchFiltersSeniorityLevelsItem(slug)
		if err := seniority.Validate(); err != nil {
			return nil, fmt.Errorf("invalid seniority %q: %w", slug, err)
		}
		req.Filters.SeniorityLevels = append(req.Filters.SeniorityLevels, seniority)
	}

	if in.Remote != "" {
		remote := cake.JobSearchFiltersRemoteItem(in.Remote)
		if err := remote.Validate(); err != nil {
			return nil, fmt.Errorf("invalid remote %q: %w", in.Remote, err)
		}
		req.Filters.Remote = []cake.JobSearchFiltersRemoteItem{remote}
	}

	if in.Page > 0 {
		req.Page = cake.NewOptInt(in.Page)
	}
	return &req, nil
}

// cakeSearchOutput mirrors cake.JobSearchResponse for the LLM: identical
// fields and JSON names.
type cakeSearchOutput struct {
	TotalEntries int              `json:"total_entries"`
	TotalPages   int              `json:"total_pages"`
	PerPage      int              `json:"per_page"`
	CurrentPage  int              `json:"current_page"`
	Data         []cakeJobSummary `json:"data"`
}

type cakeJobSummary struct {
	Path        string `json:"path" jsonschema:"Job path; pass to cake_get_job_detail."`
	Title       string `json:"title"`
	Description string `json:"description" jsonschema:"Plain-text preview; cake_get_job_detail returns the full description."`
}

func cakeHTTPToMCPResponse(resp *cake.JobSearchResponse) *cakeSearchOutput {
	out := &cakeSearchOutput{
		TotalEntries: resp.TotalEntries,
		TotalPages:   resp.TotalPages,
		PerPage:      resp.PerPage,
		CurrentPage:  resp.CurrentPage,
		Data:         make([]cakeJobSummary, 0, len(resp.Data)),
	}
	for _, j := range resp.Data {
		out.Data = append(out.Data, cakeJobSummary{
			Path:        j.Path,
			Title:       j.Title,
			Description: j.Description,
		})
	}
	return out
}

// cakeDetailOutput mirrors cake.JobDetail for the LLM: identical fields and
// JSON names.
type cakeDetailOutput struct {
	ID           int    `json:"id"`
	Path         string `json:"path"`
	PagePath     string `json:"page_path" jsonschema:"Company page slug; the public job page is https://www.cake.me/companies/{page_path}/jobs/{path}."`
	Title        string `json:"title"`
	Description  string `json:"description" jsonschema:"Full job description as an HTML string."`
	Requirements string `json:"requirements" jsonschema:"Job requirements as an HTML string; may be empty."`
}

func cakeHTTPToMCPDetail(detail *cake.JobDetail) *cakeDetailOutput {
	return &cakeDetailOutput{
		ID:           detail.ID,
		Path:         detail.Path,
		PagePath:     detail.PagePath,
		Title:        detail.Title,
		Description:  detail.Description,
		Requirements: detail.Requirements,
	}
}

// RegisterCake registers the Cake.me search and job-detail tools.
func RegisterCake(s *mcp.Server, c *cake.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "cake_search_jobs",
		Description: "Search jobs on Cake.me (formerly CakeResume) by keyword and location, with optional job-type/seniority/remote/sort filters.",
		InputSchema: cakeSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *cakeSearchInput) (*mcp.CallToolResult, *cakeSearchOutput, error) {
		req, err := cakeMCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.SearchJobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, ok := res.(*cake.JobSearchResponse)
		if !ok {
			return errorResult(fmt.Errorf("job search returned %T", res)), nil, nil
		}
		return nil, cakeHTTPToMCPResponse(resp), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "cake_get_job_detail",
		Description: "Get the full job description and requirements for a Cake.me job path (path from search results).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *cakeDetailInput) (*mcp.CallToolResult, *cakeDetailOutput, error) {
		res, err := c.GetJobDetail(ctx, cake.GetJobDetailParams{Path: in.Path})
		if err != nil {
			return errorResult(err), nil, nil
		}
		detail, ok := res.(*cake.JobDetail)
		if !ok {
			return errorResult(fmt.Errorf("job detail %s returned %T", in.Path, res)), nil, nil
		}
		return nil, cakeHTTPToMCPDetail(detail), nil
	})
}
