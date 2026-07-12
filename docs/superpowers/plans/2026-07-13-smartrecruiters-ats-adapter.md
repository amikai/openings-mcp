# SmartRecruiters ATS Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `internal/ats.Adapter` for SmartRecruiters and register it with the MCP server, so the rostered companies (and any jobs.smartrecruiters.com careers URL) join the unified `search_jobs_by_company` / `get_filters_by_company` / `get_job_detail_by_company` tools.

**Architecture:** Server-side-search adapter (like Workday) over the already-landed provider package `internal/provider/smartrecruiters` (ogen-generated client, fixtures, mock server, 52-company roster). The unified `Location` folds into the API's `q` param; `department` filter labels resolve to ids via one `listDepartments` call at search time; `location_type` maps to the static locationType enum. Spec: `docs/superpowers/specs/2026-07-13-smartrecruiters-ats-adapter-design.md`.

**Tech Stack:** Go, ogen-generated client, testify, httptest, html2text.

## Global Constraints

- Never hand-edit `oas_*_gen.go`; the provider package is not touched by this plan.
- All upstream errors wrap with a `smartrecruiters:` prefix and the slug.
- Filter-resolution failures are teaching errors naming the valid alternatives.
- Slugs are lowercased `CompanyIdentifier`s; the API accepts them case-insensitively.
- Commit messages follow Conventional Commits (see CLAUDE.md).
- Run all commands from the repo root.

## Fixture facts (used by test assertions)

The adapter tests replay `internal/provider/smartrecruiters/mocksrv.go`, which serves captured Equinox fixtures on lowercase paths (`/v1/companies/equinox/...`):

- `postings_rsp.json`: `totalFound` 662, 5 items. First item: id `744000137225639`, name `Female Locker Room Associate, Houston`, fullLocation `Houston, TX, United States`, releasedDate `2026-07-10T23:49:03.072Z`.
- `postings_filtered_rsp.json`: served when the query has exactly `q=trainer`; `totalFound` 138, 3 items.
- `departments_rsp.json`: 58 departments, all labeled; exactly one archived (`id` 1005166, label `Club - Pilot PT`). Ids arrive as unquoted integers. Contains `Club - Staff` (660916) and `Club - Sales` (660882).
- `posting_detail_rsp.json`: served for id `744000137225639`; postingUrl `https://jobs.smartrecruiters.com/Equinox/744000137225639-female-locker-room-associate-houston`, company.name `Equinox`, four jobAd sections titled Company Description / Job Description / Qualifications / Additional Information.
- id `000000000000` → HTTP 404 with `posting_not_found_rsp.json`.
- `MockUnknownCompany` (`this-company-does-not-exist-xyz`) → HTTP 200 with `totalFound` 0.

---

### Task 1: Adapter skeleton — Name, Roster, ParseCareersURL

**Files:**
- Create: `internal/ats/smartrecruiters.go`
- Create: `internal/ats/smartrecruiters_test.go`

**Interfaces:**
- Consumes: `smartrecruiters.Companies`, `smartrecruiters.CompaniesByIdentifier` (`internal/provider/smartrecruiters/companies.go`); `firstPathSegment` (`internal/ats/careersurl.go`); `CompanyInfo`, `Adapter` (`internal/ats/ats.go`).
- Produces: `NewSmartRecruitersAdapter(baseURL string, hc *http.Client) (*SmartRecruitersAdapter, error)`; `resolveSmartRecruitersCompany(slug string) (identifier, name string)`. Search/Filters/Detail are stubs replaced in Tasks 2–4.

- [ ] **Step 1: Write the failing tests**

Create `internal/ats/smartrecruiters_test.go`:

```go
package ats

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

func TestSmartRecruitersRosterMirrorsProviderRoster(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	roster := a.Roster()
	require.Len(t, roster, len(smartrecruiters.Companies))
	seen := map[string]bool{}
	for _, c := range roster {
		assert.Equal(t, strings.ToLower(c.Slug), c.Slug, "slug %q must be lowercase", c.Slug)
		require.Falsef(t, seen[c.Slug], "duplicate slug %q in roster", c.Slug)
		seen[c.Slug] = true
	}
	assert.True(t, seen["equinox"], "expected equinox in roster")
}

func TestSmartRecruitersRosterBuildsRegistry(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	_, err = NewRegistry(a)
	require.NoError(t, err)
}

func TestSmartRecruitersParseCareersURL(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	tests := []struct {
		name string
		url  string
		slug string
		ok   bool
	}{
		{"roster company", "https://jobs.smartrecruiters.com/Equinox", "equinox", true},
		{"posting page", "https://jobs.smartrecruiters.com/Equinox/744000137225639-female-locker-room-associate-houston", "equinox", true},
		{"non-roster company", "https://jobs.smartrecruiters.com/SomeUnknownCo", "someunknownco", true},
		{"host only", "https://jobs.smartrecruiters.com/", "", false},
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ats/ -run TestSmartRecruiters -v`
Expected: FAIL to compile — `undefined: NewSmartRecruitersAdapter`.

