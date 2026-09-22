package adp_wfn_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/adp_wfn"
)

func newTestClient(t *testing.T) *adp_wfn.BoardClient {
	t.Helper()
	srv := adp_wfn.NewMockServer()
	t.Cleanup(srv.Close)
	c, err := adp_wfn.NewBoardClient(adp_wfn.Config{
		BaseURL:       adp_wfn.MockBaseURL(srv.URL),
		LegacyBaseURL: srv.URL,
	})
	require.NoError(t, err)
	return c
}

func TestListReturnsBoardAndTrustsUnfilteredTotal(t *testing.T) {
	c := newTestClient(t)

	res, err := c.List(context.Background(), adp_wfn.MockSmallCID, adp_wfn.ListParams{Locale: "en_US"})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 2)
	assert.Equal(t, 2, res.TotalNumber)
	assert.True(t, res.TotalTrusted, "an unfiltered request is the only case where the upstream count is a row count")
	assert.False(t, res.HasMore)

	job := res.Jobs[0]
	assert.NotEmpty(t, job.ItemID.Value)
	assert.NotEmpty(t, job.RequisitionTitle.Value)
	assert.NotEmpty(t, adp_wfn.ExternalJobID(job), "the external id builds the public URL, so every row must carry one")
}

func TestListPagesFromOneNotZero(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	first, err := c.List(ctx, adp_wfn.MockCID, adp_wfn.ListParams{Locale: "en_US", Page: 1})
	require.NoError(t, err)
	require.Len(t, first.Jobs, adp_wfn.PageSize)
	assert.Equal(t, 48, first.TotalNumber)
	assert.True(t, first.HasMore)

	// Page 3 of a 48-row board is the tail. Reaching it at all proves the
	// 1-based offset arithmetic, since asking for row 0 upstream silently
	// drops a row.
	last, err := c.List(ctx, adp_wfn.MockCID, adp_wfn.ListParams{Locale: "en_US", Page: 3})
	require.NoError(t, err)
	assert.Len(t, last.Jobs, 8)
	assert.False(t, last.HasMore)

	firstIDs := map[string]bool{}
	for _, j := range first.Jobs {
		firstIDs[j.ItemID.Value] = true
	}
	for _, j := range last.Jobs {
		assert.False(t, firstIDs[j.ItemID.Value], "pages must not overlap")
	}
}

func TestListPageZeroIsTreatedAsPageOne(t *testing.T) {
	c := newTestClient(t)
	res, err := c.List(context.Background(), adp_wfn.MockSmallCID, adp_wfn.ListParams{Locale: "en_US", Page: 0})
	require.NoError(t, err)
	assert.Len(t, res.Jobs, 2)
}

func TestListDistrustsTotalWhenFiltered(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	byLocation, err := c.List(ctx, adp_wfn.MockCID, adp_wfn.ListParams{
		Locale:    "en_US",
		Locations: []string{adp_wfn.MockLocationValue},
	})
	require.NoError(t, err)
	assert.Len(t, byLocation.Jobs, 13)
	assert.False(t, byLocation.TotalTrusted)
	// The upstream reports twice the rows it returned. A caller that believed
	// this would page into emptiness.
	assert.Equal(t, 26, byLocation.TotalNumber)

	byQuery, err := c.List(ctx, adp_wfn.MockCID, adp_wfn.ListParams{Locale: "en_US", Query: "welder"})
	require.NoError(t, err)
	assert.NotEmpty(t, byQuery.Jobs)
	assert.False(t, byQuery.TotalTrusted)
	assert.Greater(t, byQuery.TotalNumber, len(byQuery.Jobs))
}

func TestListWellFormedLocationThatMatchesNothingReturnsNoRows(t *testing.T) {
	c := newTestClient(t)
	res, err := c.List(context.Background(), adp_wfn.MockCID, adp_wfn.ListParams{
		Locale:    "en_US",
		Locations: []string{"Atlantis,LOCATION_CITY"},
	})
	require.NoError(t, err)
	assert.Empty(t, res.Jobs, "a well-formed pair naming nothing is the safe failure: zero rows")
}

