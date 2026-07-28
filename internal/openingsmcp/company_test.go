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

func TestCompanyFiltersElicitsAmbiguousCompany(t *testing.T) {
	reg, stubA, stubB := ambiguousCompanyRegistry(t)
	stubA.filterSet = ats.FilterSet{"team": {"A"}}
	stubB.filterSet = ats.FilterSet{"team": {"B"}}

	var elicitationMessage string
	result := callCompanyFilters(t, reg, &mcp.ClientOptions{
		ElicitationHandler: func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			elicitationMessage = req.Params.Message
			return &mcp.ElicitResult{
				Action:  "accept",
				Content: map[string]any{"choice": "2"},
			}, nil
		},
	})

	assert.False(t, result.IsError)
	var out companyFiltersOutput
	data, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, []string{"B"}, out.Filters["team"])

	assert.Contains(t, elicitationMessage, "1. Nature A")
	assert.Contains(t, elicitationMessage, "2. Nature B")
	assert.Contains(t, elicitationMessage, "https://nature-a.example/nature")
	assert.Contains(t, elicitationMessage, "https://nature-b.example/nature")
	assert.NotContains(t, elicitationMessage, "stubA", "provider names must stay internal")
	assert.NotContains(t, elicitationMessage, "stubB", "provider names must stay internal")

	assert.False(t, stubA.filtersCalled)
	assert.True(t, stubB.filtersCalled)
	assert.Equal(t, "nature", stubB.filterCompany)
}

func TestCompanyFiltersElicitationCancellation(t *testing.T) {
	reg, stubA, stubB := ambiguousCompanyRegistry(t)
	result := callCompanyFilters(t, reg, &mcp.ClientOptions{
		ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "cancel"}, nil
		},
	})

	assert.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "company selection cancelled")
	assert.False(t, stubA.filtersCalled)
	assert.False(t, stubB.filtersCalled)
}

func TestCompanyFiltersAmbiguityFallsBackWithoutElicitation(t *testing.T) {
	reg, stubA, stubB := ambiguousCompanyRegistry(t)
	result := callCompanyFilters(t, reg, nil)

	assert.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, `ambiguous company "nature"`)
	assert.False(t, stubA.filtersCalled)
	assert.False(t, stubB.filtersCalled)
}

func callCompanyFilters(t *testing.T, reg *ats.Registry, options *mcp.ClientOptions) *mcp.CallToolResult {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0"}, nil)
	RegisterCompany(server, reg)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, options)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	defer clientSession.Close()

	result, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "get_filters_by_company",
		Arguments: map[string]any{"company": "nature"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}
