package ats

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/avature"
)

// avatureMockSlug is a roster slug used only for mock-backed tests. The
// adapter overrides baseURL to the mock server, so live DNS is never hit.
const avatureMockSlug = "bloomberg.avature.net/careers"

func testAvatureAdapter(t *testing.T, mockPortal string) *AvatureAdapter {
	t.Helper()
	mock := avature.NewMockServer()
	t.Cleanup(mock.Close)
	a := NewAvatureAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return mock.URL + mockPortal }
	return a
}

func TestAvatureRosterBuildsRegistry(t *testing.T) {
	_, err := NewRegistry(NewAvatureAdapter(http.DefaultClient))
	require.NoError(t, err)
}

func TestAvatureRosterReturnsCompanyNames(t *testing.T) {
	a := NewAvatureAdapter(http.DefaultClient)
	roster := a.Roster()
	require.NotEmpty(t, roster)
	found := false
	for _, c := range roster {
		if c.Slug == "koch.avature.net/careers" {
			found = true
			assert.Equal(t, "Koch Industries", c.Name)
		}
	}
	assert.True(t, found, "expected koch.avature.net/careers in roster")
}

func TestAvatureParseCareersURL(t *testing.T) {
	a := NewAvatureAdapter(http.DefaultClient)
	cases := []struct {
		raw  string
		ok   bool
		slug string
	}{
		{"https://koch.avature.net/en_US/careers/SearchJobs", true, "koch.avature.net/careers"},
		{"https://bloomberg.avature.net/careers/JobDetail/Some-Job/123", true, "bloomberg.avature.net/careers"},
		{"https://dpdhlgroup.avature.net/en_US/jobs/SearchJobs", true, "dpdhlgroup.avature.net/jobs"},
		{"https://bloomberg.avature.net/", false, ""},
		{"https://www.avature.net/career-sites/", false, ""},
		{"https://careers.unifiservice.com/careers/SearchJobs", false, ""},
		{"https://boards.greenhouse.io/x", false, ""},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.raw)
		require.NoError(t, err)
		slug, ok := a.ParseCareersURL(u)
		assert.Equal(t, tc.ok, ok, tc.raw)
		assert.Equal(t, tc.slug, slug, tc.raw)
	}
}

// TestAvatureSearch stitches the 12-job upstream pages into the unified
// 20-job page: offsets 0 and 12 both come from real Bloomberg captures.
func TestAvatureSearch(t *testing.T) {
	a := testAvatureAdapter(t, "/careers")
	res, err := a.Search(t.Context(), avatureMockSlug, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 436, res.TotalCount)
	assert.Equal(t, 22, res.TotalPages)
	assert.Equal(t, 1, res.Page)
	require.Len(t, res.Jobs, pageSize)

	first := res.Jobs[0]
	assert.Equal(t, "20873", first.JobID)
	assert.Equal(t, "Enterprise Services - FXGO Tradedesk, Client Services Specialist - Singapore", first.Title)
	assert.Equal(t, "Singapore, Singapore", first.Location)
	assert.Contains(t, first.URL, "/JobDetail/")
	// Job 13 comes from the second upstream fetch (jobOffset=12).
	assert.Equal(t, "20826", res.Jobs[12].JobID)
}

func TestAvatureSearchKeyword(t *testing.T) {
	a := testAvatureAdapter(t, "/careers")
	res, err := a.Search(t.Context(), avatureMockSlug, SearchParams{Query: "engineer"})
	require.NoError(t, err)
	assert.Equal(t, 281, res.TotalCount)
	assert.NotEmpty(t, res.Jobs)
	assert.Equal(t, "18312", res.Jobs[0].JobID)
}

func TestAvatureSearchNoResults(t *testing.T) {
	a := testAvatureAdapter(t, "/careers")
	res, err := a.Search(t.Context(), avatureMockSlug, SearchParams{Query: "zzzznonexistentkeyword12345"})
	require.NoError(t, err)
	assert.Empty(t, res.Jobs)
	assert.Equal(t, 0, res.TotalCount)
	assert.Equal(t, 0, res.TotalPages)
}

// TestAvatureSearchNoLegend covers portals that hide the results legend:
// the total becomes the lower bound proven by the walk, +1 for the next
// page the pagination link promises.
func TestAvatureSearchNoLegend(t *testing.T) {
	a := testAvatureAdapter(t, "/nolegend")
	res, err := a.Search(t.Context(), avatureMockSlug, SearchParams{})
	require.NoError(t, err)
	// The fixture replays the same 6 jobs for any offset, so the dedupe
	// guard stops the walk after one fetch.
	require.Len(t, res.Jobs, 6)
	assert.Equal(t, 7, res.TotalCount)
	assert.Equal(t, 1, res.TotalPages)
	assert.Equal(t, "Atlanta, Georgia", res.Jobs[0].Location)
}

func TestAvatureSearchLocationUnsupported(t *testing.T) {
	a := testAvatureAdapter(t, "/careers")
	_, err := a.Search(t.Context(), avatureMockSlug, SearchParams{Location: "London"})
	require.ErrorContains(t, err, "location filtering is not supported")
}

func TestAvatureSearchFiltersUnsupported(t *testing.T) {
	a := testAvatureAdapter(t, "/careers")
	_, err := a.Search(t.Context(), avatureMockSlug, SearchParams{Filters: FilterSet{"category": {"x"}}})
	require.ErrorContains(t, err, "no filters are available")
}

