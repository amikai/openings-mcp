package openingsmcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/jaytaylor/html2text"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/amikai/openings-mcp/internal/provider/amazon"
)

var amazonSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text search across Amazon job titles and descriptions.",
			"minLength": 1
		},
		"country": {
			"type": "string",
			"description": "ISO 3166-1 alpha-3 country code, e.g. TWN, USA, GBR.",
			"pattern": "^[A-Z]{3}$"
		},
		"city": {
			"type": "string",
			"description": "Normalized city name, e.g. Taipei City or Seattle.",
			"minLength": 1
		},
		"job_category": {
			"type": "string",
			"description": "Amazon job category display name, e.g. Software Development.",
			"minLength": 1
		},
		"business_category": {
			"type": "string",
			"description": "Amazon business-category slug, e.g. aws or alexa-and-amazon-devices.",
			"minLength": 1
		},
		"schedule_type": {
			"type": "string",
			"description": "Schedule type display value, e.g. Full-Time.",
			"minLength": 1
		},
		"sort": {
			"type": "string",
			"description": "Result order: relevance or newest first. Defaults to relevant.",
			"enum": ["relevant", "recent"],
			"default": "relevant"
		},
		"offset": {
			"type": "integer",
			"description": "Zero-based result offset.",
			"minimum": 0,
			"default": 0
		},
		"limit": {
			"type": "integer",
			"description": "Page size.",
			"minimum": 1,
			"maximum": 100,
			"default": 10
		}
	},
	"additionalProperties": false
}`)

var amazonSearchInputSchema = mustSchema(amazonSearchInputRawSchema)

type amazonSearchInput struct {
	Keyword          string `json:"keyword,omitempty"`
	Country          string `json:"country,omitempty"`
	City             string `json:"city,omitempty"`
	JobCategory      string `json:"job_category,omitempty"`
	BusinessCategory string `json:"business_category,omitempty"`
	ScheduleType     string `json:"schedule_type,omitempty"`
	Sort             string `json:"sort,omitempty"`
	Offset           int    `json:"offset,omitempty"`
	Limit            int    `json:"limit,omitempty"`
}

type amazonSearchOutput struct {
	Total  int                `json:"total" jsonschema:"Total number of matching jobs."`
	Offset int                `json:"offset" jsonschema:"Zero-based offset of this page."`
	Data   []amazonJobSummary `json:"data"`
}

type amazonJobSummary struct {
	ID                 string `json:"id" jsonschema:"Amazon numeric job id; pass to amazon_get_job_detail."`
	URL                string `json:"url" jsonschema:"Public Amazon Jobs posting URL."`
	Title              string `json:"title"`
	Location           string `json:"location,omitempty"`
	NormalizedLocation string `json:"normalized_location,omitempty"`
	CountryCode        string `json:"country_code,omitempty" jsonschema:"ISO 3166-1 alpha-3 country code."`
	CompanyName        string `json:"company_name,omitempty" jsonschema:"Amazon legal employing entity."`
	JobCategory        string `json:"job_category,omitempty"`
	BusinessCategory   string `json:"business_category,omitempty" jsonschema:"Amazon business-category slug."`
	ScheduleType       string `json:"schedule_type,omitempty"`
	PostedDate         string `json:"posted_date,omitempty"`
	UpdatedTime        string `json:"updated_time,omitempty"`
	Description        string `json:"description,omitempty" jsonschema:"Plain-text preview; use amazon_get_job_detail for the full posting."`
}

type amazonDetailInput struct {
	JobID string `json:"job_id" jsonschema:"Numeric id from amazon_search_jobs."`
}

type amazonDetailOutput struct {
	ID                      string `json:"id"`
	URL                     string `json:"url" jsonschema:"Public Amazon Jobs posting URL."`
	ApplyURL                string `json:"apply_url,omitempty"`
	Title                   string `json:"title"`
	Location                string `json:"location,omitempty"`
	NormalizedLocation      string `json:"normalized_location,omitempty"`
	CountryCode             string `json:"country_code,omitempty" jsonschema:"ISO 3166-1 alpha-3 country code."`
	CompanyName             string `json:"company_name,omitempty" jsonschema:"Amazon legal employing entity."`
	JobCategory             string `json:"job_category,omitempty"`
	BusinessCategory        string `json:"business_category,omitempty" jsonschema:"Amazon business-category slug."`
	ScheduleType            string `json:"schedule_type,omitempty"`
	PostedDate              string `json:"posted_date,omitempty"`
	UpdatedTime             string `json:"updated_time,omitempty"`
	Description             string `json:"description" jsonschema:"Full job description as plain text."`
	BasicQualifications     string `json:"basic_qualifications" jsonschema:"Required qualifications as plain text."`
	PreferredQualifications string `json:"preferred_qualifications" jsonschema:"Preferred qualifications as plain text."`
}

func amazonMCPToSearchRequest(input *amazonSearchInput) (*amazon.SearchRequest, error) {
	if input.Country != "" {
		if !amazonIsISO3(input.Country) {
			return nil, fmt.Errorf("invalid country %q: expected an uppercase ISO-3 code", input.Country)
		}
	}

	sort := amazon.SearchJobsSortRelevant
	if input.Sort != "" {
		sort = amazon.SearchJobsSort(input.Sort)
		if err := sort.Validate(); err != nil {
			return nil, fmt.Errorf("invalid sort %q: %w", input.Sort, err)
		}
	}
	return &amazon.SearchRequest{
		Query:              input.Keyword,
		Countries:          amazonSingleFilter(input.Country),
		Cities:             amazonSingleFilter(input.City),
		JobCategories:      amazonSingleFilter(input.JobCategory),
		BusinessCategories: amazonSingleFilter(input.BusinessCategory),
		ScheduleTypes:      amazonSingleFilter(input.ScheduleType),
		Sort:               sort,
		Offset:             input.Offset,
		Limit:              input.Limit,
	}, nil
}

func amazonIsISO3(value string) bool {
	if len(value) != 3 || strings.ToUpper(value) != value {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func amazonSingleFilter(value string) []string {
	if value == "" {
		return []string{}
	}
	return []string{value}
}

func amazonHTTPToMCPResponse(result *amazon.SearchResult, offset int) *amazonSearchOutput {
	output := &amazonSearchOutput{
		Total:  result.Total,
		Offset: offset,
		Data:   make([]amazonJobSummary, 0, len(result.Jobs)),
	}
	for _, job := range result.Jobs {
		output.Data = append(output.Data, amazonJobSummary{
			ID:                 job.IDIcims,
			URL:                amazon.JobURL(job.JobPath),
			Title:              job.Title,
			Location:           job.Location,
			NormalizedLocation: job.NormalizedLocation,
			CountryCode:        job.CountryCode,
			CompanyName:        job.CompanyName,
			JobCategory:        job.JobCategory,
			BusinessCategory:   job.BusinessCategory,
			ScheduleType:       job.JobScheduleType,
			PostedDate:         job.PostedDate,
			UpdatedTime:        job.UpdatedTime,
			Description:        job.DescriptionShort,
		})
	}
	return output
}

func amazonHTTPToMCPDetail(job *amazon.Job) *amazonDetailOutput {
	return &amazonDetailOutput{
		ID:                      job.IDIcims,
		URL:                     amazon.JobURL(job.JobPath),
		ApplyURL:                job.URLNextStep.String(),
		Title:                   job.Title,
		Location:                job.Location,
		NormalizedLocation:      job.NormalizedLocation,
		CountryCode:             job.CountryCode,
		CompanyName:             job.CompanyName,
		JobCategory:             job.JobCategory,
		BusinessCategory:        job.BusinessCategory,
		ScheduleType:            job.JobScheduleType,
		PostedDate:              job.PostedDate,
		UpdatedTime:             job.UpdatedTime,
		Description:             amazonHTMLToText(job.Description),
		BasicQualifications:     amazonHTMLToText(job.BasicQualifications),
		PreferredQualifications: amazonHTMLToText(job.PreferredQualifications),
	}
}

func amazonHTMLToText(value string) string {
	text, err := html2text.FromString(value, html2text.Options{})
	if err != nil {
		return value
	}
	return text
}

// RegisterAmazon registers the Amazon Jobs search and detail tools.
func RegisterAmazon(server *mcp.Server, client *amazon.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "amazon_search_jobs",
		Description: "Search official job postings on Amazon Jobs.",
		Annotations: &mcp.ToolAnnotations{Title: "Search Amazon jobs", ReadOnlyHint: true},
		InputSchema: amazonSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input *amazonSearchInput) (*mcp.CallToolResult, *amazonSearchOutput, error) {
		request, err := amazonMCPToSearchRequest(input)
		if err != nil {
			return errorResult(err), nil, nil
		}
		result, err := client.Search(ctx, *request)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, amazonHTTPToMCPResponse(result, request.Offset), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "amazon_get_job_detail",
		Description: "Get the full description and qualifications for a job from amazon_search_jobs.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Amazon job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input *amazonDetailInput) (*mcp.CallToolResult, *amazonDetailOutput, error) {
		job, err := client.JobDetail(ctx, input.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, amazonHTTPToMCPDetail(job), nil
	})
}
