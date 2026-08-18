package openingsmcp

import (
	"encoding/json"
	"maps"
	"slices"
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
	assertTools(t, server,
		"freehire_search_jobs",
		"freehire_get_job_facets",
		"freehire_search_companies",
		"freehire_get_company_detail",
		"freehire_search_cities",
		"freehire_get_job_detail",
	)
}

func TestFreehireSearchJobsE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"q": "golang",
	})

	// Paging mirrors upstream's meta rather than being restated as a page
	// number.
	require.NotNil(t, output.Total)
	assert.Equal(t, 2857, *output.Total)
	require.NotNil(t, output.Limit)
	assert.Equal(t, 2, *output.Limit)
	require.NotNil(t, output.Offset)
	assert.Equal(t, 0, *output.Offset)
	require.Len(t, output.Data, 2)

	first := output.Data[0]
	assert.Equal(t, "golang-developer-sequoiaat-jfzun3rb", first.PublicSlug)
	assert.Equal(t, "Golang Developer", first.Title)
	assert.Equal(t, "sequoiaat", first.Company)
	assert.Equal(t, "sequoiaat", first.CompanySlug)
	assert.Equal(t, "Tamil Nadu, Chennai, India", first.Location)
	assert.Empty(t, first.WorkMode, "absent when the posting did not state it")
	assert.Equal(t, "freshteam", first.Source)
	assert.Equal(t, "2026-08-18", first.PostedAt)
	assert.Contains(t, first.URL, "sequoiaat.freshteam.com")
	assert.Contains(t, first.Skills, "go")

	assert.Equal(t, "marktine-technology-solutions-pvt-ltd", output.Data[1].CompanySlug)
	assert.Equal(t, "remote", output.Data[1].WorkMode)
}

// TestFreehireSearchJobsCarriesUpstreamFields covers the whole Job schema
// reaching the caller. reality in particular is the only freshness signal
// this catalogue offers and it rides on every row, while ghost stays
// absent because nothing upstream raises it.
func TestFreehireSearchJobsCarriesUpstreamFields(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"q": "golang",
	})
	require.NotEmpty(t, output.Data)
	first := output.Data[0]

	assert.Equal(t, []string{"apac"}, first.Regions)
	assert.Equal(t, []string{"in"}, first.Countries)
	assert.Equal(t, []string{"Chennai"}, first.Cities)
	assert.Equal(t, "tech", first.IsTech)
	assert.Equal(t, "sequoiaat:d-QMraWxF2kA", first.ExternalID)
	require.NotNil(t, first.ManuallyAdded)
	assert.False(t, *first.ManuallyAdded)
	assert.Equal(t, "2026-07-22", first.CreatedAt)
	assert.Equal(t, "2026-08-18", first.UpdatedAt)
	assert.Equal(t, "2026-08-18", first.LastSeenAt)
	assert.Empty(t, first.ClosedAt, "an open posting has no closed_at")
	assert.Equal(t, "2026-07-23", first.EnrichedAt)
	require.NotNil(t, first.EnrichmentVersion)
	assert.Equal(t, 2, *first.EnrichmentVersion)
	require.NotNil(t, first.ViewCount)
	assert.Equal(t, 3, *first.ViewCount)
	require.NotNil(t, first.AppliedCount)
	assert.Equal(t, 0, *first.AppliedCount)

	require.NotNil(t, first.Reality)
	assert.Equal(t, "stale", first.Reality["class"])
	assert.EqualValues(t, 27, first.Reality["age_days"])
	assert.Equal(t, false, first.Reality["fake_freshness"])

	require.NotNil(t, first.Enrichment)
	assert.Equal(t, "software_engineering", first.Enrichment["category"])
	assert.Equal(t, "INR", first.Enrichment["salary_currency"])
	assert.EqualValues(t, 2, first.Enrichment["experience_years_min"])

	// Raised only when evidence fires, and nothing raises it today.
	assert.Nil(t, first.Ghost)
}

// TestFreehireSearchJobsConvertsDescription covers the preview: upstream
// cuts it near 1000 characters in the stored HTML without closing its
// open tags, so the tool converts it rather than passing markup on.
func TestFreehireSearchJobsConvertsDescription(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"q": "golang",
	})
	require.NotEmpty(t, output.Data)
	desc := output.Data[0].Description
	assert.NotEmpty(t, desc)
	assert.NotContains(t, desc, "<p>")
	assert.NotContains(t, desc, "<div>")
}