- [ ] **Step 3: Write the skeleton implementation**

Create `internal/ats/smartrecruiters.go`:

```go
package ats

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

var _ Adapter = (*SmartRecruitersAdapter)(nil)

// SmartRecruitersAdapter serves SmartRecruiters-hosted companies via the
// public Posting API. Search runs server-side: the unified Location folds
// into the q param (which full-text matches titles and location text), and
// department filter labels resolve to ids via one departments call when
// set — the stateless price, like Workday's facet probe.
type SmartRecruitersAdapter struct {
	client *smartrecruiters.Client
}

func NewSmartRecruitersAdapter(baseURL string, hc *http.Client) (*SmartRecruitersAdapter, error) {
	c, err := smartrecruiters.NewClient(baseURL, smartrecruiters.WithClient(hc))
	if err != nil {
		return nil, err
	}
	return &SmartRecruitersAdapter{client: c}, nil
}

func (a *SmartRecruitersAdapter) Name() string { return "smartrecruiters" }

func (a *SmartRecruitersAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(smartrecruiters.Companies))
	for _, c := range smartrecruiters.Companies {
		infos = append(infos, CompanyInfo{Slug: strings.ToLower(c.CompanyIdentifier), Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes jobs.smartrecruiters.com career-site URLs; the
// first path segment is the companyIdentifier, which alone addresses a
// company (the API accepts it case-insensitively), so non-roster companies
// need no special slug form. An unknown identifier cannot be validated —
// the list endpoint answers HTTP 200 with zero results — so a typo'd URL
// degrades to an empty search, mirroring the raw API.
func (a *SmartRecruitersAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if strings.ToLower(u.Hostname()) != "jobs.smartrecruiters.com" {
		return "", false
	}
	id := firstPathSegment(u)
	if id == "" {
		return "", false
	}
	return strings.ToLower(id), true
}

// resolveSmartRecruitersCompany maps a slug to the roster's
// canonically-cased identifier (used in derived public URLs) and display
// name. Non-roster slugs from ParseCareersURL pass through as both.
func resolveSmartRecruitersCompany(slug string) (identifier, name string) {
	if c, ok := smartrecruiters.CompaniesByIdentifier[slug]; ok {
		return c.CompanyIdentifier, c.Name
	}
	return slug, slug
}

func (a *SmartRecruitersAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	return nil, errors.New("smartrecruiters: Search not implemented yet")
}

func (a *SmartRecruitersAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	return nil, errors.New("smartrecruiters: Filters not implemented yet")
}

func (a *SmartRecruitersAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	return nil, errors.New("smartrecruiters: Detail not implemented yet")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ats/ -run TestSmartRecruiters -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/ats/smartrecruiters.go internal/ats/smartrecruiters_test.go
git commit -m "feat(ats): add SmartRecruiters adapter skeleton with roster and careers-URL parsing"
```

---

### Task 2: Filters() and the departments helper

**Files:**
- Modify: `internal/ats/smartrecruiters.go` (replace the Filters stub; add helpers)
- Modify: `internal/ats/smartrecruiters_test.go` (add test harness + Filters tests)

**Interfaces:**
- Consumes: `smartrecruiters.NewMockServer()` (`internal/provider/smartrecruiters/mocksrv.go`); generated `ListDepartments`, `OptDepartmentId`; `toFilterSet` is NOT used (labels are built directly).
- Produces: `(a *SmartRecruitersAdapter) departments(ctx, slug) ([]smartRecruitersDepartment, error)` where `smartRecruitersDepartment` is `struct{ id, label string }` — Task 3's department filter resolution reuses it. Also the test harness `testSmartRecruitersAdapter(t) (*SmartRecruitersAdapter, *[]string)` and `lastQueryParams(t, urls) url.Values`, reused by Tasks 3–4.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ats/smartrecruiters_test.go` (extend the import block with `io`, `net/http/httptest`, `time`):

