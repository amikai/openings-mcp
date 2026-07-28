package hrmos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.AllJobs(t.Context(), MockSlugPaged)
	require.NoError(t, err)

	assert.Equal(t, 203, got.Total)
	require.Len(t, got.Jobs, 203)

	seen := make(map[string]bool, len(got.Jobs))
	for _, j := range got.Jobs {
		require.NotEmpty(t, j.ID)
		require.False(t, seen[j.ID], "duplicate job id %q", j.ID)
		seen[j.ID] = true
	}

	require.Len(t, got.Facets, 5)
	var labels []string
	for _, f := range got.Facets {
		labels = append(labels, f.Label)
	}
	assert.Equal(t, []string{"雇用形態", "職種", "勤務地", "求人言語（Language）", "職種（詳細）"}, labels)

	assert.True(t, facetGroupHasOption(got.Facets, "職種", "プロダクトマネージャー"))
	assert.True(t, facetGroupHasOption(got.Facets, "職種（詳細）", "プロダクトマネージャー"))
	for _, f := range got.Facets {
		assert.NotContains(t, f.Options, "マーケティングアシスタント", "group %q should not claim this chip", f.Label)
	}
}

func facetGroupHasOption(groups []FacetGroup, label, option string) bool {
	for _, g := range groups {
		if g.Label != label {
			continue
		}
		for _, o := range g.Options {
			if o == option {
				return true
			}
		}
	}
	return false
}

func TestAllJobsNoFacets(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.AllJobs(t.Context(), MockSlugNoFacets)
	require.NoError(t, err)

	assert.Equal(t, 86, got.Total)
	require.Len(t, got.Jobs, 86)
	assert.Empty(t, got.Facets)
}

func TestAllJobsSmallTenant(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.AllJobs(t.Context(), MockSlugSmall)
	require.NoError(t, err)

	assert.Equal(t, 9, got.Total)
	require.Len(t, got.Jobs, 9)
	require.Len(t, got.Facets, 2)
}

func TestJobsTenantNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.Jobs(t.Context(), MockSlugNotFound, 1)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestJobDetailSalary(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), MockSlugSmall, MockJobIDSalary)
	require.NoError(t, err)

	assert.Equal(t, "JPY", got.SalaryCurrency)
	assert.Equal(t, "6000000", got.SalaryMin)
	assert.Equal(t, "10000000", got.SalaryMax)
	assert.Equal(t, "YEAR", got.SalaryUnit)
}

// New-graduate (新卒) postings render without the JobPosting JSON-LD, so the
// parser must fall back to the surrounding markup instead of failing.
func TestJobDetailShinsotsuNoJSONLD(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), MockSlugShinsotsu, MockJobIDShinsotsu)
	require.NoError(t, err)

	assert.Equal(t, "【28新卒/内定直結3days】巨大産業を変革し続ける“ラクスル流”事業開発", got.Title)
	// Exactly the employer, not the posting title — the last breadcrumb is
	// the title and merely happens to contain the company name.
	assert.Equal(t, "ラクスル株式会社", got.Company)
	assert.NotEqual(t, got.Title, got.Company)
	assert.Equal(t, "https://recruit.raksul.com/", got.CompanyURL)
	assert.NotEmpty(t, got.Description)
	assert.NotEmpty(t, got.JobInfo, "pg-descriptions job table still parses")
	assert.NotEmpty(t, got.CompanyInfo, "pg-descriptions company table still parses")

	// Both survive the missing JSON-LD: the date via #jsi-published-date-start,
	// the address via the 勤務地 row.
	assert.Equal(t, "2026-01-08T12:11:37.000Z", got.DatePosted)
	require.Len(t, got.Locations, 1)
	assert.Equal(t, "106-0041", got.Locations[0].PostalCode)
	assert.Equal(t, "東京都港区麻布台一丁目3番1号 麻布台ヒルズ 森JPタワー 19階", got.Locations[0].Street)
	// 新卒 is its own employment type on this surface, not 正社員.
	assert.Equal(t, "新卒", got.EmploymentType)
}

func TestJobDetailNilSalary(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.JobDetail(t.Context(), MockSlugPaged, MockJobID)
	require.NoError(t, err)

	assert.Empty(t, got.SalaryCurrency)
	assert.Empty(t, got.SalaryMin)
	assert.Empty(t, got.SalaryMax)
	assert.Empty(t, got.SalaryUnit)
}

func TestJobDetailNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.JobDetail(t.Context(), MockSlugPaged, MockJobIDNotFound)
	assert.ErrorIs(t, err, ErrNotFound)
}
