package ats

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/engage"
)

func testEngageAdapter(t *testing.T) *EngageAdapter {
	t.Helper()
	srv := engage.NewMockServer()
	t.Cleanup(srv.Close)
	a := NewEngageAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = srv.URL
	return a
}

// engageAllJobs pages through a search so assertions are not truncated by
// pageSize.
func engageAllJobs(t *testing.T, a *EngageAdapter, slug string, p SearchParams) []JobSummary {
	t.Helper()
	var jobs []JobSummary
	for page := 1; ; page++ {
		p.Page = page
		res, err := a.Search(t.Context(), slug, p)
		require.NoError(t, err)
		jobs = append(jobs, res.Jobs...)
		if page >= res.TotalPages {
			require.Len(t, jobs, res.TotalCount)
			return jobs
		}
	}
}

func TestEngageRoster(t *testing.T) {
	a := testEngageAdapter(t)
	assert.Len(t, a.Roster(), len(engage.Companies))
}

func TestEngageParseCareersURL(t *testing.T) {
	a := testEngageAdapter(t)

	tests := []struct {
		name     string
		rawURL   string
		wantSlug string
		wantOK   bool
	}{
		{"board", "https://en-gage.net/nova_career/", "nova_career", true},
		{"board without trailing slash", "https://en-gage.net/nova_career", "nova_career", true},
		{"posting", "https://en-gage.net/nova_career/work_17046487/", "nova_career", true},
		{"posting with query", "https://en-gage.net/nova_career/work_17046487/?via_recruit_page=1", "nova_career", true},
		{"hyphenated slug", "https://en-gage.net/aspark-tokyo/", "aspark-tokyo", true},
		{"numeric slug", "https://en-gage.net/2918/", "2918", true},
		{"uppercase host", "https://EN-GAGE.NET/nova_career/", "nova_career", true},

		// Site-owned paths are not tenants.
		{"aggregator search", "https://en-gage.net/user/search/", "", false},
		{"job desc page", "https://en-gage.net/user/search/desc/17046487/", "", false},
		{"sitemap index", "https://en-gage.net/sitemap_index.xml", "", false},
		{"static assets", "https://en-gage.net/common_user/css/dev/search.css", "", false},
		{"site root", "https://en-gage.net/", "", false},

		// Other hosts.
		{"other host", "https://herp.careers/careers/companies/notainc", "", false},
		{"lookalike host", "https://en-gage.net.evil.example/nova_career/", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.rawURL)
			require.NoError(t, err)

			slug, ok := a.ParseCareersURL(u)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantSlug, slug)
		})
	}
}

// TestEngageSearchSpansEveryCategory pins the recon error corrected during
// implementation: each employment category is its own dl.jobList, so a parser
// reading only the first one silently drops the rest. The captured nova_career
// board has two categories.
func TestEngageSearchSpansEveryCategory(t *testing.T) {
	a := testEngageAdapter(t)

	jobs := engageAllJobs(t, a, engage.MockSlug, SearchParams{})
	assert.Greater(t, len(jobs), engage.CategoryCap,
		"a board whose first category is at the cap must still contribute its later categories")

	filters, err := a.Filters(t.Context(), engage.MockSlug)
	require.NoError(t, err)
	assert.Len(t, filters["employmentType"], 2)
}

func TestEngageSearchSummaries(t *testing.T) {
	a := testEngageAdapter(t)

	res, err := a.Search(t.Context(), engage.MockSlug, SearchParams{})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)

	for _, j := range res.Jobs {
		assert.NotEmpty(t, j.JobID)
		assert.NotEmpty(t, j.Title)
		assert.True(t, strings.HasSuffix(j.URL, "/work_"+j.JobID+"/"),
			"URL must address the posting by its own work id, got %q", j.URL)
	}
}

// TestEngageSearchMinimalBoard covers the smallest board engage serves. It is
// the closest thing to an empty board: engage publishes no zero-job tenants.
func TestEngageSearchMinimalBoard(t *testing.T) {
	a := testEngageAdapter(t)

	res, err := a.Search(t.Context(), engage.MockMinimalSlug, SearchParams{})
	require.NoError(t, err)
	assert.Len(t, res.Jobs, 1)
	assert.Equal(t, 1, res.TotalCount)
}

