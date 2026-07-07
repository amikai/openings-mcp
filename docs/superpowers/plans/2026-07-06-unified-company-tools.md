# 統一 Company 工具實作計畫

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `internal/ats` package(Adapter interface + registry/resolver + full-dump 過濾引擎 + workday/lever/ashby 三個 adapter),並在 MCP server 掛上 `search_jobs_by_company` / `get_filters_by_company` / `get_job_detail_by_company` 三個工具。

**Architecture:** provider(純 client,零改動)→ ats(統一語意)→ openingsmcp(MCP schema 轉譯)→ main(組裝)。Workday 走伺服器端搜尋(帶 location/filters 時多一次 facet 探測請求);lever/ashby 抓整包後由共用引擎過濾。詳見 spec:`docs/superpowers/specs/2026-07-06-unified-company-tools-design.md`。

**Tech Stack:** Go 1.26、ogen 生成的 provider clients(已存在)、`github.com/modelcontextprotocol/go-sdk/mcp`、`github.com/jaytaylor/html2text`(皆為既有依賴,不加新依賴)。

## Global Constraints

- 不新增任何 go.mod 依賴
- 不修改 `internal/provider/*` 任何檔案
- 每頁固定 20 筆:`ats.PageSize = 20`
- 工具名:`search_jobs_by_company`、`get_filters_by_company`、`get_job_detail_by_company`(不可變動)
- Adapter interface 方法名:`Name` / `Roster` / `Search` / `Filters` / `Detail`(spec 定案)
- 所有 tool call 層級錯誤走既有 `openingsmcp.errorResult`(IsError,不炸 protocol)
- 寫 Go 時遵循 golang-style skill 與 codebase 既有慣例(註解寫「為什麼」、`sort.Slice`、錯誤訊息小寫開頭)
- 測試一律打各 provider 的 `NewMockServer()`,不打真站
- 每個 task 結尾的 commit 步驟需使用者事先同意(見使用者的 no-auto-commit 偏好);未同意則跳過 commit 步驟,變更累積由使用者處理

## 驗證指令(每個 task 通用)

```bash
go build ./...
go vet ./...
gofmt -l internal/ cmd/        # 應無輸出
go test ./internal/ats/ ./internal/openingsmcp/ -v -run <本 task 的測試>
```

## Fixture 速查(測試斷言依據,皆已驗證)

- **workday mock**(`workday.NewMockServer()`,路由 `/jobs`、`/job/`):search 回 `total=27`、20 筆;第 1 筆 title `Software Golang Kubernetes Engineer`、externalPath `/job/Israel-Tel-Aviv/Software-Golang-Kubernetes-Engineer_JR2020442`、locationsText `3 Locations`。facet 樹含 `jobFamilyGroup`(Engineering/Marketing)、`workerSubType`、`timeType`(Full time,id=`5509c0b5959810ac0029943377d47364`)、`locationMainGroup`(巢狀:locationHierarchy2/locationHierarchy1/locations;`locations` 葉含 `Israel, Tel Aviv` id=`c7769ee377291036b08490819096b8bf`)。detail 回 title `Senior Software Golang Kubernetes Engineer`、jobDescription 為 HTML(`<p>NVIDIA Networking...`)、externalUrl 非空。
- **lever mock**(`lever.NewMockServer()`;`lever.MockNotFoundSite`/`MockNotFoundPostingID` 觸發 404):list 回 3 筆;第 1 筆 id `33538a2f-d27d-4a96-8f05-fa4b0e4d940e`、text `AbelsonTaylor Writer`、categories: team `Professional Services`、commitment `Regular Full Time (Salary)`、location `Arlington, TX`、workplaceType `hybrid`、createdAt `1553186035299`(epoch ms)。detail 回同 id,descriptionPlain 以 `Welcome to the Demo` 開頭,lists 2 個區塊。
- **ashby mock**(`ashby.NewMockServer()`,只認 `ashby.MockBoardName` = `browserbase`,其他 board 回 404):5 筆全 `isListed=true`;第 1 筆 id `7724fbe3-6a27-4418-9705-2dcc40751a16`、title `Software Engineer (Agent Platform)`、department/team `Engineering`、employmentType `FullTime`、location `San Francisco`、workplaceType `OnSite`、isRemote false。

---

### Task 1: `internal/ats` 核心型別與 Adapter interface

**Files:**
- Create: `internal/ats/ats.go`

**Interfaces:**
- Consumes: 無(純型別定義)
- Produces: `Adapter` interface、`CompanyInfo`、`SearchParams`、`SearchResult`、`JobSummary`、`FilterSet`、`JobDetail`、`PageSize`——後續所有 task 都依賴這些名字與欄位,不可改動

- [ ] **Step 1: 建立 ats.go**

```go
// Package ats unifies the ATS-backed providers (workday, lever, ashby)
// behind one company-parameterized search interface, so MCP clients name a
// company and never learn which ATS serves it.
package ats

import "context"

// PageSize is the fixed page size for every adapter. Workday caps limit at
// 20 on at least one tenant, so 20 is the largest safe uniform value.
const PageSize = 20

// Adapter is one ATS's implementation of the unified search interface.
// Methods address a company by slug; slugs are declared by Roster() and
// indexed by Registry, so a slug that reaches an adapter is always one it
// declared.
type Adapter interface {
	// Name identifies the adapter ("workday", "lever", "ashby") in logs
	// and error messages only; it never reaches tool schemas.
	Name() string
	// Roster lists every curated company on this ATS.
	Roster() []CompanyInfo
	Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error)
	Filters(ctx context.Context, slug string) (FilterSet, error)
	Detail(ctx context.Context, slug, jobID string) (*JobDetail, error)
}

// CompanyInfo is one company as the registry sees it: enough to resolve a
// user-supplied name to (adapter, slug). Connection config (Workday
// tenant/instance/site etc.) stays inside each adapter, looked up by slug.
type CompanyInfo struct {
	Slug string // unique key; the provider roster's tenant/site/board slug
	Name string // display name; the resolver also matches on it
}

// SearchParams are the unified search inputs. Semantics are identical
// across adapters; how each maps them upstream is the adapter's business.
type SearchParams struct {
	Query    string              // keywords: titles, skills, tech — never locations
	Location string              // fuzzy text match; "remote" is special-cased
	Filters  map[string][]string // escape hatch; keys/values come from Filters(); OR within a key, AND across keys
	Page     int                 // 1-based; values < 1 mean page 1
}

// SearchResult is one page of unified search results.
type SearchResult struct {
	Jobs       []JobSummary
	TotalCount int
	Page       int
	TotalPages int
}

// JobSummary carries summary fields only — full descriptions are Detail's
// job, keeping search responses small for the LLM.
type JobSummary struct {
	JobID    string // provider-native id (workday externalPath, lever uuid, ashby id)
	Title    string
	Location string
	PostedAt string // ISO 8601 date where the upstream provides one; otherwise its raw text
	URL      string // human-clickable posting page
}

// FilterSet maps a filter dimension to its currently valid values, as
// display labels. Tenant-specific and discovered at call time.
type FilterSet map[string][]string

// JobDetail is one full posting, description normalized to plain text.
type JobDetail struct {
	JobID       string
	Title       string
	Company     string
	Location    string
	PostedAt    string
	URL         string
	Description string
}
```

- [ ] **Step 2: 驗證編譯**

Run: `go build ./... && go vet ./internal/ats/`
Expected: 無輸出、exit 0

- [ ] **Step 3: Commit**

```bash
git add internal/ats/ats.go
git commit -m "feat(ats): add unified adapter interface and core types"
```

---

### Task 2: Registry 與 Resolver

**Files:**
- Create: `internal/ats/registry.go`
- Test: `internal/ats/registry_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Adapter`、`CompanyInfo`
- Produces: `NewRegistry(adapters ...Adapter) (*Registry, error)`、`(*Registry).Resolve(company string) (Adapter, string, error)`——Task 7 的 MCP 層靠這兩個簽名

- [ ] **Step 1: 寫失敗測試**

`internal/ats/registry_test.go`(package `ats`,內部測試,可直接用未匯出符號):

```go
package ats

import (
	"context"
	"strings"
	"testing"
)

// fakeAdapter satisfies Adapter with a canned roster; search methods are
// never reached by registry tests.
type fakeAdapter struct {
	name   string
	roster []CompanyInfo
}

func (f *fakeAdapter) Name() string          { return f.name }
func (f *fakeAdapter) Roster() []CompanyInfo { return f.roster }
func (f *fakeAdapter) Search(context.Context, string, SearchParams) (*SearchResult, error) {
	return nil, nil
}
func (f *fakeAdapter) Filters(context.Context, string) (FilterSet, error) { return nil, nil }
func (f *fakeAdapter) Detail(context.Context, string, string) (*JobDetail, error) {
	return nil, nil
}

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{
			{Slug: "nvidia", Name: "NVIDIA Corp"},
			{Slug: "workday", Name: "Workday, Inc."},
		}},
		&fakeAdapter{name: "lever", roster: []CompanyInfo{
			{Slug: "palantir", Name: "Palantir Technologies"},
		}},
	)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return r
}

func TestResolveBySlug(t *testing.T) {
	r := testRegistry(t)
	a, slug, err := r.Resolve("nvidia")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if a.Name() != "workday" || slug != "nvidia" {
		t.Errorf("got (%s, %s), want (workday, nvidia)", a.Name(), slug)
	}
}

func TestResolveByDisplayName(t *testing.T) {
	r := testRegistry(t)
	// Case, punctuation, and spaces must not matter.
	for _, input := range []string{"NVIDIA Corp", "nvidia corp", "Workday, Inc.", "workday inc"} {
		if _, _, err := r.Resolve(input); err != nil {
			t.Errorf("Resolve(%q): %v", input, err)
		}
	}
}

func TestResolveUnknownTeaches(t *testing.T) {
	r := testRegistry(t)
	_, _, err := r.Resolve("palantir tech")
	if err == nil {
		t.Fatal("want error for unknown company")
	}
	msg := err.Error()
	if !strings.Contains(msg, "palantir") {
		t.Errorf("suggestions should contain %q, got: %s", "palantir", msg)
	}
	if !strings.Contains(msg, "3 companies") {
		t.Errorf("error should state supported count, got: %s", msg)
	}
}

func TestResolveEmpty(t *testing.T) {
	r := testRegistry(t)
	if _, _, err := r.Resolve("  "); err == nil {
		t.Fatal("want error for empty company")
	}
}

func TestNewRegistryRejectsDuplicateSlug(t *testing.T) {
	_, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{{Slug: "acme", Name: "Acme (Workday)"}}},
		&fakeAdapter{name: "lever", roster: []CompanyInfo{{Slug: "acme", Name: "Acme (Lever)"}}},
	)
	if err == nil {
		t.Fatal("want error for duplicate slug across adapters")
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./internal/ats/ -run TestResolve -v`
Expected: FAIL(`undefined: NewRegistry`)

