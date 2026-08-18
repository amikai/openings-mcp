package main

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/freehire"
)

func ptr[T any](v T) *T { return &v }

// newTestFilterFlags mirrors what registerFilterFlags leaves behind when
// the user gave no filter: empty strings, and unsetInt for the numeric
// bounds so that a real zero stays distinguishable from "not asked for".
func newTestFilterFlags() *filterFlags {
	return &filterFlags{
		q: ptr(""), regions: ptr(""), countries: ptr(""), cities: ptr(""),
		workMode: ptr(""), category: ptr(""), role: ptr(""), seniority: ptr(""),
		skills: ptr(""), skillsMode: ptr(""), isTech: ptr(""), aiArchetype: ptr(""),
		collections: ptr(""), reality: ptr(""), source: ptr(""), companySlug: ptr(""),
		employmentType: ptr(""), relocation: ptr(""), englishLevel: ptr(""),
		educationLevel: ptr(""), postingLanguage: ptr(""), domains: ptr(""),
		companyType: ptr(""), companySize: ptr(""), salaryCurrency: ptr(""),
		salaryPeriod: ptr(""), visaSponsorship: ptr(""),
		salaryMin: ptr(unsetInt), salaryMax: ptr(unsetInt),
		experienceYearsMin: ptr(unsetInt), postedWithinDays: ptr(unsetInt),
	}
}

func TestSplitCSV(t *testing.T) {
	assert.Nil(t, splitCSV(""))
	assert.Equal(t, []string{"go", "rust"}, splitCSV("go,rust"))
	assert.Equal(t, []string{"go", "rust"}, splitCSV(" go, rust "))
	assert.Equal(t, []string{"go"}, splitCSV("go,,"))
}

func TestSearchParams(t *testing.T) {
	f := newTestFilterFlags()
	*f.q = "golang"
	*f.companySlug = "stripe"
	*f.skills = "go,rust"
	*f.skillsMode = "and"
	*f.seniority = "senior"
	*f.workMode = "remote"
	*f.regions = "eu"
	*f.countries = "de"
	*f.cities = "Berlin"
	*f.role = "backend"
	*f.reality = "fresh"
	*f.source = "greenhouse"
	*f.category = "backend"
	*f.isTech = "tech"
	*f.employmentType = "full_time"
	*f.domains = "fintech"
	*f.companySize = "1000+"
	*f.salaryPeriod = "year"
	*f.salaryCurrency = "EUR"
	*f.visaSponsorship = "true"
	*f.salaryMin = 100000
	*f.postedWithinDays = 7

	params, err := searchParams(searchOpts{
		filters:        f,
		sortField:      "posted_at",
		order:          "desc",
		limit:          20,
		offset:         40,
		sourceExclude:  "adzuna",
		skillsExclude:  "php",
		regionsExclude: "mena",
	})
	require.NoError(t, err)

	assert.Equal(t, "golang", params.Q.Or(""))
	assert.Equal(t, []string{"stripe"}, params.CompanySlug)
	assert.Equal(t, []string{"go", "rust"}, params.Skills)
	assert.Equal(t, freehire.SkillsModeAnd, params.SkillsMode.Or(""))
	assert.Equal(t, []freehire.SeniorityItem{freehire.SeniorityItemSenior}, params.Seniority)
	assert.Equal(t, []freehire.WorkModeItem{freehire.WorkModeItemRemote}, params.WorkMode)
	assert.Equal(t, []freehire.RegionsItem{freehire.RegionsItemEu}, params.Regions)
	assert.Equal(t, []string{"de"}, params.Countries)
	assert.Equal(t, []string{"Berlin"}, params.Cities)
	assert.Equal(t, []string{"backend"}, params.Role)
	assert.Equal(t, []freehire.RealityItem{freehire.RealityItemFresh}, params.Reality)
	assert.Equal(t, []string{"greenhouse"}, params.Source)
	assert.Equal(t, []string{"backend"}, params.Category)
	assert.Equal(t, []freehire.IsTechItem{freehire.IsTechItemTech}, params.IsTech)
	assert.Equal(t, []freehire.EmploymentTypeItem{freehire.EmploymentTypeItemFullTime}, params.EmploymentType)
	assert.Equal(t, []freehire.DomainsItem{freehire.DomainsItemFintech}, params.Domains)
	assert.Equal(t, []freehire.CompanySizeItem{freehire.CompanySizeItem1000}, params.CompanySize)
	assert.Equal(t, []freehire.SalaryPeriodItem{freehire.SalaryPeriodItemYear}, params.SalaryPeriod)
	assert.Equal(t, []string{"EUR"}, params.SalaryCurrency)
	assert.True(t, params.VisaSponsorship.Or(false))
	assert.Equal(t, 100000, params.SalaryMin.Or(0))
	assert.Equal(t, 7, params.PostedWithinDays.Or(0))
	assert.Equal(t, freehire.SortPostedAt, params.Sort.Or(""))
	assert.Equal(t, freehire.OrderDesc, params.Order.Or(""))
	assert.Equal(t, 40, params.Offset.Or(-1))
	assert.Equal(t, 20, params.Limit.Or(0))
	assert.Equal(t, []string{"adzuna"}, params.SourceExclude)
	assert.Equal(t, []string{"php"}, params.SkillsExclude)
	assert.Equal(t, []string{"mena"}, params.RegionsExclude)
}

