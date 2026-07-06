package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/linkedin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkedinMCPToHTTPRequest(t *testing.T) {
	in := linkedinSearchInput{
		Keyword:       "software engineer",
		Location:      "Taiwan",
		WorkplaceType: "Remote",
		JobType:       "Full-time",
		CompanyIDs:    []string{"1441", "162479"},
		PostedWithin:  "Past week",
		Start:         10,
	}
	got, err := linkedinMCPToHTTPRequest(&in)
	require.NoError(t, err)

	want := &linkedin.JobsRequest{
		Keywords:            "software engineer",
		Location:            "Taiwan",
		WorkplaceType:       linkedin.WorkplaceRemote,
		JobType:             linkedin.JobTypeFullTime,
		CompanyIDs:          []string{"1441", "162479"},
		PostedWithinSeconds: 604800,
		Start:               10,
	}
	assert.Equal(t, want, got)
}

func TestLinkedinMCPToHTTPRequestMinimal(t *testing.T) {
	got, err := linkedinMCPToHTTPRequest(&linkedinSearchInput{})
	require.NoError(t, err)
	assert.Equal(t, &linkedin.JobsRequest{}, got)
}

func TestLinkedinMCPToHTTPRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   linkedinSearchInput
		want string
	}{
		{"workplace_type", linkedinSearchInput{WorkplaceType: "Space"}, `invalid workplace_type "Space"`},
		{"job_type", linkedinSearchInput{JobType: "Volunteer"}, `invalid job_type "Volunteer"`},
		{"posted_within", linkedinSearchInput{PostedWithin: "Past year"}, `invalid posted_within "Past year"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := linkedinMCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestLinkedinHTTPToMCPResponse(t *testing.T) {
	in := linkedin.JobsResponse{
		Jobs: []linkedin.Job{
			{ID: "1", Title: "t1", Company: "c1", CompanyURL: "cu1", Location: "l1", PostedDate: "d1", Remote: true},
			{ID: "2", Title: "t2"},
		},
	}
	got := linkedinHTTPToMCPResponse(&in)

	want := &linkedinSearchOutput{
		Data: []linkedinJobSummary{
			{ID: "1", Title: "t1", Company: "c1", CompanyURL: "cu1", Location: "l1", PostedDate: "d1", Remote: true, URL: "https://www.linkedin.com/jobs/view/1"},
			{ID: "2", Title: "t2", URL: "https://www.linkedin.com/jobs/view/2"},
		},
	}
	assert.Equal(t, want, got)
}

func TestLinkedinHTTPToMCPDetail(t *testing.T) {
	in := linkedin.JobDetailResponse{
		ID:             "7",
		Title:          "t",
		Company:        "c",
		Location:       "l",
		Posted:         "p",
		SeniorityLevel: "sl",
		EmploymentType: "et",
		JobFunction:    "jf",
		Industries:     "ind",
		Description:    "desc",
		CompanyLogo:    "logo-url",
		ApplyURL:       "apply-url",
		Remote:         true,
	}
	got := linkedinHTTPToMCPDetail(&in)

	// CompanyLogo has no corresponding output field: it's intentionally
	// dropped, so it must not appear anywhere in want.
	want := &linkedinDetailOutput{
		ID:             "7",
		URL:            "https://www.linkedin.com/jobs/view/7",
		Title:          "t",
		Company:        "c",
		Location:       "l",
		Posted:         "p",
		SeniorityLevel: "sl",
		EmploymentType: "et",
		JobFunction:    "jf",
		Industries:     "ind",
		Description:    "desc",
		ApplyURL:       "apply-url",
		Remote:         true,
	}
	assert.Equal(t, want, got)
}

func testLinkedinMCPClientServer(t *testing.T) (*mcp.ClientSession, *mcp.ServerSession) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := linkedin.NewMockServer()
	t.Cleanup(srv.Close)
	client := linkedin.NewClient(srv.URL, srv.Client())
	RegisterLinkedin(server, client)

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

func TestRegisterLinkedin(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client := linkedin.NewClient("https://www.linkedin.com", nil)
	RegisterLinkedin(server, client)

	assertTools(t, server, "linkedin_search_jobs", "linkedin_get_job_detail")
}

func TestLinkedinSearchJobsE2E(t *testing.T) {
	clientSession, _ := testLinkedinMCPClientServer(t)

	res, err := clientSession.ListTools(t.Context(), nil)
	require.NoError(t, err)

	tool := findTool(res.Tools, "linkedin_search_jobs")
	require.NotNil(t, tool)

	schema, ok := tool.InputSchema.(map[string]any)
	require.True(t, ok)

	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword": map[string]any{
				"type": "string",
			},
			"location": map[string]any{
				"type": "string",
			},
			"workplace_type": map[string]any{
				"type":        "string",
				"description": "Workplace type filter.",
				"enum":        []any{"On-site", "Remote", "Hybrid"},
			},
			"job_type": map[string]any{
				"type":        "string",
				"description": "Job type filter.",
				"enum":        []any{"Full-time", "Part-time", "Contract", "Temporary", "Internship"},
			},
			"company_ids": map[string]any{
				"type":        "array",
				"description": "LinkedIn numeric company IDs. IDs are opaque and must be resolved from a company's public page or a prior search response, not guessed.",
				"items": map[string]any{
					"type": "string",
				},
			},
			"posted_within": map[string]any{
				"type":        "string",
				"description": "Only jobs posted within this window.",
				"enum":        []any{"Past day", "Past week", "Past month"},
			},
			"start": map[string]any{
				"type":        "integer",
				"description": "Zero-based result offset. Each call returns exactly 10 results; increment by 10 each page (0, 10, 20, ...).",
				"minimum":     float64(0),
			},
		},
		"additionalProperties": false,
	}
	assert.Equal(t, want, schema)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "linkedin_search_jobs",
		Arguments: map[string]any{
			"keyword":        "software engineer",
			"location":       "Taiwan",
			"workplace_type": "Remote",
			"job_type":       "Full-time",
			"posted_within":  "Past week",
			"start":          0,
		},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got linkedinSearchOutput
	require.NoError(t, json.Unmarshal(data, &got))

	want2 := &linkedinSearchOutput{
		Data: []linkedinJobSummary{
			{ID: "4422697744", Title: "Software Engineer", Company: "BoostDraft", CompanyURL: "https://www.linkedin.com/company/boostdraft", Location: "Taiwan", PostedDate: "2026-06-03", URL: "https://www.linkedin.com/jobs/view/4422697744"},
			{ID: "4430577683", Title: "Software Engineer, Apps, Pixel", Company: "Google", CompanyURL: "https://www.linkedin.com/company/google", Location: "Banqiao District, New Taipei City, Taiwan", PostedDate: "2026-06-22", URL: "https://www.linkedin.com/jobs/view/4430577683"},
			{ID: "4435540496", Title: "Software Engineer", Company: "Mphasis", CompanyURL: "https://in.linkedin.com/company/mphasis", Location: "Taipei, Taipei City, Taiwan", PostedDate: "2026-07-01", URL: "https://www.linkedin.com/jobs/view/4435540496"},
			{ID: "4409906484", Title: "Software Engineer (Taipei)", Company: "Nitra", CompanyURL: "https://www.linkedin.com/company/nitrahq", Location: "Taipei, Taipei City, Taiwan", PostedDate: "2026-05-07", URL: "https://www.linkedin.com/jobs/view/4409906484"},
			{ID: "4430941394", Title: "(f2pool) Software Engineer - Back-end / Full-stack", Company: "stakefish", CompanyURL: "https://vg.linkedin.com/company/stakefish", Location: "Taiwan", PostedDate: "2026-05-25", URL: "https://www.linkedin.com/jobs/view/4430941394"},
			{ID: "4435420998", Title: "Full Stack Software Engineer", Company: "MediaTek", CompanyURL: "https://tw.linkedin.com/company/mediatek", Location: "Hsinchu, Taiwan, Taiwan", PostedDate: "2026-07-03", URL: "https://www.linkedin.com/jobs/view/4435420998"},
			{ID: "4425114186", Title: "Software Engineer - Dajia, Taichung City, Taiwan", Company: "Winbro", CompanyURL: "https://www.linkedin.com/company/winbro", Location: "Dajia District, Taichung City, Taiwan", PostedDate: "2026-05-12", URL: "https://www.linkedin.com/jobs/view/4425114186"},
			{ID: "4435546265", Title: "Software Engineer", Company: "Mphasis", CompanyURL: "https://in.linkedin.com/company/mphasis", Location: "Taipei, Taipei City, Taiwan", PostedDate: "2026-07-01", URL: "https://www.linkedin.com/jobs/view/4435546265"},
			{ID: "4435541354", Title: "Software Engineer", Company: "Mphasis", CompanyURL: "https://in.linkedin.com/company/mphasis", Location: "Taipei, Taipei City, Taiwan", PostedDate: "2026-07-01", URL: "https://www.linkedin.com/jobs/view/4435541354"},
			{ID: "4401701902", Title: "Senior Software Engineer / Lead Software Engineer", Company: "BoostDraft", CompanyURL: "https://www.linkedin.com/company/boostdraft", Location: "Taiwan", PostedDate: "2026-04-14", URL: "https://www.linkedin.com/jobs/view/4401701902"},
		},
	}
	assert.Equal(t, want2, &got)
}

func TestLinkedinSearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession, _ := testLinkedinMCPClientServer(t)

	// A value outside a property's enum is rejected by the SDK's
	// input-schema validation before the handler runs, as an IsError tool
	// result.
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "linkedin_search_jobs",
		Arguments: map[string]any{"workplace_type": "valueNotInEnum"},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, `validating /properties/workplace_type: enum`)
}

func TestLinkedinGetJobDetailE2E(t *testing.T) {
	clientSession, _ := testLinkedinMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "linkedin_get_job_detail",
		Arguments: map[string]any{"job_id": "4422697744"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got linkedinDetailOutput
	require.NoError(t, json.Unmarshal(data, &got))

	// The fixture's description is a very large bilingual blob; asserted via
	// assert.Contains below rather than pinned verbatim, same as
	// internal/provider/linkedin/parse_test.go's TestParseDetailHTML.
	description := got.Description
	got.Description = ""

	want := linkedinDetailOutput{
		ID:             "4422697744",
		URL:            "https://www.linkedin.com/jobs/view/4422697744",
		Title:          "Software Engineer",
		Company:        "BoostDraft",
		Location:       "Taiwan",
		Posted:         "1 month ago",
		SeniorityLevel: "Entry level",
		EmploymentType: "Full-time",
		JobFunction:    "Other",
		Industries:     "IT Services and IT Consulting",
	}
	assert.Equal(t, want, got)
	assert.Contains(t, description, "BoostDraft is a software engineering company")
	assert.Contains(t, description, "Fluent in coding with C#")
}
