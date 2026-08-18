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

func TestAgentSearchJobs(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.AgentSearchJobs(t.Context(), AgentSearchJobsParams{
		Q:                 NewOptString("golang"),
		Limit:             NewOptInt(2),
		DescriptionFormat: NewOptAgentSearchJobsDescriptionFormat(AgentSearchJobsDescriptionFormatText),
	})
	require.NoError(t, err)

	meta, ok := res.Meta.Get()
	require.True(t, ok)
	assert.Equal(t, 2, meta.Limit.Or(0))
	assert.Equal(t, 0, meta.Offset.Or(-1))
	assert.Equal(t, 2867, meta.Total.Or(0))
	require.Len(t, res.Data, 2)

	first := res.Data[0]
	assert.Equal(t, "golang-developer-backend-developer-boardroom-appointments-bw2y2vte", first.PublicSlug)
	assert.Equal(t, "Golang Developer (Backend Developer)", first.Title)
	assert.Equal(t, "Boardroom Appointments", first.Company)
	assert.Equal(t, "boardroom-appointments", first.CompanySlug.Or(""))
	assert.Equal(t, "Pretoria, GP, South Africa", first.Location.Or(""))
	assert.Equal(t, "manatal", first.Source.Or(""))
	assert.False(t, first.WorkMode.Set, "most list rows omit work_mode")
	assert.Contains(t, first.Skills, "go")
	assert.Equal(t, []string{"africa"}, first.Regions)
	assert.Equal(t, []string{"za"}, first.Countries)
	assert.True(t, first.PostedAt.Set)
	assert.True(t, first.PostedAt.Value.Equal(time.Date(2026, time.August, 17, 23, 27, 53, 0, time.UTC)))
	assert.Contains(t, jobURL(first), "careers-page.com/boardroom-appointments")

	// description_format=text: the search endpoint ships each row's
	// whole posting, already stripped of the stored markup.
	assert.Greater(t, len(first.Description.Or("")), 1000)
	assert.NotContains(t, first.Description.Or(""), "<p>")

	second := res.Data[1]
	assert.Equal(t, "golang-developer-caliberly-2ulpx6dm", second.PublicSlug)
	assert.Equal(t, "Caliberly", second.Company)
	assert.Equal(t, "caliberly", second.CompanySlug.Or(""))
}

func TestAgentSearchJobsFiltered(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.AgentSearchJobs(t.Context(), AgentSearchJobsParams{
		CompanySlug: []string{"stripe"},
		Limit:       NewOptInt(2),
	})
	require.NoError(t, err)

	meta, ok := res.Meta.Get()
	require.True(t, ok)
	assert.Equal(t, 520, meta.Total.Or(0))
	require.Len(t, res.Data, 2)
	assert.Equal(t, MockJobSlug, res.Data[0].PublicSlug)
	assert.Equal(t, "Stripe", res.Data[0].Company)
	assert.Equal(t, "stripe", res.Data[0].CompanySlug.Or(""))
	assert.Equal(t, "greenhouse", res.Data[0].Source.Or(""))
	assert.Equal(t, "remote", res.Data[0].WorkMode.Or(""))
	assert.Contains(t, jobURL(res.Data[0]), "stripe.com/jobs")
}

func TestAgentSearchJobsUnknownCompany(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.AgentSearchJobs(t.Context(), AgentSearchJobsParams{
		CompanySlug: []string{MockUnknownCompanySlug},
		Limit:       NewOptInt(1),
	})
	require.NoError(t, err)

	meta, ok := res.Meta.Get()
	require.True(t, ok)
	assert.Equal(t, 0, meta.Total.Or(-1))
	assert.Empty(t, res.Data)
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
	assert.Equal(t, 2864, res.Data.Total)
	require.Contains(t, res.Data.Facets, "work_mode")
	assert.Equal(t, 467, res.Data.Facets["work_mode"]["remote"])
	require.Contains(t, res.Data.Facets, "skills")
	assert.NotEmpty(t, res.Data.Facets["skills"])
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

	// CompanySummary declares three fields and additionalProperties, so
	// the rest lands here rather than being dropped.
	assert.Contains(t, res.Data[2].AdditionalProps, "tagline")
	assert.Contains(t, res.Data[2].AdditionalProps, "industries")
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
	// /jobs/{slug} takes no description_format, so detail is HTML.
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
