package openingsmcp

import (
	"context"
	"fmt"

	"github.com/amikai/openings-mcp/internal/provider/flowxtra"
	"github.com/jaytaylor/html2text"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// flowxtraPageSize matches the package-wide pageSize convention in
// internal/ats; the upstream API accepts larger pages but 20 keeps tool
// results economical.
const flowxtraPageSize = 20

var flowxtraSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"query": {
			"type": "string",
			"description": "Free-text job-title search (server-side)."
		},
		"location": {
			"type": "string",
			"description": "Location search matching company city, state, or country (server-side), e.g. 'Spain' or 'Barcelona'."
		},
		"workplace": {
			"type": "string",
			"description": "Exact workplace filter.",
			"enum": ["On-site", "Hybrid", "Remote"]
		},
		"company": {
			"type": "string",
			"description": "Company-name search (server-side)."
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; 20 results per page.",
			"minimum": 1,
			"default": 1
		}
	},
	"additionalProperties": false
}`)

var flowxtraSearchInputSchema = mustSchema(flowxtraSearchInputRawSchema)

type flowxtraSearchInput struct {
	Query     string `json:"query,omitempty"`
	Location  string `json:"location,omitempty"`
	Workplace string `json:"workplace,omitempty"`
	Company   string `json:"company,omitempty"`
	Page      int    `json:"page,omitempty"`
}

type flowxtraSearchOutput struct {
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	LastPage int                  `json:"last_page"`
	Data     []flowxtraJobSummary `json:"data"`
}

type flowxtraJobSummary struct {
	Title     string `json:"title"`
	Company   string `json:"company"`
	Location  string `json:"location,omitempty" jsonschema:"Company city/state/country; may be empty."`
	Workplace string `json:"workplace" jsonschema:"On-site, Hybrid, or Remote."`
	Salary    string `json:"salary,omitempty" jsonschema:"Advertised salary, e.g. 'EUR 21000/year'; empty when the posting lists none."`
	PostedAt  string `json:"posted_at"`
	HasID     string `json:"has_id" jsonschema:"Public hashed job id; pass to flowxtra_get_job_detail's has_id param."`
	URL       string `json:"url" jsonschema:"Public apply URL for the posting."`
}

type flowxtraDetailInput struct {
	HasID string `json:"has_id" jsonschema:"Public hashed job id (has_id from flowxtra_search_jobs results, e.g. M88PB)."`
}

type flowxtraDetailOutput struct {
	Title           string   `json:"title"`
	Company         string   `json:"company"`
	CompanyWebsite  string   `json:"company_website,omitempty"`
	Location        string   `json:"location,omitempty"`
	Workplace       string   `json:"workplace"`
	Seniority       string   `json:"seniority,omitempty"`
	EmploymentTypes []string `json:"employment_types,omitempty"`
	Salary          string   `json:"salary,omitempty"`
	Description     string   `json:"description" jsonschema:"Full job description as plain text."`
	ApplyURL        string   `json:"apply_url" jsonschema:"The company's own career-page apply URL."`
}

// flowxtraLocation joins the non-empty location parts.
func flowxtraLocation(parts ...string) string {
	out := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if out != "" {
			out += ", "
		}
		out += part
	}
	return out
}

// flowxtraSalary renders the salary fields as one line, e.g.
// "EUR 21000/year" or "USD 100000-300000/year"; empty when unset.
func flowxtraSalary(currency string, minSalary, maxSalary, salary flowxtra.NilFloat64, rate string) string {
	amount := ""
	switch {
	case !salary.Null:
		amount = fmt.Sprintf("%g", salary.Value)
	case !minSalary.Null && !maxSalary.Null:
		amount = fmt.Sprintf("%g-%g", minSalary.Value, maxSalary.Value)
	case !minSalary.Null:
		amount = fmt.Sprintf("%g", minSalary.Value)
	case !maxSalary.Null:
		amount = fmt.Sprintf("up to %g", maxSalary.Value)
	default:
		return ""
	}
	out := currency + " " + amount
	if rate != "" {
		out += "/" + rate
	}
	return out
}

func flowxtraMCPToHTTPRequest(in *flowxtraSearchInput) (flowxtra.ListJobsParams, error) {
	params := flowxtra.ListJobsParams{
		PerPage: flowxtra.NewOptInt(flowxtraPageSize),
	}
	if in.Query != "" {
		params.SearchKey = flowxtra.NewOptString(in.Query)
	}
	if in.Location != "" {
		params.Location = flowxtra.NewOptString(in.Location)
	}
	if in.Workplace != "" {
		wp := flowxtra.ListJobsWorkplace(in.Workplace)
		// The schema already enum-guards workplace; this guards direct
		// callers and clients that skip schema validation.
		if err := wp.Validate(); err != nil {
			return params, fmt.Errorf("invalid workplace %q: %w", in.Workplace, err)
		}
		params.Workplace = flowxtra.NewOptListJobsWorkplace(wp)
	}
	if in.Company != "" {
		params.CompanyName = flowxtra.NewOptString(in.Company)
	}
	if in.Page > 0 {
		params.Page = flowxtra.NewOptInt(in.Page)
	}
	return params, nil
}

func flowxtraHTTPToMCPResponse(res *flowxtra.JobListEnvelope) *flowxtraSearchOutput {
	out := &flowxtraSearchOutput{
		Total:    res.Data.Total,
		Page:     res.Data.CurrentPage,
		LastPage: res.Data.LastPage,
		Data:     make([]flowxtraJobSummary, 0, len(res.Data.Data)),
	}
	for _, j := range res.Data.Data {
		out.Data = append(out.Data, flowxtraJobSummary{
			Title:     j.Title,
			Company:   j.NameCompany,
			Location:  flowxtraLocation(j.CityCompany, j.StateCompany, j.CountryCompany),
			Workplace: j.Workplace,
			Salary:    flowxtraSalary(j.Currency, j.MinSalary, j.MaxSalary, j.Salary, j.RateSalary),
			PostedAt:  j.DateShare.Format("2006-01-02"),
			HasID:     j.HasID,
			URL:       j.UrlJobApplay,
		})
	}
	return out
}

func flowxtraHTTPToMCPDetail(job flowxtra.JobDetail) *flowxtraDetailOutput {
	descText, err := html2text.FromString(job.Description, html2text.Options{})
	if err != nil {
		descText = job.Description
	}

	state, country := "", ""
	if office, ok := job.CompanyOffice.Get(); ok {
		if v, ok := office.State.Get(); ok {
			state = v.Name
		}
		if v, ok := office.Country.Get(); ok {
			country = v.Name
		}
	}

	employmentTypes := make([]string, 0, len(job.JobTypeJob))
	for _, et := range job.JobTypeJob {
		employmentTypes = append(employmentTypes, et.Name)
	}

	return &flowxtraDetailOutput{
		Title:           job.Title,
		Company:         job.Company.Name,
		CompanyWebsite:  job.Company.Website.Or(""),
		Location:        flowxtraLocation(state, country),
		Workplace:       job.Workplace,
		Seniority:       job.Seniority.Or(""),
		EmploymentTypes: employmentTypes,
		Salary:          flowxtraSalary(job.Currency, job.MinSalary, job.MaxSalary, job.Salary, job.RateSalary),
		Description:     descText,
		ApplyURL:        job.UrlJobApplay,
	}
}

// RegisterFlowxtra registers the Flowxtra search and job-detail tools.
func RegisterFlowxtra(s *mcp.Server, c *flowxtra.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "flowxtra_search_jobs",
		Description: "Search live jobs across every company hosted on the Flowxtra ATS platform (board-wide, all narrowing server-side).",
		Annotations: &mcp.ToolAnnotations{Title: "Search Flowxtra jobs", ReadOnlyHint: true},
		InputSchema: flowxtraSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *flowxtraSearchInput) (*mcp.CallToolResult, *flowxtraSearchOutput, error) {
		params, err := flowxtraMCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.ListJobs(ctx, params)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, flowxtraHTTPToMCPResponse(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "flowxtra_get_job_detail",
		Description: "Get the full description and company profile for a Flowxtra job by its has_id.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Flowxtra job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *flowxtraDetailInput) (*mcp.CallToolResult, *flowxtraDetailOutput, error) {
		if in.HasID == "" {
			return errorResult(fmt.Errorf("has_id is required (take it from a flowxtra_search_jobs result)")), nil, nil
		}
		res, err := c.GetJobDetail(ctx, flowxtra.GetJobDetailParams{HasId: in.HasID})
		if err != nil {
			return errorResult(err), nil, nil
		}
		envelope, ok := res.(*flowxtra.JobDetailEnvelope)
		if !ok {
			return errorResult(fmt.Errorf("job %q not found (it may have expired)", in.HasID)), nil, nil
		}
		return nil, flowxtraHTTPToMCPDetail(envelope.Data), nil
	})
}
