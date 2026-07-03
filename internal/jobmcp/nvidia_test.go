package jobmcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/nvidia"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockNvidiaServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req nvidia.JobsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		serveTestdata(t, "../provider/nvidia/testdata/jobs_rsp.json")(w, r)
	})

	mux.HandleFunc("/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		serveTestdata(t, "../provider/nvidia/testdata/job_detail_rsp.json")(w, r)
	})

	return httptest.NewServer(mux)
}

func serveTestdata(t *testing.T, path string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func testNvidiaMCPClientServer(t *testing.T) (*mcp.ClientSession, *mcp.ServerSession) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := newMockNvidiaServer(t)
	t.Cleanup(srv.Close)
	client, err := nvidia.NewClient(srv.URL, nvidia.WithClient(srv.Client()))
	require.NoError(t, err)
	RegisterNvidia(server, client)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		serverSession.Close()
	})

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := mcpClient.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		clientSession.Close()
	})
	return clientSession, serverSession
}

func TestRegisterNvidia(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client, err := nvidia.NewClient("https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite")
	require.NoError(t, err)
	RegisterNvidia(server, client)

	assertTools(t, server, "nvidia_search_jobs", "nvidia_get_job_detail")
}

func TestNvidiaSearchJobsE2E(t *testing.T) {
	clientSession, _ := testNvidiaMCPClientServer(t)

	res, err := clientSession.ListTools(t.Context(), nil)
	require.NoError(t, err)

	tool := findTool(res.Tools, "nvidia_search_jobs")
	require.NotNil(t, tool)

	schema, ok := tool.InputSchema.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "object", schema["type"])

	// Test calling nvidia_search_jobs tool
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "nvidia_search_jobs",
		Arguments: map[string]any{
			"keyword":       "golang",
			"job_category":  "Engineering",
			"job_type":      "Regular Employee",
			"time_type":     "Full time",
			"location_type": "Remote",
			"country":       "Taiwan",
			"site":          "Taiwan, Taipei",
			"limit":         5,
			"offset":        0,
		},
	})
	require.NoError(t, err)
	assert.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var output nvidiaSearchOutput
	err = json.Unmarshal(data, &output)
	require.NoError(t, err)

	assert.Equal(t, 27, output.Total)
	require.NotEmpty(t, output.Data)
	assert.Equal(t, "Senior Software Golang Kubernetes Engineer", output.Data[0].Title)
	assert.Equal(t, "/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916", output.Data[0].ExternalPath)
}

func TestNvidiaGetJobDetailE2E(t *testing.T) {
	clientSession, _ := testNvidiaMCPClientServer(t)

	res, err := clientSession.ListTools(t.Context(), nil)
	require.NoError(t, err)

	tool := findTool(res.Tools, "nvidia_get_job_detail")
	require.NotNil(t, tool)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "nvidia_get_job_detail",
		Arguments: map[string]any{
			"external_path": "/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916",
		},
	})
	require.NoError(t, err)
	assert.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var output nvidiaDetailOutput
	err = json.Unmarshal(data, &output)
	require.NoError(t, err)

	assert.Equal(t, "Senior Software Golang Kubernetes Engineer", output.Title)
	assert.Contains(t, output.Description, "NVIDIA Networking is looking for an excellent Software Developer")
	assert.Equal(t, "Israel, Yokneam", output.Location)
	assert.Equal(t, []string{"Israel, Raanana", "Israel, Tel Aviv"}, output.AdditionalLocations)
	assert.Equal(t, "JR2015916", output.JobReqID)
	assert.Equal(t, "https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916", output.ExternalURL)
}
