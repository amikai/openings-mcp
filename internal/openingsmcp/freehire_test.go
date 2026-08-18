package openingsmcp

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/freehire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFreehireMCPClientServer(t *testing.T) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := freehire.NewMockServer()
	t.Cleanup(srv.Close)
	client, err := freehire.NewClient(srv.URL, freehire.WithClient(srv.Client()))
	require.NoError(t, err)
	RegisterFreehire(server, client)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { serverSession.Close() })

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := mcpClient.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { clientSession.Close() })
	return clientSession
}

func callFreehire[T any](t *testing.T, s *mcp.ClientSession, name string, args map[string]any) T {
	t.Helper()
	callRes, err := s.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	require.False(t, callRes.IsError, "%s returned an error result", name)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var out T
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

func callFreehireErr(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	callRes, err := s.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	require.True(t, callRes.IsError, "%s was expected to fail", name)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	return text.Text
}

func TestRegisterFreehire(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	client, err := freehire.NewClient("https://freehire.me/api/v1")
	require.NoError(t, err)
	RegisterFreehire(server, client)
	assertTools(t, server, "freehire_search_jobs", "freehire_get_job_facets", "freehire_search_companies", "freehire_get_job_detail")
}

func TestFreehireSearchJobsE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"query": "golang",
	})

	require.NotNil(t, output.Total)
	assert.Equal(t, 2867, *output.Total)
	assert.Equal(t, 1, output.Page)
	require.NotNil(t, output.LastPage)
	assert.Equal(t, 144, *output.LastPage)
	require.Len(t, output.Data, 2)

	first := output.Data[0]
	assert.Equal(t, "Golang Developer (Backend Developer)", first.Title)
	assert.Equal(t, "Boardroom Appointments", first.Company)
	assert.Equal(t, "Pretoria, GP, South Africa", first.Location)
	assert.Empty(t, first.WorkMode)
	assert.Equal(t, "manatal", first.Source)
	assert.Equal(t, "golang-developer-backend-developer-boardroom-appointments-bw2y2vte", first.JobID)
	assert.Equal(t, "2026-08-17", first.PostedAt)
	assert.Contains(t, first.URL, "careers-page.com")
	assert.Contains(t, first.Skills, "go")

	assert.Equal(t, "Caliberly", output.Data[1].Company)
}

// TestFreehireSearchJobsCarriesCompanySlug covers the value the company
// filter takes: the display name is not it, and nothing else in the
// tool surface hands it out.
func TestFreehireSearchJobsCarriesCompanySlug(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"query": "golang",
	})
	require.Len(t, output.Data, 2)
	assert.Equal(t, "boardroom-appointments", output.Data[0].CompanySlug)
	assert.Equal(t, "caliberly", output.Data[1].CompanySlug)
}

// TestFreehireSearchJobsTruncatesDescription covers the preview: the
// search endpoint ships every row's full posting, so the tool keeps a
// bounded, single-line opening instead of the whole thing.
func TestFreehireSearchJobsTruncatesDescription(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"query": "golang",
	})
	require.Len(t, output.Data, 2)
	desc := output.Data[0].Description
	assert.NotEmpty(t, desc)
	assert.LessOrEqual(t, len([]rune(desc)), freehireDescriptionChars+1, "preview keeps the ellipsis but nothing more")
	assert.True(t, strings.HasSuffix(desc, "…"), "a truncated preview says so")
	assert.NotContains(t, desc, "\n", "the preview collapses to one line")
}

func TestFreehireDescriptionPreview(t *testing.T) {
	assert.Empty(t, freehireDescriptionPreview(""))
	assert.Equal(t, "one two three", freehireDescriptionPreview("one\n\ntwo   three"))

	long := strings.Repeat("é", freehireDescriptionChars+50)
	got := freehireDescriptionPreview(long)
	assert.Equal(t, freehireDescriptionChars+1, len([]rune(got)), "truncation counts runes, not bytes")
	assert.True(t, strings.HasSuffix(got, "…"))
}

