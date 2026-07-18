package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/apple"
)

const (
	appleTestServerName = "test"
	appleTestClientName = "test-client"
	appleTestKeywordKey = "keyword"
	appleTestKeyword    = "software engineer"
	appleTestPageKey    = "page"
	appleTestSortKey    = "sort"
	appleTestJobIDKey   = "job_id"
	appleTestLocation   = "Taipei, Taiwan"
)

func testAppleMCPClientServer(t *testing.T) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: appleTestServerName, Version: "v0"}, nil)
	mock := apple.NewMockServer()
	t.Cleanup(mock.Close)
	client, err := apple.NewJobsClient(mock.URL, mock.Client())
	require.NoError(t, err)
	RegisterApple(server, client)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: appleTestClientName, Version: "v0"}, nil)
	clientSession, err := mcpClient.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func TestRegisterApple(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: appleTestServerName, Version: "v0"}, nil)
	client, err := apple.NewJobsClient("https://jobs.apple.com", nil)
	require.NoError(t, err)
	RegisterApple(server, client)
	assertTools(t, server, appleSearchToolName, appleDetailToolName)
}

func TestAppleSearchJobsE2E(t *testing.T) {
	client := testAppleMCPClientServer(t)
	tools, err := client.ListTools(t.Context(), nil)
	require.NoError(t, err)
	tool := findTool(tools.Tools, "apple_search_jobs")
	require.NotNil(t, tool)
	schema, ok := tool.InputSchema.(map[string]any)
	require.True(t, ok)
	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	country, ok := properties["country_code"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "^[A-Za-z]{3}$", country["pattern"])

	result, err := client.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "apple_search_jobs",
		Arguments: map[string]any{
			appleTestKeywordKey: appleTestKeyword,
			"country_code":      "TWN",
			appleTestSortKey:    string(apple.SortRelevance),
			appleTestPageKey:    1,
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	data, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	var output appleSearchOutput
	require.NoError(t, json.Unmarshal(data, &output))
	assert.Equal(t, 11, output.Total)
	assert.Equal(t, 1, output.Page)
	require.Len(t, output.Data, 11)
	assert.Equal(t, appleJobSummary{
		JobID:       apple.MockJobID,
		URL:         "https://jobs.apple.com/en-us/details/200624996/soc-packaging-engineer",
		Title:       "SoC Packaging Engineer",
		Team:        "Hardware",
		Locations:   []string{appleTestLocation},
		PostedOn:    "Jun 29, 2026",
		WeeklyHours: 40,
		Summary:     output.Data[0].Summary,
	}, output.Data[0])
	assert.NotEmpty(t, output.Data[0].Summary)
}

func TestAppleGetJobDetailE2E(t *testing.T) {
	client := testAppleMCPClientServer(t)
	result, err := client.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      appleDetailToolName,
		Arguments: map[string]any{appleTestJobIDKey: apple.MockJobID},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	data, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	var output appleDetailOutput
	require.NoError(t, json.Unmarshal(data, &output))
	assert.Equal(t, apple.MockJobID, output.JobID)
	assert.Equal(t, "SoC Packaging Engineer", output.Title)
	assert.Equal(t, []string{appleTestLocation}, output.Locations)
	assert.Equal(t, "Standard", output.EmploymentType)
	assert.NotEmpty(t, output.Responsibilities)
	assert.NotEmpty(t, output.MinimumQualifications)
}

func TestAppleGetJobDetailNotFoundE2E(t *testing.T) {
	client := testAppleMCPClientServer(t)
	result, err := client.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      appleDetailToolName,
		Arguments: map[string]any{appleTestJobIDKey: apple.MockNotFoundJobID},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	content, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, content.Text, "job not found")
}

func TestAppleLocationLabel(t *testing.T) {
	assert.Equal(t, appleTestLocation, appleLocationLabel("Taipei", "Taiwan"))
	assert.Equal(t, "Taiwan", appleLocationLabel("Taiwan", "Taiwan"))
	assert.Equal(t, "Taiwan", appleLocationLabel("", "Taiwan"))
}
