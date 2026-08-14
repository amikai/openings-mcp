package ats

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/adp_wfn"
)

func newADPWFNAdapter(t *testing.T) *ADPWFNAdapter {
	t.Helper()
	srv := adp_wfn.NewMockServer()
	t.Cleanup(srv.Close)

	a := NewADPWFNAdapter(http.DefaultClient)
	a.baseURL = adp_wfn.MockBaseURL(srv.URL)
	a.legacyURL = srv.URL
	return a
}

func TestADPWFNRosterBuildsRegistry(t *testing.T) {
	a := NewADPWFNAdapter(http.DefaultClient)
	require.Equal(t, "adp_wfn", a.Name())
	_, err := NewRegistry(a)
	require.NoError(t, err)
	require.NotEmpty(t, a.Roster())

	for _, info := range a.Roster() {
		u, ok := a.CareersURL(info.Slug)
		assert.True(t, ok, "every roster row must render a careers URL")
		assert.Contains(t, u, "recruitment.html")
	}
}

func TestADPWFNParseCareersURLCurrentForm(t *testing.T) {
	a := newADPWFNAdapter(t)
	tests := []struct {
		name string
		raw  string
		slug string
		ok   bool
	}{
		{
			name: "roster tenant resolves to its curated slug",
			raw:  "https://workforcenow.adp.com/mascsr/default/mdf/recruitment/recruitment.html?cid=" + adp_wfn.MockCID + "&ccId=19000101_000001",
			slug: adp_wfn.MockSlug,
			ok:   true,
		},
		{
			name: "a job permalink is still a careers URL",
			raw:  "https://workforcenow.adp.com/mascsr/default/mdf/recruitment/recruitment.html?cid=" + adp_wfn.MockCID + "&jobId=584587&lang=en_US",
			slug: adp_wfn.MockSlug,
			ok:   true,
		},
		{
			name: "no cid is not addressable",
			raw:  "https://workforcenow.adp.com/mascsr/default/mdf/recruitment/recruitment.html?ccId=19000101_000001",
			ok:   false,
		},
		{
			name: "the MyJobs surface belongs to another adapter",
			raw:  "https://myjobs.adp.com/guitarcenterexternal/cx",
			ok:   false,
		},
		{
			name: "unrelated host",
			raw:  "https://example.com/mascsr/default/mdf/recruitment/recruitment.html?cid=" + adp_wfn.MockCID,
			ok:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.raw)
			require.NoError(t, err)
			slug, ok := a.ParseCareersURL(u)
			assert.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, tc.slug, slug)
			}
		})
	}
}

func TestADPWFNParseCareersURLMintsCanonicalURLForUnlistedTenant(t *testing.T) {
	a := newADPWFNAdapter(t)
	u, err := url.Parse("https://workforcenow.adp.com/mascsr/default/mdf/recruitment/recruitment.html?cid=" +
		adp_wfn.MockUnlistedCID + "&ccId=19000101_000009&lang=" + adp_wfn.MockEnCALocale)
	require.NoError(t, err)

	slug, ok := a.ParseCareersURL(u)
	require.True(t, ok)

	// A bare cid would strand the locale here, and this board is empty under
	// the locale the upstream would otherwise default to — so the minted slug
	// has to carry it.
	assert.Contains(t, slug, adp_wfn.MockUnlistedCID)
	assert.Contains(t, slug, adp_wfn.MockEnCALocale)

	res, err := a.Search(context.Background(), slug, SearchParams{})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jobs, "the minted slug must reach the same board the URL pointed at")
}

func TestADPWFNParseCareersURLLegacyForm(t *testing.T) {
	a := newADPWFNAdapter(t)

	u, err := url.Parse("https://workforcenow.adp.com/jobs/apply/posting.html?client=" + adp_wfn.MockLegacySlug)
	require.NoError(t, err)
	slug, ok := a.ParseCareersURL(u)
	require.True(t, ok, "the retired form resolves to a cid once the cookie handshake completes")
	assert.Equal(t, adp_wfn.MockSlug, slug)

	// An unresolvable slug is reported as an unrecognized URL, so the caller
	// is steered to the current form rather than handed a dead end.
	u, err = url.Parse("https://workforcenow.adp.com/jobs/apply/posting.html?client=" + adp_wfn.MockUnresolvableLegacySlug)
	require.NoError(t, err)
	_, ok = a.ParseCareersURL(u)
	assert.False(t, ok)
}

