package openingsmcp

import (
	"context"
	"fmt"

	"github.com/amikai/openings-mcp/internal/provider/linkedin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var linkedinSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text search query: role titles, skills, or technologies.",
			"minLength": 1
		},
		"location": {
			"type": "string",
			"description": "Free-text location, e.g. 'Taipei, Taiwan'; omit to search worldwide.",
			"minLength": 1
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
		"company_ids": {
			"type": "array",
			"description": "LinkedIn numeric company IDs, e.g. 1441. IDs are opaque and must come from a company's LinkedIn page, never guessed; search results do not include them.",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string"
			}
		},
		"posted_within": {
			"type": "string",
			"description": "Only jobs posted within this window.",
			"enum": ["Past day", "Past week", "Past month"]
		},
		"start": {
			"type": "integer",
			"description": "Zero-based result offset. Each call returns exactly 10 results; increment by 10 each page (0, 10, 20, ...).",
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
	Keyword       string   `json:"keyword,omitempty"`
	Location      string   `json:"location,omitempty"`
	WorkplaceType string   `json:"workplace_type,omitempty"`
	JobType       string   `json:"job_type,omitempty"`
	CompanyIDs    []string `json:"company_ids,omitempty"`
	PostedWithin  string   `json:"posted_within,omitempty"`
	Start         int      `json:"start,omitempty"`
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
		Keywords:   in.Keyword,
		Location:   in.Location,
		CompanyIDs: in.CompanyIDs,
		Start:      in.Start,
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
	ID         string `json:"id" jsonschema:"Numeric LinkedIn job ID; pass to linkedin_get_job_detail's job_id param."`
	Title      string `json:"title"`
	Company    string `json:"company,omitempty"`
	CompanyURL string `json:"company_url,omitempty"`
	Location   string `json:"location,omitempty"`
	PostedDate string `json:"posted_date,omitempty"`
	Remote     bool   `json:"remote,omitempty"`
	URL        string `json:"url,omitempty" jsonschema:"Public job posting URL."`
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
			ID:         j.ID,
			Title:      j.Title,
			Company:    j.Company,
			CompanyURL: j.CompanyURL,
			Location:   j.Location,
			PostedDate: j.PostedDate,
			Remote:     j.Remote,
			URL:        linkedinJobURL(j.ID),
		})
	}
	return out
}

type linkedinDetailInput struct {
	JobID string `json:"job_id" jsonschema:"Numeric LinkedIn job ID (id from linkedin_search_jobs results, e.g. 4422697744)."`
}

type linkedinDetailOutput struct {
	ID             string `json:"id"`
	URL            string `json:"url,omitempty" jsonschema:"Public job posting URL."`
	Title          string `json:"title"`
	Company        string `json:"company,omitempty"`
	Location       string `json:"location,omitempty"`
	Posted         string `json:"posted,omitempty" jsonschema:"Relative time, e.g. '1 month ago'; LinkedIn doesn't expose an exact date."`
	SeniorityLevel string `json:"seniority_level,omitempty"` // LinkedIn's own "Seniority level" criterion, e.g. Entry level, Mid-Senior level, Director
	EmploymentType string `json:"employment_type,omitempty"` // LinkedIn's own "Employment type" criterion; distinct from the job_type search filter
	JobFunction    string `json:"job_function,omitempty"`    // LinkedIn's own "Job function" criterion, e.g. Engineering, Sales, Marketing
	Industries     string `json:"industries,omitempty"`      // LinkedIn's own "Industries" criterion, e.g. IT Services and IT Consulting
	Description    string `json:"description,omitempty" jsonschema:"Full job description as plain text."`
	ApplyURL       string `json:"apply_url,omitempty" jsonschema:"External ATS apply URL."`
	Remote         bool   `json:"remote,omitempty" jsonschema:"Keyword heuristic over title/location only (not the full description), not a field LinkedIn provides. False does not mean confirmed on-site."`
}

func linkedinHTTPToMCPDetail(detail *linkedin.JobDetailResponse) *linkedinDetailOutput {
	return &linkedinDetailOutput{
		ID:             detail.ID,
		URL:            linkedinJobURL(detail.ID),
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
		Remote:         detail.Remote,
	}
}

// RegisterLinkedin registers the LinkedIn search and job-detail tools.
func RegisterLinkedin(s *mcp.Server, c *linkedin.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "linkedin_search_jobs",
		Description: "Search jobs on LinkedIn's public guest job-search surface. LinkedIn's rate limiting is aggressive; back off instead of retrying on a 429.",
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
		Description: "Get the full job description by LinkedIn ID (id from linkedin_search_jobs results). May be flagged as a bot and blocked (HTTP 999).",
		Annotations: &mcp.ToolAnnotations{Title: "Get LinkedIn job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *linkedinDetailInput) (*mcp.CallToolResult, *linkedinDetailOutput, error) {
		res, err := c.JobDetail(ctx, in.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, linkedinHTTPToMCPDetail(res), nil
	})
}
