package ats

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

// fakePosting is one row of the fake Posting API's board, carrying only
// the fields the adapter reads.
type fakePosting struct {
	id, name                   string
	city, region, country, loc string
	deptID, deptLabel          string
}

const fakeSmartRecruitersCompany = "TestCo"

// fakeSmartRecruitersBoard builds a 250-posting board spanning three walk
// pages (100+100+50). "Rare Dept" only appears from row 200 on, so any
// test that sees it proves the walk reached the last page.
func fakeSmartRecruitersBoard() []fakePosting {
	board := make([]fakePosting, 0, 250)
	for i := range 250 {
		p := fakePosting{
			id:        fmt.Sprintf("744000%09d", i),
			name:      fmt.Sprintf("Software Engineer %d", i),
			city:      "Berlin",
			region:    "BE",
			country:   "de",
			loc:       "Berlin, BE, Germany",
			deptID:    "100",
			deptLabel: "Engineering",
		}
		if i%10 == 0 { // 25 rows
			p.name = fmt.Sprintf("Personal Trainer %d", i)
			p.city, p.region, p.country, p.loc = "Houston", "TX", "us", "Houston, TX, United States"
		}
		if i >= 200 {
			p.deptID, p.deptLabel = "999", "Rare Dept"
		}
		board = append(board, p)
	}
	return board
}