func TestADPWFNSearchPagesAndTrustsUnfilteredTotal(t *testing.T) {
	a := newADPWFNAdapter(t)
	ctx := context.Background()

	first, err := a.Search(ctx, adp_wfn.MockSlug, SearchParams{})
	require.NoError(t, err)
	require.Len(t, first.Jobs, pageSize)
	assert.Equal(t, 48, first.TotalCount)
	assert.Equal(t, 1, first.Page)
	assert.Equal(t, totalPages(48), first.TotalPages)

	job := first.Jobs[0]
	assert.NotEmpty(t, job.JobID)
	assert.NotEmpty(t, job.Title)
	assert.NotEmpty(t, job.PostedAt)
	assert.Contains(t, job.URL, "jobId="+job.JobID, "the summary URL must use the id detail accepts")

	last, err := a.Search(ctx, adp_wfn.MockSlug, SearchParams{Page: 3})
	require.NoError(t, err)
	assert.Len(t, last.Jobs, 8)
	assert.Equal(t, 3, last.Page)
}

func TestADPWFNSearchReportsLowerBoundWhenFiltered(t *testing.T) {
	a := newADPWFNAdapter(t)

	res, err := a.Search(context.Background(), adp_wfn.MockSlug, SearchParams{
		Filters: FilterSet{adpWFNLocationKey: []string{"Clarinda, IA"}},
	})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 13)

	// The upstream claims 26 for these 13 rows. Passing that through would
	// promise a second page that does not exist.
	assert.Equal(t, 13, res.TotalCount)
	assert.Equal(t, 1, res.TotalPages)
}

func TestADPWFNSearchKeywordServesOnePageOnly(t *testing.T) {
	a := newADPWFNAdapter(t)
	ctx := context.Background()

	first, err := a.Search(ctx, adp_wfn.MockSlug, SearchParams{Query: "welder"})
	require.NoError(t, err)
	assert.NotEmpty(t, first.Jobs)
	assert.Equal(t, len(first.Jobs), first.TotalCount)
	assert.Equal(t, 1, first.TotalPages, "relevance ordering is unstable between calls, so there is no sound second page")

	// Asking for one anyway returns nothing rather than a page of duplicates.
	second, err := a.Search(ctx, adp_wfn.MockSlug, SearchParams{Query: "welder", Page: 2})
	require.NoError(t, err)
	assert.Empty(t, second.Jobs)
	assert.Equal(t, 1, second.TotalPages)
}

