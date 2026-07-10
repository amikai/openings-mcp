# Careers URL Resolve Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the unified company tools' `company` parameter accept a careers-page URL on any of the four supported ATSes (Workday, Greenhouse, Lever, Ashby), so companies outside the curated roster become queryable with zero registration.

**Architecture:** Each adapter gains `ParseCareersURL(u *url.URL) (slug string, ok bool)`. For Greenhouse/Lever/Ashby the parsed slug is the board token / org — identical in shape to a roster key, so their Search/Filters paths need no change. Workday's config is three values (tenant/instance/site), so non-roster tenants use the canonical careers URL itself as the slug; the adapter re-parses it on receipt. `Registry.Resolve` tries the existing name/slug maps first, then — for URL-shaped input — polls adapters' `ParseCareersURL`. Stateless throughout: every call re-derives config from the `company` argument.

**Tech Stack:** Go stdlib (`net/url`, `regexp`), existing ogen provider clients, testify tests.

## Global Constraints

- Full spec: `docs/superpowers/specs/2026-07-10-careers-url-resolve-design.md`. This plan implements it exactly; if anything here seems to contradict it, the spec wins and this plan has a bug.
- Scope: `internal/provider/workday`, `internal/ats`, `internal/openingsmcp`, `cmd/openings-mcp` (instructions text only). Do not touch other providers or CLIs.
- Test style: testify (`assert`/`require`) everywhere touched by this plan (`internal/ats` and `internal/provider/workday` both already use it — match the neighboring tests).
- **Do not run `git commit` unless a step explicitly says so, and do not push or open a PR unless the user asks.**
- Module path: `github.com/amikai/openings-mcp`. Run `go build ./...` before every commit.
- The `Adapter` interface method is added LAST (Task 4) — adding it before all four adapters implement it breaks the build mid-plan.

---

### Task 1: `workday.CareersSite` — provider-level URL parsing

The provider layer owns Workday URL shapes (`Company.BaseURL`, `PublicSiteURL` live here). Add the inverse: parse a public careers URL into the three config values, plus the two URL renderers the ats layer needs.

**Files:**
- Create: `internal/provider/workday/careers_url.go`
- Test: `internal/provider/workday/careers_url_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces (Task 3 relies on these exact names):
  - `type CareersSite struct { Host, Tenant, Site string }`
  - `func ParseCareersURL(u *url.URL) (CareersSite, bool)`
  - `func (s CareersSite) BaseURL() string` — CXS API base, mirrors `Company.BaseURL`
  - `func (s CareersSite) CanonicalURL() string` — `https://<host>/<site>`

- [ ] **Step 1: Write the failing tests**

Create `internal/provider/workday/careers_url_test.go`:

