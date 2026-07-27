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