func TestFreehireSearchJobsFilteredE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"company_slug": freehire.MockCompanySlug,
	})

	require.NotNil(t, output.Total)
	assert.Equal(t, 531, *output.Total)
	require.NotEmpty(t, output.Data)
	assert.Equal(t, "Stripe", output.Data[0].Company)
	assert.Equal(t, "stripe", output.Data[0].CompanySlug)
	assert.Equal(t, "greenhouse", output.Data[0].Source)
	assert.Equal(t, "account-executive-enterprise-grower-israel-stripe-lvjix47f", output.Data[0].PublicSlug)
}

// TestFreehireSearchJobsUnknownCompanyE2E covers the quieter of the two
// upstream failure modes: an unrecognized VALUE is an empty page, not an
// error.
func TestFreehireSearchJobsUnknownCompanyE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireSearchOutput](t, clientSession, "freehire_search_jobs", map[string]any{
		"company_slug": freehire.MockUnknownCompanySlug,
	})
	require.NotNil(t, output.Total)
	assert.Equal(t, 0, *output.Total)
	assert.Empty(t, output.Data)
}

// TestFreehireSearchJobsIgnoredParamsE2E covers the louder failure mode.
// Upstream drops a parameter no filter reads and answers with the whole
// catalogue, which is indistinguishable from a genuine result — so the
// tool refuses to hand those rows back.
func TestFreehireSearchJobsIgnoredParamsE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	text := callFreehireErr(t, clientSession, "freehire_search_jobs", map[string]any{
		"q": freehire.MockIgnoredParamsQuery,
	})
	assert.Contains(t, text, "freehire dropped 1 parameter(s)")
	assert.Contains(t, text, "country (did you mean countries?)")
	assert.Contains(t, text, "drifted from freehire's openapi.yaml")
}

func TestFreehireIgnoredParams(t *testing.T) {
	assert.NoError(t, freehireIgnoredParams(nil))

	err := freehireIgnoredParams([]freehire.IgnoredParam{
		{Param: "country", DidYouMean: freehire.NewOptString("countries")},
		{Param: "skil"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dropped 2 parameter(s)")
	assert.Contains(t, err.Error(), "country (did you mean countries?)")
	// No did_you_mean means nothing was close enough; a guess would
	// mislead more than silence.
	assert.Contains(t, err.Error(), "skil")
	assert.NotContains(t, err.Error(), "skil (did you mean")
}

func TestFreehireGetJobFacetsE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireFacetsOutput](t, clientSession, "freehire_get_job_facets", map[string]any{
		"q": "golang",
	})
	assert.Equal(t, 2857, output.Total)
	assert.Len(t, output.Facets, 24)
	require.Contains(t, output.Facets, "work_mode")
	assert.Equal(t, 467, output.Facets["work_mode"]["remote"])

	// Nothing trims the distributions: upstream's own facets= selector is
	// the throttle, so the whole vocabulary reaches the caller.
	require.Contains(t, output.Facets, "skills")
	assert.Len(t, output.Facets["skills"], 536)
	assert.Equal(t, 2462, output.Facets["skills"]["go"])

	// The continuous facets arrive as ranges rather than counts.
	require.Contains(t, output.Stats, "salary_min")
	assert.EqualValues(t, 40000000, output.Stats["salary_min"]["max"])
	require.Contains(t, output.Stats, "experience_years_min")
}

// TestFreehireGetJobFacetsSelectsFacets covers upstream's own throttle:
// the unnarrowed answer runs to thousands of values.
func TestFreehireGetJobFacetsSelectsFacets(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	params, err := freehireFacetsParams(&freehireFacetsInput{Facets: "work_mode, seniority"})
	require.NoError(t, err)
	assert.Equal(t, "work_mode, seniority", params.Facets.Or(""))

	// The mock serves one canned distribution, so the selector is checked
	// on the request; the response shape is covered above.
	output := callFreehire[freehireFacetsOutput](t, clientSession, "freehire_get_job_facets", map[string]any{
		"q":      "golang",
		"facets": "work_mode,seniority",
	})
	assert.Contains(t, slices.Sorted(maps.Keys(output.Facets)), "work_mode")
}

// TestFreehireFacetsParamsPassesDisjunctive covers the flag that makes a
// filtered facet still report its other values — the numbers a caller
// needs to decide which filter to relax.
func TestFreehireFacetsParamsPassesDisjunctive(t *testing.T) {
	params, err := freehireFacetsParams(&freehireFacetsInput{})
	require.NoError(t, err)
	assert.False(t, params.Disjunctive.Set, "unset stays absent")

	params, err = freehireFacetsParams(&freehireFacetsInput{Disjunctive: ptr(true)})
	require.NoError(t, err)
	assert.True(t, params.Disjunctive.Or(false))
}

func TestFreehireSearchCompaniesE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireCompaniesOutput](t, clientSession, "freehire_search_companies", map[string]any{
		"q": freehire.MockCompanyQuery,
	})

	require.NotNil(t, output.Total)
	assert.Equal(t, 72, *output.Total)
	require.Len(t, output.Data, 3)

	// Two catalogue companies share this name, which is why resolving a
	// name has to hand back a list rather than choose.
	assert.Equal(t, "adria-solutions", output.Data[0]["slug"])
	assert.Equal(t, "Adria Solutions", output.Data[0]["name"])
	assert.EqualValues(t, 67, output.Data[0]["job_count"])
	assert.Equal(t, "adria-solutions-ltd", output.Data[1]["slug"])
	assert.EqualValues(t, 51, output.Data[1]["job_count"])

	// Rows pass through whole, so every field upstream serves is here.
	assert.Equal(t, "US", output.Data[2]["hq_country"])
	assert.Contains(t, output.Data[2], "tagline")
	assert.Contains(t, output.Data[2], "feedback_count")
}

