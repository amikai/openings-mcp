package ats

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/mokahr"
)

// mockMokaHRSlug is the careers site the provider mock server serves: two
// captured pages of five postings each on a board that reports 35, then a
// captured past-the-end page.
const _mockMokaHRSlug = mokahr.MockOrgID + "/" + mokahr.MockSiteID

func testMokaHRAdapter(t *testing.T) *MokaHRAdapter {
	t.Helper()
	srv := mokahr.NewMockServer()
	t.Cleanup(srv.Close)
	a, err := NewMokaHRAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	require.NoError(t, err)
	// The captured pages hold five postings each, so the board read only
	// spans more than one request at this size.
	a.dumpPageSize = mokahr.MockPageSize
	return a
}

func TestMokaHRRoster(t *testing.T) {
	a := testMokaHRAdapter(t)
	assert.Len(t, a.Roster(), len(mokahr.Companies))
	for _, c := range a.Roster() {
		assert.NotEmptyf(t, c.Slug, "roster entry with empty field: %+v", c)
		assert.NotEmptyf(t, c.Name, "roster entry with empty field: %+v", c)
		assert.Containsf(t, c.Slug, "/", "slug %q must name a site as well as a tenant", c.Slug)
	}
}

func TestMokaHRParseCareersURL(t *testing.T) {
	a := testMokaHRAdapter(t)
	for input, want := range map[string]string{
		"https://app.mokahr.com/social-recruitment/high-flyer/140576":                                       "high-flyer/140576",
		"https://app.mokahr.com/social-recruitment/high-flyer/140576#/job/7dcd6fde-84f1-4deb-890c-f1f275df": "high-flyer/140576",
		"https://APP.MokaHR.com/social-recruitment/Moka/118263":                                             "moka/118263",
		"https://app.mokahr.com/campus_apply/moka/118263":                                                   "moka/118263",
		"https://app.mokahr.com/campus-recruitment/moka/118263":                                             "moka/118263",
	} {
		u, err := url.Parse(input)
		require.NoError(t, err)
		slug, ok := a.ParseCareersURL(u)
		assert.Truef(t, ok, "want %q recognized", input)
		assert.Equal(t, want, slug)
	}

	// A tenant URL with no site id cannot be resolved without following the
	// redirect it answers with, which ParseCareersURL cannot do.
	for _, input := range []string{
		"https://app.mokahr.com/social-recruitment/high-flyer",
		"https://app.mokahr.com/login",
		"https://jobs.example.com/social-recruitment/high-flyer/140576",
	} {
		u, err := url.Parse(input)
		require.NoError(t, err)
		_, ok := a.ParseCareersURL(u)
		assert.Falsef(t, ok, "want %q rejected", input)
	}
}

// TestMokaHRSearchAll takes the cheap path: no text query means one request,
// and upstream's own total and paging carry through unchanged.
func TestMokaHRSearchAll(t *testing.T) {
	a := testMokaHRAdapter(t)

	res, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, mokahr.MockTotal, res.TotalCount)
	assert.Equal(t, 1, res.Page)
	assert.Equal(t, 2, res.TotalPages)
	require.Len(t, res.Jobs, mokahr.MockPageSize)

	first := res.Jobs[0]
	assert.Equal(t, mokahr.MockJobID, first.JobID)
	assert.Equal(t, "多模态理解（数据/算法）研究员", first.Title)
	assert.Equal(t, "2026-06-25", first.PostedAt)
	assert.Equal(t,
		"https://app.mokahr.com/social-recruitment/high-flyer/140576#/job/"+mokahr.MockJobID,
		first.URL)
	// The posting names its districts, Gongshu and Haidian; the site's
	// location facet is what turns those into cities. Workplaces are sorted,
	// not left in upstream's order, which differs per endpoint.
	assert.Equal(t, "Beijing, China; Hangzhou, Zhejiang, China", first.Location)
}

