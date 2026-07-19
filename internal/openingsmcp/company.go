package openingsmcp

import (
	"context"
	"errors"
	"slices"

	"github.com/amikai/openings-mcp/internal/ats"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The unified company tools front internal/ats: one company parameter, ATS
// invisible. Search input needs a hand-written schema because filters is
// an open map whose keys are tenant-specific and only known at runtime via
// get_filters_by_company.
var companySearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"company": {
			"type": "string",
			"description": "Company name or slug, e.g. 'nvidia', or a recognized public careers-page URL on a supported ATS. Other careers URLs are unsupported; some ATS providers accept URLs only for companies in the curated roster.",
			"minLength": 1
		},
		"query": {
			"type": "string",
			"description": "Free-text keywords: role titles, skills, or technologies. Never put locations or employment types here."
		},
		"location": {
			"type": "string",
			"description": "Location as fuzzy text, e.g. 'Tel Aviv' or 'Taiwan'; 'remote' matches remote-friendly jobs. Omit to search everywhere."
		},
		"filters": {
			"type": "object",
			"description": "Optional precise filters. Keys and values are company-specific; discover them with get_filters_by_company. Multiple values for one key are OR'd; different keys are AND'd.",
			"additionalProperties": {
				"type": "array",
				"minItems": 1,
				"uniqueItems": true,
				"items": {
					"type": "string",
					"minLength": 1
				}
			}
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; each page returns at most 20 jobs.",
			"minimum": 1,
			"default": 1
		}
	},
	"required": ["company"],
	"additionalProperties": false
}`)

var companySearchInputSchema = mustSchema(companySearchInputRawSchema)

type companySearchInput struct {
	Company  string              `json:"company"`
	Query    string              `json:"query,omitempty"`
	Location string              `json:"location,omitempty"`
	Filters  map[string][]string `json:"filters,omitempty"`
	Page     int                 `json:"page,omitempty"`
}

type companyJobSummary struct {
	JobID    string `json:"job_id" jsonschema:"Opaque job identifier; pass to get_job_detail_by_company's job_id param."`
	Title    string `json:"title"`
	Location string `json:"location,omitempty"`
	PostedAt string `json:"posted_at,omitempty"`
	URL      string `json:"url,omitempty" jsonschema:"Public job posting URL."`
}

type companySearchOutput struct {
	Data       []companyJobSummary `json:"data"`
	TotalCount int                 `json:"total_count"`
	Page       int                 `json:"page"`
	TotalPages int                 `json:"total_pages"`
}

func companySearch(ctx context.Context, reg *ats.Registry, in *companySearchInput) (*companySearchOutput, error) {
	resolved, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	params := ats.SearchParams{
		Query:    in.Query,
		Location: in.Location,
		Filters:  in.Filters,
		Page:     in.Page,
	}
	out := &companySearchOutput{Data: []companyJobSummary{}}
	var errs []error
	for _, rc := range resolved {
		res, err := rc.Adapter.Search(ctx, rc.Slug, params)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, j := range res.Jobs {
			out.Data = append(out.Data, companyJobSummary{
				JobID:    j.JobID,
				Title:    j.Title,
				Location: j.Location,
				PostedAt: j.PostedAt,
				URL:      j.URL,
			})
		}
		out.TotalCount += res.TotalCount
		out.Page = res.Page
		out.TotalPages = max(out.TotalPages, res.TotalPages)
	}
	if len(errs) == len(resolved) {
		return nil, errors.Join(errs...)
	}
	return out, nil
}

type companyFiltersInput struct {
	Company string `json:"company" jsonschema:"Company name or slug, or a recognized public careers-page URL on a supported ATS. Other careers URLs are unsupported; some ATS providers accept URLs only for companies in the curated roster."`
}

type companyFiltersOutput struct {
	Filters map[string][]string `json:"filters" jsonschema:"Filter dimension to its currently valid values. Pass any subset to search_jobs_by_company's filters param."`
}

func companyFilters(ctx context.Context, reg *ats.Registry, in *companyFiltersInput) (*companyFiltersOutput, error) {
	resolved, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	merged := ats.FilterSet{}
	var errs []error
	for _, rc := range resolved {
		fs, err := rc.Adapter.Filters(ctx, rc.Slug)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for dim, vals := range fs {
			for _, v := range vals {
				if !slices.Contains(merged[dim], v) {
					merged[dim] = append(merged[dim], v)
				}
			}
		}
	}
	if len(errs) == len(resolved) {
		return nil, errors.Join(errs...)
	}
	return &companyFiltersOutput{Filters: merged}, nil
}

type companyDetailInput struct {
	Company string `json:"company" jsonschema:"Company name or slug, or a recognized public careers-page URL on a supported ATS. Other careers URLs are unsupported; some ATS providers accept URLs only for companies in the curated roster."`
	JobID   string `json:"job_id" jsonschema:"job_id from search_jobs_by_company results."`
}

type companyDetailOutput struct {
	JobID       string `json:"job_id"`
	Title       string `json:"title"`
	Company     string `json:"company,omitempty"`
	Location    string `json:"location,omitempty"`
	PostedAt    string `json:"posted_at,omitempty"`
	URL         string `json:"url,omitempty" jsonschema:"Public job posting URL."`
	Description string `json:"description,omitempty" jsonschema:"Full job description as plain text."`
}

func companyDetail(ctx context.Context, reg *ats.Registry, in *companyDetailInput) (*companyDetailOutput, error) {
	resolved, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	// The job_id belongs to exactly one adapter; take the first that has it.
	var errs []error
	for _, rc := range resolved {
		d, err := rc.Adapter.Detail(ctx, rc.Slug, in.JobID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		return &companyDetailOutput{
			JobID:       d.JobID,
			Title:       d.Title,
			Company:     d.Company,
			Location:    d.Location,
			PostedAt:    d.PostedAt,
			URL:         d.URL,
			Description: d.Description,
		}, nil
	}
	return nil, errors.Join(errs...)
}

// RegisterCompany registers the unified company-parameterized job tools.
func RegisterCompany(s *mcp.Server, reg *ats.Registry) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_jobs_by_company",
		Description: "Search official job postings for a specific company.",
		Annotations: &mcp.ToolAnnotations{Title: "Search jobs by company", ReadOnlyHint: true},
		InputSchema: companySearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *companySearchInput) (*mcp.CallToolResult, *companySearchOutput, error) {
		out, err := companySearch(ctx, reg, in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_filters_by_company",
		Description: "Get company-specific filters when a job search needs narrowing beyond query and location.",
		Annotations: &mcp.ToolAnnotations{Title: "Get company job filters", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *companyFiltersInput) (*mcp.CallToolResult, *companyFiltersOutput, error) {
		out, err := companyFilters(ctx, reg, in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_job_detail_by_company",
		Description: "Get one job's full description (plain text) by company plus the job_id from search_jobs_by_company.",
		Annotations: &mcp.ToolAnnotations{Title: "Get company job detail", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *companyDetailInput) (*mcp.CallToolResult, *companyDetailOutput, error) {
		out, err := companyDetail(ctx, reg, in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, out, nil
	})
}