func TestFreehireSearchCompaniesNoMatchE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireCompaniesOutput](t, clientSession, "freehire_search_companies", map[string]any{
		"q": freehire.MockNoMatchCompanyQuery,
	})
	require.NotNil(t, output.Total)
	assert.Equal(t, 0, *output.Total)
	assert.Empty(t, output.Data)
}

// TestFreehireGetCompanyDetailE2E covers the reason this tool exists: the
// industries, maturity, and yc_* vocabularies have no facets endpoint, so
// a company profile is the only place their real values can be read — and
// they are not spelled the way openapi.yaml's examples suggest.
func TestFreehireGetCompanyDetailE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireCompanyDetailOutput](t, clientSession, "freehire_get_company_detail", map[string]any{
		"company_slug": freehire.MockCompanySlug,
	})

	require.NotNil(t, output.Company)
	assert.Equal(t, "stripe", output.Company["slug"])
	assert.Equal(t, "Stripe", output.Company["name"])
	assert.Equal(t, "enterprise", output.Company["maturity"])
	assert.EqualValues(t, 2012, output.Company["year_founded"])
	assert.Equal(t, []any{"Summer 2009"}, output.Company["yc_batch"], "not the documented W21 form")
	assert.Equal(t, []any{"Active"}, output.Company["yc_status"])
	assert.Contains(t, output.Company, "company_info")
	assert.Contains(t, output.Company, "remote_regions")

	require.Len(t, output.Jobs, 1)
	assert.Equal(t, "stripe", output.Jobs[0].CompanySlug)
	require.NotNil(t, output.ReferralAvailable)
	assert.False(t, *output.ReferralAvailable)
}

func TestFreehireGetCompanyDetailRequiresSlug(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	text := callFreehireErr(t, clientSession, "freehire_get_company_detail", map[string]any{})
	assert.Contains(t, text, `required: missing properties: ["company_slug"]`)
}

// TestFreehireSearchCitiesE2E covers the resolver the cities filter needs:
// the filter holds display names and matches nothing on a near miss.
func TestFreehireSearchCitiesE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireCitiesOutput](t, clientSession, "freehire_search_cities", map[string]any{
		"q":       freehire.MockCityQuery,
		"country": freehire.MockCityCountry,
	})

	require.NotEmpty(t, output.Data)
	assert.Equal(t, "London", output.Data[0].Value)
	assert.Equal(t, "gb", output.Data[0].Country)
}

