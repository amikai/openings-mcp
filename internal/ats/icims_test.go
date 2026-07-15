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
	// "US" hits Austin and Lorton options; must return all three board jobs.
	res, err := a.Search(t.Context(), mockFixtureHost, SearchParams{Location: "US"})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 3)
	assert.Equal(t, 3, res.TotalCount)
	assert.Equal(t, "1977", res.Jobs[0].JobID)
	assert.Equal(t, "1922", res.Jobs[1].JobID)
	assert.Equal(t, "1925", res.Jobs[2].JobID)
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

func TestICIMSFiltersEmpty(t *testing.T) {
	a := testICIMSAdapter(t)
	fs, err := a.Filters(t.Context(), mockFixtureHost)
	require.NoError(t, err)
	assert.Empty(t, fs)
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
