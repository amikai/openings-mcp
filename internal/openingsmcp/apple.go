package openingsmcp

import (
	"cmp"
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/amikai/openings-mcp/internal/provider/apple"
)

var appleSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text keyword query. Apple ranks matches by relevance unless sort is newest.",
			"minLength": 1
		},
		"country_code": {
			"type": "string",
			"description": "ISO 3166-1 alpha-3 country code, such as TWN, USA, JPN, or SGP.",
			"pattern": "^[A-Za-z]{3}$"
		},
		"sort": {
			"type": "string",
			"description": "Result order. Relevance ranks against keyword; newest orders by posting date.",
			"enum": ["relevance", "newest"],
			"default": "relevance"
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; a full page has 20 results.",
			"minimum": 1,
			"default": 1
		}
	},
	"required": ["keyword", "country_code"],
	"additionalProperties": false
}`)

var appleSearchInputSchema = mustSchema(appleSearchInputRawSchema)

const (
	appleSearchToolName = "apple_search_jobs"
	appleDetailToolName = "apple_get_job_detail"
)

type appleSearchInput struct {
	Keyword     string `json:"keyword"`
	CountryCode string `json:"country_code"`
	Sort        string `json:"sort,omitempty"`
	Page        int    `json:"page,omitempty"`
}

type appleSearchOutput struct {
	Data  []appleJobSummary `json:"data"`
	Total int               `json:"total" jsonschema:"Total ranked matches across all 20-result pages."`
	Page  int               `json:"page"`
}

type appleJobSummary struct {
	JobID       string   `json:"job_id" jsonschema:"Numeric Apple position ID; pass to apple_get_job_detail."`
	URL         string   `json:"url" jsonschema:"Public jobs.apple.com posting URL."`
	Title       string   `json:"title"`
	Team        string   `json:"team,omitempty"`
	PostedOn    string   `json:"posted_on,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Locations   []string `json:"locations,omitempty"`
	WeeklyHours float64  `json:"weekly_hours,omitempty"`
	HomeOffice  bool     `json:"home_office,omitempty"`
}

type appleDetailInput struct {
	JobID string `json:"job_id" jsonschema:"Numeric Apple position ID from apple_search_jobs, e.g. 200624996."`
}

type appleDetailOutput struct {
	JobID                   string   `json:"job_id"`
	URL                     string   `json:"url" jsonschema:"Public jobs.apple.com posting URL."`
	Title                   string   `json:"title"`
	Summary                 string   `json:"summary,omitempty"`
	Description             string   `json:"description,omitempty"`
	Responsibilities        string   `json:"responsibilities,omitempty"`
	MinimumQualifications   string   `json:"minimum_qualifications,omitempty"`
	PreferredQualifications string   `json:"preferred_qualifications,omitempty"`
	EmploymentType          string   `json:"employment_type,omitempty"`
	PostedOn                string   `json:"posted_on,omitempty"`
	Teams                   []string `json:"teams,omitempty"`
	Locations               []string `json:"locations,omitempty"`
	HomeOffice              bool     `json:"home_office,omitempty"`
}

func appleMCPToHTTPRequest(input *appleSearchInput) apple.SearchRequest {
	return apple.SearchRequest{
		Keyword:     input.Keyword,
		CountryCode: input.CountryCode,
		Sort:        apple.Sort(input.Sort),
		Page:        cmp.Or(input.Page, 1),
	}
}

func appleHTTPToMCPResponse(page int, response *apple.SearchResponse) *appleSearchOutput {
	output := &appleSearchOutput{
		Total: response.Res.TotalRecords,
		Page:  page,
		Data:  make([]appleJobSummary, 0, len(response.Res.SearchResults)),
	}
	for _, job := range response.Res.SearchResults {
		locations := make([]string, 0, len(job.Locations))
		for _, location := range job.Locations {
			locations = append(locations, appleLocationLabel(location.Name, location.CountryName))
		}
		output.Data = append(output.Data, appleJobSummary{
			JobID:       job.PositionId,
			URL:         apple.JobURL(job.PositionId, job.TransformedPostingTitle),
			Title:       job.PostingTitle,
			Team:        job.Team.TeamName,
			Locations:   locations,
			PostedOn:    job.PostingDate,
			WeeklyHours: job.StandardWeeklyHours,
			HomeOffice:  job.HomeOffice,
			Summary:     job.JobSummary,
		})
	}
	return output
}

func appleHTTPToMCPDetail(response *apple.JobDetailResponse) *appleDetailOutput {
	job := response.Res
	locations := make([]string, 0, len(job.Locations))
	for _, location := range job.Locations {
		locations = append(locations, appleLocationLabel(location.Name, location.CountryName))
	}
	return &appleDetailOutput{
		JobID:                   job.PositionId,
		URL:                     apple.JobURL(job.PositionId, job.TransformedPostingTitle),
		Title:                   job.PostingTitle,
		Summary:                 job.JobSummary.Or(""),
		Description:             job.Description.Or(""),
		Responsibilities:        job.Responsibilities.Or(""),
		MinimumQualifications:   job.MinimumQualifications.Or(""),
		PreferredQualifications: job.PreferredQualifications.Or(""),
		Teams:                   job.TeamNames,
		Locations:               locations,
		EmploymentType:          job.EmploymentType.Or(""),
		PostedOn:                job.PostingDate,
		HomeOffice:              job.HomeOffice,
	}
}

func appleLocationLabel(name, country string) string {
	if name == "" {
		return country
	}
	if country == "" || strings.EqualFold(name, country) {
		return name
	}
	return name + ", " + country
}

// RegisterApple registers the Apple Careers search and job-detail tools.
func RegisterApple(server *mcp.Server, client *apple.JobsClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        appleSearchToolName,
		Description: "Search jobs on Apple Careers by keyword and ISO alpha-3 country code.",
		Annotations: &mcp.ToolAnnotations{Title: "Search Apple Careers jobs", ReadOnlyHint: true},
		InputSchema: appleSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input *appleSearchInput) (*mcp.CallToolResult, *appleSearchOutput, error) {
		request := appleMCPToHTTPRequest(input)
		response, err := client.SearchJobs(ctx, request)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, appleHTTPToMCPResponse(request.Page, response), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        appleDetailToolName,
		Description: "Get the full posting, responsibilities, and qualifications for an Apple job by numeric position ID.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Apple job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input *appleDetailInput) (*mcp.CallToolResult, *appleDetailOutput, error) {
		response, err := client.JobDetail(ctx, input.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, appleHTTPToMCPDetail(response), nil
	})
}
