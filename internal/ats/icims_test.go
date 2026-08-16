package ats

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/icims"
)

func testICIMSAdapter(t *testing.T) *ICIMSAdapter {
	t.Helper()
	mock := icims.NewMockServer()
	t.Cleanup(mock.Close)
	a := NewICIMSAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return mock.URL }
	return a
}

func TestICIMSRosterBuildsRegistry(t *testing.T) {
	_, err := NewRegistry(NewICIMSAdapter(http.DefaultClient))
	require.NoError(t, err)
}

func TestICIMSRosterReturnsCompanyNames(t *testing.T) {
	a := NewICIMSAdapter(http.DefaultClient)
	roster := a.Roster()
	require.NotEmpty(t, roster)
	found := false
	for _, c := range roster {
		if c.Slug == "careers-peraton.icims.com" {
			found = true
			assert.Equal(t, "Peraton", c.Name)
		}
	}
	assert.True(t, found, "expected careers-peraton.icims.com in roster")
}

func TestICIMSParseCareersURL(t *testing.T) {
	a := NewICIMSAdapter(http.DefaultClient)
	cases := []struct {
		raw  string
		ok   bool
		slug string
	}{
		{"https://careers-peraton.icims.com/jobs/search?ss=1", true, "careers-peraton.icims.com"},
		{"https://uscareers-example.icims.com/jobs/1/x/job", true, "uscareers-example.icims.com"},
		{"https://login.icims.com/", false, ""},
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

// mockFixtureHost is a roster host used only for mock-backed tests. The
// adapter overrides baseURL to the mock server, so live DNS is never hit.
const mockFixtureHost = "careers-peraton.icims.com"

func TestICIMSSearch(t *testing.T) {
	a := testICIMSAdapter(t)
	res, err := a.Search(t.Context(), mockFixtureHost, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 3, res.TotalCount)
	assert.Equal(t, 1, res.Page)
	assert.Len(t, res.Jobs, 3)

	first := res.Jobs[0]
	assert.Equal(t, "1977", first.JobID)
	assert.Equal(t, "Senior Product Manager", first.Title)
	assert.Contains(t, first.Location, "Austin")
	assert.Equal(t, "https://careers-peraton.icims.com/jobs/1977/job/job", first.URL)
}

func TestICIMSSearchKeyword(t *testing.T) {
	a := testICIMSAdapter(t)
	res, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Query: "Product"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jobs)
}

func TestICIMSSearchLocation(t *testing.T) {
	a := testICIMSAdapter(t)
	res, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Location: "Austin"})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 2)
	assert.Equal(t, 2, res.TotalCount)
	for _, j := range res.Jobs {
		assert.Contains(t, j.Location, "Austin")
	}
	assert.Equal(t, "1977", res.Jobs[0].JobID)
	assert.Equal(t, "1922", res.Jobs[1].JobID)
}

func TestICIMSSearchLocationMultiMatch(t *testing.T) {
	a := testICIMSAdapter(t)
	// "US" hits the Austin and Lorton options; both ride one query as
	// repeated searchLocation params, returning all three board jobs.
	res, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Location: "US"})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 3)
	assert.Equal(t, 3, res.TotalCount)
	// The union keeps the board's native order.
	assert.Equal(t, "1977", res.Jobs[0].JobID)
	assert.Equal(t, "1925", res.Jobs[1].JobID)
	assert.Equal(t, "1922", res.Jobs[2].JobID)
}

func TestICIMSSearchPostedAt(t *testing.T) {
	a := testICIMSAdapter(t)
	// The "posted" keyword routes the mock to the Peraton Lorton capture,
	// whose single card carries a posted-date span.
	res, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Query: "posted"})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1)
	assert.Equal(t, "167924", res.Jobs[0].JobID)
	assert.Equal(t, "2026-06-25", res.Jobs[0].PostedAt)
}

func TestICIMSSearchPageOverflow(t *testing.T) {
	a := testICIMSAdapter(t)
	_, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Page: math.MaxInt})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestICIMSSearchNoResults(t *testing.T) {
	a := testICIMSAdapter(t)
	res, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Query: "zzzznonexistentkeyword12345"})
	require.NoError(t, err)
	assert.Empty(t, res.Jobs)
	assert.Equal(t, 0, res.TotalCount)
}

func TestICIMSFilters(t *testing.T) {
	a := testICIMSAdapter(t)
	fs, err := a.Filters(t.Context(), mockFixtureHost)
	require.NoError(t, err)
	assert.Equal(t, []string{"Sales & Communication - Sales", "Technology - Product Management"}, fs["category"])
	assert.Contains(t, fs["positionType"], "Full-Time")
	assert.Contains(t, fs["positionType"], "Part-Time")
}