// newFakeSmartRecruitersServer simulates the Posting API semantics the
// adapter depends on: offset/limit paging, q title matching, exact
// city/region matching, lowercase-only country matching, department by id,
// and the empty-200 answer for an unknown companyIdentifier.
func newFakeSmartRecruitersServer(t *testing.T, board []fakePosting) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var requests atomic.Int64
	mux := http.NewServeMux()
	listPath := "/companies/" + fakeSmartRecruitersCompany + "/postings"

	mux.HandleFunc("GET "+listPath, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		q := r.URL.Query()
		matched := make([]fakePosting, 0, len(board))
		for _, p := range board {
			if v := q.Get("q"); v != "" && !strings.Contains(strings.ToLower(p.name), strings.ToLower(v)) {
				continue
			}
			if v := q.Get("city"); v != "" && !strings.EqualFold(p.city, v) {
				continue
			}
			if v := q.Get("region"); v != "" && !strings.EqualFold(p.region, v) {
				continue
			}
			// The live API only matches lowercase country codes.
			if v := q.Get("country"); v != "" && p.country != v {
				continue
			}
			if v := q.Get("department"); v != "" && p.deptID != v {
				continue
			}
			matched = append(matched, p)
		}
		limit := 100
		if v := q.Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			require.NoError(t, err)
			limit = min(n, 100)
		}
		offset := 0
		if v := q.Get("offset"); v != "" {
			n, err := strconv.Atoi(v)
			require.NoError(t, err)
			offset = n
		}
		start := min(offset, len(matched))
		end := min(start+limit, len(matched))
		content := make([]map[string]any, 0, end-start)
		for _, p := range matched[start:end] {
			content = append(content, map[string]any{
				"id":           p.id,
				"name":         p.name,
				"company":      map[string]any{"identifier": fakeSmartRecruitersCompany, "name": "TestCo Inc"},
				"releasedDate": time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
				"location": map[string]any{
					"city": p.city, "region": p.region, "country": p.country,
					"fullLocation": p.loc,
				},
				"department": map[string]any{"id": p.deptID, "label": p.deptLabel},
			})
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"offset": offset, "limit": limit, "totalFound": len(matched), "content": content,
		})
	})

	// Detail is served for every companyIdentifier so tests can exercise the
	// roster-name fallback with slugs other than TestCo.
	mux.HandleFunc("GET /companies/{company}/postings/{id}", func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		for _, p := range board {
			if p.id != r.PathValue("id") {
				continue
			}
			writeJSON(t, w, http.StatusOK, map[string]any{
				"id":           p.id,
				"name":         p.name,
				"company":      map[string]any{"identifier": fakeSmartRecruitersCompany, "name": "TestCo Inc"},
				"releasedDate": time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
				"location":     map[string]any{"fullLocation": p.loc},
				"postingUrl":   "https://jobs.smartrecruiters.com/TestCo/" + p.id + "-seo-slug",
				"jobAd": map[string]any{"sections": map[string]any{
					"jobDescription": map[string]any{"title": "", "text": "<p>Build &amp; ship <b>Go</b> services.</p>"},
					"qualifications": map[string]any{"title": "What we need", "text": "<ul><li>Go</li></ul>"},
				}},
			})
			return
		}
		writeJSON(t, w, http.StatusNotFound, map[string]any{
			"httpCode": 404, "code": "RESOURCE_NOT_FOUND", "message": "Resource not found",
		})
	})

	// Any other companyIdentifier answers the live API's empty-200 quirk.
	mux.HandleFunc("GET /companies/{company}/postings", func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		writeJSON(t, w, http.StatusOK, map[string]any{
			"offset": 0, "limit": 100, "totalFound": 0, "content": []any{},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &requests
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(v))
}

func testSmartRecruitersAdapter(t *testing.T) (*SmartRecruitersAdapter, *atomic.Int64) {
	t.Helper()
	srv, requests := newFakeSmartRecruitersServer(t, fakeSmartRecruitersBoard())
	a, err := NewSmartRecruitersAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	require.NoError(t, err)
	return a, requests
}

func TestSmartRecruitersRoster(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	assert.Len(t, a.Roster(), len(smartrecruiters.Companies))
}

func TestSmartRecruitersRosterBuildsRegistry(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	_, err := NewRegistry(a)
	require.NoError(t, err)
}

func TestSmartRecruitersSearchPagesServerSide(t *testing.T) {
	a, requests := testSmartRecruitersAdapter(t)
	p1, err := a.Search(t.Context(), fakeSmartRecruitersCompany, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 250, p1.TotalCount)
	assert.Equal(t, totalPages(250), p1.TotalPages)
	require.Len(t, p1.Jobs, PageSize)
	for _, j := range p1.Jobs {
		assert.NotEmptyf(t, j.JobID, "summary with empty field: %+v", j)
		assert.NotEmptyf(t, j.Title, "summary with empty field: %+v", j)
		assert.Truef(t, strings.HasPrefix(j.URL, "https://jobs.smartrecruiters.com/TestCo/"), "unexpected URL %q", j.URL)
		assert.Truef(t, strings.HasPrefix(j.PostedAt, "2026-"), "PostedAt should be an ISO date, got %q", j.PostedAt)
	}

	p2, err := a.Search(t.Context(), fakeSmartRecruitersCompany, SearchParams{Page: 2})
	require.NoError(t, err)
	require.Len(t, p2.Jobs, PageSize)
	assert.NotEqual(t, p1.Jobs[0].JobID, p2.Jobs[0].JobID, "page 2 should differ from page 1")

	assert.EqualValues(t, 2, requests.Load(), "an unfiltered search is one upstream request per page")
}

func TestSmartRecruitersSearchQueryRunsUpstream(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	res, err := a.Search(t.Context(), fakeSmartRecruitersCompany, SearchParams{Query: "trainer"})
	require.NoError(t, err)
	assert.Equal(t, 25, res.TotalCount)
	for _, j := range res.Jobs {
		assert.Contains(t, j.Title, "Personal Trainer")
	}
}

// TestSmartRecruitersSearchLocation covers the probe ladder: a city hits
// on the first probe, a region code only on the second, and an uppercase
// country code only after lowercasing on the third.
func TestSmartRecruitersSearchLocation(t *testing.T) {
	for _, tc := range []struct {
		location string
		want     int
	}{
		{"Houston", 25},
		{"TX", 25},
		{"US", 25},
		{"Germany", 0}, // country name, not a code: no probe matches
	} {
		t.Run(tc.location, func(t *testing.T) {
			a, _ := testSmartRecruitersAdapter(t)
			res, err := a.Search(t.Context(), fakeSmartRecruitersCompany, SearchParams{Location: tc.location})
			if tc.want == 0 {
				require.ErrorContains(t, err, "no postings matching location")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, res.TotalCount)
		})
	}
}

func TestSmartRecruitersSearchLocationRemote(t *testing.T) {
	a, requests := testSmartRecruitersAdapter(t)
	_, err := a.Search(t.Context(), fakeSmartRecruitersCompany, SearchParams{Location: "remote"})
	require.ErrorContains(t, err, "remote")
	assert.Zero(t, requests.Load(), "remote must fail before any upstream call")
}

func TestSmartRecruitersFiltersWalkWholeBoard(t *testing.T) {
	a, requests := testSmartRecruitersAdapter(t)
	fs, err := a.Filters(t.Context(), fakeSmartRecruitersCompany)
	require.NoError(t, err)
	// "Rare Dept" only exists on the last walk page.
	assert.Equal(t, FilterSet{"department": {"Engineering", "Rare Dept"}}, fs)
	assert.EqualValues(t, 3, requests.Load(), "a 250-row board walks in 3 pages")
}

func TestSmartRecruitersDepartmentFilter(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	res, err := a.Search(t.Context(), fakeSmartRecruitersCompany, SearchParams{
		Filters: FilterSet{"department": {"rare dept"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 50, res.TotalCount, "department labels resolve case-insensitively to ids")

	_, err = a.Search(t.Context(), fakeSmartRecruitersCompany, SearchParams{
		Filters: FilterSet{"department": {"No Such Dept"}},
	})
	require.ErrorContains(t, err, "Engineering", "unknown label should teach the valid ones")

	_, err = a.Search(t.Context(), fakeSmartRecruitersCompany, SearchParams{
		Filters: FilterSet{"office": {"Berlin"}},
	})
	require.ErrorContains(t, err, `unknown filter key "office"`)

	_, err = a.Search(t.Context(), fakeSmartRecruitersCompany, SearchParams{
		Filters: FilterSet{"department": {"Engineering", "Rare Dept"}},
	})
	require.ErrorContains(t, err, "exactly one value")
}

func TestSmartRecruitersUnknownCompanyReturnsEmpty(t *testing.T) {
	// The live API answers an unknown companyIdentifier with an empty 200,
	// indistinguishable from a real company with no postings; the adapter
	// passes that through rather than inventing an error.
	a, _ := testSmartRecruitersAdapter(t)
	res, err := a.Search(t.Context(), "no-such-company", SearchParams{})
	require.NoError(t, err)
	assert.Zero(t, res.TotalCount)
	assert.Empty(t, res.Jobs)
}

func TestSmartRecruitersDetail(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	d, err := a.Detail(t.Context(), fakeSmartRecruitersCompany, "744000000000000")
	require.NoError(t, err)
	assert.Equal(t, "Personal Trainer 0", d.Title)
	assert.Equal(t, "TestCo Inc", d.Company, "non-roster slug falls back to the upstream company name")
	assert.Equal(t, "https://jobs.smartrecruiters.com/TestCo/744000000000000-seo-slug", d.URL)
	assert.Equal(t, "2026-07-01", d.PostedAt)
	assert.Contains(t, d.Description, "Job Description:", "untitled section uses its fallback heading")
	assert.Contains(t, d.Description, "Build & ship *Go* services.", "HTML converts to plain text (bold becomes *…*)")
	assert.Contains(t, d.Description, "What we need:", "titled section keeps its own heading")
	assert.NotContains(t, d.Description, "<p>")
}

func TestSmartRecruitersDetailNotFound(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	_, err := a.Detail(t.Context(), fakeSmartRecruitersCompany, "000000000000")
	require.ErrorContains(t, err, "not found")
}

// TestSmartRecruitersDetailUsesRosterName pins the Company precedence: a
// roster identifier displays its curated name even when the upstream
// response says otherwise.
func TestSmartRecruitersDetailUsesRosterName(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	d, err := a.Detail(t.Context(), "equinox", "744000000000000")
	require.NoError(t, err)
	assert.Equal(t, "Equinox", d.Company)
}

func TestSmartRecruitersParseCareersURL(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	for _, tc := range []struct {
		in   string
		slug string
		ok   bool
	}{
		{"https://jobs.smartrecruiters.com/equinox", "Equinox", true}, // folds to roster casing
		{"https://jobs.smartrecruiters.com/SomeUnknownCo/123", "SomeUnknownCo", true},
		{"https://jobs.smartrecruiters.com/sr-jobs/search?q=go", "", false},
		{"https://jobs.smartrecruiters.com/", "", false},
		{"https://jobs.lever.co/acme", "", false},
	} {
		u, err := url.Parse(tc.in)
		require.NoError(t, err)
		slug, ok := a.ParseCareersURL(u)
		assert.Equalf(t, tc.ok, ok, "ParseCareersURL(%q)", tc.in)
		assert.Equalf(t, tc.slug, slug, "ParseCareersURL(%q)", tc.in)
	}
}
