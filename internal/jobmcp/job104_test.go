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
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)

	// LLM-facing names only — no 104 API names.
	for _, field := range []string{"keyword", "area", "job_type", "sort", "remote", "edu", "shift", "page"} {
		assert.Contains(t, props, field)
	}
	for _, field := range []string{"ro", "order", "remoteWork", "s9"} {
		assert.NotContains(t, props, field)
	}

	// Label enums, not raw codes.
	area := props["area"].(map[string]any)
	assert.Contains(t, area["enum"], "Taipei")
	assert.NotContains(t, area["enum"], "6001001000")
	assert.Len(t, area["enum"], len(job104.AreaIDs))

	jobType := props["job_type"].(map[string]any)
	assert.Equal(t, []any{"Full-time", "Part-time", "Senior", "Dispatch"}, jobType["enum"])

	sort := props["sort"].(map[string]any)
	assert.Equal(t, []any{"Relevance", "Newest"}, sort["enum"])

	remote := props["remote"].(map[string]any)
	assert.Equal(t, []any{"Full", "Partial"}, remote["enum"])

	edu := props["edu"].(map[string]any)
	eduItems := edu["items"].(map[string]any)
	assert.Equal(t, []any{"HighSchoolBelow", "HighSchool", "College", "University", "Master", "Doctorate"}, eduItems["enum"])

	shift := props["shift"].(map[string]any)
	shiftItems := shift["items"].(map[string]any)
	assert.Equal(t, []any{"Day", "Night", "Graveyard", "Holiday"}, shiftItems["enum"])
}

func TestJob104ToRequest(t *testing.T) {
	in := job104SearchInput{
		Keyword: "golang",
		Area:    "Taipei",
		JobType: "Part-time",
		Sort:    "Newest",
		Remote:  "Full",
		Edu:     []string{"University", "Master"},
		Shift:   []string{"Day", "Holiday"},
		Page:    2,
	}
	got, err := job104ToRequest(in)
	require.NoError(t, err)

	want := job104.SearchJobsParams{
		Keyword:    job104.NewOptString("golang"),
		Area:       job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
		Ro:         job104.NewOptSearchJobsRo(job104.SearchJobsRo2),
		Order:      job104.NewOptSearchJobsOrder(job104.SearchJobsOrder2),
		RemoteWork: job104.NewOptSearchJobsRemoteWork(job104.SearchJobsRemoteWork1),
		Page:       job104.NewOptInt(2),
		Edu:        []job104.SearchJobsEduItem{job104.SearchJobsEduItem4, job104.SearchJobsEduItem5},
		S9:         []job104.SearchJobsS9Item{job104.SearchJobsS9Item1, job104.SearchJobsS9Item8},
	}
	assert.Equal(t, want, got)
}

func TestJob104ToRequestEmpty(t *testing.T) {
	got, err := job104ToRequest(job104SearchInput{})
	require.NoError(t, err)
	assert.Equal(t, job104.SearchJobsParams{}, got)
}

func TestJob104ToRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   job104SearchInput
		want string
	}{
		{"area", job104SearchInput{Area: "Mars"}, `invalid area "Mars"`},
		{"job_type", job104SearchInput{JobType: "full"}, `invalid job_type "full"`},
		{"sort", job104SearchInput{Sort: "newest"}, `invalid sort "newest"`},
		{"remote", job104SearchInput{Remote: "hybrid"}, `invalid remote "hybrid"`},
		{"edu", job104SearchInput{Edu: []string{"University", "PhD"}}, `invalid edu "PhD"`},
		{"shift", job104SearchInput{Shift: []string{"Midnight"}}, `invalid shift "Midnight"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := job104ToRequest(tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
