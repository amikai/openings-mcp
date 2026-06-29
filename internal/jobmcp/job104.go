package jobmcp

import (
	"context"
	"fmt"

	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type job104SearchInput struct {
	Keyword string `json:"keyword" jsonschema:"search keyword, required"`
	Area    string `json:"area,omitempty" jsonschema:"city filter; one of: taipei, new_taipei, taoyuan, taichung, tainan, kaohsiung"`
	JobType string `json:"job_type,omitempty" jsonschema:"employment basis; one of: full, part"`
	Sort    string `json:"sort,omitempty" jsonschema:"result order; one of: newest, relevance"`
	Remote  string `json:"remote,omitempty" jsonschema:"remote work; one of: none, partial, full"`
	Page    int    `json:"page,omitempty" jsonschema:"1-based page number"`
}

type job104DetailInput struct {
	JobCode string `json:"job_code" jsonschema:"104 job code (jobNo), required"`
}

var (
	job104Areas = map[string]string{
		"taipei":     job104.AreaTaipei,
		"new_taipei": job104.AreaNewTaipei,
		"taoyuan":    job104.AreaTaoyuan,
		"taichung":   job104.AreaTaichung,
		"tainan":     job104.AreaTainan,
		"kaohsiung":  job104.AreaKaohsiung,
	}
	job104JobType = map[string]int{"full": 0, "part": 1}
	job104Sort    = map[string]int{"newest": 15, "relevance": 1}
	job104Remote  = map[string]int{"none": 0, "partial": 1, "full": 2}
)

func job104ToRequest(in job104SearchInput) (*job104.JobsRequest, error) {
	r := &job104.JobsRequest{Keyword: in.Keyword}
	if in.Area != "" {
		code, ok := job104Areas[in.Area]
		if !ok {
			return nil, fmt.Errorf("invalid area %q (want taipei|new_taipei|taoyuan|taichung|tainan|kaohsiung)", in.Area)
		}
		r.Area = code
	}
	if in.JobType != "" {
		v, ok := job104JobType[in.JobType]
		if !ok {
			return nil, fmt.Errorf("invalid job_type %q (want full|part)", in.JobType)
		}
		r.RO = &v
	}
	if in.Sort != "" {
		v, ok := job104Sort[in.Sort]
		if !ok {
			return nil, fmt.Errorf("invalid sort %q (want newest|relevance)", in.Sort)
		}
		r.Order = &v
	}
	if in.Remote != "" {
		v, ok := job104Remote[in.Remote]
		if !ok {
			return nil, fmt.Errorf("invalid remote %q (want none|partial|full)", in.Remote)
		}
		r.RemoteWork = &v
	}
	if in.Page > 0 {
		p := in.Page
		r.Page = &p
	}
	return r, nil
}

// RegisterJob104 registers the 104 search and job-detail tools.
func RegisterJob104(s *mcp.Server, c *job104.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_search_jobs",
		Description: "Search jobs on 104 (Taiwan's largest job board) by keyword, with optional city/job-type/remote/sort filters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in job104SearchInput) (*mcp.CallToolResult, any, error) {
		req, err := job104ToRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, err := c.Jobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, resp, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_get_job_detail",
		Description: "Get the full job description for a 104 job code (jobNo from search results).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in job104DetailInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.JobDetail(ctx, in.JobCode)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, resp, nil
	})
}