// TestSearchParamsOmitsUnsetFlags covers the sentinels. Upstream defaults
// limit to 10 on its own, and visa_sponsorship=false is a stated value
// rather than "unknown", so an untouched flag must send nothing.
func TestSearchParamsOmitsUnsetFlags(t *testing.T) {
	params, err := searchParams(searchOpts{filters: newTestFilterFlags(), limit: unsetInt, offset: unsetInt})
	require.NoError(t, err)
	assert.False(t, params.Q.Set)
	assert.False(t, params.Limit.Set)
	assert.False(t, params.Offset.Set)
	assert.False(t, params.VisaSponsorship.Set)
	assert.False(t, params.SalaryMin.Set)
	assert.Empty(t, params.Regions)
	assert.Empty(t, params.SourceExclude)
}

func TestOptTristateBool(t *testing.T) {
	assert.False(t, optTristateBool("").Set, "an unset flag stays absent")
	require.True(t, optTristateBool("false").Set)
	assert.False(t, optTristateBool("false").Value, "false is a value upstream reads, not a default")
	assert.True(t, optTristateBool("true").Or(false))
}

func TestSearchParamsRejectsInvalidSort(t *testing.T) {
	_, err := searchParams(searchOpts{filters: newTestFilterFlags(), sortField: "nope", limit: unsetInt, offset: unsetInt})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --sort")
	assert.Contains(t, err.Error(), "nope")
}

func TestSearchParamsRejectsInvalidSeniority(t *testing.T) {
	f := newTestFilterFlags()
	*f.seniority = "staff,nope"
	_, err := searchParams(searchOpts{filters: f, limit: unsetInt, offset: unsetInt})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --seniority")
	assert.Contains(t, err.Error(), "nope")
}

// TestFacetsParamsSharesFilters covers that facets takes the same filters
// as the search, so its counts describe the slice about to be searched.
func TestFacetsParamsSharesFilters(t *testing.T) {
	f := newTestFilterFlags()
	*f.q = "golang"
	*f.cities = "London"
	*f.reality = "fresh"
	params, err := facetsParams(f, "skills,seniority", true)
	require.NoError(t, err)
	assert.Equal(t, "golang", params.Q.Or(""))
	assert.Equal(t, []string{"London"}, params.Cities)
	assert.Equal(t, []freehire.RealityItem{freehire.RealityItemFresh}, params.Reality)
	assert.Equal(t, "skills,seniority", params.Facets.Or(""))
	assert.True(t, params.Disjunctive.Or(false))
}