- [ ] **Step 3: 實作 registry.go**

```go
package ats

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// registryEntry binds one resolved company to the adapter that serves it.
type registryEntry struct {
	adapter Adapter
	slug    string
	name    string
}

// Registry is the read-only union of every adapter's roster, built once at
// startup. It owns name resolution; adapters never see unresolved input.
type Registry struct {
	bySlug map[string]registryEntry // key: normalize(slug)
	byName map[string]registryEntry // key: normalize(display name)
	slugs  []string                 // original slugs, sorted, for suggestions
}

// NewRegistry unions the adapters' rosters. A slug or normalized display
// name colliding across entries is a curation bug — fail startup loudly
// rather than silently shadowing one company with another.
func NewRegistry(adapters ...Adapter) (*Registry, error) {
	r := &Registry{
		bySlug: make(map[string]registryEntry),
		byName: make(map[string]registryEntry),
	}
	for _, a := range adapters {
		for _, c := range a.Roster() {
			e := registryEntry{adapter: a, slug: c.Slug, name: c.Name}
			slugKey := normalize(c.Slug)
			if prev, ok := r.bySlug[slugKey]; ok {
				return nil, fmt.Errorf("ats: company slug %q from %s collides with %q from %s",
					c.Slug, a.Name(), prev.slug, prev.adapter.Name())
			}
			r.bySlug[slugKey] = e
			nameKey := normalize(c.Name)
			if prev, ok := r.byName[nameKey]; ok {
				return nil, fmt.Errorf("ats: company name %q from %s collides with %q from %s",
					c.Name, a.Name(), prev.name, prev.adapter.Name())
			}
			r.byName[nameKey] = e
			r.slugs = append(r.slugs, c.Slug)
		}
	}
	sort.Strings(r.slugs)
	return r, nil
}

// Resolve maps a user-supplied company string to (adapter, slug). Misses
// return a teaching error carrying the closest slugs, so one retry from the
// LLM almost always lands.
func (r *Registry) Resolve(company string) (Adapter, string, error) {
	key := normalize(company)
	if key == "" {
		return nil, "", fmt.Errorf("company is required")
	}
	if e, ok := r.bySlug[key]; ok {
		return e.adapter, e.slug, nil
	}
	if e, ok := r.byName[key]; ok {
		return e.adapter, e.slug, nil
	}
	return nil, "", fmt.Errorf("unknown company %q; closest matches: %s. %d companies are supported — pass one of the suggested slugs",
		company, strings.Join(r.suggest(key, 3), ", "), len(r.bySlug))
}

// normalize folds case and strips everything but letters and digits, so
// "Workday, Inc." and "workday inc" collide on purpose.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// suggest ranks slugs for a missed lookup: substring hits (either
// direction) beat everything, then edit distance breaks ties.
func (r *Registry) suggest(key string, n int) []string {
	type scored struct {
		slug string
		dist int
	}
	ranked := make([]scored, 0, len(r.slugs))
	for _, slug := range r.slugs {
		norm := normalize(slug)
		dist := levenshtein(key, norm)
		if strings.Contains(norm, key) || strings.Contains(key, norm) {
			dist = 0
		}
		ranked = append(ranked, scored{slug: slug, dist: dist})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].dist != ranked[j].dist {
			return ranked[i].dist < ranked[j].dist
		}
		return ranked[i].slug < ranked[j].slug
	})
	if len(ranked) > n {
		ranked = ranked[:n]
	}
	out := make([]string, 0, len(ranked))
	for _, s := range ranked {
		out = append(out, s.slug)
	}
	return out
}

// levenshtein is the classic two-row edit distance; rosters are a few
// hundred short strings, so no need for anything fancier.
func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}
```

- [ ] **Step 4: 跑測試確認通過**

Run: `go test ./internal/ats/ -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ats/registry.go internal/ats/registry_test.go
git commit -m "feat(ats): add company registry with teaching-error resolver"
```

---

### Task 3: Full-dump 過濾引擎

**Files:**
- Create: `internal/ats/filter.go`
- Test: `internal/ats/filter_test.go`

**Interfaces:**
- Consumes: Task 1 的 `SearchParams`、`SearchResult`、`JobSummary`、`FilterSet`、`PageSize`
- Produces(package 內部,Task 4/5 依賴):
  - `type dumpJob struct { summary JobSummary; sortKey time.Time; title, orgUnit, description, locations string; fields map[string]string; isRemote bool }`
  - `func searchDump(jobs []dumpJob, p SearchParams) (*SearchResult, error)`
  - `func distinctFilters(jobs []dumpJob) FilterSet`

- [ ] **Step 1: 寫失敗測試**

`internal/ats/filter_test.go`:

```go
package ats

import (
	"strings"
	"testing"
	"time"
)

func dj(id, title, orgUnit, desc, loc string, posted time.Time, fields map[string]string, remote bool) dumpJob {
	return dumpJob{
		summary:     JobSummary{JobID: id, Title: title, Location: loc, PostedAt: posted.Format("2006-01-02"), URL: "https://example.com/" + id},
		sortKey:     posted,
		title:       title,
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
		dj("a", "Senior Go Engineer", "Platform", "You will write Go services", "Taipei, Taiwan", day(1), map[string]string{"team": "Platform", "commitment": "Full-time"}, false),
		dj("b", "Frontend Engineer", "Web", "React and TypeScript, some Go tooling", "London, UK", day(3), map[string]string{"team": "Web", "commitment": "Full-time"}, false),
		dj("c", "Data Scientist", "ML", "Python and statistics", "Remote - US", day(2), map[string]string{"team": "ML", "commitment": "Contract"}, true),
	}
}

func TestSearchDumpQueryANDAcrossWords(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Query: "go engineer"})
	if err != nil {
		t.Fatal(err)
	}
	// "go engineer": job a matches both words in title; job b matches
	// "engineer" in title and "go" only in description — still a match
	// (AND is across the whole text), but ranked below the title hit.
	if len(res.Jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(res.Jobs))
	}
	if res.Jobs[0].JobID != "a" {
		t.Errorf("title hit should rank first, got %q", res.Jobs[0].JobID)
	}
	if res.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", res.TotalCount)
	}
}

func TestSearchDumpSortNewestFirstIDTiebreak(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	// No query: rank is uniform, so order is posted desc: b(3rd) c(2nd) a(1st).
	want := []string{"b", "c", "a"}
	for i, w := range want {
		if res.Jobs[i].JobID != w {
			t.Fatalf("order = %v..., want %v", res.Jobs[i].JobID, want)
		}
	}
}

func TestSearchDumpLocation(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Location: "taipei"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 1 || res.Jobs[0].JobID != "a" {
		t.Fatalf("location=taipei should match only job a, got %v", res.Jobs)
	}
}

func TestSearchDumpRemoteSpecialCase(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Location: "remote"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 1 || res.Jobs[0].JobID != "c" {
		t.Fatalf("location=remote should match only job c, got %v", res.Jobs)
	}
}

func TestSearchDumpFiltersORWithinKeyANDAcrossKeys(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{
		Filters: map[string][]string{"team": {"Platform", "Web"}, "commitment": {"Full-time"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 2 {
		t.Fatalf("got %d jobs, want 2 (a and b)", len(res.Jobs))
	}
}

func TestSearchDumpFilterValueCaseInsensitive(t *testing.T) {
	res, err := searchDump(testJobs(), SearchParams{Filters: map[string][]string{"team": {"platform"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) != 1 || res.Jobs[0].JobID != "a" {
		t.Fatalf("got %v, want job a", res.Jobs)
	}
}

func TestSearchDumpUnknownFilterKeyTeaches(t *testing.T) {
	_, err := searchDump(testJobs(), SearchParams{Filters: map[string][]string{"bogus": {"x"}}})
	if err == nil {
		t.Fatal("want error for unknown filter key")
	}
	if !strings.Contains(err.Error(), "team") {
		t.Errorf("error should list valid keys, got: %v", err)
	}
}

func TestSearchDumpPagination(t *testing.T) {
	jobs := make([]dumpJob, 0, 45)
	for i := 0; i < 45; i++ {
		posted := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour)
		jobs = append(jobs, dj(strings.Repeat("z", 3)+string(rune('a'+i%26))+string(rune('0'+i/26)), "Engineer", "", "", "X", posted, nil, false))
	}
	page2, err := searchDump(jobs, SearchParams{Page: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page2.TotalCount != 45 || page2.TotalPages != 3 || page2.Page != 2 || len(page2.Jobs) != PageSize {
		t.Fatalf("page2 = {total %d, pages %d, page %d, len %d}", page2.TotalCount, page2.TotalPages, page2.Page, len(page2.Jobs))
	}
	page3, _ := searchDump(jobs, SearchParams{Page: 3})
	if len(page3.Jobs) != 5 {
		t.Errorf("page3 len = %d, want 5", len(page3.Jobs))
	}
	page9, _ := searchDump(jobs, SearchParams{Page: 9})
	if len(page9.Jobs) != 0 {
		t.Errorf("past-the-end page should be empty, got %d", len(page9.Jobs))
	}
	// Determinism: two identical calls agree item-for-item.
	again, _ := searchDump(jobs, SearchParams{Page: 2})
	for i := range page2.Jobs {
		if page2.Jobs[i].JobID != again.Jobs[i].JobID {
			t.Fatal("pagination is not deterministic")
		}
	}
}

func TestDistinctFilters(t *testing.T) {
	fs := distinctFilters(testJobs())
	if got := fs["team"]; len(got) != 3 || got[0] != "ML" {
		t.Errorf(`fs["team"] = %v, want sorted [ML Platform Web]`, got)
	}
	if got := fs["commitment"]; len(got) != 2 {
		t.Errorf(`fs["commitment"] = %v, want 2 distinct values`, got)
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./internal/ats/ -run 'TestSearchDump|TestDistinct' -v`
Expected: FAIL(`undefined: dumpJob`)