```go
// recordingQueryProxy forwards every request to inner and records each
// request's path+query, so tests can assert what the adapter sent
// upstream (the workday tests' recordingProxy records POST bodies; the
// SmartRecruiters API is GET-only, so the URL is the whole request).
func recordingQueryProxy(t *testing.T, inner string) (*httptest.Server, *[]string) {
	t.Helper()
	var urls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urls = append(urls, r.URL.String())
		req, err := http.NewRequestWithContext(r.Context(), r.Method, inner+r.URL.String(), nil)
		if !assert.NoError(t, err, "proxy") {
			return
		}
		rsp, err := http.DefaultClient.Do(req)
		if !assert.NoError(t, err, "proxy") {
			return
		}
		defer rsp.Body.Close()
		w.Header().Set("Content-Type", rsp.Header.Get("Content-Type"))
		w.WriteHeader(rsp.StatusCode)
		io.Copy(w, rsp.Body)
	}))
	t.Cleanup(srv.Close)
	return srv, &urls
}

func testSmartRecruitersAdapter(t *testing.T) (*SmartRecruitersAdapter, *[]string) {
	t.Helper()
	mock := smartrecruiters.NewMockServer()
	t.Cleanup(mock.Close)
	proxy, urls := recordingQueryProxy(t, mock.URL)
	a, err := NewSmartRecruitersAdapter(proxy.URL, &http.Client{Timeout: 5 * time.Second})
	require.NoError(t, err)
	return a, urls
}

// lastQueryParams parses the query string of the most recent upstream call.
func lastQueryParams(t *testing.T, urls []string) url.Values {
	t.Helper()
	require.NotEmpty(t, urls)
	u, err := url.Parse(urls[len(urls)-1])
	require.NoError(t, err)
	return u.Query()
}

func TestSmartRecruitersFilters(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	fs, err := a.Filters(t.Context(), "equinox")
	require.NoError(t, err)

	assert.Equal(t, []string{"Hybrid", "Onsite", "Remote"}, fs["location_type"])

	deps := fs["department"]
	// 58 departments in the fixture, exactly one archived.
	assert.Len(t, deps, 57)
	assert.Contains(t, deps, "Club - Staff")
	assert.Contains(t, deps, "Club - Sales")
	assert.NotContains(t, deps, "Club - Pilot PT", "archived departments must be excluded")
	assert.True(t, slices.IsSorted(deps), "department labels must be sorted")
}
```

Also add `"slices"` to the test file's imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ats/ -run TestSmartRecruitersFilters -v`
Expected: FAIL — `smartrecruiters: Filters not implemented yet`.

- [ ] **Step 3: Implement Filters and the departments helper**

In `internal/ats/smartrecruiters.go`, add `"fmt"`, `"slices"`, `"strconv"` to imports, remove `"errors"` only if no stub remains (Search/Detail stubs still use it — keep it), and replace the Filters stub:

```go
// smartRecruitersDepartment is one non-archived, labeled department: the
// id the API's department query param takes and the display label
// Filters() reports.
type smartRecruitersDepartment struct {
	id    string
	label string
}

// departments fetches the company's departments, dropping archived and
// unlabeled entries. DepartmentId is a string-or-int sum (the API returns
// both); ids normalize to their decimal string form either way.
func (a *SmartRecruitersAdapter) departments(ctx context.Context, slug string) ([]smartRecruitersDepartment, error) {
	rsp, err := a.client.ListDepartments(ctx, smartrecruiters.ListDepartmentsParams{CompanyIdentifier: slug})
	if err != nil {
		return nil, fmt.Errorf("smartrecruiters: list departments for %q: %w", slug, err)
	}
	deps := make([]smartRecruitersDepartment, 0, len(rsp.Content))
	for _, d := range rsp.Content {
		if d.Archived.Or(false) || d.Label.Value == "" {
			continue
		}
		id, ok := smartRecruitersDepartmentID(d.ID)
		if !ok {
			continue
		}
		deps = append(deps, smartRecruitersDepartment{id: id, label: d.Label.Value})
	}
	return deps, nil
}

func smartRecruitersDepartmentID(opt smartrecruiters.OptDepartmentId) (string, bool) {
	v, ok := opt.Get()
	if !ok {
		return "", false
	}
	if s, ok := v.GetString(); ok {
		return s, s != ""
	}
	if n, ok := v.GetInt(); ok {
		return strconv.Itoa(n), true
	}
	return "", false
}

func (a *SmartRecruitersAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	deps, err := a.departments(ctx, slug)
	if err != nil {
		return nil, err
	}
	// location_type is a static API enum, not tenant data.
	fs := FilterSet{"location_type": []string{"Hybrid", "Onsite", "Remote"}}
	seen := make(map[string]bool, len(deps))
	labels := make([]string, 0, len(deps))
	for _, d := range deps {
		if seen[d.label] {
			continue
		}
		seen[d.label] = true
		labels = append(labels, d.label)
	}
	if len(labels) > 0 {
		slices.Sort(labels)
		fs["department"] = labels
	}
	return fs, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ats/ -run TestSmartRecruiters -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/ats/smartrecruiters.go internal/ats/smartrecruiters_test.go
git commit -m "feat(ats): implement SmartRecruiters Filters via departments endpoint"
```

