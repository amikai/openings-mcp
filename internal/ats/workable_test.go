package ats

import (
	"encoding/json"
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

	"github.com/amikai/openings-mcp/internal/provider/workable"
)

func testWorkableAdapter(t *testing.T) *WorkableAdapter {
	t.Helper()
	mock := workable.NewMockServer()
	t.Cleanup(mock.Close)
	a, err := NewWorkableAdapter(mock.URL, &http.Client{Timeout: 5 * time.Second})
	require.NoError(t, err)
	return a
}

type workableTestJob struct {
	shortcode  string
	title      string
	department string
	location   string
	// locations are secondary places (city/region/country); when set, the
	// mock emits both location and locations[] so multi-site rendering can
	// be asserted.
	locations []map[string]string
	// body is plain text served on the detail endpoint so residual query
	// matching can see body-only keyword hits.
	body   string
	remote bool
}

// workableSearchBody is the decoded shape of one recorded search request, so
// tests can assert what the adapter sent upstream (the API is body-driven,
// unlike SmartRecruiters' query strings).
type workableSearchBody struct {
	Query      string              `json:"query"`
	Token      string              `json:"token"`
	Location   []map[string]string `json:"location"`
	Department []int               `json:"department"`
	Remote     []string            `json:"remote"`
	Workplace  []string            `json:"workplace"`
	Worktype   []string            `json:"worktype"`
}

const workableTestFacetsJSON = `{
	"locations": [
		{"country": "Greece", "countryCode": "GR", "region": "Attica", "city": "Athens", "display": "Athens, Greece"},
		{"country": "United States", "countryCode": "US", "display": "United States"}
	],
	"departments": [{"id": 1, "name": "Engineering", "filter": [1, 2], "count": 3, "parent_id": null}],
	"worktypes": ["full"],
	"remotes": [false, true],
	"workplaces": ["on_site", "remote"]
}`

// workableSearchServer serves query-specific OR candidate sets with real
// cursor pagination (tokens encode the next offset) plus static facets and
// per-job detail bodies, so adapter tests exercise residual filtering,
// description enrichment, and multi-page collection without live jobs.
func workableSearchServer(
	t *testing.T,
	byQuery map[string][]workableTestJob,
	totalOverrides map[string]int,
) (*httptest.Server, *[]workableSearchBody) {
	t.Helper()
	var bodies []workableSearchBody
	// Index every job by shortcode so detail enrichment can resolve bodies
	// regardless of which query page produced the candidate.
	byShortcode := map[string]workableTestJob{}
	for _, jobs := range byQuery {
		for _, j := range jobs {
			byShortcode[j.shortcode] = j
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/accounts/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/jobs/filters") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, workableTestFacetsJSON)
			return
		}
		var body workableSearchBody
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}
		bodies = append(bodies, body)
		jobs := slices.Clone(byQuery[body.Query])
		if slices.Equal(body.Remote, []string{"true"}) {
			jobs = slices.DeleteFunc(jobs, func(j workableTestJob) bool { return !j.remote })
		}
		offset := 0
		if body.Token != "" {
			var err error
			offset, err = strconv.Atoi(body.Token)
			if !assert.NoError(t, err) {
				return
			}
		}
		start := min(offset, len(jobs))
		end := min(start+workableUpstreamPageSize, len(jobs))
		results := make([]map[string]any, 0, end-start)
		for _, j := range jobs[start:end] {
			row := map[string]any{
				"id":         1,
				"shortcode":  j.shortcode,
				"title":      j.title,
				"remote":     j.remote,
				"location":   map[string]any{"display": j.location},
				"published":  "2026-07-10T00:00:00.000Z",
				"department": []string{j.department},
			}
			if len(j.locations) > 0 {
				locs := make([]map[string]any, 0, len(j.locations))
				for _, l := range j.locations {
					entry := map[string]any{"hidden": false}
					for k, v := range l {
						entry[k] = v
					}
					locs = append(locs, entry)
				}
				row["locations"] = locs
			}
			results = append(results, row)
		}
		total := len(jobs)
		if override, ok := totalOverrides[body.Query]; ok {
			total = override
		}
		rsp := map[string]any{"total": total, "results": results}
		if end < len(jobs) {
			rsp["nextPage"] = strconv.Itoa(end)
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(rsp))
	})
	// Detail path used by residual-query description enrichment.
	mux.HandleFunc("/api/v2/accounts/", func(w http.ResponseWriter, r *http.Request) {
		// /api/v2/accounts/{account}/jobs/{shortcode}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// api v2 accounts {account} jobs {shortcode}
		if len(parts) < 6 || parts[5] == "" {
			http.NotFound(w, r)
			return
		}
		j, ok := byShortcode[parts[5]]
		if !ok {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, "Job not found")
			return
		}
		// Nil locations encode as JSON null, which ogen rejects; always emit
		// an array so residual enrichment can decode every candidate.
		locs := j.locations
		if locs == nil {
			locs = []map[string]string{}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"id":          1,
			"shortcode":   j.shortcode,
			"title":       j.title,
			"description": "<p>" + j.body + "</p>",
			"location":    map[string]any{"display": j.location},
			"locations":   locs,
			"published":   "2026-07-10T00:00:00.000Z",
			"department":  []string{j.department},
		}))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &bodies
}

