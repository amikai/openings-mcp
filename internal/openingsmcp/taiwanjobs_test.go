package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/taiwanjobs"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTaiwanjobsMCPClientServer(t *testing.T) *mcp.ClientSession {
	t.Helper()
	srv := taiwanjobs.NewMockServer()
	t.Cleanup(srv.Close)
	c := taiwanjobs.NewClient(srv.URL, srv.Client())

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterTaiwanjobs(server, c)

	t1, t2 := mcp.NewInMemoryTransports()
	ss, err := server.Connect(t.Context(), t1, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	cs, err := client.Connect(t.Context(), t2, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestRegisterTaiwanjobs(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterTaiwanjobs(server, taiwanjobs.NewClient("https://free.taiwanjobs.gov.tw", nil))
	assertTools(t, server, "taiwanjobs_search_jobs")
}

func TestTaiwanjobsSearchTool(t *testing.T) {
	cs := testTaiwanjobsMCPClientServer(t)
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "taiwanjobs_search_jobs",
		Arguments: map[string]any{"keyword": "java"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	data, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var out taiwanjobsSearchOutput
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, 3, out.Fetched)
	require.Len(t, out.Jobs, 1)
	assert.Equal(t, "指南動力有限公司", out.Jobs[0].Company)
	assert.NotEmpty(t, out.Jobs[0].URL)
	assert.NotEmpty(t, out.Jobs[0].Description)
}