```go
package workday

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestParseCareersURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want CareersSite
		ok   bool
	}{
		{
			name: "plain",
			raw:  "https://stripe.wd5.myworkdayjobs.com/Stripe_Careers",
			want: CareersSite{Host: "stripe.wd5.myworkdayjobs.com", Tenant: "stripe", Site: "Stripe_Careers"},
			ok:   true,
		},
		{
			name: "locale prefix stripped",
			raw:  "https://stripe.wd5.myworkdayjobs.com/en-US/Stripe_Careers",
			want: CareersSite{Host: "stripe.wd5.myworkdayjobs.com", Tenant: "stripe", Site: "Stripe_Careers"},
			ok:   true,
		},
		{
			name: "lowercase locale and deep job link ignored",
			raw:  "https://acme.wd103.myworkdayjobs.com/zh-tw/jobs4acme/job/Taipei/Engineer_JR1",
			want: CareersSite{Host: "acme.wd103.myworkdayjobs.com", Tenant: "acme", Site: "jobs4acme"},
			ok:   true,
		},
		{
			name: "myworkdaysite host kept verbatim",
			raw:  "https://acme.wd1.myworkdaysite.com/recruiting",
			want: CareersSite{Host: "acme.wd1.myworkdaysite.com", Tenant: "acme", Site: "recruiting"},
			ok:   true,
		},
		{
			name: "query and fragment ignored",
			raw:  "https://acme.wd1.myworkdayjobs.com/External?q=go#top",
			want: CareersSite{Host: "acme.wd1.myworkdayjobs.com", Tenant: "acme", Site: "External"},
			ok:   true,
		},
		{name: "no site segment", raw: "https://acme.wd1.myworkdayjobs.com/", ok: false},
		{name: "locale only, no site", raw: "https://acme.wd1.myworkdayjobs.com/en-US", ok: false},
		{name: "wrong domain", raw: "https://acme.wd1.example.com/External", ok: false},
		{name: "three host labels", raw: "https://www.myworkdayjobs.com/External", ok: false},
		{name: "five host labels", raw: "https://a.b.wd1.myworkdayjobs.com/External", ok: false},
		{name: "instance not wd-prefixed", raw: "https://acme.prod.myworkdayjobs.com/External", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseCareersURL(mustParse(t, tt.raw))
			require.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCareersSiteURLs(t *testing.T) {
	s := CareersSite{Host: "stripe.wd5.myworkdayjobs.com", Tenant: "stripe", Site: "Stripe_Careers"}
	assert.Equal(t, "https://stripe.wd5.myworkdayjobs.com/wday/cxs/stripe/Stripe_Careers", s.BaseURL())
	assert.Equal(t, "https://stripe.wd5.myworkdayjobs.com/Stripe_Careers", s.CanonicalURL())
}

func TestCanonicalURLRoundTrips(t *testing.T) {
	// The ats layer circulates CanonicalURL as a slug and re-parses it, so
	// parse(canonical) must reproduce the same CareersSite.
	orig, ok := ParseCareersURL(mustParse(t, "https://stripe.wd5.myworkdayjobs.com/en-US/Stripe_Careers/job/SF/Eng_1"))
	require.True(t, ok)
	again, ok := ParseCareersURL(mustParse(t, orig.CanonicalURL()))
	require.True(t, ok)
	assert.Equal(t, orig, again)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/provider/workday/ -run 'TestParseCareersURL|TestCareersSite|TestCanonicalURL' -v`
Expected: compile error — `CareersSite` and `ParseCareersURL` undefined.

- [ ] **Step 3: Implement**

Create `internal/provider/workday/careers_url.go`:

```go
package workday

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// CareersSite addresses one Workday career site by its public-URL parts,
// for tenants outside the curated roster. It carries the same values
// Company encodes; Host keeps the instance and domain verbatim so
// myworkdaysite.com tenants work unchanged.
type CareersSite struct {
	Host   string // e.g. "stripe.wd5.myworkdayjobs.com"
	Tenant string // first host label
	Site   string // career-site path segment
}

// localeSegment matches the optional language prefix careers URLs carry
// before the site segment ("en-US", "zh-tw", "fr").
var localeSegment = regexp.MustCompile(`^[a-zA-Z]{2}(?:-[a-zA-Z]{2})?$`)

// ParseCareersURL reports whether u is a Workday career-site URL and
// extracts its parts. It accepts only the public host shape
// <tenant>.<wd*>.myworkdayjobs.com (or myworkdaysite.com) with a site path
// segment; locale prefixes and job deep links are tolerated and stripped.
func ParseCareersURL(u *url.URL) (CareersSite, bool) {
	host := strings.ToLower(u.Hostname())
	labels := strings.Split(host, ".")
	if len(labels) != 4 {
		return CareersSite{}, false
	}
	if domain := labels[2] + "." + labels[3]; domain != "myworkdayjobs.com" && domain != "myworkdaysite.com" {
		return CareersSite{}, false
	}
	if labels[0] == "" || !strings.HasPrefix(labels[1], "wd") {
		return CareersSite{}, false
	}
	segs := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	if len(segs) > 0 && localeSegment.MatchString(segs[0]) {
		segs = segs[1:]
	}
	if len(segs) == 0 || segs[0] == "" {
		return CareersSite{}, false
	}
	return CareersSite{Host: host, Tenant: labels[0], Site: segs[0]}, true
}

// BaseURL derives the CXS API base URL, mirroring Company.BaseURL.
func (s CareersSite) BaseURL() string {
	return fmt.Sprintf("https://%s/wday/cxs/%s/%s", s.Host, s.Tenant, s.Site)
}

// CanonicalURL renders the slug form the ats layer circulates for
// non-roster tenants: locale, deep links, query, and fragment stripped.
func (s CareersSite) CanonicalURL() string {
	return fmt.Sprintf("https://%s/%s", s.Host, s.Site)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/provider/workday/ -v`
Expected: all PASS (including pre-existing tests).

- [ ] **Step 5: Commit**

