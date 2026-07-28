package main

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/amikai/openings-mcp/internal/ats"
	"github.com/amikai/openings-mcp/internal/provider/amazon"
	"github.com/amikai/openings-mcp/internal/provider/apple"
	"github.com/amikai/openings-mcp/internal/provider/cake"
	"github.com/amikai/openings-mcp/internal/provider/google"
	"github.com/amikai/openings-mcp/internal/provider/indeed"
	"github.com/amikai/openings-mcp/internal/provider/job104"
	"github.com/amikai/openings-mcp/internal/provider/jobindex"
	"github.com/amikai/openings-mcp/internal/provider/linkedin"
	"github.com/amikai/openings-mcp/internal/provider/nvidia"
	"github.com/amikai/openings-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error { return nil }

func TestServerListsJobTools(t *testing.T) {
	ctx := t.Context()
	cAmazon, err := amazon.NewClient("https://www.amazon.jobs", amazon.WithClient(http.DefaultClient))
	require.NoError(t, err)
	c104, err := job104.NewClient("https://www.104.com.tw", job104.WithClient(http.DefaultClient))
	require.NoError(t, err)
	cApple, err := apple.NewJobsClient("https://jobs.apple.com", http.DefaultClient)
	require.NoError(t, err)
	cCake, err := cake.NewClient("https://api.cake.me", cake.WithClient(http.DefaultClient))
	require.NoError(t, err)
	cNvidia, err := nvidia.NewClient("https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite", nvidia.WithClient(http.DefaultClient))
	require.NoError(t, err)
	cTsmc := tsmc.NewClient("https://careers.tsmc.com", http.DefaultClient)
	cGoogle := google.NewClient("https://www.google.com/about/careers/applications", http.DefaultClient)
	cLinkedin := linkedin.NewClient("https://www.linkedin.com", http.DefaultClient)
	cIndeed := indeed.NewClient("https://apis.indeed.com/graphql", http.DefaultClient)
	cJobindex := jobindex.NewClient("https://www.jobindex.dk", http.DefaultClient)
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient)
	require.NoError(t, err)
	server := newServer(providerClients{
		amazon:   cAmazon,
		job104:   c104,
		apple:    cApple,
		cake:     cCake,
		nvidia:   cNvidia,
		tsmc:     cTsmc,
		google:   cGoogle,
		linkedin: cLinkedin,
		indeed:   cIndeed,
		jobindex: cJobindex,
	}, registry, slog.New(slog.NewTextHandler(io.Discard, nil)))
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "v0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer clientSession.Close()

	res, err := clientSession.ListTools(ctx, nil)
	require.NoError(t, err)
	got := make(map[string]*mcp.Tool, len(res.Tools))
	for _, tool := range res.Tools {
		got[tool.Name] = tool
	}
	for _, name := range []string{
		"amazon_search_jobs",
		"amazon_get_job_detail",
		"104_search_jobs",
		"104_get_job_detail",
		"apple_search_jobs",
		"apple_get_job_detail",
		"cake_search_jobs",
		"cake_get_job_detail",
		"nvidia_search_jobs",
		"nvidia_get_job_detail",
		"tsmc_search_jobs",
		"tsmc_get_job_detail",
		"google_search_jobs",
		"google_get_job_detail",
		"linkedin_search_jobs",
		"linkedin_get_job_detail",
		"indeed_search_jobs",
		"indeed_get_job_detail",
		"jobindex_search_jobs",
		"jobindex_get_job_detail",
		"mynavi_search_jobs",
		"mynavi_get_job_detail",
		"search_jobs_by_company",
		"get_filters_by_company",
		"get_job_detail_by_company",
	} {
		tool := got[name]
		require.NotNil(t, tool, name)
		assert.NotEmpty(t, tool.Description, name)
		assert.NotNil(t, tool.InputSchema, name)
		assert.NotNil(t, tool.OutputSchema, name)
		require.NotNil(t, tool.Annotations, name)
		assert.NotEmpty(t, tool.Annotations.Title, name)
		assert.True(t, tool.Annotations.ReadOnlyHint, name)
	}

	companyTool := got["search_jobs_by_company"]
	assert.Equal(t, "Search official job postings for a specific company.", companyTool.Description)
	assert.Equal(t, "Get company-specific filters when a job search needs narrowing beyond query and location.", got["get_filters_by_company"].Description)

	companyInput, ok := companyTool.InputSchema.(map[string]any)
	require.True(t, ok)
	companyProperties, ok := companyInput["properties"].(map[string]any)
	require.True(t, ok)
	companyProperty, ok := companyProperties["company"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), companyProperty["minLength"])
	assert.Contains(t, companyProperty["description"], "recognized public careers-page URL")
	assert.Contains(t, companyProperty["description"], "Other careers URLs are unsupported")
	assert.Contains(t, companyProperty["description"], "some ATS providers accept URLs only for companies in the curated roster")
	assert.NotContains(t, companyProperty["description"], "Eightfold")
	assert.NotContains(t, companyProperty["description"], "SuccessFactors")

	filtersProperty, ok := companyProperties["filters"].(map[string]any)
	require.True(t, ok)
	filterValues, ok := filtersProperty["additionalProperties"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), filterValues["minItems"])
	assert.Equal(t, true, filterValues["uniqueItems"])

	pageProperty, ok := companyProperties["page"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), pageProperty["default"])

	companyOutput, ok := companyTool.OutputSchema.(map[string]any)
	require.True(t, ok)
	companyOutputProperties, ok := companyOutput["properties"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, companyOutputProperties, "next_cursor")

	nvidiaTool := got["nvidia_search_jobs"]
	nvidiaInput, ok := nvidiaTool.InputSchema.(map[string]any)
	require.True(t, ok)
	nvidiaProperties, ok := nvidiaInput["properties"].(map[string]any)
	require.True(t, ok)
	nvidiaLimit, ok := nvidiaProperties["limit"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(20), nvidiaLimit["default"])

	nvidiaOutput, ok := nvidiaTool.OutputSchema.(map[string]any)
	require.True(t, ok)
	nvidiaOutputProperties, ok := nvidiaOutput["properties"].(map[string]any)
	require.True(t, ok)
	nvidiaData, ok := nvidiaOutputProperties["data"].(map[string]any)
	require.True(t, ok)
	nvidiaItems, ok := nvidiaData["items"].(map[string]any)
	require.True(t, ok)
	nvidiaItemProperties, ok := nvidiaItems["properties"].(map[string]any)
	require.True(t, ok)
	nvidiaExternalPath, ok := nvidiaItemProperties["external_path"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, nvidiaExternalPath["description"], "nvidia_get_job_detail")

	indeedTool := got["indeed_search_jobs"]
	assert.Equal(t, "Search job postings on Indeed.", indeedTool.Description)
	assert.Equal(t, "Get full details for an Indeed job posting.", got["indeed_get_job_detail"].Description)
	indeedInput, ok := indeedTool.InputSchema.(map[string]any)
	require.True(t, ok)
	indeedProperties, ok := indeedInput["properties"].(map[string]any)
	require.True(t, ok)
	indeedCountry, ok := indeedProperties["country"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Taiwan", indeedCountry["default"])
	indeedRadius, ok := indeedProperties["radius_miles"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(25), indeedRadius["default"])
}