func TestListUnpairedLocationSilentlyReturnsWholeBoard(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	unfiltered, err := c.List(ctx, adp_wfn.MockCID, adp_wfn.ListParams{Locale: "en_US"})
	require.NoError(t, err)

	// Documents why callers must validate: a bare token is not rejected, it
	// is ignored, and the response is indistinguishable from no filter at all
	// apart from an inflated count.
	unpaired, err := c.List(ctx, adp_wfn.MockCID, adp_wfn.ListParams{
		Locale:    "en_US",
		Locations: []string{adp_wfn.MockUnpairedLocation},
	})
	require.NoError(t, err)
	assert.Len(t, unpaired.Jobs, len(unfiltered.Jobs))
	assert.Equal(t, unfiltered.Jobs[0].ItemID.Value, unpaired.Jobs[0].ItemID.Value)
}

func TestListBogusJobTypeReturnsMisleadingSubset(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	valid, err := c.List(ctx, adp_wfn.MockJobTypeCID, adp_wfn.ListParams{
		Locale:           "en_US",
		WorkerCategories: []string{adp_wfn.MockJobTypeOID},
	})
	require.NoError(t, err)
	assert.Len(t, valid.Jobs, 16)

	// The dangerous case. An unpublished oid yields neither an error, nor an
	// empty set, nor the whole board, but 19 of 21 rows — a result that looks
	// like a filter that worked.
	bogus, err := c.List(ctx, adp_wfn.MockJobTypeCID, adp_wfn.ListParams{
		Locale:           "en_US",
		WorkerCategories: []string{adp_wfn.MockBogusJobTypeOID},
	})
	require.NoError(t, err)
	assert.Len(t, bogus.Jobs, 19)
	assert.NotEmpty(t, bogus.Jobs)
}

func TestListWrongLocaleReturnsEmptyBoardNotAnError(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	right, err := c.List(ctx, adp_wfn.MockEnCACID, adp_wfn.ListParams{Locale: adp_wfn.MockEnCALocale})
	require.NoError(t, err)
	assert.Len(t, right.Jobs, 15)

	wrong, err := c.List(ctx, adp_wfn.MockEnCACID, adp_wfn.ListParams{Locale: "en_US"})
	require.NoError(t, err)
	assert.Empty(t, wrong.Jobs, "a locale this tenant publishes nothing under is answered with an empty board, not an error")
}

func TestListUnknownTenant(t *testing.T) {
	c := newTestClient(t)
	_, err := c.List(context.Background(), adp_wfn.MockUnknownCID, adp_wfn.ListParams{Locale: "en_US"})
	require.Error(t, err)
	assert.ErrorIs(t, err, adp_wfn.ErrTenantNotFound)
}

func TestJobReturnsDescriptionAndSalary(t *testing.T) {
	c := newTestClient(t)
	job, err := c.Job(context.Background(), adp_wfn.MockSmallCID, adp_wfn.MockJobID, "en_US")
	require.NoError(t, err)

	assert.NotEmpty(t, job.ItemID.Value)
	assert.NotEmpty(t, job.RequisitionTitle.Value)
	assert.NotEmpty(t, job.RequisitionDescription.Value, "the description is the only field detail adds over a list row")
	assert.NotEmpty(t, adp_wfn.PrimaryLocation(*job))

	_, ok := adp_wfn.PostedTime(*job)
	assert.True(t, ok)
	assert.Contains(t, adp_wfn.SalaryLine(*job), "USD")
}

func TestJobUnknownIDIsReportedDespiteHTTP200(t *testing.T) {
	c := newTestClient(t)
	_, err := c.Job(context.Background(), adp_wfn.MockSmallCID, adp_wfn.MockUnknownJobID, "en_US")
	require.Error(t, err)
	assert.ErrorIs(t, err, adp_wfn.ErrJobNotFound)
}

func TestSearchFiltersSplitsTheTwoDimensionsByWireColumn(t *testing.T) {
	c := newTestClient(t)
	cat, err := c.SearchFilters(context.Background(), adp_wfn.MockCID, "en_US")
	require.NoError(t, err)

	require.NotEmpty(t, cat.Locations)
	// Locations put the already-pair-formed value on the wire and show the
	// oid; job types do the opposite. Getting this backwards returns the
	// whole board, so it is asserted rather than assumed.
	var statewide adp_wfn.FilterValue
	for _, l := range cat.Locations {
		if l.Wire == "IN,LOCATION_STATE" {
			statewide = l
		}
		assert.Contains(t, l.Wire, ",", "every published location is already a value,qualifier pair")
	}
	require.NotEmpty(t, statewide.Wire)
	assert.Equal(t, "Indiana (Statewide)", statewide.Label)
}

