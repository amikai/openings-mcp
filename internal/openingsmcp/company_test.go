package openingsmcp

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/ats"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// stubAdapter returns canned results so tests exercise only the MCP
// translation layer. name and roster default to the single-company shape
// most tests need; set them explicitly to build a colliding roster across
// two stub adapters.
type stubAdapter struct {
	name         string
	host         string
	roster       []ats.CompanyInfo
	searchResult *ats.SearchResult
	filterSet    ats.FilterSet
	detail       *ats.JobDetail
	gotParams    ats.SearchParams

	searchCalled  bool
	filtersCalled bool
	detailCalled  bool
	filterCompany string
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
func (s *stubAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if s.host == "" || u.Hostname() != s.host {
		return "", false
	}
	slug := strings.Trim(u.Path, "/")
	return slug, slug != ""
}
func (s *stubAdapter) CareersURL(slug string) (string, bool) {
	if s.host == "" {
		return "", false
	}
	return "https://" + s.host + "/" + slug, true
}
func (s *stubAdapter) Search(_ context.Context, _ string, p ats.SearchParams) (*ats.SearchResult, error) {
	s.searchCalled = true
	s.gotParams = p
	return s.searchResult, nil
}
func (s *stubAdapter) Filters(_ context.Context, company string) (ats.FilterSet, error) {
	s.filtersCalled = true
	s.filterCompany = company
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
	resolution, err := reg.Resolve("Acme Corp")
	require.NoError(t, err)
	adapter, slug, ok := resolution.Single()
	require.True(t, ok)

	out, err := companySearch(t.Context(), adapter, slug, &companySearchInput{
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
	_, _, err := resolveCompany(reg, "acme corp intl", "")
	require.ErrorContains(t, err, "acme", "want teaching error")
}

func TestCompanyFilters(t *testing.T) {
	stub := &stubAdapter{filterSet: ats.FilterSet{"team": {"ML", "Web"}}}
	reg := testCompanyRegistry(t, stub)
	resolution, err := reg.Resolve("acme")
	require.NoError(t, err)
	adapter, slug, ok := resolution.Single()
	require.True(t, ok)
	out, err := companyFilters(t.Context(), adapter, slug)
	require.NoError(t, err)
	assert.Len(t, out.Filters["team"], 2)
}

func TestCompanyDetail(t *testing.T) {
	stub := &stubAdapter{detail: &ats.JobDetail{JobID: "j1", Title: "Engineer", Company: "Acme Corp", Description: "plain text"}}
	reg := testCompanyRegistry(t, stub)
	resolution, err := reg.Resolve("acme")
	require.NoError(t, err)
	adapter, slug, ok := resolution.Single()
	require.True(t, ok)
	out, err := companyDetail(t.Context(), adapter, slug, &companyDetailInput{Company: "acme", JobID: "j1"})
	require.NoError(t, err)
	assert.Equal(t, "Engineer", out.Title)
	assert.Equal(t, "plain text", out.Description)
}

// ambiguousCompanyRegistry builds a registry from two stub adapters whose
// rosters collide on the same slug, so every Resolve("nature") call is
// ambiguous.
func ambiguousCompanyRegistry(t *testing.T) (*ats.Registry, *stubAdapter, *stubAdapter) {
	t.Helper()
	stubA := &stubAdapter{
		name:         "stubA",
		host:         "nature-a.example",
		roster:       []ats.CompanyInfo{{Slug: "nature", Name: "Nature A"}},
		searchResult: &ats.SearchResult{},
		detail:       &ats.JobDetail{},
	}
	stubB := &stubAdapter{
		name:         "stubB",
		host:         "nature-b.example",
		roster:       []ats.CompanyInfo{{Slug: "nature", Name: "Nature B"}},
		searchResult: &ats.SearchResult{},
		detail:       &ats.JobDetail{},
	}
	reg, err := ats.NewRegistry(stubA, stubB)
	require.NoError(t, err)
	return reg, stubA, stubB
}

// TestCompanyToolsRejectedWhileAmbiguous asserts that every unified company
// tool turns an ambiguous company into a teaching error listing all
// candidates, and that none of them reaches an adapter first.
func TestCompanyToolsRejectedWhileAmbiguous(t *testing.T) {
	const detailHint = "Use the same company value that produced this job_id."

	tests := []struct {
		name      string
		tool      string
		arguments map[string]any
		wantHint  bool
	}{
		{
			name:      "search",
			tool:      "search_jobs_by_company",
			arguments: map[string]any{"company": "nature"},
		},
		{
			name:      "filters",
			tool:      "get_filters_by_company",
			arguments: map[string]any{"company": "nature"},
		},
		{
			name:      "detail",
			tool:      "get_job_detail_by_company",
			arguments: map[string]any{"company": "nature", "job_id": "j1"},
			wantHint:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, stubA, stubB := ambiguousCompanyRegistry(t)
			result := callCompanyTool(t, reg, tt.tool, tt.arguments)

			require.True(t, result.IsError)
			require.Len(t, result.Content, 1)
			text, ok := result.Content[0].(*mcp.TextContent)
			require.True(t, ok)

			assert.Contains(t, text.Text, `ambiguous company "nature"`)
			assert.Contains(t, text.Text, "Nature A")
			assert.Contains(t, text.Text, "https://nature-a.example/nature")
			assert.Contains(t, text.Text, "Nature B")
			assert.Contains(t, text.Text, "https://nature-b.example/nature")
			assert.NotContains(t, text.Text, "stubA", "provider names must stay internal")
			assert.NotContains(t, text.Text, "stubB", "provider names must stay internal")

			if tt.wantHint {
				assert.Contains(t, text.Text, detailHint)
			} else {
				assert.NotContains(t, text.Text, detailHint)
			}

			assert.False(t, stubA.searchCalled)
			assert.False(t, stubA.filtersCalled)
			assert.False(t, stubA.detailCalled)
			assert.False(t, stubB.searchCalled)
			assert.False(t, stubB.filtersCalled)
			assert.False(t, stubB.detailCalled)
		})
	}
}

// TestAmbiguousCompanyErrorNamesCandidateWithoutCareersURL covers the
// candidate an adapter cannot render a URL for: its display name is the
// retry key, and the internal provider stays out of the message.
func TestAmbiguousCompanyErrorNamesCandidateWithoutCareersURL(t *testing.T) {
	err := &AmbiguousCompanyError{
		Input: "nature",
		Candidates: []ats.CompanyCandidate{
			{Name: "Nature A", Provider: "internal-a"},
			{Name: "Nature B", CareersURL: "https://nature-b.example/nature", Provider: "internal-b"},
		},
	}

	assert.ErrorContains(t, err, `retry with company="Nature A"`)
	assert.ErrorContains(t, err, "https://nature-b.example/nature")
	assert.NotContains(t, err.Error(), "internal-a")
	assert.NotContains(t, err.Error(), "internal-b")
}

// TestCompanyToolResolvesUniqueCompany pins the other side of the same
// wiring: a company matching one roster entry reaches its adapter.
func TestCompanyToolResolvesUniqueCompany(t *testing.T) {
	stub := &stubAdapter{filterSet: ats.FilterSet{"team": {"ML"}}}
	reg := testCompanyRegistry(t, stub)

	result := callCompanyTool(t, reg, "get_filters_by_company", map[string]any{"company": "acme"})

	assert.False(t, result.IsError)
	var out companyFiltersOutput
	decodeStructuredContent(t, result, &out)
	assert.Equal(t, []string{"ML"}, out.Filters["team"])
	assert.Equal(t, "acme", stub.filterCompany)
}

func decodeStructuredContent(t *testing.T, result *mcp.CallToolResult, target any) {
	t.Helper()
	data, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, target))
}

func callCompanyTool(
	t *testing.T,
	reg *ats.Registry,
	name string,
	arguments map[string]any,
) *mcp.CallToolResult {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0"}, nil)
	RegisterCompany(server, reg)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	defer clientSession.Close()

	result, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}