func TestFreehireGetJobDetailE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	output := callFreehire[freehireJob](t, clientSession, "freehire_get_job_detail", map[string]any{
		"job_id": freehire.MockJobSlug,
	})

	assert.Equal(t, freehire.MockJobSlug, output.PublicSlug)
	assert.Equal(t, "Program Manager, Intake & Portfolio Management", output.Title)
	assert.Equal(t, "Stripe", output.Company)
	assert.Equal(t, "stripe", output.CompanySlug)
	assert.Equal(t, "greenhouse", output.Source)
	assert.Equal(t, "remote", output.WorkMode)
	assert.Contains(t, output.URL, "gh_jid=7569678")

	// Unlike the search preview, detail is the whole body.
	assert.Contains(t, output.Description, "Stripe")
	assert.NotContains(t, output.Description, "<p>")
	assert.Greater(t, len(output.Description), 1000)

	assert.Equal(t, []string{"global"}, output.Regions)
	// posted_at says 2026-08-17 but freehire first saw it on 2026-07-08:
	// the gap is the repost reality reports.
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

func TestFreehireSearchParamsMapsEveryFilter(t *testing.T) {
	params, err := freehireSearchParams(&freehireSearchInput{
		freehireFilters: freehireFilters{
			Q:                  "golang",
			Regions:            []string{"eu"},
			Countries:          "de, at",
			Cities:             "Berlin",
			WorkMode:           []string{"remote"},
			Category:           "backend",
			Role:               "backend,android_developer",
			Seniority:          []string{"staff"},
			Skills:             "go, rust",
			SkillsMode:         "and",
			IsTech:             []string{"tech"},
			AIArchetype:        []string{"agent_builder"},
			Collections:        "yc,unicorn",
			Reality:            []string{"fresh"},
			Source:             "greenhouse,lever",
			CompanySlug:        "stripe",
			EmploymentType:     []string{"full_time"},
			Relocation:         []string{"supported"},
			EnglishLevel:       []string{"c1"},
			EducationLevel:     []string{"bachelor"},
			PostingLanguage:    "en,de",
			Domains:            []string{"fintech"},
			CompanyType:        []string{"product"},
			CompanySize:        []string{"1000+"},
			SalaryCurrency:     "EUR",
			SalaryPeriod:       []string{"year"},
			VisaSponsorship:    ptr(true),
			SalaryMin:          ptr(100000),
			SalaryMax:          ptr(200000),
			ExperienceYearsMin: ptr(5),
			PostedWithinDays:   ptr(7),
		},
		Sort:               "posted_at",
		Order:              "desc",
		Limit:              ptr(50),
		Offset:             ptr(100),
		RegionsExclude:     "mena",
		CountriesExclude:   "ru",
		WorkModeExclude:    "onsite",
		SkillsExclude:      "php",
		SourceExclude:      "adzuna",
		CompanySlugExclude: "acme",
	})
	require.NoError(t, err)

	assert.Equal(t, "golang", params.Q.Or(""))
	assert.Equal(t, []freehire.RegionsItem{freehire.RegionsItemEu}, params.Regions)
	assert.Equal(t, []string{"de", "at"}, params.Countries)
	assert.Equal(t, []string{"Berlin"}, params.Cities)
	assert.Equal(t, []freehire.WorkModeItem{freehire.WorkModeItemRemote}, params.WorkMode)
	assert.Equal(t, []string{"backend"}, params.Category)
	assert.Equal(t, []string{"backend", "android_developer"}, params.Role)
	assert.Equal(t, []freehire.SeniorityItem{freehire.SeniorityItemStaff}, params.Seniority)
	assert.Equal(t, []string{"go", "rust"}, params.Skills)
	assert.Equal(t, freehire.SkillsModeAnd, params.SkillsMode.Or(""))
	assert.Equal(t, []freehire.IsTechItem{freehire.IsTechItemTech}, params.IsTech)
	assert.Equal(t, []freehire.AIArchetypeItem{freehire.AIArchetypeItemAgentBuilder}, params.AiArchetype)
	assert.Equal(t, []string{"yc", "unicorn"}, params.Collections)
	assert.Equal(t, []freehire.RealityItem{freehire.RealityItemFresh}, params.Reality)
	assert.Equal(t, []string{"greenhouse", "lever"}, params.Source)
	assert.Equal(t, []string{"stripe"}, params.CompanySlug)
	assert.Equal(t, []freehire.EmploymentTypeItem{freehire.EmploymentTypeItemFullTime}, params.EmploymentType)
	assert.Equal(t, []freehire.RelocationItem{freehire.RelocationItemSupported}, params.Relocation)
	assert.Equal(t, []freehire.EnglishLevelItem{freehire.EnglishLevelItemC1}, params.EnglishLevel)
	assert.Equal(t, []freehire.EducationLevelItem{freehire.EducationLevelItemBachelor}, params.EducationLevel)
	assert.Equal(t, []string{"en", "de"}, params.PostingLanguage)
	assert.Equal(t, []freehire.DomainsItem{freehire.DomainsItemFintech}, params.Domains)
	assert.Equal(t, []freehire.CompanyTypeItem{freehire.CompanyTypeItemProduct}, params.CompanyType)
	assert.Equal(t, []freehire.CompanySizeItem{freehire.CompanySizeItem1000}, params.CompanySize)
	assert.Equal(t, []string{"EUR"}, params.SalaryCurrency)
	assert.Equal(t, []freehire.SalaryPeriodItem{freehire.SalaryPeriodItemYear}, params.SalaryPeriod)
	assert.True(t, params.VisaSponsorship.Or(false))
	assert.Equal(t, 100000, params.SalaryMin.Or(0))
	assert.Equal(t, 200000, params.SalaryMax.Or(0))
	assert.Equal(t, 5, params.ExperienceYearsMin.Or(0))
	assert.Equal(t, 7, params.PostedWithinDays.Or(0))
	assert.Equal(t, freehire.SortPostedAt, params.Sort.Or(""))
	assert.Equal(t, freehire.OrderDesc, params.Order.Or(""))
	assert.Equal(t, 50, params.Limit.Or(0))
	assert.Equal(t, 100, params.Offset.Or(-1))
	assert.Equal(t, []string{"mena"}, params.RegionsExclude)
	assert.Equal(t, []string{"ru"}, params.CountriesExclude)
	assert.Equal(t, []string{"onsite"}, params.WorkModeExclude)
	assert.Equal(t, []string{"php"}, params.SkillsExclude)
	assert.Equal(t, []string{"adzuna"}, params.SourceExclude)
	assert.Equal(t, []string{"acme"}, params.CompanySlugExclude)
}