- [ ] **Step 3: 實作 filter.go**

```go
package ats

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// dumpJob is the filter engine's intermediate shape for one job from a
// full-dump provider (lever, ashby): the unified summary plus the
// searchable text and structured fields filtering needs. Adapters build
// these; the engine never touches provider types.
type dumpJob struct {
	summary     JobSummary
	sortKey     time.Time         // posting time, for deterministic newest-first ordering
	title       string            // query tier 1
	orgUnit     string            // query tier 2: team/department text
	description string            // query tier 3: full JD plain text
	locations   string            // every location string joined, for fuzzy matching
	fields      map[string]string // structured dimensions, e.g. "team" -> "Platform"
	isRemote    bool
}

// searchDump filters, ranks, and pages a full board dump. The upstream has
// no usable server-side search, so this layer IS the search — lossless,
// since the dump is complete. Ordering is deterministic (rank, then posted
// desc, then id) because stateless pagination depends on it.
func searchDump(jobs []dumpJob, p SearchParams) (*SearchResult, error) {
	if err := validateFilterKeys(jobs, p.Filters); err != nil {
		return nil, err
	}
	matched := make([]dumpJob, 0, len(jobs))
	for _, j := range jobs {
		if matchQuery(j, p.Query) && matchLocation(j, p.Location) && matchFilters(j, p.Filters) {
			matched = append(matched, j)
		}
	}
	sort.Slice(matched, func(i, k int) bool {
		a, b := matched[i], matched[k]
		ra, rb := queryRank(a, p.Query), queryRank(b, p.Query)
		if ra != rb {
			return ra < rb
		}
		if !a.sortKey.Equal(b.sortKey) {
			return a.sortKey.After(b.sortKey)
		}
		return a.summary.JobID < b.summary.JobID
	})

	page := p.Page
	if page < 1 {
		page = 1
	}
	total := len(matched)
	totalPages := (total + PageSize - 1) / PageSize
	start := min((page-1)*PageSize, total)
	end := min(start+PageSize, total)
	out := make([]JobSummary, 0, end-start)
	for _, j := range matched[start:end] {
		out = append(out, j.summary)
	}
	return &SearchResult{Jobs: out, TotalCount: total, Page: page, TotalPages: totalPages}, nil
}

// matchQuery requires every query word somewhere in the job's text.
// Ranking (title hits first) happens separately in queryRank.
func matchQuery(j dumpJob, query string) bool {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return true
	}
	blob := strings.ToLower(j.title + " " + j.orgUnit + " " + j.description)
	for _, w := range words {
		if !strings.Contains(blob, w) {
			return false
		}
	}
	return true
}

// queryRank orders matches: 0 when the title alone satisfies the whole
// query, 1 otherwise. A title hit is a far stronger signal than a JD-body
// mention.
func queryRank(j dumpJob, query string) int {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return 0
	}
	title := strings.ToLower(j.title)
	for _, w := range words {
		if !strings.Contains(title, w) {
			return 1
		}
	}
	return 0
}

func matchLocation(j dumpJob, location string) bool {
	loc := strings.ToLower(strings.TrimSpace(location))
	if loc == "" {
		return true
	}
	if loc == "remote" {
		return j.isRemote || strings.Contains(strings.ToLower(j.locations), "remote")
	}
	return strings.Contains(strings.ToLower(j.locations), loc)
}

func matchFilters(j dumpJob, filters map[string][]string) bool {
	for key, values := range filters {
		actual := j.fields[key]
		if actual == "" {
			return false
		}
		hit := false
		for _, v := range values {
			if strings.EqualFold(actual, v) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// validateFilterKeys rejects unknown dimensions up front with a teaching
// error, instead of silently matching nothing.
func validateFilterKeys(jobs []dumpJob, filters map[string][]string) error {
	if len(filters) == 0 {
		return nil
	}
	valid := make(map[string]bool)
	for _, j := range jobs {
		for k := range j.fields {
			valid[k] = true
		}
	}
	for key := range filters {
		if !valid[key] {
			keys := make([]string, 0, len(valid))
			for k := range valid {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return fmt.Errorf("unknown filter key %q; valid keys: %s", key, strings.Join(keys, ", "))
		}
	}
	return nil
}

// distinctFilters enumerates a dump's structured dimensions — the
// full-dump family's implementation of get_filters.
func distinctFilters(jobs []dumpJob) FilterSet {
	seen := make(map[string]map[string]bool)
	for _, j := range jobs {
		for k, v := range j.fields {
			if v == "" {
				continue
			}
			if seen[k] == nil {
				seen[k] = make(map[string]bool)
			}
			seen[k][v] = true
		}
	}
	fs := make(FilterSet, len(seen))
	for k, values := range seen {
		list := make([]string, 0, len(values))
		for v := range values {
			list = append(list, v)
		}
		sort.Strings(list)
		fs[k] = list
	}
	return fs
}
```

- [ ] **Step 4: 跑測試確認通過**

Run: `go test ./internal/ats/ -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ats/filter.go internal/ats/filter_test.go
git commit -m "feat(ats): add shared filter engine for full-dump providers"
```

---

### Task 4: Lever adapter

**Files:**
- Create: `internal/ats/lever.go`
- Test: `internal/ats/lever_test.go`

**Interfaces:**
- Consumes: Task 1 型別、Task 3 `dumpJob`/`searchDump`/`distinctFilters`;provider 端 `lever.NewClient`、`lever.WithClient`、`(*lever.Client).ListPostings/GetPosting`、`lever.Companies`、`lever.CompaniesBySite`、`lever.ListPostingsModeJSON`、`lever.NewMockServer`
- Produces: `NewLeverAdapter(baseURL string, hc *http.Client) (*LeverAdapter, error)`,`*LeverAdapter` 實作 `Adapter`(Task 8 wiring 依賴)

- [ ] **Step 1: 寫失敗測試**

`internal/ats/lever_test.go`:

```go
package ats

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/lever"
)

func testLeverAdapter(t *testing.T) *LeverAdapter {
	t.Helper()
	srv := lever.NewMockServer()
	t.Cleanup(srv.Close)
	a, err := NewLeverAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestLeverRoster(t *testing.T) {
	a := testLeverAdapter(t)
	roster := a.Roster()
	if len(roster) != len(lever.Companies) {
		t.Fatalf("roster len = %d, want %d", len(roster), len(lever.Companies))
	}
	for _, c := range roster {
		if c.Slug == "" || c.Name == "" {
			t.Fatalf("roster entry with empty field: %+v", c)
		}
	}
}

func TestLeverSearchAll(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(context.Background(), "leverdemo", SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 3 || len(res.Jobs) != 3 {
		t.Fatalf("got %d/%d jobs, want 3", len(res.Jobs), res.TotalCount)
	}
	for _, j := range res.Jobs {
		if j.JobID == "" || j.Title == "" || j.URL == "" || j.PostedAt == "" {
			t.Fatalf("summary with empty field: %+v", j)
		}
	}
}

func TestLeverSearchQuery(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(context.Background(), "leverdemo", SearchParams{Query: "AbelsonTaylor"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) < 1 || res.Jobs[0].Title != "AbelsonTaylor Writer" {
		t.Fatalf("got %+v, want AbelsonTaylor Writer first", res.Jobs)
	}
}

func TestLeverSearchFilters(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(context.Background(), "leverdemo", SearchParams{
		Filters: map[string][]string{"team": {"Professional Services"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) == 0 {
		t.Fatal("want at least one Professional Services job")
	}
}

func TestLeverFilters(t *testing.T) {
	a := testLeverAdapter(t)
	fs, err := a.Filters(context.Background(), "leverdemo")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range fs["team"] {
		if v == "Professional Services" {
			found = true
		}
	}
	if !found {
		t.Fatalf(`fs["team"] = %v, want it to contain "Professional Services"`, fs["team"])
	}
}

func TestLeverDetail(t *testing.T) {
	a := testLeverAdapter(t)
	d, err := a.Detail(context.Background(), "leverdemo", "33538a2f-d27d-4a96-8f05-fa4b0e4d940e")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "AbelsonTaylor Writer" {
		t.Errorf("Title = %q", d.Title)
	}
	if !strings.Contains(d.Description, "Welcome to the Demo") {
		t.Errorf("Description should contain the fixture opening, got %.80q", d.Description)
	}
	if strings.Contains(d.Description, "<") {
		t.Errorf("Description should be plain text, got %.80q", d.Description)
	}
}

func TestLeverDetailNotFound(t *testing.T) {
	a := testLeverAdapter(t)
	if _, err := a.Detail(context.Background(), "leverdemo", lever.MockNotFoundPostingID); err == nil {
		t.Fatal("want error for unknown posting id")
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./internal/ats/ -run TestLever -v`
Expected: FAIL(`undefined: NewLeverAdapter`)

