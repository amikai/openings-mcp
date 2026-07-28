package openingsmcp

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
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
	searchCompany string
	filterCompany string
	detailCompany string
	detailJobID   string
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
func (s *stubAdapter) Search(_ context.Context, company string, p ats.SearchParams) (*ats.SearchResult, error) {
	s.searchCalled = true
	s.searchCompany = company
	s.gotParams = p
	return s.searchResult, nil
}
func (s *stubAdapter) Filters(_ context.Context, company string) (ats.FilterSet, error) {
	s.filtersCalled = true
	s.filterCompany = company
	return s.filterSet, nil
}
func (s *stubAdapter) Detail(_ context.Context, company, jobID string) (*ats.JobDetail, error) {
	s.detailCalled = true
	s.detailCompany = company
	s.detailJobID = jobID
	return s.detail, nil
}

func testCompanyRegistry(t *testing.T, stub *stubAdapter) *ats.Registry {
	t.Helper()
	r, err := ats.NewRegistry(stub)
	require.NoError(t, err)
	return r
}

func selectTestCompany(t *testing.T, reg *ats.Registry, input string) (ats.Adapter, string) {
	t.Helper()
	resolution, err := reg.Resolve(input)
	require.NoError(t, err)
	adapter, slug, ok := resolution.Select(0)
	require.True(t, ok)
	return adapter, slug
}

func TestCompanySearchMapsParamsAndResult(t *testing.T) {
	stub := &stubAdapter{searchResult: &ats.SearchResult{
		Jobs: []ats.JobSummary{{
			JobID: "j1", Title: "Engineer", Location: "Taipei", PostedAt: "2026-07-01", URL: "https://x/j1",
		}},
		TotalCount: 41, Page: 2, TotalPages: 3,
	}}
	reg := testCompanyRegistry(t, stub)
	adapter, slug := selectTestCompany(t, reg, "Acme Corp")

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
	_, _, _, err := resolveCompanyForTool(
		&mcp.CallToolRequest{},
		reg,
		"acme corp intl",
		"",
	)
	require.ErrorContains(t, err, "acme", "want teaching error")
}

func TestCompanyFilters(t *testing.T) {
	stub := &stubAdapter{filterSet: ats.FilterSet{"team": {"ML", "Web"}}}
	reg := testCompanyRegistry(t, stub)
	adapter, slug := selectTestCompany(t, reg, "acme")
	out, err := companyFilters(t.Context(), adapter, slug)
	require.NoError(t, err)
	assert.Len(t, out.Filters["team"], 2)
}