// newICIMSFilterServer serves a three-job board whose searchCategory and
// searchPositionType query params filter the cards server-side, mirroring
// the verified portal semantics (OR within a param, AND across params).
func newICIMSFilterServer(t *testing.T) *httptest.Server {
	t.Helper()
	type job struct{ id, title, cat, ptype string }
	jobs := []job{
		{"1", "Backend Engineer", "100", "2049"},
		{"2", "Frontend Engineer", "100", "2050"},
		{"3", "Account Manager", "200", "2049"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		match := func(j job) bool {
			if cats := q["searchCategory"]; len(cats) > 0 && !slices.Contains(cats, j.cat) {
				return false
			}
			if types := q["searchPositionType"]; len(types) > 0 && !slices.Contains(types, j.ptype) {
				return false
			}
			return true
		}
		var b strings.Builder
		b.WriteString(`<!DOCTYPE html><html><body><form id="searchForm"><input name="searchKeyword"/>`)
		b.WriteString(`<select name="searchCategory"><option value="">(All)</option>`)
		b.WriteString(`<option value="100">Engineering</option><option value="200">Sales</option></select>`)
		b.WriteString(`<select name="searchPositionType"><option value="">(All)</option>`)
		b.WriteString(`<option value="2049">Full-Time</option><option value="2050">Part-Time</option></select>`)
		b.WriteString(`</form><ul class="iCIMS_JobsTable">`)
		for _, j := range jobs {
			if !match(j) {
				continue
			}
			fmt.Fprintf(&b, `<li class="iCIMS_JobCardItem">
  <span class="sr-only field-label">Location</span><span>US-TX-Austin</span>
  <a class="iCIMS_Anchor" href="/jobs/%s/x/job"><h3>%s</h3></a>
</li>`, j.id, j.title)
		}
		b.WriteString(`</ul></body></html>`)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestICIMSSearchFilters(t *testing.T) {
	srv := newICIMSFilterServer(t)
	a := NewICIMSAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }

	res, err := a.Search(t.Context(), mockFixtureHost, SearchParams{
		Filters: FilterSet{"category": {"Engineering"}},
	})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 2)
	assert.Equal(t, 2, res.TotalCount)

	// Labels resolve case-insensitively; keys AND together.
	res, err = a.Search(t.Context(), mockFixtureHost, SearchParams{
		Filters: FilterSet{"category": {"engineering"}, "positionType": {"Part-Time"}},
	})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1)
	assert.Equal(t, "2", res.Jobs[0].JobID)

	// Values within a key OR together.
	res, err = a.Search(t.Context(), mockFixtureHost, SearchParams{
		Filters: FilterSet{"category": {"Engineering", "Sales"}},
	})
	require.NoError(t, err)
	assert.Len(t, res.Jobs, 3)
}

func TestICIMSSearchFilterTeachingErrors(t *testing.T) {
	srv := newICIMSFilterServer(t)
	a := NewICIMSAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }

	_, err := a.Search(t.Context(), mockFixtureHost, SearchParams{
		Filters: FilterSet{"department": {"Engineering"}},
	})
	require.ErrorContains(t, err, "unknown filter key")
	assert.Contains(t, err.Error(), "category")

	_, err = a.Search(t.Context(), mockFixtureHost, SearchParams{
		Filters: FilterSet{"category": {"Nonexistent"}},
	})
	require.ErrorContains(t, err, `filter value "Nonexistent" not found`)
	assert.Contains(t, err.Error(), "available:")
}

func TestICIMSDetail(t *testing.T) {
	a := testICIMSAdapter(t)
	d, err := a.Detail(t.Context(), mockFixtureHost, "1977")
	require.NoError(t, err)
	assert.Equal(t, "1977", d.JobID)
	assert.Equal(t, "Senior Product Manager", d.Title)
	assert.Equal(t, "Peraton", d.Company)
	assert.Contains(t, d.Location, "Austin")
	assert.NotEmpty(t, d.Description)
	assert.Equal(t, "https://careers-peraton.icims.com/jobs/1977/job/job", d.URL)
}

