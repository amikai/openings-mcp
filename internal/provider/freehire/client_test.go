package freehire

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jobURL(j Job) string {
	if u, ok := j.URL.Get(); ok {
		return u.String()
	}
	return ""
}

func TestSearchJobs(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{
		Q:     NewOptString("golang"),
		Limit: NewOptInt(2),
	})
	require.NoError(t, err)

	meta, ok := res.Meta.Get()
	require.True(t, ok)
	assert.Equal(t, 2, meta.Limit.Or(0))
	assert.Equal(t, 0, meta.Offset.Or(-1))
	assert.Equal(t, 2857, meta.Total.Or(0))
	// A clean request omits the key entirely, which is what makes its
	// presence the drift signal.
	assert.Empty(t, meta.IgnoredParams)
	require.Len(t, res.Data, 2)

	first := res.Data[0]
	assert.Equal(t, "golang-developer-sequoiaat-jfzun3rb", first.PublicSlug)
	assert.Equal(t, "Golang Developer", first.Title)
	assert.Equal(t, "sequoiaat", first.Company)
	assert.Equal(t, "sequoiaat", first.CompanySlug.Or(""))
	assert.Equal(t, "Tamil Nadu, Chennai, India", first.Location.Or(""))
	assert.Equal(t, "freshteam", first.Source.Or(""))
	assert.False(t, first.WorkMode.Set, "work_mode is absent when the posting did not state it")
	assert.Equal(t, JobIsTechTech, first.IsTech.Or(""))
	assert.Contains(t, first.Skills, "go")
	assert.Equal(t, []string{"apac"}, first.Regions)
	assert.Equal(t, []string{"in"}, first.Countries)
	assert.Equal(t, []string{"Chennai"}, first.Cities)
	assert.True(t, first.PostedAt.Set)
	assert.True(t, first.PostedAt.Value.Equal(time.Date(2026, time.August, 18, 6, 55, 20, 0, time.UTC)))
	assert.Contains(t, jobURL(first), "sequoiaat.freshteam.com")

	// The search index serves a preview cut near 1000 characters, in the
	// stored HTML and without closing the tags it opened.
	assert.Less(t, len(first.Description.Or("")), 1001)
	assert.Contains(t, first.Description.Or(""), "<")

	// reality rides on every row; ghost is raised only when evidence
	// fires, and nothing in the live catalogue raises it today.
	reality, ok := first.Reality.Get()
	require.True(t, ok)
	assert.Equal(t, RealityClassStale, reality.Class.Or(""))
	assert.Equal(t, 27, reality.AgeDays.Or(-1))
	// openapi.yaml calls ghost nullable and says most postings carry
	// null, but the API omits the key outright.
	assert.True(t, first.Ghost.IsEmpty())

	second := res.Data[1]
	assert.Equal(t, "golang-developer-marktine-technology-solutions-pvt-ltd-3jnmgyef", second.PublicSlug)
	assert.Equal(t, "marktine-technology-solutions-pvt-ltd", second.CompanySlug.Or(""))
	assert.Equal(t, JobWorkModeRemote, second.WorkMode.Or(""))
}

func TestSearchJobsFiltered(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{
		CompanySlug: []string{MockCompanySlug},
		Limit:       NewOptInt(2),
	})
	require.NoError(t, err)

	meta, ok := res.Meta.Get()
	require.True(t, ok)
	assert.Equal(t, 531, meta.Total.Or(0))
	require.Len(t, res.Data, 2)
	assert.Equal(t, "account-executive-enterprise-grower-israel-stripe-lvjix47f", res.Data[0].PublicSlug)
	assert.Equal(t, "Stripe", res.Data[0].Company)
	assert.Equal(t, "stripe", res.Data[0].CompanySlug.Or(""))
	assert.Equal(t, "greenhouse", res.Data[0].Source.Or(""))
	assert.Contains(t, jobURL(res.Data[0]), "stripe.com/jobs")
}

func TestSearchJobsUnknownCompany(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{
		CompanySlug: []string{MockUnknownCompanySlug},
		Limit:       NewOptInt(1),
	})
	require.NoError(t, err)

	meta, ok := res.Meta.Get()
	require.True(t, ok)
	assert.Equal(t, 0, meta.Total.Or(-1))
	assert.Empty(t, res.Data)
	// A bad VALUE is silent: only a bad parameter NAME is reported.
	assert.Empty(t, meta.IgnoredParams)
}

// TestSearchJobsIgnoredParams pins the shape callers have to check. The
// singular "country" filters nothing, so the response is the whole
// catalogue with a 200 and a full page of rows.
func TestSearchJobsIgnoredParams(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchJobs(t.Context(), SearchJobsParams{
		Q:     NewOptString(MockIgnoredParamsQuery),
		Limit: NewOptInt(1),
	})
	require.NoError(t, err)

	meta, ok := res.Meta.Get()
	require.True(t, ok)
	require.Len(t, meta.IgnoredParams, 1)
	assert.Equal(t, "country", meta.IgnoredParams[0].Param)
	assert.Equal(t, "countries", meta.IgnoredParams[0].DidYouMean.Or(""))
	// The total is the unfiltered catalogue, which is the whole problem.
	assert.Equal(t, 1476309, meta.Total.Or(0))
}