func TestCompanyDetail(t *testing.T) {
	stub := &stubAdapter{detail: &ats.JobDetail{JobID: "j1", Title: "Engineer", Company: "Acme Corp", Description: "plain text"}}
	reg := testCompanyRegistry(t, stub)
	adapter, slug := selectTestCompany(t, reg, "acme")
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

func TestCompanySearchElicitsAmbiguousCompany(t *testing.T) {
	reg, stubA, stubB := ambiguousCompanyRegistry(t)
	stubA.searchResult = &ats.SearchResult{Jobs: []ats.JobSummary{{JobID: "a", Title: "From A"}}}
	stubB.searchResult = &ats.SearchResult{Jobs: []ats.JobSummary{{JobID: "b", Title: "From B"}}}

	result := callCompanyTool(
		t,
		reg,
		acceptCompanyChoice("2", nil),
		"search_jobs_by_company",
		map[string]any{"company": "nature", "query": "go"},
	)

	assert.False(t, result.IsError)
	var out companySearchOutput
	decodeStructuredContent(t, result, &out)
	require.Len(t, out.Data, 1)
	assert.Equal(t, "From B", out.Data[0].Title)
	assert.False(t, stubA.searchCalled)
	assert.True(t, stubB.searchCalled)
	assert.Equal(t, "nature", stubB.searchCompany)
	assert.Equal(t, "go", stubB.gotParams.Query)
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

func TestCompanyDetailElicitsAmbiguousCompany(t *testing.T) {
	reg, stubA, stubB := ambiguousCompanyRegistry(t)
	stubA.detail = &ats.JobDetail{JobID: "j1", Title: "From A"}
	stubB.detail = &ats.JobDetail{JobID: "j1", Title: "From B"}

	var elicitationMessage string
	result := callCompanyTool(
		t,
		reg,
		acceptCompanyChoice("2", func(message string) {
			elicitationMessage = message
		}),
		"get_job_detail_by_company",
		map[string]any{"company": "nature", "job_id": "j1"},
	)

	assert.False(t, result.IsError)
	var out companyDetailOutput
	decodeStructuredContent(t, result, &out)
	assert.Equal(t, "From B", out.Title)
	assert.Contains(t, elicitationMessage, "Choose the same company that produced this job_id.")
	assert.False(t, stubA.detailCalled)
	assert.True(t, stubB.detailCalled)
	assert.Equal(t, "nature", stubB.detailCompany)
	assert.Equal(t, "j1", stubB.detailJobID)
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
	assert.Contains(t, text.Text, "https://nature-a.example/nature")
	assert.Contains(t, text.Text, "https://nature-b.example/nature")
	assert.NotContains(t, text.Text, "stubA")
	assert.NotContains(t, text.Text, "stubB")
	assert.False(t, stubA.filtersCalled)
	assert.False(t, stubB.filtersCalled)
}

func TestAmbiguousCompanyFallbackUsesExactNameWithoutCareersURL(t *testing.T) {
	err := ambiguousCompanyError("nature", []ats.CompanyCandidate{
		{Name: "Nature A"},
		{Name: "Nature B", CareersURL: "https://nature-b.example/nature"},
	}, "")

	assert.ErrorContains(t, err, `retry with company="Nature A"`)
	assert.ErrorContains(t, err, "https://nature-b.example/nature")
}

func TestCompanySelectionRequestLabelsChoiceValues(t *testing.T) {
	result := companySelectionRequest("nature", []ats.CompanyCandidate{
		{
			Name:       "Nature A",
			CareersURL: "https://nature-a.example/nature",
			Provider:   "internal-a",
		},
		{
			Name:     "Nature B",
			Provider: "internal-b",
		},
	}, "")

	params, ok := result.InputRequests[companySelectionRequestID].(*mcp.ElicitParams)
	require.True(t, ok)
	schema, ok := params.RequestedSchema.(*jsonschema.Schema)
	require.True(t, ok)
	choice := schema.Properties["choice"]
	require.NotNil(t, choice)
	assert.Empty(t, choice.Enum)
	require.Len(t, choice.OneOf, 2)

	require.NotNil(t, choice.OneOf[0].Const)
	assert.Equal(t, "1", *choice.OneOf[0].Const)
	assert.Equal(t, "Nature A — https://nature-a.example/nature", choice.OneOf[0].Title)
	require.NotNil(t, choice.OneOf[1].Const)
	assert.Equal(t, "2", *choice.OneOf[1].Const)
	assert.Equal(t, "Nature B", choice.OneOf[1].Title)

	assert.NotContains(t, choice.OneOf[0].Title, "internal-a")
	assert.NotContains(t, choice.OneOf[1].Title, "internal-b")
}

func callCompanyFilters(t *testing.T, reg *ats.Registry, options *mcp.ClientOptions) *mcp.CallToolResult {
	t.Helper()
	return callCompanyTool(
		t,
		reg,
		options,
		"get_filters_by_company",
		map[string]any{"company": "nature"},
	)
}

func acceptCompanyChoice(choice string, inspectMessage func(string)) *mcp.ClientOptions {
	return &mcp.ClientOptions{
		ElicitationHandler: func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			if inspectMessage != nil {
				inspectMessage(req.Params.Message)
			}
			return &mcp.ElicitResult{
				Action:  "accept",
				Content: map[string]any{"choice": choice},
			}, nil
		},
	}
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
	options *mcp.ClientOptions,
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

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, options)
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