---

### Task 3: Search — q composition, paging, filter resolution, summary mapping

**Files:**
- Modify: `internal/ats/smartrecruiters.go` (replace the Search stub; add helpers)
- Modify: `internal/ats/smartrecruiters_test.go` (add Search tests)

**Interfaces:**
- Consumes: Task 2's `departments()`; `clampPage`, `totalPages`, `isoDate`, `PageSize` (`internal/ats/ats.go`); `errUnknownFilterKey` (`internal/ats/filter.go`); generated `ListPostings`, `ListPostingsParams`, `ListPostingsLocationTypeItem{REMOTE,HYBRID,ONSITE}`, `OptDateTime`.
- Produces: working `Search`; `smartRecruitersPostedAt(smartrecruiters.OptDateTime) string` and `smartRecruitersPostingURL(identifier, id string) string`, reused by Task 4's Detail.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ats/smartrecruiters_test.go` (add `"math"` to imports):

```go
func TestSmartRecruitersSearch(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	res, err := a.Search(t.Context(), "equinox", SearchParams{})
	require.NoError(t, err)

	assert.Equal(t, 662, res.TotalCount)
	assert.Equal(t, 1, res.Page)
	assert.Equal(t, 34, res.TotalPages) // ceil(662/20)
	require.NotEmpty(t, res.Jobs)

	first := res.Jobs[0]
	assert.Equal(t, "744000137225639", first.JobID)
	assert.Equal(t, "Female Locker Room Associate, Houston", first.Title)
	assert.Equal(t, "Houston, TX, United States", first.Location)
	assert.Equal(t, "2026-07-10", first.PostedAt)
	// Roster casing in the derived public URL; slug-less posting URLs
	// resolve fine on jobs.smartrecruiters.com.
	assert.Equal(t, "https://jobs.smartrecruiters.com/Equinox/744000137225639", first.URL)
}

func TestSmartRecruitersSearchQueryReachesUpstream(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	// The mock serves the filtered fixture only for exactly q=trainer,
	// proving the query passes through server-side.
	res, err := a.Search(t.Context(), "equinox", SearchParams{Query: "trainer"})
	require.NoError(t, err)
	assert.Equal(t, 138, res.TotalCount)
	assert.Len(t, res.Jobs, 3)
}

func TestSmartRecruitersSearchFoldsLocationIntoQ(t *testing.T) {
	a, urls := testSmartRecruitersAdapter(t)
	_, err := a.Search(t.Context(), "equinox", SearchParams{Query: "trainer", Location: "Houston"})
	require.NoError(t, err)
	assert.Equal(t, "trainer Houston", lastQueryParams(t, *urls).Get("q"))

	_, err = a.Search(t.Context(), "equinox", SearchParams{Location: "  Houston  "})
	require.NoError(t, err)
	assert.Equal(t, "Houston", lastQueryParams(t, *urls).Get("q"))
}

func TestSmartRecruitersSearchPagination(t *testing.T) {
	a, urls := testSmartRecruitersAdapter(t)
	res, err := a.Search(t.Context(), "equinox", SearchParams{Page: 3})
	require.NoError(t, err)
	assert.Equal(t, 3, res.Page)
	q := lastQueryParams(t, *urls)
	assert.Equal(t, "20", q.Get("limit"))
	assert.Equal(t, "40", q.Get("offset"))

	_, err = a.Search(t.Context(), "equinox", SearchParams{Page: math.MaxInt})
	require.ErrorContains(t, err, "too large")
}

func TestSmartRecruitersSearchResolvesDepartmentFilter(t *testing.T) {
	a, urls := testSmartRecruitersAdapter(t)
	_, err := a.Search(t.Context(), "equinox", SearchParams{
		Filters: FilterSet{"department": []string{"Club - Staff", "club - sales"}},
	})
	require.NoError(t, err)
	// Two upstream calls: the departments probe, then the search.
	require.Len(t, *urls, 2)
	// Comma-joined ids OR together (verified live against Equinox).
	assert.Equal(t, "660916,660882", lastQueryParams(t, *urls).Get("department"))
}

func TestSmartRecruitersSearchLocationTypeFilter(t *testing.T) {
	a, urls := testSmartRecruitersAdapter(t)
	_, err := a.Search(t.Context(), "equinox", SearchParams{
		Filters: FilterSet{"location_type": []string{"Remote", "hybrid"}},
	})
	require.NoError(t, err)
	// No departments probe for location_type alone.
	require.Len(t, *urls, 1)
	assert.Equal(t, []string{"REMOTE", "HYBRID"}, lastQueryParams(t, *urls)["locationType"])
}

func TestSmartRecruitersSearchFilterErrors(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)

	_, err := a.Search(t.Context(), "equinox", SearchParams{
		Filters: FilterSet{"office": []string{"HQ"}},
	})
	require.ErrorContains(t, err, `unknown filter key "office"; valid keys: department, location_type`)

	_, err = a.Search(t.Context(), "equinox", SearchParams{
		Filters: FilterSet{"department": []string{"Nonexistent Dept"}},
	})
	require.ErrorContains(t, err, `filter value "Nonexistent Dept" not found for "department"`)
	require.ErrorContains(t, err, "…", "long label lists must truncate")

	_, err = a.Search(t.Context(), "equinox", SearchParams{
		Filters: FilterSet{"location_type": []string{"underwater"}},
	})
	require.ErrorContains(t, err, `filter value "underwater" not found for "location_type"; available: Hybrid, Onsite, Remote`)
}

func TestSmartRecruitersSearchUnknownCompanyIsEmptyNotError(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	res, err := a.Search(t.Context(), smartrecruiters.MockUnknownCompany, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.TotalCount)
	assert.Empty(t, res.Jobs)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ats/ -run TestSmartRecruitersSearch -v`
