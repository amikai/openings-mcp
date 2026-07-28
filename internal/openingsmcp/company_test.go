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
// translation layer. name and roster default to the single-company shape
// most tests need; set them explicitly to build a colliding roster across
// two stub adapters.
type stubAdapter struct {
	name         string
	roster       []ats.CompanyInfo
	searchResult *ats.SearchResult
	filterSet    ats.FilterSet
	detail       *ats.JobDetail
	gotParams    ats.SearchParams

	searchCalled  bool
	filtersCalled bool
	detailCalled  bool
}

func (s *stubAdapter) Name() string {
	if s.name != "" {
		return s.name
	}
	return "stub"
}
func (s *stubAdapter) Roster() []ats.CompanyInfo {
	if s.roster != nil {
		return s.roster
	}
	return []ats.CompanyInfo{{Slug: "acme", Name: "Acme Corp"}}
}
func (s *stubAdapter) ParseCareersURL(*url.URL) (string, bool) { return "", false }
func (s *stubAdapter) CareersURL(string) (string, bool)        { return "", false }
func (s *stubAdapter) Search(_ context.Context, _ string, p ats.SearchParams) (*ats.SearchResult, error) {
	s.searchCalled = true
	s.gotParams = p
	return s.searchResult, nil
}
func (s *stubAdapter) Filters(context.Context, string) (ats.FilterSet, error) {
	s.filtersCalled = true
	return s.filterSet, nil
}
func (s *stubAdapter) Detail(context.Context, string, string) (*ats.JobDetail, error) {
	s.detailCalled = true
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

// ambiguousCompanyRegistry builds a registry from two stub adapters whose
// rosters collide on the same slug, so every Resolve("nature") call is
// ambiguous.
func ambiguousCompanyRegistry(t *testing.T) (*ats.Registry, *stubAdapter, *stubAdapter) {
	t.Helper()
	// The stubs return usable results so that a regression which resolves the
	// ambiguity instead of rejecting it fails on the assertions below, rather
	// than panicking on a nil result and taking the rest of the package with it.
	stubA := &stubAdapter{
		name:         "stubA",
		roster:       []ats.CompanyInfo{{Slug: "nature", Name: "Nature A"}},
		searchResult: &ats.SearchResult{},
		detail:       &ats.JobDetail{},
	}
	stubB := &stubAdapter{
		name:         "stubB",
		roster:       []ats.CompanyInfo{{Slug: "nature", Name: "Nature B"}},
		searchResult: &ats.SearchResult{},
		detail:       &ats.JobDetail{},
	}
	reg, err := ats.NewRegistry(stubA, stubB)
	require.NoError(t, err)
	return reg, stubA, stubB
}

func TestCompanyToolsRejectedWhileAmbiguous(t *testing.T) {
	reg, stubA, stubB := ambiguousCompanyRegistry(t)

	_, err := companySearch(t.Context(), reg, &companySearchInput{Company: "nature"})
	require.Error(t, err)
	var ambErr *ats.AmbiguousCompanyError
	require.ErrorAs(t, err, &ambErr)

	_, err = companyFilters(t.Context(), reg, &companyFiltersInput{Company: "nature"})
	require.Error(t, err)
	require.ErrorAs(t, err, &ambErr)

	_, err = companyDetail(t.Context(), reg, &companyDetailInput{Company: "nature", JobID: "j1"})
	require.Error(t, err)
	require.ErrorAs(t, err, &ambErr)

	assert.False(t, stubA.searchCalled, "stubA.Search must not be called while ambiguous")
	assert.False(t, stubA.filtersCalled, "stubA.Filters must not be called while ambiguous")
	assert.False(t, stubA.detailCalled, "stubA.Detail must not be called while ambiguous")
	assert.False(t, stubB.searchCalled, "stubB.Search must not be called while ambiguous")
	assert.False(t, stubB.filtersCalled, "stubB.Filters must not be called while ambiguous")
	assert.False(t, stubB.detailCalled, "stubB.Detail must not be called while ambiguous")
}

func TestCompanyDetailAmbiguityAddsPreviousKeySentence(t *testing.T) {
	reg, _, _ := ambiguousCompanyRegistry(t)
	const sentence = "use the same company value that produced this job_id"

	_, searchErr := companySearch(t.Context(), reg, &companySearchInput{Company: "nature"})
	require.Error(t, searchErr)
	assert.NotContains(t, searchErr.Error(), sentence)

	_, filtersErr := companyFilters(t.Context(), reg, &companyFiltersInput{Company: "nature"})
	require.Error(t, filtersErr)
	assert.NotContains(t, filtersErr.Error(), sentence)

	_, detailErr := companyDetail(t.Context(), reg, &companyDetailInput{Company: "nature", JobID: "j1"})
	require.Error(t, detailErr)
	assert.Contains(t, detailErr.Error(), sentence)
}
