package jobmcp

import (
	"context"
	"fmt"

	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type job104SearchInput struct {
	Keyword string   `json:"keyword,omitempty"`
	Area    string   `json:"area,omitempty"`
	JobType string   `json:"job_type,omitempty"`
	Sort    string   `json:"sort,omitempty"`
	Remote  string   `json:"remote,omitempty"`
	Edu     []string `json:"edu,omitempty"`
	Shift   []string `json:"shift,omitempty"`
	Page    int      `json:"page,omitempty"`
}

type job104DetailInput struct {
	JobCode string `json:"job_code" jsonschema:"104 job code (jobNo), required"`
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

func job104ToRequest(in job104SearchInput) (job104.SearchJobsParams, error) {
	var params job104.SearchJobsParams
	if in.Keyword != "" {
		params.Keyword = job104.NewOptString(in.Keyword)
	}
	if in.Area != "" {
		code, err := lookupCode("area", in.Area, job104.AreaIDs)
		if err != nil {
			return params, err
		}
		params.Area = job104.NewOptSearchJobsArea(code)
	}
	if in.JobType != "" {
		code, err := lookupCode("job_type", in.JobType, job104.RoIDs)
		if err != nil {
			return params, err
		}
		params.Ro = job104.NewOptSearchJobsRo(code)
	}
	if in.Sort != "" {
		code, err := lookupCode("sort", in.Sort, job104.OrderIDs)
		if err != nil {
			return params, err
		}
		params.Order = job104.NewOptSearchJobsOrder(code)
	}
	if in.Remote != "" {
		code, err := lookupCode("remote", in.Remote, job104.RemoteWorkIDs)
		if err != nil {
			return params, err
		}
		params.RemoteWork = job104.NewOptSearchJobsRemoteWork(code)
	}
	var err error
	if params.Edu, err = lookupCodes("edu", in.Edu, job104.EduIDs); err != nil {
		return params, err
	}
	if params.S9, err = lookupCodes("shift", in.Shift, job104.S9IDs); err != nil {
		return params, err
	}
	if in.Page > 0 {
		params.Page = job104.NewOptInt(in.Page)
	}
	return params, nil
}

// RegisterJob104 registers the 104 search and job-detail tools.
func RegisterJob104(s *mcp.Server, c *job104.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_search_jobs",
		Description: "Search jobs on 104 (Taiwan's largest job board) by keyword, with optional city/job-type/remote/sort filters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in job104SearchInput) (*mcp.CallToolResult, any, error) {
		params, err := job104ToRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, err := c.SearchJobs(ctx, params)
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
