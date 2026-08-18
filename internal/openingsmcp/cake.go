package openingsmcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jaytaylor/html2text"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/amikai/openings-mcp/internal/provider/cake"
)

const (
	cakeSearchToolName  = "cake_search_jobs"
	cakeDetailToolName  = "cake_get_job_detail"
	cakeFiltersToolName = "cake_get_search_filters"
)

// cakeSearchInputRawSchema is hand-written JSON kept aligned with
// openapi.yaml's JobSearchRequest/JobSearchFilters: a flat property list
// instead of the query/sort_by/filters nesting. Enums are relaxed to open strings
// and normalized in cakeMCPToHTTPRequest to support both human labels and API slugs.
var cakeSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text keyword search.",
			"minLength": 1
		},
		"location": {
			"type": "string",
			"description": "Location name as shown on Cake.me, localized English or Chinese, e.g. \"Taiwan\", \"台灣\", \"Taipei City, Taiwan\".",
			"minLength": 1
		},
		"job_type": {
			"type": "string",
			"description": "Employment type value from cake_get_search_filters (e.g. 'full_time', 'part_time', 'internship', 'contract', 'freelance')."
		},
		"seniority": {
			"type": "array",
			"description": "Seniority level values from cake_get_search_filters, OR'd together (e.g. 'mid_senior_level', 'entry_level', 'director').",
			"minItems": 1,
			"uniqueItems": true,
			"items": {
				"type": "string"
			}
		},
		"remote": {
			"type": "string",
			"description": "Remote-work policy value from cake_get_search_filters (e.g. 'full_remote_work', 'partial_remote_work', 'no_remote_work'). Omit to include all."
		},
		"sort": {
			"type": "string",
			"description": "Result order. Defaults to popularity ('popularity' / '熱門', 'latest' / '最新').",
			"default": "popularity"
		},
		"page": {
			"type": "integer",
			"description": "1-based page number.",
			"minimum": 1,
			"default": 1
		}
	},
	"required": ["keyword", "location"],
	"additionalProperties": false
}`)

var cakeSearchInputSchema = mustSchema(cakeSearchInputRawSchema)

type cakeSearchInput struct {
	Keyword   string   `json:"keyword"`  // required
	Location  string   `json:"location"` // required
	JobType   string   `json:"job_type,omitempty"`
	Seniority []string `json:"seniority,omitempty"`
	Remote    string   `json:"remote,omitempty"`
	Sort      string   `json:"sort,omitempty"`
	Page      int      `json:"page,omitempty"`
}

// cakeSearchOutput mirrors cake.JobSearchResponse for the LLM: identical
// fields and JSON names.
type cakeSearchOutput struct {
	TotalEntries int              `json:"total_entries"`
	TotalPages   int              `json:"total_pages"`
	PerPage      int              `json:"per_page"`
	CurrentPage  int              `json:"current_page"`
	Data         []cakeJobSummary `json:"data"`
}

type cakeDetailInput struct {
	Path string `json:"path" jsonschema:"Cake.me job path (path from search results)."`
}

// cakeDetailOutput mirrors cake.JobDetail for the LLM: identical fields and
// JSON names.
type cakeDetailOutput struct {
	ID           int    `json:"id"`
	Path         string `json:"path"`
	URL          string `json:"url" jsonschema:"Public Cake.me job posting URL."`
	PagePath     string `json:"page_path" jsonschema:"Company page slug; the public job page is https://www.cake.me/companies/{page_path}/jobs/{path}."`
	Title        string `json:"title"`
	Description  string `json:"description" jsonschema:"Full job description as plain text/markdown."`
	Requirements string `json:"requirements" jsonschema:"Job requirements as plain text/markdown; may be empty."`
}

type cakeJobSummary struct {
	Path        string `json:"path" jsonschema:"Job path; pass to cake_get_job_detail."`
	URL         string `json:"url" jsonschema:"Public Cake.me job posting URL."`
	Title       string `json:"title"`
	Description string `json:"description" jsonschema:"Plain-text preview; cake_get_job_detail returns the full description."`
}

type cakeFilterOption struct {
	Value string `json:"value" jsonschema:"Value to pass to cake_search_jobs (e.g. 'full_time')."`
	Name  string `json:"name" jsonschema:"Human-readable display name."`
}

type cakeFiltersOutput struct {
	JobTypes        []cakeFilterOption `json:"job_types" jsonschema:"Available employment types."`
	SeniorityLevels []cakeFilterOption `json:"seniority_levels" jsonschema:"Available seniority levels."`
	Remote          []cakeFilterOption `json:"remote" jsonschema:"Available remote-work policies."`
	YearOfSeniority []cakeFilterOption `json:"year_of_seniority,omitempty" jsonschema:"Years of experience ranges."`
	Locations       []string           `json:"locations,omitempty" jsonschema:"Sample location names."`
	Professions     []cakeFilterOption `json:"professions,omitempty" jsonschema:"Sample profession categories."`
}

var cakeJobTypeLabels = map[string]string{
	"full_time":  "全職 / Full-time",
	"part_time":  "兼職 / Part-time",
	"internship": "實習生 / Internship",
	"contract":   "約聘 / Contract",
	"freelance":  "接案 / Freelance",
	"temporary":  "臨時工 / Temporary",
	"volunteer":  "志願者 / Volunteer",
}

var cakeSeniorityLabels = map[string]string{
	"internship_level": "實習 / Internship",
	"entry_level":      "初階 / Entry level",
	"associate":        "助理 / Associate",
	"mid_senior_level": "中高階 / Mid-Senior level",
	"director":         "經理 / 總監 / Director",
	"executive":        "經營層 (VP, GM, C-Level) / Executive",
}

var cakeRemoteLabels = map[string]string{
	"no_remote_work":       "無法遠端工作 / On-site only",
	"partial_remote_work":  "部分遠端工作 / Partial remote",
	"optional_remote_work": "彈性遠端工作 / Optional remote",
	"full_remote_work":     "100% 遠端工作 / Remote only",
}

var cakeYearOfSeniorityLabels = map[string]string{
	"0_1":  "< 1 年 (Less than 1 year)",
	"1_3":  "1〜3 年 (1-3 years)",
	"3_5":  "3〜5 年 (3-5 years)",
	"5_10": "5〜10 年 (5-10 years)",
	"10_":  "10+ 年 (More than 10 years)",
}

var cakeJobTypeAliases = map[string]string{
	"full-time":  "full_time",
	"fulltime":   "full_time",
	"full time":  "full_time",
	"全職":         "full_time",
	"part-time":  "part_time",
	"parttime":   "part_time",
	"part time":  "part_time",
	"兼職":         "part_time",
	"intern":     "internship",
	"實習":         "internship",
	"實習生":        "internship",
	"約聘":         "contract",
	"約聘工":        "contract",
	"接案":         "freelance",
	"自由職業者":      "freelance",
	"temp":       "temporary",
	"臨時工":        "temporary",
	"兼差":         "temporary",
	"志工":         "volunteer",
	"志願者":        "volunteer",
}

var cakeSeniorityAliases = map[string]string{
	"intern":           "internship_level",
	"internship":       "internship_level",
	"實習":               "internship_level",
	"entry":            "entry_level",
	"entry level":      "entry_level",
	"entry-level":      "entry_level",
	"junior":           "entry_level",
	"初階":               "entry_level",
	"初級":               "entry_level",
	"assistant":        "associate",
	"助理":               "associate",
	"mid_senior":       "mid_senior_level",
	"mid-senior":       "mid_senior_level",
	"mid senior":       "mid_senior_level",
	"mid-senior level": "mid_senior_level",
	"mid senior level": "mid_senior_level",
	"senior":           "mid_senior_level",
	"中高階":              "mid_senior_level",
	"資深":               "mid_senior_level",
	"中階":               "mid_senior_level",
	"manager":          "director",
	"經理":               "director",
	"總監":               "director",
	"主管":               "director",
	"c-level":          "executive",
	"c level":          "executive",
	"vp":               "executive",
	"gm":               "executive",
	"經營層":              "executive",
	"高階主管":             "executive",
}

var cakeRemoteAliases = map[string]string{
	"no_remote":       "no_remote_work",
	"no remote":       "no_remote_work",
	"onsite":          "no_remote_work",
	"on-site":         "no_remote_work",
	"on site":         "no_remote_work",
	"無遠端":             "no_remote_work",
	"無法遠端工作":         "no_remote_work",
	"不遠端":            "no_remote_work",
	"現場工作":           "no_remote_work",
	"partial_remote":  "partial_remote_work",
	"partial remote":  "partial_remote_work",
	"hybrid":          "partial_remote_work",
	"部分遠端":           "partial_remote_work",
	"部分遠端工作":         "partial_remote_work",
	"混合辦公":           "partial_remote_work",
	"optional_remote": "optional_remote_work",
	"optional remote": "optional_remote_work",
	"彈性遠端":           "optional_remote_work",
	"選擇性或彈性遠端工作":     "optional_remote_work",
	"full_remote":     "full_remote_work",
	"full remote":     "full_remote_work",
	"remote":          "full_remote_work",
	"remote_only":     "full_remote_work",
	"remote only":     "full_remote_work",
	"純遠端":             "full_remote_work",
	"100% 遠端工作":       "full_remote_work",
	"遠端":              "full_remote_work",
	"完全遠端":           "full_remote_work",
}

func normalizeCakeJobType(s string) string {
	lower := strings.TrimSpace(strings.ToLower(s))
	if _, ok := cakeJobTypeLabels[lower]; ok {
		return lower
	}
	for val, label := range cakeJobTypeLabels {
		if strings.ToLower(label) == lower {
			return val
		}
	}
	if alias, ok := cakeJobTypeAliases[lower]; ok {
		return alias
	}
	return s
}

func normalizeCakeSeniority(s string) string {
	lower := strings.TrimSpace(strings.ToLower(s))
	if _, ok := cakeSeniorityLabels[lower]; ok {
		return lower
	}
	for val, label := range cakeSeniorityLabels {
		if strings.ToLower(label) == lower {
			return val
		}
	}
	if alias, ok := cakeSeniorityAliases[lower]; ok {
		return alias
	}
	return s
}

func normalizeCakeRemote(s string) string {
	lower := strings.TrimSpace(strings.ToLower(s))
	if _, ok := cakeRemoteLabels[lower]; ok {
		return lower
	}
	for val, label := range cakeRemoteLabels {
		if strings.ToLower(label) == lower {
			return val
		}
	}
	if alias, ok := cakeRemoteAliases[lower]; ok {
		return alias
	}
	return s
}

func normalizeCakeSort(s string) (cake.JobSearchRequestSortBy, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "", "popularity", "熱門", "最熱門":
		return cake.JobSearchRequestSortByPopularity, nil
	case "latest", "newest", "最新":
		return cake.JobSearchRequestSortByLatest, nil
	default:
		return "", fmt.Errorf("invalid sort %q: must be popularity or latest", s)
	}
}

func cakeMCPToHTTPRequest(in *cakeSearchInput) (*cake.JobSearchRequest, error) {
	var req cake.JobSearchRequest
	if in.Keyword == "" {
		return nil, errors.New("keyword is required")
	}
	req.Query = in.Keyword

	if in.Location == "" {
		return nil, errors.New("location is required")
	}
	req.Filters.Locations = []string{in.Location}

	sortBy, err := normalizeCakeSort(in.Sort)
	if err != nil {
		return nil, err
	}
	req.SortBy = sortBy

	if in.JobType != "" {
		req.Filters.JobTypes = []string{normalizeCakeJobType(in.JobType)}
	}

	for _, slug := range in.Seniority {
		if norm := normalizeCakeSeniority(slug); norm != "" {
			req.Filters.SeniorityLevels = append(req.Filters.SeniorityLevels, norm)
		}
	}

	if in.Remote != "" {
		req.Filters.Remote = []string{normalizeCakeRemote(in.Remote)}
	}

	if in.Page > 0 {
		req.Page = cake.NewOptInt(in.Page)
	}
	return &req, nil
}

func cakeHTTPToMCPResponse(resp *cake.JobSearchResponse) *cakeSearchOutput {
	out := &cakeSearchOutput{
		TotalEntries: resp.TotalEntries.Value,
		TotalPages:   resp.TotalPages.Value,
		PerPage:      resp.PerPage.Value,
		CurrentPage:  resp.CurrentPage.Value,
		Data:         make([]cakeJobSummary, 0, len(resp.Data)),
	}
	for _, j := range resp.Data {
		var pagePath string
		if page, ok := j.Page.Get(); ok {
			pagePath = page.Path.Value
		}
		out.Data = append(out.Data, cakeJobSummary{
			Path:        j.Path,
			URL:         cakeJobURL(pagePath, j.Path),
			Title:       j.Title.Value,
			Description: j.Description.Value,
		})
	}
	return out
}

func cakeHTTPToMCPFilters(res *cake.JobSearchResponse) *cakeFiltersOutput {
	out := &cakeFiltersOutput{}
	facets, ok := res.AvailableFacets.Get()
	if !ok {
		for k, v := range cakeJobTypeLabels {
			out.JobTypes = append(out.JobTypes, cakeFilterOption{Value: k, Name: v})
		}
		for k, v := range cakeSeniorityLabels {
			out.SeniorityLevels = append(out.SeniorityLevels, cakeFilterOption{Value: k, Name: v})
		}
		for k, v := range cakeRemoteLabels {
			out.Remote = append(out.Remote, cakeFilterOption{Value: k, Name: v})
		}
		return out
	}

	for _, item := range facets.JobTypes {
		name := item
		if label, exists := cakeJobTypeLabels[item]; exists {
			name = label
		}
		out.JobTypes = append(out.JobTypes, cakeFilterOption{Value: item, Name: name})
	}

	for _, item := range facets.SeniorityLevels {
		name := item
		if label, exists := cakeSeniorityLabels[item]; exists {
			name = label
		}
		out.SeniorityLevels = append(out.SeniorityLevels, cakeFilterOption{Value: item, Name: name})
	}

	for _, item := range facets.Remote {
		name := item
		if label, exists := cakeRemoteLabels[item]; exists {
			name = label
		}
		out.Remote = append(out.Remote, cakeFilterOption{Value: item, Name: name})
	}

	for _, item := range facets.YearOfSeniority {
		name := item
		if label, exists := cakeYearOfSeniorityLabels[item]; exists {
			name = label
		}
		out.YearOfSeniority = append(out.YearOfSeniority, cakeFilterOption{Value: item, Name: name})
	}

	out.Locations = facets.Locations

	for _, item := range facets.Professions {
		out.Professions = append(out.Professions, cakeFilterOption{Value: item, Name: item})
	}

	return out
}

func cakeHTTPToMCPDetail(detail *cake.JobDetail) *cakeDetailOutput {
	descText, err := html2text.FromString(detail.Description.Value, html2text.Options{})
	if err != nil {
		descText = detail.Description.Value
	}
	reqsText, err := html2text.FromString(detail.Requirements.Value, html2text.Options{})
	if err != nil {
		reqsText = detail.Requirements.Value
	}
	return &cakeDetailOutput{
		ID:           detail.ID.Value,
		Path:         detail.Path.Value,
		URL:          cakeJobURL(detail.PagePath.Value, detail.Path.Value),
		PagePath:     detail.PagePath.Value,
		Title:        detail.Title.Value,
		Description:  descText,
		Requirements: reqsText,
	}
}

func cakeJobURL(pagePath, path string) string {
	if pagePath == "" || path == "" {
		return ""
	}
	return fmt.Sprintf("https://www.cake.me/companies/%s/jobs/%s", pagePath, path)
}

// RegisterCake registers the Cake.me search, filter discovery, and job-detail tools.
func RegisterCake(s *mcp.Server, c *cake.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        cakeSearchToolName,
		Description: "Search jobs on Cake.me (formerly CakeResume), a Taiwan-focused job board.",
		Annotations: &mcp.ToolAnnotations{Title: "Search Cake.me jobs", ReadOnlyHint: true},
		InputSchema: cakeSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *cakeSearchInput) (*mcp.CallToolResult, *cakeSearchOutput, error) {
		req, err := cakeMCPToHTTPRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		res, err := c.SearchJobs(ctx, req)
		if err != nil {
			if ue, ok := errors.AsType[*cake.ErrorResponseStatusCode](err); ok {
				return errorResult(fmt.Errorf("upstream error: %d", ue.StatusCode)), nil, nil
			}
			return errorResult(err), nil, nil
		}
		return nil, cakeHTTPToMCPResponse(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        cakeFiltersToolName,
		Description: "Get Cake.me current search filter values (job types, seniority levels, remote policies, and locations). Call before filtered cake_search_jobs queries.",
		Annotations: &mcp.ToolAnnotations{Title: "Get Cake.me search filters", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ *struct{}) (*mcp.CallToolResult, *cakeFiltersOutput, error) {
		req := &cake.JobSearchRequest{
			Query:   "",
			SortBy:  cake.JobSearchRequestSortByPopularity,
			Filters: cake.JobSearchFilters{},
		}
		res, err := c.SearchJobs(ctx, req)
		if err != nil {
			if ue, ok := errors.AsType[*cake.ErrorResponseStatusCode](err); ok {
				return errorResult(fmt.Errorf("upstream error: %d", ue.StatusCode)), nil, nil
			}
			return errorResult(err), nil, nil
		}
		return nil, cakeHTTPToMCPFilters(res), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        cakeDetailToolName,
		Description: "Get the full job description and requirements for a Cake.me job path (path from search results).",
		Annotations: &mcp.ToolAnnotations{Title: "Get Cake.me job details", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *cakeDetailInput) (*mcp.CallToolResult, *cakeDetailOutput, error) {
		res, err := c.GetJobDetail(ctx, cake.GetJobDetailParams{Path: in.Path})
		if err != nil {
			if ue, ok := errors.AsType[*cake.ErrorResponseStatusCode](err); ok {
				return errorResult(fmt.Errorf("upstream error: %d", ue.StatusCode)), nil, nil
			}
			return errorResult(err), nil, nil
		}
		return nil, cakeHTTPToMCPDetail(res), nil
	})
}