func TestGetJobFacets(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobFacets(t.Context(), GetJobFacetsParams{
		Q: NewOptString("golang"),
	})
	require.NoError(t, err)
	assert.Equal(t, 2857, res.Data.Total)
	assert.Len(t, res.Data.Facets, 24)
	require.Contains(t, res.Data.Facets, "work_mode")
	assert.Equal(t, 467, res.Data.Facets["work_mode"]["remote"])
	require.Contains(t, res.Data.Facets, "skills")
	assert.NotEmpty(t, res.Data.Facets["skills"])
	// company_slug has no distribution; searchCompanies is its typeahead.
	assert.NotContains(t, res.Data.Facets, "company_slug")

	// The continuous facets come back as ranges instead of counts.
	stats, ok := res.Data.Stats.Get()
	require.True(t, ok)
	assert.Contains(t, stats, "salary_min")
	assert.Contains(t, stats, "experience_years_min")
}

func TestSearchCompanies(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchCompanies(t.Context(), SearchCompaniesParams{
		Q:     NewOptString(MockCompanyQuery),
		Limit: NewOptInt(3),
	})
	require.NoError(t, err)

	meta, ok := res.Meta.Get()
	require.True(t, ok)
	assert.Equal(t, 72, meta.Total.Or(0))
	require.Len(t, res.Data, 3)

	// Two catalogue companies share this name, which is why resolving a
	// name has to return a list.
	assert.Equal(t, "adria-solutions", res.Data[0].Slug)
	assert.Equal(t, "Adria Solutions", res.Data[0].Name)
	assert.Equal(t, 67, res.Data[0].JobCount.Or(0))
	assert.Equal(t, "adria-solutions-ltd", res.Data[1].Slug)
	assert.Equal(t, 51, res.Data[1].JobCount.Or(0))

	// v1.1.0 names every field CompanySummary serves, so nothing spills
	// into additionalProperties any more.
	assert.Empty(t, res.Data[0].AdditionalProps)
}

func TestSearchCompaniesNoMatch(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchCompanies(t.Context(), SearchCompaniesParams{
		Q: NewOptString(MockNoMatchCompanyQuery),
	})
	require.NoError(t, err)

	meta, ok := res.Meta.Get()
	require.True(t, ok)
	assert.Equal(t, 0, meta.Total.Or(-1))
	assert.Empty(t, res.Data)
}

// TestGetCompany covers the fields no other endpoint serves — the reason
// this operation is wired at all.
func TestGetCompany(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetCompany(t.Context(), GetCompanyParams{
		Slug:  MockCompanySlug,
		Limit: NewOptInt(1),
	})
	require.NoError(t, err)

	company, ok := res.Data.Company.Get()
	require.True(t, ok)
	assert.Equal(t, "stripe", company.Slug)
	assert.Equal(t, "Stripe", company.Name)
	assert.Equal(t, "enterprise", company.Maturity.Or(""))
	assert.Equal(t, 2012, company.YearFounded.Or(0))
	assert.Contains(t, company.Industries, "fintech")
	assert.Contains(t, company.RemoteRegions, "eu")
	// openapi.yaml documents W21 as the yc_batch form, but the values the
	// API holds are written out in full — which is why this endpoint is
	// the only way to learn the vocabulary.
	assert.Equal(t, []string{"Summer 2009"}, company.YcBatch)
	assert.Equal(t, []string{"Active"}, company.YcStatus)
	assert.Contains(t, company.YcFlags, "top_company")
	// Documented as always null on this unauthenticated schema.
	assert.Equal(t, 0, company.MyVote.Or(-1))

	require.Len(t, res.Data.Jobs, 1)
	assert.Equal(t, "stripe", res.Data.Jobs[0].CompanySlug.Or(""))
	assert.False(t, res.Data.ReferralAvailable.Or(true))
}

func TestSearchCities(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.SearchCities(t.Context(), SearchCitiesParams{
		Q:       NewOptString(MockCityQuery),
		Country: NewOptString(MockCityCountry),
	})
	require.NoError(t, err)

	require.NotEmpty(t, res.Data)
	assert.Equal(t, "London", res.Data[0].Value)
	assert.Equal(t, "gb", res.Data[0].Country)
}

func TestGetJob(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJob(t.Context(), GetJobParams{Slug: MockJobSlug})
	require.NoError(t, err)

	job := res.Data
	assert.Equal(t, MockJobSlug, job.PublicSlug)
	assert.Equal(t, "Program Manager, Intake & Portfolio Management", job.Title)
	assert.Equal(t, "Stripe", job.Company)
	assert.Equal(t, "stripe", job.CompanySlug.Or(""))
	assert.Contains(t, jobURL(job), "gh_jid=7569678")
	// Unlike the search preview, detail is the whole stored body.
	assert.Greater(t, len(job.Description.Or("")), 1000)
	assert.Contains(t, job.Description.Or(""), "<p>")
}

func TestGetJobNotFound(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	_, err = client.GetJob(t.Context(), GetJobParams{Slug: MockUnknownJobSlug})
	require.Error(t, err)
	var sc *ErrorStatusCode
	require.ErrorAs(t, err, &sc)
	assert.Equal(t, 404, sc.StatusCode)
	assert.Equal(t, "not found", sc.Response.Error)
}
