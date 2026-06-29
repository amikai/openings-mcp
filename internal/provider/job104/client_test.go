package job104

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/search/api/jobs", serveTestdata("testdata/jobs_rsp.json"))
	mux.HandleFunc("/job/ajax/content/", serveTestdata("testdata/job_detail_rsp.json"))
	mux.HandleFunc("/company/ajax/list", serveTestdata("testdata/companies_rsp.json"))
	mux.HandleFunc("/api/companies/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/content") {
			serveTestdata("testdata/company_detail_rsp.json")(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/jobs") {
			serveTestdata("testdata/company_jobs_rsp.json")(w, r)
		}
	})
	return httptest.NewServer(mux)
}

func serveTestdata(path string) http.HandlerFunc {
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

func jobLink(job, cust string) struct {
	Job  string `json:"job"`
	Cust string `json:"cust"`
} {
	return struct {
		Job  string `json:"job"`
		Cust string `json:"cust"`
	}{Job: job, Cust: cust}
}

func TestJobs(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.Jobs(t.Context(), &JobsRequest{Keyword: "Golang", Area: AreaTaipei})
	require.NoError(t, err)

	want := &JobsResponse{
		Data: []Job{
			{JobNo: "10177057", JobName: "GoLang Developer", CustName: "曜驊智能股份有限公司", CustNo: "130000000042972", Link: jobLink("https://www.104.com.tw/job/624o1", "https://www.104.com.tw/company/1a2x6biwgs"), JobAddrNoDesc: "台北市內湖區", AppearDate: "20260515", ApplyCnt: 3},
			{JobNo: "15015281", JobName: "Golang 後端工程師", CustName: "富一代資訊有限公司", CustNo: "130000000264142", Link: jobLink("https://www.104.com.tw/job/8xtv5", "https://www.104.com.tw/company/1a2x6bnn4e"), SalaryHigh: 120000, SalaryLow: 60000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260609", ApplyCnt: 8},
			{JobNo: "11282518", JobName: "Golang 工程師", CustName: "百阜科技股份有限公司", CustNo: "130000000112061", Link: jobLink("https://www.104.com.tw/job/6ptna", "https://www.104.com.tw/company/1a2x6bkdrx"), JobAddrNoDesc: "台北市內湖區", AppearDate: "20260526", ApplyCnt: 4},
			{JobNo: "12689685", JobName: "Senior Cloud Backend Engineer (Golang)", CustName: "華玉科技股份有限公司", CustNo: "130000000180812", Link: jobLink("https://www.104.com.tw/job/7jzf9", "https://www.104.com.tw/company/1a2x6bluto"), JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 6},
			{JobNo: "14271913", JobName: "軟體工程師 Golang", CustName: "線上探索科技股份有限公司", CustNo: "130000000147477", Link: jobLink("https://www.104.com.tw/job/8hwa1", "https://www.104.com.tw/company/1a2x6bl53p"), JobAddrNoDesc: "台北市大同區", AppearDate: "20260304", ApplyCnt: 6},
			{JobNo: "15160106", JobName: "Software Engineer (Golang, Flutter), Virtual insurance", CustName: "香港商六度科技有限公司", CustNo: "130000000161268", Link: jobLink("https://www.104.com.tw/job/90xm2", "https://www.104.com.tw/company/1a2x6blfqs"), JobAddrNoDesc: "台北市信義區", AppearDate: "20260618", ApplyCnt: 5, RemoteWorkType: 2},
			{JobNo: "13305625", JobName: "Golang開發工程師", CustName: "太禾科技有限公司", CustNo: "130000000177509", Link: jobLink("https://www.104.com.tw/job/7x6op", "https://www.104.com.tw/company/1a2x6bls9x"), JobAddrNoDesc: "台北市中山區", AppearDate: "20260618", ApplyCnt: 5},
			{JobNo: "14954565", JobName: "Golang 後端工程師 / Golang Backend Engineer", CustName: "炫石有限公司", CustNo: "130000000241271", Link: jobLink("https://www.104.com.tw/job/8wj0l", "https://www.104.com.tw/company/1a2x6bn5h3"), SalaryHigh: 9999999, SalaryLow: 60000, JobAddrNoDesc: "台北市信義區", AppearDate: "20260511", ApplyCnt: 8},
			{JobNo: "14893390", JobName: "Golang開發工程師", CustName: "四天科技有限公司", CustNo: "130000000231318", Link: jobLink("https://www.104.com.tw/job/8v7ta", "https://www.104.com.tw/company/1a2x6bmxsm"), SalaryHigh: 150000, SalaryLow: 80000, JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 8},
			{JobNo: "14953361", JobName: "【擴編】資深Golang後端工程師 / Senior Golang Developer", CustName: "瑞典商英鉑科股份有限公司台灣分公司", CustNo: "130000000217988", Link: jobLink("https://www.104.com.tw/job/8wi35", "https://www.104.com.tw/company/1a2x6bmnic"), JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 4},
			{JobNo: "15115498", JobName: "軟體工程師 (Software Engineer - Golang)", CustName: "立視科技股份有限公司", CustNo: "130000000266972", Link: jobLink("https://www.104.com.tw/job/8zz6y", "https://www.104.com.tw/company/1a2x6bnpb0"), SalaryHigh: 88000, SalaryLow: 55000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260625", ApplyCnt: 4},
			{JobNo: "14439753", JobName: "GOLANG 開發工程師", CustName: "益晨資訊科技有限公司", CustNo: "130000000221207", Link: jobLink("https://www.104.com.tw/job/8lhs9", "https://www.104.com.tw/company/1a2x6bmpzr"), SalaryHigh: 90000, SalaryLow: 72000, JobAddrNoDesc: "台北市中正區", AppearDate: "20260625", ApplyCnt: 7},
			{JobNo: "13761398", JobName: "Senior Backend Engineer ( Golang )（每月有遠端日）", CustName: "幣託科技股份有限公司", CustNo: "130000000223436", Link: jobLink("https://www.104.com.tw/job/86yd2", "https://www.104.com.tw/company/1a2x6bmrpo"), SalaryHigh: 150000, SalaryLow: 85000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260622", ApplyCnt: 10, RemoteWorkType: 2},
			{JobNo: "15097562", JobName: "後端工程師（Golang）", CustName: "米奈娛樂有限公司", CustNo: "130000000251337", Link: jobLink("https://www.104.com.tw/job/8zlcq", "https://www.104.com.tw/company/1a2x6bnd8p"), SalaryHigh: 80000, SalaryLow: 70000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260620", ApplyCnt: 7},
			{JobNo: "14335204", JobName: "Golang後端與DevOps工程師", CustName: "時刻無限股份有限公司", CustNo: "130000000242671", Link: jobLink("https://www.104.com.tw/job/8j944", "https://www.104.com.tw/company/1a2x6bn6jz"), JobAddrNoDesc: "台北市大安區", AppearDate: "20260622", ApplyCnt: 8},
			{JobNo: "14660408", JobName: "Golang 遊戲開發工程師(大安)", CustName: "天晴資訊有限公司", CustNo: "130000000167545", Link: jobLink("https://www.104.com.tw/job/8q81k", "https://www.104.com.tw/company/1a2x6blkl5"), SalaryHigh: 95000, SalaryLow: 50000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 7},
			{JobNo: "15245367", JobName: "Golang Engineer", CustName: "瞬聯科技股份有限公司", CustNo: "130000000159109", Link: jobLink("https://www.104.com.tw/job/92ref", "https://www.104.com.tw/company/1a2x6ble2t"), JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 20},
			{JobNo: "13766806", JobName: "【純遠端】國際遊戲公司 誠徵  Go/Golang 工程師", CustName: "台灣英特艾倫人力資源有限公司", CustNo: "130000000048447", Link: jobLink("https://www.104.com.tw/job/872ja", "https://www.104.com.tw/company/1a2x6bj0ov"), SalaryHigh: 180000, SalaryLow: 150000, JobAddrNoDesc: "台北市中山區", AppearDate: "20260623", ApplyCnt: 12, RemoteWorkType: 1},
			{JobNo: "14645682", JobName: "Golang 後端工程師(大安)", CustName: "天晴資訊有限公司", CustNo: "130000000167545", Link: jobLink("https://www.104.com.tw/job/8pwoi", "https://www.104.com.tw/company/1a2x6blkl5"), SalaryHigh: 9999999, SalaryLow: 50000, JobAddrNoDesc: "台北市大安區", AppearDate: "20260623", ApplyCnt: 5},
			{JobNo: "15043542", JobName: "【TENG0502】Software Engineer (Backend) - Golang / RoR", CustName: "喬富科技股份有限公司", CustNo: "130000000264905", Link: jobLink("https://www.104.com.tw/job/8yfo6", "https://www.104.com.tw/company/1a2x6bnnpl"), JobAddrNoDesc: "台北市松山區", AppearDate: "20260623", ApplyCnt: 10},
			{JobNo: "14525012", JobName: "Golang工程師-Junior", CustName: "彼雅特科技股份有限公司", CustNo: "130000000220505", Link: jobLink("https://www.104.com.tw/job/8nbkk", "https://www.104.com.tw/company/1a2x6bmpg9"), JobAddrNoDesc: "台北市信義區", AppearDate: "20260529", ApplyCnt: 6},
			{JobNo: "14797877", JobName: "[資訊部]Golang工程師", CustName: "虹耀建設股份有限公司", CustNo: "130000000145239", Link: jobLink("https://www.104.com.tw/job/8t645", "https://www.104.com.tw/company/1a2x6bl3dj"), SalaryHigh: 9999999, SalaryLow: 75000, JobAddrNoDesc: "台北市中正區", AppearDate: "20260622", ApplyCnt: 6},
			{JobNo: "14965947", JobName: "資深後端工程師（Golang / Java） / Senior Backend Engineer（Golang / Java）", CustName: "炫石有限公司", CustNo: "130000000241271", Link: jobLink("https://www.104.com.tw/job/8wrsr", "https://www.104.com.tw/company/1a2x6bn5h3"), SalaryHigh: 9999999, SalaryLow: 60000, JobAddrNoDesc: "台北市信義區", AppearDate: "20260511", ApplyCnt: 2},
			{JobNo: "14935253", JobName: "【擴編】Golang後端工程師/ Golang Developer", CustName: "瑞典商英鉑科股份有限公司台灣分公司", CustNo: "130000000217988", Link: jobLink("https://www.104.com.tw/job/8w445", "https://www.104.com.tw/company/1a2x6bmnic"), JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 9},
			{JobNo: "14408054", JobName: "後端工程師-Golang-台北", CustName: "立特有限公司", CustNo: "130000000211187", Link: jobLink("https://www.104.com.tw/job/8ktbq", "https://www.104.com.tw/company/1a2x6bmi9f"), JobAddrNoDesc: "台北市中山區", AppearDate: "20260622", ApplyCnt: 10},
			{JobNo: "15106548", JobName: "Senior Backend Engineer (Golang), Virtual insurance", CustName: "香港商六度科技有限公司", CustNo: "130000000161268", Link: jobLink("https://www.104.com.tw/job/8zsac", "https://www.104.com.tw/company/1a2x6blfqs"), JobAddrNoDesc: "台北市信義區", AppearDate: "20260622", ApplyCnt: 12, RemoteWorkType: 2},
			{JobNo: "15139656", JobName: "Golang 後端工程師", CustName: "昕展資訊有限公司", CustNo: "130000000261162", Link: jobLink("https://www.104.com.tw/job/90hu0", "https://www.104.com.tw/company/1a2x6bnktm"), JobAddrNoDesc: "台北市中山區", AppearDate: "20260623", ApplyCnt: 10},
			{JobNo: "14946666", JobName: "後端工程師 (Backend Engineer - Golang)", CustName: "開端智能股份有限公司", CustNo: "130000000255283", Link: jobLink("https://www.104.com.tw/job/8wcx6", "https://www.104.com.tw/company/1a2x6bngab"), SalaryHigh: 80000, SalaryLow: 50000, JobAddrNoDesc: "台北市松山區", AppearDate: "20260626", ApplyCnt: 17},
			{JobNo: "13903564", JobName: "Backend Engineer(Java or Golang)", CustName: "重高科技股份有限公司", CustNo: "130000000227435", Link: jobLink("https://www.104.com.tw/job/8a024", "https://www.104.com.tw/company/1a2x6bmusr"), JobAddrNoDesc: "台北市大安區", AppearDate: "20260622", ApplyCnt: 17},
			{JobNo: "14115841", JobName: "Golang 網站開發工程師(Backend)_零售解決方案課", CustName: "日本NEC集團_統智科技股份有限公司", CustNo: "12876266000", Link: jobLink("https://www.104.com.tw/job/8ejup", "https://www.104.com.tw/company/5wy72fk"), JobAddrNoDesc: "台北市內湖區", AppearDate: "20260612", ApplyCnt: 19},
		},
		Metadata: struct {
			Pagination struct {
				CurrentPage int `json:"currentPage"`
				LastPage    int `json:"lastPage"`
				Total       int `json:"total"`
			} `json:"pagination"`
		}{
			Pagination: struct {
				CurrentPage int `json:"currentPage"`
				LastPage    int `json:"lastPage"`
				Total       int `json:"total"`
			}{CurrentPage: 1, LastPage: 7, Total: 189},
		},
	}
	assert.Equal(t, want, got)
}

func TestJobDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.JobDetail(t.Context(), "624o1")
	require.NoError(t, err)

	want := &JobDetailResponse{
		Data: JobDetail{
			Header: struct {
				JobName    string `json:"jobName"`
				CustName   string `json:"custName"`
				CustURL    string `json:"custUrl"`
				AppearDate string `json:"appearDate"`
				IsSaved    bool   `json:"isSaved"`
				IsApplied  bool   `json:"isApplied"`
			}{
				JobName:    "軟體工程師 (數位工程發展部)",
				CustName:   "亞新工程顧問股份有限公司",
				CustURL:    "https://www.104.com.tw/company/264c9zc",
				AppearDate: "2026/06/22",
			},
			Contact: struct {
				HRName string `json:"hrName"`
				Email  string `json:"email"`
				Reply  string `json:"reply"`
			}{
				HRName: "Rachel Chiu 邱小姐",
				Email:  "personnel@maaconsultants.com,cj.yu@maaconsultants.com,eugene.shen@maaconsultants.com,fred.chou@maaconsultants.com",
			},
			Condition: struct {
				WorkExp   string   `json:"workExp"`
				Edu       string   `json:"edu"`
				Major     []string `json:"major"`
				Specialty []struct {
					Code        string `json:"code"`
					Description string `json:"description"`
				} `json:"specialty"`
			}{
				WorkExp: "不拘",
				Edu:     "大學以上",
				Major:   []string{"資訊工程相關"},
				Specialty: []struct {
					Code        string `json:"code"`
					Description string `json:"description"`
				}{
					{Code: "12001003009", Description: "C#"},
					{Code: "12001003006", Description: "ASP.NET"},
					{Code: "12001004031", Description: "MS SQL"},
					{Code: "12001003045", Description: "Python"},
					{Code: "12003001003", Description: "GIS"},
					{Code: "12001003094", Description: "IoT"},
					{Code: "12002003010", Description: "Revit"},
				},
			},
			Welfare: struct {
				Welfare string `json:"welfare"`
			}{
				Welfare: "在亞新，我們重視同仁的職涯成長與友善職場，透過全方位的福利與支持，推動以人為本、永續發展的職場環境，實現工作與生活的和諧平衡。\n\n【薪酬與獎金】\n  •  具市場競爭力的薪資水準\n  •  年節獎金與專案獎金，共享成果回饋\n\n【健康與保障】\n  •  勞健保及完整團體保險(意外、醫療、重大疾病、職災保障)\n  •  定期健康檢查、健康講座與員工關懷方案\n\n【休假與彈性】\n  •  彈性上下班、育兒友善措施，兼顧生活平衡\n\n【教育訓練與發展】\n  •  完善新人培訓與師徒制\n  •  E-learning 線上學習資源\n  •  專業證照補助（如 PMP、專業技師等）\n  •  外部訓練與國際研討會，拓展國際視野\n  •  參與國家級重大工程，累積獨特專業經驗\n\n【生活與休閒】\n  •  福委會關懷：生日禮金、節慶禮品或禮券、婚喪喜慶、傷病住院慰問與生育補助\n  •  部門聚餐、咖啡分享日、社團活動、Happy Hour，促進交流與凝聚力\n  •  舒適職場環境：明亮開放空間、零食吧、茶包與自助研磨咖啡機\n\n【招募流程】\n  1. 投遞履歷\n  2. HR初審履歷 → 部門主管面試\n  3. Final面談（含專案介紹與Q&A）\n  4. 錄取通知\n （流程清楚透明，讓你安心應徵!)",
			},
			JobDetail: struct {
				JobDescription string `json:"jobDescription"`
				JobCategory    []struct {
					Code        string `json:"code"`
					Description string `json:"description"`
				} `json:"jobCategory"`
				Salary        string `json:"salary"`
				SalaryMin     int    `json:"salaryMin"`
				SalaryMax     int    `json:"salaryMax"`
				JobType       int    `json:"jobType"`
				AddressRegion string `json:"addressRegion"`
				AddressDetail string `json:"addressDetail"`
				ManageResp    string `json:"manageResp"`
				NeedEmp       string `json:"needEmp"`
				RemoteWork    string `json:"remoteWork"`
			}{
				JobDescription: "無相關經驗可，大學以上資訊工程、資訊管理等相關科系畢業\n\n【工作內容】\n- 參與智慧工程數位平台的設計、開發與維運\n- 開發與維護 GIS、BIM 系統，並支援無人機地形數據應用\n- 參與 AI 工具與文件管理系統之開發 \n- 與跨領域團隊合作（工程、IoT、BIM、AI），推動數位轉型與自動化流程\n\n【希望條件】\n- 熟悉現代軟體系統研發流程與版本控制\n- 熟悉至少一種指令式程式設計語言（C#、JavaScript、Python、PHP 尤佳）\n- 具 ASP.NET、SQL、Vue.js、Laravel、Unity、GIS、IoT、Revit等開發經驗\n- 具軟體設計、開發、運營、開發、機器學習、AI 模型訓練 (Finetuning)、 AI 應用設計（OCR、RAG、LLM、Agentic 等）開發、導入經驗\n- 具 Azure DevOps、Docker、Kubernetes 經驗者優先\n\n＊我們期待具備高度邏輯思維、善於溝通系統需求與設計選擇，並能獨立完成軟體開發的夥伴加入，一起參與系統規劃與優化。",
				JobCategory: []struct {
					Code        string `json:"code"`
					Description string `json:"description"`
				}{
					{Code: "2007001004", Description: "軟體工程師"},
				},
				Salary:        "待遇面議",
				JobType:       1,
				AddressRegion: "新北市汐止區",
				AddressDetail: "新台五路一段112號22樓",
				ManageResp:    "不需負擔管理責任",
				NeedEmp:       "2~3人",
			},
			Industry:  "建築及工程技術服務業",
			Employees: "1200人",
			CustNo:    "264c9zc",
		},
	}
	assert.Equal(t, want, got)
}

func TestCompanies(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.Companies(t.Context(), &CompaniesRequest{Keyword: "科技"})
	require.NoError(t, err)

	want := &CompaniesResponse{
		Data: []Company{
			{EncodedCustNo: "a5h92m0", Name: "台灣積體電路製造股份有限公司(台積電)", AreaDesc: "新竹市", IndustryDesc: "半導體製造業", CapitalDesc: "資本額暫不提供", EmployeeCountDesc: "員工數暫不提供", Profile: "台積公司是全世界最大的專業積體電路製造服務公司．\n\n台積公司在民國七十六年成立於台灣新竹科學工業園區，並開創了專業積體電路製造服務商業模式。\n\n台積公司以領先業界的製程技術及設計解決方案組合支援其全球客戶及夥伴生態系統的蓬勃發展，以此釋放全球半導體產業的創新。身為全球的企業公民，台積公司的營運範圍遍及亞洲、歐洲及北美。", JobCount: 187},
			{EncodedCustNo: "1a2x6blcv1", Name: "台積電機有限公司", AreaDesc: "台中市南屯區", IndustryDesc: "空調水機電工程業", CapitalDesc: "資本額暫不提供", EmployeeCountDesc: "員工數暫不提供", Profile: "我們重視每一位員工，除了有良好工作環境、也提供學習及成長的空間，歡迎優秀的朋友一起加入台積電機有限公司的工作行列。\r\n"},
			{EncodedCustNo: "1a2x6bnjin", Name: "城市漢堡(楠梓台積電)_乾多多商行", AreaDesc: "高雄市前金區", IndustryDesc: "餐館業", CapitalDesc: "資本額暫不提供", EmployeeCountDesc: "員工數暫不提供", Profile: "我們重視每一位員工，除了有良好工作環境、也提供學習及成長的空間，歡迎優秀的朋友一起加入「乾多多商行」的工作行列。"},
			{EncodedCustNo: "9ka2llc", Name: "中鼎集團_中鼎工程股份有限公司", AreaDesc: "台北市士林區", IndustryDesc: "建築及工程技術服務業", CapitalDesc: "資本額76億", EmployeeCountDesc: "員工數8000人", Profile: "中鼎工程股份有限公司創立於西元1979年4月6日，總部設立於臺北市。數十年來，以豐富的技術經驗、穩健的財務與管理制度、精實整齊的人力資源以及卓越的品質口碑，執臺灣工程業界之牛耳，同時享譽國際。秉持專業、誠信、團隊、創新之企業文化精神，中鼎公司不斷蓄積能量，強化體質，致力拓展業務至全球市場，締造斐然佳績。目前於全球超過15個國家地區成立約40家關係企業，集團員工總數約8,000人。\n\n中鼎公司是臺灣最大也是唯一自工程規劃、設計、採購、製造、建造施工、監理到試車操作都能勝任的統包工程公司，素以承攬全球重大工程聞名。一路發展下來，除原有煉油、石化、化工等服務範圍之工程設計建造外，進而開拓電力、鋼鐵、儲運、交通、焚化爐、公共建設、及環境工程等領域；並成功自國外轉移工程技術於臺灣生根，成為國際知名工程公司指定的合作夥伴，展現傲人的成績。近年來，更積極地朝向國際化與多元化的經營目標邁進，業務穩定成長。中鼎公司治理良好，財務透明度高，資訊揭露公平、公正、公開，因而獲得外資青睞，目前外資持股比率攀升至約 52％。 \n\n此外，中鼎公司在服務品質提升、安全管理、健康照護及環境保護等方面亦不遺餘力。目前已通過ISO 9001: 2015品質管理系統、ISO 14001: 2015環境管理系統、OHSAS 18001: 2007職業安全衛生系統等國際驗證，並隨國際標準之改版持續維護管理系統之更新與驗證。通過TOSHMS: 2018臺灣職業安全衛生管理系統驗證，並於2010年通過行政院勞委會職業安全衛生管理系統績效認可，以及衛生署國民健康局健康職場自主認證，亦已順利通過ISO 9001:2015轉版驗證稽核，充分展現其成果已獲得政府及顧客之肯定與信任。 \n\n◎ 公司榮耀 ◎\n● 持續入圍DJSI(道瓊永續指數)台灣企業\n● 英國標準協會(BSI) 「永續卓越獎」\n● TCSA「十大永續典範獎」及GCSA等17獎項\n● SGS 「人才發展卓越獎」、「知識管理品質典範獎」、「職安衛績效管理典範獎」\n● 天下雜誌調查:\n-650大服務業工程承攬類第1名\n-天下CSR企業公民獎\n● 2019年ENR雜誌國際工程設計公司(International Design Firms)排名第80名、 \n    2019年ENR雜誌國際工程統包商 (International Contractors)排名第73名", JobCount: 84},
			{EncodedCustNo: "a9pjy3k", Name: "信義房屋股份有限公司", AreaDesc: "台北市信義區", IndustryDesc: "不動產經營業", CapitalDesc: "資本額73億6800萬元", EmployeeCountDesc: "員工數5000人", Profile: "信義房屋成立於1981年，過去40年我們致力打造「好交易」為目標，下個40年將聚焦以「家」的相關領域為核心，以「永續、好生活」為發展藍圖。\n\n信義房屋是ESG先行者、也是PropTech房地產科技的實踐家，我們持續優化人才培育計畫與薪酬福利制度，致力於讓每個夥伴都能在信義房屋發揮獨特價值，你的好，不需被年資綁架，我們會持續打造能發揮每個人最大潛能的職涯舞台！\n\n《業界頂標頂規培訓制度》\n信義房屋熱情歡迎大學畢業且無房仲經驗的菁英人才，體驗從零開始打遍天下無敵手的成就感！信義房屋提供「180天完整教育訓練」，由總部專業講師精心設計系統性培訓課程，整合AI虛實運用提升人才實戰能力，搭配「一對一師徒制」分享實務經驗，信義堅信，人才養成了，服務品質自然到位。\n\n《業界規模實力最大企業》\n→信義房屋為首家房仲業上市公司、實收資本額73.7億，穩健經營。\n→公司財務、資訊透明公開，信義房屋連續11年榮獲證交所公司治理評鑑前5%，與台積電、台灣大並列。\n→台灣公司治理100指數成分股，信義房屋為唯一入選的房仲業者。\n\n多年來信義房屋經營成效卓著，改變了產業生態，成為業界領導品牌，更贏得社會的信任。「提供良好環境，確保同仁獲得就業安全與成長」，是信義房屋一路走來不變的承諾。 從「以人為本」的思考出發，兼顧同仁在經濟上、個人發展上以及身心健康管理的均衡。我們以「吸引優秀人才」、 「營造友善職場」兩大主軸作為持續努力的方向，信義房屋推動以同仁為核心的各項施策，提升勞動環境，以達成我們的核心願景目標：「以人為本」，同仁不只是同仁。", JobCount: 110},
			{EncodedCustNo: "10wuv3fs", Name: "SILICON LABORATORIES ASIA PACIFIC, LIMITED_香港商芯科實驗室亞太有限公司台灣分公司", AreaDesc: "新竹縣竹北市", IndustryDesc: "IC設計相關業", CapitalDesc: "資本額150億", EmployeeCountDesc: "員工數1500人", Profile: "Silicon Labs 創立於1996年，總部设在德州奧斯汀，在全球擁有超過15個分支机构。\n\nSilicon Labs是一家無廠半導體設計公司，與多家業界頂尖製造商如台積電、日月光、矽品、京元电子等合作以完成製造、封裝和測試其開發的芯片。憑藉這些夥伴關係，使公司得以集中資源設計業界领先的物联网芯片及軟體开發。\n\nSilicon Labs長期透過多項業界首創的設計帶動廣大市場的轉型及升級。憑借基于CMOS混合訊號和射频芯片設計、軟體及系統集成之技術優勢，我們提供產品及其综合解決方案以協助客戶简化設計、降低成本並加速其產品上市。", JobCount: 1},
			{EncodedCustNo: "1a2x6bkj5x", Name: "(ASML)台灣艾司摩爾科技股份有限公司", AreaDesc: "新竹市", IndustryDesc: "半導體製造業", CapitalDesc: "資本額18億8000萬元", EmployeeCountDesc: "員工數4500人", Profile: "總部位於荷蘭的ASML (台灣艾司摩爾) 是全球最大晶片微影設備市場的翹楚，為半導體製造商提供微影設備及相關服務，英特爾、三星和台積電等全球頂尖的半導體廠皆為ASML的客戶。ASML是一個國際化的企業，2023年的全球銷售額逾276億歐元，研發投資金額達40億歐元，佔當年度總營收的14.5%，ASML的業務快速成長，躍升為全球市值最大的半導體設備公司。40年來，ASML透過和客戶及供應商的緊密合作，搭配上高效能的營運流程，以及來自全球的優秀員工，逐步開創了我們在晶片微影領域的技術領先地位，協助其設計研發及整合高階系統，開發可用於各類資訊科技產品、行動通訊及物聯網相關產品的晶片，簡單來說，您每天都在使用的電子產品都仰賴我們的設備與技術服務。 \n\nASML聚集全球頂尖人才來服務客戶 \n\n面對摩爾定律所帶來的技術挑戰，ASML 彙集了來自全球物理、電子、機電、軟體與精密技術領域最具有創造力的人才，不斷挑戰技術極限，讓終端消費者能夠用合理的價格買到更強大、更小巧、更便宜和更節能的電子設備，進而提升人類的生活品質。 \n\nGreat Place To Work \nASML致力於為員工打造最佳工作環境，讓全球優秀的工程師樂於在此工作、交流、學習和分享。ASML開放、尊重與創新導向的企業文化，不僅促進員工與同儕及主管間的率直討論、相互學習，更讓ASML能夠持續維持技術領先優勢。\n\n總部位於荷蘭的ASML (台灣艾司摩爾) 是全球最大晶片微影設備市場的翹楚，為半導體製造商提供微影設備及相關服務。在全球16個國家設有超過60個辦公室，員工逾42,000人，來自143個國家。ASML在台灣員工為4,500人，於新竹、台中、台南設有辦公室，並在林口設有智慧製造中心，負責機台翻修與量測設備生產，於台南則設有電子束檢測設備製造中心。\n\nASML Linkou Office - 桃園縣龜山區華亞科技園區科技六路59號 (No.59, Ke Ji 6th Rd., Hwa Ya Technology Park, Gueishan Dist, Taoyuan City)\nASML Hsinchu Office - 新竹市公道五路三段1號11樓 (11F, No. 1, Sec. 3, Gongdao 5th Rd., Hsinchu City)\nASML Taichung Office - 台中市西屯區市政路480號10F（10F., No. 480, Shizheng Rd., Xitun Dist., Taichung City )\nASML Tainan Office - 台南市新市區國際路13號C棟1樓  (1F, building C, No.13, Guoji Rd., Xinshi Dist., Tainan City)\nASML Tainan Factory - 台南市新市區大利一路9號 (No.9, Dali 1st Rd., Xinshi Dist., Tainan City)\n\nASML在阿姆斯特丹泛歐交易所及納斯達克上市，股票代碼＂ASML＂。更多關於ASML及其產品、職缺，請參閱 : www.asml.com\n\n", JobCount: 201},
			{EncodedCustNo: "k46629s", Name: "光洋應用材料科技股份有限公司", AreaDesc: "台南市安南區", IndustryDesc: "其他金屬相關製造業", CapitalDesc: "資本額80億", EmployeeCountDesc: "員工數1246人", Profile: "　　光洋應材創立於1978年，為全球規模最大「磁儲存媒體薄膜靶材製造廠」，致力推動綠色全循環經濟(circular economy)模式，以「新技術、新環保、新材料」因應未來大趨勢，成為ESG領航者。透過領先全球的高純度材料回收再製技術，成為半導體產業的策略合作夥伴，並為光電、資通、石化及消費性產業應用等提供關鍵性的原料、產品與整合型服務方案。主要產品包括：貴金屬化學品∕材料、薄膜濺鍍蒸鍍靶材、特用化學品及資源再生四大類。\n　　主要核心競爭力在於厚實的研發能力，擁有台灣環保署核發之氰化物電鍍廢液回收許可及唯一氰化銀化學品製造執照，更配合政府綠色產業與傳統產業高價值化政策，投資設立電子廢料、石化觸媒廢料及汽車觸媒廢料等的貴金屬回收精鍊廠，發展高附加價值與精密之貴金屬材料產品。\n　　目前光洋集團全球員工約有1,800名，在台灣、香港、中國大陸、歐洲、美洲各地設有辦事處及客戶服務中心，提供全球客戶完備的ICTS(Inside Chamber Total Solution)整合型價值統包服務。光洋應材的營運模式係整合其核心技術以及彈性製造與快速服務能力，不僅為全球客戶提供即時的創新、品質與服務，同時也建構市場最具成本效益的製造能力與動態服務。光洋應材將持續致力於提供獲得客戶肯定之產品技術與服務，以完整滿足客戶在貴金屬整體價值鏈的期盼與需求！\n\n【光洋應材相關報導】\n　(1)三立新聞-光洋應材打入台積電綠色供應鏈\n　https://www.youtube.com/watch?v=UUpHorppnvA\n　(2)遠見雜誌-「光洋應材」如何讓廢料重生、化身夢幻記憶體材料\n　https://reurl.cc/R0KdRn\n　(3)老謝看世界-靶材廠「煉金」躋身台積綠鏈 「城市採礦」3C廢料挖出金山銀山\n　https://www.youtube.com/watch?v=hlFNUbPJqZE\n【循環經濟相關報導】\n　(1)天下雜誌-循環經濟\n　https://topic.cw.com.tw/2016circularkoo\n　(2)經濟日報-光洋科攜手東台、通業技研 打造工具機智慧化生態系\n　https://reurl.cc/L489QL", JobCount: 66},
			{EncodedCustNo: "kixqgrk", Name: "美亞鋼管廠股份有限公司", AreaDesc: "台北市中山區", IndustryDesc: "其他金屬相關製造業", CapitalDesc: "資本額22億2526萬元", EmployeeCountDesc: "員工數330人", Profile: "美亞鋼管廠成立於1959年，為台灣一家專業鋼管製造公司-(股票代號2020)至今經營超過一甲子。「品質與服務」是美亞企業精神的傳承。美亞數十年來對台灣經濟發展有著不可磨滅的貢獻，雖為傳統產業，美亞不但立足台灣，亦拓展至越南及泰國的海外產線，服務東南亞國協等客戶，亦專注先進高端產品製程，除引進高品質製管設備，更增設三倍生產線，憑藉專業技術和高端人才，創造出「價值重於價格」的市場競爭力，深受供應商及客戶的信賴，讓美亞這個品牌成為市場的領頭羊。\n美亞不但在經營上不斷研發及引進最新設備，在管理思維上也以高品質、高效率為主軸。主要產品為各類碳鋼管、不銹鋼管及不銹鋼板材商品。從101大樓到民生住宅、從台積電廠房到捷運工程公共建設、從汽機車的精密管材到運動健身器材管配件，處處都有美亞的足跡。\n美亞從不紙上談兵，循序漸進的強化公司治理、員工福祉、自動化生產、AI輔助管理、ERP建置、ESG的落實，穩健的提升公司整體能效，兼容並具奠定永續發展路徑，也是美亞一路走來的“ＤＮＡ”。", JobCount: 6},
			{EncodedCustNo: "10xpppow", Name: "Kanto-PPC _關東鑫林科技股份有限公司", AreaDesc: "桃園市蘆竹區", IndustryDesc: "化學原料製造業", CapitalDesc: "資本額10億", EmployeeCountDesc: "員工數500人", Profile: "關東鑫林集團為全球電子化學材料領導廠商，提供半導體及光電產業製程所需之超高純度化學品。公司的行銷網路遍及全球半導體及平面顯示器大廠，更被台積電指定於美國設廠，進行全球化佈局。\n\n我們擁有傲視全球的實驗室儀器設備及科技化廠房，致力打造世界第一的半導體材料供應鏈，除了提供一貫高品質的電子化學產品之外，再添加開創性的服務元素，以客戶的角度出發，創造客製化的服務模式。\n\n20年來，我們經歷了技術導向、環境導向、客戶導向的幾個里程碑；未來，關東鑫林集團除將不斷強化核心技術，提供最佳的產品與服務，也希望為改善社會、環境、人類之生活盡一份心力，落實永續投資的概念，達到「綠色化學」的目標。\n\n總公司：338011 桃園市蘆竹區山鼻里機捷路二段935巷51號5樓(距捷運山鼻站約230公尺)\n桃園廠：338025 桃園市蘆竹區海湖東路379號(海湖工業區)\n雲林廠：640104 雲林縣斗六市科加路27號(雲林科技工業區)", JobCount: 50},
		},
		Metadata: struct {
			Pagination struct {
				Total       int `json:"total"`
				CurrentPage int `json:"currentPage"`
				LastPage    int `json:"lastPage"`
			} `json:"pagination"`
		}{
			Pagination: struct {
				Total       int `json:"total"`
				CurrentPage int `json:"currentPage"`
				LastPage    int `json:"lastPage"`
			}{Total: 609, CurrentPage: 1, LastPage: 61},
		},
	}
	assert.Equal(t, want, got)
}

func TestCompanyDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.CompanyDetail(t.Context(), "a5h92m0")
	require.NoError(t, err)

	want := &CompanyDetailResponse{
		Data: CompanyDetail{
			CustName:     "台灣積體電路製造股份有限公司(台積電)",
			CustNo:       "a5h92m0",
			IndustryDesc: "半導體製造業",
			EmpNo:        "暫不提供",
			Capital:      "暫不提供",
			Address:      "新竹市研新二路6號",
			CustLink:     "http://www.tsmc.com.tw",
			Profile:      "台積公司是全世界最大的專業積體電路製造服務公司．\n\n台積公司在民國七十六年成立於台灣新竹科學工業園區，並開創了專業積體電路製造服務商業模式。\n\n台積公司以領先業界的製程技術及設計解決方案組合支援其全球客戶及夥伴生態系統的蓬勃發展，以此釋放全球半導體產業的創新。身為全球的企業公民，台積公司的營運範圍遍及亞洲、歐洲及北美。",
			Product:      "台積公司專注於生產由客戶所設計的晶片，本身並不設計、生產或銷售自有品牌產品，確保絕不與客戶競爭。基於這個創始的原則，台積公司成功的關鍵就在於協助客戶獲得成功。台積公司的專業積體電路製造服務商業模式造就了全球無晶圓廠IC設計產業的崛起。\n\n自創立以來，台積公司一直是世界領先的專業積體電路製造服務公司，單單在民國一百一十三年，台積公司就以288種製程技術，為522個客戶生產1萬1,878種不同產品。\n\n台積公司的眾多客戶遍布全球，為客戶生產的晶片廣泛地被運用在各種終端市場，例如高效能運算、智慧型手機、物聯網、車用電子與消費性電子產品等。",
			Welfare:      "詳見企業網站 https://www.tsmc.com/static/chinese/careers/life_at_tsmc.htm\n投遞履歷請上台積人才招募網站 https://careers.tsmc.com/zh_TW/careers/",
			HRName:       "招募部",
			FollowerCount: 30040,
		},
	}
	assert.Equal(t, want, got)
}

func TestCompanyJobs(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	got, err := c.CompanyJobs(t.Context(), "a5h92m0")
	require.NoError(t, err)

	want := &CompanyJobsResponse{}
	want.Data.List.TopJobs = []CompanyJob{
		{JobName: "Accounting Associate", EncodedJobNo: "77fa2", JobSalaryDesc: "待遇面議", JobAddrNoDesc: "新竹市"},
		{JobName: "身心障礙人才招募專區", EncodedJobNo: "8xxdx", JobSalaryDesc: "待遇面議", JobAddrNoDesc: "新竹縣寶山鄉"},
		{JobName: "先進封裝廠 廠務工程師 AP7(嘉義)", EncodedJobNo: "901pe", JobSalaryDesc: "待遇面議", JobAddrNoDesc: "嘉義縣太保市"},
	}
	want.Data.List.NormalJobs = []CompanyJob{
		{JobName: "儲備模組副工程師 (台南)", EncodedJobNo: "76avm", JobSalaryDesc: "待遇面議", JobAddrNoDesc: "台南市善化區"},
		{JobName: "新建廠工程隊 助理工程師", EncodedJobNo: "8vgi6", JobSalaryDesc: "月薪33,800以上", JobAddrNoDesc: "嘉義縣太保市"},
		{JobName: "廠務助理工程師(裝機工程)", EncodedJobNo: "7g7h8", JobSalaryDesc: "月薪34,400以上", JobAddrNoDesc: "台南市善化區"},
		{JobName: "實驗室技術員", EncodedJobNo: "78he6", JobSalaryDesc: "月薪32,000~43,000元", JobAddrNoDesc: "新竹市"},
		{JobName: "南科電子束作業處技術員", EncodedJobNo: "7z50l", JobSalaryDesc: "月薪32,000~43,000元", JobAddrNoDesc: "台南市善化區"},
		{JobName: "3DIC flow engineer", EncodedJobNo: "74kr7", JobSalaryDesc: "待遇面議", JobAddrNoDesc: "新竹市"},
		{JobName: "A10/A14 RD Integration Engineer", EncodedJobNo: "6o9z0", JobSalaryDesc: "待遇面議", JobAddrNoDesc: "新竹市"},
		{JobName: "IT AI/ML Engineer", EncodedJobNo: "759k7", JobSalaryDesc: "待遇面議", JobAddrNoDesc: "新竹市"},
		{JobName: "APR Engineer", EncodedJobNo: "6pto6", JobSalaryDesc: "待遇面議", JobAddrNoDesc: "新竹市"},
		{JobName: "Backend Software Engineer", EncodedJobNo: "759lt", JobSalaryDesc: "待遇面議", JobAddrNoDesc: "新竹市"},
	}
	assert.Equal(t, want, got)
}