// TestFreehireSearchJobsCarriesUpstreamFields covers the rest of the
// spec's Job schema. reality in particular is the only freshness signal
// this catalogue offers, and it is on every row.
func TestFreehireSearchJobsCarriesUpstreamFields(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"query": "golang",
	})
	require.NotEmpty(t, output.Data)
	first := output.Data[0]

	assert.Equal(t, []string{"africa"}, first.Regions)
	assert.Equal(t, []string{"za"}, first.Countries)
	assert.Equal(t, "2026-08-17", first.CreatedAt)
	assert.Empty(t, first.ClosedAt, "an open posting has no closed_at")

	require.NotNil(t, first.Reality)
	assert.Equal(t, "fresh", first.Reality["class"])
	assert.EqualValues(t, 0, first.Reality["age_days"])
	assert.Equal(t, false, first.Reality["fake_freshness"])

	require.NotNil(t, first.Enrichment)
	assert.Equal(t, "backend", first.Enrichment["category"])
}

func TestFreehireSearchJobsFilteredE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"company": "stripe",
	})

	require.NotNil(t, output.Total)
	assert.Equal(t, 520, *output.Total)
	require.NotEmpty(t, output.Data)
	assert.Equal(t, "Program Manager, Intake & Portfolio Management", output.Data[0].Title)
	assert.Equal(t, "Stripe", output.Data[0].Company)
	assert.Equal(t, "stripe", output.Data[0].CompanySlug)
	assert.Equal(t, "greenhouse", output.Data[0].Source)
	assert.Equal(t, "remote", output.Data[0].WorkMode)
	assert.Equal(t, freehire.MockJobSlug, output.Data[0].JobID)
}

func TestFreehireSearchJobsUnknownCompanyE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"company": freehire.MockUnknownCompanySlug,
	})
	require.NotNil(t, output.Total)
	assert.Equal(t, 0, *output.Total)
	assert.Empty(t, output.Data)
}

func TestFreehireGetJobFacetsE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireFacetsOutput](t, clientSession, "freehire_get_job_facets", map[string]any{
		"query": "golang",
	})
	assert.Equal(t, 2864, output.Total)
	require.Contains(t, output.Facets, "work_mode")
	assert.Equal(t, 467, output.Facets["work_mode"].Values["remote"])
	assert.Equal(t, 3, output.Facets["work_mode"].TotalValues)
	require.Contains(t, output.Facets, "skills")
	assert.NotEmpty(t, output.Facets["skills"].Values)
}

// TestFreehireGetJobFacetsCapsValues covers the context budget: skills
// and cities carry hundreds of values each, so a default call keeps only
// the most common ones and says how many it left out.
func TestFreehireGetJobFacetsCapsValues(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireFacetsOutput](t, clientSession, "freehire_get_job_facets", map[string]any{
		"query": "golang",
	})
	skills := output.Facets["skills"]
	assert.Equal(t, 533, skills.TotalValues)
	assert.Len(t, skills.Values, freehireDefaultFacetValues)
	assert.Contains(t, skills.Values, "go", "the cap keeps the most common values")

	raised := callFreehire[freehireFacetsOutput](t, clientSession, "freehire_get_job_facets", map[string]any{
		"query":      "golang",
		"max_values": 600,
	})
	assert.Len(t, raised.Facets["skills"].Values, 533)
}

// TestFreehireGetJobFacetsSelectsFacets covers the facets selector: the
// whole vocabulary is tens of kilobytes, and a caller usually wants one
// facet.
func TestFreehireGetJobFacetsSelectsFacets(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireFacetsOutput](t, clientSession, "freehire_get_job_facets", map[string]any{
		"query":  "golang",
		"facets": "work_mode, seniority",
	})
	assert.Equal(t, []string{"seniority", "work_mode"}, slices.Sorted(maps.Keys(output.Facets)))
}

// TestFreehireSearchCompaniesE2E covers the resolver for the one filter
// facets cannot serve. The fixture query matches two companies, so the
// tool has to hand back both rather than choose.
func TestFreehireSearchCompaniesE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireCompaniesOutput](t, clientSession, "freehire_search_companies", map[string]any{
		"query": freehire.MockCompanyQuery,
	})

	require.NotNil(t, output.Total)
	assert.Equal(t, 72, *output.Total)
	assert.Equal(t, 1, output.Page)
	require.Len(t, output.Data, 3)

	assert.Equal(t, "adria-solutions", output.Data[0].Slug)
	assert.Equal(t, "Adria Solutions", output.Data[0].Name)
	require.NotNil(t, output.Data[0].JobCount)
	assert.Equal(t, 67, *output.Data[0].JobCount)

	assert.Equal(t, "adria-solutions-ltd", output.Data[1].Slug)
	assert.Equal(t, "Adria Solutions Ltd", output.Data[1].Name)
	require.NotNil(t, output.Data[1].JobCount)
	assert.Equal(t, 51, *output.Data[1].JobCount)

	// The spec declares three fields plus additionalProperties; the rest
	// rides along instead of being dropped.
	assert.Equal(t, "US", output.Data[2].Details["hq_country"])
	assert.Contains(t, output.Data[2].Details, "tagline")
}

