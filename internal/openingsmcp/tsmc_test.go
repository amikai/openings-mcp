package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTsmcMCPClientServer(t *testing.T) (*mcp.ClientSession, *mcp.ServerSession) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := tsmc.NewMockServer()
	t.Cleanup(srv.Close)
	client := tsmc.NewClient(srv.URL, srv.Client())
	RegisterTsmc(server, client)

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

func TestRegisterTsmc(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client := tsmc.NewClient("https://careers.tsmc.com", nil)
	RegisterTsmc(server, client)

	assertTools(t, server, "tsmc_search_jobs", "tsmc_get_job_detail")
}

func TestTsmcSearchJobsE2E(t *testing.T) {
	clientSession, _ := testTsmcMCPClientServer(t)

	res, err := clientSession.ListTools(t.Context(), nil)
	require.NoError(t, err)

	tool := findTool(res.Tools, "tsmc_search_jobs")
	require.NotNil(t, tool)

	schema, ok := tool.InputSchema.(map[string]any)
	require.True(t, ok)

	// Full golden schema: label enums instead of the site's numeric
	// form-field IDs, keyword and location required.
	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword": map[string]any{
				"type":        "string",
				"description": "Free-text keyword search across job titles.",
			},
			"location": map[string]any{
				"type":        "string",
				"description": "Location filter.",
				"enum": []any{
					"Taiwan", "Canada", "China", "Germany-Dresden", "Germany-Munich",
					"Japan-Yokohama", "Japan-Osaka", "Japan-Tsukuba", "Japan-Kumamoto",
					"Korea", "Netherlands", "USA-Arizona", "USA-California",
					"USA-Massachusetts", "USA-Texas", "USA-Washington", "USA-Washington, D.C.",
				},
			},
			"category": map[string]any{
				"type":        "string",
				"description": "Job category filter.",
				"enum": []any{
					"R&D", "Specialty Technology", "IC Design Technology", "Manufacturing (fabs)",
					"Facility & Industrial Safety / Environmental Protection", "Product Development",
					"R&D Advanced Packaging Technology Development", "Testing Development and Technology",
					"Quality and Reliability", "Information Technology", "Internal Audit",
					"Business Development", "Customer Service", "Corporate Planning",
					"Finance / Accounting / Risk Management", "Human Resources", "Legal",
					"Materials Management", "Corporate Sustainability (ESG)", "Administration",
					"Accessibility Inclusion",
				},
			},
			"job_type": map[string]any{
				"type":        "string",
				"description": "Job level filter.",
				"enum": []any{
					"Technician", "Associate Engineer / Admin", "Engineer / Admin",
					"Manager / Executive", "Others",
				},
			},
			"employment_type": map[string]any{
				"type":        "string",
				"description": "Employment type filter.",
				"enum":        []any{"Regular", "Temporary", "Intern", "Apprenticeship"},
			},
			"page": map[string]any{
				"type":        "integer",
				"description": "1-based page number; 10 results per page.",
				"minimum":     float64(1),
			},
		},
		"required":             []any{"keyword", "location"},
		"additionalProperties": false,
	}
	assert.Equal(t, want, schema)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "tsmc_search_jobs",
		Arguments: map[string]any{"keyword": "engineer", "location": "Taiwan"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got tsmcSearchOutput
	require.NoError(t, json.Unmarshal(data, &got))

	wantResp := &tsmcSearchOutput{
		Total: 22,
		Data: []tsmcJobSummary{
			{
				ID:             "21826",
				URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/R-D-Advanced-Packaging-Integration-Engineer/21826",
				Title:          "R&D Advanced Packaging Integration Engineer",
				Location:       "台灣",
				CareerArea:     "研究發展",
				EmploymentType: "正職",
				Posted:         "2026/05/28",
			},
			{
				ID:             "19509",
				URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/TSMC-R-D-Process-Engineer-FLM-Forward-Looking-Module/19509",
				Title:          "TSMC R&D Process Engineer / FLM (Forward Looking Module)",
				Location:       "台灣",
				CareerArea:     "研究發展",
				EmploymentType: "正職",
				Posted:         "2026/03/17",
			},
			{
				ID:             "302",
				URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/A10-RD-Device-Engineer/302",
				Title:          "A10 RD Device Engineer",
				Location:       "台灣",
				CareerArea:     "研究發展",
				EmploymentType: "正職",
				Posted:         "2026/03/17",
			},
			{
				ID:             "5354",
				URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/A14-R-D-Device-Engineer/5354",
				Title:          "A14 R&D Device Engineer",
				Location:       "台灣",
				CareerArea:     "研究發展",
				EmploymentType: "正職",
				Posted:         "2026/03/17",
			},
			{
				ID:             "353",
				URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/A10-A14-RD-Integration-Engineer/353",
				Title:          "A10/A14 RD Integration Engineer",
				Location:       "台灣",
				CareerArea:     "研究發展",
				EmploymentType: "正職",
				Posted:         "2026/03/17",
			},
			{
				ID:             "6152",
				URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/Research-Pathfinding-Engineer/6152",
				Title:          "Research & Pathfinding Engineer",
				Location:       "台灣",
				CareerArea:     "研究發展",
				EmploymentType: "正職",
				Posted:         "2026/03/17",
			},
			{
				ID:             "5351",
				URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/A14-R-D-SRAM-Engineer/5351",
				Title:          "A14 R&D SRAM Engineer",
				Location:       "台灣",
				CareerArea:     "研究發展",
				EmploymentType: "正職",
				Posted:         "2026/03/17",
			},
			{
				ID:             "5353",
				URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/A14-R-D-Integration-Engineer/5353",
				Title:          "A14 R&D Integration Engineer",
				Location:       "台灣",
				CareerArea:     "研究發展",
				EmploymentType: "正職",
				Posted:         "2026/03/17",
			},
			{
				ID:             "16359",
				URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/Research-and-Development-Engineer-R-D/16359",
				Title:          "Research and Development Engineer (R&D)",
				Location:       "台灣",
				CareerArea:     "研究發展",
				EmploymentType: "正職",
				Posted:         "2026/03/17",
			},
			{
				ID:             "6154",
				URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/R-D-Module-Engineer/6154",
				Title:          "R&D Module Engineer",
				Location:       "台灣",
				CareerArea:     "研究發展",
				EmploymentType: "正職",
				Posted:         "2026/03/17",
			},
		},
	}
	assert.Equal(t, wantResp, &got)
}

