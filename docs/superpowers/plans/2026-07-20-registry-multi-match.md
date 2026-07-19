# Registry Multi-Match Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Merge the registry's `bySlug`/`byName` maps into one multi-value map so colliding rosters no longer fail startup, and fan the three company MCP tools out over every match, merging results.

**Architecture:** `ats.Registry` keeps a single `entries map[string][]registryEntry`; each roster company is inserted under `normalize(slug)` and `normalize(name)`. `Resolve` returns `[]ResolvedCompany`. The three tools in `internal/openingsmcp/company.go` iterate the matches sequentially: search concatenates/sums, filters unions, detail returns the first success. A failed adapter is skipped; only all-fail errors.

**Tech Stack:** Go, stretchr/testify (assert/require).

**Spec:** `docs/superpowers/specs/2026-07-20-registry-multi-match-design.md`

## Global Constraints

- Commit messages loosely follow Conventional Commits; body states the why in one short line, never restates the diff.
- All code, comments, and test names in English.
- Doc comments describe caller-facing behavior, not internal mechanism.
- No wrapper getters; no warnings side-channel in tool outputs; sequential fan-out (no goroutines).
- A duplicate slug **within one adapter's roster** still fails `NewRegistry`; cross-adapter collisions are now legal.

---

### Task 1: Registry multi-value map and slice-returning Resolve

**Files:**
- Modify: `internal/ats/registry.go`
- Modify: `internal/ats/registry_test.go`
- Modify: `internal/openingsmcp/company.go:80,120,147` (minimal signature adaptation only; real merging is Tasks 2–3)

**Interfaces:**
- Consumes: existing `Adapter`, `CompanyInfo`, `normalize`, `parseCareersInput`, `suggest`.
- Produces (Tasks 2–3 rely on these exact shapes):

```go
type ResolvedCompany struct {
    Adapter Adapter
    Slug    string
}

func (r *Registry) Resolve(company string) ([]ResolvedCompany, error)
```

`Resolve` never returns an empty slice with a nil error. Match order is adapter registration order.

- [ ] **Step 1: Rewrite the registry tests for the new semantics**

In `internal/ats/registry_test.go`, keep the `fakeAdapter` type and its methods as-is; replace everything from `testRegistry` down with the following (the lever roster gains `nvidia-jp` to create a cross-adapter name collision):