func TestFreehireSearchCompaniesNoMatchE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireCompaniesOutput](t, clientSession, "freehire_search_companies", map[string]any{
		"query": freehire.MockNoMatchCompanyQuery,
	})
	require.NotNil(t, output.Total)
	assert.Equal(t, 0, *output.Total)
	assert.Empty(t, output.Data)
}

// TestFreehireSearchCompaniesRequiresQuery covers the one constraint this
// tool adds: without q the endpoint pages through every company freehire
// holds, which is a dump rather than a lookup.
func TestFreehireSearchCompaniesRequiresQuery(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	text := callFreehireErr(t, clientSession, "freehire_search_companies", map[string]any{})
	assert.Contains(t, text, `required: missing properties: ["query"]`)
}

func TestFreehireGetJobDetailE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireDetailOutput](t, clientSession, "freehire_get_job_detail", map[string]any{
		"job_id": freehire.MockJobSlug,
	})

	assert.Equal(t, "Program Manager, Intake & Portfolio Management", output.Title)
	assert.Equal(t, "Stripe", output.Company)
	assert.Equal(t, "stripe", output.CompanySlug)
	assert.Equal(t, "greenhouse", output.Source)
	assert.Contains(t, output.ApplyURL, "gh_jid=7569678")
	assert.Contains(t, output.Description, "Stripe")
	assert.NotContains(t, output.Description, "<p>")
	assert.Greater(t, len(output.Description), 1000)
}

// TestFreehireGetJobDetailCarriesUpstreamFields covers the same fields
// on detail. This fixture is a reposted listing freehire itself marks
// stale — exactly what the tool used to hide.
func TestFreehireGetJobDetailCarriesUpstreamFields(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireDetailOutput](t, clientSession, "freehire_get_job_detail", map[string]any{
		"job_id": freehire.MockJobSlug,
	})

	assert.Equal(t, freehire.MockJobSlug, output.JobID)
	assert.Equal(t, []string{"global"}, output.Regions)
	// posted_at says 2026-08-17 but freehire first saw it on 2026-07-08:
	// the gap is the repost the reality block reports.
	assert.Equal(t, "2026-08-17", output.PostedAt)
	assert.Equal(t, "2026-07-08", output.CreatedAt)
	assert.Empty(t, output.ClosedAt)

	require.NotNil(t, output.Reality)
	assert.Equal(t, "stale", output.Reality["class"])
	assert.EqualValues(t, 40, output.Reality["age_days"])
	assert.EqualValues(t, 2, output.Reality["repost_count"])
}

func TestFreehireGetJobDetailNotFoundE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	text := callFreehireErr(t, clientSession, "freehire_get_job_detail", map[string]any{
		"job_id": freehire.MockUnknownJobSlug,
	})
	assert.Contains(t, text, "not found")
}

func TestFreehireLastPage(t *testing.T) {
	assert.Equal(t, 1, freehireLastPage(0))
	assert.Equal(t, 1, freehireLastPage(20))
	assert.Equal(t, 144, freehireLastPage(2867))
	// The catalogue is far deeper than the result window, so last_page
	// must not advertise a page the API refuses.
	assert.Equal(t, freehireMaxPage, freehireLastPage(3_300_000))
}

// TestFreehireSearchRejectsUnreachablePage covers the window: freehire
// answers 400 "pagination too deep" past it, so the tool says so first.
func TestFreehireSearchRejectsUnreachablePage(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	text := callFreehireErr(t, clientSession, "freehire_search_jobs", map[string]any{
		"page": freehireMaxPage + 1,
	})
	assert.Contains(t, text, "validating /properties/page: maximum")
}

