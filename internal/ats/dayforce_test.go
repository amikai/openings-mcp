package ats

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/dayforce"
)

func newTestDayforceAdapter(t *testing.T) (*DayforceAdapter, func()) {
	t.Helper()
	srv := dayforce.NewMockServer()
	a, err := NewDayforceAdapter(srv.URL, srv.Client())
	require.NoError(t, err)
	return a, srv.Close
}

func TestDayforceRoster(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	roster := a.Roster()
	require.NotEmpty(t, roster)

	names := make(map[string]string, len(roster))
	for _, c := range roster {
		names[c.Slug] = c.Name
	}
	assert.Equal(t, "Packaging Corporation of America", names["pca"])
}

func TestDayforceCareersURL(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	url, ok := a.CareersURL("pca")
	require.True(t, ok)
	assert.Equal(t, "https://jobs.dayforcehcm.com/en-US/pca/CANDIDATEPORTAL", url)

	_, ok = a.CareersURL("no-such-company")
	assert.False(t, ok)
}

func TestDayforceParseCareersURL(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	tests := []struct {
		name string
		url  string
		slug string
	}{
		{"culture form folds to roster slug", "https://jobs.dayforcehcm.com/en-US/pca/CANDIDATEPORTAL", "pca"},
		{"culture form with job id folds to roster slug", "https://jobs.dayforcehcm.com/en-US/pca/CANDIDATEPORTAL/jobs/62374", "pca"},
		{"locale-less form folds to roster slug", "https://jobs.dayforcehcm.com/pca/CANDIDATEPORTAL", "pca"},
		{"case-insensitive board code folds to roster slug", "https://jobs.dayforcehcm.com/en-US/PCA/candidateportal", "pca"},
		{"non-roster board mints canonical slug", "https://jobs.dayforcehcm.com/en-US/corpay/CANDIDATEPORTAL", "corpay/candidateportal"},
		{"non-roster locale-less board uses default culture", "https://jobs.dayforcehcm.com/unknown/CANDIDATEPORTAL", "unknown/candidateportal"},
		{"non-roster board preserves non-default culture", "https://jobs.dayforcehcm.com/fr-CA/unknown/CANDIDATEPORTAL", "unknown/candidateportal/fr-CA"},
		{"non-roster board omits default culture", "https://jobs.dayforcehcm.com/en-US/unknown/CANDIDATEPORTAL", "unknown/candidateportal"},
		{"mixed-case culture canonicalizes", "https://jobs.dayforcehcm.com/FR-ca/unknown/CANDIDATEPORTAL", "unknown/candidateportal/fr-CA"},
		{"same namespace different board does not fold back", "https://jobs.dayforcehcm.com/en-US/pca/someotherboard", "pca/someotherboard"},
		{"legacy per-tenant host, default board folds to roster slug", "https://us1234.dayforcehcm.com/CandidatePortal/en-US/pca", "pca"},
		{"legacy per-tenant host, explicit board", "https://us1234.dayforcehcm.com/CandidatePortal/en-US/mydayforce/Site/alljobs", "mydayforce/alljobs"},
		{"legacy host preserves non-default culture", "https://us1234.dayforcehcm.com/CandidatePortal/fr-ca/unknown/Site/CANDIDATEPORTAL", "unknown/candidateportal/fr-CA"},
		{"mydayforce legacy host folds to roster slug", "https://www.mydayforce.com/CandidatePortal/en-US/pca", "pca"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug, ok := a.ParseCareersURL(mustParseURL(t, tt.url))
			require.True(t, ok)
			assert.Equal(t, tt.slug, slug)
		})
	}
}

func TestDayforceParseCareersURLRejects(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	rejected := []string{
		"https://jobs.dayforcehcm.com/",
		"https://jobs.dayforcehcm.com/api/geo/pca/jobposting/search",
		"https://jobs.dayforcehcm.com/api/auth/csrf",
		"https://jobs.dayforcehcm.com/_next/static/chunks/foo.js",
		"https://jobs.dayforcehcm.com/_next/foo",
		"https://jobs.dayforcehcm.com/profile",
		"https://jobs.dayforcehcm.com/en-US/pca",
		"https://jobs.dayforcehcm.com/en-US/pca/jobs/62374",
		"https://jobs.dayforcehcm.com/en-US/profile",
		"https://jobs.dayforcehcm.com",
		"https://example.com/pca/CANDIDATEPORTAL",
		"https://us1234.dayforcehcm.com/CandidatePortal/en-US",
	}
	for _, raw := range rejected {
		t.Run(raw, func(t *testing.T) {
			_, ok := a.ParseCareersURL(mustParseURL(t, raw))
			assert.False(t, ok, "expected %q to be rejected", raw)
		})
	}
}