- [ ] **Step 3: 實作 lever.go**

```go
package ats

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/lever"
)

// LeverAdapter serves Lever-hosted companies. Lever's list endpoint dumps
// the whole board (its native filter params are exact-match only, verified
// useless for fuzzy search), so searching happens in searchDump.
type LeverAdapter struct {
	client *lever.Client
}

func NewLeverAdapter(baseURL string, hc *http.Client) (*LeverAdapter, error) {
	c, err := lever.NewClient(baseURL, lever.WithClient(hc))
	if err != nil {
		return nil, err
	}
	return &LeverAdapter{client: c}, nil
}

func (a *LeverAdapter) Name() string { return "lever" }

func (a *LeverAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(lever.Companies))
	for _, c := range lever.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Site, Name: c.Name})
	}
	return infos
}

func (a *LeverAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *LeverAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *LeverAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	p, err := a.client.GetPosting(ctx, lever.GetPostingParams{Site: slug, PostingId: jobID})
	if err != nil {
		return nil, fmt.Errorf("lever: fetch posting %q for %q: %w", jobID, slug, err)
	}
	desc, err := leverDescription(p)
	if err != nil {
		return nil, err
	}
	return &JobDetail{
		JobID:       p.ID,
		Title:       p.Text,
		Company:     lever.CompaniesBySite[slug].Name,
		Location:    leverLocations(p),
		PostedAt:    leverPostedAt(p),
		URL:         p.HostedUrl.Value,
		Description: desc,
	}, nil
}

// dump fetches the full board and reshapes it for the filter engine.
func (a *LeverAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	postings, err := a.client.ListPostings(ctx, lever.ListPostingsParams{
		Site: slug,
		Mode: lever.ListPostingsModeJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("lever: list postings for %q: %w", slug, err)
	}
	jobs := make([]dumpJob, 0, len(postings))
	for _, p := range postings {
		cat := p.Categories.Value
		fields := map[string]string{}
		if cat.Team.Value != "" {
			fields["team"] = cat.Team.Value
		}
		if cat.Commitment.Value != "" {
			fields["commitment"] = cat.Commitment.Value
		}
		if p.WorkplaceType.Value != "" {
			fields["workplaceType"] = p.WorkplaceType.Value
		}
		posted := time.UnixMilli(p.CreatedAt.Value).UTC()
		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    p.ID,
				Title:    p.Text,
				Location: cat.Location.Value,
				PostedAt: posted.Format("2006-01-02"),
				URL:      p.HostedUrl.Value,
			},
			sortKey:     posted,
			title:       p.Text,
			orgUnit:     cat.Team.Value + " " + cat.Department.Value,
			description: p.DescriptionPlain.Value,
			locations:   cat.Location.Value + " " + strings.Join(cat.AllLocations, " "),
			fields:      fields,
			isRemote:    strings.EqualFold(p.WorkplaceType.Value, "remote"),
		})
	}
	return jobs, nil
}

// leverDescription assembles the full plain-text JD from Lever's sectioned
// fields: opening+body (descriptionPlain), then each list section (its
// content is HTML), then the closing (additionalPlain).
func leverDescription(p *lever.Posting) (string, error) {
	var parts []string
	if s := strings.TrimSpace(p.DescriptionPlain.Value); s != "" {
		parts = append(parts, s)
	}
	for _, l := range p.Lists {
		content, err := html2text.FromString(l.Content, html2text.Options{})
		if err != nil {
			return "", fmt.Errorf("lever: convert list section %q: %w", l.Text, err)
		}
		parts = append(parts, l.Text+"\n"+content)
	}
	if s := strings.TrimSpace(p.AdditionalPlain.Value); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n\n"), nil
}

func leverLocations(p *lever.Posting) string {
	cat := p.Categories.Value
	if len(cat.AllLocations) > 0 {
		return strings.Join(cat.AllLocations, "; ")
	}
	return cat.Location.Value
}

func leverPostedAt(p *lever.Posting) string {
	if !p.CreatedAt.Set {
		return ""
	}
	return time.UnixMilli(p.CreatedAt.Value).UTC().Format("2006-01-02")
}
```

- [ ] **Step 4: 跑測試確認通過**

Run: `go test ./internal/ats/ -run TestLever -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ats/lever.go internal/ats/lever_test.go
git commit -m "feat(ats): add lever adapter"
```

---

### Task 5: Ashby adapter

**Files:**
- Create: `internal/ats/ashby.go`
- Test: `internal/ats/ashby_test.go`

**Interfaces:**
- Consumes: Task 1 型別、Task 3 引擎;provider 端 `ashby.NewClient`、`ashby.WithClient`、`(*ashby.Client).GetJobBoard`(union 回傳:`*ashby.JobBoardResponse` / `*ashby.GetJobBoardNotFound`)、`ashby.Companies`、`ashby.CompaniesByBoard`、`ashby.NewMockServer`、`ashby.MockBoardName`
- Produces: `NewAshbyAdapter(baseURL string, hc *http.Client) (*AshbyAdapter, error)`,`*AshbyAdapter` 實作 `Adapter`

- [ ] **Step 1: 寫失敗測試**

`internal/ats/ashby_test.go`:

```go
package ats

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/ashby"
)

func testAshbyAdapter(t *testing.T) *AshbyAdapter {
	t.Helper()
	srv := ashby.NewMockServer()
	t.Cleanup(srv.Close)
	a, err := NewAshbyAdapter(srv.URL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAshbyRoster(t *testing.T) {
	a := testAshbyAdapter(t)
	if got, want := len(a.Roster()), len(ashby.Companies); got != want {
		t.Fatalf("roster len = %d, want %d", got, want)
	}
}

func TestAshbySearchAll(t *testing.T) {
	a := testAshbyAdapter(t)
	res, err := a.Search(context.Background(), ashby.MockBoardName, SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCount != 5 {
		t.Fatalf("TotalCount = %d, want 5 (all fixture jobs are listed)", res.TotalCount)
	}
	for _, j := range res.Jobs {
		if j.JobID == "" || j.Title == "" || j.URL == "" {
			t.Fatalf("summary with empty field: %+v", j)
		}
	}
}

func TestAshbySearchQueryAndFilters(t *testing.T) {
	a := testAshbyAdapter(t)
	res, err := a.Search(context.Background(), ashby.MockBoardName, SearchParams{Query: "agent platform"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Jobs) == 0 || res.Jobs[0].Title != "Software Engineer (Agent Platform)" {
		t.Fatalf("got %+v, want the Agent Platform job first", res.Jobs)
	}

	filtered, err := a.Search(context.Background(), ashby.MockBoardName, SearchParams{
		Filters: map[string][]string{"employmentType": {"FullTime"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Jobs) == 0 {
		t.Fatal("want at least one FullTime job")
	}
}

func TestAshbyFilters(t *testing.T) {
	a := testAshbyAdapter(t)
	fs, err := a.Filters(context.Background(), ashby.MockBoardName)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"department", "employmentType", "workplaceType"} {
		if len(fs[key]) == 0 {
			t.Errorf("FilterSet missing %q: %v", key, fs)
		}
	}
}

func TestAshbyDetailRefetchesBoard(t *testing.T) {
	a := testAshbyAdapter(t)
	d, err := a.Detail(context.Background(), ashby.MockBoardName, "7724fbe3-6a27-4418-9705-2dcc40751a16")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Software Engineer (Agent Platform)" {
		t.Errorf("Title = %q", d.Title)
	}
	if d.Description == "" {
		t.Error("Description should be non-empty plain text")
	}
}

func TestAshbyDetailNotFound(t *testing.T) {
	a := testAshbyAdapter(t)
	if _, err := a.Detail(context.Background(), ashby.MockBoardName, "no-such-id"); err == nil {
		t.Fatal("want error for unknown job id")
	}
}

func TestAshbyUnknownBoardUpstream(t *testing.T) {
	a := testAshbyAdapter(t)
	if _, err := a.Search(context.Background(), "not-in-mock", SearchParams{}); err == nil {
		t.Fatal("want error when upstream returns 404")
	}
}

func TestAshbySearchIsDeterministic(t *testing.T) {
	a := testAshbyAdapter(t)
	r1, _ := a.Search(context.Background(), ashby.MockBoardName, SearchParams{})
	r2, _ := a.Search(context.Background(), ashby.MockBoardName, SearchParams{})
	for i := range r1.Jobs {
		if r1.Jobs[i].JobID != r2.Jobs[i].JobID {
			t.Fatal("search order is not deterministic")
		}
	}
	if !strings.HasPrefix(r1.Jobs[0].PostedAt, "20") {
		t.Errorf("PostedAt should be an ISO date, got %q", r1.Jobs[0].PostedAt)
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./internal/ats/ -run TestAshby -v`
Expected: FAIL(`undefined: NewAshbyAdapter`)

- [ ] **Step 3: 實作 ashby.go**

