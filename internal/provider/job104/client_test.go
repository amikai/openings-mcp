package job104

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/search/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Golang", r.URL.Query().Get("keyword"))
		assert.Equal(t, "6001001000", r.URL.Query().Get("area"))
		assert.Equal(t, "1,3", r.URL.Query().Get("jobexp"))
		assert.Equal(t, "13", r.URL.Query().Get("order"))
		serveMockJSON(mockJobsRsp)(w, r)
	})
	mux.HandleFunc("/job/ajax/content/", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		serveMockJSON(mockJobDetailRsp)(w, r)
	})
	return httptest.NewServer(mux)
}

func TestSearchJobs(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	got, err := c.SearchJobs(t.Context(), SearchJobsParams{
		Keyword: NewOptString("Golang"),
		Area:    NewOptSearchJobsArea(AreaIDs["Taipei"]),
		Jobexp:  []SearchJobsJobexpItem{JobExpIDs["Under1Year"], JobExpIDs["1To3Years"]},
		Order:   NewOptSearchJobsOrder(OrderIDs["SalaryHigh"]),
	})
	require.NoError(t, err)

	want := &JobsResponse{
		Data: []JobSummary{
			{JobNo: "10177057", JobName: "GoLang Developer", CustName: "曜驊智能股份有限公司", CustNo: "130000000042972", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/624o1", Cust: "https://www.104.com.tw/company/1a2x6biwgs"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市內湖區", AppearDate: "20260515", ApplyCnt: 3, RemoteWorkType: 0, JobRo: 1, Period: 0},
			{JobNo: "15015281", JobName: "Golang 後端工程師", CustName: "富一代資訊有限公司", CustNo: "130000000264142", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8xtv5", Cust: "https://www.104.com.tw/company/1a2x6bnn4e"}, SalaryHigh: 120000, SalaryLow: 60000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260609", ApplyCnt: 8, RemoteWorkType: 0, JobRo: 1, Period: 0},
			{JobNo: "11282518", JobName: "Golang 工程師", CustName: "百阜科技股份有限公司", CustNo: "130000000112061", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/6ptna", Cust: "https://www.104.com.tw/company/1a2x6bkdrx"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市內湖區", AppearDate: "20260526", ApplyCnt: 4, RemoteWorkType: 0, JobRo: 1, Period: 2},
			{JobNo: "12689685", JobName: "Senior Cloud Backend Engineer (Golang)", CustName: "華玉科技股份有限公司", CustNo: "130000000180812", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/7jzf9", Cust: "https://www.104.com.tw/company/1a2x6bluto"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 6, RemoteWorkType: 0, JobRo: 1, Period: 6},
			{JobNo: "14271913", JobName: "軟體工程師 Golang", CustName: "線上探索科技股份有限公司", CustNo: "130000000147477", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8hwa1", Cust: "https://www.104.com.tw/company/1a2x6bl53p"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大同區", AppearDate: "20260304", ApplyCnt: 6, RemoteWorkType: 0, JobRo: 1, Period: 2},
			{JobNo: "15160106", JobName: "Software Engineer (Golang, Flutter), Virtual insurance", CustName: "香港商六度科技有限公司", CustNo: "130000000161268", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/90xm2", Cust: "https://www.104.com.tw/company/1a2x6blfqs"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市信義區", AppearDate: "20260618", ApplyCnt: 5, RemoteWorkType: 2, JobRo: 1, Period: 0},
			{JobNo: "13305625", JobName: "Golang開發工程師", CustName: "太禾科技有限公司", CustNo: "130000000177509", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/7x6op", Cust: "https://www.104.com.tw/company/1a2x6bls9x"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260618", ApplyCnt: 5, RemoteWorkType: 0, JobRo: 1, Period: 4},
			{JobNo: "14954565", JobName: "Golang 後端工程師 / Golang Backend Engineer", CustName: "炫石有限公司", CustNo: "130000000241271", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8wj0l", Cust: "https://www.104.com.tw/company/1a2x6bn5h3"}, SalaryHigh: 9999999, SalaryLow: 60000, JobAddrNoDesc: "台北市信義區", AppearDate: "20260511", ApplyCnt: 8, RemoteWorkType: 0, JobRo: 1, Period: 4},
			{JobNo: "14893390", JobName: "Golang開發工程師", CustName: "四天科技有限公司", CustNo: "130000000231318", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8v7ta", Cust: "https://www.104.com.tw/company/1a2x6bmxsm"}, SalaryHigh: 150000, SalaryLow: 80000, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 8, RemoteWorkType: 0, JobRo: 1, Period: 3},
			{JobNo: "14953361", JobName: "【擴編】資深Golang後端工程師 / Senior Golang Developer", CustName: "瑞典商英鉑科股份有限公司台灣分公司", CustNo: "130000000217988", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8wi35", Cust: "https://www.104.com.tw/company/1a2x6bmnic"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 4, RemoteWorkType: 0, JobRo: 1, Period: 6},
			{JobNo: "15115498", JobName: "軟體工程師 (Software Engineer - Golang)", CustName: "立視科技股份有限公司", CustNo: "130000000266972", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8zz6y", Cust: "https://www.104.com.tw/company/1a2x6bnpb0"}, SalaryHigh: 88000, SalaryLow: 55000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260625", ApplyCnt: 4, RemoteWorkType: 0, JobRo: 1, Period: 4},
			{JobNo: "14439753", JobName: "GOLANG 開發工程師", CustName: "益晨資訊科技有限公司", CustNo: "130000000221207", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8lhs9", Cust: "https://www.104.com.tw/company/1a2x6bmpzr"}, SalaryHigh: 90000, SalaryLow: 72000, JobAddrNoDesc: "台北市中正區", AppearDate: "20260625", ApplyCnt: 7, RemoteWorkType: 0, JobRo: 1, Period: 4},
			{JobNo: "13761398", JobName: "Senior Backend Engineer ( Golang )（每月有遠端日）", CustName: "幣託科技股份有限公司", CustNo: "130000000223436", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/86yd2", Cust: "https://www.104.com.tw/company/1a2x6bmrpo"}, SalaryHigh: 150000, SalaryLow: 85000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260622", ApplyCnt: 10, RemoteWorkType: 2, JobRo: 1, Period: 6},
			{JobNo: "15097562", JobName: "後端工程師（Golang）", CustName: "米奈娛樂有限公司", CustNo: "130000000251337", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8zlcq", Cust: "https://www.104.com.tw/company/1a2x6bnd8p"}, SalaryHigh: 80000, SalaryLow: 70000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260620", ApplyCnt: 7, RemoteWorkType: 0, JobRo: 1, Period: 3},
			{JobNo: "14335204", JobName: "Golang後端與DevOps工程師", CustName: "時刻無限股份有限公司", CustNo: "130000000242671", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8j944", Cust: "https://www.104.com.tw/company/1a2x6bn6jz"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260622", ApplyCnt: 8, RemoteWorkType: 0, JobRo: 1, Period: 2},
			{JobNo: "14660408", JobName: "Golang 遊戲開發工程師(大安)", CustName: "天晴資訊有限公司", CustNo: "130000000167545", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8q81k", Cust: "https://www.104.com.tw/company/1a2x6blkl5"}, SalaryHigh: 95000, SalaryLow: 50000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 7, RemoteWorkType: 0, JobRo: 1, Period: 0},
			{JobNo: "15245367", JobName: "Golang Engineer", CustName: "瞬聯科技股份有限公司", CustNo: "130000000159109", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/92ref", Cust: "https://www.104.com.tw/company/1a2x6ble2t"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 20, RemoteWorkType: 0, JobRo: 1, Period: 2},
			{JobNo: "13766806", JobName: "【純遠端】國際遊戲公司 誠徵  Go/Golang 工程師", CustName: "台灣英特艾倫人力資源有限公司", CustNo: "130000000048447", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/872ja", Cust: "https://www.104.com.tw/company/1a2x6bj0ov"}, SalaryHigh: 180000, SalaryLow: 150000, JobAddrNoDesc: "台北市中山區", AppearDate: "20260623", ApplyCnt: 12, RemoteWorkType: 1, JobRo: 1, Period: 6},
			{JobNo: "14645682", JobName: "Golang 後端工程師(大安)", CustName: "天晴資訊有限公司", CustNo: "130000000167545", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8pwoi", Cust: "https://www.104.com.tw/company/1a2x6blkl5"}, SalaryHigh: 9999999, SalaryLow: 50000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 5, RemoteWorkType: 0, JobRo: 1, Period: 4},
			{JobNo: "15043542", JobName: "【TENG0502】Software Engineer (Backend) - Golang / RoR", CustName: "喬富科技股份有限公司", CustNo: "130000000264905", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8yfo6", Cust: "https://www.104.com.tw/company/1a2x6bnnpl"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市松山區", AppearDate: "20260623", ApplyCnt: 10, RemoteWorkType: 0, JobRo: 1, Period: 3},
			{JobNo: "14525012", JobName: "Golang工程師-Junior", CustName: "彼雅特科技股份有限公司", CustNo: "130000000220505", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8nbkk", Cust: "https://www.104.com.tw/company/1a2x6bmpg9"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市信義區", AppearDate: "20260529", ApplyCnt: 6, RemoteWorkType: 0, JobRo: 1, Period: 0},
			{JobNo: "14797877", JobName: "[資訊部]Golang工程師", CustName: "虹耀建設股份有限公司", CustNo: "130000000145239", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8t645", Cust: "https://www.104.com.tw/company/1a2x6bl3dj"}, SalaryHigh: 9999999, SalaryLow: 75000, JobAddrNoDesc: "台北市中正區", AppearDate: "20260622", ApplyCnt: 6, RemoteWorkType: 0, JobRo: 1, Period: 6},
			{JobNo: "14965947", JobName: "資深後端工程師（Golang / Java） / Senior Backend Engineer（Golang / Java）", CustName: "炫石有限公司", CustNo: "130000000241271", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8wrsr", Cust: "https://www.104.com.tw/company/1a2x6bn5h3"}, SalaryHigh: 9999999, SalaryLow: 60000, JobAddrNoDesc: "台北市信義區", AppearDate: "20260511", ApplyCnt: 2, RemoteWorkType: 0, JobRo: 1, Period: 4},
			{JobNo: "14935253", JobName: "【擴編】Golang後端工程師/ Golang Developer", CustName: "瑞典商英鉑科股份有限公司台灣分公司", CustNo: "130000000217988", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8w445", Cust: "https://www.104.com.tw/company/1a2x6bmnic"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 9, RemoteWorkType: 0, JobRo: 1, Period: 4},
			{JobNo: "14408054", JobName: "後端工程師-Golang-台北", CustName: "立特有限公司", CustNo: "130000000211187", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8ktbq", Cust: "https://www.104.com.tw/company/1a2x6bmi9f"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 10, RemoteWorkType: 0, JobRo: 1, Period: 3},
			{JobNo: "15106548", JobName: "Senior Backend Engineer (Golang), Virtual insurance", CustName: "香港商六度科技有限公司", CustNo: "130000000161268", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8zsac", Cust: "https://www.104.com.tw/company/1a2x6blfqs"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市信義區", AppearDate: "20260622", ApplyCnt: 12, RemoteWorkType: 2, JobRo: 1, Period: 0},
			{JobNo: "15139656", JobName: "Golang 後端工程師", CustName: "昕展資訊有限公司", CustNo: "130000000261162", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/90hu0", Cust: "https://www.104.com.tw/company/1a2x6bnktm"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市中山區", AppearDate: "20260623", ApplyCnt: 10, RemoteWorkType: 0, JobRo: 1, Period: 0},
			{JobNo: "14946666", JobName: "後端工程師 (Backend Engineer - Golang)", CustName: "開端智能股份有限公司", CustNo: "130000000255283", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8wcx6", Cust: "https://www.104.com.tw/company/1a2x6bngab"}, SalaryHigh: 80000, SalaryLow: 50000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260626", ApplyCnt: 17, RemoteWorkType: 0, JobRo: 1, Period: 0},
			{JobNo: "13903564", JobName: "Backend Engineer(Java or Golang)", CustName: "重高科技股份有限公司", CustNo: "130000000227435", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8a024", Cust: "https://www.104.com.tw/company/1a2x6bmusr"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市大安區", AppearDate: "20260622", ApplyCnt: 17, RemoteWorkType: 0, JobRo: 1, Period: 3},
			{JobNo: "14115841", JobName: "Golang 網站開發工程師(Backend)_零售解決方案課", CustName: "日本NEC集團_統智科技股份有限公司", CustNo: "12876266000", Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8ejup", Cust: "https://www.104.com.tw/company/5wy72fk"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "台北市內湖區", AppearDate: "20260612", ApplyCnt: 19, RemoteWorkType: 0, JobRo: 1, Period: 2},
		},
		Metadata: JobsResponseMetadata{
			Pagination: JobsResponseMetadataPagination{CurrentPage: 1, LastPage: 7, Total: 189},
		},
	}
	assert.Equal(t, want, got)
}

func TestGetJobDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	got, err := c.GetJobDetail(t.Context(), GetJobDetailParams{JobCode: "624o1"})
	require.NoError(t, err)

	want := &JobDetailResponse{
		Data: JobDetail{
			Header: JobDetailHeader{
				JobName:    "軟體工程師 (數位工程發展部)",
				CustName:   "亞新工程顧問股份有限公司",
				CustUrl:    "https://www.104.com.tw/company/264c9zc",
				AppearDate: "2026/06/22",
				IsSaved:    false,
				IsApplied:  false,
			},
			Contact: JobDetailContact{
				HrName: NewOptString("Rachel Chiu 邱小姐"),
				Email:  NewOptString("personnel@maaconsultants.com,cj.yu@maaconsultants.com,eugene.shen@maaconsultants.com,fred.chou@maaconsultants.com"),
				Reply:  NewOptString(""),
			},
			Condition: JobDetailCondition{
				WorkExp: NewOptString("不拘"),
				Edu:     NewOptString("大學以上"),
				Major:   []string{"資訊工程相關"},
				Specialty: []CodeDescription{
					{Code: NewOptString("12001003009"), Description: NewOptString("C#")},
					{Code: NewOptString("12001003006"), Description: NewOptString("ASP.NET")},
					{Code: NewOptString("12001004031"), Description: NewOptString("MS SQL")},
					{Code: NewOptString("12001003045"), Description: NewOptString("Python")},
					{Code: NewOptString("12003001003"), Description: NewOptString("GIS")},
					{Code: NewOptString("12001003094"), Description: NewOptString("IoT")},
					{Code: NewOptString("12002003010"), Description: NewOptString("Revit")},
				},
			},
			Welfare: JobDetailWelfare{
				Welfare: NewOptString("在亞新，我們重視同仁的職涯成長與友善職場，透過全方位的福利與支持，推動以人為本、永續發展的職場環境，實現工作與生活的和諧平衡。\n\n【薪酬與獎金】\n  •  具市場競爭力的薪資水準\n  •  年節獎金與專案獎金，共享成果回饋\n\n【健康與保障】\n  •  勞健保及完整團體保險(意外、醫療、重大疾病、職災保障)\n  •  定期健康檢查、健康講座與員工關懷方案\n\n【休假與彈性】\n  •  彈性上下班、育兒友善措施，兼顧生活平衡\n\n【教育訓練與發展】\n  •  完善新人培訓與師徒制\n  •  E-learning 線上學習資源\n  •  專業證照補助（如 PMP、專業技師等）\n  •  外部訓練與國際研討會，拓展國際視野\n  •  參與國家級重大工程，累積獨特專業經驗\n\n【生活與休閒】\n  •  福委會關懷：生日禮金、節慶禮品或禮券、婚喪喜慶、傷病住院慰問與生育補助\n  •  部門聚餐、咖啡分享日、社團活動、Happy Hour，促進交流與凝聚力\n  •  舒適職場環境：明亮開放空間、零食吧、茶包與自助研磨咖啡機\n\n【招募流程】\n  1. 投遞履歷\n  2. HR初審履歷 → 部門主管面試\n  3. Final面談（含專案介紹與Q&A）\n  4. 錄取通知\n （流程清楚透明，讓你安心應徵!)"),
			},
			JobDetail: JobDetailJobDetail{
				JobDescription: NewOptString("無相關經驗可，大學以上資訊工程、資訊管理等相關科系畢業\n\n【工作內容】\n- 參與智慧工程數位平台的設計、開發與維運\n- 開發與維護 GIS、BIM 系統，並支援無人機地形數據應用\n- 參與 AI 工具與文件管理系統之開發 \n- 與跨領域團隊合作（工程、IoT、BIM、AI），推動數位轉型與自動化流程\n\n【希望條件】\n- 熟悉現代軟體系統研發流程與版本控制\n- 熟悉至少一種指令式程式設計語言（C#、JavaScript、Python、PHP 尤佳）\n- 具 ASP.NET、SQL、Vue.js、Laravel、Unity、GIS、IoT、Revit等開發經驗\n- 具軟體設計、開發、運營、開發、機器學習、AI 模型訓練 (Finetuning)、 AI 應用設計（OCR、RAG、LLM、Agentic 等）開發、導入經驗\n- 具 Azure DevOps、Docker、Kubernetes 經驗者優先\n\n＊我們期待具備高度邏輯思維、善於溝通系統需求與設計選擇，並能獨立完成軟體開發的夥伴加入，一起參與系統規劃與優化。"),
				JobCategory: []CodeDescription{
					{Code: NewOptString("2007001004"), Description: NewOptString("軟體工程師")},
				},
				Salary:        NewOptString("待遇面議"),
				SalaryMin:     NewOptInt(0),
				SalaryMax:     NewOptInt(0),
				AddressRegion: NewOptString("新北市汐止區"),
				AddressDetail: NewOptString("新台五路一段112號22樓"),
				ManageResp:    NewOptString("不需負擔管理責任"),
				NeedEmp:       NewOptString("2~3人"),
				JobType:       NewOptInt(1),
				RemoteWork:    OptNilJobDetailJobDetailRemoteWork{Null: true, Set: true},
			},
			Industry:  "建築及工程技術服務業",
			Employees: "1200人",
			CustNo:    "264c9zc",
		},
	}
	assert.Equal(t, want, got)
}

func TestSearchJobsUpstreamError(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	_, err = c.SearchJobs(t.Context(), SearchJobsParams{
		Keyword: NewOptString(MockErrorKeyword),
		Area:    NewOptSearchJobsArea(AreaIDs["Taipei"]),
	})
	require.Error(t, err)

	ue, ok := errors.AsType[*ErrorResponseStatusCode](err)
	require.True(t, ok, "expected *ErrorResponseStatusCode in %v", err)
	want := &ErrorResponseStatusCode{
		StatusCode: 500,
		Response:   ErrorResponse{Message: NewOptString("internal error"), AdditionalProps: ErrorResponseAdditional{}},
	}
	assert.Equal(t, want, ue)
}

func TestGetJobDetailUpstreamError(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	_, err = c.GetJobDetail(t.Context(), GetJobDetailParams{JobCode: MockNotFoundJobCode})
	require.Error(t, err)

	ue, ok := errors.AsType[*ErrorResponseStatusCode](err)
	require.True(t, ok, "expected *ErrorResponseStatusCode in %v", err)
	want := &ErrorResponseStatusCode{
		StatusCode: 404,
		Response:   ErrorResponse{Message: NewOptString("job not found"), AdditionalProps: ErrorResponseAdditional{}},
	}
	assert.Equal(t, want, ue)
}
