package openingsmcp

import (
	"context"

	"github.com/amikai/openings-mcp/internal/provider/mynavi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var mynaviSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text query: role titles, skills, technologies, or place names. Space-separated terms AND together. The site is Japanese; Japanese terms match best (e.g. 機械学習), though Latin tech terms like Python work too. A literal '/' cannot be searched.",
			"minLength": 1
		},
		"min_salary": {
			"type": "integer",
			"description": "Minimum first-year annual salary (初年度年収) in 万円 (10,000 JPY); e.g. 700 = ¥7M/year. Only these fixed steps are accepted.",
			"enum": [150, 200, 250, 300, 350, 400, 450, 500, 550, 600, 650, 700, 800, 900, 1000, 1100, 1200, 1300, 1400, 1500]
		},
		"page": {
			"type": "integer",
			"description": "1-based page. Each full page carries 50 results; total in the response counts every match.",
			"minimum": 1
		}
	},
	"additionalProperties": false
}`)

// mynaviSearchInputSchema is hand-written JSON kept aligned with the
// mynavi package's path-token DSL (kw/min/pg); min_salary's enum mirrors
// mynavi.MinSalaries.
var mynaviSearchInputSchema = mustSchema(mynaviSearchInputRawSchema)

type mynaviSearchInput struct {
	Keyword   string `json:"keyword,omitempty"`
	MinSalary int    `json:"min_salary,omitempty"`
	Page      int    `json:"page,omitempty"`
}

func mynaviMCPToHTTPRequest(in *mynaviSearchInput) *mynavi.JobsRequest {
	return &mynavi.JobsRequest{
		Keywords:  in.Keyword,
		MinSalary: in.MinSalary,
		Page:      in.Page,
	}
}

type mynaviSearchOutput struct {
	Total int                `json:"total" jsonschema:"Total matches across all pages, 50 per page."`
	Data  []mynaviJobSummary `json:"data"`
}

type mynaviJobSummary struct {
	ID               string   `json:"id" jsonschema:"Mynavi job ID (four hyphen-separated numbers); pass to mynavi_get_job_detail's job_id param."`
	Title            string   `json:"title"`
	Company          string   `json:"company,omitempty"`
	CatchCopy        string   `json:"catch_copy,omitempty" jsonschema:"The employer's own tagline shown beside its name."`
	EmploymentStatus string   `json:"employment_status,omitempty" jsonschema:"One of the site's employment types, e.g. 正社員 (permanent), 契約社員 (contract), 業務委託 (freelance contract), 一般派遣 (temp staffing)."`
	Conditions       []string `json:"conditions,omitempty" jsonschema:"Tags from a small fixed set: 転勤なし (no relocation), リモートワーク可 (remote OK), 完全週休2日制 (5-day workweek), 学歴不問 (no education requirement), 第二新卒歓迎 (early-career switchers welcome), 職種・業種未経験OK (no experience needed), 上場 (listed company)."`
	Description      string   `json:"description,omitempty" jsonschema:"Job-description (仕事内容) summary, truncated by the site; fetch the detail for the full text."`
	Target           string   `json:"target,omitempty" jsonschema:"Target-applicant (対象となる方) summary, truncated by the site."`
	Location         string   `json:"location,omitempty"`
	Salary           string   `json:"salary,omitempty"`
	FirstYearIncome  string   `json:"first_year_income,omitempty" jsonschema:"First-year income (初年度年収) range, e.g. 420万円～1000万円."`
	UpdatedDate      string   `json:"updated_date,omitempty" jsonschema:"YYYY/MM/DD."`
	EndDate          string   `json:"end_date,omitempty" jsonschema:"Listing end date (掲載終了予定日), YYYY/MM/DD; the posting 404s after this."`
	URL              string   `json:"url,omitempty" jsonschema:"Public job posting URL."`
}

func mynaviJobURL(id string) string {
	if id == "" {
		return ""
	}
	return "https://tenshoku.mynavi.jp/jobinfo-" + id + "/"
}

func mynaviHTTPToMCPResponse(resp *mynavi.JobsResponse) *mynaviSearchOutput {
	out := &mynaviSearchOutput{Total: resp.Total, Data: make([]mynaviJobSummary, 0, len(resp.Jobs))}
	for _, j := range resp.Jobs {
		out.Data = append(out.Data, mynaviJobSummary{
			ID:               j.ID,
			Title:            j.Title,
			Company:          j.Company,
			CatchCopy:        j.CatchCopy,
			EmploymentStatus: j.EmploymentStatus,
			Conditions:       j.Conditions,
			Description:      j.Description,
			Target:           j.Target,
			Location:         j.Location,
			Salary:           j.Salary,
			FirstYearIncome:  j.FirstYearIncome,
			UpdatedDate:      j.UpdatedDate,
			EndDate:          j.EndDate,
			URL:              mynaviJobURL(j.ID),
		})
	}
	return out
}

type mynaviDetailInput struct {
	JobID string `json:"job_id" jsonschema:"Mynavi job ID (id from mynavi_search_jobs results, e.g. 348855-1-29-1)."`
}

type mynaviLocation struct {
	Region   string `json:"region"`             // prefecture, e.g. 東京都
	Locality string `json:"locality,omitempty"` // city or ward, e.g. 渋谷区
}

type mynaviDetailOutput struct {
	ID                     string           `json:"id"`
	URL                    string           `json:"url,omitempty" jsonschema:"Public job posting URL."`
	Title                  string           `json:"title"`
	Company                string           `json:"company,omitempty"`
	CompanyURL             string           `json:"company_url,omitempty" jsonschema:"The employer's own website."`
	EmploymentType         string           `json:"employment_type,omitempty" jsonschema:"schema.org enum: FULL_TIME (正社員), CONTRACTOR (契約社員), or OTHER (dispatch/freelance types); coarser than the search summary's employment_status."`
	Industry               string           `json:"industry,omitempty"`
	Occupation             string           `json:"occupation,omitempty"`
	DatePosted             string           `json:"date_posted,omitempty" jsonschema:"YYYY-MM-DD."`
	ValidThrough           string           `json:"valid_through,omitempty" jsonschema:"YYYY-MM-DD; the posting 404s after this."`
	Locations              []mynaviLocation `json:"locations,omitempty" jsonschema:"Work locations by prefecture. A nationwide-remote posting lists all 47 prefectures."`
	SalaryCurrency         string           `json:"salary_currency,omitempty"`
	SalaryMin              string           `json:"salary_min,omitempty" jsonschema:"e.g. 4200000 (JPY when salary_currency is JPY)."`
	SalaryMax              string           `json:"salary_max,omitempty"`
	SalaryUnit             string           `json:"salary_unit,omitempty" jsonschema:"e.g. YEAR."`
	Description            string           `json:"description,omitempty" jsonschema:"Full job description as plain text (Japanese)."`
	ExperienceRequirements string           `json:"experience_requirements,omitempty" jsonschema:"Application requirements (応募条件) as plain text."`
	WorkHours              string           `json:"work_hours,omitempty"`
	JobBenefits            string           `json:"job_benefits,omitempty"`
}

func mynaviHTTPToMCPDetail(detail *mynavi.JobDetailResponse) *mynaviDetailOutput {
	out := &mynaviDetailOutput{
		ID:                     detail.ID,
		URL:                    detail.URL,
		Title:                  detail.Title,
		Company:                detail.Company,
		CompanyURL:             detail.CompanyURL,
		EmploymentType:         detail.EmploymentType,
		Industry:               detail.Industry,
		Occupation:             detail.OccupationalCategory,
		DatePosted:             detail.DatePosted,
		ValidThrough:           detail.ValidThrough,
		SalaryCurrency:         detail.SalaryCurrency,
		SalaryMin:              detail.SalaryMin,
		SalaryMax:              detail.SalaryMax,
		SalaryUnit:             detail.SalaryUnit,
		Description:            detail.Description,
		ExperienceRequirements: detail.ExperienceRequirements,
		WorkHours:              detail.WorkHours,
		JobBenefits:            detail.JobBenefits,
	}
	if out.URL == "" {
		out.URL = mynaviJobURL(detail.ID)
	}
	for _, loc := range detail.Locations {
		out.Locations = append(out.Locations, mynaviLocation{Region: loc.Region, Locality: loc.Locality})
	}
	return out
}

// RegisterMynavi registers the Mynavi Tenshoku search and job-detail tools.
func RegisterMynavi(s *mcp.Server, c *mynavi.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "mynavi_search_jobs",
		Description: "Search jobs on マイナビ転職 (Mynavi Tenshoku), a major Japanese job board for mid-career hires. Listings are in Japanese.",
		Annotations: &mcp.ToolAnnotations{Title: "Search Mynavi Tenshoku jobs", ReadOnlyHint: true},
		InputSchema: mynaviSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *mynaviSearchInput) (*mcp.CallToolResult, *mynaviSearchOutput, error) {
		res, err := c.Jobs(ctx, mynaviMCPToHTTPRequest(in))
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, mynaviHTTPToMCPResponse(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "mynavi_get_job_detail",
		Description: "Get the full posting by Mynavi job ID (id from mynavi_search_jobs results): complete description, salary range, prefectures, requirements, work hours, and benefits.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Mynavi Tenshoku job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *mynaviDetailInput) (*mcp.CallToolResult, *mynaviDetailOutput, error) {
		res, err := c.JobDetail(ctx, in.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, mynaviHTTPToMCPDetail(res), nil
	})
}
