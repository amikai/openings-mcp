package jobmcp

import (
	"context"
	"fmt"

	"github.com/amikai/job-mcp/internal/provider/google"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// googleSearchInputRawSchema is hand-written JSON kept aligned with
// openapi.yaml's searchJobs parameters. The spec marks every query parameter
// optional; keyword and location are required here so searches stay scoped.
var googleSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text search query matched against job title and description."
		},
		"location": {
			"type": "string",
			"description": "Location filter; a city, region, or country name (e.g. \"Taiwan\", \"New York, NY, USA\")."
		},
		"has_remote": {
			"type": "boolean",
			"description": "When true, restricts results to jobs marked Remote eligible."
		},
		"target_level": {
			"type": "string",
			"description": "Experience level filter.",
			"enum": ["EARLY", "MID", "ADVANCED", "INTERN_AND_APPRENTICE", "DIRECTOR_PLUS"]
		},
		"skills": {
			"type": "string",
			"description": "Free-text skills and qualifications filter."
		},
		"degree": {
			"type": "string",
			"description": "Minimum education level filter.",
			"enum": ["PURSUING_DEGREE", "ASSOCIATE", "BACHELORS", "MASTERS", "PHD"]
		},
		"employment_type": {
			"type": "string",
			"description": "Job type filter.",
			"enum": ["FULL_TIME", "PART_TIME", "TEMPORARY", "INTERN"]
		},
		"company": {
			"type": "string",
			"description": "Organization (sub-company) filter.",
			"enum": ["DeepMind", "GFiber", "Google", "Verily Life Sciences", "Waymo", "Wing", "YouTube"]
		},
		"sort_by": {
			"type": "string",
			"description": "Sort order. Defaults to relevance; date sorts newest first.",
			"enum": ["relevance", "date"]
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; 20 results per page.",
			"minimum": 1
		}
	},
	"required": ["keyword", "location"],
	"additionalProperties": false
}`)

var googleSearchInputSchema = mustSchema(googleSearchInputRawSchema)

type googleSearchInput struct {
	Keyword        string `json:"keyword"`  // required
	Location       string `json:"location"` // required
	HasRemote      bool   `json:"has_remote,omitempty"`
	TargetLevel    string `json:"target_level,omitempty"`
	Skills         string `json:"skills,omitempty"`
	Degree         string `json:"degree,omitempty"`
	EmploymentType string `json:"employment_type,omitempty"`
	Company        string `json:"company,omitempty"`
	SortBy         string `json:"sort_by,omitempty"`
	Page           int    `json:"page,omitempty"`
}

type googleSearchOutput struct {
	Data []googleJobSummary `json:"data"`
}

type googleJobSummary struct {
	ID                    string   `json:"id" jsonschema:"Job ID; pass to google_get_job_detail."`
	URL                   string   `json:"url,omitempty" jsonschema:"Public Google Careers job posting URL."`
	Title                 string   `json:"title"`
	Company               string   `json:"company,omitempty"`
	Location              string   `json:"location,omitempty"`
	ExperienceLevel       string   `json:"experience_level,omitempty" jsonschema:"Experience level badge (e.g. Early, Mid, Advanced)."`
	MinimumQualifications []string `json:"minimum_qualifications,omitempty" jsonschema:"Minimum qualifications summary from the search results card."`
}

type googleDetailInput struct {
	JobID string `json:"job_id" jsonschema:"Google job ID (id from search results, e.g. 106863362666570438)."`
}

type googleDetailOutput struct {
	ID               string `json:"id"`
	URL              string `json:"url,omitempty" jsonschema:"Public Google Careers job posting URL."`
	Title            string `json:"title"`
	Company          string `json:"company,omitempty"`
	Location         string `json:"location,omitempty"`
	About            string `json:"about,omitempty" jsonschema:"About-the-job section as plain text."`
	Qualifications   string `json:"qualifications,omitempty" jsonschema:"Minimum and preferred qualifications as plain text."`
	Responsibilities string `json:"responsibilities,omitempty" jsonschema:"Job responsibilities as plain text."`
}

// googleMCPToHTTPRequest maps tool input onto the provider request. Enum
// values pass through verbatim; the input schema already constrains them, and
// the site silently ignores unrecognized values.
func googleMCPToHTTPRequest(in *googleSearchInput) (*google.JobsRequest, error) {
	var req google.JobsRequest
	// The schema already marks keyword and location required; this guards
	// direct callers and clients that skip schema validation.
	if in.Keyword == "" {
		return nil, fmt.Errorf("keyword is required")
	}
	req.Query = in.Keyword

	if in.Location == "" {
		return nil, fmt.Errorf("location is required")
	}
	req.Locations = []string{in.Location}

	req.HasRemote = in.HasRemote
	if in.TargetLevel != "" {
		req.TargetLevels = []string{in.TargetLevel}
	}
	req.Skills = in.Skills
	if in.Degree != "" {
		req.Degrees = []string{in.Degree}
	}
	if in.EmploymentType != "" {
		req.EmploymentType = []string{in.EmploymentType}
	}
	if in.Company != "" {
		req.Companies = []string{in.Company}
	}
	req.SortBy = in.SortBy
	req.Page = in.Page
	return &req, nil
}

func googleHTTPToMCPResponse(resp *google.JobsResponse) *googleSearchOutput {
	out := &googleSearchOutput{
		Data: make([]googleJobSummary, 0, len(resp.Jobs)),
	}
	for _, j := range resp.Jobs {
		out.Data = append(out.Data, googleJobSummary{
			ID:                    j.ID,
			URL:                   googleJobURL(j.ID),
			Title:                 j.Title,
			Company:               j.Company,
			Location:              j.Location,
			ExperienceLevel:       j.ExperienceLevel,
			MinimumQualifications: j.MinimumQualifications,
		})
	}
	return out
}

func googleHTTPToMCPDetail(detail *google.JobDetailResponse) *googleDetailOutput {
	return &googleDetailOutput{
		ID:               detail.ID,
		URL:              googleJobURL(detail.ID),
		Title:            detail.Title,
		Company:          detail.Company,
		Location:         detail.Location,
		About:            detail.About,
		Qualifications:   detail.Qualifications,
		Responsibilities: detail.Responsibilities,
	}
}

func googleJobURL(id string) string {
	if id == "" {
		return ""
	}
	return "https://www.google.com/about/careers/applications/jobs/results/" + id
}

// RegisterGoogle registers the Google Careers search and job-detail tools.
func RegisterGoogle(s *mcp.Server, c *google.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "google_search_jobs",
		Description: "Search jobs on the Google Careers site by keyword and location, with optional remote/experience-level/skills/degree/employment-type/company/sort filters.",
		InputSchema: googleSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *googleSearchInput) (*mcp.CallToolResult, *googleSearchOutput, error) {
		req, err := googleMCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.Jobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, googleHTTPToMCPResponse(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "google_get_job_detail",
		Description: "Get the full job description and requirements for a Google Careers job by job ID.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *googleDetailInput) (*mcp.CallToolResult, *googleDetailOutput, error) {
		res, err := c.JobDetail(ctx, in.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, googleHTTPToMCPDetail(res), nil
	})
}
