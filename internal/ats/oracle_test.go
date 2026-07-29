package ats

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/oracle"
)

const oracleMayoKey = "fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com/CX_1"

func testOracleAdapter(t *testing.T) (*OracleAdapter, *[]string) {
	t.Helper()
	mock := oracle.NewMockServer()
	t.Cleanup(mock.Close)

	var finders []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		finders = append(finders, r.URL.Query().Get("finder"))
		req, err := http.NewRequestWithContext(
			r.Context(),
			r.Method,
			mock.URL+r.URL.RequestURI(),
			nil,
		)
		if !assert.NoError(t, err) {
			return
		}
		req.Header = r.Header.Clone()
		rsp, err := mock.Client().Do(req)
		if !assert.NoError(t, err) {
			return
		}
		defer rsp.Body.Close()
		w.Header().Set("Content-Type", rsp.Header.Get("Content-Type"))
		w.WriteHeader(rsp.StatusCode)
		_, _ = io.Copy(w, rsp.Body)
	}))
	t.Cleanup(proxy.Close)

	a := NewOracleAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(oracle.Company) string { return proxy.URL }
	return a, &finders
}

func oracleMayoSlug(t *testing.T) string {
	t.Helper()
	company, ok := oracle.CompaniesByKey[oracleMayoKey]
	require.True(t, ok)
	return oracleCompanySlug(company)
}

func TestOracleRosterBuildsRegistry(t *testing.T) {
	a := NewOracleAdapter(http.DefaultClient)
	require.Len(t, a.Roster(), len(oracle.Companies))
	_, err := NewRegistry(a)
	require.NoError(t, err)
}

func TestOracleRosterReturnsIndependentSlice(t *testing.T) {
	a := NewOracleAdapter(http.DefaultClient)
	roster := a.Roster()
	require.NotEmpty(t, roster)
	original := roster[0]
	roster[0] = CompanyInfo{Slug: "mutated", Name: "Mutated"}
	assert.Equal(t, original, a.Roster()[0])
}

func TestOracleParseCareersURL(t *testing.T) {
	a := NewOracleAdapter(http.DefaultClient)
	tests := []struct {
		name string
		raw  string
		slug string
		ok   bool
	}{
		{
			name: "curated posting",
			raw: "https://fa-euwp-saasfaprod1.fa.ocs.oraclecloud.com/" +
				"hcmUI/CandidateExperience/en/sites/Mayo-US/job/386920",
			slug: oracleMayoSlug(t),
			ok:   true,
		},
		{
			name: "uncurated site",
			raw: "https://fa-example.fa.us2.oraclecloud.com/" +
				"hcmUI/CandidateExperience/en-US/sites/Acme/jobs",
			slug: "https://fa-example.fa.us2.oraclecloud.com/" +
				"hcmUI/CandidateExperience/en-US/sites/Acme/jobs",
			ok: true,
		},
		{
			name: "uncurated detail canonicalizes",
			raw: "http://fa-example.fa.us2.oraclecloud.com/" +
				"hcmUI/CandidateExperience/en/sites/Acme/job/123?source=test",
			slug: "https://fa-example.fa.us2.oraclecloud.com/" +
				"hcmUI/CandidateExperience/en/sites/Acme/jobs",
			ok: true,
		},
		{
			name: "spoofed suffix",
			raw: "https://fa-example.oraclecloud.com.example.com/" +
				"hcmUI/CandidateExperience/en/sites/Acme/jobs",
		},
		{
			name: "custom port",
			raw: "https://fa-example.fa.us2.oraclecloud.com:8443/" +
				"hcmUI/CandidateExperience/en/sites/Acme/jobs",
		},
		{
			name: "other oracle page",
			raw:  "https://fa-example.fa.us2.oraclecloud.com/fscmUI/faces/FuseWelcome",
		},
		{
			name: "non fusion oracle cloud host",
			raw: "https://objectstorage.us-phoenix-1.oraclecloud.com/" +
				"hcmUI/CandidateExperience/en/sites/Acme/jobs",
		},
		{name: "other ATS", raw: "https://jobs.lever.co/acme"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug, ok := a.ParseCareersURL(mustParseURL(t, tt.raw))
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.slug, slug)
		})
	}
}

func TestOracleSearch(t *testing.T) {
	a, finders := testOracleAdapter(t)
	res, err := a.Search(t.Context(), oracleMayoSlug(t), SearchParams{
		Query: " analyst ",
		Page:  2,
	})
	require.NoError(t, err)
	assert.Equal(t, 33, res.TotalCount)
	assert.Equal(t, 2, res.Page)
	assert.Equal(t, 2, res.TotalPages)
	require.Len(t, res.Jobs, 3)

	first := res.Jobs[0]
	assert.Equal(t, "385004", first.JobID)
	assert.Equal(t, "Quality Analyst - FDA Regulated Manufacturing", first.Title)
	assert.Equal(t, "Rochester, MN, United States", first.Location)
	assert.Equal(t, "2026-06-23", first.PostedAt)
	assert.Contains(t, first.URL, "/sites/Mayo-US/job/385004")

	require.Len(t, *finders, 1)
	assert.Contains(t, (*finders)[0], "limit=20,offset=20")
	assert.Contains(t, (*finders)[0], `keyword="analyst"`)
}

