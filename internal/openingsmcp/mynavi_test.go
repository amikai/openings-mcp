package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/mynavi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMynaviMCPToHTTPRequest(t *testing.T) {
	got := mynaviMCPToHTTPRequest(&mynaviSearchInput{
		Keyword:   "Python 機械学習",
		MinSalary: 700,
		Page:      2,
	})
	assert.Equal(t, &mynavi.JobsRequest{
		Keywords:  "Python 機械学習",
		MinSalary: 700,
		Page:      2,
	}, got)

	assert.Equal(t, &mynavi.JobsRequest{}, mynaviMCPToHTTPRequest(&mynaviSearchInput{}))
}

func TestMynaviHTTPToMCPResponse(t *testing.T) {
	in := mynavi.JobsResponse{
		Total: 2,
		Jobs: []mynavi.Job{
			{
				ID: "1-2-3-4", Title: "t1", Company: "c1", CatchCopy: "cc1",
				EmploymentStatus: "正社員", Conditions: []string{"転勤なし"},
				Description: "d1", Target: "tg1", Location: "l1", Salary: "s1",
				FirstYearIncome: "400万円～", UpdatedDate: "2026/07/15", EndDate: "2026/07/30",
			},
			{ID: "5-6-7-8", Title: "t2"},
		},
	}
	got := mynaviHTTPToMCPResponse(&in)

	want := &mynaviSearchOutput{
		Total: 2,
		Data: []mynaviJobSummary{
			{
				ID: "1-2-3-4", Title: "t1", Company: "c1", CatchCopy: "cc1",
				EmploymentStatus: "正社員", Conditions: []string{"転勤なし"},
				Description: "d1", Target: "tg1", Location: "l1", Salary: "s1",
				FirstYearIncome: "400万円～", UpdatedDate: "2026/07/15", EndDate: "2026/07/30",
				URL: "https://tenshoku.mynavi.jp/jobinfo-1-2-3-4/",
			},
			{ID: "5-6-7-8", Title: "t2", URL: "https://tenshoku.mynavi.jp/jobinfo-5-6-7-8/"},
		},
	}
	assert.Equal(t, want, got)
}

func TestMynaviHTTPToMCPDetail(t *testing.T) {
	in := mynavi.JobDetailResponse{
		ID:                     "1-2-3-4",
		URL:                    "https://tenshoku.mynavi.jp/jobinfo-1-2-3-4/",
		Title:                  "t",
		Company:                "c",
		CompanyURL:             "cu",
		EmploymentType:         "FULL_TIME",
		Industry:               "ind",
		OccupationalCategory:   "occ",
		DatePosted:             "2026-07-03",
		ValidThrough:           "2026-07-30",
		Locations:              []mynavi.Location{{Region: "東京都", Locality: "渋谷区"}, {Region: "大阪府"}},
		SalaryCurrency:         "JPY",
		SalaryMin:              "4200000",
		SalaryMax:              "10000000",
		SalaryUnit:             "YEAR",
		Description:            "desc",
		ExperienceRequirements: "req",
		WorkHours:              "wh",
		JobBenefits:            "jb",
	}
	got := mynaviHTTPToMCPDetail(&in)

	want := &mynaviDetailOutput{
		ID:                     "1-2-3-4",
		URL:                    "https://tenshoku.mynavi.jp/jobinfo-1-2-3-4/",
		Title:                  "t",
		Company:                "c",
		CompanyURL:             "cu",
		EmploymentType:         "FULL_TIME",
		Industry:               "ind",
		Occupation:             "occ",
		DatePosted:             "2026-07-03",
		ValidThrough:           "2026-07-30",
		Locations:              []mynaviLocation{{Region: "東京都", Locality: "渋谷区"}, {Region: "大阪府"}},
		SalaryCurrency:         "JPY",
		SalaryMin:              "4200000",
		SalaryMax:              "10000000",
		SalaryUnit:             "YEAR",
		Description:            "desc",
		ExperienceRequirements: "req",
		WorkHours:              "wh",
		JobBenefits:            "jb",
	}
	assert.Equal(t, want, got)
}

func testMynaviMCPClientServer(t *testing.T) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := mynavi.NewMockServer()
	t.Cleanup(srv.Close)
	client := mynavi.NewClient(srv.URL, srv.Client())
	RegisterMynavi(server, client)

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
	return clientSession
}

func TestRegisterMynavi(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client := mynavi.NewClient("https://tenshoku.mynavi.jp", nil)
	RegisterMynavi(server, client)

	assertTools(t, server, "mynavi_search_jobs", "mynavi_get_job_detail")
}

func TestMynaviSearchJobsE2E(t *testing.T) {
	clientSession := testMynaviMCPClientServer(t)

	res, err := clientSession.ListTools(t.Context(), nil)
	require.NoError(t, err)

	tool := findTool(res.Tools, "mynavi_search_jobs")
	require.NotNil(t, tool)

	schema, ok := tool.InputSchema.(map[string]any)
	require.True(t, ok)

	var wantSchema map[string]any
	require.NoError(t, json.Unmarshal(mynaviSearchInputRawSchema, &wantSchema))
	assert.Equal(t, wantSchema, schema)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "mynavi_search_jobs",
		Arguments: map[string]any{
			"keyword": "Python",
		},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got mynaviSearchOutput
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, 2111, got.Total)
	require.Len(t, got.Data, 50)
	first := got.Data[0]
	assert.Equal(t, "348855-1-29-1", first.ID)
	assert.Equal(t, "【ITエンジニア】還元率80%超！フルリモOK☆初年度年収420万円～", first.Title)
	assert.Equal(t, "ウィンヴォルブ株式会社", first.Company)
	assert.Equal(t, "正社員", first.EmploymentStatus)
	assert.Equal(t, "420万円～1000万円", first.FirstYearIncome)
	assert.Equal(t, "https://tenshoku.mynavi.jp/jobinfo-348855-1-29-1/", first.URL)
}

func TestMynaviSearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession := testMynaviMCPClientServer(t)

	// A min_salary outside the enum is rejected by the SDK's input-schema
	// validation before the handler runs, as an IsError tool result.
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "mynavi_search_jobs",
		Arguments: map[string]any{"min_salary": 720},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, `validating /properties/min_salary: enum`)
}

func TestMynaviGetJobDetailE2E(t *testing.T) {
	clientSession := testMynaviMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "mynavi_get_job_detail",
		Arguments: map[string]any{"job_id": "348855-1-29-1"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got mynaviDetailOutput
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, "348855-1-29-1", got.ID)
	assert.Equal(t, "ITエンジニア／システムエンジニア（アプリ設計／WEB・オープン・モバイル系）", got.Title)
	assert.Equal(t, "ウィンヴォルブ株式会社", got.Company)
	assert.Equal(t, "FULL_TIME", got.EmploymentType)
	assert.Equal(t, "2026-07-30", got.ValidThrough)
	assert.Len(t, got.Locations, 47)
	assert.Equal(t, mynaviLocation{Region: "北海道", Locality: "札幌市"}, got.Locations[0])
	assert.Equal(t, "4200000", got.SalaryMin)
	assert.Contains(t, got.Description, "この求人のポイント")
}

func TestMynaviGetJobDetailNotFoundE2E(t *testing.T) {
	clientSession := testMynaviMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "mynavi_get_job_detail",
		Arguments: map[string]any{"job_id": mynavi.MockNotFoundJobID},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "404")
}
