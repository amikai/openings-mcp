package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
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
	"github.com/amikai/openings-mcp/internal/provider/mynavi"
	"github.com/amikai/openings-mcp/internal/provider/nvidia"
	"github.com/amikai/openings-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/peterbourgon/ff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// update regenerates testdata/company_collisions.txt from the current
// production rosters. Run with:
//
//	go test ./cmd/openings-mcp -run TestCompanyCollisionReport -update
var update = flag.Bool("update", false, "regenerate the collision-report golden file")

// collisionGoldenPath is the golden file TestCompanyCollisionReport compares
// against. NewRegistry no longer fails on a cross-adapter slug or name
// collision, so this report is what keeps new ones visible.
const collisionGoldenPath = "testdata/company_collisions.txt"

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error { return nil }

// Flags must beat the environment, and an unparseable env value must fail
// startup rather than silently falling back to the default.
func TestFlagsBeatEnvVars(t *testing.T) {
	const envKey = envVarPrefix + "_ENABLE_COMMAND_LOGGING"

	for _, tc := range []struct {
		name    string
		args    []string
		env     string
		want    bool
		wantErr bool
	}{
		{name: "unset", want: false},
		{name: "flag only", args: []string{"--enable-command-logging"}, want: true},
		{name: "env true", env: "true", want: true},
		{name: "env 1", env: "1", want: true},
		{name: "env false", env: "false", want: false},
		{name: "flag false overrides env true", args: []string{"--enable-command-logging=false"}, env: "true", want: false},
		{name: "flag true overrides env false", args: []string{"--enable-command-logging"}, env: "false", want: true},
		{name: "unparseable env fails", env: "on", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv(envKey, tc.env)
			}
			fs := ff.NewFlagSet("openings-mcp")
			got := fs.BoolLong("enable-command-logging", "usage")
			cmd := &ff.Command{Name: "openings-mcp", Flags: fs}

			err := cmd.Parse(tc.args, ff.WithEnvVarPrefix(envVarPrefix))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, *got)
		})
	}
}

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
	cMynavi := mynavi.NewClient("https://tenshoku.mynavi.jp", http.DefaultClient)

	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient, nil)
	require.NoError(t, err)
	server := newServer(&providerClients{
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
		mynavi:   cMynavi,
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
		assertToolContract(t, got, name)
	}

	assertCompanyAndIndeedSchemas(t, got)

}

// assertToolContract holds every registered tool to the same baseline: hosts
// route on the description and annotations, and gate write-confirmation
// prompts on ReadOnlyHint, so a tool missing any of them misbehaves in the
// client rather than failing here.
func assertToolContract(t *testing.T, got map[string]*mcp.Tool, name string) {
	t.Helper()

	tool := got[name]
	require.NotNil(t, tool, name)
	assert.NotEmpty(t, tool.Description, name)
	assert.NotNil(t, tool.InputSchema, name)
	assert.NotNil(t, tool.OutputSchema, name)
	require.NotNil(t, tool.Annotations, name)
	assert.NotEmpty(t, tool.Annotations.Title, name)
	assert.True(t, tool.Annotations.ReadOnlyHint, name)
}

