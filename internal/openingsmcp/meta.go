package openingsmcp

import (
	"context"

	"github.com/jaytaylor/html2text"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/amikai/openings-mcp/internal/provider/meta"
)

var metaSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text keyword query matched server-side.",
			"minLength": 1
		},
		"teams": {
			"type": "array",
			"items": {"type": "string"},
			"description": "Team display names, options from meta_get_search_filters."
		},
		"sub_teams": {
			"type": "array",
			"items": {"type": "string"},
			"description": "Sub-team display names. Not enumerated by any filter endpoint; take values from search results' sub_teams."
		},
		"offices": {
			"type": "array",
			"items": {"type": "string"},
			"description": "Office display names or IDs, options from meta_get_search_filters; both forms match."
		},
		"technologies": {
			"type": "array",
			"items": {"type": "string"},
			"description": "Product filter, options from meta_get_search_filters."
		},
		"roles": {
			"type": "array",
			"items": {"type": "string"},
			"description": "Employment types, options from meta_get_search_filters."
		},
		"is_remote_only": {
			"type": "boolean",
			"description": "Only remote-eligible roles.",
			"default": false
		},
		"is_leadership": {
			"type": "boolean",
			"description": "Only leadership roles.",
			"default": false
		},
		"sort_by_new": {
			"type": "boolean",
			"description": "Order by posting date instead of relevance.",
			"default": false
		}
	},
	"additionalProperties": false
}`)

var metaSearchInputSchema = mustSchema(metaSearchInputRawSchema)

const (
	metaSearchToolName  = "meta_search_jobs"
	metaDetailToolName  = "meta_get_job_detail"
	metaFiltersToolName = "meta_get_search_filters"
)

type metaSearchInput struct {
	Keyword      string   `json:"keyword,omitempty"`
	Teams        []string `json:"teams,omitempty"`
	SubTeams     []string `json:"sub_teams,omitempty"`
	Offices      []string `json:"offices,omitempty"`
	Technologies []string `json:"technologies,omitempty"`
	Roles        []string `json:"roles,omitempty"`
	IsRemoteOnly bool     `json:"is_remote_only,omitempty"`
	IsLeadership bool     `json:"is_leadership,omitempty"`
	SortByNew    bool     `json:"sort_by_new,omitempty"`
}

type metaFiltersInput struct{}

type metaFiltersOutput struct {
	Teams        []string       `json:"teams"`
	Technologies []string       `json:"technologies"`
	Roles        []string       `json:"roles"`
	Offices      []metaLocation `json:"offices" jsonschema:"id and display_name both match."`
}

type metaLocation struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	State       string `json:"state,omitempty"`
	Country     string `json:"country,omitempty"`
	IsRemote    bool   `json:"is_remote,omitempty"`
}

type metaSearchOutput struct {
	Data  []metaJobSummary `json:"data"`
	Total int              `json:"total" jsonschema:"Total matches. Search is unpaginated: data always carries every match, so narrow with filters when this is large."`
}

type metaJobSummary struct {
	JobID     string   `json:"job_id" jsonschema:"Numeric requisition ID; pass to meta_get_job_detail."`
	URL       string   `json:"url" jsonschema:"Public metacareers.com posting URL."`
	Title     string   `json:"title"`
	Locations []string `json:"locations,omitempty"`
	Teams     []string `json:"teams,omitempty"`
	SubTeams  []string `json:"sub_teams,omitempty"`
	Featured  bool     `json:"featured,omitempty" jsonschema:"Listed in the site's small curated Featured Jobs rail."`
}

type metaDetailInput struct {
	JobID string `json:"job_id" jsonschema:"Numeric requisition ID from meta_search_jobs, e.g. 1063741453022215."`
}

type metaCompensation struct {
	CountryCode string `json:"country_code"`
	Minimum     string `json:"minimum,omitempty" jsonschema:"e.g. $201,000/year."`
	Maximum     string `json:"maximum,omitempty"`
	HasBonus    bool   `json:"has_bonus,omitempty"`
	HasEquity   bool   `json:"has_equity,omitempty"`
}

type metaDetailOutput struct {
	JobID                   string             `json:"job_id"`
	URL                     string             `json:"url" jsonschema:"Public metacareers.com posting URL."`
	Title                   string             `json:"title"`
	Locations               []string           `json:"locations,omitempty"`
	Teams                   []string           `json:"teams,omitempty"`
	SubTeams                []string           `json:"sub_teams,omitempty"`
	Description             string             `json:"description,omitempty"`
	Responsibilities        []string           `json:"responsibilities,omitempty"`
	MinimumQualifications   []string           `json:"minimum_qualifications,omitempty"`
	PreferredQualifications []string           `json:"preferred_qualifications,omitempty"`
	Compensation            []metaCompensation `json:"compensation,omitempty" jsonschema:"Public pay ranges per country, where disclosure applies."`
}

func metaMCPToHTTPRequest(input *metaSearchInput) meta.SearchRequest {
	return meta.SearchRequest{
		Q:        input.Keyword,
		Teams:    input.Teams,
		SubTeams: input.SubTeams,
		Offices:  input.Offices,
		// The site's Technology filter submits under the divisions key.
		Divisions:    input.Technologies,
		Roles:        input.Roles,
		IsRemoteOnly: input.IsRemoteOnly,
		IsLeadership: input.IsLeadership,
		SortByNew:    input.SortByNew,
	}
}

func metaHTTPToMCPFilters(filters *meta.SearchFilters) *metaFiltersOutput {
	offices := make([]metaLocation, 0, len(filters.Locations))
	for _, location := range filters.Locations {
		offices = append(offices, metaLocation{
			ID:          location.ID,
			DisplayName: location.DisplayName,
			State:       location.State,
			Country:     location.Country,
			IsRemote:    location.IsRemote,
		})
	}
	return &metaFiltersOutput{
		Teams:        filters.Teams,
		Technologies: filters.Technologies,
		Roles:        filters.Roles,
		Offices:      offices,
	}
}

func metaHTTPToMCPResponse(response *meta.SearchResponse) *metaSearchOutput {
	featured := make(map[string]bool, len(response.FeaturedJobs))
	for _, job := range response.FeaturedJobs {
		featured[job.ID] = true
	}
	output := &metaSearchOutput{
		Total: len(response.AllJobs),
		Data:  make([]metaJobSummary, 0, len(response.AllJobs)),
	}
	for _, job := range response.AllJobs {
		output.Data = append(output.Data, metaJobSummary{
			JobID:     job.ID,
			URL:       meta.JobURL(job.ID),
			Title:     job.Title,
			Locations: job.Locations,
			Teams:     job.Teams,
			SubTeams:  job.SubTeams,
			Featured:  featured[job.ID],
		})
	}
	return output
}

func metaHTTPToMCPDetail(detail *meta.JobDetail) (*metaDetailOutput, error) {
	description, err := html2text.FromString(detail.DescriptionHTML, html2text.Options{})
	if err != nil {
		return nil, err
	}
	compensation := make([]metaCompensation, 0, len(detail.PublicCompensation))
	for _, comp := range detail.PublicCompensation {
		compensation = append(compensation, metaCompensation{
			CountryCode: comp.CountryCode,
			Minimum:     comp.Minimum,
			Maximum:     comp.Maximum,
			HasBonus:    comp.HasBonus,
			HasEquity:   comp.HasEquity,
		})
	}
	return &metaDetailOutput{
		JobID:                   detail.ID,
		URL:                     meta.JobURL(detail.ID),
		Title:                   detail.Title,
		Locations:               detail.Locations,
		Teams:                   detail.Departments,
		SubTeams:                detail.InternalDepartments,
		Description:             description,
		Responsibilities:        detail.Responsibilities,
		MinimumQualifications:   detail.MinimumQualifications,
		PreferredQualifications: detail.PreferredQualifications,
		Compensation:            compensation,
	}, nil
}

// RegisterMeta registers the Meta Careers search and job-detail tools.
func RegisterMeta(server *mcp.Server, client *meta.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        metaSearchToolName,
		Description: "Search jobs on the Meta careers site.",
		Annotations: &mcp.ToolAnnotations{Title: "Search Meta Careers jobs", ReadOnlyHint: true},
		InputSchema: metaSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input *metaSearchInput) (*mcp.CallToolResult, *metaSearchOutput, error) {
		response, err := client.SearchJobs(ctx, metaMCPToHTTPRequest(input))
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, metaHTTPToMCPResponse(response), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        metaFiltersToolName,
		Description: "List Meta Careers' current search filter values: teams, technologies (products), employment types, and offices. Call before filtered meta_search_jobs queries when unsure of exact values.",
		Annotations: &mcp.ToolAnnotations{Title: "List Meta Careers search filters", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ *metaFiltersInput) (*mcp.CallToolResult, *metaFiltersOutput, error) {
		filters, err := client.SearchFilters(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, metaHTTPToMCPFilters(filters), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        metaDetailToolName,
		Description: "Get the full posting, responsibilities, qualifications, and public pay ranges for a Meta job by requisition ID.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Meta job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input *metaDetailInput) (*mcp.CallToolResult, *metaDetailOutput, error) {
		detail, err := client.JobDetail(ctx, input.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		output, err := metaHTTPToMCPDetail(detail)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, output, nil
	})
}