// TestFreehireSearchParamsRejectsOverflowPage covers the arithmetic
// behind that guard: a page large enough to overflow the offset must not
// slip through as a negative offset.
func TestFreehireSearchParamsRejectsOverflowPage(t *testing.T) {
	for _, page := range []int{freehireMaxPage + 1, 461168601842738792, 1 << 62} {
		_, err := freehireMCPToHTTPRequest(&freehireSearchInput{}, page)
		require.Error(t, err, "page %d", page)
		assert.Contains(t, err.Error(), "deepest freehire pages to")
	}
}

func TestFreehireSearchParamsMapsEveryFilter(t *testing.T) {
	params, err := freehireMCPToHTTPRequest(&freehireSearchInput{
		freehireFilters: freehireFilters{
			Query:     "golang",
			Company:   "stripe",
			Skills:    "go, rust",
			Seniority: []string{"staff"},
			WorkMode:  []string{"remote"},
			Regions:   []string{"eu"},
			Country:   "de",
			Source:    "greenhouse,lever",
			Category:  "backend",
			SalaryMin: ptr(100000),
			SalaryMax: ptr(200000),
		},
		Sort:          "posted_at",
		Order:         "desc",
		SemanticRatio: ptr(0.5),
	}, 3)
	require.NoError(t, err)

	assert.Equal(t, "golang", params.Q.Or(""))
	assert.Equal(t, []string{"stripe"}, params.CompanySlug)
	assert.Equal(t, []string{"go", "rust"}, params.Skills)
	assert.Equal(t, []freehire.AgentSearchJobsSeniorityItem{freehire.AgentSearchJobsSeniorityItemStaff}, params.Seniority)
	assert.Equal(t, []freehire.AgentSearchJobsWorkModeItem{freehire.AgentSearchJobsWorkModeItemRemote}, params.WorkMode)
	assert.Equal(t, []freehire.AgentSearchJobsRegionsItem{freehire.AgentSearchJobsRegionsItemEu}, params.Regions)
	assert.Equal(t, []string{"de"}, params.Countries)
	assert.Equal(t, []string{"greenhouse", "lever"}, params.Source)
	assert.Equal(t, []string{"backend"}, params.Category)
	assert.Equal(t, freehire.AgentSearchJobsSortPostedAt, params.Sort.Or(""))
	assert.Equal(t, freehire.AgentSearchJobsOrderDesc, params.Order.Or(""))
	assert.Equal(t, 40, params.Offset.Or(-1))
	assert.Equal(t, freehirePageSize, params.Limit.Or(0))
	assert.Equal(t, freehire.AgentSearchJobsDescriptionFormatText, params.DescriptionFormat.Or(""))
	assert.Equal(t, 100000, params.SalaryMin.Or(0))
	assert.Equal(t, 200000, params.SalaryMax.Or(0))
	assert.InEpsilon(t, 0.5, params.SemanticRatio.Or(0), 1e-9)
}

func TestFreehireFacetsParamsMapsSalary(t *testing.T) {
	params, err := freehireMCPToFacetsRequest(&freehireFacetsInput{
		freehireFilters: freehireFilters{
			Query:     "golang",
			Source:    "greenhouse",
			Category:  "backend",
			SalaryMin: ptr(100000),
			SalaryMax: ptr(200000),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "golang", params.Q.Or(""))
	assert.Equal(t, []string{"greenhouse"}, params.Source)
	assert.Equal(t, []string{"backend"}, params.Category)
	assert.Equal(t, 100000, params.SalaryMin.Or(0))
	assert.Equal(t, 200000, params.SalaryMax.Or(0))
}

func ptr[T any](v T) *T { return &v }

// TestFreehireSearchOmitsUnknownTotal covers a response without the
// spec-optional meta: reporting total 0 beside a full page would read as
// "no matches".
func TestFreehireSearchOmitsUnknownTotal(t *testing.T) {
	out := freehireHTTPToMCPResponse(&freehire.JobListEnvelope{
		Data: []freehire.Job{{PublicSlug: "a", Title: "t", Company: "c"}},
	}, 1)
	assert.Nil(t, out.Total)
	assert.Nil(t, out.LastPage)
	require.Len(t, out.Data, 1)
}

func TestFreehireSearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	text := callFreehireErr(t, clientSession, "freehire_search_jobs", map[string]any{
		"seniority": []any{"valueNotInEnum"},
	})
	assert.Contains(t, text, `validating /properties/seniority/items: enum`)
}