Expected: FAIL — every test errors with `smartrecruiters: Search not implemented yet`.

- [ ] **Step 3: Implement Search**

In `internal/ats/smartrecruiters.go`, add `"math"` to imports (keep `"errors"` — the Detail stub still uses it until Task 4) and replace the Search stub:

```go
func (a *SmartRecruitersAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	page := clampPage(p.Page)
	pageIndex := page - 1
	if pageIndex > math.MaxInt/PageSize {
		return nil, fmt.Errorf("smartrecruiters: page %d is too large; retry with a smaller page", page)
	}
	params := smartrecruiters.ListPostingsParams{
		CompanyIdentifier: slug,
		Limit:             smartrecruiters.NewOptInt(PageSize),
		Offset:            smartrecruiters.NewOptInt(pageIndex * PageSize),
	}
	// q full-text matches titles and location text upstream, so the
	// unified Location folds into it rather than guessing among the
	// exact-match country/region/city params.
	if q := strings.TrimSpace(strings.TrimSpace(p.Query) + " " + strings.TrimSpace(p.Location)); q != "" {
		params.Q = smartrecruiters.NewOptString(q)
	}
	if err := a.applyFilters(ctx, slug, p.Filters, &params); err != nil {
		return nil, err
	}
	rsp, err := a.client.ListPostings(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("smartrecruiters: search %q: %w", slug, err)
	}
	identifier, _ := resolveSmartRecruitersCompany(slug)
	jobs := make([]JobSummary, 0, len(rsp.Content))
	for _, it := range rsp.Content {
		id := it.ID.Value
		if id == "" {
			// A posting with no id can't be detailed; skip rather than
			// hand out an unusable job_id.
			continue
		}
		jobs = append(jobs, JobSummary{
			JobID:    id,
			Title:    it.Name.Value,
			Location: it.Location.Value.FullLocation.Value,
			PostedAt: smartRecruitersPostedAt(it.ReleasedDate),
			URL:      smartRecruitersPostingURL(identifier, id),
		})
	}
	return &SearchResult{
		Jobs:       jobs,
		TotalCount: rsp.TotalFound,
		Page:       page,
		TotalPages: totalPages(rsp.TotalFound),
	}, nil
}

// smartRecruitersLocationTypes maps the location_type filter's display
// values to the API's locationType enum.
var smartRecruitersLocationTypes = map[string]smartrecruiters.ListPostingsLocationTypeItem{
	"remote": smartrecruiters.ListPostingsLocationTypeItemREMOTE,
	"hybrid": smartrecruiters.ListPostingsLocationTypeItemHYBRID,
	"onsite": smartrecruiters.ListPostingsLocationTypeItemONSITE,
}

// applyFilters maps unified filters onto the list endpoint's query params,
// failing with teaching errors that name the valid alternatives.
func (a *SmartRecruitersAdapter) applyFilters(ctx context.Context, slug string, filters FilterSet, params *smartrecruiters.ListPostingsParams) error {
	for key, values := range filters {
		switch key {
		case "department":
			ids, err := a.resolveDepartments(ctx, slug, values)
			if err != nil {
				return err
			}
			// Comma-joined ids OR together (verified live against Equinox:
			// 129 + 23 postings filter to 152).
			params.Department = smartrecruiters.NewOptString(strings.Join(ids, ","))
		case "location_type":
			for _, v := range values {
				lt, ok := smartRecruitersLocationTypes[strings.ToLower(strings.TrimSpace(v))]
				if !ok {
					return fmt.Errorf("filter value %q not found for %q; available: Hybrid, Onsite, Remote", v, key)
				}
				params.LocationType = append(params.LocationType, lt)
			}
		default:
			return errUnknownFilterKey(key, map[string]bool{"department": true, "location_type": true})
		}
	}
	return nil
}

// resolveDepartments maps department display labels to ids via one
// departments call, matching labels case-insensitively.
func (a *SmartRecruitersAdapter) resolveDepartments(ctx context.Context, slug string, values []string) ([]string, error) {
	deps, err := a.departments(ctx, slug)
	if err != nil {
		return nil, err
	}
	byLabel := make(map[string]string, len(deps))
	labels := make([]string, 0, len(deps))
	for _, d := range deps {
		lower := strings.ToLower(d.label)
		if _, ok := byLabel[lower]; !ok {
			byLabel[lower] = d.id
			labels = append(labels, d.label)
		}
	}
	ids := make([]string, 0, len(values))
	for _, v := range values {
		id, ok := byLabel[strings.ToLower(strings.TrimSpace(v))]
		if !ok {
			slices.Sort(labels)
			const maxListed = 20
			listed := labels
			suffix := ""
			if len(listed) > maxListed {
				listed = listed[:maxListed]
				suffix = ", …"
			}
			return nil, fmt.Errorf("filter value %q not found for %q; available: %s%s", v, "department", strings.Join(listed, ", "), suffix)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// smartRecruitersPostedAt guards a present-but-missing releasedDate:
// OptDateTime's zero Value would otherwise format as a fake date.
func smartRecruitersPostedAt(t smartrecruiters.OptDateTime) string {
	v, ok := t.Get()
	if !ok {
		return ""
	}
	return isoDate(v)
}

// smartRecruitersPostingURL derives the public posting page. List items
// carry no postingUrl; slug-less URLs (no title suffix) resolve fine on
// jobs.smartrecruiters.com.
func smartRecruitersPostingURL(identifier, id string) string {
	return "https://jobs.smartrecruiters.com/" + url.PathEscape(identifier) + "/" + url.PathEscape(id)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ats/ -run TestSmartRecruiters -v`