// TestMokaHRSearchQueryReadsDescriptions is the reason text search runs
// locally: "CLIP" appears in one posting's description and in no title, so
// MokaHR's own title-only keyword parameter would find nothing.
func TestMokaHRSearchQueryReadsDescriptions(t *testing.T) {
	a := testMokaHRAdapter(t)

	res, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{Query: "CLIP"})
	require.NoError(t, err)
	require.Equal(t, 1, res.TotalCount)
	assert.Equal(t, "多模态理解（数据/算法）研究员", res.Jobs[0].Title)
}

func TestMokaHRSearchQueryRanksTitleHitsFirst(t *testing.T) {
	a := testMokaHRAdapter(t)

	res, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{Query: "研究员"})
	require.NoError(t, err)
	require.Equal(t, 4, res.TotalCount)
	for _, j := range res.Jobs {
		assert.Containsf(t, j.Title, "研究员", "title hits rank ahead of description hits")
	}
}

// TestMokaHRLocationResolvesUpstream covers the split: a place the site
// publishes is filtered upstream by id, and anything else falls through to
// local matching.
func TestMokaHRLocationResolvesUpstream(t *testing.T) {
	a := testMokaHRAdapter(t)

	facets, err := a.facets(t.Context(), mokaHRSite{org: mokahr.MockOrgID, site: mokahr.MockSiteID})
	require.NoError(t, err)

	// A facet city, and the province it sits in, both resolve to the ids
	// MokaHR filters on — which is the only path that works on a tenant
	// whose postings carry no locations.
	city, ok := facets.locationIDsFor("hangzhou")
	require.True(t, ok)
	assert.Equal(t, []int{31373}, city)
	province, ok := facets.locationIDsFor("Zhejiang")
	require.True(t, ok)
	assert.Equal(t, []int{31373}, province)

	// A district is not a facet value, so it stays a local text match.
	_, ok = facets.locationIDsFor("Gongshu")
	assert.False(t, ok)
	district, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{Location: "Gongshu"})
	require.NoError(t, err)
	assert.Equal(t, 9, district.TotalCount)
}

// TestMokaHRLocationRendersFromFacets guards the invariant Detail depends on:
// postings report districts ("Gongshu"), and only the facet knows the city
// and province, so every rendering must go through it.
func TestMokaHRLocationRendersFromFacets(t *testing.T) {
	a := testMokaHRAdapter(t)

	res, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{Location: "Gongshu"})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
	for _, j := range res.Jobs {
		assert.Containsf(t, j.Location, "Hangzhou", "job %q: %s", j.Title, j.Location)
		assert.NotContainsf(t, j.Location, "Gongshu", "the district is matched on, not displayed: %s", j.Location)
	}
}

// TestMokaHRSearchWalksEveryPage covers the full-board read: the mock hands
// out two captured pages and then an empty one.
func TestMokaHRSearchWalksEveryPage(t *testing.T) {
	a := testMokaHRAdapter(t)

	res, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{Query: "团队"})
	require.NoError(t, err)
	// Every posting on both captured pages matches, so a single-page read
	// would have found half of them.
	assert.Equal(t, 2*mokahr.MockPageSize, res.TotalCount)
	seen := make(map[string]bool, res.TotalCount)
	for _, j := range res.Jobs {
		assert.Falsef(t, seen[j.JobID], "job %s returned twice", j.JobID)
		seen[j.JobID] = true
	}
}