func TestServerInstructionsDisambiguateCompanyAndSourceRouting(t *testing.T) {
	assert.Contains(t, serverInstructions, "A company name by itself is not a source selection.")
	assert.Contains(t, serverInstructions, "recognized public careers-page URLs on supported ATS providers")
	assert.Contains(t, serverInstructions, "some ATS providers accept URLs only for companies already in the curated roster")
	assert.NotContains(t, serverInstructions, "Eightfold")
	assert.NotContains(t, serverInstructions, "SuccessFactors")
	assert.NotContains(t, serverInstructions, "When the user names a site or company, use that provider's tools.")
}

// TestAmbiguousCompanyRetryInstructionIsTaught asserts the ambiguity retry
// instruction — retry with one of the careers URLs listed in the error —
// reaches the host LLM through both channels it can come from:
// serverInstructions, and each unified company tool's own company
// parameter description. All three tools need it independently since a
// host may only ever read the tool description for the one it calls.
func TestAmbiguousCompanyRetryInstructionIsTaught(t *testing.T) {
	const retryInstruction = "If the company is ambiguous, retry with one of the careers URLs listed in the error."

	assert.Contains(t, serverInstructions, "retry the same tool with one of the listed careers URLs, not with the original name")

	ctx := t.Context()
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient)
	require.NoError(t, err)
	server := newServer(providerClients{}, registry, slog.New(slog.NewTextHandler(io.Discard, nil)))
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "v0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer clientSession.Close()

	res, err := clientSession.ListTools(ctx, nil)
	require.NoError(t, err)
	got := make(map[string]*mcp.Tool, len(res.Tools))
	for _, tool := range res.Tools {
		got[tool.Name] = tool
	}

	for _, name := range []string{"search_jobs_by_company", "get_filters_by_company", "get_job_detail_by_company"} {
		tool := got[name]
		require.NotNil(t, tool, name)
		input, ok := tool.InputSchema.(map[string]any)
		require.True(t, ok, name)
		properties, ok := input["properties"].(map[string]any)
		require.True(t, ok, name)
		companyProperty, ok := properties["company"].(map[string]any)
		require.True(t, ok, name)
		assert.Contains(t, companyProperty["description"], retryInstruction, name)
	}
}