func TestDayforceSearch(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	res, err := a.Search(t.Context(), "pca", SearchParams{Page: 1})
	require.NoError(t, err)
	assert.Equal(t, 352, res.TotalCount)
	assert.Equal(t, 1, res.Page)
	assert.Equal(t, totalPages(352), res.TotalPages)
	require.Len(t, res.Jobs, pageSize)

	job := res.Jobs[0]
	assert.Equal(t, "62333", job.JobID)
	assert.Equal(t, "General Laborer- Wallula", job.Title)
	assert.Equal(t, "https://jobs.dayforcehcm.com/en-US/pca/CANDIDATEPORTAL/jobs/62333", job.URL)
}

// TestDayforceSearchPage2 proves Search remaps the upstream 25-row pages
// onto ats.pageSize: page 2 starts at search_rsp.json index 20 (62244)
// and does not overlap page 1.
func TestDayforceSearchPage2(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	page1, err := a.Search(t.Context(), "pca", SearchParams{Page: 1})
	require.NoError(t, err)
	require.Len(t, page1.Jobs, pageSize)
	assert.Equal(t, "60828", page1.Jobs[pageSize-1].JobID)

	page2, err := a.Search(t.Context(), "pca", SearchParams{Page: 2})
	require.NoError(t, err)
	require.NotEmpty(t, page2.Jobs)
	assert.Equal(t, "62244", page2.Jobs[0].JobID)

	ids1 := make(map[string]bool, len(page1.Jobs))
	for _, j := range page1.Jobs {
		ids1[j.JobID] = true
	}
	for _, j := range page2.Jobs {
		assert.False(t, ids1[j.JobID], "job %s appears on both page 1 and page 2", j.JobID)
	}
}

func TestDayforceSearchRemoteLocation(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	res, err := a.Search(t.Context(), "pca", SearchParams{})
	require.NoError(t, err)

	var found bool
	for _, j := range res.Jobs {
		if j.JobID == "62239" {
			found = true
			assert.Contains(t, j.Location, "Remote")
			assert.Contains(t, j.Location, "Tomahawk, WI 54487, USA")
		}
	}
	assert.True(t, found, "expected job 62239 (hasVirtualLocation) in the fixture")
}

// TestDayforceSearchNullLocation proves a continent-level posting location
// (isoCountryCode, stateCode, and cityName all null) both decodes without
// error and renders a non-empty, non-broken location string — it falls back
// to formattedAddress ("Europe") rather than an empty string built from the
// null fields.
func TestDayforceSearchNullLocation(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	res, err := a.Search(t.Context(), "mymilacron", SearchParams{})
	require.NoError(t, err)

	var found bool
	for _, j := range res.Jobs {
		if j.JobID == strconv.Itoa(dayforce.MockNullLocationJobID) {
			found = true
			assert.Equal(t, "Europe", j.Location)
		}
	}
	assert.True(t, found, "expected job %d (continent-level location) in the fixture", dayforce.MockNullLocationJobID)
}

// TestDayforceSearchNullPostingLocations proves a search row whose
// postingLocations array itself is null (not just a field within a
// location) renders as "Remote" rather than crashing or leaving a dangling
// "Remote · " prefix — jdemea/mymilacron/corpay's confirmed shape, mock's
// mymilacron page 2, jobPostingId 2418 (hasVirtualLocation: true).
func TestDayforceSearchNullPostingLocations(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	res, err := a.Search(t.Context(), "mymilacron", SearchParams{Page: 2})
	require.NoError(t, err)

	var found bool
	for _, j := range res.Jobs {
		if j.JobID == strconv.Itoa(dayforce.MockNullPostingLocationsJobID) {
			found = true
			assert.Equal(t, "Remote", j.Location)
		}
	}
	assert.True(t, found, "expected job %d (null postingLocations) in the fixture", dayforce.MockNullPostingLocationsJobID)
}

func TestDayforceSearchUnknownCompany(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	_, err := a.Search(t.Context(), "no-such-company", SearchParams{})
	assert.Error(t, err)
}

func TestDayforceSearchEmptyNonRosterBoard(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	res, err := a.Search(t.Context(), "emptyboard/candidateportal", SearchParams{})
	require.NoError(t, err)
	assert.Empty(t, res.Jobs)
	assert.Equal(t, 0, res.TotalCount)
}

