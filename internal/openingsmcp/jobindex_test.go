package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/jobindex"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testJobindexMCPClientServer(t *testing.T) *mcp.ClientSession {
	t.Helper()
	srv := jobindex.NewMockServer()
	t.Cleanup(srv.Close)
	c := jobindex.NewClient(srv.URL, srv.Client())

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterJobindex(server, c)

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

func TestRegisterJobindex(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterJobindex(server, jobindex.NewClient("https://www.jobindex.dk", nil))
	assertTools(t, server, "jobindex_search_jobs", "jobindex_get_job_detail")
}

func TestJobindexSearchTool(t *testing.T) {
	cs := testJobindexMCPClientServer(t)
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "jobindex_search_jobs",
		Arguments: map[string]any{"keyword": "backend", "page": 1},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	data, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var out jobindexSearchOutput
	require.NoError(t, json.Unmarshal(data, &out))
	require.NotEmpty(t, out.Results)
	assert.Greater(t, out.Hitcount, 0)
	assert.Equal(t, "h1683131", out.Results[0]["tid"])
	assert.Equal(t, "Senior Backend Engineer", out.Results[0]["headline"])
	_, hasHTML := out.Results[0]["html"]
	assert.False(t, hasHTML)
}

func TestJobindexDetailTool(t *testing.T) {
	cs := testJobindexMCPClientServer(t)
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "jobindex_get_job_detail",
		Arguments: map[string]any{"tid": "h1683131"},
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	data, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var out jobindexDetailOutput
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, "h1683131", out.Tid)
	assert.Equal(t, "Senior Backend Engineer", out.Headline)
	assert.Contains(t, out.Description, "Whiteaway")
}

func TestJobindexMCPToHTTPRequest(t *testing.T) {
	req, err := jobindexMCPToHTTPRequest(&jobindexSearchInput{
		Keyword:    "go",
		Area:       "storkoebenhavn",
		JobAgeDays: 14,
		Sort:       "date",
		Page:       2,
	})
	require.NoError(t, err)
	assert.Equal(t, "go", req.Keyword)
	assert.Equal(t, "storkoebenhavn", req.Area)
	assert.Equal(t, 14, req.JobAgeDays)
	assert.Equal(t, jobindex.SortDate, req.Sort)
	assert.Equal(t, 2, req.Page)
}
