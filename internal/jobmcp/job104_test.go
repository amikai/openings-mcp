package jobmcp

import (
	"context"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterJob104(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client, err := job104.NewClient("https://www.104.com.tw")
	require.NoError(t, err)
	RegisterJob104(server, client)

	assertTools(t, server, "104_search_jobs", "104_get_job_detail")
}

func TestJob104SearchJobsSchema(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	client, err := job104.NewClient("https://www.104.com.tw")
	require.NoError(t, err)
	RegisterJob104(server, client)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer clientSession.Close()

	res, err := clientSession.ListTools(ctx, nil)
	require.NoError(t, err)

	var searchTool *mcp.Tool
	for _, tool := range res.Tools {
		if tool.Name == "104_search_jobs" {
			searchTool = tool
			break
		}
	}
	require.NotNil(t, searchTool)

	schema, ok := searchTool.InputSchema.(map[string]any)
	require.True(t, ok)

	// area's 74-label enum is impractical to hand-type; spot-check the ends
	// of the list, then strip it so the golden compare below stays a single
	// hand-typed whole-value assertion.
	area, ok := schema["properties"].(map[string]any)["area"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, area["enum"], "Taipei")
	assert.Contains(t, area["enum"], "WestAfrica")
	delete(area, "enum")

	// Full golden schema: LLM-facing names only (no ro/order/remoteWork/s9),
	// label enums instead of raw codes, keyword and area required.
	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword": map[string]any{
				"type":        "string",
				"description": "Free-text keyword search.",
			},
			"area": map[string]any{
				"type":        "string",
				"description": "City/region filter.",
			},
			"job_type": map[string]any{
				"type":        "string",
				"description": "Employment basis. Soft filter — verify each result's jobRo.",
				"enum":        []any{"Full-time", "Part-time", "Senior", "Dispatch"},
			},
			"sort": map[string]any{
				"type":        "string",
				"description": "Result order.",
				"enum":        []any{"Relevance", "Newest"},
			},
			"remote": map[string]any{
				"type":        "string",
				"description": "Remote work. Soft filter — verify each result's remoteWorkType. Omit for on-site.",
				"enum":        []any{"Full", "Partial"},
			},
			"edu": map[string]any{
				"type":        "array",
				"description": "Education levels, OR'd together.",
				"uniqueItems": true,
				"items": map[string]any{
					"type": "string",
					"enum": []any{"HighSchoolBelow", "HighSchool", "College", "University", "Master", "Doctorate"},
				},
			},
			"page": map[string]any{
				"type":        "integer",
				"description": "1-based page number.",
				"minimum":     float64(1),
			},
		},
		"required":             []any{"keyword", "area"},
		"additionalProperties": false,
	}
	assert.Equal(t, want, schema)
}

func TestJob104MCPToHTTPRequest(t *testing.T) {
	in := job104SearchInput{
		Keyword: "golang",
		Area:    "Taipei",
		JobType: "Part-time",
		Sort:    "Newest",
		Remote:  "Full",
		Edu:     []string{"University", "Master"},
		Page:    2,
	}
	got, err := job104MCPToHTTPRequest(&in)
	require.NoError(t, err)

	want := job104.SearchJobsParams{
		Keyword:    job104.NewOptString("golang"),
		Area:       job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
		Ro:         job104.NewOptSearchJobsRo(job104.SearchJobsRo2),
		Order:      job104.NewOptSearchJobsOrder(job104.SearchJobsOrder2),
		RemoteWork: job104.NewOptSearchJobsRemoteWork(job104.SearchJobsRemoteWork1),
		Page:       job104.NewOptInt(2),
		Edu:        []job104.SearchJobsEduItem{job104.SearchJobsEduItem4, job104.SearchJobsEduItem5},
	}
	assert.Equal(t, want, *got)
}

func TestJob104MCPToHTTPRequestMissingRequired(t *testing.T) {
	cases := []struct {
		name string
		in   job104SearchInput
		want string
	}{
		{"all empty", job104SearchInput{}, "keyword is required"},
		{"filters only", job104SearchInput{Area: "Taipei", Sort: "Newest", Page: 2}, "keyword is required"},
		{"keyword only", job104SearchInput{Keyword: "golang"}, `invalid area ""`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := job104MCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestJob104MCPToHTTPRequestMinimal(t *testing.T) {
	got, err := job104MCPToHTTPRequest(&job104SearchInput{Keyword: "golang", Area: "Taipei"})
	require.NoError(t, err)
	want := job104.SearchJobsParams{
		Keyword: job104.NewOptString("golang"),
		Area:    job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
	}
	assert.Equal(t, want, *got)
}

func TestJob104MCPToHTTPRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   job104SearchInput
		want string
	}{
		{"area", job104SearchInput{Keyword: "x", Area: "Mars"}, `invalid area "Mars"`},
		{"job_type", job104SearchInput{Keyword: "x", Area: "Taipei", JobType: "full"}, `invalid job_type "full"`},
		{"sort", job104SearchInput{Keyword: "x", Area: "Taipei", Sort: "newest"}, `invalid sort "newest"`},
		{"remote", job104SearchInput{Keyword: "x", Area: "Taipei", Remote: "hybrid"}, `invalid remote "hybrid"`},
		{"edu", job104SearchInput{Keyword: "x", Area: "Taipei", Edu: []string{"University", "PhD"}}, `invalid edu "PhD"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := job104MCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