```go
package ats

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/amikai/openings-mcp/internal/provider/ashby"
)

// AshbyAdapter serves Ashby-hosted companies. Ashby's public API is a
// single full-board endpoint — no server-side search and no per-job
// endpoint (returns 401) — so Search filters the dump via searchDump and
// Detail refetches the board and picks the one job out. The refetch is a
// bandwidth cost between this server and Ashby, invisible to the client.
type AshbyAdapter struct {
	client *ashby.Client
}

func NewAshbyAdapter(baseURL string, hc *http.Client) (*AshbyAdapter, error) {
	c, err := ashby.NewClient(baseURL, ashby.WithClient(hc))
	if err != nil {
		return nil, err
	}
	return &AshbyAdapter{client: c}, nil
}

func (a *AshbyAdapter) Name() string { return "ashby" }

func (a *AshbyAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(ashby.Companies))
	for _, c := range ashby.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Board, Name: c.Name})
	}
	return infos
}

func (a *AshbyAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *AshbyAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *AshbyAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	board, err := a.board(ctx, slug)
	if err != nil {
		return nil, err
	}
	for _, j := range board.Jobs {
		if j.ID.Value != jobID {
			continue
		}
		return &JobDetail{
			JobID:       j.ID.Value,
			Title:       j.Title,
			Company:     ashby.CompaniesByBoard[slug].Name,
			Location:    ashbyLocations(j),
			PostedAt:    j.PublishedAt.UTC().Format("2006-01-02"),
			URL:         j.JobUrl,
			Description: j.DescriptionPlain.Value,
		}, nil
	}
	return nil, fmt.Errorf("ashby: job %q not found for company %q; pass the job_id returned by search_jobs_by_company", jobID, slug)
}

// board fetches the full job board, unwrapping ogen's union response.
func (a *AshbyAdapter) board(ctx context.Context, slug string) (*ashby.JobBoardResponse, error) {
	res, err := a.client.GetJobBoard(ctx, ashby.GetJobBoardParams{JobBoardName: slug})
	if err != nil {
		return nil, fmt.Errorf("ashby: fetch board %q: %w", slug, err)
	}
	switch r := res.(type) {
	case *ashby.JobBoardResponse:
		return r, nil
	case *ashby.GetJobBoardNotFound:
		return nil, fmt.Errorf("ashby: board %q not found upstream", slug)
	default:
		return nil, fmt.Errorf("ashby: unexpected response type %T", res)
	}
}

func (a *AshbyAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	board, err := a.board(ctx, slug)
	if err != nil {
		return nil, err
	}
	jobs := make([]dumpJob, 0, len(board.Jobs))
	for _, j := range board.Jobs {
		if !j.IsListed {
			continue
		}
		fields := map[string]string{}
		if j.Department.Value != "" {
			fields["department"] = j.Department.Value
		}
		if j.Team.Value != "" {
			fields["team"] = j.Team.Value
		}
		if string(j.EmploymentType) != "" {
			fields["employmentType"] = string(j.EmploymentType)
		}
		if string(j.WorkplaceType) != "" {
			fields["workplaceType"] = string(j.WorkplaceType)
		}
		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    j.ID.Value,
				Title:    j.Title,
				Location: j.Location.Value,
				PostedAt: j.PublishedAt.UTC().Format("2006-01-02"),
				URL:      j.JobUrl,
			},
			sortKey:     j.PublishedAt,
			title:       j.Title,
			orgUnit:     j.Department.Value + " " + j.Team.Value,
			description: j.DescriptionPlain.Value,
			locations:   ashbyLocations(j),
			fields:      fields,
			isRemote:    j.IsRemote,
		})
	}
	return jobs, nil
}

func ashbyLocations(j ashby.JobPosting) string {
	parts := []string{j.Location.Value}
	for _, s := range j.SecondaryLocations {
		parts = append(parts, s.Location.Value)
	}
	return strings.Join(parts, "; ")
}
```

注意:`SecondaryLocation` 的欄位名以 `internal/provider/ashby/oas_schemas_gen.go:1053` 起的定義為準,實作時確認(預期是 `Location OptString`;若不同,改用實際欄位)。

- [ ] **Step 4: 跑測試確認通過**

Run: `go test ./internal/ats/ -run TestAshby -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ats/ashby.go internal/ats/ashby_test.go
git commit -m "feat(ats): add ashby adapter"
```

---

### Task 6: Workday adapter

**Files:**
- Create: `internal/ats/workday.go`
- Test: `internal/ats/workday_test.go`

**Interfaces:**
- Consumes: Task 1 型別;provider 端 `workday.NewClient`、`workday.WithClient`、`(*workday.Client).SearchJobs/GetJobDetail`、`workday.JobsRequest`、`workday.AppliedFacets`、`workday.FacetNode`、`workday.GetJobDetailParams`、`workday.Companies`、`workday.CompaniesByTenant`、`workday.Company.BaseURL`、`workday.SplitExternalPath`、`workday.PublicSiteURL`、`workday.NewMockServer`
- Produces: `NewWorkdayAdapter(hc *http.Client) *WorkdayAdapter`,`*WorkdayAdapter` 實作 `Adapter`;struct 有未匯出欄位 `baseURL func(workday.Company) string`(測試覆寫用)

- [ ] **Step 1: 寫失敗測試**

`internal/ats/workday_test.go`。mock server 只認固定路由,測試把 `baseURL` 覆寫成 mock URL;另外用一層錄音 proxy 驗證「二段請求」的次數與第二次請求帶的 `appliedFacets`:

```go
package ats

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/workday"
)

// recordingProxy forwards every request to inner and keeps the bodies, so
// tests can assert how many upstream calls a Search made and what the real
// (second) search request contained.
func recordingProxy(t *testing.T, inner string) (*httptest.Server, *[][]byte) {
	t.Helper()
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, body)
		req, err := http.NewRequestWithContext(r.Context(), r.Method, inner+r.URL.Path, strings.NewReader(string(body)))
		if err != nil {
			t.Errorf("proxy: %v", err)
			return
		}
		req.Header = r.Header.Clone()
		rsp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("proxy: %v", err)
			return
		}
		defer rsp.Body.Close()
		w.Header().Set("Content-Type", rsp.Header.Get("Content-Type"))
		w.WriteHeader(rsp.StatusCode)
		io.Copy(w, rsp.Body)
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

func testWorkdayAdapter(t *testing.T) (*WorkdayAdapter, *[][]byte) {
	t.Helper()
	mock := workday.NewMockServer()
	t.Cleanup(mock.Close)
	proxy, bodies := recordingProxy(t, mock.URL)
	a := NewWorkdayAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(workday.Company) string { return proxy.URL }
	return a, bodies
}

func TestWorkdayRosterDedupesShareClasses(t *testing.T) {
	a := NewWorkdayAdapter(http.DefaultClient)
	seen := map[string]bool{}
	for _, c := range a.Roster() {
		if seen[c.Slug] {
			t.Fatalf("duplicate slug %q in roster", c.Slug)
		}
		seen[c.Slug] = true
	}
	// fox and dowjones each occupy two share-class rows in companies.yaml
	// sharing one tenant; the roster must carry each slug once.
	if !seen["fox"] || !seen["dowjones"] {
		t.Fatal("expected fox and dowjones slugs present exactly once")
	}
}

func TestWorkdaySearchPlainIsOneRequest(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	res, err := a.Search(context.Background(), "nvidia", SearchParams{Query: "golang"})
	if err != nil {
		t.Fatal(err)
	}
	if len(*bodies) != 1 {
		t.Fatalf("plain search should be 1 upstream request, got %d", len(*bodies))
	}
	if res.TotalCount != 27 || res.TotalPages != 2 || len(res.Jobs) != 20 {
		t.Fatalf("got {total %d, pages %d, len %d}, want {27, 2, 20}", res.TotalCount, res.TotalPages, len(res.Jobs))
	}
	first := res.Jobs[0]
	if first.Title != "Software Golang Kubernetes Engineer" {
		t.Errorf("Title = %q", first.Title)
	}
	if first.JobID != "/job/Israel-Tel-Aviv/Software-Golang-Kubernetes-Engineer_JR2020442" {
		t.Errorf("JobID = %q", first.JobID)
	}
}

func TestWorkdaySearchWithFiltersIsTwoRequests(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	_, err := a.Search(context.Background(), "nvidia", SearchParams{
		Filters: map[string][]string{"timeType": {"Full time"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(*bodies) != 2 {
		t.Fatalf("filtered search should probe then search (2 requests), got %d", len(*bodies))
	}
	var real struct {
		AppliedFacets map[string][]string `json:"appliedFacets"`
	}
	if err := json.Unmarshal((*bodies)[1], &real); err != nil {
		t.Fatal(err)
	}
	if got := real.AppliedFacets["timeType"]; len(got) != 1 || got[0] != "5509c0b5959810ac0029943377d47364" {
		t.Fatalf("appliedFacets[timeType] = %v, want the Full time GUID", got)
	}
}

func TestWorkdaySearchLocationResolvesToFacet(t *testing.T) {
	a, bodies := testWorkdayAdapter(t)
	_, err := a.Search(context.Background(), "nvidia", SearchParams{Location: "Tel Aviv"})
	if err != nil {
		t.Fatal(err)
	}
	var real struct {
		AppliedFacets map[string][]string `json:"appliedFacets"`
	}
	if err := json.Unmarshal((*bodies)[1], &real); err != nil {
		t.Fatal(err)
	}
	if got := real.AppliedFacets["locations"]; len(got) != 1 || got[0] != "c7769ee377291036b08490819096b8bf" {
		t.Fatalf(`appliedFacets[locations] = %v, want the "Israel, Tel Aviv" GUID`, got)
	}
}

func TestWorkdayFilterValueNotFoundTeaches(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	_, err := a.Search(context.Background(), "nvidia", SearchParams{
		Filters: map[string][]string{"timeType": {"Part time"}},
	})
	if err == nil {
		t.Fatal("want error for unknown facet value")
	}
	if !strings.Contains(err.Error(), "Full time") {
		t.Errorf("error should list available values, got: %v", err)
	}
}

func TestWorkdayFilterKeyNotFoundTeaches(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	_, err := a.Search(context.Background(), "nvidia", SearchParams{
		Filters: map[string][]string{"bogus": {"x"}},
	})
	if err == nil {
		t.Fatal("want error for unknown facet key")
	}
	if !strings.Contains(err.Error(), "jobFamilyGroup") {
		t.Errorf("error should list valid keys, got: %v", err)
	}
}

func TestWorkdayFilters(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	fs, err := a.Filters(context.Background(), "nvidia")
	if err != nil {
		t.Fatal(err)
	}
	if len(fs["jobFamilyGroup"]) == 0 || len(fs["timeType"]) == 0 {
		t.Fatalf("FilterSet missing expected dimensions: %v", fs)
	}
	found := false
	for _, v := range fs["jobFamilyGroup"] {
		if v == "Engineering" {
			found = true
		}
	}
	if !found {
		t.Errorf(`fs["jobFamilyGroup"] = %v, want it to contain "Engineering"`, fs["jobFamilyGroup"])
	}
}

func TestWorkdayDetail(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	d, err := a.Detail(context.Background(), "nvidia", "/job/Israel-Tel-Aviv/Software-Golang-Kubernetes-Engineer_JR2020442")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Senior Software Golang Kubernetes Engineer" {
		t.Errorf("Title = %q", d.Title)
	}
	if strings.Contains(d.Description, "<p>") {
		t.Errorf("Description should be converted from HTML, got %.80q", d.Description)
	}
	if !strings.Contains(d.Description, "NVIDIA Networking") {
		t.Errorf("Description should carry the fixture text, got %.80q", d.Description)
	}
}

func TestWorkdayDetailRejectsMalformedJobID(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	if _, err := a.Detail(context.Background(), "nvidia", "garbage"); err == nil {
		t.Fatal("want error for malformed job_id")
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./internal/ats/ -run TestWorkday -v`
Expected: FAIL(`undefined: NewWorkdayAdapter`)