// TestFreehireSearchParamsOmitsUnset covers the difference between "not
// asked for" and a zero value. visa_sponsorship=false is a stated value
// upstream, so an unset filter must not send it.
func TestFreehireSearchParamsOmitsUnset(t *testing.T) {
	params, err := freehireSearchParams(&freehireSearchInput{})
	require.NoError(t, err)
	assert.False(t, params.Q.Set)
	assert.False(t, params.Limit.Set, "an unset limit lets upstream apply its own default")
	assert.False(t, params.Offset.Set)
	assert.False(t, params.VisaSponsorship.Set)
	assert.False(t, params.SalaryMin.Set)
	assert.Empty(t, params.Regions)

	params, err = freehireSearchParams(&freehireSearchInput{
		freehireFilters: freehireFilters{VisaSponsorship: ptr(false), SalaryMin: ptr(0)},
	})
	require.NoError(t, err)
	require.True(t, params.VisaSponsorship.Set)
	assert.False(t, params.VisaSponsorship.Value)
	require.True(t, params.SalaryMin.Set)
	assert.Equal(t, 0, params.SalaryMin.Value)
}

// TestFreehireFacetsParamsSharesFilters covers that the facets tool takes
// the same filters as the search, so its counts describe the slice the
// caller is about to search.
func TestFreehireFacetsParamsSharesFilters(t *testing.T) {
	params, err := freehireFacetsParams(&freehireFacetsInput{
		freehireFilters: freehireFilters{
			Q:                "golang",
			Cities:           "London",
			Role:             "backend",
			Reality:          []string{"fresh"},
			PostedWithinDays: ptr(7),
			SalaryMin:        ptr(100000),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "golang", params.Q.Or(""))
	assert.Equal(t, []string{"London"}, params.Cities)
	assert.Equal(t, []string{"backend"}, params.Role)
	assert.Equal(t, []freehire.RealityItem{freehire.RealityItemFresh}, params.Reality)
	assert.Equal(t, 7, params.PostedWithinDays.Or(0))
	assert.Equal(t, 100000, params.SalaryMin.Or(0))
}

func TestFreehireCompaniesParamsMapsEveryFilter(t *testing.T) {
	params, err := freehireCompaniesParams(&freehireCompaniesInput{
		Q:             "adria",
		Limit:         ptr(3),
		Offset:        ptr(10),
		Sort:          "rating",
		Collections:   "yc,unicorn",
		Regions:       "eu",
		Countries:     "de",
		RemoteRegions: "global",
		Industries:    "fintech, developer-tools",
		Domains:       "devtools",
		CompanyType:   []string{"product"},
		CompanySize:   []string{"1000+"},
		Maturity:      "enterprise",
		YcBatch:       "Summer 2009",
		YcStatus:      "Active",
		YcStage:       "Growth",
		YcFlags:       "top_company,hiring",
	})
	require.NoError(t, err)

	assert.Equal(t, "adria", params.Q.Or(""))
	assert.Equal(t, 3, params.Limit.Or(0))
	assert.Equal(t, 10, params.Offset.Or(-1))
	assert.Equal(t, freehire.SearchCompaniesSortRating, params.Sort.Or(""))
	assert.Equal(t, []string{"yc", "unicorn"}, params.Collections)
	assert.Equal(t, []string{"eu"}, params.Regions)
	assert.Equal(t, []string{"de"}, params.Countries)
	assert.Equal(t, []string{"global"}, params.RemoteRegions)
	assert.Equal(t, []string{"fintech", "developer-tools"}, params.Industries)
	assert.Equal(t, []string{"devtools"}, params.Domains)
	assert.Equal(t, []freehire.SearchCompaniesCompanyTypeItem{freehire.SearchCompaniesCompanyTypeItemProduct}, params.CompanyType)
	assert.Equal(t, []freehire.SearchCompaniesCompanySizeItem{freehire.SearchCompaniesCompanySizeItem1000}, params.CompanySize)
	assert.Equal(t, []string{"enterprise"}, params.Maturity)
	// Written out in full, not the W21 form openapi.yaml gives as its
	// example.
	assert.Equal(t, []string{"Summer 2009"}, params.YcBatch)
	assert.Equal(t, []string{"Active"}, params.YcStatus)
	assert.Equal(t, []string{"Growth"}, params.YcStage)
	assert.Equal(t, []string{"top_company", "hiring"}, params.YcFlags)
}

// TestFreehireGeoWarningReachesEveryGeographyFilter covers the one piece
// of upstream semantics a caller cannot guess: the three geography
// filters OR together, so adding a second widens the search.
func TestFreehireGeoWarningReachesEveryGeographyFilter(t *testing.T) {
	var schema struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(freehireSearchInputRawSchema, &schema))
	for _, name := range []string{"regions", "countries", "cities"} {
		require.Contains(t, schema.Properties, name)
		assert.Contains(t, schema.Properties[name].Description, "ONE OR-group",
			"%s must warn that it widens rather than narrows", name)
	}
}

func TestFreehirePassthroughForwardsUndeclaredKeys(t *testing.T) {
	var reality freehire.Reality
	require.NoError(t, reality.UnmarshalJSON([]byte(`{"class":"fresh","age_days":3,"brand_new_signal":true}`)))

	out := freehirePassthrough(&reality)
	assert.Equal(t, "fresh", out["class"])
	assert.EqualValues(t, 3, out["age_days"])
	// The spec names five fields and allows extras; a key this build has
	// never heard of still reaches the caller.
	assert.Equal(t, true, out["brand_new_signal"])
}

// TestFreehireSearchOmitsUnknownPagination covers a response without the
// spec-optional meta: reporting total 0 beside a full page would read as
// "no matches".
func TestFreehireSearchOmitsUnknownPagination(t *testing.T) {
	out, err := freehireSearchOutputOf(&freehire.JobListEnvelope{
		Data: []freehire.Job{{PublicSlug: "a", Title: "t", Company: "c"}},
	})
	require.NoError(t, err)
	assert.Nil(t, out.Total)
	assert.Nil(t, out.Limit)
	assert.Nil(t, out.Offset)
	require.Len(t, out.Data, 1)
}

func TestFreehireSearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	text := callFreehireErr(t, clientSession, "freehire_search_jobs", map[string]any{
		"seniority": []any{"valueNotInEnum"},
	})
	assert.Contains(t, text, `validating /properties/seniority/items: enum`)
}

// TestFreehireSearchJobsRejectsUnknownParam covers the schema guard that
// keeps meta.ignored_params from ever firing on a caller's behalf: a name
// upstream would silently drop is refused here instead.
func TestFreehireSearchJobsRejectsUnknownParam(t *testing.T) {
	clientSession := testFreehireMCPClientServer(t)

	text := callFreehireErr(t, clientSession, "freehire_search_jobs", map[string]any{
		"country": "it",
	})
	assert.Contains(t, text, `unexpected additional properties ["country"]`)
}

func ptr[T any](v T) *T { return &v }
