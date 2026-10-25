package openingsmcp

import (
	"context"

	"github.com/amikai/openings-mcp/internal/provider/taiwanjobs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var taiwanjobsSearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text keyword search across title, company, and description.",
			"minLength": 1
		},
		"zipno": {
			"type": "string",
			"description": "First three digits of the Taiwan postal code for the work location, e.g. 110 for Taipei Xinyi District.",
			"pattern": "^[0-9]{3}$"
		},
		"jobno": {
			"type": "string",
			"description": "Official job category code: 2-digit major (e.g. 07 for engineering/R&D) or 6-digit minor (e.g. 070113).",
			"pattern": "^[0-9]{2}([0-9]{4})?$"
		},
		"count": {
			"type": "integer",
			"description": "Rows to fetch from the feed before the keyword filter (upstream max 1000). Defaults to 100. There is no pagination.",
			"minimum": 1,
			"maximum": 1000,
			"default": 100
		}
	},
	"additionalProperties": false
}`)

var taiwanjobsSearchInputSchema = mustSchema(taiwanjobsSearchInputRawSchema)

type taiwanjobsSearchInput struct {
	Keyword string `json:"keyword,omitempty"`
	ZipNo   string `json:"zipno,omitempty"`
	JobNo   string `json:"jobno,omitempty"`
	Count   int    `json:"count,omitempty"`
}

// taiwanjobsSearchOutput is the client-facing search payload.
type taiwanjobsSearchOutput struct {
	Fetched int              `json:"fetched" jsonschema:"Rows the feed returned before the keyword filter."`
	Jobs    []taiwanjobs.Job `json:"jobs" jsonschema:"Matching jobs. Each row is the full posting, so there is no separate detail call."`
}

// RegisterTaiwanjobs registers the TaiwanJobs search tool. Search rows carry
// the full posting, so the feed needs no detail endpoint and this file
// registers no detail tool.
func RegisterTaiwanjobs(s *mcp.Server, c *taiwanjobs.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "taiwanjobs_search_jobs",
		Description: "Search jobs on TaiwanJobs (台灣就業通), the Taiwan Ministry of Labor's official job board. Rows carry the full posting, so there's no separate detail tool.",
		Annotations: &mcp.ToolAnnotations{Title: "Search TaiwanJobs jobs", ReadOnlyHint: true},
		InputSchema: taiwanjobsSearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *taiwanjobsSearchInput) (*mcp.CallToolResult, *taiwanjobsSearchOutput, error) {
		res, err := c.Jobs(ctx, &taiwanjobs.JobsRequest{
			Count:   in.Count,
			ZipNo:   in.ZipNo,
			JobNo:   in.JobNo,
			Keyword: in.Keyword,
		})
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, &taiwanjobsSearchOutput{Fetched: res.Fetched, Jobs: res.Jobs}, nil
	})
}
