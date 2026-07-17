package openingsmcp

import (
	"context"
	"fmt"

	"github.com/amikai/openings-mcp/internal/provider/jobindex"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var jobindexSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text search: role titles, skills, technologies, or a city name (e.g. 'backend engineer', 'python aarhus').",
			"minLength": 1
		},
		"area": {
			"type": "string",
			"description": "Optional Jobindex area path slug, e.g. 'storkoebenhavn', 'midtjylland', 'fyn'. Free-text cities can go in keyword instead.",
			"minLength": 1
		},
		"job_age_days": {
			"type": "integer",
			"description": "Only jobs posted within this many days. Common values: 1, 7, 14, 30. Omit for all ages.",
			"minimum": 1
		},
		"sort": {
			"type": "string",
			"description": "Result order. Defaults to relevance (score).",
			"enum": ["score", "date"],
			"default": "score"
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; each page returns about 20 jobs.",
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

type jobindexJobSummary struct {
	ID         string `json:"id" jsonschema:"Jobindex job id (tid), e.g. h1683131; pass to jobindex_get_job_detail's job_id."`
	Title      string `json:"title"`
	Company    string `json:"company,omitempty"`
	CompanyURL string `json:"company_url,omitempty"`
	Location   string `json:"location,omitempty"`
	PostedDate string `json:"posted_date,omitempty"`
	Deadline   string `json:"deadline,omitempty"`
	URL        string `json:"url,omitempty" jsonschema:"Public Jobindex posting URL (/vis-job/{id})."`
}

type jobindexSearchOutput struct {
	Data       []jobindexJobSummary `json:"data"`
	TotalCount int                  `json:"total_count"`
	Page       int                  `json:"page"`
	TotalPages int                  `json:"total_pages"`
}

func jobindexMCPToHTTPRequest(in *jobindexSearchInput) (*jobindex.JobsRequest, error) {
	req := &jobindex.JobsRequest{
		Keyword:    in.Keyword,
		Area:       in.Area,
		Page:       in.Page,
		JobAgeDays: in.JobAgeDays,
		Sort:       in.Sort,
	}
	if req.Sort == "" {
		req.Sort = jobindex.SortScore
	}
	if req.Sort != jobindex.SortScore && req.Sort != jobindex.SortDate {
		return nil, fmt.Errorf("invalid sort %q", in.Sort)
	}
	return req, nil
}

func jobindexHTTPToMCPResponse(resp *jobindex.JobsResponse) *jobindexSearchOutput {
	out := &jobindexSearchOutput{
		Data:       make([]jobindexJobSummary, 0, len(resp.Jobs)),
		TotalCount: resp.TotalCount,
		Page:       resp.Page,
		TotalPages: resp.TotalPages,
	}
	for _, j := range resp.Jobs {
		out.Data = append(out.Data, jobindexJobSummary{
			ID:         j.ID,
			Title:      j.Title,
			Company:    j.Company,
			CompanyURL: j.CompanyURL,
			Location:   j.Location,
			PostedDate: j.PostedDate,
			Deadline:   j.Deadline,
			URL:        j.URL,
		})
	}
	return out
}

type jobindexDetailInput struct {
	JobID string `json:"job_id" jsonschema:"Jobindex job id from search (tid), e.g. h1683131, or a /vis-job/ or /jobannonce/ URL."`
}

type jobindexDetailOutput struct {
	ID             string `json:"id"`
	URL            string `json:"url,omitempty" jsonschema:"Public Jobindex posting URL."`
	Title          string `json:"title"`
	Company        string `json:"company,omitempty"`
	CompanyURL     string `json:"company_url,omitempty"`
	Location       string `json:"location,omitempty"`
	PostedDate     string `json:"posted_date,omitempty"`
	Deadline       string `json:"deadline,omitempty"`
	EmploymentType string `json:"employment_type,omitempty"`
	Hours          string `json:"hours,omitempty"`
	Description    string `json:"description,omitempty" jsonschema:"Job appetizer text on Jobindex; full JD may live on the employer site."`
	ApplyURL       string `json:"apply_url,omitempty" jsonschema:"Employer / ATS apply URL when Jobindex deep-links off-site."`
}

func jobindexHTTPToMCPDetail(d *jobindex.JobDetail) *jobindexDetailOutput {
	return &jobindexDetailOutput{
		ID:             d.ID,
		URL:            d.URL,
		Title:          d.Title,
		Company:        d.Company,
		CompanyURL:     d.CompanyURL,
		Location:       d.Location,
		PostedDate:     d.PostedDate,
		Deadline:       d.Deadline,
		EmploymentType: d.EmploymentType,
		Hours:          d.Hours,
		Description:    d.Description,
		ApplyURL:       d.ApplyURL,
	}
}

// RegisterJobindex registers Jobindex search and detail tools.
func RegisterJobindex(s *mcp.Server, c *jobindex.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "jobindex_search_jobs",
		Description: "Search jobs on Jobindex.dk (Denmark's largest commercial job board). " +
			"Public HTML search; ~20 results per page. Prefer keyword with role and optional city.",
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
		Description: "Get a Jobindex posting by id (tid from jobindex_search_jobs) or /vis-job/ URL. " +
			"Returns the Jobindex page text; apply_url may point at the employer's external careers site.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Jobindex job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *jobindexDetailInput) (*mcp.CallToolResult, *jobindexDetailOutput, error) {
		res, err := c.JobDetail(ctx, in.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, jobindexHTTPToMCPDetail(res), nil
	})
}