func TestAvatureSearchPageOverflow(t *testing.T) {
	a := testAvatureAdapter(t, "/careers")
	_, err := a.Search(t.Context(), avatureMockSlug, SearchParams{Page: math.MaxInt})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestAvatureFilters(t *testing.T) {
	a := testAvatureAdapter(t, "/careers")
	fs, err := a.Filters(t.Context(), avatureMockSlug)
	require.NoError(t, err)
	assert.Empty(t, fs)
}

func TestAvatureDetail(t *testing.T) {
	a := testAvatureAdapter(t, "/careers")
	d, err := a.Detail(t.Context(), avatureMockSlug, "20873")
	require.NoError(t, err)
	assert.Equal(t, "20873", d.JobID)
	assert.Equal(t, "Enterprise Services - FXGO Tradedesk, Client Services Specialist - Singapore", d.Title)
	assert.Equal(t, "Bloomberg", d.Company)
	assert.Equal(t, "Singapore", d.Location)
	assert.Contains(t, d.Description, "FXGO")
	assert.Contains(t, d.URL, "/JobDetail/job/20873")
}

func TestAvatureDetailNotFound(t *testing.T) {
	a := testAvatureAdapter(t, "/careers")
	_, err := a.Detail(t.Context(), avatureMockSlug, "999999999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestAvatureUnknownSlug(t *testing.T) {
	a := testAvatureAdapter(t, "/careers")
	_, err := a.Search(t.Context(), "not-a-company", SearchParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown company")
	// A custom-domain slug off the roster is rejected too: it cannot be
	// verified as an Avature portal.
	_, err = a.Search(t.Context(), "careers.example.com/careers", SearchParams{})
	require.Error(t, err)
}

// TestAvatureSearchPaginationSweep walks every unified page over a grid of
// portal page sizes and board sizes, with and without the results legend.
// The union must be exactly jobs 1..N in order; legend portals must report
// exact totals on every page.
func TestAvatureSearchPaginationSweep(t *testing.T) {
	for _, legend := range []bool{true, false} {
		for _, up := range []int{1, 6, 12, 20, 25} {
			for _, total := range []int{0, 1, 20, 41, 60} {
				t.Run(fmt.Sprintf("legend%v_up%d_total%d", legend, up, total), func(t *testing.T) {
					srv := newAvatureMultiPageServer(t, total, up, legend)
					a := NewAvatureAdapter(&http.Client{Timeout: 5 * time.Second})
					a.baseURL = func(string) string { return srv.URL + "/careers" }

					wantPages := totalPages(total)
					var got []string
					for page := 1; page <= max(wantPages, 1); page++ {
						res, err := a.Search(t.Context(), avatureMockSlug, SearchParams{Page: page})
						require.NoError(t, err, "page %d", page)
						if legend {
							assert.Equal(t, total, res.TotalCount, "TotalCount on page %d", page)
							assert.Equal(t, wantPages, res.TotalPages, "TotalPages on page %d", page)
						} else if page >= wantPages {
							// The walk reaches the board's end on the last
							// page, so the lower bound becomes exact.
							assert.Equal(t, total, res.TotalCount, "final-page TotalCount")
						} else {
							assert.Greater(t, res.TotalPages, page, "TotalPages must signal the next page")
						}
						for _, j := range res.Jobs {
							got = append(got, j.JobID)
						}
					}
					var want []string
					for id := 1; id <= total; id++ {
						want = append(want, strconv.Itoa(id))
					}
					assert.Equal(t, want, got, "walk of all unified pages must cover every job exactly once, in order")

					beyond, err := a.Search(t.Context(), avatureMockSlug, SearchParams{Page: wantPages + 1})
					require.NoError(t, err)
					assert.Empty(t, beyond.Jobs, "page past the end must be empty")
				})
			}
		}
	}
}

// newAvatureMultiPageServer serves minimal Avature listing HTML with
// totalJobs items, a fixed page size, arbitrary jobOffset support, and an
// optional results legend — the shapes verified live on Bloomberg (legend)
// and Koch (no legend).
func newAvatureMultiPageServer(t *testing.T, totalJobs, upSize int, legend bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/careers/SearchJobs") {
			http.NotFound(w, r)
			return
		}
		off, _ := strconv.Atoi(r.URL.Query().Get("jobOffset"))
		off = max(off, 0)
		end := min(off+upSize, totalJobs)
		var b strings.Builder
		b.WriteString(`<!DOCTYPE html><html><body>`)
		if legend {
			fmt.Fprintf(&b, `<div class="list-controls__text__legend" aria-label="%d results">%d-%d of %d results</div>`,
				totalJobs, off+1, end, totalJobs)
		}
		for id := off + 1; id <= end; id++ {
			fmt.Fprintf(&b, `<article class="article article--result">
  <h3><a class="link" href="https://mock.avature.net/careers/JobDetail/Job-%d/%d">Job %d</a></h3>
  <span class="list-item-location">Testville</span>
</article>`, id, id, id)
		}
		if end < totalJobs {
			fmt.Fprintf(&b, `<a class="paginationNextLink" href="/careers/SearchJobs/?jobOffset=%d">Next</a>`, end)
		}
		b.WriteString(`</body></html>`)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)
	return srv
}
