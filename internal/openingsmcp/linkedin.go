package openingsmcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/amikai/openings-mcp/internal/provider/linkedin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var linkedinSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text search query matched against job title, company, and description."
		},
		"location": {
			"type": "string",
			"description": "Free-text location filter. LinkedIn searches globally; there is no separate country-code parameter."
		},
		"distance": {
			"type": "integer",
			"description": "Search radius in miles around location.",
			"minimum": 0
		},
		"workplace_type": {
			"type": "string",
			"description": "Workplace type filter.",
			"enum": ["On-site", "Remote", "Hybrid"]
		},
		"job_type": {
			"type": "string",
			"description": "Job type filter.",
			"enum": ["Full-time", "Part-time", "Contract", "Temporary", "Internship"]
		},
		"easy_apply": {
			"type": "boolean",
			"description": "Only jobs with LinkedIn Easy Apply."
		},
		"company_ids": {
			"type": "string",
			"description": "Comma-separated LinkedIn numeric company IDs. IDs are opaque and must be resolved from a company's public page or a prior search response, not guessed."
		},
		"posted_within": {
			"type": "string",
			"description": "Only jobs posted within this window.",
			"enum": ["Past day", "Past week", "Past month"]
		},
		"start": {
			"type": "integer",
			"description": "Zero-based result offset; default 0. The endpoint always returns exactly 10 cards per call regardless of this value, so paging through results must increment start by exactly 10 each call (0, 10, 20, ...) to avoid gaps. Do not mimic a real browser's 25-per-step scroll traffic, which skips 10 of every 25 positions this endpoint can return.",
			"minimum": 0
		}
	},
	"additionalProperties": false
}`)

// linkedinSearchInputSchema is hand-written JSON kept aligned with
// openapi.yaml's searchJobs parameters: human labels instead of the site's
// raw form-field codes (workplace_type/job_type map back via ids.go;
// posted_within maps back via linkedinPostedWithinSeconds below).
var linkedinSearchInputSchema = mustSchema(linkedinSearchInputRawSchema)

type linkedinSearchInput struct {
	Keyword       string `json:"keyword,omitempty"`
	Location      string `json:"location,omitempty"`
	Distance      int    `json:"distance,omitempty"`
	WorkplaceType string `json:"workplace_type,omitempty"`
	JobType       string `json:"job_type,omitempty"`
	EasyApply     bool   `json:"easy_apply,omitempty"`
	CompanyIDs    string `json:"company_ids,omitempty"`
	PostedWithin  string `json:"posted_within,omitempty"`
	Start         int    `json:"start,omitempty"`
}

// linkedinPostedWithinSeconds maps a human label to the seconds value
// linkedin.JobsRequest.PostedWithinSeconds expects (f_TPR=r{n} on the wire).
var linkedinPostedWithinSeconds = map[string]int{
	"Past day":   86400,
	"Past week":  604800,
	"Past month": 2592000,
}

func linkedinMCPToHTTPRequest(in *linkedinSearchInput) (*linkedin.JobsRequest, error) {
	req := &linkedin.JobsRequest{
		Keywords:  in.Keyword,
		Location:  in.Location,
		Distance:  in.Distance,
		EasyApply: in.EasyApply,
		Start:     in.Start,
	}

	if in.WorkplaceType != "" {
		id, ok := linkedin.WorkplaceTypeIDs[in.WorkplaceType]
		if !ok {
			return nil, fmt.Errorf("invalid workplace_type %q", in.WorkplaceType)
		}
		req.WorkplaceType = id
	}

	if in.JobType != "" {
		id, ok := linkedin.JobTypeIDs[in.JobType]
		if !ok {
			return nil, fmt.Errorf("invalid job_type %q", in.JobType)
		}
		req.JobType = id
	}

	if in.CompanyIDs != "" {
		for _, id := range strings.Split(in.CompanyIDs, ",") {
			if id = strings.TrimSpace(id); id != "" {
				req.CompanyIDs = append(req.CompanyIDs, id)
			}
		}
	}

	if in.PostedWithin != "" {
		seconds, ok := linkedinPostedWithinSeconds[in.PostedWithin]
		if !ok {
			return nil, fmt.Errorf("invalid posted_within %q", in.PostedWithin)
		}
		req.PostedWithinSeconds = seconds
	}

	return req, nil
}

type linkedinSearchOutput struct {
	Data []linkedinJobSummary `json:"data"`
}

type linkedinJobSummary struct {
	ID          string `json:"id" jsonschema:"Numeric LinkedIn job ID; pass to linkedin_get_job_detail's job_id param."`
	Title       string `json:"title"`
	Company     string `json:"company,omitempty"`
	CompanyURL  string `json:"company_url,omitempty"`
	Location    string `json:"location,omitempty"`
	PostedDate  string `json:"posted_date,omitempty"`
	LooksRemote bool   `json:"looks_remote,omitempty" jsonschema:"Keyword heuristic (title/location substring match for 'remote'/'work from home'/'wfh'), not a field LinkedIn provides. False does not mean confirmed on-site."`
	URL         string `json:"url,omitempty" jsonschema:"Public job posting URL."`
}

func linkedinJobURL(id string) string {
	if id == "" {
		return ""
	}
	return "https://www.linkedin.com/jobs/view/" + id
}

func linkedinHTTPToMCPResponse(resp *linkedin.JobsResponse) *linkedinSearchOutput {
	out := &linkedinSearchOutput{Data: make([]linkedinJobSummary, 0, len(resp.Jobs))}
	for _, j := range resp.Jobs {
		out.Data = append(out.Data, linkedinJobSummary{
			ID:          j.ID,
			Title:       j.Title,
			Company:     j.Company,
			CompanyURL:  j.CompanyURL,
			Location:    j.Location,
			PostedDate:  j.PostedDate,
			LooksRemote: j.Remote,
			URL:         linkedinJobURL(j.ID),
		})
	}
	return out
}

type linkedinDetailInput struct {
	JobID string `json:"job_id" jsonschema:"Numeric LinkedIn job ID (id from linkedin_search_jobs results, e.g. 4422697744)."`
}

type linkedinDetailOutput struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Company        string `json:"company,omitempty"`
	Location       string `json:"location,omitempty"`
	Posted         string `json:"posted,omitempty"`
	SeniorityLevel string `json:"seniority_level,omitempty"`
	EmploymentType string `json:"employment_type,omitempty"`
	JobFunction    string `json:"job_function,omitempty"`
	Industries     string `json:"industries,omitempty"`
	Description    string `json:"description,omitempty" jsonschema:"Full job description as plain text."`
	ApplyURL       string `json:"apply_url,omitempty" jsonschema:"External ATS apply URL; absent for LinkedIn Easy Apply postings."`
	LooksRemote    bool   `json:"looks_remote,omitempty" jsonschema:"Keyword heuristic over title/location only (not the full description), not a field LinkedIn provides. False does not mean confirmed on-site."`
}

func linkedinHTTPToMCPDetail(detail *linkedin.JobDetailResponse) *linkedinDetailOutput {
	return &linkedinDetailOutput{
		ID:             detail.ID,
		Title:          detail.Title,
		Company:        detail.Company,
		Location:       detail.Location,
		Posted:         detail.Posted,
		SeniorityLevel: detail.SeniorityLevel,
		EmploymentType: detail.EmploymentType,
		JobFunction:    detail.JobFunction,
		Industries:     detail.Industries,
		Description:    detail.Description,
		ApplyURL:       detail.ApplyURL,
		LooksRemote:    detail.Remote,
	}
}

// RegisterLinkedin registers the LinkedIn search and job-detail tools.
func RegisterLinkedin(s *mcp.Server, c *linkedin.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "linkedin_search_jobs",
		Description: "Search jobs on LinkedIn's public guest job-search surface by keyword/location, with optional workplace-type/job-type/easy-apply/company/posted-within filters. Caution: LinkedIn rate-limits aggressively -- a single session is typically cut off around the 10th consecutive search request with a plain HTTP 429 carrying no Retry-After hint. Page conservatively (start in steps of 10) and back off on your own schedule rather than retrying immediately after a 429.",
		Annotations: &mcp.ToolAnnotations{Title: "Search LinkedIn jobs", ReadOnlyHint: true},
		InputSchema: linkedinSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *linkedinSearchInput) (*mcp.CallToolResult, *linkedinSearchOutput, error) {
		req, err := linkedinMCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.Jobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, linkedinHTTPToMCPResponse(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "linkedin_get_job_detail",
		Description: "Get the full job description and criteria for a LinkedIn job by numeric ID (id from linkedin_search_jobs results). Caution: this is the most block-prone endpoint -- a cold request can return HTTP 999 (bot-suspected authwall) -- and it shares the same session-wide rate-limit budget as search, so avoid fetching details for many jobs in one session.",
		Annotations: &mcp.ToolAnnotations{Title: "Get LinkedIn job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *linkedinDetailInput) (*mcp.CallToolResult, *linkedinDetailOutput, error) {
		res, err := c.JobDetail(ctx, in.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, linkedinHTTPToMCPDetail(res), nil
	})
}