func TestTsmcSearchJobsMissingRequiredE2E(t *testing.T) {
	clientSession, _ := testTsmcMCPClientServer(t)

	// Missing required params are rejected by the SDK's input-schema
	// validation before the handler runs, as an IsError tool result.
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"no keyword", map[string]any{"location": "Taiwan"}, `validating "arguments": validating root: required: missing properties: ["keyword"]`},
		{"no location", map[string]any{"keyword": "engineer"}, `validating "arguments": validating root: required: missing properties: ["location"]`},
		{"empty args", map[string]any{}, `validating "arguments": validating root: required: missing properties: ["keyword" "location"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
				Name:      "tsmc_search_jobs",
				Arguments: tc.args,
			})
			require.NoError(t, err)
			require.True(t, callRes.IsError)
			text, ok := callRes.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.Equal(t, tc.want, text.Text)
		})
	}
}

func TestTsmcSearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession, _ := testTsmcMCPClientServer(t)

	// A value outside a property's enum is rejected by the SDK's
	// input-schema validation before the handler runs, as an IsError
	// tool result.
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "tsmc_search_jobs",
		Arguments: map[string]any{"keyword": "engineer", "location": "Taiwan", "employment_type": "valueNotInEnum"},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, `validating "arguments": validating root: validating /properties/employment_type: enum: valueNotInEnum does not equal any of: [Regular Temporary Intern Apprenticeship]`, text.Text)
}

func TestTsmcGetJobDetailE2E(t *testing.T) {
	clientSession, _ := testTsmcMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "tsmc_get_job_detail",
		Arguments: map[string]any{"job_id": "21826"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got tsmcDetailOutput
	require.NoError(t, json.Unmarshal(data, &got))

	want := &tsmcDetailOutput{
		ID:             "21826",
		URL:            "https://careers.tsmc.com/zh_TW/careers/JobDetail/R-D-Advanced-Packaging-Integration-Engineer/21826",
		Title:          "R&D Advanced Packaging Integration Engineer",
		Company:        "台灣積體電路製造股份有限公司",
		Location:       "台灣",
		CareerArea:     "研究發展",
		JobType:        "工程師/管理師",
		EmploymentType: "正職",
		Posted:         "2026/05/28",
		Responsibilities: `1. Lead the development and integration of novel advanced packaging technologies, processes, and materials.
2. Design, execute, and analyze experiments to optimize packaging performance, reliability, and cost.
3. Collaborate cross-functionally with design, module, and operations teams to ensure seamless integration and manufacturability.
4. Identify and troubleshoot complex technical challenges in packaging integration, driving root cause analysis and solutions.
5. Contribute to the strategic roadmap for advanced packaging, evaluating new technologies and intellectual property.`,
		Qualifications: `1. MS or Ph.D. in Electrical, Mechanical, Chemical, Materials Science, Physics, or a related engineering/science discipline.
2. Possesses strong technical problem-solving and analytical skills, grounded in fundamental principles, with a proactive, hands-on approach and a strong sense of ownership; consistently demonstrates a growth mindset for continuous learning and development.
3. Exceptional team player who fosters trust, drives innovation with disciplined execution, and is fluent in both Mandarin and English with excellent cross-functional communication skills.`,
	}
	assert.Equal(t, want, &got)
}

func TestTsmcHTTPToMCPResponse(t *testing.T) {
	in := tsmc.JobsResponse{
		Total: 2,
		Jobs: []tsmc.Job{
			{ID: "1", Slug: "Some-Engineer", Title: "t1", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/01/01"},
			{ID: "2", Title: "t2"},
		},
	}
	got := tsmcHTTPToMCPResponse(&in)

	want := &tsmcSearchOutput{
		Total: 2,
		Data: []tsmcJobSummary{
			{ID: "1", URL: "https://careers.tsmc.com/zh_TW/careers/JobDetail/Some-Engineer/1", Title: "t1", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/01/01"},
			{ID: "2", Title: "t2"},
		},
	}
	assert.Equal(t, want, got)
}

func TestTsmcHTTPToMCPDetail(t *testing.T) {
	in := tsmc.JobDetailResponse{
		ID:               "7",
		Slug:             "Some-Engineer",
		Title:            "t",
		Company:          "c",
		Location:         "台灣",
		CareerArea:       "研究發展",
		JobType:          "工程師/管理師",
		EmploymentType:   "正職",
		Posted:           "2026/01/01",
		Responsibilities: "r",
		Qualifications:   "q",
	}
	got := tsmcHTTPToMCPDetail(&in)

	want := &tsmcDetailOutput{
		ID:               "7",
		URL:              "https://careers.tsmc.com/zh_TW/careers/JobDetail/Some-Engineer/7",
		Title:            "t",
		Company:          "c",
		Location:         "台灣",
		CareerArea:       "研究發展",
		JobType:          "工程師/管理師",
		EmploymentType:   "正職",
		Posted:           "2026/01/01",
		Responsibilities: "r",
		Qualifications:   "q",
	}
	assert.Equal(t, want, got)
}

func TestTsmcMCPToHTTPRequest(t *testing.T) {
	in := tsmcSearchInput{
		Keyword:        "engineer",
		Location:       "Taiwan",
		Category:       "R&D",
		JobType:        "Engineer / Admin",
		EmploymentType: "Regular",
		Page:           2,
	}
	got, err := tsmcMCPToHTTPRequest(&in)
	require.NoError(t, err)

	want := &tsmc.JobsRequest{
		Keyword:         "engineer",
		Locations:       []string{"13209"},
		Categories:      []string{"38617"},
		JobTypes:        []string{"5709"},
		EmploymentTypes: []string{"5701"},
		Page:            2,
	}
	assert.Equal(t, want, got)
}

func TestTsmcMCPToHTTPRequestMinimal(t *testing.T) {
	got, err := tsmcMCPToHTTPRequest(&tsmcSearchInput{Keyword: "engineer", Location: "Japan-Kumamoto"})
	require.NoError(t, err)

	want := tsmc.JobsRequest{
		Keyword:   "engineer",
		Locations: []string{"13217"},
	}
	assert.Equal(t, want, *got)
}

func TestTsmcMCPToHTTPRequestMissingRequired(t *testing.T) {
	cases := []struct {
		name string
		in   tsmcSearchInput
		want string
	}{
		{"all empty", tsmcSearchInput{}, "keyword is required"},
		{"filters only", tsmcSearchInput{Location: "Taiwan", Category: "R&D", Page: 2}, "keyword is required"},
		{"keyword only", tsmcSearchInput{Keyword: "engineer"}, "location is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tsmcMCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestTsmcMCPToHTTPRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   tsmcSearchInput
		want string
	}{
		{"location", tsmcSearchInput{Keyword: "x", Location: "Tainan"}, `invalid location "Tainan"`},
		{"category", tsmcSearchInput{Keyword: "x", Location: "Taiwan", Category: "HR"}, `invalid category "HR"`},
		{"job_type", tsmcSearchInput{Keyword: "x", Location: "Taiwan", JobType: "Boss"}, `invalid job_type "Boss"`},
		{"employment_type", tsmcSearchInput{Keyword: "x", Location: "Taiwan", EmploymentType: "Contract"}, `invalid employment_type "Contract"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tsmcMCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