```bash
git add internal/provider/workday/careers_url.go internal/provider/workday/careers_url_test.go
git commit -m "feat(workday): parse public careers URLs into CareersSite"
```

---

### Task 2: URL helpers + Greenhouse/Lever/Ashby `ParseCareersURL` and Detail name fallback

The three slug-family adapters: recognize their hosts, take the first path segment as the slug. Their Search/Filters already work for any slug (no roster lookup); only `Detail`'s Company field reads the roster map, which returns "" for non-roster slugs — fall back to the slug.

**Files:**
- Create: `internal/ats/careersurl.go`
- Create: `internal/ats/careersurl_test.go`
- Modify: `internal/ats/greenhouse.go` (add method; Detail Company line ~65)
- Modify: `internal/ats/lever.go` (add method; Detail Company line ~60)
- Modify: `internal/ats/ashby.go` (add method; Detail Company line ~59)
- Modify: `internal/provider/greenhouse/mocksrv.go` (non-roster alias route for the fallback test)
- Modify: `internal/provider/ashby/mocksrv.go` (non-roster alias route for the fallback test)

Note on mocks: the lever mock already routes any site (`/v0/postings/{site}` wildcard), so its fallback test needs no mock change. The greenhouse and ashby mocks route fixed roster slugs (`safariai`/`anthropic`, `browserbase`/`weaviate`), all of which ARE in the curated rosters — so exercising the non-roster fallback needs one alias route each, reusing existing fixtures.

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces (Tasks 3–4 rely on these exact names):
  - `func parseCareersInput(s string) (*url.URL, bool)` — URL-candidate detection + parse, scheme-less input gets `https://`
  - `func firstPathSegment(u *url.URL) string` — first non-empty path segment, URL-decoded, `""` if none
  - `func (a *GreenhouseAdapter) ParseCareersURL(u *url.URL) (string, bool)` (same signature on `*LeverAdapter`, `*AshbyAdapter`)

- [ ] **Step 1: Write the failing tests**

Create `internal/ats/careersurl_test.go`:

```go
package ats

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestParseCareersInput(t *testing.T) {
	tests := []struct {
		in       string
		ok       bool
		wantHost string
	}{
		{in: "https://jobs.lever.co/acme", ok: true, wantHost: "jobs.lever.co"},
		{in: "jobs.lever.co/acme", ok: true, wantHost: "jobs.lever.co"}, // scheme-less
		{in: "  https://jobs.ashbyhq.com/acme  ", ok: true, wantHost: "jobs.ashbyhq.com"},
		{in: "nvidia", ok: false},          // plain name
		{in: "NVIDIA Corp", ok: false},     // display name
		{in: "acme.io", ok: false},         // dot but no path
		{in: "ftp://x.co/acme", ok: false}, // non-http scheme
		{in: "", ok: false},
	}
	for _, tt := range tests {
		u, ok := parseCareersInput(tt.in)
		require.Equalf(t, tt.ok, ok, "parseCareersInput(%q)", tt.in)
		if ok {
			assert.Equal(t, tt.wantHost, u.Hostname())
		}
	}
}

func TestFirstPathSegment(t *testing.T) {
	assert.Equal(t, "acme", firstPathSegment(mustParseURL(t, "https://x.co/acme/jobs/1")))
	assert.Equal(t, "acme co", firstPathSegment(mustParseURL(t, "https://x.co/acme%20co")))
	assert.Equal(t, "", firstPathSegment(mustParseURL(t, "https://x.co/")))
}

func TestSlugAdaptersParseCareersURL(t *testing.T) {
	gh, err := NewGreenhouseAdapter("https://example.invalid", http.DefaultClient)
	require.NoError(t, err)
	lv, err := NewLeverAdapter("https://example.invalid", http.DefaultClient)
	require.NoError(t, err)
	ab, err := NewAshbyAdapter("https://example.invalid", http.DefaultClient)
	require.NoError(t, err)

	tests := []struct {
		name    string
		adapter interface {
			ParseCareersURL(*url.URL) (string, bool)
		}
		raw  string
		slug string
		ok   bool
	}{
		{name: "greenhouse job-boards", adapter: gh, raw: "https://job-boards.greenhouse.io/acme", slug: "acme", ok: true},
		{name: "greenhouse boards legacy", adapter: gh, raw: "https://boards.greenhouse.io/acme/jobs/123", slug: "acme", ok: true},
		{name: "greenhouse eu", adapter: gh, raw: "https://job-boards.eu.greenhouse.io/acme", slug: "acme", ok: true},
		{name: "greenhouse wrong host", adapter: gh, raw: "https://jobs.lever.co/acme", ok: false},
		{name: "greenhouse empty path", adapter: gh, raw: "https://job-boards.greenhouse.io/", ok: false},
		{name: "lever", adapter: lv, raw: "https://jobs.lever.co/acme", slug: "acme", ok: true},
		{name: "lever eu", adapter: lv, raw: "https://jobs.eu.lever.co/acme", slug: "acme", ok: true},
		{name: "lever deep link", adapter: lv, raw: "https://jobs.lever.co/acme/00000000-0000", slug: "acme", ok: true},
		{name: "lever wrong host", adapter: lv, raw: "https://job-boards.greenhouse.io/acme", ok: false},
		{name: "ashby", adapter: ab, raw: "https://jobs.ashbyhq.com/acme", slug: "acme", ok: true},
		{name: "ashby url-encoded org", adapter: ab, raw: "https://jobs.ashbyhq.com/Acme%20Inc", slug: "Acme Inc", ok: true},
		{name: "ashby wrong host", adapter: ab, raw: "https://jobs.lever.co/acme", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug, ok := tt.adapter.ParseCareersURL(mustParseURL(t, tt.raw))
			require.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.slug, slug)
			}
		})
	}
}
```