func newWorkableSearchAdapter(t *testing.T, serverURL string) *WorkableAdapter {
	t.Helper()
	a, err := NewWorkableAdapter(serverURL, &http.Client{Timeout: 5 * time.Second})
	require.NoError(t, err)
	return a
}

func TestWorkableFilters(t *testing.T) {
	a := testWorkableAdapter(t)
	fs, err := a.Filters(t.Context(), "blueground")
	require.NoError(t, err)

	assert.Equal(t, []string{"City Core", "Shared Services"}, fs["department"])
	assert.Equal(t, []string{"hybrid", "on_site", "remote"}, fs["workplace"])
	assert.Equal(t, []string{"contract", "full", "temporary"}, fs["worktype"])
}

func TestWorkableRosterMirrorsProviderRoster(t *testing.T) {
	a, err := NewWorkableAdapter("https://apply.workable.com", http.DefaultClient)
	require.NoError(t, err)
	roster := a.Roster()
	require.Len(t, roster, len(workable.Companies))
	seen := map[string]bool{}
	for _, c := range roster {
		assert.Equal(t, strings.ToLower(c.Slug), c.Slug, "slug %q must be lowercase", c.Slug)
		require.Falsef(t, seen[c.Slug], "duplicate slug %q in roster", c.Slug)
		seen[c.Slug] = true
	}
	assert.True(t, seen["blueground"], "expected blueground in roster")
}

func TestWorkableRosterBuildsRegistry(t *testing.T) {
	a, err := NewWorkableAdapter("https://apply.workable.com", http.DefaultClient)
	require.NoError(t, err)
	_, err = NewRegistry(a)
	require.NoError(t, err)
}

func TestWorkableParseCareersURL(t *testing.T) {
	a, err := NewWorkableAdapter("https://apply.workable.com", http.DefaultClient)
	require.NoError(t, err)
	tests := []struct {
		name string
		url  string
		slug string
		ok   bool
	}{
		{"roster company", "https://apply.workable.com/blueground", "blueground", true},
		{"trailing slash", "https://apply.workable.com/blueground/", "blueground", true},
		{"posting page", "https://apply.workable.com/blueground/j/B02DA69C8F/", "blueground", true},
		{"non-roster company", "https://apply.workable.com/some-unknown-co", "some-unknown-co", true},
		{"api path", "https://apply.workable.com/api/v3/accounts/blueground/jobs", "", false},
		{"host only", "https://apply.workable.com/", "", false},
		{"other ats", "https://jobs.lever.co/acme", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			require.NoError(t, err)
			slug, ok := a.ParseCareersURL(u)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.slug, slug)
		})
	}
}

