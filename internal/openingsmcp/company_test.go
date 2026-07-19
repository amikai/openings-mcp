package openingsmcp

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/ats"
)

// stubAdapter returns canned results so tests exercise only the MCP
// translation layer. Zero-value fields keep the historical defaults:
// name "stub", roster [{acme, Acme Corp}].
type stubAdapter struct {
	name         string
	roster       []ats.CompanyInfo
	searchResult *ats.SearchResult
	searchErr    error
	filterSet    ats.FilterSet
	filtersErr   error
	detail       *ats.JobDetail
	detailErr    error
	gotParams    ats.SearchParams
}

func (s *stubAdapter) Name() string {
	if s.name == "" {
		return "stub"
	}
	return s.name
}

func (s *stubAdapter) Roster() []ats.CompanyInfo {
	if s.roster == nil {
		return []ats.CompanyInfo{{Slug: "acme", Name: "Acme Corp"}}
	}
	return s.roster
}

func (s *stubAdapter) ParseCareersURL(*url.URL) (string, bool) { return "", false }

func (s *stubAdapter) Search(_ context.Context, _ string, p ats.SearchParams) (*ats.SearchResult, error) {
	s.gotParams = p
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResult, nil
}

func (s *stubAdapter) Filters(context.Context, string) (ats.FilterSet, error) {
	if s.filtersErr != nil {
		return nil, s.filtersErr
	}
	return s.filterSet, nil
}

func (s *stubAdapter) Detail(context.Context, string, string) (*ats.JobDetail, error) {
	if s.detailErr != nil {
		return nil, s.detailErr
	}
	return s.detail, nil
}

func testCompanyRegistry(t *testing.T, stub *stubAdapter) *ats.Registry {
	t.Helper()
	r, err := ats.NewRegistry(stub)
	require.NoError(t, err)
	return r
}

// testMultiRegistry registers two stubs whose rosters share the display
// name "Acme Corp", so Resolve("Acme Corp") fans out to both.
func testMultiRegistry(t *testing.T, a, b *stubAdapter) *ats.Registry {
	t.Helper()
	a.name, b.name = "stub-a", "stub-b"
	a.roster = []ats.CompanyInfo{{Slug: "acme", Name: "Acme Corp"}}
	b.roster = []ats.CompanyInfo{{Slug: "acme-jp", Name: "Acme Corp"}}
	r, err := ats.NewRegistry(a, b)
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

func TestCompanyFiltersUnionsMultiMatch(t *testing.T) {
	a := &stubAdapter{filterSet: ats.FilterSet{"team": {"ML", "Web"}, "level": {"Senior"}}}
	b := &stubAdapter{filterSet: ats.FilterSet{"team": {"Web", "Hardware"}}}
	reg := testMultiRegistry(t, a, b)

	out, err := companyFilters(t.Context(), reg, &companyFiltersInput{Company: "Acme Corp"})
	require.NoError(t, err)
	assert.Equal(t, []string{"ML", "Web", "Hardware"}, out.Filters["team"], "values dedupe, first-seen order")
	assert.Equal(t, []string{"Senior"}, out.Filters["level"], "dimensions union")
}

func TestCompanyFiltersSkipsFailedAdapter(t *testing.T) {
	a := &stubAdapter{filtersErr: errors.New("upstream 500")}
	b := &stubAdapter{filterSet: ats.FilterSet{"team": {"Web"}}}
	reg := testMultiRegistry(t, a, b)

	out, err := companyFilters(t.Context(), reg, &companyFiltersInput{Company: "Acme Corp"})
	require.NoError(t, err)
	assert.Equal(t, []string{"Web"}, out.Filters["team"])
}

func TestCompanyDetailFirstSuccess(t *testing.T) {
	a := &stubAdapter{detailErr: errors.New("job not found")}
	b := &stubAdapter{detail: &ats.JobDetail{JobID: "j1", Title: "Engineer"}}
	reg := testMultiRegistry(t, a, b)

	out, err := companyDetail(t.Context(), reg, &companyDetailInput{Company: "Acme Corp", JobID: "j1"})
	require.NoError(t, err, "the job belongs to the second adapter")
	assert.Equal(t, "Engineer", out.Title)
}

func TestCompanyDetailAllFail(t *testing.T) {
	a := &stubAdapter{detailErr: errors.New("job not found in a")}
	b := &stubAdapter{detailErr: errors.New("job not found in b")}
	reg := testMultiRegistry(t, a, b)

	_, err := companyDetail(t.Context(), reg, &companyDetailInput{Company: "Acme Corp", JobID: "zzz"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "not found in a")
	assert.ErrorContains(t, err, "not found in b")
}

func TestCompanySearchMergesMultiMatch(t *testing.T) {
	a := &stubAdapter{searchResult: &ats.SearchResult{
		Jobs:       []ats.JobSummary{{JobID: "a1", Title: "Engineer"}},
		TotalCount: 21, Page: 1, TotalPages: 2,
	}}
	b := &stubAdapter{searchResult: &ats.SearchResult{
		Jobs:       []ats.JobSummary{{JobID: "b1", Title: "Engineer, Japan"}},
		TotalCount: 5, Page: 1, TotalPages: 1,
	}}
	reg := testMultiRegistry(t, a, b)

	out, err := companySearch(t.Context(), reg, &companySearchInput{Company: "Acme Corp"})
	require.NoError(t, err)

	require.Len(t, out.Data, 2)
	assert.Equal(t, "a1", out.Data[0].JobID, "jobs keep adapter registration order")
	assert.Equal(t, "b1", out.Data[1].JobID)
	assert.Equal(t, 26, out.TotalCount, "total_count sums across adapters")
	assert.Equal(t, 2, out.TotalPages, "total_pages takes the max")
	assert.Equal(t, 1, out.Page)
}

func TestCompanySearchSkipsFailedAdapter(t *testing.T) {
	a := &stubAdapter{searchErr: errors.New("upstream 500")}
	b := &stubAdapter{searchResult: &ats.SearchResult{
		Jobs:       []ats.JobSummary{{JobID: "b1"}},
		TotalCount: 1, Page: 1, TotalPages: 1,
	}}
	reg := testMultiRegistry(t, a, b)

	out, err := companySearch(t.Context(), reg, &companySearchInput{Company: "Acme Corp"})
	require.NoError(t, err, "one healthy adapter is enough")
	require.Len(t, out.Data, 1)
	assert.Equal(t, "b1", out.Data[0].JobID)
	assert.Equal(t, 1, out.TotalCount)
}

func TestCompanySearchAllAdaptersFail(t *testing.T) {
	a := &stubAdapter{searchErr: errors.New("upstream 500")}
	b := &stubAdapter{searchErr: errors.New("upstream 503")}
	reg := testMultiRegistry(t, a, b)

	_, err := companySearch(t.Context(), reg, &companySearchInput{Company: "Acme Corp"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "500")
	assert.ErrorContains(t, err, "503")
}

func TestCompanyDetail(t *testing.T) {
	stub := &stubAdapter{detail: &ats.JobDetail{JobID: "j1", Title: "Engineer", Company: "Acme Corp", Description: "plain text"}}
	reg := testCompanyRegistry(t, stub)
	out, err := companyDetail(t.Context(), reg, &companyDetailInput{Company: "acme", JobID: "j1"})
	require.NoError(t, err)
	assert.Equal(t, "Engineer", out.Title)
	assert.Equal(t, "plain text", out.Description)
}
