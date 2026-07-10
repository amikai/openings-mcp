package openingsmcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/amikai/openings-mcp/internal/provider/job104"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var job104SearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text keyword search."
		},
		"area": {
			"type": "string",
			"description": "City/region filter.",
			"enum": [
				"Taipei", "NewTaipei", "Yilan", "Keelung", "Taoyuan",
				"Hsinchu", "Miaoli", "Taichung", "Changhua", "Nantou",
				"Yunlin", "Chiayi", "Tainan", "Kaohsiung", "Pingtung",
				"Taitung", "Hualien", "Penghu", "Kinmen", "Lienchiang",
				"Beijing", "Tianjin", "Shanghai", "Chongqing", "Guangdong",
				"Fujian", "Hainan", "Zhejiang", "Jiangsu", "Shandong",
				"Hebei", "Liaoning", "Jilin", "Heilongjiang", "Hunan",
				"Hubei", "Jiangxi", "Anhui", "Henan", "Shanxi",
				"Shaanxi", "Gansu", "Qinghai", "Sichuan", "Guizhou",
				"Yunnan", "InnerMongolia", "Tibet", "Ningxia", "Xinjiang",
				"Guangxi", "HongKong", "Macao",
				"NortheastAsia", "SoutheastAsia", "OtherAsia",
				"AustraliaNZ", "OtherOceania",
				"Canada", "EasternUS", "WesternUS", "MidwesternUS",
				"CentralAmerica", "SouthAmerica",
				"NorthernEurope", "SouthernEurope", "EasternEurope",
				"WesternEurope", "CentralEurope",
				"NorthAfrica", "CentralAfrica", "SouthAfrica",
				"EastAfrica", "WestAfrica"
			]
		},
		"job_type": {
			"type": "string",
			"description": "Employment basis. Soft filter — verify each result's job_type.",
			"enum": ["Full-time", "Part-time", "Senior", "Dispatch"]
		},
		"sort": {
			"type": "string",
			"description": "Result order. SalaryHigh also excludes postings without a disclosed salary (待遇面議), not just sorting.",
			"enum": ["Relevance", "Newest", "SalaryHigh"]
		},
		"remote": {
			"type": "string",
			"description": "Remote work. Soft filter — verify each result's remote. Omit for on-site.",
			"enum": ["Full", "Partial"]
		},
		"edu": {
			"type": "array",
			"description": "Education levels, OR'd together.",
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["HighSchoolBelow", "HighSchool", "College", "University", "Master", "Doctorate"]
			}
		},
		"experience": {
			"type": "array",
			"description": "Minimum-experience brackets, OR'd together. Soft filter — verify each result's experience.",
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["Under1Year", "1To3Years", "3To5Years", "5To10Years", "Over10Years"]
			}
		},
		"page": {
			"type": "integer",
			"description": "1-based page number.",
			"minimum": 1
		}
	},
	"required": ["keyword", "area"],
	"additionalProperties": false
}`)

// job104SearchInputSchema is hand-written JSON kept aligned with openapi.yaml's
// searchJobs parameters: friendly property names, human labels instead of 104
// codes (the ids.go maps translate labels back to codes — enum labels here
// must match those map keys). Descriptions carry semantics only, never
// id=label tables.
var job104SearchInputSchema = mustSchema(job104SearchInputRawSchema)

type job104SearchInput struct {
	Keyword    string   `json:"keyword"` // required
	Area       string   `json:"area"`    // required
	JobType    string   `json:"job_type,omitempty"`
	Sort       string   `json:"sort,omitempty"`
	Remote     string   `json:"remote,omitempty"`
	Edu        []string `json:"edu,omitempty"`
	Experience []string `json:"experience,omitempty"`
	Page       int      `json:"page,omitempty"`
}

// job104SearchOutput reshapes job104.JobsResponse for the LLM, making it
// easy to read.
type job104SearchOutput struct {
	Data     []job104JobSummary   `json:"data"`
	Metadata job104SearchMetadata `json:"metadata"`
}

type job104DetailInput struct {
	JobCode string `json:"job_code" jsonschema:"104 job code, from a search result's jobCode field (not jobNo)."`
}

// job104DetailOutput reshapes job104.JobDetailResponse for the LLM, making
// it easy to read.
type job104DetailOutput struct {
	JobName        string   `json:"jobName"`                                                                                               // header.jobName
	CompanyName    string   `json:"companyName"`                                                                                           // header.custName
	URL            string   `json:"url" jsonschema:"Public job posting URL."`                                                              // built from the job_code input; the 104 response has no posting URL
	CompanyURL     string   `json:"companyUrl" jsonschema:"Company profile page URL."`                                                     // header.custUrl
	AppearDate     string   `json:"appearDate"`                                                                                            // header.appearDate
	JobDescription string   `json:"jobDescription,omitempty"`                                                                              // jobDetail.jobDescription
	JobCategory    []string `json:"jobCategory,omitempty"`                                                                                 // jobDetail.jobCategory[].description
	Salary         string   `json:"salary,omitempty"`                                                                                      // jobDetail.salary
	SalaryMin      int      `json:"salaryMin,omitempty"`                                                                                   // jobDetail.salaryMin
	SalaryMax      int      `json:"salaryMax,omitempty"`                                                                                   // jobDetail.salaryMax
	JobType        string   `json:"job_type,omitempty" jsonschema:"Employment-basis label; matches the job_type input values."`            // jobDetail.jobType code → label; unknown code drops the field
	Remote         string   `json:"remote,omitempty" jsonschema:"Remote-work label; matches the remote input values. Absent for on-site."` // jobDetail.remoteWork.type code → label; null/unknown drops the field
	AddressRegion  string   `json:"addressRegion,omitempty"`                                                                               // jobDetail.addressRegion
	AddressDetail  string   `json:"addressDetail,omitempty"`                                                                               // jobDetail.addressDetail
	WorkExp        string   `json:"workExp,omitempty"`                                                                                     // condition.workExp
	Edu            string   `json:"edu,omitempty"`                                                                                         // condition.edu
	Major          []string `json:"major,omitempty"`                                                                                       // condition.major
	Specialty      []string `json:"specialty,omitempty"`                                                                                   // condition.specialty[].description
	ManageResp     string   `json:"manageResp,omitempty"`                                                                                  // jobDetail.manageResp
	NeedEmp        string   `json:"needEmp,omitempty"`                                                                                     // jobDetail.needEmp
	Welfare        string   `json:"welfare,omitempty"`                                                                                     // welfare.welfare
	Industry       string   `json:"industry,omitempty"`
	Employees      string   `json:"employees,omitempty"`
}

func job104MCPToHTTPRequest(in *job104SearchInput) (*job104.SearchJobsParams, error) {
	var params job104.SearchJobsParams
	// The schema already marks keyword and area required; this guards direct
	// callers and clients that skip schema validation — a missing area fails
	// its enum Validate below (empty label maps to the zero value).
	if in.Keyword == "" {
		return nil, errors.New("keyword is required")
	}
	params.Keyword = job104.NewOptString(in.Keyword)
	// Always on: without it, a keyword 104 recognizes as a company name (e.g.
	// 聯發科) gets a pagination-less companyKeyword response instead of job
	// results, which fails JobsResponse decoding. See the parameter's
	// description in openapi.yaml.
	params.ExcludeCompanyKeyword = job104.NewOptBool(true)

	area := job104.AreaIDs[in.Area]
	if err := area.Validate(); err != nil {
		return nil, fmt.Errorf("invalid area %q: %w", in.Area, err)
	}
	params.Area = job104.NewOptSearchJobsArea(area)

	if in.JobType != "" {
		jobType := job104.RoIDs[in.JobType]
		if err := jobType.Validate(); err != nil {
			return nil, fmt.Errorf("invalid job_type %q: %w", in.JobType, err)
		}
		params.Ro = job104.NewOptSearchJobsRo(jobType)
	}

	if in.Sort != "" {
		sort := job104.OrderIDs[in.Sort]
		if err := sort.Validate(); err != nil {
			return nil, fmt.Errorf("invalid sort %q: %w", in.Sort, err)
		}
		params.Order = job104.NewOptSearchJobsOrder(sort)
	}

	if in.Remote != "" {
		remote := job104.RemoteWorkIDs[in.Remote]
		if err := remote.Validate(); err != nil {
			return nil, fmt.Errorf("invalid remote %q: %w", in.Remote, err)
		}
		params.RemoteWork = job104.NewOptSearchJobsRemoteWork(remote)
	}

	for _, label := range in.Edu {
		edu := job104.EduIDs[label]
		if err := edu.Validate(); err != nil {
			return nil, fmt.Errorf("invalid edu %q: %w", label, err)
		}
		params.Edu = append(params.Edu, edu)
	}

	for _, label := range in.Experience {
		exp := job104.JobExpIDs[label]
		if err := exp.Validate(); err != nil {
			return nil, fmt.Errorf("invalid experience %q: %w", label, err)
		}
		params.Jobexp = append(params.Jobexp, exp)
	}

	if in.Page > 0 {
		params.Page = job104.NewOptInt(in.Page)
	}
	return &params, nil
}

type job104JobSummary struct {
	JobCode       string `json:"jobCode" jsonschema:"104 job code — pass this to 104_get_job_detail's job_code param."` // link.job's trailing path segment
	JobName       string `json:"jobName"`
	CompanyName   string `json:"companyName"`                                       // custName
	URL           string `json:"url" jsonschema:"Public job posting URL."`          // link.job
	CompanyURL    string `json:"companyUrl" jsonschema:"Company profile page URL."` // link.cust
	SalaryHigh    int    `json:"salaryHigh"`
	SalaryLow     int    `json:"salaryLow"`
	JobAddrNoDesc string `json:"jobAddrNoDesc"`
	AppearDate    string `json:"appearDate" jsonschema:"Posting date, YYYYMMDD."`
	ApplyCnt      int    `json:"applyCnt"`
	Remote        string `json:"remote,omitempty" jsonschema:"Remote-work label; matches the remote input values. Absent for on-site."`                                   // remoteWorkType code → label; unknown code drops the field
	JobType       string `json:"job_type,omitempty" jsonschema:"Employment-basis label; matches the job_type input values."`                                              // jobRo code → label; unknown code drops the field
	Experience    string `json:"experience" jsonschema:"Minimum-experience bracket label; matches the experience input values. Soft filter — verify against this field."` // period bucketed into the same brackets as the experience input
}

type job104SearchMetadata struct {
	Pagination job104Pagination `json:"pagination"`
}

type job104Pagination struct {
	CurrentPage int `json:"currentPage"`
	LastPage    int `json:"lastPage"`
	Total       int `json:"total"`
}

// job104RoLabels and job104RemoteWorkLabels invert the ids.go request maps
// for response conversion, keeping ids.go the single source of truth.
var job104RoLabels = func() map[job104.SearchJobsRo]string {
	m := make(map[job104.SearchJobsRo]string, len(job104.RoIDs))
	for label, code := range job104.RoIDs {
		m[code] = label
	}
	return m
}()

var job104RemoteWorkLabels = func() map[job104.SearchJobsRemoteWork]string {
	m := make(map[job104.SearchJobsRemoteWork]string, len(job104.RemoteWorkIDs))
	for label, code := range job104.RemoteWorkIDs {
		m[code] = label
	}
	return m
}()

// job104ExperienceLabel buckets a JobSummary's raw period value into the
// same labels as the experience input, mirroring the jobexp/period mapping
// documented on JobSummary.period in openapi.yaml (jobexp 1 → period 0-1,
// 3 → 2-3, 5 → 4-5, 10 → 6-10, 99 → 11+).
func job104ExperienceLabel(period int) string {
	switch {
	case period <= 1:
		return "Under1Year"
	case period <= 3:
		return "1To3Years"
	case period <= 5:
		return "3To5Years"
	case period <= 10:
		return "5To10Years"
	default:
		return "Over10Years"
	}
}

func job104HTTPToMCPResponse(resp *job104.JobsResponse) *job104SearchOutput {
	out := &job104SearchOutput{
		Data: make([]job104JobSummary, 0, len(resp.Data)),
		Metadata: job104SearchMetadata{
			Pagination: job104Pagination{
				CurrentPage: resp.Metadata.Pagination.CurrentPage,
				LastPage:    resp.Metadata.Pagination.LastPage,
				Total:       resp.Metadata.Pagination.Total,
			},
		},
	}
	for _, j := range resp.Data {
		out.Data = append(out.Data, job104JobSummary{
			JobCode:       job104.JobCodeFromURL(j.Link.Job),
			JobName:       j.JobName,
			CompanyName:   j.CustName,
			URL:           j.Link.Job,
			CompanyURL:    j.Link.Cust,
			SalaryHigh:    j.SalaryHigh,
			SalaryLow:     j.SalaryLow,
			JobAddrNoDesc: j.JobAddrNoDesc,
			AppearDate:    j.AppearDate,
			ApplyCnt:      j.ApplyCnt,
			Remote:        job104RemoteWorkLabels[job104.SearchJobsRemoteWork(j.RemoteWorkType)],
			JobType:       job104RoLabels[job104.SearchJobsRo(j.JobRo)],
			Experience:    job104ExperienceLabel(j.Period),
		})
	}
	return out
}

func job104HTTPToMCPDetail(resp *job104.JobDetailResponse, jobCode string) *job104DetailOutput {
	d := resp.Data
	out := &job104DetailOutput{
		JobName:        d.Header.JobName,
		CompanyName:    d.Header.CustName,
		URL:            "https://www.104.com.tw/job/" + jobCode,
		CompanyURL:     d.Header.CustUrl,
		AppearDate:     d.Header.AppearDate,
		JobDescription: d.JobDetail.JobDescription.Or(""),
		JobCategory:    job104Descriptions(d.JobDetail.JobCategory),
		Salary:         d.JobDetail.Salary.Or(""),
		SalaryMin:      d.JobDetail.SalaryMin.Or(0),
		SalaryMax:      d.JobDetail.SalaryMax.Or(0),
		JobType:        job104RoLabels[job104.SearchJobsRo(d.JobDetail.JobType.Or(0))],
		AddressRegion:  d.JobDetail.AddressRegion.Or(""),
		AddressDetail:  d.JobDetail.AddressDetail.Or(""),
		WorkExp:        d.Condition.WorkExp.Or(""),
		Edu:            d.Condition.Edu.Or(""),
		Major:          d.Condition.Major,
		Specialty:      job104Descriptions(d.Condition.Specialty),
		ManageResp:     d.JobDetail.ManageResp.Or(""),
		NeedEmp:        d.JobDetail.NeedEmp.Or(""),
		Welfare:        d.Welfare.Welfare.Or(""),
		Industry:       d.Industry,
		Employees:      d.Employees,
	}
	if rw, ok := d.JobDetail.RemoteWork.Get(); ok {
		out.Remote = job104RemoteWorkLabels[job104.SearchJobsRemoteWork(rw.Type.Or(0))]
	}
	return out
}

func job104Descriptions(in []job104.CodeDescription) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, cd := range in {
		out = append(out, cd.Description.Or(""))
	}
	return out
}

// RegisterJob104 registers the 104 search and job-detail tools.
func RegisterJob104(s *mcp.Server, c *job104.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_search_jobs",
		Description: "Search jobs on 104 (Taiwan's largest job board) by keyword and area, with optional job-type/remote/education/experience/sort filters.",
		Annotations: &mcp.ToolAnnotations{Title: "Search 104 jobs", ReadOnlyHint: true},
		InputSchema: job104SearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *job104SearchInput) (*mcp.CallToolResult, *job104SearchOutput, error) {
		params, err := job104MCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, err := c.SearchJobs(ctx, *params)
		if err != nil {
			if ue, ok := errors.AsType[*job104.ErrorResponseStatusCode](err); ok {
				return errorResult(fmt.Errorf("upstream error: %d", ue.StatusCode)), nil, nil
			}
			return errorResult(err), nil, nil
		}
		return nil, job104HTTPToMCPResponse(resp), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_get_job_detail",
		Description: "Get the full job description for a 104 job code (jobCode from 104_search_jobs results, not jobNo).",
		Annotations: &mcp.ToolAnnotations{Title: "Get 104 job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *job104DetailInput) (*mcp.CallToolResult, *job104DetailOutput, error) {
		resp, err := c.GetJobDetail(ctx, job104.GetJobDetailParams{JobCode: in.JobCode})
		if err != nil {
			if ue, ok := errors.AsType[*job104.ErrorResponseStatusCode](err); ok {
				return errorResult(fmt.Errorf("upstream error: %d", ue.StatusCode)), nil, nil
			}
			return errorResult(err), nil, nil
		}
		return nil, job104HTTPToMCPDetail(resp, in.JobCode), nil
	})
}
