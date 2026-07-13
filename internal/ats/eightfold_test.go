package ats

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/eightfold"
)

// testEightfoldAdapter points the adapter at eightfold's mock server, which
// was captured from morganstanley.eightfold.ai. It doesn't inspect the
// domain query param, so tests can address it through any roster slug
// (e.g. "eaton") — Company in JobDetail/JobSummary comes from the roster
// lookup, not from the fixture content, so assertions stay accurate even
// though the underlying fixture data is Morgan-Stanley-flavored.
func testEightfoldAdapter(t *testing.T) *EightfoldAdapter {
	t.Helper()
	mock := eightfold.NewMockServer()
	t.Cleanup(mock.Close)
	a := NewEightfoldAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return mock.URL }
	return a
}

func TestEightfoldRosterBuildsRegistry(t *testing.T) {
	_, err := NewRegistry(NewEightfoldAdapter(http.DefaultClient))
	require.NoError(t, err)
}

func TestEightfoldRosterReturnsCompanyNames(t *testing.T) {
	a := NewEightfoldAdapter(http.DefaultClient)
	roster := a.Roster()
	require.NotEmpty(t, roster)
	found := false
	for _, c := range roster {
		if c.Slug == "eaton" {
			found = true
			assert.Equal(t, "Eaton", c.Name)
		}
	}
	assert.True(t, found, "expected eaton in roster")
}

// TestEightfoldSearchFillsUnifiedPageFromTwoUpstreamPages proves the
// upstream-fixed-page-of-10 workaround: a unified page (20) is two
// upstream requests (start=0 and start=10) concatenated.
func TestEightfoldSearchFillsUnifiedPageFromTwoUpstreamPages(t *testing.T) {
	a := testEightfoldAdapter(t)
	res, err := a.Search(t.Context(), "eaton", SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 1330, res.TotalCount)
	assert.Equal(t, 1, res.Page)
	assert.Len(t, res.Jobs, 20, "should concatenate two upstream pages of 10")

	first := res.Jobs[0]
	assert.Equal(t, "549798858854", first.JobID)
	assert.Equal(t, "Vice President - Prin Software Eng", first.Title)
	assert.Contains(t, first.URL, "/careers/job/549798858854")
}

func TestEightfoldSearchQueryLocation(t *testing.T) {
	a := testEightfoldAdapter(t)
	res, err := a.Search(t.Context(), "eaton", SearchParams{Query: "engineer", Location: "New York"})
	require.NoError(t, err)
	assert.Equal(t, 43, res.TotalCount)
	require.NotEmpty(t, res.Jobs)
	for _, j := range res.Jobs {
		assert.Contains(t, j.Location, "New York")
	}
}

func TestEightfoldFilters(t *testing.T) {
	a := testEightfoldAdapter(t)
	fs, err := a.Filters(t.Context(), "eaton")
	require.NoError(t, err)
	require.NotEmptyf(t, fs["businessarea"], "FilterSet missing expected dimension: %v", fs)
	assert.Contains(t, fs["businessarea"], "Technology")
	// "include_remote" is a toggle with null options — it must not appear.
	assert.NotContains(t, fs, "include_remote")
}

// TestEightfoldSearchWithFilterResolvesLabelToValue proves a display label
// ("Technology") is resolved to the API's lowercase value ("technology")
// via a probe search before the real filtered request.
func TestEightfoldSearchWithFilterResolvesLabelToValue(t *testing.T) {
	a := testEightfoldAdapter(t)
	res, err := a.Search(t.Context(), "eaton", SearchParams{
		Filters: map[string][]string{"businessarea": {"Technology"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 112, res.TotalCount)
}

func TestEightfoldFilterKeyNotFoundTeaches(t *testing.T) {
	a := testEightfoldAdapter(t)
	_, err := a.Search(t.Context(), "eaton", SearchParams{
		Filters: map[string][]string{"bogus": {"x"}},
	})
	require.ErrorContains(t, err, "businessarea", "error should list valid keys")
}

func TestEightfoldFilterValueNotFoundTeaches(t *testing.T) {
	a := testEightfoldAdapter(t)
	_, err := a.Search(t.Context(), "eaton", SearchParams{
		Filters: map[string][]string{"businessarea": {"Not A Real Area"}},
	})
	require.ErrorContains(t, err, "Technology", "error should list available values")
}

func TestEightfoldDetail(t *testing.T) {
	a := testEightfoldAdapter(t)
	d, err := a.Detail(t.Context(), "eaton", strconv.Itoa(eightfold.MockPositionID))
	require.NoError(t, err)
	assert.Equal(t, "Vice President - Prin Software Eng", d.Title)
	assert.Equal(t, "Eaton", d.Company, "Company comes from the eaton roster lookup, not the fixture")
	assert.NotContains(t, d.Description, "<p>", "Description should be converted from HTML")
}

func TestEightfoldDetailNotFound(t *testing.T) {
	a := testEightfoldAdapter(t)
	_, err := a.Detail(t.Context(), "eaton", "1")
	require.ErrorContains(t, err, "not found")
}

func TestEightfoldDetailRejectsMalformedJobID(t *testing.T) {
	a := testEightfoldAdapter(t)
	_, err := a.Detail(t.Context(), "eaton", "garbage")
	assert.Error(t, err, "want error for malformed job_id")
}

func TestEightfoldUnknownSlugTeaches(t *testing.T) {
	a := NewEightfoldAdapter(http.DefaultClient)
	_, err := a.Search(t.Context(), "not-a-tenant", SearchParams{})
	require.ErrorContains(t, err, "roster tenant slug")
}

func TestEightfoldParseCareersURL(t *testing.T) {
	a := NewEightfoldAdapter(http.DefaultClient)

	slug, ok := a.ParseCareersURL(mustParseURL(t, "https://eaton.eightfold.ai/careers/job/687237347150"))
	require.True(t, ok)
	assert.Equal(t, "eaton", slug)

	// A tenant not on the roster is refused: the "domain" query param can't
	// be recovered from the URL alone (see EightfoldAdapter's doc comment).
	_, ok = a.ParseCareersURL(mustParseURL(t, "https://some-other-tenant.eightfold.ai/careers"))
	assert.False(t, ok)

	_, ok = a.ParseCareersURL(mustParseURL(t, "https://jobs.lever.co/acme"))
	assert.False(t, ok)
}

func TestEightfoldSearchRejectsHugePage(t *testing.T) {
	a := testEightfoldAdapter(t)
	_, err := a.Search(t.Context(), "eaton", SearchParams{Page: 1 << 40})
	require.NoError(t, err, "a huge page is just an empty result, not an error, for a fixed-size upstream page")
}