- [ ] **Step 3: 實作 workday.go**

```go
package ats

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/workday"
)

// WorkdayAdapter serves Workday CXS tenants. Search runs server-side;
// location and filters name facet labels, which the adapter resolves to
// tenant-specific GUIDs via a probe request (appliedFacets wants GUIDs but
// get_filters reports labels — the stateless price is one extra upstream
// call whenever location or filters are set).
type WorkdayAdapter struct {
	hc *http.Client
	// baseURL derives a tenant's CXS base URL; tests point it at a mock.
	baseURL func(workday.Company) string
}

func NewWorkdayAdapter(hc *http.Client) *WorkdayAdapter {
	return &WorkdayAdapter{hc: hc, baseURL: workday.Company.BaseURL}
}

func (a *WorkdayAdapter) Name() string { return "workday" }

// Roster dedupes by tenant slug: fox and dowjones each hold two
// share-class rows in companies.yaml sharing one tenant, and the registry
// treats duplicate slugs as curation bugs.
func (a *WorkdayAdapter) Roster() []CompanyInfo {
	seen := make(map[string]bool, len(workday.Companies))
	infos := make([]CompanyInfo, 0, len(workday.Companies))
	for _, c := range workday.Companies {
		slug := strings.ToLower(c.Tenant)
		if seen[slug] {
			continue
		}
		seen[slug] = true
		infos = append(infos, CompanyInfo{Slug: slug, Name: c.Name})
	}
	return infos
}

func (a *WorkdayAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	client, company, err := a.client(slug)
	if err != nil {
		return nil, err
	}
	applied := workday.AppliedFacets{}
	if p.Location != "" || len(p.Filters) > 0 {
		flat, err := a.probeFacets(ctx, client, slug)
		if err != nil {
			return nil, err
		}
		applied, err = resolveFacets(flat, p.Location, p.Filters)
		if err != nil {
			return nil, err
		}
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	rsp, err := client.SearchJobs(ctx, &workday.JobsRequest{
		AppliedFacets: applied,
		Limit:         PageSize,
		Offset:        (page - 1) * PageSize,
		SearchText:    p.Query,
	})
	if err != nil {
		return nil, fmt.Errorf("workday: search %q: %w", slug, err)
	}

	// Public posting URLs derive from the tenant's career-site origin;
	// derivation can fail only on malformed base URLs (e.g. a test mock),
	// in which case summaries simply omit URLs.
	publicURL, pubErr := workday.PublicSiteURL(a.baseURL(company))
	jobs := make([]JobSummary, 0, len(rsp.JobPostings))
	for _, js := range rsp.JobPostings {
		path := js.ExternalPath.Value
		if path == "" {
			// Transient posting with no fetchable path; skip rather than
			// hand out a job_id that can't be detailed.
			continue
		}
		url := ""
		if pubErr == nil {
			url = publicURL + path
		}
		jobs = append(jobs, JobSummary{
			JobID:    path,
			Title:    js.Title.Value,
			Location: js.LocationsText.Value,
			PostedAt: js.PostedOn.Value,
			URL:      url,
		})
	}
	total := rsp.Total
	return &SearchResult{
		Jobs:       jobs,
		TotalCount: total,
		Page:       page,
		TotalPages: (total + PageSize - 1) / PageSize,
	}, nil
}

func (a *WorkdayAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	client, _, err := a.client(slug)
	if err != nil {
		return nil, err
	}
	flat, err := a.probeFacets(ctx, client, slug)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]map[string]bool)
	for _, f := range flat {
		if seen[f.param] == nil {
			seen[f.param] = make(map[string]bool)
		}
		seen[f.param][f.label] = true
	}
	fs := make(FilterSet, len(seen))
	for param, labels := range seen {
		list := make([]string, 0, len(labels))
		for l := range labels {
			list = append(list, l)
		}
		sort.Strings(list)
		fs[param] = list
	}
	return fs, nil
}

func (a *WorkdayAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	client, company, err := a.client(slug)
	if err != nil {
		return nil, err
	}
	loc, titleSlug, ok := workday.SplitExternalPath(jobID)
	if !ok {
		return nil, fmt.Errorf("workday: invalid job_id %q; pass the job_id returned by search_jobs_by_company", jobID)
	}
	rsp, err := client.GetJobDetail(ctx, workday.GetJobDetailParams{Location: loc, TitleSlug: titleSlug})
	if err != nil {
		return nil, fmt.Errorf("workday: fetch job %q for %q: %w", jobID, slug, err)
	}
	info := rsp.JobPostingInfo
	desc, err := html2text.FromString(info.JobDescription, html2text.Options{})
	if err != nil {
		return nil, fmt.Errorf("workday: convert job description: %w", err)
	}
	location := info.Location.Value
	if len(info.AdditionalLocations) > 0 {
		location = strings.Join(append([]string{location}, info.AdditionalLocations...), "; ")
	}
	return &JobDetail{
		JobID:       jobID,
		Title:       info.Title,
		Company:     company.Name,
		Location:    location,
		PostedAt:    info.PostedOn.Value,
		URL:         info.ExternalUrl.Value,
		Description: desc,
	}, nil
}

// client builds a per-tenant CXS client on demand. The wrapper is
// stateless and cheap; connection pooling lives in the shared http.Client.
func (a *WorkdayAdapter) client(slug string) (*workday.Client, workday.Company, error) {
	company, ok := workday.CompaniesByTenant[slug]
	if !ok {
		// Registry slugs come from this adapter's own Roster, so a miss is
		// an internal inconsistency, not user error.
		return nil, workday.Company{}, fmt.Errorf("workday: tenant %q not in roster", slug)
	}
	c, err := workday.NewClient(a.baseURL(company), workday.WithClient(a.hc))
	if err != nil {
		return nil, workday.Company{}, err
	}
	return c, company, nil
}

// flatFacet is one facet leaf attributed to its nearest ancestor group
// carrying a facetParameter (groups nest, e.g. locationMainGroup wraps
// locationHierarchy1 and locations).
type flatFacet struct {
	param string
	label string
	id    string
}

// probeFacets fetches the tenant's complete current facet tree with a
// minimal unfiltered search (searchText narrows the tree as much as a
// facet filter does, so the probe sends neither).
func (a *WorkdayAdapter) probeFacets(ctx context.Context, client *workday.Client, slug string) ([]flatFacet, error) {
	rsp, err := client.SearchJobs(ctx, &workday.JobsRequest{
		AppliedFacets: workday.AppliedFacets{},
		Limit:         1,
	})
	if err != nil {
		return nil, fmt.Errorf("workday: probe facets for %q: %w", slug, err)
	}
	nodes, ok := rsp.Facets.Get()
	if !ok || len(nodes) == 0 {
		return nil, fmt.Errorf("workday: company %q reports no filter dimensions; retry without location/filters", slug)
	}
	return flattenFacets(nodes), nil
}

func flattenFacets(nodes []workday.FacetNode) []flatFacet {
	var out []flatFacet
	var walk func(n workday.FacetNode, param string)
	walk = func(n workday.FacetNode, param string) {
		if n.FacetParameter.Set {
			param = n.FacetParameter.Value
		}
		if len(n.Values) == 0 {
			if param != "" && n.ID.Set && n.Descriptor.Set {
				out = append(out, flatFacet{param: param, label: n.Descriptor.Value, id: n.ID.Value})
			}
			return
		}
		for _, c := range n.Values {
			walk(c, param)
		}
	}
	for _, n := range nodes {
		walk(n, "")
	}
	return out
}

// resolveFacets turns unified location/filter inputs into appliedFacets
// GUIDs, failing with teaching errors that name the valid alternatives.
func resolveFacets(flat []flatFacet, location string, filters map[string][]string) (workday.AppliedFacets, error) {
	applied := workday.AppliedFacets{}
	if location != "" {
		param, ids, err := resolveLocationFacet(flat, location)
		if err != nil {
			return nil, err
		}
		applied[param] = ids
	}
	for key, values := range filters {
		ids, err := resolveFacetValues(flat, key, values)
		if err != nil {
			return nil, err
		}
		applied[key] = append(applied[key], ids...)
	}
	return applied, nil
}

// resolveLocationFacet fuzzy-matches the location text against every
// location-flavored facet leaf (params prefixed "location"), then applies
// the single param with the most hits — mixing params would AND them and
// over-constrain.
func resolveLocationFacet(flat []flatFacet, location string) (string, []string, error) {
	loc := strings.ToLower(strings.TrimSpace(location))
	hits := make(map[string][]string)
	for _, f := range flat {
		if !strings.HasPrefix(strings.ToLower(f.param), "location") {
			continue
		}
		if strings.Contains(strings.ToLower(f.label), loc) {
			hits[f.param] = append(hits[f.param], f.id)
		}
	}
	if len(hits) == 0 {
		return "", nil, fmt.Errorf("no location matching %q; call get_filters_by_company to see available location values", location)
	}
	params := make([]string, 0, len(hits))
	for p := range hits {
		params = append(params, p)
	}
	sort.Slice(params, func(i, j int) bool {
		if len(hits[params[i]]) != len(hits[params[j]]) {
			return len(hits[params[i]]) > len(hits[params[j]])
		}
		return params[i] < params[j]
	})
	return params[0], hits[params[0]], nil
}

// resolveFacetValues maps display labels to GUIDs within one facet param,
// matching labels case-insensitively.
func resolveFacetValues(flat []flatFacet, key string, values []string) ([]string, error) {
	byLabel := make(map[string]string)
	var labels []string
	params := make(map[string]bool)
	for _, f := range flat {
		params[f.param] = true
		if f.param != key {
			continue
		}
		lower := strings.ToLower(f.label)
		if _, ok := byLabel[lower]; !ok {
			byLabel[lower] = f.id
			labels = append(labels, f.label)
		}
	}
	if len(byLabel) == 0 {
		keys := make([]string, 0, len(params))
		for p := range params {
			keys = append(keys, p)
		}
		sort.Strings(keys)
		return nil, fmt.Errorf("unknown filter key %q; valid keys: %s", key, strings.Join(keys, ", "))
	}
	ids := make([]string, 0, len(values))
	for _, v := range values {
		id, ok := byLabel[strings.ToLower(v)]
		if !ok {
			sort.Strings(labels)
			const maxListed = 20
			listed := labels
			suffix := ""
			if len(listed) > maxListed {
				listed = listed[:maxListed]
				suffix = ", …"
			}
			return nil, fmt.Errorf("filter value %q not found for %q; available: %s%s", v, key, strings.Join(listed, ", "), suffix)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
```

