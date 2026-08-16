package mynavi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var wantFirstJob = Job{
	ID:               "348855-1-29-1",
	Title:            "【ITエンジニア】還元率80%超！フルリモOK☆初年度年収420万円～",
	Company:          "ウィンヴォルブ株式会社",
	CatchCopy:        "《上場企業グループ》★還元率80％★残業月5h以下★年休130日～",
	EmploymentStatus: "正社員",
	Conditions: []string{
		"職種・業種未経験OK",
		"転勤なし",
		"学歴不問",
		"完全週休2日制",
		"第二新卒歓迎",
		"リモートワーク可",
	},
	Description:     "【収入・働き方・ポジションなど、希望にマッチした案件をご紹介】システム開発・インフラ構築など豊富な案件あり！《将来のキャリアパスも選べる》",
	Target:          "【フルリモート可／全国募集】実務未経験でもOK◎「エンジニア経験」または「プログラミングのスキル」がある方★学歴不問・第二新卒歓迎！",
	Location:        "【フルリモートOK！／転勤なし】 ★東京・大阪・札幌・名古屋・福岡 ※希望を考慮し決定いたします。…",
	Salary:          "◆月給35万円〜110万円＜入社時から年収200万円UP実現多数！前職給与を100％保証！還元率8…",
	FirstYearIncome: "420万円～1000万円",
	UpdatedDate:     "2026/07/15",
	EndDate:         "2026/07/30",
}

func TestJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{Keywords: "Python"})
	require.NoError(t, err)

	assert.Equal(t, 2111, got.Total)
	require.Len(t, got.Jobs, 50)
	assert.Equal(t, wantFirstJob, got.Jobs[0])
}

func TestJobsZeroHits(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Jobs(t.Context(), &JobsRequest{Keywords: "zzzqqqxyzabc"})
	require.NoError(t, err)

	assert.Equal(t, 0, got.Total)
	assert.Empty(t, got.Jobs)
}

func TestJobsRejectsBadRequests(t *testing.T) {
	c := NewClient("https://example.invalid", http.DefaultClient)

	_, err := c.Jobs(t.Context(), &JobsRequest{MinSalary: 720})
	assert.ErrorContains(t, err, "720")

	_, err = c.Jobs(t.Context(), &JobsRequest{Keywords: "TCP/IP"})
	assert.ErrorContains(t, err, "TCP/IP")

	_, err = c.Jobs(t.Context(), &JobsRequest{Page: -1})
	assert.ErrorContains(t, err, "1-based")
}

func TestJobsURL(t *testing.T) {
	c := NewClient("https://tenshoku.mynavi.jp", nil)

	for _, tc := range []struct {
		req  JobsRequest
		want string
	}{
		{JobsRequest{}, "https://tenshoku.mynavi.jp/list/"},
		{JobsRequest{Keywords: "Python"}, "https://tenshoku.mynavi.jp/list/kwPython/"},
		{JobsRequest{Keywords: "Python AWS"}, "https://tenshoku.mynavi.jp/list/kwPython%20AWS/"},
		{JobsRequest{Keywords: "Go言語"}, "https://tenshoku.mynavi.jp/list/kwGo%E8%A8%80%E8%AA%9E/"},
		{JobsRequest{Keywords: "Python", MinSalary: 700, Page: 2}, "https://tenshoku.mynavi.jp/list/min0700/kwPython/pg2/"},
		{JobsRequest{Page: 1}, "https://tenshoku.mynavi.jp/list/"},
	} {
		got, err := c.jobsURL(&tc.req)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "348855-1-29-1")
	require.NoError(t, err)

	assert.Equal(t, "348855-1-29-1", got.ID)
	assert.Equal(t, "https://tenshoku.mynavi.jp/jobinfo-348855-1-29-1/", got.URL)
	assert.Equal(t, "ITエンジニア／システムエンジニア（アプリ設計／WEB・オープン・モバイル系）", got.Title)
	assert.Equal(t, "ウィンヴォルブ株式会社", got.Company)
	assert.Equal(t, "https://winvolve.co.jp/", got.CompanyURL)
	assert.Equal(t, "FULL_TIME", got.EmploymentType)
	assert.Equal(t, "ソフトウェア・情報処理／インターネット関連／サービス（その他）／専門コンサルタント", got.Industry)
	assert.Equal(t, "システムエンジニア（アプリ設計／WEB・オープン・モバイル系）", got.OccupationalCategory)
	assert.Equal(t, "2026-07-03", got.DatePosted)
	assert.Equal(t, "2026-07-30", got.ValidThrough)
	require.Len(t, got.Locations, 47) // nationwide-remote posting lists every prefecture
	assert.Equal(t, Location{Region: "北海道", Locality: "札幌市"}, got.Locations[0])
	assert.Equal(t, Location{Region: "青森県"}, got.Locations[1])
	assert.Equal(t, "JPY", got.SalaryCurrency)
	assert.Equal(t, "4200000", got.SalaryMin)
	assert.Equal(t, "10000000", got.SalaryMax)
	assert.Equal(t, "YEAR", got.SalaryUnit)
	assert.Contains(t, got.Description, "この求人のポイント")
	assert.NotContains(t, got.Description, "<br>", "description should be plain text")
	assert.NotContains(t, got.Description, "&lt;br&gt;", "double-escaped upstream <br> artifacts should be flattened")
	assert.Contains(t, got.ExperienceRequirements, "エンジニア経験者")
	assert.Contains(t, got.WorkHours, "9:00～18:00")
	assert.Contains(t, got.JobBenefits, "健康診断")
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), MockNotFoundJobID)
	assert.ErrorContains(t, err, "404")
}

func TestJobDetailMalformedID(t *testing.T) {
	c := NewClient("https://example.invalid", http.DefaultClient)

	for _, id := range []string{"", "abc", "123", "123-4-5", "1-2-3-4-5", "../etc"} {
		_, err := c.JobDetail(t.Context(), id)
		assert.Error(t, err, "id %q", id)
	}
}
