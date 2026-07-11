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
			{JobNo: NewNilString("10177057"), JobName: NewNilString("GoLang Developer"), CustName: NewNilString("曜驊智能股份有限公司"), CustNo: NewNilString("130000000042972"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/624o1", Cust: NewNilString("https://www.104.com.tw/company/1a2x6biwgs")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市內湖區"), AppearDate: NewNilString("20260515"), ApplyCnt: NewNilInt(3), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(0)},
			{JobNo: NewNilString("15015281"), JobName: NewNilString("Golang 後端工程師"), CustName: NewNilString("富一代資訊有限公司"), CustNo: NewNilString("130000000264142"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8xtv5", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bnn4e")}, SalaryHigh: NewNilInt(120000), SalaryLow: NewNilInt(60000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市松山區"), AppearDate: NewNilString("20260609"), ApplyCnt: NewNilInt(8), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(0)},
			{JobNo: NewNilString("11282518"), JobName: NewNilString("Golang 工程師"), CustName: NewNilString("百阜科技股份有限公司"), CustNo: NewNilString("130000000112061"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/6ptna", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bkdrx")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市內湖區"), AppearDate: NewNilString("20260526"), ApplyCnt: NewNilInt(4), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(2)},
			{JobNo: NewNilString("12689685"), JobName: NewNilString("Senior Cloud Backend Engineer (Golang)"), CustName: NewNilString("華玉科技股份有限公司"), CustNo: NewNilString("130000000180812"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/7jzf9", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bluto")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市大安區"), AppearDate: NewNilString("20260623"), ApplyCnt: NewNilInt(6), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(6)},
			{JobNo: NewNilString("14271913"), JobName: NewNilString("軟體工程師 Golang"), CustName: NewNilString("線上探索科技股份有限公司"), CustNo: NewNilString("130000000147477"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8hwa1", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bl53p")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市大同區"), AppearDate: NewNilString("20260304"), ApplyCnt: NewNilInt(6), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(2)},
			{JobNo: NewNilString("15160106"), JobName: NewNilString("Software Engineer (Golang, Flutter), Virtual insurance"), CustName: NewNilString("香港商六度科技有限公司"), CustNo: NewNilString("130000000161268"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/90xm2", Cust: NewNilString("https://www.104.com.tw/company/1a2x6blfqs")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市信義區"), AppearDate: NewNilString("20260618"), ApplyCnt: NewNilInt(5), RemoteWorkType: NewNilInt(2), JobRo: NewNilInt(1), Period: NewNilInt(0)},
			{JobNo: NewNilString("13305625"), JobName: NewNilString("Golang開發工程師"), CustName: NewNilString("太禾科技有限公司"), CustNo: NewNilString("130000000177509"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/7x6op", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bls9x")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市中山區"), AppearDate: NewNilString("20260618"), ApplyCnt: NewNilInt(5), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(4)},
			{JobNo: NewNilString("14954565"), JobName: NewNilString("Golang 後端工程師 / Golang Backend Engineer"), CustName: NewNilString("炫石有限公司"), CustNo: NewNilString("130000000241271"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8wj0l", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bn5h3")}, SalaryHigh: NewNilInt(9999999), SalaryLow: NewNilInt(60000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市信義區"), AppearDate: NewNilString("20260511"), ApplyCnt: NewNilInt(8), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(4)},
			{JobNo: NewNilString("14893390"), JobName: NewNilString("Golang開發工程師"), CustName: NewNilString("四天科技有限公司"), CustNo: NewNilString("130000000231318"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8v7ta", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bmxsm")}, SalaryHigh: NewNilInt(150000), SalaryLow: NewNilInt(80000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市中山區"), AppearDate: NewNilString("20260622"), ApplyCnt: NewNilInt(8), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(3)},
			{JobNo: NewNilString("14953361"), JobName: NewNilString("【擴編】資深Golang後端工程師 / Senior Golang Developer"), CustName: NewNilString("瑞典商英鉑科股份有限公司台灣分公司"), CustNo: NewNilString("130000000217988"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8wi35", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bmnic")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市中山區"), AppearDate: NewNilString("20260622"), ApplyCnt: NewNilInt(4), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(6)},
			{JobNo: NewNilString("15115498"), JobName: NewNilString("軟體工程師 (Software Engineer - Golang)"), CustName: NewNilString("立視科技股份有限公司"), CustNo: NewNilString("130000000266972"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8zz6y", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bnpb0")}, SalaryHigh: NewNilInt(88000), SalaryLow: NewNilInt(55000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市松山區"), AppearDate: NewNilString("20260625"), ApplyCnt: NewNilInt(4), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(4)},
			{JobNo: NewNilString("14439753"), JobName: NewNilString("GOLANG 開發工程師"), CustName: NewNilString("益晨資訊科技有限公司"), CustNo: NewNilString("130000000221207"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8lhs9", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bmpzr")}, SalaryHigh: NewNilInt(90000), SalaryLow: NewNilInt(72000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市中正區"), AppearDate: NewNilString("20260625"), ApplyCnt: NewNilInt(7), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(4)},
			{JobNo: NewNilString("13761398"), JobName: NewNilString("Senior Backend Engineer ( Golang )（每月有遠端日）"), CustName: NewNilString("幣託科技股份有限公司"), CustNo: NewNilString("130000000223436"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/86yd2", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bmrpo")}, SalaryHigh: NewNilInt(150000), SalaryLow: NewNilInt(85000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市松山區"), AppearDate: NewNilString("20260622"), ApplyCnt: NewNilInt(10), RemoteWorkType: NewNilInt(2), JobRo: NewNilInt(1), Period: NewNilInt(6)},
			{JobNo: NewNilString("15097562"), JobName: NewNilString("後端工程師（Golang）"), CustName: NewNilString("米奈娛樂有限公司"), CustNo: NewNilString("130000000251337"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8zlcq", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bnd8p")}, SalaryHigh: NewNilInt(80000), SalaryLow: NewNilInt(70000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市大安區"), AppearDate: NewNilString("20260620"), ApplyCnt: NewNilInt(7), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(3)},
			{JobNo: NewNilString("14335204"), JobName: NewNilString("Golang後端與DevOps工程師"), CustName: NewNilString("時刻無限股份有限公司"), CustNo: NewNilString("130000000242671"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8j944", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bn6jz")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市大安區"), AppearDate: NewNilString("20260622"), ApplyCnt: NewNilInt(8), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(2)},
			{JobNo: NewNilString("14660408"), JobName: NewNilString("Golang 遊戲開發工程師(大安)"), CustName: NewNilString("天晴資訊有限公司"), CustNo: NewNilString("130000000167545"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8q81k", Cust: NewNilString("https://www.104.com.tw/company/1a2x6blkl5")}, SalaryHigh: NewNilInt(95000), SalaryLow: NewNilInt(50000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市大安區"), AppearDate: NewNilString("20260623"), ApplyCnt: NewNilInt(7), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(0)},
			{JobNo: NewNilString("15245367"), JobName: NewNilString("Golang Engineer"), CustName: NewNilString("瞬聯科技股份有限公司"), CustNo: NewNilString("130000000159109"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/92ref", Cust: NewNilString("https://www.104.com.tw/company/1a2x6ble2t")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市大安區"), AppearDate: NewNilString("20260623"), ApplyCnt: NewNilInt(20), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(2)},
			{JobNo: NewNilString("13766806"), JobName: NewNilString("【純遠端】國際遊戲公司 誠徵  Go/Golang 工程師"), CustName: NewNilString("台灣英特艾倫人力資源有限公司"), CustNo: NewNilString("130000000048447"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/872ja", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bj0ov")}, SalaryHigh: NewNilInt(180000), SalaryLow: NewNilInt(150000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市中山區"), AppearDate: NewNilString("20260623"), ApplyCnt: NewNilInt(12), RemoteWorkType: NewNilInt(1), JobRo: NewNilInt(1), Period: NewNilInt(6)},
			{JobNo: NewNilString("14645682"), JobName: NewNilString("Golang 後端工程師(大安)"), CustName: NewNilString("天晴資訊有限公司"), CustNo: NewNilString("130000000167545"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8pwoi", Cust: NewNilString("https://www.104.com.tw/company/1a2x6blkl5")}, SalaryHigh: NewNilInt(9999999), SalaryLow: NewNilInt(50000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市大安區"), AppearDate: NewNilString("20260623"), ApplyCnt: NewNilInt(5), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(4)},
			{JobNo: NewNilString("15043542"), JobName: NewNilString("【TENG0502】Software Engineer (Backend) - Golang / RoR"), CustName: NewNilString("喬富科技股份有限公司"), CustNo: NewNilString("130000000264905"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8yfo6", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bnnpl")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市松山區"), AppearDate: NewNilString("20260623"), ApplyCnt: NewNilInt(10), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(3)},
			{JobNo: NewNilString("14525012"), JobName: NewNilString("Golang工程師-Junior"), CustName: NewNilString("彼雅特科技股份有限公司"), CustNo: NewNilString("130000000220505"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8nbkk", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bmpg9")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市信義區"), AppearDate: NewNilString("20260529"), ApplyCnt: NewNilInt(6), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(0)},
			{JobNo: NewNilString("14797877"), JobName: NewNilString("[資訊部]Golang工程師"), CustName: NewNilString("虹耀建設股份有限公司"), CustNo: NewNilString("130000000145239"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8t645", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bl3dj")}, SalaryHigh: NewNilInt(9999999), SalaryLow: NewNilInt(75000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市中正區"), AppearDate: NewNilString("20260622"), ApplyCnt: NewNilInt(6), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(6)},
			{JobNo: NewNilString("14965947"), JobName: NewNilString("資深後端工程師（Golang / Java） / Senior Backend Engineer（Golang / Java）"), CustName: NewNilString("炫石有限公司"), CustNo: NewNilString("130000000241271"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8wrsr", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bn5h3")}, SalaryHigh: NewNilInt(9999999), SalaryLow: NewNilInt(60000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市信義區"), AppearDate: NewNilString("20260511"), ApplyCnt: NewNilInt(2), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(4)},
			{JobNo: NewNilString("14935253"), JobName: NewNilString("【擴編】Golang後端工程師/ Golang Developer"), CustName: NewNilString("瑞典商英鉑科股份有限公司台灣分公司"), CustNo: NewNilString("130000000217988"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8w445", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bmnic")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市中山區"), AppearDate: NewNilString("20260622"), ApplyCnt: NewNilInt(9), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(4)},
			{JobNo: NewNilString("14408054"), JobName: NewNilString("後端工程師-Golang-台北"), CustName: NewNilString("立特有限公司"), CustNo: NewNilString("130000000211187"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8ktbq", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bmi9f")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市中山區"), AppearDate: NewNilString("20260622"), ApplyCnt: NewNilInt(10), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(3)},
			{JobNo: NewNilString("15106548"), JobName: NewNilString("Senior Backend Engineer (Golang), Virtual insurance"), CustName: NewNilString("香港商六度科技有限公司"), CustNo: NewNilString("130000000161268"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8zsac", Cust: NewNilString("https://www.104.com.tw/company/1a2x6blfqs")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市信義區"), AppearDate: NewNilString("20260622"), ApplyCnt: NewNilInt(12), RemoteWorkType: NewNilInt(2), JobRo: NewNilInt(1), Period: NewNilInt(0)},
			{JobNo: NewNilString("15139656"), JobName: NewNilString("Golang 後端工程師"), CustName: NewNilString("昕展資訊有限公司"), CustNo: NewNilString("130000000261162"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/90hu0", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bnktm")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市中山區"), AppearDate: NewNilString("20260623"), ApplyCnt: NewNilInt(10), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(0)},
			{JobNo: NewNilString("14946666"), JobName: NewNilString("後端工程師 (Backend Engineer - Golang)"), CustName: NewNilString("開端智能股份有限公司"), CustNo: NewNilString("130000000255283"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8wcx6", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bngab")}, SalaryHigh: NewNilInt(80000), SalaryLow: NewNilInt(50000), S10: NewNilInt(50), JobAddrNoDesc: NewNilString("台北市松山區"), AppearDate: NewNilString("20260626"), ApplyCnt: NewNilInt(17), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(0)},
			{JobNo: NewNilString("13903564"), JobName: NewNilString("Backend Engineer(Java or Golang)"), CustName: NewNilString("重高科技股份有限公司"), CustNo: NewNilString("130000000227435"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8a024", Cust: NewNilString("https://www.104.com.tw/company/1a2x6bmusr")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市大安區"), AppearDate: NewNilString("20260622"), ApplyCnt: NewNilInt(17), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(3)},
			{JobNo: NewNilString("14115841"), JobName: NewNilString("Golang 網站開發工程師(Backend)_零售解決方案課"), CustName: NewNilString("日本NEC集團_統智科技股份有限公司"), CustNo: NewNilString("12876266000"), Link: JobSummaryLink{Job: "https://www.104.com.tw/job/8ejup", Cust: NewNilString("https://www.104.com.tw/company/5wy72fk")}, SalaryHigh: NewNilInt(0), SalaryLow: NewNilInt(0), S10: NewNilInt(10), JobAddrNoDesc: NewNilString("台北市內湖區"), AppearDate: NewNilString("20260612"), ApplyCnt: NewNilInt(19), RemoteWorkType: NewNilInt(0), JobRo: NewNilInt(1), Period: NewNilInt(2)},
		},
		Metadata: JobsResponseMetadata{
			Pagination: JobsResponseMetadataPagination{CurrentPage: NewNilInt(1), LastPage: NewNilInt(7), Total: NewNilInt(189)},
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
				JobName:    NewNilString("軟體工程師 (數位工程發展部)"),
				CustName:   NewNilString("亞新工程顧問股份有限公司"),
				CustUrl:    NewNilString("https://www.104.com.tw/company/264c9zc"),
				AppearDate: NewNilString("2026/06/22"),
				IsSaved:    NewNilBool(false),
				IsApplied:  NewNilBool(false),
			},
			Contact: JobDetailContact{
				HrName: NewOptNilString("Rachel Chiu 邱小姐"),
				Email:  NewOptNilString("personnel@maaconsultants.com,cj.yu@maaconsultants.com,eugene.shen@maaconsultants.com,fred.chou@maaconsultants.com"),
				Reply:  NewOptNilString(""),
			},
			Condition: JobDetailCondition{
				WorkExp: NewOptNilString("不拘"),
				Edu:     NewOptNilString("大學以上"),
				Major:   []string{"資訊工程相關"},
				Specialty: []CodeDescription{
					{Code: NewOptNilString("12001003009"), Description: NewOptNilString("C#")},
					{Code: NewOptNilString("12001003006"), Description: NewOptNilString("ASP.NET")},
					{Code: NewOptNilString("12001004031"), Description: NewOptNilString("MS SQL")},
					{Code: NewOptNilString("12001003045"), Description: NewOptNilString("Python")},
					{Code: NewOptNilString("12003001003"), Description: NewOptNilString("GIS")},
					{Code: NewOptNilString("12001003094"), Description: NewOptNilString("IoT")},
					{Code: NewOptNilString("12002003010"), Description: NewOptNilString("Revit")},
				},
			},
			Welfare: JobDetailWelfare{
				Welfare: NewOptNilString("在亞新，我們重視同仁的職涯成長與友善職場，透過全方位的福利與支持，推動以人為本、永續發展的職場環境，實現工作與生活的和諧平衡。\n\n【薪酬與獎金】\n  •  具市場競爭力的薪資水準\n  •  年節獎金與專案獎金，共享成果回饋\n\n【健康與保障】\n  •  勞健保及完整團體保險(意外、醫療、重大疾病、職災保障)\n  •  定期健康檢查、健康講座與員工關懷方案\n\n【休假與彈性】\n  •  彈性上下班、育兒友善措施，兼顧生活平衡\n\n【教育訓練與發展】\n  •  完善新人培訓與師徒制\n  •  E-learning 線上學習資源\n  •  專業證照補助（如 PMP、專業技師等）\n  •  外部訓練與國際研討會，拓展國際視野\n  •  參與國家級重大工程，累積獨特專業經驗\n\n【生活與休閒】\n  •  福委會關懷：生日禮金、節慶禮品或禮券、婚喪喜慶、傷病住院慰問與生育補助\n  •  部門聚餐、咖啡分享日、社團活動、Happy Hour，促進交流與凝聚力\n  •  舒適職場環境：明亮開放空間、零食吧、茶包與自助研磨咖啡機\n\n【招募流程】\n  1. 投遞履歷\n  2. HR初審履歷 → 部門主管面試\n  3. Final面談（含專案介紹與Q&A）\n  4. 錄取通知\n （流程清楚透明，讓你安心應徵!)"),
			},
			JobDetail: JobDetailJobDetail{
				JobDescription: NewOptNilString("無相關經驗可，大學以上資訊工程、資訊管理等相關科系畢業\n\n【工作內容】\n- 參與智慧工程數位平台的設計、開發與維運\n- 開發與維護 GIS、BIM 系統，並支援無人機地形數據應用\n- 參與 AI 工具與文件管理系統之開發 \n- 與跨領域團隊合作（工程、IoT、BIM、AI），推動數位轉型與自動化流程\n\n【希望條件】\n- 熟悉現代軟體系統研發流程與版本控制\n- 熟悉至少一種指令式程式設計語言（C#、JavaScript、Python、PHP 尤佳）\n- 具 ASP.NET、SQL、Vue.js、Laravel、Unity、GIS、IoT、Revit等開發經驗\n- 具軟體設計、開發、運營、開發、機器學習、AI 模型訓練 (Finetuning)、 AI 應用設計（OCR、RAG、LLM、Agentic 等）開發、導入經驗\n- 具 Azure DevOps、Docker、Kubernetes 經驗者優先\n\n＊我們期待具備高度邏輯思維、善於溝通系統需求與設計選擇，並能獨立完成軟體開發的夥伴加入，一起參與系統規劃與優化。"),
				JobCategory: []CodeDescription{
					{Code: NewOptNilString("2007001004"), Description: NewOptNilString("軟體工程師")},
				},
				Salary:        NewOptNilString("待遇面議"),
				SalaryMin:     NewOptNilInt(0),
				SalaryMax:     NewOptNilInt(0),
				AddressRegion: NewOptNilString("新北市汐止區"),
				AddressDetail: NewOptNilString("新台五路一段112號22樓"),
				ManageResp:    NewOptNilString("不需負擔管理責任"),
				NeedEmp:       NewOptNilString("2~3人"),
				JobType:       NewOptNilInt(1),
				RemoteWork:    OptNilJobDetailJobDetailRemoteWork{Null: true, Set: true},
			},
			Industry:  NewNilString("建築及工程技術服務業"),
			Employees: NewNilString("1200人"),
			CustNo:    NewNilString("264c9zc"),
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
		Response:   ErrorResponse{Message: NewOptNilString("internal error"), AdditionalProps: ErrorResponseAdditional{}},
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
		Response:   ErrorResponse{Message: NewOptNilString("job not found"), AdditionalProps: ErrorResponseAdditional{}},
	}
	assert.Equal(t, want, ue)
}
