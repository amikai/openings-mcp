package ats

import (
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/hrmos"
)

func testHrmosAdapter(t *testing.T) *HrmosAdapter {
	t.Helper()
	srv := hrmos.NewMockServer()
	t.Cleanup(srv.Close)
	return NewHrmosAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second}, nil)
}

func TestHrmosRoster(t *testing.T) {
	a := testHrmosAdapter(t)
	assert.Len(t, a.Roster(), len(hrmos.Companies))
	for _, c := range a.Roster() {
		assert.NotEmptyf(t, c.Slug, "roster entry with empty field: %+v", c)
		assert.NotEmptyf(t, c.Name, "roster entry with empty field: %+v", c)
	}
}

func TestHrmosSearchAll(t *testing.T) {
	a := testHrmosAdapter(t)
	res, err := a.Search(t.Context(), hrmos.MockSlugPaged, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 203, res.TotalCount)
}

// Every key and every value Filters returns must round-trip through
// Search with a non-zero TotalCount, for every fixture tenant.
func TestHrmosFiltersSearchRoundTrip(t *testing.T) {
	a := testHrmosAdapter(t)
	for _, slug := range []string{hrmos.MockSlugPaged, hrmos.MockSlugSmall} {
		fs, err := a.Filters(t.Context(), slug)
		require.NoError(t, err)
		require.NotEmpty(t, fs, "tenant %q", slug)
		for key, values := range fs {
			for _, v := range values {
				res, err := a.Search(t.Context(), slug, SearchParams{Filters: FilterSet{key: {v}}})
				require.NoError(t, err, "tenant %q key %q value %q", slug, key, v)
				assert.Greaterf(t, res.TotalCount, 0, "tenant %q key %q value %q", slug, key, v)
			}
		}
	}
}

// A filtered search must return every job carrying the chip and nothing else,
// so this pins TotalCount to a count taken independently from the dump rather
// than to a "fewer than unfiltered" band, which under-inclusion would pass.
func TestHrmosSearchFilterEngineer(t *testing.T) {
	srv := hrmos.NewMockServer()
	t.Cleanup(srv.Close)
	hc := &http.Client{Timeout: 5 * time.Second}
	a := NewHrmosAdapter(srv.URL, hc, nil)

	dump, err := hrmos.NewClient(srv.URL, hc).AllJobs(t.Context(), hrmos.MockSlugPaged)
	require.NoError(t, err)
	want := make(map[string]bool)
	for _, j := range dump.Jobs {
		if slices.Contains(j.Chips, "エンジニア") {
			want[j.ID] = true
		}
	}
	require.NotEmpty(t, want, "fixture carries no エンジニア chip")
	require.Less(t, len(want), dump.Total, "chip is on every job; filter proves nothing")

	filtered, err := a.Search(t.Context(), hrmos.MockSlugPaged, SearchParams{
		Filters: FilterSet{"職種": {"エンジニア"}},
	})
	require.NoError(t, err)
	assert.Equal(t, len(want), filtered.TotalCount)

	// Walk every page: no returned job may lack the chip.
	for page := 1; page <= filtered.TotalPages; page++ {
		res, err := a.Search(t.Context(), hrmos.MockSlugPaged, SearchParams{
			Filters: FilterSet{"職種": {"エンジニア"}},
			Page:    page,
		})
		require.NoError(t, err)
		for _, j := range res.Jobs {
			assert.Truef(t, want[j.JobID], "job %q lacks the エンジニア chip", j.JobID)
		}
	}
}

func TestHrmosSearchUnknownFilterKey(t *testing.T) {
	a := testHrmosAdapter(t)
	_, err := a.Search(t.Context(), hrmos.MockSlugPaged, SearchParams{
		Filters: FilterSet{"no-such-key": {"x"}},
	})
	assert.ErrorContains(t, err, `unknown filter key "no-such-key"; valid keys:`)
}

func TestHrmosSearchNoFacetsTenant(t *testing.T) {
	a := testHrmosAdapter(t)
	res, err := a.Search(t.Context(), hrmos.MockSlugNoFacets, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 86, res.TotalCount)

	fs, err := a.Filters(t.Context(), hrmos.MockSlugNoFacets)
	require.NoError(t, err)
	assert.Empty(t, fs)
}

func TestHrmosParseCareersURL(t *testing.T) {
	a := testHrmosAdapter(t)
	for _, raw := range []string{
		"https://hrmos.co/pages/moneyforward",
		"https://hrmos.co/pages/moneyforward/jobs",
		"https://hrmos.co/pages/moneyforward/jobs/0000265",
	} {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		slug, ok := a.ParseCareersURL(u)
		assert.Truef(t, ok, "url %q", raw)
		assert.Equal(t, "moneyforward", slug)
	}
}

func TestHrmosParseCareersURLUnrelatedHost(t *testing.T) {
	a := testHrmosAdapter(t)
	u, err := url.Parse("https://example.com/pages/moneyforward/jobs")
	require.NoError(t, err)
	_, ok := a.ParseCareersURL(u)
	assert.False(t, ok)
}