```go
func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := NewRegistry(
		&fakeAdapter{name: "workday", host: "jobs.fake-workday.example", roster: []CompanyInfo{
			{Slug: "nvidia", Name: "NVIDIA Corp"},
			{Slug: "workday", Name: "Workday, Inc."},
		}},
		&fakeAdapter{name: "lever", host: "jobs.fake-lever.example", roster: []CompanyInfo{
			{Slug: "palantir", Name: "Palantir Technologies"},
			{Slug: "nvidia-jp", Name: "NVIDIA Corp"},
		}},
	)
	require.NoError(t, err)
	return r
}

func TestResolveBySlug(t *testing.T) {
	r := testRegistry(t)
	rs, err := r.Resolve("palantir")
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, "lever", rs[0].Adapter.Name())
	assert.Equal(t, "palantir", rs[0].Slug)
}

func TestResolveByDisplayName(t *testing.T) {
	r := testRegistry(t)
	// Case, punctuation, and spaces must not matter.
	for _, input := range []string{"Workday, Inc.", "workday inc"} {
		rs, err := r.Resolve(input)
		require.NoErrorf(t, err, "Resolve(%q)", input)
		require.Lenf(t, rs, 1, "Resolve(%q)", input)
		assert.Equal(t, "workday", rs[0].Slug)
	}
}

func TestResolveMultiMatch(t *testing.T) {
	r := testRegistry(t)
	// "NVIDIA Corp" is workday's name+slug key and lever's name key; both
	// entries come back, in adapter registration order.
	rs, err := r.Resolve("NVIDIA Corp")
	require.NoError(t, err)
	require.Len(t, rs, 2)
	assert.Equal(t, "workday", rs[0].Adapter.Name())
	assert.Equal(t, "nvidia", rs[0].Slug)
	assert.Equal(t, "lever", rs[1].Adapter.Name())
	assert.Equal(t, "nvidia-jp", rs[1].Slug)
}

func TestResolveSlugKeyStaysSpecific(t *testing.T) {
	r := testRegistry(t)
	// The regional slug hits only its own entry, not the shared name key.
	rs, err := r.Resolve("nvidia-jp")
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, "lever", rs[0].Adapter.Name())
}

func TestResolveUnknownTeaches(t *testing.T) {
	r := testRegistry(t)
	_, err := r.Resolve("palantir tech")
	require.ErrorContains(t, err, "palantir", "suggestions should contain the input")
	assert.ErrorContains(t, err, "4 companies", "error should state supported count")
}

func TestResolveEmpty(t *testing.T) {
	r := testRegistry(t)
	_, err := r.Resolve("  ")
	assert.Error(t, err, "want error for empty company")
}

func TestNewRegistryAllowsCrossAdapterCollision(t *testing.T) {
	r, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{{Slug: "acme", Name: "Acme (Workday)"}}},
		&fakeAdapter{name: "lever", roster: []CompanyInfo{{Slug: "acme", Name: "Acme (Lever)"}}},
	)
	require.NoError(t, err, "cross-adapter slug collision must not fail startup")
	rs, err := r.Resolve("acme")
	require.NoError(t, err)
	assert.Len(t, rs, 2)
}

func TestNewRegistryRejectsDuplicateSlugWithinAdapter(t *testing.T) {
	_, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{
			{Slug: "acme", Name: "Acme"},
			{Slug: "Acme", Name: "Acme Holdings"},
		}},
	)
	assert.Error(t, err, "want error for duplicate slug within one adapter")
}

func TestResolveCareersURL(t *testing.T) {
	r := testRegistry(t)
	rs, err := r.Resolve("https://jobs.fake-lever.example/somestartup")
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, "lever", rs[0].Adapter.Name())
	assert.Equal(t, "somestartup", rs[0].Slug)
}

func TestResolveCareersURLSchemeless(t *testing.T) {
	r := testRegistry(t)
	rs, err := r.Resolve("jobs.fake-workday.example/acme")
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, "workday", rs[0].Adapter.Name())
	assert.Equal(t, "acme", rs[0].Slug)
}

func TestResolveUnrecognizedCareersURLTeaches(t *testing.T) {
	r := testRegistry(t)
	_, err := r.Resolve("https://careers.example.com/acme")
	require.ErrorContains(t, err, "careers URL", "URL misses should get the URL error, not name suggestions")
	assert.NotContains(t, err.Error(), "closest matches", "no levenshtein suggestions for URLs")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ats/ -run 'TestResolve|TestNewRegistry' 2>&1 | head -30`