Expected: PASS (all TestSmartRecruiters* so far).

- [ ] **Step 5: Commit**

```bash
git add internal/ats/smartrecruiters.go internal/ats/smartrecruiters_test.go
git commit -m "feat(ats): implement SmartRecruiters server-side search with department and location_type filters"
```

---

### Task 4: Detail

**Files:**
- Modify: `internal/ats/smartrecruiters.go` (replace the Detail stub; add description helper)
- Modify: `internal/ats/smartrecruiters_test.go` (add Detail tests)

**Interfaces:**
- Consumes: Task 3's `smartRecruitersPostedAt`; generated `GetPosting`, `GetPostingParams`, `Posting`, `PostingErrorResponse`, `OptJobAdSections`, `OptJobAdSection`; `html2text` (already a module dependency); `cmp.Or`.
- Produces: working `Detail`; the adapter is now feature-complete.

- [ ] **Step 1: Write the failing tests**

Add to `internal/ats/smartrecruiters_test.go`:

```go
func TestSmartRecruitersDetail(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	d, err := a.Detail(t.Context(), "equinox", "744000137225639")
	require.NoError(t, err)

	assert.Equal(t, "744000137225639", d.JobID)
	assert.Equal(t, "Female Locker Room Associate, Houston", d.Title)
	assert.Equal(t, "Equinox", d.Company)
	assert.Equal(t, "Houston, TX, United States", d.Location)
	assert.Equal(t, "2026-07-10", d.PostedAt)
	assert.Equal(t, "https://jobs.smartrecruiters.com/Equinox/744000137225639-female-locker-room-associate-houston", d.URL)

	// All four jobAd sections joined as titled plain-text blocks, HTML
	// stripped.
	assert.Contains(t, d.Description, "Company Description:")
	assert.Contains(t, d.Description, "Job Description:")
	assert.Contains(t, d.Description, "Qualifications:")
	assert.Contains(t, d.Description, "Additional Information:")
	assert.NotContains(t, d.Description, "<p>")
}

func TestSmartRecruitersDetailNotFound(t *testing.T) {
	a, _ := testSmartRecruitersAdapter(t)
	_, err := a.Detail(t.Context(), "equinox", "000000000000")
	require.ErrorContains(t, err, "pass a job_id exactly as returned by the job search")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ats/ -run TestSmartRecruitersDetail -v`
Expected: FAIL — `smartrecruiters: Detail not implemented yet`.

- [ ] **Step 3: Implement Detail**