// TestEngageSearchAtCapIsLowerBound documents the ceiling's effect on the
// unified result: TotalCount reports what the board gave, which for a capped
// tenant is fewer jobs than the tenant actually has.
func TestEngageSearchAtCapIsLowerBound(t *testing.T) {
	a := testEngageAdapter(t)

	res, err := a.Search(t.Context(), engage.MockCapSlug, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, engage.CategoryCap, res.TotalCount)
}

func TestEngageSearchQueryAndFilter(t *testing.T) {
	a := testEngageAdapter(t)

	all := engageAllJobs(t, a, engage.MockSlug, SearchParams{})
	require.NotEmpty(t, all)

	filters, err := a.Filters(t.Context(), engage.MockSlug)
	require.NoError(t, err)
	require.NotEmpty(t, filters["employmentType"])

	// Filtering to one employment type must be a strict subset of the board.
	one := filters["employmentType"][0]
	filtered := engageAllJobs(t, a, engage.MockSlug, SearchParams{
		Filters: FilterSet{"employmentType": {one}},
	})
	assert.NotEmpty(t, filtered)
	assert.Less(t, len(filtered), len(all))

	// An unknown filter key is rejected rather than silently ignored.
	_, err = a.Search(t.Context(), engage.MockSlug, SearchParams{
		Filters: FilterSet{"nonsense": {"x"}},
	})
	assert.Error(t, err)
}

func TestEngageSearchUnknownCompany(t *testing.T) {
	a := testEngageAdapter(t)

	_, err := a.Search(t.Context(), engage.MockUnknownSlug, SearchParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), engage.MockUnknownSlug)
}

func TestEngageDetail(t *testing.T) {
	a := testEngageAdapter(t)

	d, err := a.Detail(t.Context(), engage.MockSlug, engage.MockWorkID)
	require.NoError(t, err)

	assert.Equal(t, engage.MockWorkID, d.JobID)
	assert.NotEmpty(t, d.Title)
	assert.NotEmpty(t, d.Company)
	assert.NotEmpty(t, d.Description)
	assert.True(t, strings.HasSuffix(d.URL, "/work_"+engage.MockWorkID+"/"))

	// The description is rendered from the JSON-LD fragment, so no markup
	// should survive into the unified plain-text field.
	assert.NotContains(t, d.Description, "<br")
	assert.NotContains(t, d.Description, "<p>")
	assert.NotContains(t, d.Description, "&nbsp;")
}

func TestEngageDetailUnknownJob(t *testing.T) {
	a := testEngageAdapter(t)

	_, err := a.Detail(t.Context(), engage.MockSlug, engage.MockUnknownWorkID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), engage.MockUnknownWorkID)
	assert.Contains(t, err.Error(), "job search")
}

func TestEngageSalaryLine(t *testing.T) {
	tests := []struct {
		name string
		in   engage.Salary
		want string
	}{
		{
			"annual range",
			engage.Salary{Currency: "JPY", UnitText: "YEAR", MinValue: "3600000", MaxValue: "5000000"},
			"年収 3600000〜5000000円",
		},
		{
			"monthly minimum only",
			engage.Salary{Currency: "JPY", UnitText: "MONTH", MinValue: "213000"},
			"月給 213000円〜",
		},
		{
			"hourly maximum only",
			engage.Salary{Currency: "JPY", UnitText: "HOUR", MaxValue: "1500"},
			"時給 〜1500円",
		},
		{
			"unknown period falls back to a generic label",
			engage.Salary{Currency: "JPY", UnitText: "FORTNIGHT", MinValue: "1"},
			"給与 1円〜",
		},
		{"no bounds yields nothing", engage.Salary{Currency: "JPY", UnitText: "YEAR"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, engageSalaryLine(tt.in))
		})
	}
}

func TestEngageIsRemote(t *testing.T) {
	assert.True(t, engageIsRemote(engage.Job{Title: "フルリモート可のエンジニア"}))
	assert.True(t, engageIsRemote(engage.Job{Area: "在宅勤務"}))
	assert.True(t, engageIsRemote(engage.Job{Title: "Fully Remote Engineer"}))
	assert.False(t, engageIsRemote(engage.Job{Title: "受付", Area: "北海道札幌市"}))
}

func TestEngagePlainText(t *testing.T) {
	got := engagePlainText("未経験OK<br/><p>■ 仕事内容</p><div>A&nbsp;B</div>")
	assert.Equal(t, "未経験OK\n■ 仕事内容\nA B", got)
}