func TestDayforceSearchUnsupportedCulture(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	_, err := a.Search(t.Context(), "badculture/candidateportal", SearchParams{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "site-info")
}

func TestDayforceSearchPastEndNonRosterSkipsSiteInfo(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	page1, err := a.Search(t.Context(), "nossr/candidateportal", SearchParams{Page: 1})
	require.NoError(t, err)
	require.NotEmpty(t, page1.Jobs)

	res, err := a.Search(t.Context(), "nossr/candidateportal", SearchParams{Page: 99})
	require.NoError(t, err)
	assert.Empty(t, res.Jobs)
	assert.Equal(t, page1.TotalCount, res.TotalCount)
}

func TestDayforceFilters(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	fs, err := a.Filters(t.Context(), "pca")
	require.NoError(t, err)
	assert.Len(t, fs, 4)
	assert.Contains(t, fs["pay_class"], "Full-time")
	assert.Contains(t, fs["pay_type"], "Salary")
	assert.NotEmpty(t, fs["department"])
	assert.Equal(t, []string{"true", "false"}, fs["travel_required"])
}

func TestDayforceSearchWithFilters(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	// pay_type resolves by display label to the attributeId the mock
	// server's fixture selection doesn't branch on, so this exercises the
	// resolution path without depending on a filtered fixture.
	_, err := a.Search(t.Context(), "pca", SearchParams{Filters: FilterSet{"pay_type": {"Salary"}}})
	require.NoError(t, err)

	_, err = a.Search(t.Context(), "pca", SearchParams{Filters: FilterSet{"pay_type": {"NoSuchPayType"}}})
	assert.Error(t, err)

	_, err = a.Search(t.Context(), "pca", SearchParams{Filters: FilterSet{"travel_required": {"true"}}})
	require.NoError(t, err)

	_, err = a.Search(t.Context(), "pca", SearchParams{Filters: FilterSet{"travel_required": {"maybe"}}})
	assert.Error(t, err)

	_, err = a.Search(t.Context(), "pca", SearchParams{Filters: FilterSet{"unknown_key": {"x"}}})
	assert.Error(t, err)

	_, err = a.Search(t.Context(), "pca", SearchParams{Filters: FilterSet{"pay_type": {"Salary", "Hourly"}}})
	assert.Error(t, err)
}

func TestDayforceSearchByLocation(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	unfiltered, err := a.Search(t.Context(), "pca", SearchParams{})
	require.NoError(t, err)
	require.NotEmpty(t, unfiltered.Jobs)

	res, err := a.Search(t.Context(), "pca", SearchParams{Location: "Chicago, Illinois, United States"})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
	assert.NotEqual(t, unfiltered.Jobs[0].JobID, res.Jobs[0].JobID)
}

func TestDayforceSearchLocationRemote(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	res, err := a.Search(t.Context(), "pca", SearchParams{Location: "  Remote  "})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
	assert.Equal(t, 1, res.TotalCount)
	assert.Equal(t, "62239", res.Jobs[0].JobID)
	assert.Contains(t, res.Jobs[0].Location, "Remote")
}

func TestDayforceDetail(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	d, err := a.Detail(t.Context(), "pca", strconv.Itoa(dayforce.MockJobID))
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(dayforce.MockJobID), d.JobID)
	assert.Equal(t, "Electrical Engineering Co Op/Intern - All Mills", d.Title)
	assert.Equal(t, "Packaging Corporation of America", d.Company)
	assert.Equal(t, "https://jobs.dayforcehcm.com/en-US/pca/CANDIDATEPORTAL/jobs/62374", d.URL)
	assert.NotContains(t, d.Description, "<p")
	assert.NotEmpty(t, d.Description)
}

// TestDayforceDetailNoLocationNotRemote proves a posting with zero
// postingLocations and hasVirtualLocation: false (jdemea 35762 — no location
// signal at all, unlike the other null-postingLocations postings which are
// all remote) renders an empty Location rather than a misleading value, and
// still decodes despite isoCurrencyRegion: null.
func TestDayforceDetailNoLocationNotRemote(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	d, err := a.Detail(t.Context(), "jdemea/candidateportal", strconv.Itoa(dayforce.MockNullCurrencyJobID))
	require.NoError(t, err)
	assert.Equal(t, "", d.Location)
}

func TestDayforceDetailNotFound(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	_, err := a.Detail(t.Context(), "pca", strconv.Itoa(dayforce.MockNotFoundJobID))
	assert.Error(t, err)
}

func TestDayforceDetailNonNumericJobID(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	_, err := a.Detail(t.Context(), "pca", "not-a-number")
	assert.Error(t, err)
}

// TestDayforceResolveNonRosterSlug proves a non-roster slug parses
// namespace, board, and culture without a SiteInfo fetch.
func TestDayforceResolveNonRosterSlug(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	tests := []struct {
		name    string
		slug    string
		culture string
	}{
		{name: "default culture", slug: "otherns/otherboard", culture: "en-US"},
		{name: "url culture", slug: "otherns/otherboard/fr-CA", culture: "fr-CA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)
			_, board, err := a.resolveSlug(t.Context(), tt.slug)
			require.NoError(err)
			assert.Equal("otherns", board.Namespace)
			assert.Equal("otherboard", board.JobBoardCode)
			assert.Equal(tt.culture, board.Culture())
			assert.Zero(board.JobBoardID)
		})
	}
}

func TestDayforceFiltersResolvesNonRosterJobBoardID(t *testing.T) {
	a, closeSrv := newTestDayforceAdapter(t)
	defer closeSrv()

	fs, err := a.Filters(t.Context(), "otherns/otherboard")
	require.NoError(t, err)
	assert.NotEmpty(t, fs)

	_, board, err := a.resolveSlug(t.Context(), "otherns/otherboard")
	require.NoError(t, err)
	id, err := a.jobBoardID(t.Context(), &board)
	require.NoError(t, err)
	assert.Equal(t, 1, id)
}
