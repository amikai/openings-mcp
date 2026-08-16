package ats

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dj(id, title, orgUnit, desc, loc string, posted time.Time, fields map[string][]string, remote bool) dumpJob {
	return dumpJob{
		summary:     JobSummary{JobID: id, Title: title, Location: loc, PostedAt: posted.Format("2006-01-02"), URL: "https://example.com/" + id},
		sortKey:     posted,
		orgUnit:     orgUnit,
		description: desc,
		locations:   loc,
		fields:      fields,
		isRemote:    remote,
	}
}

func testJobs() []dumpJob {
	day := func(n int) time.Time { return time.Date(2026, 7, n, 0, 0, 0, 0, time.UTC) }
	return []dumpJob{
		dj("a", "Senior Go Engineer", "Platform", "You will write Go services", "Taipei, Taiwan", day(1), map[string][]string{"team": {"Platform"}, "commitment": {"Full-time"}}, false),
		dj("b", "Frontend Engineer", "Web", "React and TypeScript, some Go tooling", "London, UK", day(3), map[string][]string{"team": {"Web"}, "commitment": {"Full-time"}}, false),
		dj("c", "Data Scientist", "ML", "Python and statistics", "Remote - US", day(2), map[string][]string{"team": {"ML"}, "commitment": {"Contract"}}, true),
	}
}

func TestSearchDumpQueryANDAcrossWords(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Query: "go engineer"})
	require.NoError(t, err)
	// "go engineer": job a matches both words in title; job b matches
	// "engineer" in title and "go" only in description — still a match
	// (AND is across the whole text), but ranked below the title hit.
	require.Len(t, res.Jobs, 2)
	assert.Equal(t, "a", res.Jobs[0].JobID, "title hit should rank first")
	assert.Equal(t, 2, res.TotalCount)
}

func TestSearchDumpRanksOrgUnitBeforeDescription(t *testing.T) {
	older := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	jobs := []dumpJob{
		dj("org", "Engineer", "Platform", "", "Remote", older, nil, true),
		dj("body", "Engineer", "", "Build the platform", "Remote", newer, nil, true),
	}

	res, err := searchDump(jobs, SearchParams{Query: "platform"})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 2)
	assert.Equal(t, "org", res.Jobs[0].JobID)
}

func TestSearchDumpSortNewestFirstIDTiebreak(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{})
	require.NoError(t, err)
	// No query: rank is uniform, so order is posted desc: b(3rd) c(2nd) a(1st).
	want := []string{"b", "c", "a"}
	got := make([]string, len(res.Jobs))
	for i, j := range res.Jobs {
		got[i] = j.JobID
	}
	assert.Equal(t, want, got)
}

func TestSearchDumpSortsByIDWhenRankAndTimeTie(t *testing.T) {
	posted := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	jobs := []dumpJob{
		dj("b", "Engineer", "", "", "Remote", posted, nil, true),
		dj("a", "Engineer", "", "", "Remote", posted, nil, true),
	}

	res, err := searchDump(jobs, SearchParams{})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 2)
	assert.Equal(t, "a", res.Jobs[0].JobID)
	assert.Equal(t, "b", res.Jobs[1].JobID)
}

func TestSearchDumpLocation(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Location: "taipei"})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1, "location=taipei should match only job a")
	assert.Equal(t, "a", res.Jobs[0].JobID)
}

func TestSearchDumpRemoteSpecialCase(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Location: "remote"})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1, "location=remote should match only job c")
	assert.Equal(t, "c", res.Jobs[0].JobID)
}

func TestSearchDumpFiltersORWithinKeyANDAcrossKeys(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{
		Filters: map[string][]string{"team": {"Platform", "Web"}, "commitment": {"Full-time"}},
	})
	require.NoError(t, err)
	assert.Len(t, res.Jobs, 2, "want a and b")
}

func TestSearchDumpFilterValueCaseInsensitive(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Filters: map[string][]string{"team": {"platform"}}})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1, "want job a")
	assert.Equal(t, "a", res.Jobs[0].JobID)
}

func TestSearchDumpUnknownFilterKeyTeaches(t *testing.T) {
	_, err := searchDump(testJobs(), SearchParams{Filters: map[string][]string{"bogus": {"x"}}})
	require.ErrorContains(t, err, "team", "error should list valid keys")
}

func TestSearchDumpPagination(t *testing.T) {
	jobs := make([]dumpJob, 0, 45)
	for i := range 45 {
		posted := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour)
		jobs = append(jobs, dj(strings.Repeat("z", 3)+string(rune('a'+i%26))+string(rune('0'+i/26)), "Engineer", "", "", "X", posted, nil, false))
	}
	page2, err := searchDump(jobs, SearchParams{Page: 2})
	require.NoError(t, err)
	assert.Equal(t, 45, page2.TotalCount)
	assert.Equal(t, 3, page2.TotalPages)
	assert.Equal(t, 2, page2.Page)
	assert.Len(t, page2.Jobs, pageSize)

	page3, err := searchDump(jobs, SearchParams{Page: 3})
	require.NoError(t, err)
	assert.Len(t, page3.Jobs, 5)

	page9, err := searchDump(jobs, SearchParams{Page: 9})
	require.NoError(t, err)
	assert.Empty(t, page9.Jobs, "past-the-end page should be empty")

	// Determinism: two identical calls agree item-for-item.
	again, err := searchDump(jobs, SearchParams{Page: 2})
	require.NoError(t, err)
	for i := range page2.Jobs {
		assert.Equal(t, page2.Jobs[i].JobID, again.Jobs[i].JobID, "pagination is not deterministic")
	}
}

func TestSearchDumpHugePageDoesNotPanic(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Page: math.MaxInt})
	require.NoError(t, err)
	assert.Empty(t, res.Jobs)
	assert.Equal(t, math.MaxInt, res.Page)
}

func TestDistinctFilters(t *testing.T) {
	fs := distinctFilters(testJobs())
	require.Len(t, fs["team"], 3)
	assert.Equal(t, "ML", fs["team"][0], `fs["team"] should be sorted [ML Platform Web]`)
	assert.Len(t, fs["commitment"], 2)
}

// A job can carry several values in one dimension (e.g. a Greenhouse job in
// two departments); a filter must match any of them, and get_filters must
// list all of them.
func TestSearchDumpFilterMatchesAnyFieldValue(t *testing.T) {
	posted := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	jobs := []dumpJob{
		dj("m", "Engineer", "", "", "X", posted, map[string][]string{"department": {"Engineering", "Platform"}}, false),
		dj("n", "Designer", "", "", "X", posted, map[string][]string{"department": {"Design"}}, false),
	}

	res, err := searchDump(jobs, SearchParams{Filters: map[string][]string{"department": {"platform"}}})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1, "secondary department value should match")
	assert.Equal(t, "m", res.Jobs[0].JobID)
}

func TestDistinctFiltersListsAllFieldValues(t *testing.T) {
	posted := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	jobs := []dumpJob{
		dj("m", "Engineer", "", "", "X", posted, map[string][]string{"department": {"Engineering", "Platform"}}, false),
	}
	fs := distinctFilters(jobs)
	assert.Equal(t, []string{"Engineering", "Platform"}, fs["department"])
}
