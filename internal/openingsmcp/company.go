package openingsmcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/amikai/openings-mcp/internal/ats"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The unified company tools front internal/ats: one company parameter, ATS
// invisible. Search input needs a hand-written schema because filters is
// an open map whose keys are tenant-specific and only known at runtime via
// get_filters_by_company.
var _companySearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"company": {
			"type": "string",
			"description": "Company name or slug, e.g. 'nvidia', or a recognized public careers-page URL. Other careers URLs are unsupported; some career systems accept URLs only for companies in the curated roster. If the company is ambiguous, retry with one of the careers URLs listed in the error.",
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

var _companySearchInputSchema = mustSchema(_companySearchInputRawSchema)

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

func companySearch(
	ctx context.Context,
	adapter ats.Adapter,
	slug string,
	in *companySearchInput,
) (*companySearchOutput, error) {
	res, err := adapter.Search(ctx, slug, ats.SearchParams{
		Query:    in.Query,
		Location: in.Location,
		Filters:  in.Filters,
		Page:     in.Page,
	})
	if err != nil {
		return nil, err
	}
	out := &companySearchOutput{
		Data:       make([]companyJobSummary, 0, len(res.Jobs)),
		TotalCount: res.TotalCount,
		Page:       res.Page,
		TotalPages: res.TotalPages,
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
	return out, nil
}

type companyFiltersInput struct {
	Company string `json:"company" jsonschema:"Company name or slug, or a recognized public careers-page URL. Other careers URLs are unsupported; some career systems accept URLs only for companies in the curated roster. If the company is ambiguous, retry with one of the careers URLs listed in the error."`
}

type companyFiltersOutput struct {
	Filters map[string][]string `json:"filters" jsonschema:"Filter dimension to its currently valid values. Pass any subset to search_jobs_by_company's filters param."`
}

func companyFilters(
	ctx context.Context,
	adapter ats.Adapter,
	slug string,
) (*companyFiltersOutput, error) {
	fs, err := adapter.Filters(ctx, slug)
	if err != nil {
		return nil, err
	}
	return &companyFiltersOutput{Filters: fs}, nil
}

// AmbiguousCompanyError reports that a company input matched more than one
// roster entry. The caller must retry with the careers URL of the intended
// candidate, or with its display name when that candidate has no public URL.
//
// It lives at the MCP boundary rather than in internal/ats because several
// candidates are a successful [ats.CompanyResolution]: whether that needs an
// error depends on what the client can do with the choices. A client able to
// ask its user would consume the same resolution and never build this.
type AmbiguousCompanyError struct {
	Input      string
	Candidates []ats.CompanyCandidate
	// Hint is tool-specific guidance rendered after the candidate list.
	Hint string
}

func (e *AmbiguousCompanyError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "ambiguous company %q: %d companies match. Retry with the careers URL of the one you want:\n",
		e.Input, len(e.Candidates))
	tw := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	for _, c := range e.Candidates {
		if c.CareersURL != "" {
			fmt.Fprintf(tw, "  %s\t%s\n", c.Name, c.CareersURL)
		} else {
			fmt.Fprintf(tw, "  %s\t(no public careers URL; retry with company=%q)\n", c.Name, c.Name)
		}
	}
	tw.Flush()
	if e.Hint != "" {
		b.WriteString(e.Hint)
	}
	return strings.TrimRight(b.String(), "\n")
}

// resolveCompany narrows a company input to the single adapter and
// provider-local slug the tools need. An input matching several roster
// entries yields an [*AmbiguousCompanyError] carrying the same
// human-readable candidates the user would otherwise pick from; hint adds
// tool-specific guidance to it.
func resolveCompany(reg *ats.Registry, input, hint string) (ats.Adapter, string, error) {
	resolution, err := reg.Resolve(input)
	if err != nil {
		return nil, "", err
	}
	if adapter, slug, ok := resolution.Single(); ok {
		return adapter, slug, nil
	}
	if !resolution.IsAmbiguous() {
		return nil, "", errors.New("company resolution returned no candidates")
	}
	return nil, "", &AmbiguousCompanyError{
		Input:      input,
		Candidates: resolution.Candidates(),
		Hint:       hint,
	}
}

type companyDetailInput struct {
	Company string `json:"company" jsonschema:"Company name or slug, or a recognized public careers-page URL. Other careers URLs are unsupported; some career systems accept URLs only for companies in the curated roster. If the company is ambiguous, retry with one of the careers URLs listed in the error."`
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

func companyDetail(
	ctx context.Context,
	adapter ats.Adapter,
	slug string,
	in *companyDetailInput,
) (*companyDetailOutput, error) {
	d, err := adapter.Detail(ctx, slug, in.JobID)
	if err != nil {
		return nil, err
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

// RegisterCompany registers the unified company-parameterized job tools.
func RegisterCompany(s *mcp.Server, reg *ats.Registry) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_jobs_by_company",
		Description: "Search official job postings for a specific company.",
		Annotations: &mcp.ToolAnnotations{Title: "Search jobs by company", ReadOnlyHint: true},
		InputSchema: _companySearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *companySearchInput) (*mcp.CallToolResult, *companySearchOutput, error) {
		adapter, slug, err := resolveCompany(reg, in.Company, "")
		if err != nil {
			return nil, nil, err
		}
		out, err := companySearch(ctx, adapter, slug, in)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_filters_by_company",
		Description: "Get company-specific filters when a job search needs narrowing beyond query and location.",
		Annotations: &mcp.ToolAnnotations{Title: "Get company job filters", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *companyFiltersInput) (*mcp.CallToolResult, *companyFiltersOutput, error) {
		adapter, slug, err := resolveCompany(reg, in.Company, "")
		if err != nil {
			return nil, nil, err
		}
		out, err := companyFilters(ctx, adapter, slug)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_job_detail_by_company",
		Description: "Get one job's full description (plain text) by company plus the job_id from search_jobs_by_company.",
		Annotations: &mcp.ToolAnnotations{Title: "Get company job detail", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *companyDetailInput) (*mcp.CallToolResult, *companyDetailOutput, error) {
		const hint = "Use the same company value that produced this job_id."
		adapter, slug, err := resolveCompany(reg, in.Company, hint)
		if err != nil {
			return nil, nil, err
		}
		out, err := companyDetail(ctx, adapter, slug, in)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})
}