func TestHrmosDetail(t *testing.T) {
	a := testHrmosAdapter(t)
	d, err := a.Detail(t.Context(), hrmos.MockSlugSmall, hrmos.MockJobIDSalary)
	require.NoError(t, err)
	assert.Equal(t, hrmos.MockJobIDSalary, d.JobID)
	assert.NotEmpty(t, d.Title)
	assert.NotEmpty(t, d.Description)
}

// A 新卒 posting has no JSON-LD, so Detail must still surface the employer,
// the posting date, and the location from the page markup — and must not
// mistake the posting title for the company.
func TestHrmosDetailShinsotsuKeepsAllFields(t *testing.T) {
	a := testHrmosAdapter(t)
	d, err := a.Detail(t.Context(), hrmos.MockSlugShinsotsu, hrmos.MockJobIDShinsotsu)
	require.NoError(t, err)

	// Company is also roster-backed for this slug; the discriminating
	// assertion that it is the employer and not the title lives in the
	// provider package, where the markup is actually read.
	assert.Equal(t, "ラクスル株式会社", d.Company)
	assert.NotEqual(t, d.Title, d.Company)
	assert.Equal(t, "2026-01-08", d.PostedAt)
	assert.Equal(t, "東京都港区麻布台一丁目3番1号 麻布台ヒルズ 森JPタワー 19階", d.Location)
	assert.NotEmpty(t, d.Description)
}

// Detail must round-trip a JobID Search actually returned.
func TestHrmosDetailRoundTripsSearchJobID(t *testing.T) {
	a := testHrmosAdapter(t)
	res, err := a.Search(t.Context(), hrmos.MockSlugSmall, SearchParams{})
	require.NoError(t, err)

	var jobID string
	for _, j := range res.Jobs {
		if j.JobID == hrmos.MockJobIDSalary {
			jobID = j.JobID
			break
		}
	}
	require.NotEmpty(t, jobID, "mock job id present in search results")

	d, err := a.Detail(t.Context(), hrmos.MockSlugSmall, jobID)
	require.NoError(t, err)
	assert.Equal(t, jobID, d.JobID)
}

func TestHrmosDetailNotFound(t *testing.T) {
	a := testHrmosAdapter(t)
	_, err := a.Detail(t.Context(), hrmos.MockSlugPaged, hrmos.MockJobIDNotFound)
	assert.Error(t, err)
}

// Locations come from the sg-tag-location chip alone (fact 17): the
// verbatim address, including a "他(N)" suffix, never collapsed to a bare
// facet chip like "東京" alone.
func TestHrmosSearchLocationFromChipAlone(t *testing.T) {
	a := testHrmosAdapter(t)
	res, err := a.Search(t.Context(), hrmos.MockSlugPaged, SearchParams{})
	require.NoError(t, err)

	var job *JobSummary
	for i := range res.Jobs {
		if res.Jobs[i].JobID == "00000des01" {
			job = &res.Jobs[i]
			break
		}
	}
	require.NotNil(t, job, "fixture job 00000des01 present")
	assert.Equal(t, "東京都港区芝浦3-1-21 msb Tamachi 田町ステーションタワーS 21F 他(3)", job.Location)
	assert.NotEqual(t, "東京", job.Location)
}

// The visional fixture job's location retains the bracketed office label
// prefix, the other half of fact 17's "verbatim" requirement.
func TestHrmosSearchLocationRetainsOfficeLabel(t *testing.T) {
	a := testHrmosAdapter(t)
	res, err := a.Search(t.Context(), hrmos.MockSlugSmall, SearchParams{})
	require.NoError(t, err)

	var job *JobSummary
	for i := range res.Jobs {
		if res.Jobs[i].JobID == hrmos.MockJobIDSalary {
			job = &res.Jobs[i]
			break
		}
	}
	require.NotNil(t, job, "fixture job present")
	assert.Contains(t, job.Location, "［渋谷本社］")
}

// isRemote is a known gap (fact 16/the package decision): it only matches
// the sg-tag-location chip against a fixed marker list, never JD prose, so
// it is false on every fixture tenant and location=remote matches nothing.
func TestHrmosSearchLocationRemoteIsAKnownGap(t *testing.T) {
	a := testHrmosAdapter(t)
	res, err := a.Search(t.Context(), hrmos.MockSlugPaged, SearchParams{Location: "remote"})
	require.NoError(t, err)
	assert.Equal(t, 0, res.TotalCount)
}

func TestHrmosSearchLocationSubstring(t *testing.T) {
	a := testHrmosAdapter(t)
	res, err := a.Search(t.Context(), hrmos.MockSlugSmall, SearchParams{Location: "渋谷"})
	require.NoError(t, err)
	assert.Greater(t, res.TotalCount, 0)
}

func TestHrmosIsRemote(t *testing.T) {
	for _, tc := range []struct {
		location string
		want     bool
	}{
		{"東京都港区芝浦3-1-21", false},
		{"フルリモート", true},
		{"リモート勤務可", true},
		{"在宅勤務", true},
		{"フルテレワーク", true},
		{"Remote (Japan)", true},
	} {
		assert.Equal(t, tc.want, hrmosIsRemote(tc.location), "location %q", tc.location)
	}
}

func TestHrmosPostedAt(t *testing.T) {
	assert.Equal(t, "2021-06-02", hrmosPostedAt("2021-06-02T13:25:56.000Z"))
	assert.Equal(t, "", hrmosPostedAt(""))
}
