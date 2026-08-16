package openingsmcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/amikai/openings-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tsmcSearchInputRawSchema is hand-written JSON kept aligned with
// openapi.yaml's SearchJobs parameters: label enums instead of the site's
// numeric form-field IDs, which the converter maps back via the provider's
// lookup tables.
var tsmcSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text keyword search across job titles.",
			"minLength": 1
		},
		"location": {
			"type": "string",
			"description": "Location filter.",
			"enum": [
				"Taiwan", "Canada", "China", "Germany-Dresden", "Germany-Munich",
				"Japan-Yokohama", "Japan-Osaka", "Japan-Tsukuba", "Japan-Kumamoto",
				"Korea", "Netherlands", "USA-Arizona", "USA-California",
				"USA-Massachusetts", "USA-Texas", "USA-Washington", "USA-Washington, D.C."
			]
		},
		"category": {
			"type": "string",
			"description": "Job category filter.",
			"enum": [
				"R&D", "Specialty Technology", "IC Design Technology", "Manufacturing (fabs)",
				"Facility & Industrial Safety / Environmental Protection", "Product Development",
				"R&D Advanced Packaging Technology Development", "Testing Development and Technology",
				"Quality and Reliability", "Information Technology", "Internal Audit",
				"Business Development", "Customer Service", "Corporate Planning",
				"Finance / Accounting / Risk Management", "Human Resources", "Legal",
				"Materials Management", "Corporate Sustainability (ESG)", "Administration",
				"Accessibility Inclusion"
			]
		},
		"job_type": {
			"type": "string",
			"description": "Job level filter.",
			"enum": [
				"Technician", "Associate Engineer / Admin", "Engineer / Admin",
				"Manager / Executive", "Others"
			]
		},
		"employment_type": {
			"type": "string",
			"description": "Employment type filter.",
			"enum": ["Regular", "Temporary", "Intern", "Apprenticeship"]
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; 10 results per page.",
			"minimum": 1,
			"default": 1
		}
	},
	"required": ["keyword", "location"],
	"additionalProperties": false
}`)

var tsmcSearchInputSchema = mustSchema(tsmcSearchInputRawSchema)

type tsmcSearchInput struct {
	Keyword        string `json:"keyword"`  // required
	Location       string `json:"location"` // required
	Category       string `json:"category,omitempty"`
	JobType        string `json:"job_type,omitempty"`
	EmploymentType string `json:"employment_type,omitempty"`
	Page           int    `json:"page,omitempty"`
}

type tsmcSearchOutput struct {
	Total int              `json:"total"`
	Data  []tsmcJobSummary `json:"data"`
}

type tsmcJobSummary struct {
	ID             string `json:"id" jsonschema:"Job ID; pass to tsmc_get_job_detail."`
	URL            string `json:"url,omitempty" jsonschema:"Public TSMC careers job posting URL."`
	Title          string `json:"title"`
	Location       string `json:"location,omitempty"`
	CareerArea     string `json:"career_area,omitempty"`
	EmploymentType string `json:"employment_type,omitempty"`
	Posted         string `json:"posted,omitempty"`
}

type tsmcDetailInput struct {
	JobID string `json:"job_id" jsonschema:"TSMC job ID (id from search results, e.g. 21826)."`
}

type tsmcDetailOutput struct {
	ID               string `json:"id"`
	URL              string `json:"url,omitempty" jsonschema:"Public TSMC careers job posting URL."`
	Title            string `json:"title"`
	Company          string `json:"company,omitempty"`
	Location         string `json:"location,omitempty"`
	CareerArea       string `json:"career_area,omitempty"`
	JobType          string `json:"job_type,omitempty"`
	EmploymentType   string `json:"employment_type,omitempty"`
	Posted           string `json:"posted,omitempty"`
	Responsibilities string `json:"responsibilities,omitempty" jsonschema:"Job responsibilities as plain text."`
	Qualifications   string `json:"qualifications,omitempty" jsonschema:"Job qualifications as plain text."`
}

func tsmcMCPToHTTPRequest(in *tsmcSearchInput) (*tsmc.JobsRequest, error) {
	var req tsmc.JobsRequest
	// The schema already marks keyword and location required; this guards
	// direct callers and clients that skip schema validation.
	if in.Keyword == "" {
		return nil, errors.New("keyword is required")
	}
	req.Keyword = in.Keyword

	if in.Location == "" {
		return nil, errors.New("location is required")
	}
	loc, ok := tsmc.LocationIDs[in.Location]
	if !ok {
		return nil, fmt.Errorf("invalid location %q", in.Location)
	}
	req.Locations = []string{loc}

	if in.Category != "" {
		id, ok := tsmc.CategoryIDs[in.Category]
		if !ok {
			return nil, fmt.Errorf("invalid category %q", in.Category)
		}
		req.Categories = []string{id}
	}

	if in.JobType != "" {
		id, ok := tsmc.JobTypeIDs[in.JobType]
		if !ok {
			return nil, fmt.Errorf("invalid job_type %q", in.JobType)
		}
		req.JobTypes = []string{id}
	}

	if in.EmploymentType != "" {
		id, ok := tsmc.EmploymentTypeIDs[in.EmploymentType]
		if !ok {
			return nil, fmt.Errorf("invalid employment_type %q", in.EmploymentType)
		}
		req.EmploymentTypes = []string{id}
	}

	req.Page = in.Page
	return &req, nil
}

func tsmcHTTPToMCPResponse(resp *tsmc.JobsResponse) *tsmcSearchOutput {
	out := &tsmcSearchOutput{
		Total: resp.Total,
		Data:  make([]tsmcJobSummary, 0, len(resp.Jobs)),
	}
	for _, j := range resp.Jobs {
		out.Data = append(out.Data, tsmcJobSummary{
			ID:             j.ID,
			URL:            tsmcJobURL(j.Slug, j.ID),
			Title:          j.Title,
			Location:       j.Location,
			CareerArea:     j.CareerArea,
			EmploymentType: j.EmploymentType,
			Posted:         j.Posted,
		})
	}
	return out
}

func tsmcHTTPToMCPDetail(detail *tsmc.JobDetailResponse) *tsmcDetailOutput {
	return &tsmcDetailOutput{
		ID:               detail.ID,
		URL:              tsmcJobURL(detail.Slug, detail.ID),
		Title:            detail.Title,
		Company:          detail.Company,
		Location:         detail.Location,
		CareerArea:       detail.CareerArea,
		JobType:          detail.JobType,
		EmploymentType:   detail.EmploymentType,
		Posted:           detail.Posted,
		Responsibilities: detail.Responsibilities,
		Qualifications:   detail.Qualifications,
	}
}

func tsmcJobURL(slug, id string) string {
	if slug == "" || id == "" {
		return ""
	}
	return fmt.Sprintf("https://careers.tsmc.com/zh_TW/careers/JobDetail/%s/%s", slug, id)
}

// RegisterTsmc registers the TSMC search and job-detail tools.
func RegisterTsmc(s *mcp.Server, c *tsmc.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "tsmc_search_jobs",
		Description: "Search jobs on the TSMC careers site.",
		Annotations: &mcp.ToolAnnotations{Title: "Search TSMC jobs", ReadOnlyHint: true},
		InputSchema: tsmcSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *tsmcSearchInput) (*mcp.CallToolResult, *tsmcSearchOutput, error) {
		req, err := tsmcMCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.Jobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, tsmcHTTPToMCPResponse(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "tsmc_get_job_detail",
		Description: "Get the full job description and requirements for a TSMC job by job ID.",
		Annotations: &mcp.ToolAnnotations{Title: "Get TSMC job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *tsmcDetailInput) (*mcp.CallToolResult, *tsmcDetailOutput, error) {
		res, err := c.JobDetail(ctx, in.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, tsmcHTTPToMCPDetail(res), nil
	})
}
