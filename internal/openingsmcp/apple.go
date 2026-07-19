package openingsmcp

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/amikai/openings-mcp/internal/provider/apple"
)

var appleSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text keyword query.",
			"minLength": 1
		},
		"country_code": {
			"type": "string",
			"description": "ISO 3166-1 alpha-3 country code, e.g. TWN or USA. Required unless locations is set.",
			"pattern": "^[A-Za-z]{3}$"
		},
		"locations": {
			"type": "array",
			"description": "Case-sensitive location codes at any granularity (state, metro, or city), e.g. TPEI or state953, OR'd with country_code. Required unless country_code is set.",
			"items": {"type": "string", "pattern": "^[A-Za-z0-9]+$"}
		},
		"sort": {
			"type": "string",
			"description": "Result order. Defaults to relevance.",
			"enum": ["relevance", "newest", "teamAsc", "teamDesc", "locationAsc", "locationDesc"],
			"default": "relevance"
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; 20 results per page.",
			"minimum": 1,
			"default": 1
		},
		"home_office": {
			"type": "boolean",
			"description": "Only remote-eligible postings."
		},
		"keywords": {
			"type": "array",
			"description": "Extra keyword filter chips.",
			"items": {"type": "string", "minLength": 1}
		},
		"teams": {
			"type": "array",
			"description": "TEAM/SUBTEAM pairs, options from apple_get_search_filters.",
			"items": {"type": "string", "pattern": "^[A-Za-z0-9]+/[A-Za-z0-9]+$"}
		},
		"products": {
			"type": "array",
			"description": "Product codes, options from apple_get_search_filters.",
			"items": {"type": "string", "pattern": "^[A-Za-z0-9]+$"}
		},
		"languages": {
			"type": "array",
			"description": "Language codes, e.g. en_US or zh_HK.",
			"items": {"type": "string", "pattern": "^[A-Za-z_]+$"}
		}
	},
	"required": ["keyword"],
	"additionalProperties": false
}`)

var appleSearchInputSchema = mustSchema(appleSearchInputRawSchema)

const (
	appleSearchToolName  = "apple_search_jobs"
	appleDetailToolName  = "apple_get_job_detail"
	appleFiltersToolName = "apple_get_search_filters"
)

type appleSearchInput struct {
	Keyword     string   `json:"keyword"`
	CountryCode string   `json:"country_code,omitempty"`
	Sort        string   `json:"sort,omitempty"`
	Locations   []string `json:"locations,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Teams       []string `json:"teams,omitempty"`
	Products    []string `json:"products,omitempty"`
	Languages   []string `json:"languages,omitempty"`
	Page        int      `json:"page,omitempty"`
	HomeOffice  bool     `json:"home_office,omitempty"`
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

type appleFilterOption struct {
	Value string `json:"value" jsonschema:"Code to pass to apple_search_jobs."`
	Name  string `json:"name" jsonschema:"Human-readable label."`
}

type appleFiltersOutput struct {
	Teams    []appleFilterOption `json:"teams" jsonschema:"TEAM/SUBTEAM pairs for apple_search_jobs teams, fetched live from Apple."`
	Products []appleFilterOption `json:"products" jsonschema:"Product codes for apple_search_jobs products."`
}

func appleHTTPToMCPFilters(teams *apple.TeamsResponse) *appleFiltersOutput {
	output := &appleFiltersOutput{
		Products: make([]appleFilterOption, 0, len(apple.Products)),
	}
	for _, group := range teams.Res {
		for _, subTeam := range group.Teams {
			output.Teams = append(output.Teams, appleFilterOption{
				Value: subTeam.TeamCode + "/" + subTeam.Code,
				Name:  subTeam.DisplayName,
			})
		}
	}
	for _, product := range apple.Products {
		output.Products = append(output.Products, appleFilterOption{
			Value: product.Code,
			Name:  product.Name,
		})
	}
	return output
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

func appleMCPToHTTPRequest(input *appleSearchInput) (apple.SearchRequest, error) {
	teams := make([]apple.TeamFilter, 0, len(input.Teams))
	for _, value := range input.Teams {
		team, err := apple.ParseTeamFilter(value)
		if err != nil {
			return apple.SearchRequest{}, fmt.Errorf("parse team filter: %w", err)
		}
		teams = append(teams, team)
	}
	return apple.SearchRequest{
		Keyword:     input.Keyword,
		CountryCode: input.CountryCode,
		Locations:   input.Locations,
		Sort:        apple.Sort(input.Sort),
		Page:        cmp.Or(input.Page, 1),
		HomeOffice:  input.HomeOffice,
		Keywords:    input.Keywords,
		Teams:       teams,
		Products:    input.Products,
		Languages:   input.Languages,
	}, nil
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
		Description: "Search jobs on the Apple careers site.",
		Annotations: &mcp.ToolAnnotations{Title: "Search Apple Careers jobs", ReadOnlyHint: true},
		InputSchema: appleSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input *appleSearchInput) (*mcp.CallToolResult, *appleSearchOutput, error) {
		request, err := appleMCPToHTTPRequest(input)
		if err != nil {
			return errorResult(err), nil, nil
		}
		response, err := client.SearchJobs(ctx, request)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, appleHTTPToMCPResponse(request.Page, response), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        appleFiltersToolName,
		Description: "Get Apple Careers' current search filter values. Call before filtered apple_search_jobs queries.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Apple search filters", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ *struct{}) (*mcp.CallToolResult, *appleFiltersOutput, error) {
		teams, err := client.ListTeams(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, appleHTTPToMCPFilters(teams), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        appleDetailToolName,
		Description: "Get the full job description and requirements for an Apple job by job ID.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Apple job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input *appleDetailInput) (*mcp.CallToolResult, *appleDetailOutput, error) {
		response, err := client.JobDetail(ctx, input.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, appleHTTPToMCPDetail(response), nil
	})
}