func TestOracleSearchRejectsHugePage(t *testing.T) {
	a, finders := testOracleAdapter(t)
	_, err := a.Search(t.Context(), oracleMayoSlug(t), SearchParams{Page: math.MaxInt})
	require.ErrorContains(t, err, "smaller page")
	assert.Empty(t, *finders)
}

func TestOracleFilters(t *testing.T) {
	a, _ := testOracleAdapter(t)
	filters, err := a.Filters(t.Context(), oracleMayoSlug(t))
	require.NoError(t, err)
	assert.Contains(t, filters["location"], "Rochester, MN, United States")
	assert.Contains(t, filters["category"], "Direct Patient Care")
	assert.Equal(t, []string{"On-site"}, filters["workplace-type"])
	assert.Contains(t, filters["posting-date"], "Less than 7 days")
}

func TestOracleSearchResolvesLocationAndFilterLabels(t *testing.T) {
	a, finders := testOracleAdapter(t)
	_, err := a.Search(t.Context(), oracleMayoSlug(t), SearchParams{
		Location: "Rochester",
		Filters: FilterSet{
			"workplace-type": {"on-SITE"},
		},
	})
	require.NoError(t, err)
	require.Len(t, *finders, 2, "filtered search should probe facets then search")
	assert.Contains(t, (*finders)[1], "selectedLocationsFacet=300000006426003")
	assert.Contains(t, (*finders)[1], "selectedWorkplaceTypesFacet=ORA_ON_SITE")
}

func TestOracleSearchFilterTeachingErrors(t *testing.T) {
	a, _ := testOracleAdapter(t)

	_, err := a.Search(t.Context(), oracleMayoSlug(t), SearchParams{
		Filters: FilterSet{"bogus": {"x"}},
	})
	require.ErrorContains(t, err, "unknown filter key")
	assert.Contains(t, err.Error(), "workplace-type")

	_, err = a.Search(t.Context(), oracleMayoSlug(t), SearchParams{
		Filters: FilterSet{"category": {"Not a category"}},
	})
	require.ErrorContains(t, err, `filter value "Not a category" not found`)
	assert.Contains(t, err.Error(), "Direct Patient Care")
}

func TestOracleSearchRejectsLocationFacetConflict(t *testing.T) {
	a, _ := testOracleAdapter(t)
	_, err := a.Search(t.Context(), oracleMayoSlug(t), SearchParams{
		Location: "Rochester",
		Filters:  FilterSet{"location": {"United States"}},
	})
	require.ErrorContains(t, err, "also set in filters")
}

func TestOracleDetail(t *testing.T) {
	a, _ := testOracleAdapter(t)
	detail, err := a.Detail(t.Context(), oracleMayoSlug(t), "386920")
	require.NoError(t, err)
	assert.Equal(t, "386920", detail.JobID)
	assert.Equal(t, "Senior Analyst - ATS", detail.Title)
	assert.Equal(t, "Mayo Clinic", detail.Company)
	assert.Equal(t, "Phoenix, AZ, United States", detail.Location)
	assert.Equal(t, "2026-07-14", detail.PostedAt)
	assert.Contains(t, detail.URL, "/sites/Mayo-US/job/386920")
	assert.Contains(t, detail.Description, "Access Technologies and Systems")
	assert.Contains(t, detail.Description, "Qualifications")
	assert.NotContains(t, detail.Description, "<p>")
}

func TestOracleDetailNotFound(t *testing.T) {
	a, _ := testOracleAdapter(t)
	_, err := a.Detail(t.Context(), oracleMayoSlug(t), "999999999999")
	require.ErrorContains(t, err, "pass a job_id exactly as returned")
}

func TestOracleSearchDiscoversNonRosterURL(t *testing.T) {
	a, _ := testOracleAdapter(t)
	called := false
	a.discoverSite = func(
		_ context.Context,
		rawURL string,
		_ *http.Client,
	) (oracle.Site, error) {
		called = true
		assert.Equal(
			t,
			"https://fa-example.fa.us2.oraclecloud.com/"+
				"hcmUI/CandidateExperience/en/sites/Acme/jobs",
			rawURL,
		)
		return oracle.Site{
			CareersURL: rawURL,
			APIBaseURL: a.baseURL(oracle.Company{}),
			Site:       "Acme",
			SiteNumber: "CX_1",
			Language:   "en",
		}, nil
	}

	res, err := a.Search(
		t.Context(),
		"https://fa-example.fa.us2.oraclecloud.com/"+
			"hcmUI/CandidateExperience/en/sites/Acme/jobs",
		SearchParams{},
	)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 1330, res.TotalCount)
}

func TestOracleUnknownSlugTeaches(t *testing.T) {
	a := NewOracleAdapter(http.DefaultClient)
	_, err := a.Search(t.Context(), "not-a-company", SearchParams{})
	require.ErrorContains(t, err, "careers URL")
}

func TestOracleDescriptionSkipsEmptySections(t *testing.T) {
	description := oracleDescription(&oracle.Job{
		DescriptionHTML:      "<p>Main description.</p>",
		QualificationsHTML:   "<p>Must know Go.</p>",
		ResponsibilitiesHTML: " ",
	})
	assert.Contains(t, description, "Main description.")
	assert.Contains(t, description, "Qualifications")
	assert.Contains(t, description, "Must know Go.")
	assert.NotContains(t, description, "Responsibilities")
	assert.False(t, strings.Contains(description, "<p>"))
}