// assertCompanyAndIndeedSchemas pins the schema details the server
// instructions promise the host LLM; they hold in both tool sets.
func assertCompanyAndIndeedSchemas(t *testing.T, got map[string]*mcp.Tool) {
	t.Helper()

	companyTool := got["search_jobs_by_company"]
	require.NotNil(t, companyTool)
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
	assert.Contains(t, companyProperty["description"], "some career systems accept URLs only for companies in the curated roster")
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
	require.NotNil(t, nvidiaTool)
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
	require.NotNil(t, indeedTool)
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
	assert.Contains(t, serverInstructions, "recognized public careers-page URLs from the career systems this server supports")
	assert.Contains(t, serverInstructions, "some career systems accept URLs only for companies already in the curated roster")
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
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient, nil)
	require.NoError(t, err)
	server := newServer(&providerClients{}, registry, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
		"search_jobs_by_company",
		"get_filters_by_company",
		"get_job_detail_by_company",
	} {
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

func TestRunStdioTreatsStdinEOFAsCleanExit(t *testing.T) {
	transport := &mcp.IOTransport{
		Reader: io.NopCloser(strings.NewReader("")),
		Writer: writeCloser{Writer: io.Discard},
	}
	err := runStdio(transport, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	require.NoError(t, err)
}

// TestStreamableHTTPServesTools covers the --http path end to end: the same
// server reachable over streamable HTTP, listing tools through a real client.
func TestStreamableHTTPServesTools(t *testing.T) {
	ctx := t.Context()
	server, err := newProviderServer(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	require.NoError(t, err)

	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	))
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "v0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	require.NoError(t, err)
	defer session.Close()

	res, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	names := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	assert.Contains(t, names, "search_jobs_by_company")
	assert.Contains(t, names, "linkedin_search_jobs")
}

func TestATSRegistryIncludesTeamtailor(t *testing.T) {
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient, nil)
	require.NoError(t, err)

	resolution, err := registry.Resolve("Teamtailor")
	require.NoError(t, err)
	adapter, slug, ok := resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "teamtailor", adapter.Name())
	assert.Equal(t, "career.teamtailor.com", slug)

	resolution, err = registry.Resolve("https://unlisted.na.teamtailor.com/jobs")
	require.NoError(t, err)
	adapter, slug, ok = resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "teamtailor", adapter.Name())
	assert.Equal(t, "unlisted.na.teamtailor.com", slug)
}

func TestATSRegistryIncludesOracle(t *testing.T) {
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient, nil)
	require.NoError(t, err)

	resolution, err := registry.Resolve("Mayo Clinic")
	require.NoError(t, err)
	adapter, slug, ok := resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "oracle", adapter.Name())
	assert.Equal(t, "fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com/CX_1", slug)

	resolution, err = registry.Resolve(
		"https://fa-example.fa.us2.oraclecloud.com/" +
			"hcmUI/CandidateExperience/en/sites/Acme/jobs",
	)
	require.NoError(t, err)
	adapter, slug, ok = resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "oracle", adapter.Name())
	assert.Equal(
		t,
		"https://fa-example.fa.us2.oraclecloud.com/"+
			"hcmUI/CandidateExperience/en/sites/Acme/jobs",
		slug,
	)
}

func TestATSRegistryIncludesJoin(t *testing.T) {
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient, nil)
	require.NoError(t, err)

	resolution, err := registry.Resolve("Routine Labs")
	require.NoError(t, err)
	adapter, slug, ok := resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "join", adapter.Name())
	assert.Equal(t, "routinelabs", slug)

	resolution, err = registry.Resolve("https://join.com/companies/routinelabs")
	require.NoError(t, err)
	adapter, slug, ok = resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "join", adapter.Name())
	assert.Equal(t, "routinelabs", slug)
}