- [ ] **Step 4: 跑測試確認通過**

Run: `go test ./internal/ats/ -v`
Expected: 全部 PASS(含前面 task 的測試)

- [ ] **Step 5: Commit**

```bash
git add internal/ats/workday.go internal/ats/workday_test.go
git commit -m "feat(ats): add workday adapter with label-to-GUID facet resolution"
```

---

### Task 7: MCP 層 — 三個 `*_by_company` 工具

**Files:**
- Create: `internal/openingsmcp/company.go`
- Test: `internal/openingsmcp/company_test.go`

**Interfaces:**
- Consumes: `ats.Registry`、`ats.Adapter` 及 Task 1 全部型別;既有 `errorResult`、`mustSchema`
- Produces: `RegisterCompany(s *mcp.Server, r *ats.Registry)`(Task 8 依賴)

- [ ] **Step 1: 寫失敗測試**

`internal/openingsmcp/company_test.go`。handler 邏輯抽成純函式(`companySearch`/`companyFilters`/`companyDetail`),測試直接打函式,不經 MCP transport(與既有 provider 工具檔的轉譯函式測試同精神):

```go
package openingsmcp

import (
	"context"
	"strings"
	"testing"

	"github.com/amikai/openings-mcp/internal/ats"
)

// stubAdapter returns canned results so tests exercise only the MCP
// translation layer.
type stubAdapter struct {
	searchResult *ats.SearchResult
	filterSet    ats.FilterSet
	detail       *ats.JobDetail
	gotParams    ats.SearchParams
}

func (s *stubAdapter) Name() string { return "stub" }
func (s *stubAdapter) Roster() []ats.CompanyInfo {
	return []ats.CompanyInfo{{Slug: "acme", Name: "Acme Corp"}}
}
func (s *stubAdapter) Search(_ context.Context, _ string, p ats.SearchParams) (*ats.SearchResult, error) {
	s.gotParams = p
	return s.searchResult, nil
}
func (s *stubAdapter) Filters(context.Context, string) (ats.FilterSet, error) {
	return s.filterSet, nil
}
func (s *stubAdapter) Detail(context.Context, string, string) (*ats.JobDetail, error) {
	return s.detail, nil
}

func testCompanyRegistry(t *testing.T, stub *stubAdapter) *ats.Registry {
	t.Helper()
	r, err := ats.NewRegistry(stub)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCompanySearchMapsParamsAndResult(t *testing.T) {
	stub := &stubAdapter{searchResult: &ats.SearchResult{
		Jobs: []ats.JobSummary{{
			JobID: "j1", Title: "Engineer", Location: "Taipei", PostedAt: "2026-07-01", URL: "https://x/j1",
		}},
		TotalCount: 41, Page: 2, TotalPages: 3,
	}}
	reg := testCompanyRegistry(t, stub)

	out, err := companySearch(context.Background(), reg, &companySearchInput{
		Company:  "Acme Corp",
		Query:    "golang",
		Location: "taipei",
		Filters:  map[string][]string{"team": {"Platform"}},
		Page:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.gotParams.Query != "golang" || stub.gotParams.Page != 2 || stub.gotParams.Filters["team"][0] != "Platform" {
		t.Errorf("params not forwarded: %+v", stub.gotParams)
	}
	if out.TotalCount != 41 || out.Page != 2 || out.TotalPages != 3 || len(out.Data) != 1 {
		t.Errorf("result not mapped: %+v", out)
	}
	if out.Data[0].JobID != "j1" || out.Data[0].URL != "https://x/j1" {
		t.Errorf("summary not mapped: %+v", out.Data[0])
	}
}

func TestCompanySearchUnknownCompanyTeaches(t *testing.T) {
	reg := testCompanyRegistry(t, &stubAdapter{})
	_, err := companySearch(context.Background(), reg, &companySearchInput{Company: "acme corp intl"})
	if err == nil {
		t.Fatal("want teaching error")
	}
	if !strings.Contains(err.Error(), "acme") {
		t.Errorf("error should suggest acme, got: %v", err)
	}
}

func TestCompanyFilters(t *testing.T) {
	stub := &stubAdapter{filterSet: ats.FilterSet{"team": {"ML", "Web"}}}
	reg := testCompanyRegistry(t, stub)
	out, err := companyFilters(context.Background(), reg, &companyFiltersInput{Company: "acme"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Filters["team"]) != 2 {
		t.Errorf("filters not mapped: %+v", out)
	}
}

func TestCompanyDetail(t *testing.T) {
	stub := &stubAdapter{detail: &ats.JobDetail{JobID: "j1", Title: "Engineer", Company: "Acme Corp", Description: "plain text"}}
	reg := testCompanyRegistry(t, stub)
	out, err := companyDetail(context.Background(), reg, &companyDetailInput{Company: "acme", JobID: "j1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "Engineer" || out.Description != "plain text" {
		t.Errorf("detail not mapped: %+v", out)
	}
}
```

- [ ] **Step 2: 跑測試確認失敗**

Run: `go test ./internal/openingsmcp/ -run TestCompany -v`
Expected: FAIL(`undefined: companySearch`)

- [ ] **Step 3: 實作 company.go**