func TestADPWFNSearchRejectsUnvalidatableFilters(t *testing.T) {
	a := newADPWFNAdapter(t)
	ctx := context.Background()

	_, err := a.Search(ctx, adp_wfn.MockSlug, SearchParams{Filters: FilterSet{"department": []string{"x"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown filter key")

	// A value that is not published must never reach the wire: upstream would
	// answer it with the whole board rather than an error.
	_, err = a.Search(ctx, adp_wfn.MockSlug, SearchParams{Filters: FilterSet{adpWFNLocationKey: []string{"Atlantis"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestADPWFNSearchAcceptsSeveralValuesForOneDimension(t *testing.T) {
	a := newADPWFNAdapter(t)
	_, err := a.Search(context.Background(), adp_wfn.MockSlug, SearchParams{
		Filters: FilterSet{adpWFNLocationKey: []string{"Clarinda, IA", "Markle, IN"}},
	})
	// Values within a dimension OR upstream, so a second one is legitimate
	// rather than something to reject.
	require.NoError(t, err)
}

func TestADPWFNLocationBuildsAPairRatherThanForwardingRawText(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Clarinda, IA", "Clarinda, IA"},         // published verbatim
		{"IN", "IN,LOCATION_STATE"},              // a bare state code
		{"Kitchener", "Kitchener,LOCATION_CITY"}, // an unpublished city still gets a qualifier
		{"Markle, IN", "Markle, IN"},             // already reads as a pair
		// A comma is not enough: upstream reads a half-empty pair as an
		// unpaired token and answers with the whole board.
		{"Clarinda,", "Clarinda,LOCATION_CITY"},
		{"Clarinda, ", "Clarinda,LOCATION_CITY"},
		{",IA", "IA,LOCATION_CITY"},
	}
	published := []adp_wfn.FilterValue{{Label: "Clarinda - IA", Wire: "Clarinda, IA"}}
	for _, tc := range tests {
		got, ok := adpWFNLocationPair(published, tc.in)
		require.True(t, ok, tc.in)
		assert.Equal(t, tc.want, got)
		assert.Contains(t, got, ",", "a value without a qualifier would silently return the whole board")
	}

	_, ok := adpWFNLocationPair(published, "   ")
	assert.False(t, ok)
}

func TestADPWFNSearchWithFreeTextLocation(t *testing.T) {
	a := newADPWFNAdapter(t)
	res, err := a.Search(context.Background(), adp_wfn.MockSlug, SearchParams{Location: "Clarinda, IA"})
	require.NoError(t, err)
	assert.Len(t, res.Jobs, 13)
}

func TestADPWFNFiltersPublishesBothDimensions(t *testing.T) {
	a := newADPWFNAdapter(t)
	fs, err := a.Filters(context.Background(), adp_wfn.MockSlug)
	require.NoError(t, err)
	require.Contains(t, fs, adpWFNLocationKey)
	assert.NotEmpty(t, fs[adpWFNLocationKey])
	assert.Contains(t, fs[adpWFNLocationKey], "Indiana (Statewide)")
}

func TestADPWFNFiltersOmitsDimensionTheTenantDoesNotPublish(t *testing.T) {
	a := newADPWFNAdapter(t)
	slug := adp_wfn.BoardURL(adp_wfn.MockEnCACID, "", adp_wfn.MockEnCALocale)

	fs, err := a.Filters(context.Background(), slug)
	require.NoError(t, err)
	assert.NotContains(t, fs, adpWFNLocationKey, "this tenant publishes no location list")
	assert.Contains(t, fs, adpWFNJobTypeKey)

	// Publishing no location list does not disable location search, so the
	// adapter must still accept one.
	res, err := a.Search(context.Background(), slug, SearchParams{Location: "Kitchener"})
	require.NoError(t, err)
	assert.NotNil(t, res)
}

func TestADPWFNDetail(t *testing.T) {
	a := newADPWFNAdapter(t)
	slug := adp_wfn.CompaniesByCID[adp_wfn.MockSmallCID].Slug
	require.NotEmpty(t, slug)

	d, err := a.Detail(context.Background(), slug, adp_wfn.MockJobID)
	require.NoError(t, err)

	assert.Equal(t, adp_wfn.MockJobID, d.JobID)
	assert.NotEmpty(t, d.Title)
	assert.Equal(t, "MetLife Legal Plans", d.Company, "the roster name wins over ADP's own record")
	assert.NotEmpty(t, d.Location)
	assert.NotEmpty(t, d.PostedAt)
	assert.Contains(t, d.URL, "jobId="+adp_wfn.MockJobID)
	assert.NotEmpty(t, d.Description)
	assert.NotContains(t, d.Description, "<p>", "the description is converted to plain text")
	// The unified detail has no salary field, so a published range rides in
	// the description rather than being dropped.
	assert.True(t, strings.HasPrefix(d.Description, "Pay range:"))
}

func TestADPWFNDetailUnlistedTenantUsesADPsOwnClientName(t *testing.T) {
	a := newADPWFNAdapter(t)
	slug := adp_wfn.BoardURL(adp_wfn.MockSmallCID, "", "en_US")

	d, err := a.Detail(context.Background(), slug, adp_wfn.MockJobID)
	require.NoError(t, err)
	// Verbatim, casing and all: the job endpoints carry no company name, and
	// reconstructing a tidier one would be inventing it.
	assert.Equal(t, "NOVAE LLC", d.Company)
}

func TestADPWFNDetailUnknownJob(t *testing.T) {
	a := newADPWFNAdapter(t)
	_, err := a.Detail(context.Background(), adp_wfn.MockSlug, adp_wfn.MockUnknownJobID)
	require.Error(t, err)
	assert.ErrorIs(t, err, adp_wfn.ErrJobNotFound)
}

func TestADPWFNUnknownCompany(t *testing.T) {
	a := newADPWFNAdapter(t)
	_, err := a.Search(context.Background(), "not-a-company", SearchParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown company")
}
