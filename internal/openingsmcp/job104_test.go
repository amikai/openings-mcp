package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/job104"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findTool(tools []*mcp.Tool, toolName string) *mcp.Tool {
	for _, tool := range tools {
		if tool.Name == toolName {
			return tool
		}
	}
	return nil
}

func testJob104MCPClientServer(t *testing.T) (*mcp.ClientSession, *mcp.ServerSession) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := job104.NewMockServer()
	t.Cleanup(srv.Close)
	client, err := job104.NewClient(srv.URL, job104.WithClient(srv.Client()))
	require.NoError(t, err)
	RegisterJob104(server, client)

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

func TestRegisterJob104(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client, err := job104.NewClient("https://www.104.com.tw")
	require.NoError(t, err)
	RegisterJob104(server, client)

	assertTools(t, server, "104_search_jobs", "104_get_job_detail")
}

func TestJob104SearchJobE2E(t *testing.T) {
	clientSession, _ := testJob104MCPClientServer(t)

	res, err := clientSession.ListTools(t.Context(), nil)
	require.NoError(t, err)

	tool := findTool(res.Tools, "104_search_jobs")
	require.NotNil(t, tool)

	schema, ok := tool.InputSchema.(map[string]any)
	require.True(t, ok)

	// Full golden schema: LLM-facing names only (no ro/order/remoteWork/s9),
	// label enums instead of raw codes, keyword and area required.
	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword": map[string]any{
				"type":        "string",
				"description": "Free-text keyword search.",
			},
			"area": map[string]any{
				"type":        "string",
				"description": "City/region filter.",
				"enum": []any{
					"Taipei", "NewTaipei", "Yilan", "Keelung", "Taoyuan",
					"Hsinchu", "Miaoli", "Taichung", "Changhua", "Nantou",
					"Yunlin", "Chiayi", "Tainan", "Kaohsiung", "Pingtung",
					"Taitung", "Hualien", "Penghu", "Kinmen", "Lienchiang",
					"Beijing", "Tianjin", "Shanghai", "Chongqing", "Guangdong",
					"Fujian", "Hainan", "Zhejiang", "Jiangsu", "Shandong",
					"Hebei", "Liaoning", "Jilin", "Heilongjiang", "Hunan",
					"Hubei", "Jiangxi", "Anhui", "Henan", "Shanxi",
					"Shaanxi", "Gansu", "Qinghai", "Sichuan", "Guizhou",
					"Yunnan", "InnerMongolia", "Tibet", "Ningxia", "Xinjiang",
					"Guangxi", "HongKong", "Macao",
					"NortheastAsia", "SoutheastAsia", "OtherAsia",
					"AustraliaNZ", "OtherOceania",
					"Canada", "EasternUS", "WesternUS", "MidwesternUS",
					"CentralAmerica", "SouthAmerica",
					"NorthernEurope", "SouthernEurope", "EasternEurope",
					"WesternEurope", "CentralEurope",
					"NorthAfrica", "CentralAfrica", "SouthAfrica",
					"EastAfrica", "WestAfrica",
				},
			},
			"job_type": map[string]any{
				"type":        "string",
				"description": "Employment basis. Soft filter — verify each result's job_type.",
				"enum":        []any{"Full-time", "Part-time", "Senior", "Dispatch"},
			},
			"sort": map[string]any{
				"type":        "string",
				"description": "Result order.",
				"enum":        []any{"Relevance", "Newest"},
			},
			"remote": map[string]any{
				"type":        "string",
				"description": "Remote work. Soft filter — verify each result's remote. Omit for on-site.",
				"enum":        []any{"Full", "Partial"},
			},
			"edu": map[string]any{
				"type":        "array",
				"description": "Education levels, OR'd together.",
				"uniqueItems": true,
				"items": map[string]any{
					"type": "string",
					"enum": []any{"HighSchoolBelow", "HighSchool", "College", "University", "Master", "Doctorate"},
				},
			},
			"page": map[string]any{
				"type":        "integer",
				"description": "1-based page number.",
				"minimum":     float64(1),
			},
		},
		"required":             []any{"keyword", "area"},
		"additionalProperties": false,
	}
	assert.Equal(t, want, schema)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "104_search_jobs",
		Arguments: map[string]any{"keyword": "Golang", "area": "Taipei"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got job104SearchOutput
	require.NoError(t, json.Unmarshal(data, &got))

	wantResp := &job104SearchOutput{
		Data: []job104JobSummary{
			{JobCode: "624o1", JobName: "GoLang Developer", CompanyName: "曜驊智能股份有限公司", URL: "https://www.104.com.tw/job/624o1", CompanyURL: "https://www.104.com.tw/company/1a2x6biwgs", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市內湖區", AppearDate: "20260515", ApplyCnt: 3, JobType: "Full-time"},
			{JobCode: "8xtv5", JobName: "Golang 後端工程師", CompanyName: "富一代資訊有限公司", URL: "https://www.104.com.tw/job/8xtv5", CompanyURL: "https://www.104.com.tw/company/1a2x6bnn4e", SalaryHigh: 120000, SalaryLow: 60000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260609", ApplyCnt: 8, JobType: "Full-time"},
			{JobCode: "6ptna", JobName: "Golang 工程師", CompanyName: "百阜科技股份有限公司", URL: "https://www.104.com.tw/job/6ptna", CompanyURL: "https://www.104.com.tw/company/1a2x6bkdrx", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市內湖區", AppearDate: "20260526", ApplyCnt: 4, JobType: "Full-time"},
			{JobCode: "7jzf9", JobName: "Senior Cloud Backend Engineer (Golang)", CompanyName: "華玉科技股份有限公司", URL: "https://www.104.com.tw/job/7jzf9", CompanyURL: "https://www.104.com.tw/company/1a2x6bluto", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 6, JobType: "Full-time"},
			{JobCode: "8hwa1", JobName: "軟體工程師 Golang", CompanyName: "線上探索科技股份有限公司", URL: "https://www.104.com.tw/job/8hwa1", CompanyURL: "https://www.104.com.tw/company/1a2x6bl53p", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大同區", AppearDate: "20260304", ApplyCnt: 6, JobType: "Full-time"},
			{JobCode: "90xm2", JobName: "Software Engineer (Golang, Flutter), Virtual insurance", CompanyName: "香港商六度科技有限公司", URL: "https://www.104.com.tw/job/90xm2", CompanyURL: "https://www.104.com.tw/company/1a2x6blfqs", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市信義區", AppearDate: "20260618", ApplyCnt: 5, Remote: "Partial", JobType: "Full-time"},
			{JobCode: "7x6op", JobName: "Golang開發工程師", CompanyName: "太禾科技有限公司", URL: "https://www.104.com.tw/job/7x6op", CompanyURL: "https://www.104.com.tw/company/1a2x6bls9x", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260618", ApplyCnt: 5, JobType: "Full-time"},
			{JobCode: "8wj0l", JobName: "Golang 後端工程師 / Golang Backend Engineer", CompanyName: "炫石有限公司", URL: "https://www.104.com.tw/job/8wj0l", CompanyURL: "https://www.104.com.tw/company/1a2x6bn5h3", SalaryHigh: 9999999, SalaryLow: 60000, JobAddrNoDesc: "台北市信義區", AppearDate: "20260511", ApplyCnt: 8, JobType: "Full-time"},
			{JobCode: "8v7ta", JobName: "Golang開發工程師", CompanyName: "四天科技有限公司", URL: "https://www.104.com.tw/job/8v7ta", CompanyURL: "https://www.104.com.tw/company/1a2x6bmxsm", SalaryHigh: 150000, SalaryLow: 80000, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 8, JobType: "Full-time"},
			{JobCode: "8wi35", JobName: "【擴編】資深Golang後端工程師 / Senior Golang Developer", CompanyName: "瑞典商英鉑科股份有限公司台灣分公司", URL: "https://www.104.com.tw/job/8wi35", CompanyURL: "https://www.104.com.tw/company/1a2x6bmnic", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 4, JobType: "Full-time"},
			{JobCode: "8zz6y", JobName: "軟體工程師 (Software Engineer - Golang)", CompanyName: "立視科技股份有限公司", URL: "https://www.104.com.tw/job/8zz6y", CompanyURL: "https://www.104.com.tw/company/1a2x6bnpb0", SalaryHigh: 88000, SalaryLow: 55000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260625", ApplyCnt: 4, JobType: "Full-time"},
			{JobCode: "8lhs9", JobName: "GOLANG 開發工程師", CompanyName: "益晨資訊科技有限公司", URL: "https://www.104.com.tw/job/8lhs9", CompanyURL: "https://www.104.com.tw/company/1a2x6bmpzr", SalaryHigh: 90000, SalaryLow: 72000, JobAddrNoDesc: "台北市中正區", AppearDate: "20260625", ApplyCnt: 7, JobType: "Full-time"},
			{JobCode: "86yd2", JobName: "Senior Backend Engineer ( Golang )（每月有遠端日）", CompanyName: "幣託科技股份有限公司", URL: "https://www.104.com.tw/job/86yd2", CompanyURL: "https://www.104.com.tw/company/1a2x6bmrpo", SalaryHigh: 150000, SalaryLow: 85000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260622", ApplyCnt: 10, Remote: "Partial", JobType: "Full-time"},
			{JobCode: "8zlcq", JobName: "後端工程師（Golang）", CompanyName: "米奈娛樂有限公司", URL: "https://www.104.com.tw/job/8zlcq", CompanyURL: "https://www.104.com.tw/company/1a2x6bnd8p", SalaryHigh: 80000, SalaryLow: 70000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260620", ApplyCnt: 7, JobType: "Full-time"},
			{JobCode: "8j944", JobName: "Golang後端與DevOps工程師", CompanyName: "時刻無限股份有限公司", URL: "https://www.104.com.tw/job/8j944", CompanyURL: "https://www.104.com.tw/company/1a2x6bn6jz", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260622", ApplyCnt: 8, JobType: "Full-time"},
			{JobCode: "8q81k", JobName: "Golang 遊戲開發工程師(大安)", CompanyName: "天晴資訊有限公司", URL: "https://www.104.com.tw/job/8q81k", CompanyURL: "https://www.104.com.tw/company/1a2x6blkl5", SalaryHigh: 95000, SalaryLow: 50000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 7, JobType: "Full-time"},
			{JobCode: "92ref", JobName: "Golang Engineer", CompanyName: "瞬聯科技股份有限公司", URL: "https://www.104.com.tw/job/92ref", CompanyURL: "https://www.104.com.tw/company/1a2x6ble2t", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 20, JobType: "Full-time"},
			{JobCode: "872ja", JobName: "【純遠端】國際遊戲公司 誠徵  Go/Golang 工程師", CompanyName: "台灣英特艾倫人力資源有限公司", URL: "https://www.104.com.tw/job/872ja", CompanyURL: "https://www.104.com.tw/company/1a2x6bj0ov", SalaryHigh: 180000, SalaryLow: 150000, JobAddrNoDesc: "台北市中山區", AppearDate: "20260623", ApplyCnt: 12, Remote: "Full", JobType: "Full-time"},
			{JobCode: "8pwoi", JobName: "Golang 後端工程師(大安)", CompanyName: "天晴資訊有限公司", URL: "https://www.104.com.tw/job/8pwoi", CompanyURL: "https://www.104.com.tw/company/1a2x6blkl5", SalaryHigh: 9999999, SalaryLow: 50000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 5, JobType: "Full-time"},
			{JobCode: "8yfo6", JobName: "【TENG0502】Software Engineer (Backend) - Golang / RoR", CompanyName: "喬富科技股份有限公司", URL: "https://www.104.com.tw/job/8yfo6", CompanyURL: "https://www.104.com.tw/company/1a2x6bnnpl", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市松山區", AppearDate: "20260623", ApplyCnt: 10, JobType: "Full-time"},
			{JobCode: "8nbkk", JobName: "Golang工程師-Junior", CompanyName: "彼雅特科技股份有限公司", URL: "https://www.104.com.tw/job/8nbkk", CompanyURL: "https://www.104.com.tw/company/1a2x6bmpg9", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市信義區", AppearDate: "20260529", ApplyCnt: 6, JobType: "Full-time"},
			{JobCode: "8t645", JobName: "[資訊部]Golang工程師", CompanyName: "虹耀建設股份有限公司", URL: "https://www.104.com.tw/job/8t645", CompanyURL: "https://www.104.com.tw/company/1a2x6bl3dj", SalaryHigh: 9999999, SalaryLow: 75000, JobAddrNoDesc: "台北市中正區", AppearDate: "20260622", ApplyCnt: 6, JobType: "Full-time"},
			{JobCode: "8wrsr", JobName: "資深後端工程師（Golang / Java） / Senior Backend Engineer（Golang / Java）", CompanyName: "炫石有限公司", URL: "https://www.104.com.tw/job/8wrsr", CompanyURL: "https://www.104.com.tw/company/1a2x6bn5h3", SalaryHigh: 9999999, SalaryLow: 60000, JobAddrNoDesc: "台北市信義區", AppearDate: "20260511", ApplyCnt: 2, JobType: "Full-time"},
			{JobCode: "8w445", JobName: "【擴編】Golang後端工程師/ Golang Developer", CompanyName: "瑞典商英鉑科股份有限公司台灣分公司", URL: "https://www.104.com.tw/job/8w445", CompanyURL: "https://www.104.com.tw/company/1a2x6bmnic", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 9, JobType: "Full-time"},
			{JobCode: "8ktbq", JobName: "後端工程師-Golang-台北", CompanyName: "立特有限公司", URL: "https://www.104.com.tw/job/8ktbq", CompanyURL: "https://www.104.com.tw/company/1a2x6bmi9f", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 10, JobType: "Full-time"},
			{JobCode: "8zsac", JobName: "Senior Backend Engineer (Golang), Virtual insurance", CompanyName: "香港商六度科技有限公司", URL: "https://www.104.com.tw/job/8zsac", CompanyURL: "https://www.104.com.tw/company/1a2x6blfqs", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市信義區", AppearDate: "20260622", ApplyCnt: 12, Remote: "Partial", JobType: "Full-time"},
			{JobCode: "90hu0", JobName: "Golang 後端工程師", CompanyName: "昕展資訊有限公司", URL: "https://www.104.com.tw/job/90hu0", CompanyURL: "https://www.104.com.tw/company/1a2x6bnktm", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260623", ApplyCnt: 10, JobType: "Full-time"},
			{JobCode: "8wcx6", JobName: "後端工程師 (Backend Engineer - Golang)", CompanyName: "開端智能股份有限公司", URL: "https://www.104.com.tw/job/8wcx6", CompanyURL: "https://www.104.com.tw/company/1a2x6bngab", SalaryHigh: 80000, SalaryLow: 50000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260626", ApplyCnt: 17, JobType: "Full-time"},
			{JobCode: "8a024", JobName: "Backend Engineer(Java or Golang)", CompanyName: "重高科技股份有限公司", URL: "https://www.104.com.tw/job/8a024", CompanyURL: "https://www.104.com.tw/company/1a2x6bmusr", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260622", ApplyCnt: 17, JobType: "Full-time"},
			{JobCode: "8ejup", JobName: "Golang 網站開發工程師(Backend)_零售解決方案課", CompanyName: "日本NEC集團_統智科技股份有限公司", URL: "https://www.104.com.tw/job/8ejup", CompanyURL: "https://www.104.com.tw/company/5wy72fk", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市內湖區", AppearDate: "20260612", ApplyCnt: 19, JobType: "Full-time"},
		},
		Metadata: job104SearchMetadata{
			Pagination: job104Pagination{CurrentPage: 1, LastPage: 7, Total: 189},
		},
	}
	assert.Equal(t, wantResp, &got)
}

func TestJob104SearchJobsCompanyKeywordE2E(t *testing.T) {
	clientSession, _ := testJob104MCPClientServer(t)

	// A keyword 104 recognizes as a company name flips the API into a
	// pagination-less companyKeyword response unless the handler sends
	// excludeCompanyKeyword=true; the mock reproduces that behavior, so
	// this call only succeeds when the parameter is actually on the wire.
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "104_search_jobs",
		Arguments: map[string]any{"keyword": job104.MockCompanyKeyword, "area": "Hsinchu"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got job104SearchOutput
	require.NoError(t, json.Unmarshal(data, &got))

	assert.NotEmpty(t, got.Data)
	assert.Equal(t, job104Pagination{CurrentPage: 1, LastPage: 7, Total: 189}, got.Metadata.Pagination)
}

func TestJob104SearchJobsMissingRequiredE2E(t *testing.T) {
	clientSession, _ := testJob104MCPClientServer(t)

	// Missing required params are rejected by the SDK's input-schema
	// validation before the handler runs, as an IsError tool result.
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"no keyword", map[string]any{"area": "Taipei"}, `validating "arguments": validating root: required: missing properties: ["keyword"]`},
		{"no area", map[string]any{"keyword": "Golang"}, `validating "arguments": validating root: required: missing properties: ["area"]`},
		{"empty args", map[string]any{}, `validating "arguments": validating root: required: missing properties: ["keyword" "area"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
				Name:      "104_search_jobs",
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

func TestJob104SearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession, _ := testJob104MCPClientServer(t)

	// A value outside a property's enum is rejected by the SDK's
	// input-schema validation before the handler runs, as an IsError
	// tool result.
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "104_search_jobs",
		Arguments: map[string]any{"keyword": "Golang", "area": "Taipei", "job_type": "valueNotInEnum"},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, `validating "arguments": validating root: validating /properties/job_type: enum: valueNotInEnum does not equal any of: [Full-time Part-time Senior Dispatch]`, text.Text)
}

func TestJob104GetJobDetailE2E(t *testing.T) {
	clientSession, _ := testJob104MCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "104_get_job_detail",
		Arguments: map[string]any{"job_code": "624o1"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got job104DetailOutput
	require.NoError(t, json.Unmarshal(data, &got))

	want := &job104DetailOutput{
		JobName:        "軟體工程師 (數位工程發展部)",
		CompanyName:    "亞新工程顧問股份有限公司",
		URL:            "https://www.104.com.tw/job/624o1",
		CompanyURL:     "https://www.104.com.tw/company/264c9zc",
		AppearDate:     "2026/06/22",
		JobDescription: "無相關經驗可，大學以上資訊工程、資訊管理等相關科系畢業\n\n【工作內容】\n- 參與智慧工程數位平台的設計、開發與維運\n- 開發與維護 GIS、BIM 系統，並支援無人機地形數據應用\n- 參與 AI 工具與文件管理系統之開發 \n- 與跨領域團隊合作（工程、IoT、BIM、AI），推動數位轉型與自動化流程\n\n【希望條件】\n- 熟悉現代軟體系統研發流程與版本控制\n- 熟悉至少一種指令式程式設計語言（C#、JavaScript、Python、PHP 尤佳）\n- 具 ASP.NET、SQL、Vue.js、Laravel、Unity、GIS、IoT、Revit等開發經驗\n- 具軟體設計、開發、運營、開發、機器學習、AI 模型訓練 (Finetuning)、 AI 應用設計（OCR、RAG、LLM、Agentic 等）開發、導入經驗\n- 具 Azure DevOps、Docker、Kubernetes 經驗者優先\n\n＊我們期待具備高度邏輯思維、善於溝通系統需求與設計選擇，並能獨立完成軟體開發的夥伴加入，一起參與系統規劃與優化。",
		JobCategory:    []string{"軟體工程師"},
		Salary:         "待遇面議",
		JobType:        "Full-time",
		AddressRegion:  "新北市汐止區",
		AddressDetail:  "新台五路一段112號22樓",
		WorkExp:        "不拘",
		Edu:            "大學以上",
		Major:          []string{"資訊工程相關"},
		Specialty:      []string{"C#", "ASP.NET", "MS SQL", "Python", "GIS", "IoT", "Revit"},
		ManageResp:     "不需負擔管理責任",
		NeedEmp:        "2~3人",
		Welfare:        "在亞新，我們重視同仁的職涯成長與友善職場，透過全方位的福利與支持，推動以人為本、永續發展的職場環境，實現工作與生活的和諧平衡。\n\n【薪酬與獎金】\n  •  具市場競爭力的薪資水準\n  •  年節獎金與專案獎金，共享成果回饋\n\n【健康與保障】\n  •  勞健保及完整團體保險(意外、醫療、重大疾病、職災保障)\n  •  定期健康檢查、健康講座與員工關懷方案\n\n【休假與彈性】\n  •  彈性上下班、育兒友善措施，兼顧生活平衡\n\n【教育訓練與發展】\n  •  完善新人培訓與師徒制\n  •  E-learning 線上學習資源\n  •  專業證照補助（如 PMP、專業技師等）\n  •  外部訓練與國際研討會，拓展國際視野\n  •  參與國家級重大工程，累積獨特專業經驗\n\n【生活與休閒】\n  •  福委會關懷：生日禮金、節慶禮品或禮券、婚喪喜慶、傷病住院慰問與生育補助\n  •  部門聚餐、咖啡分享日、社團活動、Happy Hour，促進交流與凝聚力\n  •  舒適職場環境：明亮開放空間、零食吧、茶包與自助研磨咖啡機\n\n【招募流程】\n  1. 投遞履歷\n  2. HR初審履歷 → 部門主管面試\n  3. Final面談（含專案介紹與Q&A）\n  4. 錄取通知\n （流程清楚透明，讓你安心應徵!)",
		Industry:       "建築及工程技術服務業",
		Employees:      "1200人",
	}
	assert.Equal(t, want, &got)
}

func TestJob104SearchJobsUpstreamErrorE2E(t *testing.T) {
	clientSession, _ := testJob104MCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "104_search_jobs",
		Arguments: map[string]any{"keyword": job104.MockErrorKeyword, "area": "Taipei"},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "upstream error: 500", text.Text)
}

func TestJob104GetJobDetailUpstreamErrorE2E(t *testing.T) {
	clientSession, _ := testJob104MCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "104_get_job_detail",
		Arguments: map[string]any{"job_code": job104.MockNotFoundJobCode},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "upstream error: 404", text.Text)
}

func TestJob104HTTPToMCPResponse(t *testing.T) {
	in := job104.JobsResponse{
		Data: []job104.JobSummary{
			{JobNo: "1", JobName: "onsite", CustName: "c1", CustNo: "n1", Link: job104.JobSummaryLink{Job: "j1", Cust: "u1"}, SalaryHigh: 2, SalaryLow: 1, JobAddrNoDesc: "a1", AppearDate: "20260101", ApplyCnt: 3, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "2", JobName: "full-remote", CustName: "c2", CustNo: "n2", Link: job104.JobSummaryLink{Job: "j2", Cust: "u2"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "a2", AppearDate: "20260102", ApplyCnt: 4, RemoteWorkType: 1, JobRo: 2},
			{JobNo: "3", JobName: "hybrid", CustName: "c3", CustNo: "n3", Link: job104.JobSummaryLink{Job: "j3", Cust: "u3"}, SalaryHigh: 9, SalaryLow: 5, JobAddrNoDesc: "a3", AppearDate: "20260103", ApplyCnt: 5, RemoteWorkType: 2, JobRo: 4},
			{JobNo: "4", JobName: "unknown-codes", CustName: "c4", CustNo: "n4", Link: job104.JobSummaryLink{Job: "j4", Cust: "u4"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "a4", AppearDate: "20260104", ApplyCnt: 6, RemoteWorkType: 9, JobRo: 9},
		},
		Metadata: job104.JobsResponseMetadata{
			Pagination: job104.JobsResponseMetadataPagination{CurrentPage: 1, LastPage: 2, Total: 34},
		},
	}
	got := job104HTTPToMCPResponse(&in)

	// Unknown codes (jobRo 9, remoteWorkType 9) map to no label at all.
	want := &job104SearchOutput{
		Data: []job104JobSummary{
			{JobCode: "j1", JobName: "onsite", CompanyName: "c1", URL: "j1", CompanyURL: "u1", SalaryHigh: 2, SalaryLow: 1, JobAddrNoDesc: "a1", AppearDate: "20260101", ApplyCnt: 3, JobType: "Full-time"},
			{JobCode: "j2", JobName: "full-remote", CompanyName: "c2", URL: "j2", CompanyURL: "u2", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "a2", AppearDate: "20260102", ApplyCnt: 4, Remote: "Full", JobType: "Part-time"},
			{JobCode: "j3", JobName: "hybrid", CompanyName: "c3", URL: "j3", CompanyURL: "u3", SalaryHigh: 9, SalaryLow: 5, JobAddrNoDesc: "a3", AppearDate: "20260103", ApplyCnt: 5, Remote: "Partial", JobType: "Dispatch"},
			{JobCode: "j4", JobName: "unknown-codes", CompanyName: "c4", URL: "j4", CompanyURL: "u4", SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "a4", AppearDate: "20260104", ApplyCnt: 6},
		},
		Metadata: job104SearchMetadata{
			Pagination: job104Pagination{CurrentPage: 1, LastPage: 2, Total: 34},
		},
	}
	assert.Equal(t, want, got)
}

func TestJob104HTTPToMCPDetail(t *testing.T) {
	in := job104.JobDetailResponse{
		Data: job104.JobDetail{
			Header: job104.JobDetailHeader{JobName: "j", CustName: "c", CustUrl: "u", AppearDate: "2026/01/01", IsSaved: true, IsApplied: false},
			Contact: job104.JobDetailContact{
				HrName: job104.NewOptString("hr"),
				Email:  job104.NewOptString("e@x"),
				Reply:  job104.NewOptString(""),
			},
			Condition: job104.JobDetailCondition{
				WorkExp: job104.NewOptString("exp"),
				Edu:     job104.NewOptString("edu"),
				Major:   []string{"m1"},
				Specialty: []job104.CodeDescription{
					{Code: job104.NewOptString("s1"), Description: job104.NewOptString("d1")},
				},
			},
			Welfare: job104.JobDetailWelfare{Welfare: job104.NewOptString("w")},
			JobDetail: job104.JobDetailJobDetail{
				JobDescription: job104.NewOptString("desc"),
				JobCategory: []job104.CodeDescription{
					{Code: job104.NewOptString("k1"), Description: job104.NewOptString("kd1")},
				},
				Salary:        job104.NewOptString("sal"),
				SalaryMin:     job104.NewOptInt(10),
				SalaryMax:     job104.NewOptInt(20),
				JobType:       job104.NewOptInt(1),
				AddressRegion: job104.NewOptString("region"),
				AddressDetail: job104.NewOptString("detail"),
				ManageResp:    job104.NewOptString("mr"),
				NeedEmp:       job104.NewOptString("ne"),
				RemoteWork: job104.OptNilJobDetailJobDetailRemoteWork{
					Set:   true,
					Value: job104.JobDetailJobDetailRemoteWork{Type: job104.NewOptInt(1), Description: job104.NewOptString("遠端")},
				},
			},
			Industry:  "ind",
			Employees: "9人",
			CustNo:    "cn",
		},
	}
	got := job104HTTPToMCPDetail(&in, "jc1")

	// isSaved/isApplied/custNo/contact are dropped, specialty/jobCategory
	// keep only descriptions, and everything else flattens to one level.
	want := &job104DetailOutput{
		JobName:        "j",
		CompanyName:    "c",
		URL:            "https://www.104.com.tw/job/jc1",
		CompanyURL:     "u",
		AppearDate:     "2026/01/01",
		JobDescription: "desc",
		JobCategory:    []string{"kd1"},
		Salary:         "sal",
		SalaryMin:      10,
		SalaryMax:      20,
		JobType:        "Full-time",
		Remote:         "Full",
		AddressRegion:  "region",
		AddressDetail:  "detail",
		WorkExp:        "exp",
		Edu:            "edu",
		Major:          []string{"m1"},
		Specialty:      []string{"d1"},
		ManageResp:     "mr",
		NeedEmp:        "ne",
		Welfare:        "w",
		Industry:       "ind",
		Employees:      "9人",
	}
	assert.Equal(t, want, got)
}

func TestJob104HTTPToMCPDetailNullRemoteUnknownJobType(t *testing.T) {
	in := job104.JobDetailResponse{
		Data: job104.JobDetail{
			Header: job104.JobDetailHeader{JobName: "j", CustName: "c", CustUrl: "u", AppearDate: "2026/01/01"},
			JobDetail: job104.JobDetailJobDetail{
				JobType:    job104.NewOptInt(9),
				RemoteWork: job104.OptNilJobDetailJobDetailRemoteWork{Set: true, Null: true},
			},
			Industry:  "ind",
			Employees: "9人",
			CustNo:    "cn",
		},
	}
	got := job104HTTPToMCPDetail(&in, "jc1")

	// Null remoteWork and unknown jobType code both drop their labels.
	want := &job104DetailOutput{
		JobName:     "j",
		CompanyName: "c",
		URL:         "https://www.104.com.tw/job/jc1",
		CompanyURL:  "u",
		AppearDate:  "2026/01/01",
		Industry:    "ind",
		Employees:   "9人",
	}
	assert.Equal(t, want, got)
}

func TestJob104MCPToHTTPRequest(t *testing.T) {
	in := job104SearchInput{
		Keyword: "golang",
		Area:    "Taipei",
		JobType: "Part-time",
		Sort:    "Newest",
		Remote:  "Full",
		Edu:     []string{"University", "Master"},
		Page:    2,
	}
	got, err := job104MCPToHTTPRequest(&in)
	require.NoError(t, err)

	want := &job104.SearchJobsParams{
		Keyword:               job104.NewOptString("golang"),
		ExcludeCompanyKeyword: job104.NewOptBool(true),
		Area:                  job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
		Ro:                    job104.NewOptSearchJobsRo(job104.SearchJobsRo2),
		Order:                 job104.NewOptSearchJobsOrder(job104.SearchJobsOrder2),
		RemoteWork:            job104.NewOptSearchJobsRemoteWork(job104.SearchJobsRemoteWork1),
		Page:                  job104.NewOptInt(2),
		Edu:                   []job104.SearchJobsEduItem{job104.SearchJobsEduItem4, job104.SearchJobsEduItem5},
	}
	assert.Equal(t, want, got)
}

func TestJob104MCPToHTTPRequestMissingRequired(t *testing.T) {
	cases := []struct {
		name string
		in   job104SearchInput
		want string
	}{
		{"all empty", job104SearchInput{}, "keyword is required"},
		{"filters only", job104SearchInput{Area: "Taipei", Sort: "Newest", Page: 2}, "keyword is required"},
		{"keyword only", job104SearchInput{Keyword: "golang"}, `invalid area ""`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := job104MCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestJob104MCPToHTTPRequestMinimal(t *testing.T) {
	got, err := job104MCPToHTTPRequest(&job104SearchInput{Keyword: "golang", Area: "Taipei"})
	require.NoError(t, err)
	want := job104.SearchJobsParams{
		Keyword:               job104.NewOptString("golang"),
		ExcludeCompanyKeyword: job104.NewOptBool(true),
		Area:                  job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
	}
	assert.Equal(t, want, *got)
}

func TestJob104MCPToHTTPRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   job104SearchInput
		want string
	}{
		{"area", job104SearchInput{Keyword: "x", Area: "Mars"}, `invalid area "Mars"`},
		{"job_type", job104SearchInput{Keyword: "x", Area: "Taipei", JobType: "full"}, `invalid job_type "full"`},
		{"sort", job104SearchInput{Keyword: "x", Area: "Taipei", Sort: "newest"}, `invalid sort "newest"`},
		{"remote", job104SearchInput{Keyword: "x", Area: "Taipei", Remote: "hybrid"}, `invalid remote "hybrid"`},
		{"edu", job104SearchInput{Keyword: "x", Area: "Taipei", Edu: []string{"University", "PhD"}}, `invalid edu "PhD"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := job104MCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
