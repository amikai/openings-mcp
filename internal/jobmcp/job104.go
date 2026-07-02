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
			"description": "Employment basis. Soft filter — verify each result's jobRo.",
			"enum": ["Full-time", "Part-time", "Senior", "Dispatch"]
		},
		"sort": {
			"type": "string",
			"description": "Result order.",
			"enum": ["Relevance", "Newest"]
		},
		"remote": {
			"type": "string",
			"description": "Remote work. Soft filter — verify each result's remoteWorkType. Omit for on-site.",
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
	JobCode string `json:"job_code" jsonschema:"104 job code (jobNo), required"`
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
		panic(fmt.Sprintf("job104 search schema: %v", err))
	}
	return &s
}

// lookupCode translates one human label to its typed code, erroring with the
// field name on unknown labels.
func lookupCode[T any](field, label string, m map[string]T) (T, error) {
	code, ok := m[label]
	if !ok {
		var zero T
		return zero, fmt.Errorf("invalid %s %q", field, label)
	}
	return code, nil
}

// lookupCodes is lookupCode over a multi-select field.
func lookupCodes[T any](field string, labels []string, m map[string]T) ([]T, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	out := make([]T, 0, len(labels))
	for _, label := range labels {
		code, err := lookupCode(field, label, m)
		if err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, nil
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

// RegisterJob104 registers the 104 search and job-detail tools.
func RegisterJob104(s *mcp.Server, c *job104.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_search_jobs",
		Description: "Search jobs on 104 (Taiwan's largest job board) by keyword and area, with optional job-type/remote/education/sort filters.",
		InputSchema: job104SearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in job104SearchInput) (*mcp.CallToolResult, any, error) {
		params, err := job104MCPToHTTPRequest(&in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, err := c.SearchJobs(ctx, *params)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, resp, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_get_job_detail",
		Description: "Get the full job description for a 104 job code (jobNo from search results).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in job104DetailInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.GetJobDetail(ctx, job104.GetJobDetailParams{JobCode: in.JobCode})
		if err != nil {
			return errorResult(err), nil, nil
		}
		detail, ok := resp.(*job104.JobDetailResponse)
		if !ok {
			return errorResult(fmt.Errorf("job detail %s returned %T", in.JobCode, resp)), nil, nil
		}
		return nil, detail, nil
	})
}
