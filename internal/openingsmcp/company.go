package openingsmcp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/amikai/openings-mcp/internal/ats"
	"github.com/google/jsonschema-go/jsonschema"
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
			"description": "Company name or slug, e.g. 'nvidia', or a recognized public careers-page URL on a supported ATS. Other careers URLs are unsupported; some ATS providers accept URLs only for companies in the curated roster. If the company is ambiguous, retry with one of the careers URLs listed in the error.",
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
	adapter, slug, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
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
	Company string `json:"company" jsonschema:"Company name or slug, or a recognized public careers-page URL on a supported ATS. Other careers URLs are unsupported; some ATS providers accept URLs only for companies in the curated roster. If the company is ambiguous and the client supports elicitation, the tool asks the user to choose; otherwise it lists careers URLs for a retry."`
}

type companyFiltersOutput struct {
	Filters map[string][]string `json:"filters" jsonschema:"Filter dimension to its currently valid values. Pass any subset to search_jobs_by_company's filters param."`
}

const companySelectionRequestID = "company_selection"

func companyFilters(ctx context.Context, reg *ats.Registry, in *companyFiltersInput) (*companyFiltersOutput, error) {
	adapter, slug, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	fs, err := adapter.Filters(ctx, slug)
	if err != nil {
		return nil, err
	}
	return &companyFiltersOutput{Filters: fs}, nil
}

// companyFiltersTool adds one multi-round-trip step around companyFilters:
// an elicitation-capable client asks the user to choose an ambiguous company,
// then retries the same call with that choice in InputResponses.
func companyFiltersTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	reg *ats.Registry,
	in *companyFiltersInput,
) (*mcp.CallToolResult, *companyFiltersOutput, error) {
	out, err := companyFilters(ctx, reg, in)
	if err == nil {
		return nil, out, nil
	}

	var ambiguous *ats.AmbiguousCompanyError
	if !errors.As(err, &ambiguous) {
		return nil, nil, err
	}

	choice, answered, selectionErr := companySelection(req, len(ambiguous.Candidates))
	if selectionErr != nil {
		return nil, nil, selectionErr
	}
	if answered {
		candidate := ambiguous.Candidates[choice]
		selectedCompany := candidate.CareersURL
		if selectedCompany == "" {
			selectedCompany = candidate.Name
		}
		out, err := companyFilters(ctx, reg, &companyFiltersInput{Company: selectedCompany})
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	}

	if !supportsFormElicitation(req) {
		return nil, nil, ambiguous
	}
	return companySelectionRequest(ambiguous), nil, nil
}

// companySelection reads a form-elicitation response. The returned choice is
// zero-based; answered is false only on the first pass through the handler.
func companySelection(req *mcp.CallToolRequest, candidateCount int) (choice int, answered bool, err error) {
	if len(req.Params.InputResponses) == 0 {
		return 0, false, nil
	}

	raw, ok := req.Params.InputResponses[companySelectionRequestID]
	if !ok {
		return 0, true, errors.New("company selection response is missing")
	}
	response, ok := raw.(*mcp.ElicitResult)
	if !ok {
		return 0, true, fmt.Errorf("company selection response has unexpected type %T", raw)
	}

	switch response.Action {
	case "accept":
		// Continue below.
	case "decline":
		return 0, true, errors.New("company selection declined")
	case "cancel":
		return 0, true, errors.New("company selection cancelled")
	default:
		return 0, true, fmt.Errorf("company selection returned unknown action %q", response.Action)
	}

	value, ok := response.Content["choice"].(string)
	if !ok {
		return 0, true, errors.New("company selection choice must be a string")
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > candidateCount {
		return 0, true, fmt.Errorf("company selection choice %q is out of range", value)
	}
	return n - 1, true, nil
}

func supportsFormElicitation(req *mcp.CallToolRequest) bool {
	capabilities := req.ClientCapabilities()
	if capabilities == nil || capabilities.Elicitation == nil {
		return false
	}
	elicitation := capabilities.Elicitation
	// Per the MCP SDK, an empty elicitation capability means form support.
	return elicitation.Form != nil || elicitation.URL == nil
}

func companySelectionRequest(ambiguous *ats.AmbiguousCompanyError) *mcp.CallToolResult {
	var message strings.Builder
	fmt.Fprintf(&message, "More than one company matches %q. Choose the company you mean:\n", ambiguous.Input)

	choices := make([]any, 0, len(ambiguous.Candidates))
	for i, candidate := range ambiguous.Candidates {
		number := strconv.Itoa(i + 1)
		choices = append(choices, number)
		fmt.Fprintf(&message, "%s. %s", number, candidate.Name)
		if candidate.CareersURL != "" {
			fmt.Fprintf(&message, " — %s", candidate.CareersURL)
		}
		message.WriteByte('\n')
	}

	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{
			companySelectionRequestID: &mcp.ElicitParams{
				Message: strings.TrimRight(message.String(), "\n"),
				RequestedSchema: &jsonschema.Schema{
					Type:     "object",
					Required: []string{"choice"},
					Properties: map[string]*jsonschema.Schema{
						"choice": {
							Type:        "string",
							Title:       "Company",
							Description: "The number of the intended company.",
							Enum:        choices,
						},
					},
				},
			},
		},
	}
}

type companyDetailInput struct {
	Company string `json:"company" jsonschema:"Company name or slug, or a recognized public careers-page URL on a supported ATS. Other careers URLs are unsupported; some ATS providers accept URLs only for companies in the curated roster. If the company is ambiguous, retry with one of the careers URLs listed in the error."`
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
	adapter, slug, err := reg.Resolve(in.Company)
	if err != nil {
		var ambErr *ats.AmbiguousCompanyError
		if errors.As(err, &ambErr) {
			return nil, fmt.Errorf("%w\nuse the same company value that produced this job_id", err)
		}
		return nil, err
	}
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
	}, func(ctx context.Context, req *mcp.CallToolRequest, in *companyFiltersInput) (*mcp.CallToolResult, *companyFiltersOutput, error) {
		return companyFiltersTool(ctx, req, reg, in)
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