Append to `internal/ats/lever_test.go` (the mock serves fixtures for any site, so a non-roster slug exercises the fallback with no mock change):

```go
func TestLeverDetailCompanyFallsBackToSlug(t *testing.T) {
	a := testLeverAdapter(t)
	res, err := a.Search(t.Context(), "somestartup", SearchParams{})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
	d, err := a.Detail(t.Context(), "somestartup", res.Jobs[0].JobID)
	require.NoError(t, err)
	assert.Equal(t, "somestartup", d.Company, "non-roster slug should be used as company name")
}
```

Append to `internal/ats/greenhouse_test.go` (job ID 4461450008 is the detail fixture's; the alias route below serves it under the non-roster board):

```go
func TestGreenhouseDetailCompanyFallsBackToSlug(t *testing.T) {
	a := testGreenhouseAdapter(t)
	d, err := a.Detail(t.Context(), greenhouse.MockNonRosterBoard, "4461450008")
	require.NoError(t, err)
	assert.Equal(t, greenhouse.MockNonRosterBoard, d.Company, "non-roster slug should be used as company name")
}
```

Append to `internal/ats/ashby_test.go` (Ashby's Detail refetches the whole board; the alias route serves the standard board fixture, so the fixture job ID resolves):

```go
func TestAshbyDetailCompanyFallsBackToSlug(t *testing.T) {
	a := testAshbyAdapter(t)
	d, err := a.Detail(t.Context(), ashby.MockNonRosterBoard, "7724fbe3-6a27-4418-9705-2dcc40751a16")
	require.NoError(t, err)
	assert.Equal(t, ashby.MockNonRosterBoard, d.Company, "non-roster slug should be used as company name")
}
```

In `internal/provider/greenhouse/mocksrv.go`, add the exported alias and route (inside `NewMockServer`, after the anthropic detail route):

```go
// MockNonRosterBoard is a board token deliberately absent from
// companies.yaml, so ats-layer tests can exercise non-roster behavior.
const MockNonRosterBoard = "somestartup"
```

```go
	mux.HandleFunc("/boards/"+MockNonRosterBoard+"/jobs/4461450008", serveMockJSON(mockJobDetailRsp))
```

In `internal/provider/ashby/mocksrv.go`, add the exported alias and route (inside `NewMockServer`, before the catch-all 404 route):

```go
// MockNonRosterBoard is a board name deliberately absent from
// companies.yaml, so ats-layer tests can exercise non-roster behavior.
const MockNonRosterBoard = "somestartup"
```

```go
	mux.HandleFunc("/posting-api/job-board/"+MockNonRosterBoard, serveMockJSON(mockBoardRsp))
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ats/ -run 'TestParseCareersInput|TestFirstPathSegment|TestSlugAdaptersParseCareersURL|TestLeverDetailCompanyFallsBack|TestGreenhouseDetailCompanyFallsBack|TestAshbyDetailCompanyFallsBack' -v`
Expected: compile error — `parseCareersInput` undefined.

- [ ] **Step 3: Implement**

Create `internal/ats/careersurl.go`:

```go
package ats

import (
	"net/url"
	"strings"
)

// parseCareersInput reports whether a company input is a careers-URL
// candidate and parses it. Scheme-less inputs like "jobs.lever.co/acme"
// get https; anything without both a dot and a path stays a name.
func parseCareersInput(s string) (*url.URL, bool) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "://") {
		if !strings.Contains(s, ".") || !strings.Contains(s, "/") {
			return nil, false
		}
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, false
	}
	return u, true
}

// firstPathSegment returns the first non-empty path segment, URL-decoded,
// or "" when the path has none (or decoding fails).
func firstPathSegment(u *url.URL) string {
	for _, seg := range strings.Split(strings.Trim(u.EscapedPath(), "/"), "/") {
		if seg == "" {
			continue
		}
		dec, err := url.PathUnescape(seg)
		if err != nil {
			return ""
		}
		return dec
	}
	return ""
}
```

In `internal/ats/greenhouse.go`, add (plus `"net/url"` and `"cmp"` imports):

```go
// greenhouseHosts are the public board hosts Greenhouse serves careers
// pages from, including the EU data-residency variants.
var greenhouseHosts = map[string]bool{
	"job-boards.greenhouse.io":    true,
	"boards.greenhouse.io":        true,
	"job-boards.eu.greenhouse.io": true,
	"boards.eu.greenhouse.io":     true,
}

// ParseCareersURL recognizes Greenhouse-hosted board URLs; the first path
// segment is the board token, which is already this adapter's slug form.
func (a *GreenhouseAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if !greenhouseHosts[strings.ToLower(u.Hostname())] {
		return "", false
	}
	token := firstPathSegment(u)
	return token, token != ""
}
```

and change the Detail Company line:

```go
Company: cmp.Or(greenhouse.CompaniesByBoardToken[strings.ToLower(slug)].Name, slug),
```

In `internal/ats/lever.go`, add (plus `"net/url"` and `"cmp"` imports):

```go
// leverHosts are Lever's public board hosts, including the EU variant.
var leverHosts = map[string]bool{
	"jobs.lever.co":    true,
	"jobs.eu.lever.co": true,
}

// ParseCareersURL recognizes Lever-hosted board URLs; the first path
// segment is the organization, which is already this adapter's slug form.
func (a *LeverAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if !leverHosts[strings.ToLower(u.Hostname())] {
		return "", false
	}
	org := firstPathSegment(u)
	return org, org != ""
}
```

and change the Detail Company line:

```go
Company: cmp.Or(lever.CompaniesBySite[slug].Name, slug),
```

In `internal/ats/ashby.go`, add (plus `"net/url"` and `"cmp"` imports):

```go
// ParseCareersURL recognizes Ashby-hosted board URLs; the first path
// segment is the organization name, which is already this adapter's slug
// form (URL-decoded — Ashby org names may contain spaces).
func (a *AshbyAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if strings.ToLower(u.Hostname()) != "jobs.ashbyhq.com" {
		return "", false
	}
	org := firstPathSegment(u)
	return org, org != ""
}
```

and change the Detail Company line:

```go
Company: cmp.Or(ashby.CompaniesByBoard[slug].Name, slug),
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ats/ -v`
Expected: all PASS (new and pre-existing).

- [ ] **Step 5: Commit**

```bash
git add internal/ats/careersurl.go internal/ats/careersurl_test.go internal/ats/greenhouse.go internal/ats/lever.go internal/ats/ashby.go internal/ats/greenhouse_test.go internal/ats/lever_test.go internal/ats/ashby_test.go internal/provider/greenhouse/mocksrv.go internal/provider/ashby/mocksrv.go
git commit -m "feat(ats): parse careers URLs for greenhouse/lever/ashby"
```

---

### Task 3: Workday adapter — URL slugs end to end

Workday's slug gains a second form: the canonical careers URL, carrying tenant/instance/site for non-roster companies. Rework the adapter's `client()` into roster-then-URL resolution, and add the `ParseCareersURL` method that folds roster tenants back to their roster key.

**Files:**
- Modify: `internal/ats/workday.go` (struct fields, `NewWorkdayAdapter`, `client`, `Search`, `Filters`, `Detail`; add `ParseCareersURL`, `resolveSlug`, `tenantSite`)
- Modify: `internal/ats/workday_test.go` (helper gains `siteBaseURL` override; new tests)

**Interfaces:**
- Consumes (from Task 1): `workday.CareersSite`, `workday.ParseCareersURL(u *url.URL) (CareersSite, bool)`, `(CareersSite).BaseURL()`, `(CareersSite).CanonicalURL()`. From Task 2: `parseCareersInput`.
- Produces: `func (a *WorkdayAdapter) ParseCareersURL(u *url.URL) (string, bool)` (Task 4's interface addition needs it).

- [ ] **Step 1: Write the failing tests**

Append to `internal/ats/workday_test.go` (note: `net/url` import needed; `mustParseURL` comes from Task 2's `careersurl_test.go`):

```go
func TestWorkdayParseCareersURL(t *testing.T) {
	a := NewWorkdayAdapter(http.DefaultClient)

	// A roster tenant folds back to its roster key, keeping display names.
	slug, ok := a.ParseCareersURL(mustParseURL(t, "https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite"))
	require.True(t, ok)
	assert.Equal(t, "nvidia", slug)

	// An unknown tenant gets the canonical URL as a self-describing slug.
	slug, ok = a.ParseCareersURL(mustParseURL(t, "https://stripe.wd5.myworkdayjobs.com/en-US/Stripe_Careers/job/SF/Eng_1"))
	require.True(t, ok)
	assert.Equal(t, "https://stripe.wd5.myworkdayjobs.com/Stripe_Careers", slug)

	_, ok = a.ParseCareersURL(mustParseURL(t, "https://jobs.lever.co/acme"))
	assert.False(t, ok)
}

func TestWorkdayURLSlugSearchAndDetail(t *testing.T) {
	a, _ := testWorkdayAdapter(t)
	urlSlug := "https://stripe.wd5.myworkdayjobs.com/Stripe_Careers"

	res, err := a.Search(t.Context(), urlSlug, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 27, res.TotalCount)
	require.NotEmpty(t, res.Jobs)

	d, err := a.Detail(t.Context(), urlSlug, res.Jobs[0].JobID)
	require.NoError(t, err)
	assert.Equal(t, "stripe", d.Company, "URL-resolved company name should be the tenant")
	assert.NotEmpty(t, d.Description)
}

func TestWorkdayUnknownSlugTeaches(t *testing.T) {
	a := NewWorkdayAdapter(http.DefaultClient)
	_, err := a.Search(t.Context(), "not-a-tenant", SearchParams{})
	require.ErrorContains(t, err, "careers URL", "error should teach the URL alternative")
}
```

Update the existing `testWorkdayAdapter` helper so both resolution paths hit the mock:

```go
func testWorkdayAdapter(t *testing.T) (*WorkdayAdapter, *[][]byte) {
	t.Helper()
	mock := workday.NewMockServer(workday.MockNvidiaJobsRsp, workday.MockNvidiaJobDetailRsp)
	t.Cleanup(mock.Close)
	proxy, bodies := recordingProxy(t, mock.URL)
	a := NewWorkdayAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(workday.Company) string { return proxy.URL }
	a.siteBaseURL = func(workday.CareersSite) string { return proxy.URL }
	return a, bodies
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ats/ -run 'TestWorkday' -v`
Expected: compile error — `siteBaseURL` and `ParseCareersURL` undefined on `*WorkdayAdapter`.

- [ ] **Step 3: Implement**

In `internal/ats/workday.go` (add `"net/url"` import):

Struct and constructor:

```go
type WorkdayAdapter struct {
	hc *http.Client
	// baseURL and siteBaseURL derive CXS base URLs for roster tenants and
	// URL-resolved career sites respectively; tests point them at a mock.
	baseURL     func(workday.Company) string
	siteBaseURL func(workday.CareersSite) string
}

func NewWorkdayAdapter(hc *http.Client) *WorkdayAdapter {
	return &WorkdayAdapter{
		hc:          hc,
		baseURL:     workday.Company.BaseURL,
		siteBaseURL: workday.CareersSite.BaseURL,
	}
}
```

New method and resolution (replace the existing `client` function):

```go
// ParseCareersURL recognizes myworkdayjobs.com / myworkdaysite.com careers
// URLs. Roster tenants fold back to their roster slug so display names
// stay identical to name-based resolution; unknown tenants get the
// canonical URL as a self-describing slug (workday config is three values,
// which a bare tenant slug can't carry).
func (a *WorkdayAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	site, ok := workday.ParseCareersURL(u)
	if !ok {
		return "", false
	}
	slug := strings.ToLower(site.Tenant)
	if _, ok := workday.CompaniesByTenant[slug]; ok {
		return slug, true
	}
	return site.CanonicalURL(), true
}

// tenantSite is one reachable career site, however the slug named it.
// name feeds JobDetail.Company; base is the CXS base URL.
type tenantSite struct {
	name string
	base string
}

// resolveSlug maps a slug to its career site: roster key first, then the
// canonical-URL form ParseCareersURL hands out for non-roster tenants.
func (a *WorkdayAdapter) resolveSlug(slug string) (tenantSite, error) {
	if company, ok := workday.CompaniesByTenant[slug]; ok {
		return tenantSite{name: company.Name, base: a.baseURL(company)}, nil
	}
	if u, ok := parseCareersInput(slug); ok {
		if site, ok := workday.ParseCareersURL(u); ok {
			return tenantSite{name: site.Tenant, base: a.siteBaseURL(site)}, nil
		}
	}
	return tenantSite{}, fmt.Errorf("workday: unknown company %q; pass a roster slug or a myworkdayjobs.com careers URL", slug)
}

// client builds a per-site CXS client on demand. The wrapper is stateless
// and cheap; connection pooling lives in the shared http.Client.
func (a *WorkdayAdapter) client(slug string) (*workday.Client, tenantSite, error) {
	ts, err := a.resolveSlug(slug)
	if err != nil {
		return nil, tenantSite{}, err
	}
	c, err := workday.NewClient(ts.base, workday.WithClient(a.hc))
	if err != nil {
		return nil, tenantSite{}, err
	}
	return c, ts, nil
}
```

Call-site updates:

- `Search`: `client, ts, err := a.client(slug)`; replace `workday.PublicSiteURL(a.baseURL(company))` with `workday.PublicSiteURL(ts.base)`. (Behavior identical for roster tenants: `ts.base` is exactly `a.baseURL(company)`.)
- `Filters`: `client, _, err := a.client(slug)` — shape unchanged.
- `Detail`: `client, ts, err := a.client(slug)`; replace `Company: company.Name` with `Company: ts.name`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ats/ -v && go build ./...`
Expected: all PASS; build OK.

- [ ] **Step 5: Commit**

```bash
git add internal/ats/workday.go internal/ats/workday_test.go
git commit -m "feat(ats): resolve workday careers URLs as self-describing slugs"
```

---

### Task 4: `Adapter` interface method + `Registry.Resolve` URL branch

All four adapters now implement `ParseCareersURL`, so the interface addition compiles. Registry keeps the adapter list and, for URL-shaped input that misses the maps, polls adapters in registration order.

**Files:**
- Modify: `internal/ats/ats.go` (interface)
- Modify: `internal/ats/registry.go` (`Registry` struct, `NewRegistry`, `Resolve`)
- Modify: `internal/ats/registry_test.go` (`fakeAdapter` gains the method; new tests)

**Interfaces:**
- Consumes (from Task 2): `parseCareersInput`, `firstPathSegment`.
- Produces: `Adapter.ParseCareersURL(u *url.URL) (slug string, ok bool)` as an interface method; `Resolve` accepting careers URLs.

- [ ] **Step 1: Write the failing tests**

In `internal/ats/registry_test.go`, extend `fakeAdapter` (add `"net/url"` import):

```go
type fakeAdapter struct {
	name   string
	host   string // careers host this fake claims, e.g. "jobs.fake-lever.example"
	roster []CompanyInfo
}

func (f *fakeAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if f.host == "" || u.Hostname() != f.host {
		return "", false
	}
	slug := firstPathSegment(u)
	return slug, slug != ""
}
```

Give the two fakes in `testRegistry` hosts: `host: "jobs.fake-workday.example"` on the workday fake and `host: "jobs.fake-lever.example"` on the lever fake. Then append:

```go
func TestResolveCareersURL(t *testing.T) {
	r := testRegistry(t)
	a, slug, err := r.Resolve("https://jobs.fake-lever.example/somestartup")
	require.NoError(t, err)
	assert.Equal(t, "lever", a.Name())
	assert.Equal(t, "somestartup", slug)
}

func TestResolveCareersURLSchemeless(t *testing.T) {
	r := testRegistry(t)
	a, slug, err := r.Resolve("jobs.fake-workday.example/acme")
	require.NoError(t, err)
	assert.Equal(t, "workday", a.Name())
	assert.Equal(t, "acme", slug)
}

func TestResolveUnrecognizedCareersURLTeaches(t *testing.T) {
	r := testRegistry(t)
	_, _, err := r.Resolve("https://careers.example.com/acme")
	require.ErrorContains(t, err, "careers URL", "URL misses should get the URL error, not name suggestions")
	assert.NotContains(t, err.Error(), "closest matches", "no levenshtein suggestions for URLs")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ats/ -run 'TestResolve' -v`
Expected: `TestResolveCareersURL*` FAIL — Resolve returns the unknown-company error (the fakes' method exists but Resolve never calls it).

- [ ] **Step 3: Implement**

In `internal/ats/ats.go`, add `"net/url"` to imports and extend the interface (after `Roster`):

```go
	// ParseCareersURL reports whether u is a careers-page URL on this ATS,
	// and if so returns the slug that addresses that company. The slug may
	// be a roster key or a self-describing form (workday returns the
	// canonical careers URL for tenants outside its roster).
	ParseCareersURL(u *url.URL) (slug string, ok bool)
```

In `internal/ats/registry.go`:

Struct and constructor keep the adapter list:

```go
type Registry struct {
	adapters []Adapter                 // registration order; polled for careers-URL input
	bySlug   map[string]registryEntry  // key: normalize(slug)
	byName   map[string]registryEntry  // key: normalize(display name)
	slugs    []slugEntry               // sorted by slug, for suggestions
}
```

In `NewRegistry`, first line of the adapter loop body is unchanged; add `r.adapters = adapters` right after constructing `r` (before the loop).

In `Resolve`, insert the URL branch between the `byName` lookup and the final error:

```go
	if u, ok := parseCareersInput(company); ok {
		for _, a := range r.adapters {
			if slug, ok := a.ParseCareersURL(u); ok {
				return a, slug, nil
			}
		}
		return nil, "", fmt.Errorf("unrecognized careers URL %q; supported careers-page hosts: <tenant>.<wd*>.myworkdayjobs.com/<site>, job-boards.greenhouse.io/<board>, jobs.lever.co/<org>, jobs.ashbyhq.com/<org>", company)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ats/ -v && go build ./...`
Expected: all PASS; build OK.

- [ ] **Step 5: Commit**

```bash
git add internal/ats/ats.go internal/ats/registry.go internal/ats/registry_test.go
git commit -m "feat(ats): resolve careers-page URLs to adapters in Registry"
```

---

### Task 5: MCP schema text + server instructions

Code paths are done; teach the LLM. Text only — no behavior change, so no new tests; the gate is the full build+test run.

**Files:**
- Modify: `internal/openingsmcp/company.go` (schema description, two jsonschema tags, one tool description)
- Modify: `cmd/openings-mcp/main.go` (`serverInstructions`)

**Interfaces:**
- Consumes: everything above, transitively. Produces: nothing consumed by other tasks.

- [ ] **Step 1: Update `internal/openingsmcp/company.go`**

In `companySearchInputRawSchema`, replace the `company` description with:

```
"description": "Company name or slug, e.g. 'nvidia' or 'NVIDIA Corp', or a careers-page URL on a supported ATS (Workday, Greenhouse, Lever, Ashby), e.g. 'https://jobs.lever.co/acme'. If a name isn't recognized, the error message suggests the closest supported companies."
```

Change the `Company` field tags on `companyFiltersInput` and `companyDetailInput` to:

```go
Company string `json:"company" jsonschema:"Company name, slug, or careers-page URL, e.g. 'nvidia' or 'https://jobs.lever.co/acme'."`
```

Append one sentence to the `search_jobs_by_company` tool description:

```
Companies outside the list can be searched by passing their careers-page URL (Workday, Greenhouse, Lever, or Ashby) as company.
```

- [ ] **Step 2: Update `cmd/openings-mcp/main.go`**

Add one bullet to the "Tool selection" section of `serverInstructions`:

```
- search_jobs_by_company also accepts a careers-page URL on Workday, Greenhouse, Lever, or Ashby. When a company isn't in the supported list, find its careers page URL (e.g. via web search) and pass that URL as the company argument.
```

- [ ] **Step 3: Full gate**

Run: `go build ./... && go test ./...`
Expected: build OK, all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/openingsmcp/company.go cmd/openings-mcp/main.go
git commit -m "docs(mcp): teach careers-URL input on company tools"
```
