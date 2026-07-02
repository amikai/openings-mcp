package jobmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/job104"
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

func testClientServer(t *testing.T) (*mcp.ClientSession, *mcp.ServerSession) {
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
	clientSession, _ := testClientServer(t)

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
				"description": "Employment basis. Soft filter — verify each result's jobRo.",
				"enum":        []any{"Full-time", "Part-time", "Senior", "Dispatch"},
			},
			"sort": map[string]any{
				"type":        "string",
				"description": "Result order.",
				"enum":        []any{"Relevance", "Newest"},
			},
			"remote": map[string]any{
				"type":        "string",
				"description": "Remote work. Soft filter — verify each result's remoteWorkType. Omit for on-site.",
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
	var got job104.JobsResponse
	require.NoError(t, json.Unmarshal(data, &got))

	wantResp := &job104.JobsResponse{
		Data: []job104.JobSummary{
			{JobNo: "10177057", JobName: "GoLang Developer", CustName: "曜驊智能股份有限公司", CustNo: "130000000042972", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/624o1", Cust: "https://www.104.com.tw/company/1a2x6biwgs"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市內湖區", AppearDate: "20260515", ApplyCnt: 3, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "15015281", JobName: "Golang 後端工程師", CustName: "富一代資訊有限公司", CustNo: "130000000264142", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8xtv5", Cust: "https://www.104.com.tw/company/1a2x6bnn4e"}, SalaryHigh: 120000, SalaryLow: 60000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260609", ApplyCnt: 8, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "11282518", JobName: "Golang 工程師", CustName: "百阜科技股份有限公司", CustNo: "130000000112061", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/6ptna", Cust: "https://www.104.com.tw/company/1a2x6bkdrx"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市內湖區", AppearDate: "20260526", ApplyCnt: 4, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "12689685", JobName: "Senior Cloud Backend Engineer (Golang)", CustName: "華玉科技股份有限公司", CustNo: "130000000180812", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/7jzf9", Cust: "https://www.104.com.tw/company/1a2x6bluto"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 6, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14271913", JobName: "軟體工程師 Golang", CustName: "線上探索科技股份有限公司", CustNo: "130000000147477", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8hwa1", Cust: "https://www.104.com.tw/company/1a2x6bl53p"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大同區", AppearDate: "20260304", ApplyCnt: 6, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "15160106", JobName: "Software Engineer (Golang, Flutter), Virtual insurance", CustName: "香港商六度科技有限公司", CustNo: "130000000161268", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/90xm2", Cust: "https://www.104.com.tw/company/1a2x6blfqs"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市信義區", AppearDate: "20260618", ApplyCnt: 5, RemoteWorkType: 2, JobRo: 1},
			{JobNo: "13305625", JobName: "Golang開發工程師", CustName: "太禾科技有限公司", CustNo: "130000000177509", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/7x6op", Cust: "https://www.104.com.tw/company/1a2x6bls9x"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260618", ApplyCnt: 5, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14954565", JobName: "Golang 後端工程師 / Golang Backend Engineer", CustName: "炫石有限公司", CustNo: "130000000241271", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8wj0l", Cust: "https://www.104.com.tw/company/1a2x6bn5h3"}, SalaryHigh: 9999999, SalaryLow: 60000, JobAddrNoDesc: "台北市信義區", AppearDate: "20260511", ApplyCnt: 8, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14893390", JobName: "Golang開發工程師", CustName: "四天科技有限公司", CustNo: "130000000231318", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8v7ta", Cust: "https://www.104.com.tw/company/1a2x6bmxsm"}, SalaryHigh: 150000, SalaryLow: 80000, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 8, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14953361", JobName: "【擴編】資深Golang後端工程師 / Senior Golang Developer", CustName: "瑞典商英鉑科股份有限公司台灣分公司", CustNo: "130000000217988", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8wi35", Cust: "https://www.104.com.tw/company/1a2x6bmnic"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 4, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "15115498", JobName: "軟體工程師 (Software Engineer - Golang)", CustName: "立視科技股份有限公司", CustNo: "130000000266972", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8zz6y", Cust: "https://www.104.com.tw/company/1a2x6bnpb0"}, SalaryHigh: 88000, SalaryLow: 55000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260625", ApplyCnt: 4, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14439753", JobName: "GOLANG 開發工程師", CustName: "益晨資訊科技有限公司", CustNo: "130000000221207", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8lhs9", Cust: "https://www.104.com.tw/company/1a2x6bmpzr"}, SalaryHigh: 90000, SalaryLow: 72000, JobAddrNoDesc: "台北市中正區", AppearDate: "20260625", ApplyCnt: 7, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "13761398", JobName: "Senior Backend Engineer ( Golang )（每月有遠端日）", CustName: "幣託科技股份有限公司", CustNo: "130000000223436", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/86yd2", Cust: "https://www.104.com.tw/company/1a2x6bmrpo"}, SalaryHigh: 150000, SalaryLow: 85000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260622", ApplyCnt: 10, RemoteWorkType: 2, JobRo: 1},
			{JobNo: "15097562", JobName: "後端工程師（Golang）", CustName: "米奈娛樂有限公司", CustNo: "130000000251337", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8zlcq", Cust: "https://www.104.com.tw/company/1a2x6bnd8p"}, SalaryHigh: 80000, SalaryLow: 70000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260620", ApplyCnt: 7, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14335204", JobName: "Golang後端與DevOps工程師", CustName: "時刻無限股份有限公司", CustNo: "130000000242671", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8j944", Cust: "https://www.104.com.tw/company/1a2x6bn6jz"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260622", ApplyCnt: 8, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14660408", JobName: "Golang 遊戲開發工程師(大安)", CustName: "天晴資訊有限公司", CustNo: "130000000167545", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8q81k", Cust: "https://www.104.com.tw/company/1a2x6blkl5"}, SalaryHigh: 95000, SalaryLow: 50000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 7, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "15245367", JobName: "Golang Engineer", CustName: "瞬聯科技股份有限公司", CustNo: "130000000159109", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/92ref", Cust: "https://www.104.com.tw/company/1a2x6ble2t"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 20, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "13766806", JobName: "【純遠端】國際遊戲公司 誠徵  Go/Golang 工程師", CustName: "台灣英特艾倫人力資源有限公司", CustNo: "130000000048447", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/872ja", Cust: "https://www.104.com.tw/company/1a2x6bj0ov"}, SalaryHigh: 180000, SalaryLow: 150000, JobAddrNoDesc: "台北市中山區", AppearDate: "20260623", ApplyCnt: 12, RemoteWorkType: 1, JobRo: 1},
			{JobNo: "14645682", JobName: "Golang 後端工程師(大安)", CustName: "天晴資訊有限公司", CustNo: "130000000167545", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8pwoi", Cust: "https://www.104.com.tw/company/1a2x6blkl5"}, SalaryHigh: 9999999, SalaryLow: 50000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 5, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "15043542", JobName: "【TENG0502】Software Engineer (Backend) - Golang / RoR", CustName: "喬富科技股份有限公司", CustNo: "130000000264905", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8yfo6", Cust: "https://www.104.com.tw/company/1a2x6bnnpl"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市松山區", AppearDate: "20260623", ApplyCnt: 10, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14525012", JobName: "Golang工程師-Junior", CustName: "彼雅特科技股份有限公司", CustNo: "130000000220505", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8nbkk", Cust: "https://www.104.com.tw/company/1a2x6bmpg9"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市信義區", AppearDate: "20260529", ApplyCnt: 6, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14797877", JobName: "[資訊部]Golang工程師", CustName: "虹耀建設股份有限公司", CustNo: "130000000145239", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8t645", Cust: "https://www.104.com.tw/company/1a2x6bl3dj"}, SalaryHigh: 9999999, SalaryLow: 75000, JobAddrNoDesc: "台北市中正區", AppearDate: "20260622", ApplyCnt: 6, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14965947", JobName: "資深後端工程師（Golang / Java） / Senior Backend Engineer（Golang / Java）", CustName: "炫石有限公司", CustNo: "130000000241271", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8wrsr", Cust: "https://www.104.com.tw/company/1a2x6bn5h3"}, SalaryHigh: 9999999, SalaryLow: 60000, JobAddrNoDesc: "台北市信義區", AppearDate: "20260511", ApplyCnt: 2, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14935253", JobName: "【擴編】Golang後端工程師/ Golang Developer", CustName: "瑞典商英鉑科股份有限公司台灣分公司", CustNo: "130000000217988", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8w445", Cust: "https://www.104.com.tw/company/1a2x6bmnic"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 9, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14408054", JobName: "後端工程師-Golang-台北", CustName: "立特有限公司", CustNo: "130000000211187", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8ktbq", Cust: "https://www.104.com.tw/company/1a2x6bmi9f"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 10, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "15106548", JobName: "Senior Backend Engineer (Golang), Virtual insurance", CustName: "香港商六度科技有限公司", CustNo: "130000000161268", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8zsac", Cust: "https://www.104.com.tw/company/1a2x6blfqs"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市信義區", AppearDate: "20260622", ApplyCnt: 12, RemoteWorkType: 2, JobRo: 1},
			{JobNo: "15139656", JobName: "Golang 後端工程師", CustName: "昕展資訊有限公司", CustNo: "130000000261162", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/90hu0", Cust: "https://www.104.com.tw/company/1a2x6bnktm"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260623", ApplyCnt: 10, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14946666", JobName: "後端工程師 (Backend Engineer - Golang)", CustName: "開端智能股份有限公司", CustNo: "130000000255283", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8wcx6", Cust: "https://www.104.com.tw/company/1a2x6bngab"}, SalaryHigh: 80000, SalaryLow: 50000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260626", ApplyCnt: 17, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "13903564", JobName: "Backend Engineer(Java or Golang)", CustName: "重高科技股份有限公司", CustNo: "130000000227435", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8a024", Cust: "https://www.104.com.tw/company/1a2x6bmusr"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260622", ApplyCnt: 17, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "14115841", JobName: "Golang 網站開發工程師(Backend)_零售解決方案課", CustName: "日本NEC集團_統智科技股份有限公司", CustNo: "12876266000", Link: job104.JobSummaryLink{Job: "https://www.104.com.tw/job/8ejup", Cust: "https://www.104.com.tw/company/5wy72fk"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市內湖區", AppearDate: "20260612", ApplyCnt: 19, RemoteWorkType: 0, JobRo: 1},
		},
		Metadata: job104.JobsResponseMetadata{
			Pagination: job104.JobsResponseMetadataPagination{CurrentPage: 1, LastPage: 7, Total: 189},
		},
	}
	assert.Equal(t, wantResp, &got)
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
			{JobNo: "1", JobName: "onsite", CustName: "c1", CustNo: "n1", Link: job104JobSummaryLink{Job: "j1", Cust: "u1"}, SalaryHigh: 2, SalaryLow: 1, JobAddrNoDesc: "a1", AppearDate: "20260101", ApplyCnt: 3, JobType: "Full-time"},
			{JobNo: "2", JobName: "full-remote", CustName: "c2", CustNo: "n2", Link: job104JobSummaryLink{Job: "j2", Cust: "u2"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "a2", AppearDate: "20260102", ApplyCnt: 4, Remote: "Full", JobType: "Part-time"},
			{JobNo: "3", JobName: "hybrid", CustName: "c3", CustNo: "n3", Link: job104JobSummaryLink{Job: "j3", Cust: "u3"}, SalaryHigh: 9, SalaryLow: 5, JobAddrNoDesc: "a3", AppearDate: "20260103", ApplyCnt: 5, Remote: "Partial", JobType: "Dispatch"},
			{JobNo: "4", JobName: "unknown-codes", CustName: "c4", CustNo: "n4", Link: job104JobSummaryLink{Job: "j4", Cust: "u4"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "a4", AppearDate: "20260104", ApplyCnt: 6},
		},
		Metadata: job104SearchMetadata{
			Pagination: job104Pagination{CurrentPage: 1, LastPage: 2, Total: 34},
		},
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
		Keyword:    job104.NewOptString("golang"),
		Area:       job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
		Ro:         job104.NewOptSearchJobsRo(job104.SearchJobsRo2),
		Order:      job104.NewOptSearchJobsOrder(job104.SearchJobsOrder2),
		RemoteWork: job104.NewOptSearchJobsRemoteWork(job104.SearchJobsRemoteWork1),
		Page:       job104.NewOptInt(2),
		Edu:        []job104.SearchJobsEduItem{job104.SearchJobsEduItem4, job104.SearchJobsEduItem5},
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
		Keyword: job104.NewOptString("golang"),
		Area:    job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
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