In `internal/ats/smartrecruiters.go`, add `"cmp"` and `"github.com/jaytaylor/html2text"` to imports, remove `"errors"` (no stub remains), and replace the Detail stub:

```go
func (a *SmartRecruitersAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	res, err := a.client.GetPosting(ctx, smartrecruiters.GetPostingParams{
		CompanyIdentifier: slug,
		PostingId:         jobID,
	})
	if err != nil {
		return nil, fmt.Errorf("smartrecruiters: fetch job %q for %q: %w", jobID, slug, err)
	}
	d, ok := res.(*smartrecruiters.Posting)
	if !ok {
		// The only other GetPostingRes variant is the 404
		// PostingErrorResponse, for an unknown company or posting id.
		return nil, fmt.Errorf("smartrecruiters: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
	}
	_, name := resolveSmartRecruitersCompany(slug)
	return &JobDetail{
		JobID:       cmp.Or(d.ID.Value, jobID),
		Title:       d.Name.Value,
		Company:     cmp.Or(d.Company.Value.Name.Value, name),
		Location:    d.Location.Value.FullLocation.Value,
		PostedAt:    smartRecruitersPostedAt(d.ReleasedDate),
		URL:         d.PostingUrl.Value,
		Description: smartRecruitersDescription(d.JobAd),
	}, nil
}

// smartRecruitersDescription joins the jobAd's non-empty HTML sections as
// titled plain-text blocks, in the API's canonical section order.
func smartRecruitersDescription(jobAd smartrecruiters.OptJobAdSections) string {
	sections, ok := jobAd.Value.Sections.Get()
	if !ok {
		return ""
	}
	ordered := []struct {
		fallbackTitle string
		sec           smartrecruiters.OptJobAdSection
	}{
		{"Company Description", sections.CompanyDescription},
		{"Job Description", sections.JobDescription},
		{"Qualifications", sections.Qualifications},
		{"Additional Information", sections.AdditionalInformation},
	}
	var parts []string
	for _, s := range ordered {
		sec, ok := s.sec.Get()
		if !ok || sec.Text.Value == "" {
			continue
		}
		text, err := html2text.FromString(sec.Text.Value, html2text.Options{})
		if err != nil {
			// Keep the section as raw HTML rather than dropping it
			// (mirrors cmd/smartrecruiters's printSection).
			text = sec.Text.Value
		}
		parts = append(parts, cmp.Or(sec.Title.Value, s.fallbackTitle)+":\n"+text)
	}
	return strings.Join(parts, "\n\n")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ats/ -v -run TestSmartRecruiters`
Expected: PASS (all TestSmartRecruiters* tests).

- [ ] **Step 5: Commit**

```bash
git add internal/ats/smartrecruiters.go internal/ats/smartrecruiters_test.go
git commit -m "feat(ats): implement SmartRecruiters job detail with sectioned description"
```

---

### Task 5: Wire the adapter into the MCP server, verify-companies, and docs

**Files:**
- Modify: `cmd/openings-mcp/main.go` (`newATSRegistry`, around line 171)
- Modify: `internal/ats/registry.go` (`careersHostPatternsByAdapter`, around line 101)
- Modify: `cmd/verify-companies/main.go` (`providerOrder` line 29, `--provider` usage string line 60, `buildAdapters` around line 142)
- Modify: `README.md` (ATS platform list, lines 13-14)
- Modify: `internal/ats/smartrecruiters_test.go` (registry resolution test)

**Interfaces:**
- Consumes: `NewSmartRecruitersAdapter` (Task 1); `ats.NewRegistry`, `Registry.Resolve` (`internal/ats/registry.go`).
- Produces: the SmartRecruiters roster live behind `search_jobs_by_company` and friends; `verify-companies --provider smartrecruiters` works.

- [ ] **Step 1: Write the failing registry-resolution test**

Add to `internal/ats/smartrecruiters_test.go`:

```go
func TestSmartRecruitersCareersHostPatternRegistered(t *testing.T) {
	// The registry only advertises careers-URL shapes for adapters listed
	// in careersHostPatternsByAdapter; a missing entry silently degrades
	// the "unrecognized careers URL" teaching error.
	assert.Contains(t, careersHostPatternsByAdapter, "smartrecruiters")
}

func TestSmartRecruitersResolvesThroughRegistry(t *testing.T) {
	a, err := NewSmartRecruitersAdapter("https://api.smartrecruiters.com", http.DefaultClient)
	require.NoError(t, err)
	r, err := NewRegistry(a)
	require.NoError(t, err)

	got, slug, err := r.Resolve("Equinox")
	require.NoError(t, err)
	assert.Equal(t, "smartrecruiters", got.Name())
	assert.Equal(t, "equinox", slug)

	got, slug, err = r.Resolve("https://jobs.smartrecruiters.com/SomeUnknownCo")
	require.NoError(t, err)
	assert.Equal(t, "smartrecruiters", got.Name())
	assert.Equal(t, "someunknownco", slug)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/ats/ -run 'TestSmartRecruitersCareersHostPattern|TestSmartRecruitersResolvesThroughRegistry' -v`
