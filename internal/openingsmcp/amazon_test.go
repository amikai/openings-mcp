package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/amazon"
)

func testAmazonMCPClientServer(t *testing.T) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	mockServer := amazon.NewMockServer()
	t.Cleanup(mockServer.Close)
	client, err := amazon.NewClient(mockServer.URL, amazon.WithClient(mockServer.Client()))
	require.NoError(t, err)
	RegisterAmazon(server, client)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, serverSession.Close()) })
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := mcpClient.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clientSession.Close()) })
	return clientSession
}

func TestRegisterAmazon(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	client, err := amazon.NewClient("https://www.amazon.jobs")
	require.NoError(t, err)
	RegisterAmazon(server, client)
	assertTools(t, server, "amazon_search_jobs", "amazon_get_job_detail")
}

func TestAmazonSearchJobsE2E(t *testing.T) {
	clientSession := testAmazonMCPClientServer(t)

	list, err := clientSession.ListTools(t.Context(), nil)
	require.NoError(t, err)
	tool := findTool(list.Tools, "amazon_search_jobs")
	require.NotNil(t, tool)
	schema, ok := tool.InputSchema.(map[string]any)
	require.True(t, ok)
	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "^[A-Z]{3}$", properties["country"].(map[string]any)["pattern"])
	assert.Equal(t, float64(100), properties["limit"].(map[string]any)["maximum"])
	assert.Equal(t, "relevant", properties["sort"].(map[string]any)["default"])

	response, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "amazon_search_jobs",
		Arguments: map[string]any{
			"keyword": "software engineer",
			"country": "TWN",
			"limit":   2,
		},
	})
	require.NoError(t, err)
	require.False(t, response.IsError)
	data, err := json.Marshal(response.StructuredContent)
	require.NoError(t, err)
	var output amazonSearchOutput
	require.NoError(t, json.Unmarshal(data, &output))
	require.Len(t, output.Data, 2)
	assert.Equal(t, 11, output.Total)
	assert.Equal(t, "3164253", output.Data[0].ID)
	assert.Equal(t, "TWN", output.Data[0].CountryCode)
	assert.Contains(t, output.Data[0].URL, "/en/jobs/3164253/")
}

func TestAmazonGetJobDetailE2E(t *testing.T) {
	clientSession := testAmazonMCPClientServer(t)
	response, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "amazon_get_job_detail",
		Arguments: map[string]any{"job_id": "3164253"},
	})
	require.NoError(t, err)
	require.False(t, response.IsError)
	data, err := json.Marshal(response.StructuredContent)
	require.NoError(t, err)
	var output amazonDetailOutput
	require.NoError(t, json.Unmarshal(data, &output))
	assert.Equal(t, "3164253", output.ID)
	assert.Equal(t, "Software Dev Engineer, eero", output.Title)
	assert.NotEmpty(t, output.Description)
	assert.NotContains(t, output.Description, "<br")
	assert.NotEmpty(t, output.BasicQualifications)
	assert.NotEmpty(t, output.PreferredQualifications)
}

func TestAmazonGetJobDetailNotFound(t *testing.T) {
	clientSession := testAmazonMCPClientServer(t)
	response, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "amazon_get_job_detail",
		Arguments: map[string]any{"job_id": "9999999999"},
	})
	require.NoError(t, err)
	require.True(t, response.IsError)
	assert.Nil(t, response.StructuredContent)
	assert.Contains(t, response.Content[0].(*mcp.TextContent).Text, "job not found")
}

func TestAmazonMCPToSearchRequestRejectsCountry(t *testing.T) {
	_, err := amazonMCPToSearchRequest(&amazonSearchInput{Country: "tw"})
	assert.ErrorContains(t, err, "expected an uppercase ISO-3 code")

	_, err = amazonMCPToSearchRequest(&amazonSearchInput{Country: "12A"})
	assert.ErrorContains(t, err, "expected an uppercase ISO-3 code")
}
