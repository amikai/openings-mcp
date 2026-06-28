package jobmcp

import (
	"context"
	"fmt"

	job104 "github.com/amikai/job-mcp/internal/104"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type tw104SearchInput struct {
	Keyword string `json:"keyword" jsonschema:"search keyword, required"`
	Area    string `json:"area,omitempty" jsonschema:"city filter; one of: taipei, new_taipei, taoyuan, taichung, tainan, kaohsiung"`
	JobType string `json:"job_type,omitempty" jsonschema:"employment basis; one of: full, part"`
	Sort    string `json:"sort,omitempty" jsonschema:"result order; one of: newest, relevance"`
	Remote  string `json:"remote,omitempty" jsonschema:"remote work; one of: none, partial, full"`
	Page    int    `json:"page,omitempty" jsonschema:"1-based page number"`
}

type tw104DetailInput struct {
	JobCode string `json:"job_code" jsonschema:"104 job code (jobNo), required"`
}

var (
	tw104Areas = map[string]string{
		"taipei":     job104.AreaTaipei,
		"new_taipei": job104.AreaNewTaipei,
		"taoyuan":    job104.AreaTaoyuan,
		"taichung":   job104.AreaTaichung,
		"tainan":     job104.AreaTainan,
		"kaohsiung":  job104.AreaKaohsiung,
	}
	tw104JobType = map[string]int{"full": 0, "part": 1}
	tw104Sort    = map[string]int{"newest": 15, "relevance": 1}
	tw104Remote  = map[string]int{"none": 0, "partial": 1, "full": 2}
)

func tw104ToRequest(in tw104SearchInput) (*job104.JobRequest, error) {
	r := &job104.JobRequest{Keyword: in.Keyword}
	if in.Area != "" {
		code, ok := tw104Areas[in.Area]
		if !ok {
			return nil, fmt.Errorf("invalid area %q (want taipei|new_taipei|taoyuan|taichung|tainan|kaohsiung)", in.Area)
		}
		r.Area = code
	}
	if in.JobType != "" {
		v, ok := tw104JobType[in.JobType]
		if !ok {
			return nil, fmt.Errorf("invalid job_type %q (want full|part)", in.JobType)
		}
		r.RO = &v
	}
	if in.Sort != "" {
		v, ok := tw104Sort[in.Sort]
		if !ok {
			return nil, fmt.Errorf("invalid sort %q (want newest|relevance)", in.Sort)
		}
		r.Order = &v
	}
	if in.Remote != "" {
		v, ok := tw104Remote[in.Remote]
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

// RegisterTW104 registers the 104 search and job-detail tools.
func RegisterTW104(s *mcp.Server, c *job104.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "tw104_search_jobs",
		Description: "Search jobs on 104 (Taiwan's largest job board) by keyword, with optional city/job-type/remote/sort filters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in tw104SearchInput) (*mcp.CallToolResult, any, error) {
		req, err := tw104ToRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, err := c.Jobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(job104.FormatSearchJobResponse(resp)), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "tw104_get_job_detail",
		Description: "Get the full job description for a 104 job code (jobNo from search results).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in tw104DetailInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.JobDetail(ctx, in.JobCode)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(job104.FormatJobDetail(resp, in.JobCode)), nil, nil
	})
}