func TestCompaniesParams(t *testing.T) {
	params, err := companiesParams(companiesOpts{
		q:           "adria",
		limit:       3,
		offset:      unsetInt,
		sortField:   "rating",
		collections: "yc",
		industries:  "fintech, developer-tools",
		companyType: "product",
		companySize: "1000+",
		maturity:    "enterprise",
		ycBatch:     "Summer 2009",
		ycFlags:     "top_company,hiring",
	})
	require.NoError(t, err)
	assert.Equal(t, "adria", params.Q.Or(""))
	assert.Equal(t, 3, params.Limit.Or(0))
	assert.False(t, params.Offset.Set)
	assert.Equal(t, freehire.SearchCompaniesSortRating, params.Sort.Or(""))
	assert.Equal(t, []string{"yc"}, params.Collections)
	assert.Equal(t, []string{"fintech", "developer-tools"}, params.Industries)
	assert.Equal(t, []freehire.SearchCompaniesCompanyTypeItem{freehire.SearchCompaniesCompanyTypeItemProduct}, params.CompanyType)
	assert.Equal(t, []freehire.SearchCompaniesCompanySizeItem{freehire.SearchCompaniesCompanySizeItem1000}, params.CompanySize)
	assert.Equal(t, []string{"enterprise"}, params.Maturity)
	// Written out in full, not the W21 form openapi.yaml documents.
	assert.Equal(t, []string{"Summer 2009"}, params.YcBatch)
	assert.Equal(t, []string{"top_company", "hiring"}, params.YcFlags)
}

func TestIgnoredParams(t *testing.T) {
	assert.NoError(t, ignoredParams(nil))

	err := ignoredParams([]freehire.IgnoredParam{
		{Param: "country", DidYouMean: freehire.NewOptString("countries")},
		{Param: "skil"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dropped 2 parameter(s)")
	assert.Contains(t, err.Error(), "country (did you mean countries?)")
	assert.Contains(t, err.Error(), "skil")
}

func TestSummarize(t *testing.T) {
	u, err := url.Parse("https://example.com/job")
	require.NoError(t, err)
	job := freehire.Job{
		PublicSlug:  "golang-developer-sequoiaat-jfzun3rb",
		Title:       "Golang Developer",
		Company:     "sequoiaat",
		CompanySlug: freehire.NewOptString("sequoiaat"),
		Location:    freehire.NewOptString("Tamil Nadu, Chennai, India"),
		WorkMode:    freehire.NewOptJobWorkMode(freehire.JobWorkModeRemote),
		Source:      freehire.NewOptString("freshteam"),
		Skills:      []string{"go", "docker"},
		Cities:      []string{"Chennai"},
		IsTech:      freehire.NewOptJobIsTech(freehire.JobIsTechTech),
		PostedAt:    freehire.NewOptNilDateTime(time.Date(2026, time.August, 16, 19, 35, 12, 0, time.UTC)),
		URL:         freehire.NewOptURI(*u),
	}
	assert.Equal(t, jobJSON{
		Slug:        "golang-developer-sequoiaat-jfzun3rb",
		Title:       "Golang Developer",
		Company:     "sequoiaat",
		CompanySlug: "sequoiaat",
		Location:    "Tamil Nadu, Chennai, India",
		WorkMode:    "remote",
		Source:      "freshteam",
		Skills:      []string{"go", "docker"},
		Cities:      []string{"Chennai"},
		IsTech:      "tech",
		PostedAt:    "2026-08-16",
		URL:         "https://example.com/job",
	}, summarize(job))
}

// TestPassthroughForwardsUndeclaredKeys covers why enrichment, reality,
// and ghost are forwarded as maps: the spec names their fields but still
// allows extras.
func TestPassthroughForwardsUndeclaredKeys(t *testing.T) {
	var reality freehire.Reality
	require.NoError(t, reality.UnmarshalJSON([]byte(`{"class":"fresh","brand_new_signal":true}`)))

	out := passthrough(&reality)
	assert.Equal(t, "fresh", out["class"])
	assert.Equal(t, true, out["brand_new_signal"])
}

func TestRenderHTML(t *testing.T) {
	assert.Empty(t, renderHTML(""))
	assert.Equal(t, "one\ntwo", renderHTML("one<br/>two"))
}
