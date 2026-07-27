package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/flowxtra"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFlowxtraMCPClientServer(t *testing.T) (*mcp.ClientSession, *mcp.ServerSession) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := flowxtra.NewMockServer()
	t.Cleanup(srv.Close)
	client, err := flowxtra.NewClient(srv.URL, flowxtra.WithClient(srv.Client()))
	require.NoError(t, err)
	RegisterFlowxtra(server, client)

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

func TestRegisterFlowxtra(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client, err := flowxtra.NewClient("https://app.flowxtra.com/api")
	require.NoError(t, err)
	RegisterFlowxtra(server, client)

	assertTools(t, server, "flowxtra_search_jobs", "flowxtra_get_job_detail")
}

func TestFlowxtraSearchJobsE2E(t *testing.T) {
	clientSession, _ := testFlowxtraMCPClientServer(t)

	// A bare call (no arguments) lists the newest postings board-wide.
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "flowxtra_search_jobs",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	assert.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var output flowxtraSearchOutput
	require.NoError(t, json.Unmarshal(data, &output))

	want := flowxtraSearchOutput{
		Total:    116,
		Page:     1,
		LastPage: 39,
		Data: []flowxtraJobSummary{
			{
				Title:     "Operario/a de envasado",
				Company:   "Arogreen",
				Location:  "Barcelona, Spain",
				Workplace: "On-site",
				Salary:    "EUR 21000/year",
				PostedAt:  "2026-07-23",
				JobID:     "M88PB",
				URL:       "https://flowxtra.com/apply/M88PB",
			},
			{
				Title:     "sales",
				Company:   "3S Spring",
				Location:  "Bizerte, Tunisia",
				Workplace: "Remote",
				Salary:    "EUR 100/monthly",
				PostedAt:  "2026-07-21",
				JobID:     "L99OW",
				URL:       "https://flowxtra.com/apply/L99OW",
			},
			{
				Title:     "warehouse assistant",
				Company:   "joseph logistics",
				Location:  "Alberta, Canada",
				Workplace: "On-site",
				PostedAt:  "2026-07-19",
				JobID:     "KrrNz",
				URL:       "https://flowxtra.com/apply/KrrNz",
			},
		},
	}
	assert.Equal(t, want, output)
}

func TestFlowxtraSearchJobsFilteredE2E(t *testing.T) {
	clientSession, _ := testFlowxtraMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "flowxtra_search_jobs",
		Arguments: map[string]any{
			"query":     "sales",
			"workplace": "Remote",
			"page":      1,
		},
	})
	require.NoError(t, err)
	assert.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var output flowxtraSearchOutput
	require.NoError(t, json.Unmarshal(data, &output))

	// The mock serves the captured filtered fixture whenever a
	// search-key is present (the real API narrows server-side).
	assert.Equal(t, 4, output.Total)
	require.NotEmpty(t, output.Data)
	assert.Equal(t, "sales", output.Data[0].Title)
	assert.Equal(t, "3S Spring", output.Data[0].Company)
	assert.Equal(t, "Remote", output.Data[0].Workplace)
}

func TestFlowxtraGetJobDetailE2E(t *testing.T) {
	clientSession, _ := testFlowxtraMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "flowxtra_get_job_detail",
		Arguments: map[string]any{
			"job_id": "M88PB",
		},
	})
	require.NoError(t, err)
	assert.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var output flowxtraDetailOutput
	require.NoError(t, json.Unmarshal(data, &output))

	assert.Equal(t, "Operario/a de envasado", output.Title)
	assert.Equal(t, "Arogreen", output.Company)
	assert.Equal(t, "https://www.arogreen.com", output.CompanyWebsite)
	assert.Equal(t, "Barcelona, Spain", output.Location)
	assert.Equal(t, "On-site", output.Workplace)
	assert.Equal(t, "Midlevel", output.Seniority)
	assert.Equal(t, []string{"Full-time"}, output.EmploymentTypes)
	assert.Equal(t, "EUR 21000/year", output.Salary)
	// Description is html2text-rendered plain text, not HTML.
	assert.Contains(t, output.Description, "Arogreen")
	assert.NotContains(t, output.Description, "<p>")
	// Detail exposes the company's own career-page apply URL.
	assert.Equal(t, "https://arogreen.postular.link/job/operarioa-de-envasado/M88PB", output.ApplyURL)
}

func TestFlowxtraGetJobDetailNotFoundE2E(t *testing.T) {
	clientSession, _ := testFlowxtraMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "flowxtra_get_job_detail",
		Arguments: map[string]any{
			"job_id": "ZZZZZ99",
		},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "not found")
}

func TestFlowxtraSearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession, _ := testFlowxtraMCPClientServer(t)

	// A value outside the workplace enum is rejected by the SDK's
	// input-schema validation before the handler runs.
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "flowxtra_search_jobs",
		Arguments: map[string]any{
			"workplace": "valueNotInEnum",
		},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, `validating /properties/workplace: enum`)
}

func TestFlowxtraSalary(t *testing.T) {
	null := flowxtra.NilFloat64{Null: true}
	tests := []struct {
		name             string
		min, max, salary flowxtra.NilFloat64
		currency, rate   string
		want             string
	}{
		{name: "min only", min: flowxtra.NewNilFloat64(21000), max: null, salary: null, currency: "EUR", rate: "year", want: "EUR 21000/year"},
		{name: "range", min: flowxtra.NewNilFloat64(100000), max: flowxtra.NewNilFloat64(300000), salary: null, currency: "USD", rate: "year", want: "USD 100000-300000/year"},
		{name: "fixed salary wins", min: flowxtra.NewNilFloat64(1), max: null, salary: flowxtra.NewNilFloat64(500), currency: "EUR", rate: "month", want: "EUR 500/month"},
		{name: "max only", min: null, max: flowxtra.NewNilFloat64(90), salary: null, currency: "EUR", rate: "hour", want: "EUR up to 90/hour"},
		{name: "unset", min: null, max: null, salary: null, currency: "EUR", rate: "monthly", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, flowxtraSalary(tt.currency, tt.min, tt.max, tt.salary, tt.rate))
		})
	}
}
