package mediatek

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchFiltered(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Search(t.Context(), SearchRequest{
		Categories: []string{"9020"},
		Locations:  []string{"0000009256"},
	})
	require.NoError(t, err)
	assert.Equal(t, 31, got.Pagination.TotalItems)
	require.Len(t, got.Jobs, 6)
	assert.Equal(t, "MTK120260629001", got.Jobs[0].ID)
	assert.Equal(t, "AI Software Tool Engineer", got.Jobs[0].Title)
	assert.Equal(t, "Software", got.Jobs[0].Category)
	assert.Equal(t, "9020", got.Jobs[0].CategoryCode)
	assert.Equal(t, "More than 2 Years Work Expe.", got.Jobs[0].WorkExperience)
	assert.Equal(t, "0003", got.Jobs[0].WorkExperienceCode)
	assert.Equal(t, "HsinChu", got.Jobs[0].Location)
	assert.Equal(t, "0000009256", got.Jobs[0].LocationCode)
}

func TestSearchKeyword(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Search(t.Context(), SearchRequest{Keyword: "AI"})
	require.NoError(t, err)
	assert.Equal(t, 554, got.Pagination.TotalItems)
	require.Len(t, got.Jobs, 6)
	assert.Equal(t, "Sr. Manager – Data Center & B2B AI Marketing (US & Europe)", got.Jobs[0].Title)
}

func TestSearchEmpty(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Search(t.Context(), SearchRequest{Keyword: "__mtk_no_such_job_20260720__"})
	require.NoError(t, err)
	assert.Empty(t, got.Jobs)
	assert.Equal(t, 0, got.Pagination.TotalItems)
}

func TestJobDetail(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), "MTK120220511000")
	require.NoError(t, err)
	assert.Equal(t, &JobDetail{
		ID:             "MTK120220511000",
		URL:            JobURL(srv.URL, "MTK120220511000"),
		Title:          "Web Front-End Engineer (contractor)",
		Category:       "Software",
		Location:       "HsinChu",
		Experience:     "No Work Expe.",
		Education:      "Bachelor's Degree",
		Description:    "•\t負責 Web 前端元件開發並完成交付\n•\t與 UI/UX designer 合作，制定使用者介面\n•\t與 PM 確認系統需求與操作流程，以實作商業邏輯\n•\t與 Backend engineer 合作，進行 API 整合與開發\n•\t頁面效能調校、持續更新前端元件架構",
		Qualifications: "#必備條件\n•\t熟悉 React / Angular / Vue 任一框架\n•\t熟悉 HTML5 / CSS3 / ES6+\n•\t熟悉 Git 版控\n#加分條件\n•\tReact hooks\n•\tGraphQL (Apollo)\n•\tTypescript\n•\tMaterial UI\n•\tJest / Cypress\n•\tNext.js\n•\t有網站效能優化經驗\n•\t善於溝通與團隊合作\n•\t能獨立作業完成專案\n•\t熟悉 Agile 開發方式\n•\t熟悉 CI/CD 工具\n•\tFunctional programming\n",
	}, got)
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), "MTK999999999999")
	assert.ErrorContains(t, err, "HTTP 500")
}

func TestJobDetailRejectsInvalidID(t *testing.T) {
	c := NewClient("https://careers.mediatek.com", nil)
	for _, id := range []string{"", "123", "MTK-1", "MTK123/4"} {
		_, err := c.JobDetail(t.Context(), id)
		assert.ErrorContains(t, err, "expected company prefix followed by digits")
	}
}

func TestSearchValidation(t *testing.T) {
	c := NewClient("https://careers.mediatek.com", nil)
	_, err := c.Search(t.Context(), SearchRequest{Page: -1})
	assert.ErrorContains(t, err, "page must be >= 1")
	_, err = c.Search(t.Context(), SearchRequest{Limit: 101})
	assert.ErrorContains(t, err, "limit must be between 1 and 100")
}

func TestSearchURL(t *testing.T) {
	c := NewClient("https://careers.mediatek.com", nil)
	raw, err := c.searchURL(searchInput{
		Locale: "en_US",
		Page:   2,
		Query:  queryInput{Keywords: []string{"software", "engineer"}, Relation: "AND"},
		Filters: searchFilters{
			Categories: []string{"9020"},
			Locations:  []string{"0000009256"},
		},
		SortBy: "publishedDate",
		Order:  "DESC",
		Limit:  6,
	})
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	var input struct {
		JSON searchInput `json:"json"`
	}
	require.NoError(t, json.Unmarshal([]byte(u.Query().Get("input")), &input))
	assert.Equal(t, 2, input.JSON.Page)
	assert.Equal(t, []string{"software", "engineer"}, input.JSON.Query.Keywords)
	assert.Equal(t, []string{"9020"}, input.JSON.Filters.Categories)
	assert.Equal(t, []string{"0000009256"}, input.JSON.Filters.Locations)
}

func TestQueryInfo(t *testing.T) {
	assert.Equal(t, queryInput{Keywords: []string{}, Relation: "AND"}, queryInfo(""))
	assert.Equal(t, queryInput{Keywords: []string{"software", "engineer"}, Relation: "AND"}, queryInfo("software AND engineer"))
	assert.Equal(t, queryInput{Keywords: []string{"software", "engineer"}, Relation: "OR"}, queryInfo("software OR engineer"))
}

func TestJobURL(t *testing.T) {
	assert.Equal(t, "https://careers.mediatek.com/en/jobs/MTK120220511000", JobURL("https://careers.mediatek.com/", "MTK120220511000"))
}