// TestMokaHRLocationUnanswerable covers the tenants that keep workplaces off
// the listing. Matching locally against a board that publishes none would
// answer "no such jobs", which is not the same as "cannot be asked here".
func TestMokaHRLocationUnanswerable(t *testing.T) {
	a := testMokaHRAdapter(t)
	slug := mokahr.MockNoLocationOrgID + "/" + mokahr.MockNoLocationSiteID

	// The board itself still searches.
	res, err := a.Search(t.Context(), slug, SearchParams{})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jobs)
	assert.Empty(t, res.Jobs[0].Location, "the listing carries no workplace to report")

	_, err = a.Search(t.Context(), slug, SearchParams{Location: "Shanghai"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not publish per-job locations")

	// It publishes no facets either, so get_filters is honestly empty and a
	// filter says so rather than trailing an empty list.
	fs, err := a.Filters(t.Context(), slug)
	require.NoError(t, err)
	assert.Empty(t, fs)
	_, err = a.Search(t.Context(), slug, SearchParams{Filters: FilterSet{"city": {"Shanghai"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publishes no filter dimensions at all")
}

func TestMokaHRFilters(t *testing.T) {
	a := testMokaHRAdapter(t)

	fs, err := a.Filters(t.Context(), _mockMokaHRSlug)
	require.NoError(t, err)
	assert.Equal(t, []string{"Beijing", "Hangzhou", "Ulanqab"}, fs["city"])
	// 职能 is a two-level tree; both levels are offered.
	assert.Contains(t, fs["category"], "深度学习研究员")
	assert.Contains(t, fs["category"], "职能部门")
	assert.Contains(t, fs["category"], "法务团队")

	// Anything get_filters advertises has to be something search accepts,
	// including a 职能 that only appears as a child of another.
	for key, values := range fs {
		for _, v := range values {
			_, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{Filters: FilterSet{key: {v}}})
			require.NoErrorf(t, err, "%s=%q is offered but rejected", key, v)
		}
	}
}

func TestMokaHRFiltersRejectUnknownKeyAndValue(t *testing.T) {
	a := testMokaHRAdapter(t)

	_, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{Filters: FilterSet{"department": {"DeepSeek"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown filter key "department"`)
	assert.Contains(t, err.Error(), "valid keys: category, city")

	_, err = a.Search(t.Context(), _mockMokaHRSlug, SearchParams{Filters: FilterSet{"city": {"Shenzhen"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `filter value "Shenzhen" not found for "city"`)
	assert.Contains(t, err.Error(), "Beijing, Hangzhou, Ulanqab")
}

func TestMokaHRDetail(t *testing.T) {
	a := testMokaHRAdapter(t)

	d, err := a.Detail(t.Context(), _mockMokaHRSlug, mokahr.MockJobID)
	require.NoError(t, err)
	assert.Equal(t, mokahr.MockJobID, d.JobID)
	assert.Equal(t, "多模态理解（数据/算法）研究员", d.Title)
	assert.Equal(t, "DeepSeek", d.Company, "a roster slug reports the curated display name")
	assert.Equal(t, "2026-06-25", d.PostedAt)
	assert.Equal(t,
		"https://app.mokahr.com/social-recruitment/high-flyer/140576#/job/"+mokahr.MockJobID,
		d.URL)
	// The description arrives as HTML and is handed over as plain text.
	assert.NotContains(t, d.Description, "<p>")
	assert.Contains(t, d.Description, "【团队使命】")

	// Detail resolves districts to cities the same way search does, so a
	// posting does not change address between the two.
	res, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{})
	require.NoError(t, err)
	require.Equal(t, mokahr.MockJobID, res.Jobs[0].JobID)
	assert.Equal(t, res.Jobs[0].Location, d.Location)
}

func TestMokaHRDetailNotFound(t *testing.T) {
	a := testMokaHRAdapter(t)

	_, err := a.Detail(t.Context(), _mockMokaHRSlug, mokahr.MockUnknownJobID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "job_id exactly as returned by the job search")
}

func TestMokaHRUnknownSite(t *testing.T) {
	a := testMokaHRAdapter(t)

	_, err := a.Search(t.Context(), mokahr.MockUnknownOrgID+"/1", SearchParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found upstream")

	// A slug that names no site at all never reaches the network.
	_, err = a.Search(t.Context(), "high-flyer", SearchParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "including its site id")
}

// TestMokaHRPageCount pins the arithmetic the concurrent board read fans out
// on. An off-by-one here would silently skip a page's postings rather than
// fail, which is why it is tested apart from the network path.
func TestMokaHRPageCount(t *testing.T) {
	for _, tc := range []struct {
		name      string
		total     int
		pageSize  int
		wantPages int
	}{
		{"empty board still holds the fetched page", 0, 50, 1},
		{"single posting", 1, 50, 1},
		{"exactly one page", 50, 50, 1},
		{"one past a page boundary", 51, 50, 2},
		{"exact multiple", 100, 50, 2},
		{"largest roster board", 1284, 50, 26},
		{"at the candidate cap", _maxMokaHRCandidates, 50, 40},
		{"total upstream declined to report", -1, 50, 1},
		{"page size lost", 35, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mokaHRPageCount(tc.total, tc.pageSize)
			assert.Equal(t, tc.wantPages, got)
			if tc.total > 0 && tc.pageSize > 0 {
				// Every posting has to land on some page.
				assert.GreaterOrEqual(t, got*tc.pageSize, tc.total)
				// And no page beyond the last may be requested.
				assert.Less(t, (got-1)*tc.pageSize, tc.total)
			}
		})
	}
}

func TestMokaHRResolveSite(t *testing.T) {
	// A roster company resolves by slug and keeps its display name; an
	// unlisted site falls back to naming itself after its tenant.
	site, err := resolveMokaHRSite("HIGH-FLYER/140576")
	require.NoError(t, err)
	assert.Equal(t, mokaHRSite{org: "high-flyer", site: "140576", name: "DeepSeek"}, site)

	site, err = resolveMokaHRSite("someco/99")
	require.NoError(t, err)
	assert.Equal(t, mokaHRSite{org: "someco", site: "99", name: "someco"}, site)

	_, err = resolveMokaHRSite("someco")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "unknown company"))
}

// TestMokaHRSearchFiltersUpstream proves the structured filter reaches the
// wire. The mock answers filtered requests with a different, smaller capture,
// so dropping LocationIds from the request would change the result set rather
// than pass silently.
func TestMokaHRSearchFiltersUpstream(t *testing.T) {
	a := testMokaHRAdapter(t)

	unfiltered, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{})
	require.NoError(t, err)
	require.Equal(t, mokahr.MockTotal, unfiltered.TotalCount)

	// "Ulanqab" is the facet city whose ids the filtered capture was taken for.
	res, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{Filters: FilterSet{"city": {"Ulanqab"}}})
	require.NoError(t, err)
	assert.Equal(t, 2, res.TotalCount, "the filter has to narrow the board upstream")
	require.Len(t, res.Jobs, 2)
	assert.Equal(t, "采购团队", res.Jobs[0].Title)

	// A 职能 filter travels the same way.
	byCategory, err := a.Search(t.Context(), _mockMokaHRSlug, SearchParams{Filters: FilterSet{"category": {"运维"}}})
	require.NoError(t, err)
	assert.Equal(t, 2, byCategory.TotalCount)
}

// TestMokaHRSearchWithoutUsableTotal covers a board whose listing reports
// total 0 while still returning a full page. Trusting that total would end the
// read after one page and answer from a fraction of the board.
func TestMokaHRSearchWithoutUsableTotal(t *testing.T) {
	a := testMokaHRAdapter(t)
	slug := mokahr.MockOrgID + "/" + mokahr.MockZeroTotalSiteID

	res, err := a.Search(t.Context(), slug, SearchParams{Query: "团队"})
	require.NoError(t, err)
	// Both captured pages match; stopping at the reported total would have
	// found only the first.
	assert.Equal(t, 2*mokahr.MockPageSize, res.TotalCount)
	seen := make(map[string]bool, res.TotalCount)
	for _, j := range res.Jobs {
		assert.Falsef(t, seen[j.JobID], "job %s returned twice", j.JobID)
		seen[j.JobID] = true
	}
}
