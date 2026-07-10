package openingsmcp

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/ats"
)

// stubAdapter returns canned results so tests exercise only the MCP
// translation layer.
type stubAdapter struct {
	searchResult *ats.SearchResult
	filterSet    ats.FilterSet
	detail       *ats.JobDetail
	gotParams    ats.SearchParams
}

func (s *stubAdapter) Name() string { return "stub" }
func (s *stubAdapter) Roster() []ats.CompanyInfo {
	return []ats.CompanyInfo{{Slug: "acme", Name: "Acme Corp"}}
}
func (s *stubAdapter) ParseCareersURL(*url.URL) (string, bool) { return "", false }
func (s *stubAdapter) Search(_ context.Context, _ string, p ats.SearchParams) (*ats.SearchResult, error) {
	s.gotParams = p
	return s.searchResult, nil
}
func (s *stubAdapter) Filters(context.Context, string) (ats.FilterSet, error) {
	return s.filterSet, nil
}
func (s *stubAdapter) Detail(context.Context, string, string) (*ats.JobDetail, error) {
	return s.detail, nil
}

func testCompanyRegistry(t *testing.T, stub *stubAdapter) *ats.Registry {
	t.Helper()
	r, err := ats.NewRegistry(stub)
	require.NoError(t, err)
	return r
}

func TestCompanySearchMapsParamsAndResult(t *testing.T) {
	stub := &stubAdapter{searchResult: &ats.SearchResult{
		Jobs: []ats.JobSummary{{
			JobID: "j1", Title: "Engineer", Location: "Taipei", PostedAt: "2026-07-01", URL: "https://x/j1",
		}},
		TotalCount: 41, Page: 2, TotalPages: 3,
	}}
	reg := testCompanyRegistry(t, stub)

	out, err := companySearch(t.Context(), reg, &companySearchInput{
		Company:  "Acme Corp",
		Query:    "golang",
		Location: "taipei",
		Filters:  map[string][]string{"team": {"Platform"}},
		Page:     2,
	})
	require.NoError(t, err)

	assert.Equal(t, "golang", stub.gotParams.Query)
	assert.Equal(t, 2, stub.gotParams.Page)
	require.Contains(t, stub.gotParams.Filters, "team")
	assert.Equal(t, "Platform", stub.gotParams.Filters["team"][0])

	assert.Equal(t, 41, out.TotalCount)
	assert.Equal(t, 2, out.Page)
	assert.Equal(t, 3, out.TotalPages)
	require.Len(t, out.Data, 1)
	assert.Equal(t, "j1", out.Data[0].JobID)
	assert.Equal(t, "https://x/j1", out.Data[0].URL)
}

func TestCompanySearchUnknownCompanyTeaches(t *testing.T) {
	reg := testCompanyRegistry(t, &stubAdapter{})
	_, err := companySearch(t.Context(), reg, &companySearchInput{Company: "acme corp intl"})
	require.ErrorContains(t, err, "acme", "want teaching error")
}

func TestCompanyFilters(t *testing.T) {
	stub := &stubAdapter{filterSet: ats.FilterSet{"team": {"ML", "Web"}}}
	reg := testCompanyRegistry(t, stub)
	out, err := companyFilters(t.Context(), reg, &companyFiltersInput{Company: "acme"})
	require.NoError(t, err)
	assert.Len(t, out.Filters["team"], 2)
}

func TestCompanyDetail(t *testing.T) {
	stub := &stubAdapter{detail: &ats.JobDetail{JobID: "j1", Title: "Engineer", Company: "Acme Corp", Description: "plain text"}}
	reg := testCompanyRegistry(t, stub)
	out, err := companyDetail(t.Context(), reg, &companyDetailInput{Company: "acme", JobID: "j1"})
	require.NoError(t, err)
	assert.Equal(t, "Engineer", out.Title)
	assert.Equal(t, "plain text", out.Description)
}
