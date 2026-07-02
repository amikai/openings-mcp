package jobmcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/google/jsonschema-go/jsonschema"
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
			"description": "Result order.",
			"enum": ["Relevance", "Newest"]
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
		"page": {
			"type": "integer",
			"description": "1-based page number.",
			"minimum": 1
		}
	},
	"required": ["keyword", "area"],
	"additionalProperties": false
}`)

type job104SearchInput struct {
	Keyword string   `json:"keyword"` // required
	Area    string   `json:"area"`    // required
	JobType string   `json:"job_type,omitempty"`
	Sort    string   `json:"sort,omitempty"`
	Remote  string   `json:"remote,omitempty"`
	Edu     []string `json:"edu,omitempty"`
	Page    int      `json:"page,omitempty"`
}

type job104DetailInput struct {
	JobCode string `json:"job_code" jsonschema:"104 job code (jobNo)"`
}

// job104SearchInputSchema is hand-written JSON kept aligned with openapi.yaml's
// searchJobs parameters: friendly property names, human labels instead of 104
// codes (the ids.go maps translate labels back to codes — enum labels here
// must match those map keys). Descriptions carry semantics only, never
// id=label tables.
var job104SearchInputSchema = mustSchema(job104SearchInputRawSchema)

// mustSchema unmarshals a raw JSON schema, panicking on malformed JSON —
// a programmer error, same failure mode as jsonschema.For before it.
func mustSchema(rawSchema []byte) *jsonschema.Schema {
	var s jsonschema.Schema
	if err := json.Unmarshal(rawSchema, &s); err != nil {
		panic(fmt.Sprintf("jobmcp tool schema: %v", err))
	}
	return &s
}

func job104MCPToHTTPRequest(in *job104SearchInput) (*job104.SearchJobsParams, error) {
	var params job104.SearchJobsParams
	// The schema already marks keyword and area required; this guards direct
	// callers and clients that skip schema validation — a missing area fails
	// its enum Validate below (empty label maps to the zero value).
	if in.Keyword == "" {
		return nil, fmt.Errorf("keyword is required")
	}
	params.Keyword = job104.NewOptString(in.Keyword)

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

	if in.Page > 0 {
		params.Page = job104.NewOptInt(in.Page)
	}
	return &params, nil
}

// job104SearchOutput mirrors job104.JobsResponse for the LLM: identical
// fields and JSON names, except the coded jobRo/remoteWorkType become the
// job_type/remote labels used by the search input params. Unknown codes
// leave the label empty and omitempty drops the field.
type job104SearchOutput struct {
	Data     []job104JobSummary   `json:"data"`
	Metadata job104SearchMetadata `json:"metadata"`
}

type job104JobSummary struct {
	JobNo         string               `json:"jobNo"`
	JobName       string               `json:"jobName"`
	CustName      string               `json:"custName"`
	CustNo        string               `json:"custNo"`
	Link          job104JobSummaryLink `json:"link"`
	SalaryHigh    int                  `json:"salaryHigh"`
	SalaryLow     int                  `json:"salaryLow"`
	JobAddrNoDesc string               `json:"jobAddrNoDesc"`
	AppearDate    string               `json:"appearDate" jsonschema:"Posting date, YYYYMMDD."`
	ApplyCnt      int                  `json:"applyCnt"`
	Remote        string               `json:"remote,omitempty" jsonschema:"Remote-work label; matches the remote input values. Absent for on-site."`
	JobType       string               `json:"job_type,omitempty" jsonschema:"Employment-basis label; matches the job_type input values."`
}

type job104JobSummaryLink struct {
	Job  string `json:"job" jsonschema:"Public job posting URL; the trailing path segment is the job code for 104_get_job_detail."`
	Cust string `json:"cust"`
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
			JobNo:         j.JobNo,
			JobName:       j.JobName,
			CustName:      j.CustName,
			CustNo:        j.CustNo,
			Link:          job104JobSummaryLink{Job: j.Link.Job, Cust: j.Link.Cust},
			SalaryHigh:    j.SalaryHigh,
			SalaryLow:     j.SalaryLow,
			JobAddrNoDesc: j.JobAddrNoDesc,
			AppearDate:    j.AppearDate,
			ApplyCnt:      j.ApplyCnt,
			Remote:        job104RemoteWorkLabels[job104.SearchJobsRemoteWork(j.RemoteWorkType)],
			JobType:       job104RoLabels[job104.SearchJobsRo(j.JobRo)],
		})
	}
	return out
}

// job104DetailOutput mirrors job104.JobDetailResponse for the LLM: Opt
// fields flatten to plain values with omitempty, and the coded
// jobType/remoteWork become the job_type/remote labels used by the search
// input params. Unknown codes and null remoteWork drop the label.
type job104DetailOutput struct {
	Data job104JobDetail `json:"data"`
}

type job104JobDetail struct {
	Header    job104DetailHeader    `json:"header"`
	Contact   job104DetailContact   `json:"contact"`
	Condition job104DetailCondition `json:"condition"`
	Welfare   job104DetailWelfare   `json:"welfare"`
	JobDetail job104DetailJobDetail `json:"jobDetail"`
	Industry  string                `json:"industry"`
	Employees string                `json:"employees"`
	CustNo    string                `json:"custNo"`
}

type job104DetailHeader struct {
	JobName    string `json:"jobName"`
	CustName   string `json:"custName"`
	CustUrl    string `json:"custUrl"`
	AppearDate string `json:"appearDate"`
	IsSaved    bool   `json:"isSaved"`
	IsApplied  bool   `json:"isApplied"`
}

type job104DetailContact struct {
	HrName string `json:"hrName,omitempty"`
	Email  string `json:"email,omitempty"`
	Reply  string `json:"reply,omitempty"`
}

type job104DetailCondition struct {
	WorkExp   string                  `json:"workExp,omitempty"`
	Edu       string                  `json:"edu,omitempty"`
	Major     []string                `json:"major,omitempty"`
	Specialty []job104CodeDescription `json:"specialty,omitempty"`
}

type job104DetailWelfare struct {
	Welfare string `json:"welfare,omitempty"`
}

type job104DetailJobDetail struct {
	JobDescription string                  `json:"jobDescription,omitempty"`
	JobCategory    []job104CodeDescription `json:"jobCategory,omitempty"`
	Salary         string                  `json:"salary,omitempty"`
	SalaryMin      int                     `json:"salaryMin,omitempty"`
	SalaryMax      int                     `json:"salaryMax,omitempty"`
	JobType        string                  `json:"job_type,omitempty" jsonschema:"Employment-basis label; matches the job_type input values."`
	AddressRegion  string                  `json:"addressRegion,omitempty"`
	AddressDetail  string                  `json:"addressDetail,omitempty"`
	ManageResp     string                  `json:"manageResp,omitempty"`
	NeedEmp        string                  `json:"needEmp,omitempty"`
	Remote         string                  `json:"remote,omitempty" jsonschema:"Remote-work label; matches the remote input values. Absent for on-site."`
}

type job104CodeDescription struct {
	Code        string `json:"code,omitempty"`
	Description string `json:"description,omitempty"`
}

func job104HTTPToMCPDetail(resp *job104.JobDetailResponse) *job104DetailOutput {
	d := resp.Data
	out := &job104DetailOutput{
		Data: job104JobDetail{
			Header: job104DetailHeader{
				JobName:    d.Header.JobName,
				CustName:   d.Header.CustName,
				CustUrl:    d.Header.CustUrl,
				AppearDate: d.Header.AppearDate,
				IsSaved:    d.Header.IsSaved,
				IsApplied:  d.Header.IsApplied,
			},
			Contact: job104DetailContact{
				HrName: d.Contact.HrName.Or(""),
				Email:  d.Contact.Email.Or(""),
				Reply:  d.Contact.Reply.Or(""),
			},
			Condition: job104DetailCondition{
				WorkExp:   d.Condition.WorkExp.Or(""),
				Edu:       d.Condition.Edu.Or(""),
				Major:     d.Condition.Major,
				Specialty: job104CodeDescriptions(d.Condition.Specialty),
			},
			Welfare: job104DetailWelfare{Welfare: d.Welfare.Welfare.Or("")},
			JobDetail: job104DetailJobDetail{
				JobDescription: d.JobDetail.JobDescription.Or(""),
				JobCategory:    job104CodeDescriptions(d.JobDetail.JobCategory),
				Salary:         d.JobDetail.Salary.Or(""),
				SalaryMin:      d.JobDetail.SalaryMin.Or(0),
				SalaryMax:      d.JobDetail.SalaryMax.Or(0),
				JobType:        job104RoLabels[job104.SearchJobsRo(d.JobDetail.JobType.Or(0))],
				AddressRegion:  d.JobDetail.AddressRegion.Or(""),
				AddressDetail:  d.JobDetail.AddressDetail.Or(""),
				ManageResp:     d.JobDetail.ManageResp.Or(""),
				NeedEmp:        d.JobDetail.NeedEmp.Or(""),
			},
			Industry:  d.Industry,
			Employees: d.Employees,
			CustNo:    d.CustNo,
		},
	}
	if rw, ok := d.JobDetail.RemoteWork.Get(); ok {
		out.Data.JobDetail.Remote = job104RemoteWorkLabels[job104.SearchJobsRemoteWork(rw.Type.Or(0))]
	}
	return out
}

func job104CodeDescriptions(in []job104.CodeDescription) []job104CodeDescription {
	if len(in) == 0 {
		return nil
	}
	out := make([]job104CodeDescription, 0, len(in))
	for _, cd := range in {
		out = append(out, job104CodeDescription{Code: cd.Code.Or(""), Description: cd.Description.Or("")})
	}
	return out
}

// RegisterJob104 registers the 104 search and job-detail tools.
func RegisterJob104(s *mcp.Server, c *job104.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_search_jobs",
		Description: "Search jobs on 104 (Taiwan's largest job board) by keyword and area, with optional job-type/remote/education/sort filters.",
		InputSchema: job104SearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *job104SearchInput) (*mcp.CallToolResult, *job104SearchOutput, error) {
		params, err := job104MCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, err := c.SearchJobs(ctx, *params)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, job104HTTPToMCPResponse(resp), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_get_job_detail",
		Description: "Get the full job description for a 104 job code (jobNo from search results).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *job104DetailInput) (*mcp.CallToolResult, *job104DetailOutput, error) {
		resp, err := c.GetJobDetail(ctx, job104.GetJobDetailParams{JobCode: in.JobCode})
		if err != nil {
			return errorResult(err), nil, nil
		}
		detail, ok := resp.(*job104.JobDetailResponse)
		if !ok {
			return errorResult(fmt.Errorf("job detail %s returned %T", in.JobCode, resp)), nil, nil
		}
		return nil, job104HTTPToMCPDetail(detail), nil
	})
}