```go
package openingsmcp

import (
	"context"

	"github.com/amikai/openings-mcp/internal/ats"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The unified company tools front internal/ats: one company parameter, ATS
// invisible. Search input needs a hand-written schema because filters is
// an open map whose keys are tenant-specific and only known at runtime via
// get_filters_by_company.
var companySearchInputRawSchema = []byte(`{
	"type": "object",
	"properties": {
		"company": {
			"type": "string",
			"description": "Company name or slug, e.g. 'nvidia' or 'NVIDIA Corp'. If unknown, the error message suggests the closest supported companies."
		},
		"query": {
			"type": "string",
			"description": "Free-text keywords: role titles, skills, or technologies. Never put locations or employment types here."
		},
		"location": {
			"type": "string",
			"description": "Location as fuzzy text, e.g. 'Tel Aviv' or 'Taiwan'; 'remote' matches remote-friendly jobs. Omit to search everywhere."
		},
		"filters": {
			"type": "object",
			"description": "Optional precise filters. Keys and values are company-specific; discover them with get_filters_by_company. Multiple values for one key are OR'd; different keys are AND'd.",
			"additionalProperties": {
				"type": "array",
				"items": { "type": "string" }
			}
		},
		"page": {
			"type": "integer",
			"description": "1-based page number; each page returns at most 20 jobs.",
			"minimum": 1
		}
	},
	"required": ["company"],
	"additionalProperties": false
}`)

var companySearchInputSchema = mustSchema(companySearchInputRawSchema)

type companySearchInput struct {
	Company  string              `json:"company"`
	Query    string              `json:"query,omitempty"`
	Location string              `json:"location,omitempty"`
	Filters  map[string][]string `json:"filters,omitempty"`
	Page     int                 `json:"page,omitempty"`
}

type companyJobSummary struct {
	JobID    string `json:"job_id" jsonschema:"Opaque job identifier; pass to get_job_detail_by_company's job_id param."`
	Title    string `json:"title"`
	Location string `json:"location,omitempty"`
	PostedAt string `json:"posted_at,omitempty"`
	URL      string `json:"url,omitempty" jsonschema:"Public job posting URL."`
}

type companySearchOutput struct {
	Data       []companyJobSummary `json:"data"`
	TotalCount int                 `json:"total_count"`
	Page       int                 `json:"page"`
	TotalPages int                 `json:"total_pages"`
	// NextCursor is reserved for a future keyset-pagination upgrade; it is
	// always empty today.
	NextCursor string `json:"next_cursor,omitempty"`
}

func companySearch(ctx context.Context, reg *ats.Registry, in *companySearchInput) (*companySearchOutput, error) {
	adapter, slug, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	res, err := adapter.Search(ctx, slug, ats.SearchParams{
		Query:    in.Query,
		Location: in.Location,
		Filters:  in.Filters,
		Page:     in.Page,
	})
	if err != nil {
		return nil, err
	}
	out := &companySearchOutput{
		Data:       make([]companyJobSummary, 0, len(res.Jobs)),
		TotalCount: res.TotalCount,
		Page:       res.Page,
		TotalPages: res.TotalPages,
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
	return out, nil
}

type companyFiltersInput struct {
	Company string `json:"company" jsonschema:"Company name or slug, e.g. 'nvidia'."`
}

type companyFiltersOutput struct {
	Filters map[string][]string `json:"filters" jsonschema:"Filter dimension to its currently valid values. Pass any subset to search_jobs_by_company's filters param."`
}

func companyFilters(ctx context.Context, reg *ats.Registry, in *companyFiltersInput) (*companyFiltersOutput, error) {
	adapter, slug, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	fs, err := adapter.Filters(ctx, slug)
	if err != nil {
		return nil, err
	}
	return &companyFiltersOutput{Filters: fs}, nil
}

type companyDetailInput struct {
	Company string `json:"company" jsonschema:"Company name or slug, e.g. 'nvidia'."`
	JobID   string `json:"job_id" jsonschema:"job_id from search_jobs_by_company results."`
}

type companyDetailOutput struct {
	JobID       string `json:"job_id"`
	Title       string `json:"title"`
	Company     string `json:"company,omitempty"`
	Location    string `json:"location,omitempty"`
	PostedAt    string `json:"posted_at,omitempty"`
	URL         string `json:"url,omitempty" jsonschema:"Public job posting URL."`
	Description string `json:"description,omitempty" jsonschema:"Full job description as plain text."`
}

func companyDetail(ctx context.Context, reg *ats.Registry, in *companyDetailInput) (*companyDetailOutput, error) {
	adapter, slug, err := reg.Resolve(in.Company)
	if err != nil {
		return nil, err
	}
	d, err := adapter.Detail(ctx, slug, in.JobID)
	if err != nil {
		return nil, err
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

// RegisterCompany registers the unified company-parameterized job tools.
func RegisterCompany(s *mcp.Server, reg *ats.Registry) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_jobs_by_company",
		Description: "Search a specific company's official job openings by company name. Covers hundreds of companies; if the company isn't recognized, the error suggests the closest supported names. Results are summaries — use get_job_detail_by_company for full descriptions.",
		Annotations: &mcp.ToolAnnotations{Title: "Search jobs by company", ReadOnlyHint: true},
		InputSchema: companySearchInputSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *companySearchInput) (*mcp.CallToolResult, *companySearchOutput, error) {
		out, err := companySearch(ctx, reg, in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_filters_by_company",
		Description: "Discover a company's currently valid job-search filter dimensions and values (e.g. job family, employment type). Optional: only call it when a search needs precise narrowing beyond query and location.",
		Annotations: &mcp.ToolAnnotations{Title: "Get company job filters", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *companyFiltersInput) (*mcp.CallToolResult, *companyFiltersOutput, error) {
		out, err := companyFilters(ctx, reg, in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_job_detail_by_company",
		Description: "Get one job's full description (plain text) by company plus the job_id from search_jobs_by_company.",
		Annotations: &mcp.ToolAnnotations{Title: "Get company job detail", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in *companyDetailInput) (*mcp.CallToolResult, *companyDetailOutput, error) {
		out, err := companyDetail(ctx, reg, in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return nil, out, nil
	})
}
```

- [ ] **Step 4: 跑測試確認通過**

Run: `go test ./internal/openingsmcp/ -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/openingsmcp/company.go internal/openingsmcp/company_test.go
git commit -m "feat(mcp): add unified *_by_company tools"
```

---

### Task 8: main.go 組裝與 serverInstructions

**Files:**
- Modify: `cmd/openings-mcp/main.go`(`serverInstructions` 常數、`runWithTransport`、`newServer`)

**Interfaces:**
- Consumes: `ats.NewWorkdayAdapter`、`ats.NewLeverAdapter`、`ats.NewAshbyAdapter`、`ats.NewRegistry`、`openingsmcp.RegisterCompany`
- Produces: 完整可跑的 server

- [ ] **Step 1: 修改 runWithTransport 與 newServer**

在 `runWithTransport` 中(`hc` 建立之後、`newServer` 呼叫之前)建 adapter 與 registry;production base URL 沿用 CLI 使用的正式端點(lever `https://api.lever.co`、ashby `https://api.ashbyhq.com`;workday 的 base URL 由 roster 逐公司推導,不需傳入):

```go
	adapterWorkday := ats.NewWorkdayAdapter(hc)
	adapterLever, err := ats.NewLeverAdapter("https://api.lever.co", hc)
	if err != nil {
		return err
	}
	adapterAshby, err := ats.NewAshbyAdapter("https://api.ashbyhq.com", hc)
	if err != nil {
		return err
	}
	registry, err := ats.NewRegistry(adapterWorkday, adapterLever, adapterAshby)
	if err != nil {
		return err
	}
```

import 加 `"github.com/amikai/openings-mcp/internal/ats"`。`newServer` 簽名尾端加一個參數 `registry *ats.Registry`,函式內在既有 Register 呼叫後加:

```go
	openingsmcp.RegisterCompany(server, registry)
```

`runWithTransport` 的呼叫處同步改為 `newServer(c104, cCake, cNvidia, cTsmc, cGoogle, cLinkedin, registry, logger)`(registry 放 logger 之前)。

- [ ] **Step 2: 更新 serverInstructions**

把常數開頭第一段換成(其餘各段保留):

```
openings-mcp exposes job-search tools in two families: (1) per-provider tools for the job boards 104 and Cake.me (Taiwan-centric) and LinkedIn (global), plus the careers sites of Google, NVIDIA, and TSMC; (2) unified company tools — search_jobs_by_company, get_filters_by_company, get_job_detail_by_company — covering hundreds of companies' official careers sites behind one company parameter.
```

並在 `Tool selection:` 清單最前面加一條:

```
- When the user names a specific company, try search_jobs_by_company first; it covers hundreds of companies and its error message suggests close matches when a name isn't recognized. Fall back to the per-provider tools (linkedin, 104, ...) when the company isn't covered.
```

- [ ] **Step 3: 全量驗證**

```bash
go build ./...
go vet ./...
gofmt -l internal/ cmd/     # 應無輸出
go test ./...
```

Expected: 全部通過。再煙霧測試 server 能啟動:

```bash
go run ./cmd/openings-mcp --version
```

Expected: 印出版本資訊、exit 0(registry 建構失敗會在 run 時報錯,version 路徑不觸發;真正的啟動驗證由 `go test ./internal/...` 的 registry 測試涵蓋)。

- [ ] **Step 4: Commit**

```bash
git add cmd/openings-mcp/main.go
git commit -m "feat(mcp): wire unified company tools into the server"
```

---

## Self-Review 紀錄

- **Spec 覆蓋**:三工具(Task 7)、Adapter interface 與型別(Task 1)、registry/resolver 與教學錯誤(Task 2)、full-dump 引擎含決定性排序與分頁(Task 3)、lever JD 分段組成(Task 4)、ashby detail 重抓整包與 isListed(Task 5)、workday 二段請求/facet 攤平/fox 去重/SplitExternalPath(Task 6)、main 組裝與 serverInstructions(Task 8)、錯誤處理總表各列(散在 Task 2/3/5/6/7 的教學錯誤與 errorResult)。spec 的「slug 撞名啟動失敗」由 Task 2 覆蓋。
- **型別一致性**:`dumpJob`/`searchDump`/`distinctFilters`(Task 3 產出、4/5 消費)、`Registry.Resolve` 簽名(Task 2 產出、7 消費)、adapter 建構子簽名(4/5/6 產出、8 消費)已核對一致。
- **已知留白(刻意)**:ashby `SecondaryLocation` 欄位名以生成碼為準(Task 5 內註明);workday mock 場景下 summary URL 為空是 `PublicSiteURL` 對無路徑 URL 的預期行為,production 不受影響(Task 6 code comment 有記載)。