Expected: compile errors (`r.Resolve` used with 3 return values in registry.go's own signature mismatch / undefined behavior) — the package does not build yet.

- [ ] **Step 3: Implement the registry changes**

In `internal/ats/registry.go`:

Replace the `Registry` struct and its comment:

```go
// Registry is the read-only union of every adapter's roster, built once at
// startup. It owns name resolution; adapters never see unresolved input.
type Registry struct {
	adapters []Adapter                  // registration order; polled for careers-URL input
	entries  map[string][]registryEntry // key: normalize(slug) and normalize(name); collisions append
	count    int                        // total roster companies, for the teaching error
	slugs    []slugEntry                // sorted by slug, for suggestions
}
```

Add `ResolvedCompany` next to it:

```go
// ResolvedCompany is one (adapter, slug) pair a company string resolved to.
// A query can match several rosters at once; callers fan out over all of
// them and merge.
type ResolvedCompany struct {
	Adapter Adapter
	Slug    string
}
```

Replace `NewRegistry`:

```go
// NewRegistry unions the adapters' rosters. Slugs and display names may
// collide across adapters — such keys resolve to every colliding entry. A
// duplicate slug within one adapter's roster is a curation bug — fail
// startup loudly rather than silently shadowing one company with another.
func NewRegistry(adapters ...Adapter) (*Registry, error) {
	r := &Registry{
		adapters: adapters,
		entries:  make(map[string][]registryEntry),
	}
	for _, a := range adapters {
		seen := make(map[string]string) // normalized slug -> original slug, this adapter only
		for _, c := range a.Roster() {
			e := registryEntry{adapter: a, slug: c.Slug, name: c.Name}
			slugKey := normalize(c.Slug)
			if prev, ok := seen[slugKey]; ok {
				return nil, fmt.Errorf("ats: adapter %s lists duplicate slug %q (collides with %q)",
					a.Name(), c.Slug, prev)
			}
			seen[slugKey] = c.Slug
			r.entries[slugKey] = append(r.entries[slugKey], e)
			if nameKey := normalize(c.Name); nameKey != slugKey {
				r.entries[nameKey] = append(r.entries[nameKey], e)
			}
			r.count++
			r.slugs = append(r.slugs, slugEntry{slug: c.Slug, norm: slugKey})
		}
	}
	slices.SortFunc(r.slugs, func(a, b slugEntry) int { return strings.Compare(a.slug, b.slug) })
	return r, nil
}
```

Replace `Resolve`:

```go
// Resolve maps a user-supplied company string to every (adapter, slug) it
// names. The input can be a roster slug, a display name, or a careers URL.
// Slugs and names may collide across adapters; all colliding entries come
// back, in adapter registration order, and callers merge their results. A
// returned slug is not always a roster key: a careers URL for a company
// outside the roster resolves to whatever slug the owning adapter minted via
// [Adapter.ParseCareersURL] (Workday mints the canonical careers URL).
// Misses return a teaching error carrying the closest slugs, so one retry
// from the LLM almost always lands. A nil error implies at least one match.
func (r *Registry) Resolve(company string) ([]ResolvedCompany, error) {
	key := normalize(company)
	if key == "" {
		return nil, errors.New("company is required")
	}
	// 1. match slug or display name
	if es, ok := r.entries[key]; ok {
		out := make([]ResolvedCompany, 0, len(es))
		for _, e := range es {
			out = append(out, ResolvedCompany{Adapter: e.adapter, Slug: e.slug})
		}
		return out, nil
	}
	// 2. fallback to careers URL to match url slug
	if u, ok := parseCareersInput(company); ok {
		for _, a := range r.adapters {
			if slug, ok := a.ParseCareersURL(u); ok {
				return []ResolvedCompany{{Adapter: a, Slug: slug}}, nil
			}
		}
		return nil, fmt.Errorf("unrecognized careers URL %q; supported careers-page hosts: %s", company, strings.Join(r.careersHostPatterns(), ", "))
	}
	return nil, fmt.Errorf("unknown company %q; closest matches: %s. %d companies are supported — pass one of the suggested slugs",
		company, strings.Join(r.suggest(key, 3), ", "), r.count)
}
```

Everything else in the file (`registryEntry`, `slugEntry`, `careersHostPatternsByAdapter`, `careersHostPatterns`, `normalize`, `suggest`, `levenshtein`) is unchanged. The `name` field of `registryEntry` is still used by `NewRegistry`'s insertion; keep it.

- [ ] **Step 4: Minimally adapt the three call sites in `internal/openingsmcp/company.go`**

This task only restores compilation; merging semantics land in Tasks 2–3. In each of `companySearch` (line ~80), `companyFilters` (line ~120), and `companyDetail` (line ~147), replace:

```go
	adapter, slug, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
```

with:

```go
	resolved, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	adapter, slug := resolved[0].Adapter, resolved[0].Slug
```

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: all PASS (existing company tests exercise the single-match path, which is unchanged).

- [ ] **Step 6: Commit**

```bash
git add internal/ats/registry.go internal/ats/registry_test.go internal/openingsmcp/company.go
git commit -m "refactor(ats): merge bySlug/byName into multi-match Resolve

Same company can be listed on several ATSes and unrelated companies can
share a normalized key; collisions resolve to every entry instead of
failing startup."
```

---

### Task 2: search_jobs_by_company fan-out merge

**Files:**
- Modify: `internal/openingsmcp/company.go` (`companySearch`)
- Modify: `internal/openingsmcp/company_test.go` (`stubAdapter` + new tests)

**Interfaces:**
- Consumes: `reg.Resolve(company) ([]ats.ResolvedCompany, error)` from Task 1; `ats.ResolvedCompany{Adapter, Slug}`.
- Produces: `stubAdapter` gains `name string`, `roster []ats.CompanyInfo`, `searchErr error` fields with zero-value backward compatibility — Task 3's tests reuse them.

- [ ] **Step 1: Extend `stubAdapter` and write the failing tests**

In `internal/openingsmcp/company_test.go`, replace the `stubAdapter` struct and its `Name`/`Roster`/`Search` methods:

```go
// stubAdapter returns canned results so tests exercise only the MCP
// translation layer. Zero-value fields keep the historical defaults:
// name "stub", roster [{acme, Acme Corp}].
type stubAdapter struct {
	name         string
	roster       []ats.CompanyInfo
	searchResult *ats.SearchResult
	searchErr    error
	filterSet    ats.FilterSet
	filtersErr   error
	detail       *ats.JobDetail
	detailErr    error
	gotParams    ats.SearchParams
}

func (s *stubAdapter) Name() string {
	if s.name == "" {
		return "stub"
	}
	return s.name
}

func (s *stubAdapter) Roster() []ats.CompanyInfo {
	if s.roster == nil {
		return []ats.CompanyInfo{{Slug: "acme", Name: "Acme Corp"}}
	}
	return s.roster
}

func (s *stubAdapter) ParseCareersURL(*url.URL) (string, bool) { return "", false }

func (s *stubAdapter) Search(_ context.Context, _ string, p ats.SearchParams) (*ats.SearchResult, error) {
	s.gotParams = p
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResult, nil
}

func (s *stubAdapter) Filters(context.Context, string) (ats.FilterSet, error) {
	if s.filtersErr != nil {
		return nil, s.filtersErr
	}
	return s.filterSet, nil
}

func (s *stubAdapter) Detail(context.Context, string, string) (*ats.JobDetail, error) {
	if s.detailErr != nil {
		return nil, s.detailErr
	}
	return s.detail, nil
}
```

Add a multi-adapter registry helper next to `testCompanyRegistry`:

```go
// testMultiRegistry registers two stubs whose rosters share the display
// name "Acme Corp", so Resolve("Acme Corp") fans out to both.
func testMultiRegistry(t *testing.T, a, b *stubAdapter) *ats.Registry {
	t.Helper()
	a.name, b.name = "stub-a", "stub-b"
	a.roster = []ats.CompanyInfo{{Slug: "acme", Name: "Acme Corp"}}
	b.roster = []ats.CompanyInfo{{Slug: "acme-jp", Name: "Acme Corp"}}
	r, err := ats.NewRegistry(a, b)
	require.NoError(t, err)
	return r
}
```

Append the new tests:

```go
func TestCompanySearchMergesMultiMatch(t *testing.T) {
	a := &stubAdapter{searchResult: &ats.SearchResult{
		Jobs:       []ats.JobSummary{{JobID: "a1", Title: "Engineer"}},
		TotalCount: 21, Page: 1, TotalPages: 2,
	}}
	b := &stubAdapter{searchResult: &ats.SearchResult{
		Jobs:       []ats.JobSummary{{JobID: "b1", Title: "Engineer, Japan"}},
		TotalCount: 5, Page: 1, TotalPages: 1,
	}}
	reg := testMultiRegistry(t, a, b)

	out, err := companySearch(t.Context(), reg, &companySearchInput{Company: "Acme Corp"})
	require.NoError(t, err)

	require.Len(t, out.Data, 2)
	assert.Equal(t, "a1", out.Data[0].JobID, "jobs keep adapter registration order")
	assert.Equal(t, "b1", out.Data[1].JobID)
	assert.Equal(t, 26, out.TotalCount, "total_count sums across adapters")
	assert.Equal(t, 2, out.TotalPages, "total_pages takes the max")
	assert.Equal(t, 1, out.Page)
}

func TestCompanySearchSkipsFailedAdapter(t *testing.T) {
	a := &stubAdapter{searchErr: errors.New("upstream 500")}
	b := &stubAdapter{searchResult: &ats.SearchResult{
		Jobs:       []ats.JobSummary{{JobID: "b1"}},
		TotalCount: 1, Page: 1, TotalPages: 1,
	}}
	reg := testMultiRegistry(t, a, b)

	out, err := companySearch(t.Context(), reg, &companySearchInput{Company: "Acme Corp"})
	require.NoError(t, err, "one healthy adapter is enough")
	require.Len(t, out.Data, 1)
	assert.Equal(t, "b1", out.Data[0].JobID)
	assert.Equal(t, 1, out.TotalCount)
}

func TestCompanySearchAllAdaptersFail(t *testing.T) {
	a := &stubAdapter{searchErr: errors.New("upstream 500")}
	b := &stubAdapter{searchErr: errors.New("upstream 503")}
	reg := testMultiRegistry(t, a, b)

	_, err := companySearch(t.Context(), reg, &companySearchInput{Company: "Acme Corp"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "500")
	assert.ErrorContains(t, err, "503")
}
```

Add `"errors"` to the test file's imports.

- [ ] **Step 2: Run tests to verify the new ones fail**

Run: `go test ./internal/openingsmcp/ -run 'TestCompanySearch' -v 2>&1 | tail -20`
Expected: `TestCompanySearchMergesMultiMatch` FAILs (only one job returned — current code uses `resolved[0]` only); `TestCompanySearchSkipsFailedAdapter` FAILs with the upstream error. Pre-existing search tests PASS.

- [ ] **Step 3: Implement the merge in `companySearch`**

Replace `companySearch` in `internal/openingsmcp/company.go`:

```go
func companySearch(ctx context.Context, reg *ats.Registry, in *companySearchInput) (*companySearchOutput, error) {
	resolved, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	params := ats.SearchParams{
		Query:    in.Query,
		Location: in.Location,
		Filters:  in.Filters,
		Page:     in.Page,
	}
	out := &companySearchOutput{Data: []companyJobSummary{}}
	var errs []error
	for _, rc := range resolved {
		res, err := rc.Adapter.Search(ctx, rc.Slug, params)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, j := range res.Jobs {
			out.Data = append(out.Data, companyJobSummary{
				JobID:    j.JobID,
				Title:    j.Title,
				Location: j.Location,
				PostedAt: j.PostedAt,
				URL:      j.URL,
			})
		}
		out.TotalCount += res.TotalCount
		out.Page = res.Page
		out.TotalPages = max(out.TotalPages, res.TotalPages)
	}
	if len(errs) == len(resolved) {
		return nil, errors.Join(errs...)
	}
	return out, nil
}
```

Add `"errors"` to `company.go`'s imports.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/openingsmcp/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/openingsmcp/company.go internal/openingsmcp/company_test.go
git commit -m "feat(mcp): fan search_jobs_by_company out over every resolved match

Multi-match companies live on several ATSes; one page per adapter merged
beats hiding all but the first roster hit."
```

---

### Task 3: filters union and detail first-success

**Files:**
- Modify: `internal/openingsmcp/company.go` (`companyFilters`, `companyDetail`)
- Modify: `internal/openingsmcp/company_test.go`

**Interfaces:**
- Consumes: `reg.Resolve` slice semantics (Task 1); `stubAdapter.filtersErr`/`detailErr` and `testMultiRegistry` (Task 2).
- Produces: nothing new for later tasks; this is the last task.

- [ ] **Step 1: Write the failing tests**

Append to `internal/openingsmcp/company_test.go`:

```go
func TestCompanyFiltersUnionsMultiMatch(t *testing.T) {
	a := &stubAdapter{filterSet: ats.FilterSet{"team": {"ML", "Web"}, "level": {"Senior"}}}
	b := &stubAdapter{filterSet: ats.FilterSet{"team": {"Web", "Hardware"}}}
	reg := testMultiRegistry(t, a, b)

	out, err := companyFilters(t.Context(), reg, &companyFiltersInput{Company: "Acme Corp"})
	require.NoError(t, err)
	assert.Equal(t, []string{"ML", "Web", "Hardware"}, out.Filters["team"], "values dedupe, first-seen order")
	assert.Equal(t, []string{"Senior"}, out.Filters["level"], "dimensions union")
}

func TestCompanyFiltersSkipsFailedAdapter(t *testing.T) {
	a := &stubAdapter{filtersErr: errors.New("upstream 500")}
	b := &stubAdapter{filterSet: ats.FilterSet{"team": {"Web"}}}
	reg := testMultiRegistry(t, a, b)

	out, err := companyFilters(t.Context(), reg, &companyFiltersInput{Company: "Acme Corp"})
	require.NoError(t, err)
	assert.Equal(t, []string{"Web"}, out.Filters["team"])
}

func TestCompanyDetailFirstSuccess(t *testing.T) {
	a := &stubAdapter{detailErr: errors.New("job not found")}
	b := &stubAdapter{detail: &ats.JobDetail{JobID: "j1", Title: "Engineer"}}
	reg := testMultiRegistry(t, a, b)

	out, err := companyDetail(t.Context(), reg, &companyDetailInput{Company: "Acme Corp", JobID: "j1"})
	require.NoError(t, err, "the job belongs to the second adapter")
	assert.Equal(t, "Engineer", out.Title)
}

func TestCompanyDetailAllFail(t *testing.T) {
	a := &stubAdapter{detailErr: errors.New("job not found in a")}
	b := &stubAdapter{detailErr: errors.New("job not found in b")}
	reg := testMultiRegistry(t, a, b)

	_, err := companyDetail(t.Context(), reg, &companyDetailInput{Company: "Acme Corp", JobID: "zzz"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "not found in a")
	assert.ErrorContains(t, err, "not found in b")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/openingsmcp/ -run 'TestCompanyFilters|TestCompanyDetail' -v 2>&1 | tail -20`
Expected: `TestCompanyFiltersUnionsMultiMatch` FAILs (missing "Hardware" — only first adapter consulted); `TestCompanyDetailFirstSuccess` FAILs with "job not found". Pre-existing filters/detail tests PASS.

- [ ] **Step 3: Implement the merges**

Replace `companyFilters` in `internal/openingsmcp/company.go`:

```go
func companyFilters(ctx context.Context, reg *ats.Registry, in *companyFiltersInput) (*companyFiltersOutput, error) {
	resolved, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	merged := ats.FilterSet{}
	var errs []error
	for _, rc := range resolved {
		fs, err := rc.Adapter.Filters(ctx, rc.Slug)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for dim, vals := range fs {
			for _, v := range vals {
				if !slices.Contains(merged[dim], v) {
					merged[dim] = append(merged[dim], v)
				}
			}
		}
	}
	if len(errs) == len(resolved) {
		return nil, errors.Join(errs...)
	}
	return &companyFiltersOutput{Filters: merged}, nil
}
```

Replace `companyDetail`:

```go
func companyDetail(ctx context.Context, reg *ats.Registry, in *companyDetailInput) (*companyDetailOutput, error) {
	resolved, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	// The job_id belongs to exactly one adapter; take the first that has it.
	var errs []error
	for _, rc := range resolved {
		d, err := rc.Adapter.Detail(ctx, rc.Slug, in.JobID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		return &companyDetailOutput{
			JobID:       d.JobID,
			Title:       d.Title,
			Company:     d.Company,
			Location:    d.Location,
			PostedAt:    d.PostedAt,
			URL:         d.URL,
			Description: d.Description,
		}, nil
	}
	return nil, errors.Join(errs...)
}
```

Add `"slices"` to `company.go`'s imports.

- [ ] **Step 4: Run the full test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 5: Run vet and the linter (if configured)**

Run: `go vet ./... && gofmt -l internal/`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add internal/openingsmcp/company.go internal/openingsmcp/company_test.go
git commit -m "feat(mcp): union filters and try each match for job detail

Completes multi-match fan-out: filters merge across adapters, detail
returns the first adapter that recognizes the job_id."
```
