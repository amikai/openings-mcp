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
			"description": "Company name or slug, e.g. 'nvidia', or a recognized public careers-page URL. Other careers URLs are unsupported; some career systems accept URLs only for companies in the curated roster. If more than one company matches and the client supports elicitation, the tool asks the user to choose using company names and public careers URLs.",
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
	Company string `json:"company" jsonschema:"Company name or slug, or a recognized public careers-page URL. Other careers URLs are unsupported; some career systems accept URLs only for companies in the curated roster. If more than one company matches and the client supports elicitation, the tool asks the user to choose using company names and public careers URLs."`
}

type companyFiltersOutput struct {
	Filters map[string][]string `json:"filters" jsonschema:"Filter dimension to its currently valid values. Pass any subset to search_jobs_by_company's filters param."`
}

const companySelectionRequestID = "company_selection"

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

// resolveCompanyForTool resolves unique company input immediately. For an
// ambiguous input, it either consumes the client's elicitation response or
// pauses the tool call with a form request that contains human-readable
// company choices.
func resolveCompanyForTool(
	req *mcp.CallToolRequest,
	reg *ats.Registry,
	input string,
	selectionHint string,
) (ats.Adapter, string, *mcp.CallToolResult, error) {
	resolution, err := reg.Resolve(input)
	if err != nil {
		return nil, "", nil, err
	}

	if !resolution.IsAmbiguous() {
		adapter, slug, ok := resolution.Select(0)
		if !ok {
			return nil, "", nil, errors.New("company resolution returned no candidates")
		}
		return adapter, slug, nil, nil
	}

	candidates := resolution.Candidates()
	choice, answered, selectionErr := companySelection(req, len(candidates))
	if selectionErr != nil {
		return nil, "", nil, selectionErr
	}
	if answered {
		adapter, slug, ok := resolution.Select(choice)
		if !ok {
			return nil, "", nil, fmt.Errorf("company selection %d is out of range", choice+1)
		}
		return adapter, slug, nil, nil
	}

	if !supportsFormElicitation(req) {
		return nil, "", nil, ambiguousCompanyError(input, candidates, selectionHint)
	}
	return nil, "", companySelectionRequest(input, candidates, selectionHint), nil
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

func companySelectionRequest(
	input string,
	candidates []ats.CompanyCandidate,
	selectionHint string,
) *mcp.CallToolResult {
	var message strings.Builder
	fmt.Fprintf(&message, "More than one company matches %q. Choose the company you mean:", input)
	if selectionHint != "" {
		fmt.Fprintf(&message, " %s", selectionHint)
	}
	message.WriteByte('\n')

	choices := make([]*jsonschema.Schema, 0, len(candidates))
	for i, candidate := range candidates {
		number := strconv.Itoa(i + 1)
		value := any(number)
		title := candidate.Name
		if candidate.CareersURL != "" {
			title += " — " + candidate.CareersURL
		}
		choices = append(choices, &jsonschema.Schema{
			Const: &value,
			Title: title,
		})
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
							Description: "Select the intended company.",
							OneOf:       choices,
						},
					},
				},
			},
		},
	}
}

func ambiguousCompanyError(
	input string,
	candidates []ats.CompanyCandidate,
	selectionHint string,
) error {
	var message strings.Builder
	fmt.Fprintf(&message, "ambiguous company %q: %d companies match.", input, len(candidates))
	if selectionHint != "" {
		fmt.Fprintf(&message, " %s", selectionHint)
	}
	message.WriteString(" This client cannot display a company choice form; retry with the public careers URL shown below, or the exact company name when no URL is available:")
	for _, candidate := range candidates {
		fmt.Fprintf(&message, "\n  %s", candidate.Name)
		if candidate.CareersURL != "" {
			fmt.Fprintf(&message, " — %s", candidate.CareersURL)
		} else {
			fmt.Fprintf(&message, " — retry with company=%q", candidate.Name)
		}
	}
	return errors.New(message.String())
}

type companyDetailInput struct {
	Company string `json:"company" jsonschema:"Company name or slug, or a recognized public careers-page URL. Other careers URLs are unsupported; some career systems accept URLs only for companies in the curated roster. If more than one company matches and the client supports elicitation, the tool asks the user to choose using company names and public careers URLs."`
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
		InputSchema: companySearchInputSchema,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in *companySearchInput) (*mcp.CallToolResult, *companySearchOutput, error) {
		adapter, slug, pending, err := resolveCompanyForTool(req, reg, in.Company, "")
		if err != nil {
			return nil, nil, err
		}
		if pending != nil {
			return pending, nil, nil
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
	}, func(ctx context.Context, req *mcp.CallToolRequest, in *companyFiltersInput) (*mcp.CallToolResult, *companyFiltersOutput, error) {
		adapter, slug, pending, err := resolveCompanyForTool(req, reg, in.Company, "")
		if err != nil {
			return nil, nil, err
		}
		if pending != nil {
			return pending, nil, nil
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
	}, func(ctx context.Context, req *mcp.CallToolRequest, in *companyDetailInput) (*mcp.CallToolResult, *companyDetailOutput, error) {
		const hint = "Choose the same company that produced this job_id."
		adapter, slug, pending, err := resolveCompanyForTool(req, reg, in.Company, hint)
		if err != nil {
			return nil, nil, err
		}
		if pending != nil {
			return pending, nil, nil
		}
		out, err := companyDetail(ctx, adapter, slug, in)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})
}