func TestSearchFiltersTenantWithoutLocationDimension(t *testing.T) {
	c := newTestClient(t)
	cat, err := c.SearchFilters(context.Background(), adp_wfn.MockEnCACID, adp_wfn.MockEnCALocale)
	require.NoError(t, err)

	// No published location list does not mean location filtering is
	// unavailable on this tenant; it means there is nothing to validate a
	// caller's input against.
	assert.Empty(t, cat.Locations)
	require.NotEmpty(t, cat.WorkerCategories)
	for _, w := range cat.WorkerCategories {
		assert.NotEmpty(t, w.Wire)
		assert.NotEmpty(t, w.Label)
		assert.NotEqual(t, w.Wire, w.Label, "job types send the oid and show the label")
	}
}

func TestTenantInfoReadsTheOnlyPublicCompanyName(t *testing.T) {
	c := newTestClient(t)
	info, err := c.TenantInfo(context.Background(), adp_wfn.MockCID)
	require.NoError(t, err)
	// Passed through verbatim, casing and all.
	assert.Equal(t, "NOVAE LLC", info.ClientName)
	assert.Equal(t, "novae", info.ClientID)
}

func TestPrimaryLocaleTakesTheFirstAdvertisedLocale(t *testing.T) {
	c := newTestClient(t)
	locale, err := c.PrimaryLocale(context.Background(), adp_wfn.MockEnCACID)
	require.NoError(t, err)
	assert.Equal(t, adp_wfn.MockEnCALocale, locale, "the first advertised locale is the job-bearing one")
}

func TestResolveLegacySlug(t *testing.T) {
	c := newTestClient(t)

	cid, ok := c.ResolveLegacySlug(adp_wfn.MockLegacySlug)
	require.True(t, ok, "the retired career center resolves a client slug once a cookie jar completes the handshake")
	assert.Equal(t, adp_wfn.MockCID, cid)

	// Not every slug resolves; that is ordinary, not an error condition.
	_, ok = c.ResolveLegacySlug(adp_wfn.MockUnresolvableLegacySlug)
	assert.False(t, ok)

	_, ok = c.ResolveLegacySlug("   ")
	assert.False(t, ok)
}

func TestJobURLUsesExternalJobID(t *testing.T) {
	u := adp_wfn.JobURL("cid-1", "cc-1", "584587", "en_CA")
	assert.Contains(t, u, "recruitment.html?")
	assert.Contains(t, u, "cid=cid-1")
	assert.Contains(t, u, "jobId=584587")
	assert.Contains(t, u, "lang=en_CA")

	board := adp_wfn.BoardURL("cid-1", "cc-1", "en_CA")
	assert.NotContains(t, board, "jobId")
}

func TestCompaniesRosterIsWellFormed(t *testing.T) {
	require.NotEmpty(t, adp_wfn.Companies)
	seenSlug := map[string]bool{}
	seenCID := map[string]bool{}
	for _, c := range adp_wfn.Companies {
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.Slug)
		assert.NotEmpty(t, c.CID)
		assert.NotEmpty(t, c.Locale, "locale can never be defaulted: the wrong one returns an empty board")
		assert.False(t, seenSlug[c.Slug], "duplicate slug %q", c.Slug)
		assert.False(t, seenCID[c.CID], "duplicate cid %q", c.CID)
		seenSlug[c.Slug] = true
		seenCID[c.CID] = true
		assert.Contains(t, c.CareersURL(), c.CID)
	}
	novae, ok := adp_wfn.CompaniesBySlug[adp_wfn.MockSlug]
	require.True(t, ok)
	assert.Equal(t, adp_wfn.MockCID, novae.CID)
	byCID, ok := adp_wfn.CompaniesByCID[adp_wfn.MockCID]
	require.True(t, ok)
	assert.Equal(t, adp_wfn.MockSlug, byCID.Slug)
}

func TestErrorsAreDistinguishable(t *testing.T) {
	assert.False(t, errors.Is(adp_wfn.ErrJobNotFound, adp_wfn.ErrTenantNotFound))
}