func TestRunWithTransportTreatsStdinEOFAsCleanExit(t *testing.T) {
	transport := &mcp.IOTransport{
		Reader: io.NopCloser(strings.NewReader("")),
		Writer: writeCloser{Writer: io.Discard},
	}
	err := runWithTransport(transport, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
}

func TestATSRegistryIncludesTeamtailor(t *testing.T) {
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient)
	require.NoError(t, err)

	adapter, slug, err := registry.Resolve("Teamtailor")
	require.NoError(t, err)
	assert.Equal(t, "teamtailor", adapter.Name())
	assert.Equal(t, "career.teamtailor.com", slug)

	adapter, slug, err = registry.Resolve("https://unlisted.na.teamtailor.com/jobs")
	require.NoError(t, err)
	assert.Equal(t, "teamtailor", adapter.Name())
	assert.Equal(t, "unlisted.na.teamtailor.com", slug)
}

func TestATSRegistryIncludesOracle(t *testing.T) {
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient)
	require.NoError(t, err)

	adapter, slug, err := registry.Resolve("Mayo Clinic")
	require.NoError(t, err)
	assert.Equal(t, "oracle", adapter.Name())
	assert.Equal(t, "fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com/CX_1", slug)

	adapter, slug, err = registry.Resolve(
		"https://fa-example.fa.us2.oraclecloud.com/" +
			"hcmUI/CandidateExperience/en/sites/Acme/jobs",
	)
	require.NoError(t, err)
	assert.Equal(t, "oracle", adapter.Name())
	assert.Equal(
		t,
		"https://fa-example.fa.us2.oraclecloud.com/"+
			"hcmUI/CandidateExperience/en/sites/Acme/jobs",
		slug,
	)
}

func TestATSRegistryIncludesJoin(t *testing.T) {
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient)
	require.NoError(t, err)

	adapter, slug, err := registry.Resolve("Routine Labs")
	require.NoError(t, err)
	assert.Equal(t, "join", adapter.Name())
	assert.Equal(t, "routinelabs", slug)

	adapter, slug, err = registry.Resolve("https://join.com/companies/routinelabs")
	require.NoError(t, err)
	assert.Equal(t, "join", adapter.Name())
	assert.Equal(t, "routinelabs", slug)
}

func TestATSRegistryIncludesBambooHR(t *testing.T) {
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient)
	require.NoError(t, err)

	adapter, slug, err := registry.Resolve("Concept2")
	require.NoError(t, err)
	assert.Equal(t, "bamboohr", adapter.Name())
	assert.Equal(t, "concept2", slug)

	adapter, slug, err = registry.Resolve("https://unlisted.bamboohr.com/careers")
	require.NoError(t, err)
	assert.Equal(t, "bamboohr", adapter.Name())
	assert.Equal(t, "unlisted", slug)
}

// TestATSCareersURLRoundTripsThroughRegistry sweeps every roster entry of
// every production adapter: its rendered [ats.Adapter.CareersURL] must
// resolve, through the same registry the server actually runs, back to the
// (adapter, slug) it came from. Resolving through the registry rather than
// the originating adapter alone matters because Resolve polls every
// registered adapter's ParseCareersURL in order — a URL that round-trips
// within its own adapter could still be shadowed by a different adapter
// registered earlier.
//
// The enumerated ok=false set is asserted exactly (currently empty): every
// roster row of every adapter renders a URL today, so both a renderer
// regressing to ok=false and an unnamed new exemption fail this test rather
// than being silently absorbed.
func TestATSCareersURLRoundTripsThroughRegistry(t *testing.T) {
	adapters, err := atsAdapters(http.DefaultClient, http.DefaultClient)
	require.NoError(t, err)
	registry, err := ats.NewRegistry(adapters...)
	require.NoError(t, err)

	wantNoURL := map[string]bool{}

	var gotNoURL []string
	for _, a := range adapters {
		for _, c := range a.Roster() {
			careersURL, ok := a.CareersURL(c.Slug)
			if !ok {
				gotNoURL = append(gotNoURL, a.Name()+":"+c.Slug)
				continue
			}
			gotAdapter, gotSlug, err := registry.Resolve(careersURL)
			if !assert.NoError(t, err, "%s %q: resolve %q", a.Name(), c.Slug, careersURL) {
				continue
			}
			assert.Equal(t, a.Name(), gotAdapter.Name(), "%s %q: resolve %q", a.Name(), c.Slug, careersURL)
			assert.Equal(t, c.Slug, gotSlug, "%s %q: resolve %q", a.Name(), c.Slug, careersURL)
		}
	}

	for _, entry := range gotNoURL {
		assert.True(t, wantNoURL[entry], "unexpected ok=false entry %q; name it in the plan before exempting it", entry)
	}
	for entry := range wantNoURL {
		assert.Contains(t, gotNoURL, entry, "expected ok=false entry %q no longer is one", entry)
	}
}