Expected: FAIL — `careersHostPatternsByAdapter` has no `"smartrecruiters"` key. (The Resolve test may already pass; that's fine.)

- [ ] **Step 3: Add the host pattern**

In `internal/ats/registry.go`, extend the map:

```go
var careersHostPatternsByAdapter = map[string]string{
	"workday":         "<tenant>.<wd*>.myworkdayjobs.com/<site>",
	"greenhouse":      "job-boards.greenhouse.io/<board>",
	"lever":           "jobs.lever.co/<org>",
	"ashby":           "jobs.ashbyhq.com/<org>",
	"smartrecruiters": "jobs.smartrecruiters.com/<company>",
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/ats/ -run TestSmartRecruiters -v`
Expected: PASS.

- [ ] **Step 5: Register the adapter in the MCP server**

In `cmd/openings-mcp/main.go`, inside `newATSRegistry` after the greenhouse adapter block:

```go
	smartrecruitersAdapter, err := ats.NewSmartRecruitersAdapter("https://api.smartrecruiters.com", hc)
	if err != nil {
		return nil, fmt.Errorf("create SmartRecruiters ATS adapter: %w", err)
	}

	return ats.NewRegistry(
		ats.NewWorkdayAdapter(hc),
		leverAdapter,
		ashbyAdapter,
		greenhouseAdapter,
		smartrecruitersAdapter,
	)
```

- [ ] **Step 6: Add the provider to verify-companies**

In `cmd/verify-companies/main.go`:

```go
// providerOrder fixes the --provider default and the report's grouping order.
var providerOrder = []string{"ashby", "greenhouse", "lever", "smartrecruiters", "workday"}
```

Update the `--provider` flag help string to match:

```go
		providers   = fs.StringLong("provider", strings.Join(providerOrder, ","), "comma-separated subset of ashby,greenhouse,lever,smartrecruiters,workday")
```

And in `buildAdapters`, add a case (keep the switch alphabetical):

```go
		case "smartrecruiters":
			a, err = ats.NewSmartRecruitersAdapter("https://api.smartrecruiters.com", hc)
```

- [ ] **Step 7: Update the README provider list**

In `README.md`, the "Company career sites" bullet currently reads:

```markdown
- **Company career sites**: 2,000+ companies hosted on the
  **[Workday](https://www.workday.com)**, **[Ashby](https://www.ashbyhq.com)**,
  **[Greenhouse](https://www.greenhouse.com)**, and **[Lever](https://www.lever.co)**
  ATS platforms, all behind one company-search tool. A company outside the
  built-in roster works too: pass its careers-page URL on any of those platforms.
```

Change the platform list to include SmartRecruiters:

```markdown
- **Company career sites**: 2,000+ companies hosted on the
  **[Workday](https://www.workday.com)**, **[Ashby](https://www.ashbyhq.com)**,
  **[Greenhouse](https://www.greenhouse.com)**, **[Lever](https://www.lever.co)**,
  and **[SmartRecruiters](https://www.smartrecruiters.com)**
  ATS platforms, all behind one company-search tool. A company outside the
  built-in roster works too: pass its careers-page URL on any of those platforms.
```

- [ ] **Step 8: Build and run the full test suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: everything passes; `cmd/openings-mcp` and `cmd/verify-companies` compile with the new adapter.

- [ ] **Step 9: Smoke-check verify-companies wiring (no network needed for the flag path)**

Run: `go run ./cmd/verify-companies --provider bogus`
Expected: `err: unknown provider "bogus" (want any of ashby, greenhouse, lever, smartrecruiters, workday)` (nonzero exit).

- [ ] **Step 10: Commit**

```bash
git add cmd/openings-mcp/main.go internal/ats/registry.go cmd/verify-companies/main.go README.md internal/ats/smartrecruiters_test.go
git commit -m "feat: wire SmartRecruiters adapter into MCP server and verify-companies"
```

---

## Final verification

- `go build ./... && go vet ./... && go test ./...` — all green.
- Optional live smoke test (network): start the server and call `search_jobs_by_company` with `company: "Equinox"`, then `get_filters_by_company` and `get_job_detail_by_company` with a returned job_id; and once with `company: "https://jobs.smartrecruiters.com/Visa"` to exercise the careers-URL path.