func TestICIMSDetailNotFound(t *testing.T) {
	a := testICIMSAdapter(t)
	_, err := a.Detail(t.Context(), mockFixtureHost, "999999999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestICIMSDetailBadID(t *testing.T) {
	a := testICIMSAdapter(t)
	_, err := a.Detail(t.Context(), mockFixtureHost, "not-a-number")
	require.Error(t, err)
}

func TestICIMSUnknownSlug(t *testing.T) {
	a := testICIMSAdapter(t)
	_, err := a.Search(t.Context(), "not-a-company", SearchParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown company")
}

// TestICIMSSearchMultiPagePagination covers the review bugs for a synthetic
// 60-job tenant with upstream page size 50:
//   - unified page 2 must return jobs 21–40 (not empty from bad page-size guess)
//   - TotalCount must be 60 (not 100 = 2 * 50)
func TestICIMSSearchMultiPagePagination(t *testing.T) {
	const (
		totalJobs = 60
		upSize    = 50
	)
	srv := newICIMSMultiPageServer(t, totalJobs, upSize)
	a := NewICIMSAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(string) string { return srv.URL }

	page1, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Page: 1})
	require.NoError(t, err)
	assert.Equal(t, totalJobs, page1.TotalCount)
	assert.Equal(t, 3, page1.TotalPages) // ceil(60/20)
	require.Len(t, page1.Jobs, pageSize)
	assert.Equal(t, "1", page1.Jobs[0].JobID)
	assert.Equal(t, "20", page1.Jobs[len(page1.Jobs)-1].JobID)

	page2, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Page: 2})
	require.NoError(t, err)
	assert.Equal(t, totalJobs, page2.TotalCount)
	require.Len(t, page2.Jobs, pageSize, "page 2 must not collapse to empty from partial last-page PageSize")
	assert.Equal(t, "21", page2.Jobs[0].JobID)
	assert.Equal(t, "40", page2.Jobs[len(page2.Jobs)-1].JobID)

	page3, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Page: 3})
	require.NoError(t, err)
	assert.Equal(t, totalJobs, page3.TotalCount)
	require.Len(t, page3.Jobs, 20)
	assert.Equal(t, "41", page3.Jobs[0].JobID)
	assert.Equal(t, "60", page3.Jobs[len(page3.Jobs)-1].JobID)
}

// TestICIMSSearchPaginationSweep walks every unified page across a grid of
// tenant page sizes and board sizes: the union must be exactly jobs 1..N in
// order, with exact totals on every page. Small and misaligned upstream page
// sizes force the stitch loop to fetch several upstream pages per unified
// page — a single-stitch implementation silently drops rows here.
func TestICIMSSearchPaginationSweep(t *testing.T) {
	for _, up := range []int{1, 5, 10, 17, 20, 50} {
		for _, total := range []int{0, 1, 20, 41, 60, 101} {
			t.Run(fmt.Sprintf("up%d_total%d", up, total), func(t *testing.T) {
				srv := newICIMSMultiPageServer(t, total, up)
				a := NewICIMSAdapter(&http.Client{Timeout: 5 * time.Second})
				a.baseURL = func(string) string { return srv.URL }

				wantPages := totalPages(total)
				var got []string
				for page := 1; page <= max(wantPages, 1); page++ {
					res, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Page: page})
					require.NoError(t, err, "page %d", page)
					assert.Equal(t, total, res.TotalCount, "TotalCount on page %d", page)
					assert.Equal(t, wantPages, res.TotalPages, "TotalPages on page %d", page)
					for _, j := range res.Jobs {
						got = append(got, j.JobID)
					}
				}
				var want []string
				for id := 1; id <= total; id++ {
					want = append(want, strconv.Itoa(id))
				}
				assert.Equal(t, want, got, "walk of all unified pages must cover every job exactly once, in order")

				beyond, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Page: wantPages + 1})
				require.NoError(t, err)
				assert.Empty(t, beyond.Jobs, "page past the end must be empty")
				assert.Equal(t, total, beyond.TotalCount, "past-the-end TotalCount")
			})
		}
	}
}

// newICIMSMultiPageServer serves minimal iCIMS search HTML with totalJobs
// cards and a fixed upstream page size of upSize (pr is zero-based).
func newICIMSMultiPageServer(t *testing.T, totalJobs, upSize int) *httptest.Server {
	t.Helper()
	totalPagesUp := (totalJobs + upSize - 1) / upSize
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/jobs/search") {
			http.NotFound(w, r)
			return
		}
		pr, _ := strconv.Atoi(r.URL.Query().Get("pr"))
		if pr < 0 {
			pr = 0
		}
		start := pr * upSize
		end := min(start+upSize, totalJobs)
		var b strings.Builder
		b.WriteString(`<!DOCTYPE html><html><body>`)
		b.WriteString(`<form id="searchForm"><input name="searchKeyword"/>`)
		b.WriteString(`<select name="searchLocation"><option value=""></option>`)
		b.WriteString(`<option value="1-1-Austin">TX Austin US</option></select></form>`)
		fmt.Fprintf(&b, `<div class="iCIMS_PagingBatch">Page %d of %d</div>`, pr+1, totalPagesUp)
		b.WriteString(`<ul class="iCIMS_JobsTable">`)
		for id := start + 1; id <= end; id++ {
			fmt.Fprintf(&b, `
<li class="iCIMS_JobCardItem">
  <span class="sr-only field-label">Location</span><span>US-TX-Austin</span>
  <a class="iCIMS_Anchor" href="/jobs/%d/job-title/job"><h3>Job %d</h3></a>
</li>`, id, id)
		}
		b.WriteString(`</ul></body></html>`)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)
	return srv
}