func TestATSRegistryIncludesBambooHR(t *testing.T) {
	registry, err := newATSRegistry(http.DefaultClient, http.DefaultClient, nil)
	require.NoError(t, err)

	resolution, err := registry.Resolve("Concept2")
	require.NoError(t, err)
	adapter, slug, ok := resolution.Single()
	require.True(t, ok)
	assert.Equal(t, "bamboohr", adapter.Name())
	assert.Equal(t, "concept2", slug)

	resolution, err = registry.Resolve("https://unlisted.bamboohr.com/careers")
	require.NoError(t, err)
	adapter, slug, ok = resolution.Single()
	require.True(t, ok)
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
	adapters, err := atsAdapters(http.DefaultClient, http.DefaultClient, nil)
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
			resolution, err := registry.Resolve(careersURL)
			if !assert.NoError(t, err, "%s %q: resolve %q", a.Name(), c.Slug, careersURL) {
				continue
			}
			if !assert.False(t, resolution.IsAmbiguous(), "%s %q: resolve %q", a.Name(), c.Slug, careersURL) {
				continue
			}
			gotAdapter, gotSlug, ok := resolution.Select(0)
			if !assert.True(t, ok, "%s %q: resolve %q", a.Name(), c.Slug, careersURL) {
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

// TestCompanyCollisionReport walks every roster entry of every production
// adapter (the same atsAdapters enumeration TestATSCareersURLRoundTripsThroughRegistry
// uses) and resolves both its slug and its display name through the
// production registry. Any input that resolves to multiple candidates is a
// collision, recorded as one line in testdata/company_collisions.txt.
//
// It keys by Resolve rather than by a private normalizer: internal/ats is
// out of scope for this slice, and a reimplemented normalizer could drift
// from the one the resolver actually uses, pinning a baseline that
// describes collisions callers don't actually experience.
func TestCompanyCollisionReport(t *testing.T) {
	adapters, err := atsAdapters(http.DefaultClient, http.DefaultClient, nil)
	require.NoError(t, err)
	registry, err := ats.NewRegistry(adapters...)
	require.NoError(t, err)

	findings := map[string][]ats.CompanyCandidate{}
	probed := map[string]bool{}
	for _, a := range adapters {
		for _, c := range a.Roster() {
			for _, probe := range []string{c.Slug, c.Name} {
				if probed[probe] {
					continue
				}
				probed[probe] = true

				resolution, err := registry.Resolve(probe)
				require.NoError(t, err)
				if resolution.IsAmbiguous() {
					findings[probe] = resolution.Candidates()
				}
			}
		}
	}

	inputs := make([]string, 0, len(findings))
	for input := range findings {
		inputs = append(inputs, input)
	}
	sort.Strings(inputs)

	var b strings.Builder
	for _, input := range inputs {
		b.WriteString(input)
		for _, c := range findings[input] {
			fmt.Fprintf(&b, "\t%s|%s|%s", c.Provider, c.Name, c.CareersURL)
		}
		b.WriteString("\n")
	}
	got := b.String()

	if *update {
		require.NoError(t, os.WriteFile(collisionGoldenPath, []byte(got), 0o644))
		return
	}

	want, err := os.ReadFile(collisionGoldenPath)
	require.NoError(t, err)
	if got != string(want) {
		t.Errorf(
			"%s is stale; regenerate with `go test ./cmd/openings-mcp -run TestCompanyCollisionReport -update`.\n\nExpected content:\n%s",
			collisionGoldenPath, got,
		)
	}
}

// --http carries an optional address, so its empty value is ambiguous: the
// env binding makes an exported-but-empty OPENINGS_MCP_HTTP reach Set, and
// reading that as an address would start a listener on nothing.
func TestHTTPFlagRejectsEmptyValue(t *testing.T) {
	var f httpFlag
	require.Error(t, f.Set(""))
	assert.False(t, f.enabled)

	require.NoError(t, f.Set("true"))
	assert.True(t, f.enabled)
	assert.Equal(t, defaultHTTPAddr, f.addr)

	require.NoError(t, f.Set(":9000"))
	assert.True(t, f.enabled)
	assert.Equal(t, ":9000", f.addr)
}

func TestHTTPFlagEmptyEnvIsAStartupError(t *testing.T) {
	t.Setenv(envVarPrefix+"_HTTP", "")

	fs := ff.NewFlagSet("openings-mcp")
	_, err := fs.AddFlag(ff.FlagConfig{LongName: "http", Value: &httpFlag{addr: defaultHTTPAddr}, NoPlaceholder: true})
	require.NoError(t, err)
	cmd := &ff.Command{Name: "openings-mcp", Flags: fs}

	require.ErrorContains(t, cmd.Parse(nil, ff.WithEnvVarPrefix(envVarPrefix)), "empty value")
}