// TestWorkableSearch exercises the exact path (no query): one unified page
// is two upstream cursor pages, joined in order.
func TestWorkableSearch(t *testing.T) {
	a := testWorkableAdapter(t)
	res, err := a.Search(t.Context(), "blueground", SearchParams{})
	require.NoError(t, err)

	assert.Equal(t, 29, res.TotalCount)
	assert.Equal(t, 1, res.Page)
	assert.Equal(t, 2, res.TotalPages) // ceil(29/20)
	require.Len(t, res.Jobs, 20)       // both fixture cursor pages

	first := res.Jobs[0]
	assert.Equal(t, "264C395E51", first.JobID)
	assert.Equal(t, "Senior Performance Marketing Strategist", first.Title)
	assert.Equal(t, "Athens, Greece", first.Location)
	assert.Equal(t, "2026-07-14", first.PostedAt)
	assert.Equal(t, "https://apply.workable.com/blueground/j/264C395E51/", first.URL)

	// The 11th job must come from the second cursor page.
	assert.Equal(t, "9D3D73F77D", res.Jobs[10].JobID)
}

func TestWorkableSearchExcludesLocationOnlyQueryHits(t *testing.T) {
	server, _ := workableSearchServer(t, map[string][]workableTestJob{
		"trainer": {
			{shortcode: "a", title: "Personal Trainer", location: "Los Angeles"},
			{shortcode: "b", title: "Personal Trainer", location: "Houston"},
			// Upstream OR-matched this via location text only; the unified
			// Query semantics exclude it even after detail enrichment.
			{shortcode: "c", title: "Front Desk Associate", location: "Trainer, PA", body: "front desk duties"},
		},
	}, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	res, err := a.Search(t.Context(), "acme", SearchParams{Query: "trainer"})
	require.NoError(t, err)
	assert.Equal(t, 2, res.TotalCount)
	assert.Equal(t, []string{"a", "b"}, []string{res.Jobs[0].JobID, res.Jobs[1].JobID})
}

// TestWorkableSearchKeepsBodyOnlyQueryHits guards the residual-filter path:
// Workable's upstream query matches posting bodies, but list rows omit JD
// text. Without detail enrichment, searchDump would drop body-only hits.
func TestWorkableSearchKeepsBodyOnlyQueryHits(t *testing.T) {
	server, _ := workableSearchServer(t, map[string][]workableTestJob{
		"engineer": {
			{shortcode: "title-hit", title: "Software Engineer", location: "Athens"},
			// Title has no query word; body does — must survive residual AND.
			{
				shortcode: "body-hit",
				title:     "Maintenance Technician",
				location:  "San Jose",
				body:      "Support the engineering team with lab equipment.",
			},
			// Upstream false positive: neither title nor body match.
			{
				shortcode: "noise",
				title:     "SEO Senior Associate",
				location:  "Athens",
				body:      "Drive organic search growth.",
			},
		},
	}, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	res, err := a.Search(t.Context(), "acme", SearchParams{Query: "engineer"})
	require.NoError(t, err)
	assert.Equal(t, 2, res.TotalCount)
	ids := []string{res.Jobs[0].JobID, res.Jobs[1].JobID}
	assert.Equal(t, []string{"title-hit", "body-hit"}, ids)
}

// TestWorkableSearchIncludesSecondaryLocations proves multi-site postings
// surface every distinct visible place, not only the primary location.
func TestWorkableSearchIncludesSecondaryLocations(t *testing.T) {
	server, _ := workableSearchServer(t, map[string][]workableTestJob{
		"": {{
			shortcode: "multi",
			title:     "Exam Coordinator",
			// Primary is Tokyo; London is secondary (PeopleCert-style).
			location: "Tokyo, Japan",
			locations: []map[string]string{
				{"country": "Japan", "city": "Tokyo"},
				{"country": "United Kingdom", "city": "London"},
			},
		}},
	}, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	res, err := a.Search(t.Context(), "acme", SearchParams{})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1)
	assert.Equal(t, "Tokyo, Japan; London, United Kingdom", res.Jobs[0].Location)
}

func TestWorkableJobLocationDedupesPrimaryAgainstSecondaries(t *testing.T) {
	primary := workable.NewOptLocation(workable.Location{
		Display: workable.NewOptNilString("Athens, Greece"),
		Country: workable.NewOptNilString("Greece"),
		Region:  workable.NewOptNilString("Attica"),
		City:    workable.NewOptNilString("Athens"),
	})
	secondaries := []workable.Location{
		{
			Country: workable.NewOptNilString("Greece"),
			Region:  workable.NewOptNilString("Attica"),
			City:    workable.NewOptNilString("Athens"),
			Hidden:  workable.NewOptBool(false),
		},
		{
			Country: workable.NewOptNilString("United Kingdom"),
			City:    workable.NewOptNilString("London"),
			Hidden:  workable.NewOptBool(false),
		},
		{
			Country: workable.NewOptNilString("Germany"),
			City:    workable.NewOptNilString("Stuttgart"),
			Hidden:  workable.NewOptBool(true), // hidden secondaries stay out
		},
	}
	assert.Equal(t, "Athens, Greece; London, United Kingdom", workableJobLocation(primary, secondaries))
}

func TestWorkableSearchRemoteLocationUsesRemoteFilter(t *testing.T) {
	server, bodies := workableSearchServer(t, map[string][]workableTestJob{
		"": {
			{shortcode: "remote", title: "Rider Growth Analyst", location: "Athens, Greece", remote: true},
			{shortcode: "onsite", title: "Remote Operations Manager", location: "Berlin, Germany"},
		},
	}, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	res, err := a.Search(t.Context(), "acme", SearchParams{Location: "  remote  "})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1)
	assert.Equal(t, "remote", res.Jobs[0].JobID)
	require.Len(t, *bodies, 1)
	assert.Equal(t, []string{"true"}, (*bodies)[0].Remote)
	assert.Empty(t, (*bodies)[0].Query)
}

// TestWorkableSearchLocationResolvesFacets proves fuzzy location text maps
// to the facets' structured entries, which then filter server-side.
func TestWorkableSearchLocationResolvesFacets(t *testing.T) {
	server, bodies := workableSearchServer(t, map[string][]workableTestJob{
		"": {{shortcode: "a", title: "Analyst", location: "Athens, Greece"}},
	}, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	res, err := a.Search(t.Context(), "acme", SearchParams{Location: "athens"})
	require.NoError(t, err)
	assert.Equal(t, 1, res.TotalCount)
	// The facets call plus one search.
	require.Len(t, *bodies, 1)
	require.Len(t, (*bodies)[0].Location, 1)
	assert.Equal(t, map[string]string{
		"country": "Greece", "region": "Attica", "city": "Athens",
	}, (*bodies)[0].Location[0])
}

// TestWorkableSearchLocationWithoutFacetMatchIsEmpty guards the no-match
// path: the facets list every location with published jobs, so nothing may
// be sent upstream and the result is an empty page, not an error.
func TestWorkableSearchLocationWithoutFacetMatchIsEmpty(t *testing.T) {
	server, bodies := workableSearchServer(t, map[string][]workableTestJob{
		"": {{shortcode: "a", title: "Analyst", location: "Athens, Greece"}},
	}, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	res, err := a.Search(t.Context(), "acme", SearchParams{Location: "atlantis", Page: 3})
	require.NoError(t, err)
	assert.Equal(t, 0, res.TotalCount)
	assert.Empty(t, res.Jobs)
	assert.Equal(t, 3, res.Page)
	assert.Empty(t, *bodies, "no search request when no facet location matches")
}

func TestWorkableSearchCombinesQueryAndLocationWithAND(t *testing.T) {
	server, bodies := workableSearchServer(t, map[string][]workableTestJob{
		"trainer": {
			{shortcode: "a", title: "Personal Trainer", location: "Athens, Greece"},
			// Location-only query hit, excluded locally even though the
			// structured Athens filter kept it.
			{shortcode: "b", title: "Front Desk Associate", location: "Trainer Street, Athens"},
		},
	}, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	res, err := a.Search(t.Context(), "acme", SearchParams{Query: "trainer", Location: "athens"})
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1)
	assert.Equal(t, "a", res.Jobs[0].JobID)

	require.Len(t, *bodies, 1)
	assert.Equal(t, "trainer", (*bodies)[0].Query)
	require.Len(t, (*bodies)[0].Location, 1)
	assert.Equal(t, "Athens", (*bodies)[0].Location[0]["city"])
}

func TestWorkableSearchPagesAfterResidualFiltering(t *testing.T) {
	jobs := make([]workableTestJob, 0, 105)
	for i := range 25 {
		jobs = append(jobs, workableTestJob{
			shortcode: fmt.Sprintf("match-%03d", i),
			title:     "Personal Trainer",
			location:  "Chicago",
		})
	}
	for i := range 80 {
		jobs = append(jobs, workableTestJob{
			shortcode: fmt.Sprintf("noise-%03d", i),
			title:     "Front Desk Associate",
			location:  "Trainer, PA",
		})
	}
	server, bodies := workableSearchServer(t, map[string][]workableTestJob{"trainer": jobs}, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	res, err := a.Search(t.Context(), "acme", SearchParams{Query: "trainer", Page: 2})
	require.NoError(t, err)
	assert.Equal(t, 25, res.TotalCount)
	assert.Equal(t, 2, res.TotalPages)
	assert.Len(t, res.Jobs, 5)
	// The whole 105-job candidate set is 11 cursor pages.
	assert.Len(t, *bodies, 11)
}

func TestWorkableSearchRejectsUnboundedCandidateSet(t *testing.T) {
	server, _ := workableSearchServer(t,
		map[string][]workableTestJob{
			"common": {{shortcode: "a", title: "Common Role", location: "Anywhere"}},
		},
		map[string]int{"common": maxWorkableCandidates + 1},
	)
	a := newWorkableSearchAdapter(t, server.URL)

	_, err := a.Search(t.Context(), "acme", SearchParams{Query: "common"})
	require.ErrorContains(t, err, "search is too broad")
	require.ErrorContains(t, err, strconv.Itoa(maxWorkableCandidates+1))
}

func TestWorkableSearchPagination(t *testing.T) {
	jobs := make([]workableTestJob, 0, 50)
	for i := range 50 {
		jobs = append(jobs, workableTestJob{
			shortcode: fmt.Sprintf("job-%03d", i),
			title:     "Role",
			location:  "Athens",
		})
	}
	server, bodies := workableSearchServer(t, map[string][]workableTestJob{"": jobs}, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	res, err := a.Search(t.Context(), "acme", SearchParams{Page: 2})
	require.NoError(t, err)
	assert.Equal(t, 2, res.Page)
	assert.Equal(t, 50, res.TotalCount)
	assert.Equal(t, 3, res.TotalPages)
	require.Len(t, res.Jobs, 20)
	assert.Equal(t, "job-020", res.Jobs[0].JobID)
	// Reaching unified page 2 walks cursor pages 1-4.
	assert.Len(t, *bodies, 4)

	// A page past the end walks only to the board's last cursor page and
	// returns an empty page with the exact total.
	res, err = a.Search(t.Context(), "acme", SearchParams{Page: 9})
	require.NoError(t, err)
	assert.Empty(t, res.Jobs)
	assert.Equal(t, 50, res.TotalCount)
	assert.Equal(t, 9, res.Page)

	_, err = a.Search(t.Context(), "acme", SearchParams{Page: math.MaxInt})
	require.ErrorContains(t, err, "too large")
}

func TestWorkableSearchResolvesDepartmentFilter(t *testing.T) {
	server, bodies := workableSearchServer(t, map[string][]workableTestJob{
		"": {{shortcode: "a", title: "Engineer", location: "Athens"}},
	}, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	_, err := a.Search(t.Context(), "acme", SearchParams{
		Filters: FilterSet{"department": []string{"engineering"}},
	})
	require.NoError(t, err)
	require.Len(t, *bodies, 1)
	// The facet's filter id set (self plus descendants), not the bare id.
	assert.Equal(t, []int{1, 2}, (*bodies)[0].Department)
}

func TestWorkableSearchFilterErrors(t *testing.T) {
	server, _ := workableSearchServer(t, nil, nil)
	a := newWorkableSearchAdapter(t, server.URL)

	_, err := a.Search(t.Context(), "acme", SearchParams{
		Filters: FilterSet{"office": []string{"HQ"}},
	})
	require.ErrorContains(t, err, `unknown filter key "office"; valid keys: department, workplace, worktype`)

	_, err = a.Search(t.Context(), "acme", SearchParams{
		Filters: FilterSet{"department": []string{"Nonexistent Dept"}},
	})
	require.ErrorContains(t, err, `filter value "Nonexistent Dept" not found for "department"; available: Engineering`)

	_, err = a.Search(t.Context(), "acme", SearchParams{
		Filters: FilterSet{"workplace": []string{"underwater"}},
	})
	require.ErrorContains(t, err, `filter value "underwater" not found for "workplace"; available: on_site, remote`)

	_, err = a.Search(t.Context(), "acme", SearchParams{
		Filters: FilterSet{"worktype": []string{"gig"}},
	})
	require.ErrorContains(t, err, `filter value "gig" not found for "worktype"; available: full`)
}

// TestWorkableSearchUnknownCompanyErrors guards the 404 quirk: unlike
// SmartRecruiters, Workable does distinguish an unknown account, so the
// adapter reports it instead of faking an empty board.
func TestWorkableSearchUnknownCompanyErrors(t *testing.T) {
	a := testWorkableAdapter(t)
	_, err := a.Search(t.Context(), workable.MockUnknownCompany, SearchParams{})
	require.ErrorContains(t, err, "not found")
	require.ErrorContains(t, err, workable.MockUnknownCompany)
}

func TestWorkableDetail(t *testing.T) {
	a := testWorkableAdapter(t)
	d, err := a.Detail(t.Context(), "blueground", "B02DA69C8F")
	require.NoError(t, err)

	assert.Equal(t, "B02DA69C8F", d.JobID)
	assert.Equal(t, "Senior Software Engineer, iOS", d.Title)
	assert.Equal(t, "Blueground", d.Company)
	assert.Equal(t, "Athens, Greece", d.Location)
	assert.Equal(t, "2026-07-14", d.PostedAt)
	assert.Equal(t, "https://apply.workable.com/blueground/j/B02DA69C8F/", d.URL)

	// HTML stripped, and the fixture's empty requirements/benefits fields
	// must not add empty titled blocks.
	assert.Contains(t, d.Description, "Description:")
	assert.Contains(t, d.Description, "Redefining how people live")
	assert.NotContains(t, d.Description, "<p>")
	assert.NotContains(t, d.Description, "Requirements:")
	assert.NotContains(t, d.Description, "Benefits:")
}

func TestWorkableDetailNotFound(t *testing.T) {
	a := testWorkableAdapter(t)
	_, err := a.Detail(t.Context(), "blueground", "0000000000")
	require.ErrorContains(t, err, "pass a job_id exactly as returned by the job search")
}

func TestWorkableCareersHostPatternRegistered(t *testing.T) {
	// The registry only advertises careers-URL shapes for adapters listed
	// in careersHostPatternsByAdapter; a missing entry silently degrades
	// the "unrecognized careers URL" teaching error.
	assert.Contains(t, careersHostPatternsByAdapter, "workable")
}

func TestWorkableResolvesThroughRegistry(t *testing.T) {
	a, err := NewWorkableAdapter("https://apply.workable.com", http.DefaultClient)
	require.NoError(t, err)
	r, err := NewRegistry(a)
	require.NoError(t, err)

	got, slug, err := r.Resolve("Blueground")
	require.NoError(t, err)
	assert.Equal(t, "workable", got.Name())
	assert.Equal(t, "blueground", slug)

	got, slug, err = r.Resolve("https://apply.workable.com/some-unknown-co/")
	require.NoError(t, err)
	assert.Equal(t, "workable", got.Name())
	assert.Equal(t, "some-unknown-co", slug)
}
