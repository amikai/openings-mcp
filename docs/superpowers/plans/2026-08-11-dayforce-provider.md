# Dayforce provider integration

Date: 2026-08-11
Provider key: `dayforce`
Host: `jobs.dayforcehcm.com` (legacy per-tenant hosts `us<NNN>.dayforcehcm.com`,
`www.mydayforce.com` redirect here)

## Recon summary

Dayforce (formerly Ceridian Dayforce) Recruiting serves every customer's
candidate portal from one Next.js app on `jobs.dayforcehcm.com`, keyed by
`clientNamespace` (tenant) + `careerSiteXRefCode` (job board within the
tenant). Behind it sits a **public JSON REST API** with server-side search,
filters and pagination. This is the classic multi-tenant ATS shape: one
`ats.Adapter`, many tenants, roster-driven.

Answer to the feasibility question up front: **yes, and on the best surface
tier the repo recognises** — JSON REST, no login, no browser, no HTML parsing
for the job data itself.

### Surfaces found (all public, verified with curl outside the browser)

| Surface | Method | Shape | Verdict |
|---|---|---|---|
| `/api/geo/{ns}/jobposting/search` | POST | JSON, 25/page, server-side filters | **adapter Search** |
| `/api/geo/{ns}/jobposting/{ns}/{culture}/{jobBoardId}/{postingId}` | GET | JSON, full posting | **adapter Detail** |
| `/api/geo/{ns}/postingattributes/{departments,payclasses,paytypes}/{ns}/{jobBoardId}/{culture}` | GET | JSON id+value lists | **adapter Filters** |
| `/api/geo/{ns}/location/search?filter=<q>` | POST | JSON location typeahead | optional; location filter helper |
| `/{culture}/{ns}/{xref}` and `/{culture}/{ns}/{xref}/jobs/{id}` | GET | SSR Next.js, `__NEXT_DATA__` | site-info (`jobBoardId`) + human URLs |
| `/api/auth/csrf` | GET | `{"csrfToken": "..."}` | transport prerequisite (below) |

`jobs.dayforcehcm.com/robots.txt` contains only Cloudflare's content-signal
preamble — no `User-agent`, no `Disallow`, no ClaudeBot section. Nothing is
disallowed, and there is no sitemap (`/sitemap.xml` → 404).

### The one non-obvious gate: next-auth CSRF

`POST` is rejected with a bare `403 Forbidden` (9 bytes, no JSON) regardless of
body. It is not Cloudflare and not body validation. The app's axios instance
installs a request interceptor that, for `POST/PUT/PATCH/DELETE` only, sets
`X-CSRF-TOKEN` from next-auth's `getCsrfToken()`:

```
h.A.interceptors.request.use(async e => {
  if (!["POST","PUT","PATCH","DELETE"].includes(e.method...)) return e
  e.headers["X-CSRF-TOKEN"] = await getCsrfToken()
  return e
})
```

So a client must:

1. `GET /api/auth/csrf` keeping the response cookies
   (`__Host-next-auth.csrf-token`),
2. send both the cookie **and** the `X-CSRF-TOKEN` header on every POST.

Verified: header without cookie → `403`; cookie + header → `200`. The token is
reusable for the life of the cookie jar. `GET` endpoints (detail, posting
attributes) need neither.

### Search request/response, recovered from the search-page chunk

The payload shape comes from
`/_next/static/chunks/pages/[clientNamespace]/[careerSiteXRefCode]-*.js` and
was then verified field by field against the live API:

```json
{
  "clientNamespace": "pca",          // required; unknown tenant → 404
  "jobBoardCode": "CANDIDATEPORTAL", // required; wrong code → 404; case-insensitive
  "cultureCode": "en-US",            // required; missing → 400
  "searchText": "electrical engineer",
  "locationId": 0, "locationType": 0, "locationString": "Chicago, Illinois, United States",
  "distance": 50, "distanceUnit": 0,
  "payClass": 1, "payType": 2, "departmentId": 290,
  "travelRequired": true,
  "fromDate": "2026-08-01T00:00:00.000Z",
  "paginationStart": 0
}
```

Response: `{"jobPostings":[...],"maxCount":352,"offset":0,"count":25}`.

Measured behaviour:

- **Page size is fixed at 25** and not client-controllable. `pageSize`,
  `count`, `offset`, `page`, `pageNumber`, `skip`/`take` in body or query
  string are all silently ignored. Pagination is `paginationStart` = `(page-1)*25`,
  which the API echoes back as `offset`. `paginationStart` past the end →
  `200` with an empty `jobPostings` and the true `maxCount`.
- Every documented filter is **genuinely server-side**: `searchText`
  352 → 115, `departmentId=290` → 1, `locationString`+`distance=50` → 13,
  `fromDate` → 23. No soft-filter compensation needed.
- Each row carries the full `jobDescription`, so list-level results are
  already rich; `postingLocations[]` carries `formattedAddress`, `cityName`,
  `stateCode`, `isoCountryCode`, `coordinates`, `locationType`, and
  `hasVirtualLocation` is the remote signal.
- **Silent-empty trap:** an unsupported `cultureCode` returns `200` with
  `maxCount: 0` rather than an error (`ja-JP` on `pca` → 0 jobs). A
  wrong culture is indistinguishable from an empty board unless we pin it.

### Detail

`GET /api/geo/{ns}/jobposting/{ns}/{culture}/{jobBoardId}/{postingId}` returns
a superset of the search row: `jobPostingContent`
(`jobDescriptionHeader`/body/footer HTML), `jobPostingAttributes[]`
(name/value/type pairs — `PayType`, `HiringMinRate`, `HiringMaxRate`),
`isoCurrencyRegion`, `isInternal`, `relocationEligible`,
`createdTimestampUTC`, `lastModifiedTimestampUTC`, `postingStatus`,
`postingType`. Unknown posting id → `404`; a posting id belonging to another
tenant → `404`. The same object is also embedded in the SSR page's
`__NEXT_DATA__` (`props.pageProps.jobData`), which is the fallback if the API
route ever locks down.

### `jobBoardId` — where it comes from

Detail and the posting-attribute endpoints need `jobBoardId`, which is *not* in
the URL a human sees. Two sources, both verified:

- every search row carries `jobBoardId` (and `clientNamespace`), and
- the SSR board page's `__NEXT_DATA__` `dehydratedState` holds a `site-info`
  query with `jobBoardId`, `clientId`, `jobBoardCode`, theme and logo.

It is `1` for the default `CANDIDATEPORTAL` board on all 13 tenants sampled,
but **not universally 1**: `mydayforce` serves `CANDIDATEPORTAL` as
`jobBoardId 1` (2 jobs) and `alljobs` as `jobBoardId 8` (140 jobs). So a
tenant can have several boards, `careerSiteXRefCode` is not always
`CANDIDATEPORTAL`, and `jobBoardId` must be stored per roster row rather than
assumed.

### Why an ATS adapter with server-side search

One API, thousands of tenants, stable per-tenant keys → `ats.Adapter`, joining
`search_jobs_by_company` (the workday/ultipro/smartrecruiters shape). Search is
**server-side**, so this adapter never touches `searchDump`
(`internal/ats/filter.go`).

### Roster discovery mechanism

There is no public tenant index and no sitemap. `site:jobs.dayforcehcm.com`
search-engine harvesting yields namespaces cheaply — one query returned 10
distinct ones — and each candidate is confirmed by a live search call
(`200` + `maxCount > 0`). The API never returns the employer's display name
(the portal shows only a logo), so **names are curated**, sourced from the
portal branding or the company site, exactly as the existing rosters do.

## Envelope (shared constraints)

- **ogen path**, because this is real JSON REST: minimal
  `internal/provider/dayforce/openapi.yaml` covering the six operations used
  (the five data endpoints plus the csrf pre-flight), `gen.go` with the
  `go:generate` line, spec added to `OPENAPI_SPECS` in the Makefile,
  `make validate-openapi` green. Fields marked nullable per real responses.
  Discovered quirks (fixed 25 page size, `paginationStart`, culture
  silent-empty, `jobBoardCode`/`jobBoardId` duality) are documented **in the
  spec**, then regenerated.
- The spec also declares the **observed non-200 responses** S1 turns into typed
  errors, so the generated client surfaces them instead of an opaque
  `UnexpectedStatusCode`: `404` on search (unknown tenant / unknown
  `jobBoardCode`), `404` on detail (unknown posting, or a posting belonging to
  another tenant), and `403` on search with a **non-JSON `Forbidden` body**
  (missing or stale csrf token). In-tree precedent for documenting
  live-observed statuses the vendor never wrote down:
  `internal/provider/smartrecruiters/openapi.yaml:301`,
  `internal/provider/apple/openapi.yaml:83`.
- **The CSRF pre-flight follows the apple session pattern, not a custom
  transport** (`internal/provider/apple/client.go:100-142`): `GET
  /api/auth/csrf` is a **spec operation**, the cookie jar lives on the
  `*http.Client` handed to ogen's `WithClient` (a cloned copy, so a shared
  transport does not leak Dayforce session state), and `X-CSRF-TOKEN` is a
  typed header parameter on the search operation. The wrapper type in
  `client.go` — `BoardClient`, named so it can share the package with the
  generated `Client`, as apple's `JobsClient` does — owns the lazy token fetch
  behind a mutex, caches the token for the life of the jar (unlike apple's,
  it is not bound to the immediately preceding request, so searches need no
  serialization), and refreshes once on a `403` before returning the error.
  There is no `csrf.go` and no `http.RoundTripper`: a `cookiejar.Jar` set on a
  transport is inert, because `http.Client` does jar send/store above the
  transport, and `WithClient` takes a Do-er, not a RoundTripper.
- `siteinfo.go` is the only hand-written parse: `__NEXT_DATA__` →
  `dehydratedState` `site-info` → `jobBoardId`. goquery to pull the script tag,
  `encoding/json` for the blob.
- Descriptions are HTML; convert with `jaytaylor/html2text`, already a repo
  dependency.
- `cultureCode` is pinned to `en-US` by default with an optional per-roster
  override, because a wrong culture returns an empty board instead of an error.
- Fixtures are real captures committed as hurl req/rsp pairs under
  `internal/provider/dayforce/testdata/`; `mocksrv.go` replays them;
  `make hurl-fmt` before commit. The search fixtures need the CSRF chain, so
  they use hurl `[Captures]` to take `csrfToken` from `/api/auth/csrf` and feed
  it into the POST (in-tree precedent: `internal/provider/apple/testdata/jobs_req.hurl`,
  `internal/provider/workable/testdata/jobs_req.hurl`).
- No `cmd/dayforce/doc.go`; the command comment sits atop `main.go`.

### Request headers (client and every fixture)

A package-level `userAgent`, mirrored verbatim into each `.hurl` req so live
replay matches the client:

```
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
accept: application/json          # API surfaces
accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8   # SSR site-info
```

### Stop conditions (halt and report; do not improvise)

- `/api/auth/csrf` stops yielding a usable token, or POST keeps returning
  `403` with a valid cookie+header pair — the whole Search path is gated on it.
- The `403` body stops being the bare `Forbidden` string, or Cloudflare starts
  challenging the API host (`403`/`429` with an HTML body) — the transport
  assumption changes.
- `paginationStart` stops paginating (repeated first page), or page size stops
  being 25 — pagination arithmetic is wrong and totals mislead.
- Unknown tenant / unknown posting id stop returning `404` (e.g. `200` with an
  empty body) — the not-found signal must be re-derived.
- A roster tenant returns `200 maxCount: 0` while its portal shows jobs — the
  `cultureCode` or `jobBoardCode` assumption for that tenant is wrong.
- `__NEXT_DATA__` disappears from the SSR board page, or `site-info` no longer
  carries `jobBoardId` — `Filters`/`Detail` lose their id source for
  non-roster boards.

### Rollback

S1–S3 add only `internal/provider/dayforce/` and `cmd/dayforce/`; nothing else
references them, so rollback is deleting those two directories plus the
`OPENAPI_SPECS` line. S4 is the first slice touching shared files
(`cmd/openings-mcp/main.go`, `internal/ats/registry.go`,
`cmd/verify-companies/main.go`, the collision golden file); rollback there is
reverting those edits and deleting `internal/ats/dayforce.go`.

## Slices

### S1 — provider package: spec, generated client, CSRF transport, fixtures

Scope: `internal/provider/dayforce/{doc.go,openapi.yaml,gen.go,client.go,siteinfo.go,mocksrv.go,*_test.go}`
plus `testdata/`, and the `OPENAPI_SPECS` Makefile entry.

`mocksrv.go` must be an `httptest.NewTLSServer`, with tests constructing the
client from `srv.Client()` — the gate cookie is `__Host-next-auth.csrf-token`,
`__Host-` cookies are `Secure`, and Go's `cookiejar` stores but never sends a
Secure cookie to an `http://` origin, so a plain `httptest.NewServer` mock
(smartrecruiters' pattern) would loop on `403` forever. apple hit and solved
exactly this: `internal/provider/apple/mocksrv.go:66,95`. The mock also
*requires* the cookie and the header on the search POST
(`internal/provider/apple/mocksrv.go:73-77`), so a regression in the transport
fails a Go test rather than only showing up live.

- `Search(ctx, SearchRequest) (*SearchResponse, error)` → POST search. Takes
  `paginationStart`, not a page number; the caller does the arithmetic so the
  fixed 25 page size stays visible at the call site.
- `Job(ctx, ns, culture, jobBoardID, postingID)` → GET detail.
- `Departments/PayClasses/PayTypes(ctx, ns, jobBoardID, culture)` → attribute
  lists.
- `SiteInfo(ctx, ns, xref, culture)` → SSR `__NEXT_DATA__` → `jobBoardId`.
- Typed errors distinguishing unknown tenant (`404` on search), unknown
  posting (`404` on detail) and the CSRF `403`.

Fixtures:

| fixture | target / observed | covers |
|---|---|---|
| csrf + search happy path (chained) | `pca` — `maxCount 352`, `count 25` | the CSRF chain and the default board |
| search filtered | `pca` + `searchText=electrical engineer` — `maxCount 115` | server-side keyword narrowing |
| search page 2 | `pca` + `paginationStart=25` — `offset 25`, different ids | pagination |
| search non-default board | `mydayforce` + `alljobs` — `jobBoardId 8`, `maxCount 140` | `xref` ≠ `CANDIDATEPORTAL`, `jobBoardId` ≠ 1 |
| search unknown tenant | `nosuchtenantxyz` — **404** | unknown company |
| search POST without CSRF | `pca` — **403** `Forbidden` | the gate itself, so a regression is loud |
| detail happy path | `pca/en-US/1/57127` — 200, `jobPostingAttributes` with `HiringMinRate` | detail parse incl. pay attributes |
| detail unknown id | `pca/en-US/1/99999999` — **404** | not-found |
| posting attributes | `pca` departments (id+value list), payclasses, paytypes | `Filters` source |
| SSR site-info | `/en-US/pca/CANDIDATEPORTAL` — `__NEXT_DATA__` `jobBoardId 1` | `jobBoardId` resolution |

Note for whoever captures these: postings expire, so prefer a posting with
`postingExpiryTimestampUTC: null` for the detail fixture, and re-verify the id
at capture time rather than reusing `57127` blindly.

Acceptance: `client_test.go` carries at least one assertion per fixture row
above, and by name:

- the search POST is served only when both the csrf cookie and
  `X-CSRF-TOKEN` reach the mock, and a POST without them yields the typed
  csrf error;
- page 2 returns `offset == 25` with a job-id set **disjoint** from page 1
  (proves `paginationStart` arithmetic, and fails if the parameter is ignored);
- the `mydayforce` / `alljobs` row yields `jobBoardId 8`, so the non-default
  board path is exercised rather than assumed;
- each of the three typed errors — unknown tenant, unknown posting, csrf
  `403` — is asserted as itself, not as a generic decode failure;
- `SiteInfo` returns `jobBoardId 1` for `/en-US/pca/CANDIDATEPORTAL`, and
  fails (not zero-values) when `__NEXT_DATA__` or the `site-info` query is
  absent.

Plus: `go test ./internal/provider/dayforce/...` green against mocksrv,
`make hurl-test` green including the chained CSRF file, `make hurl-lint` and
`make validate-openapi` clean.

### S2 — roster

Scope: `internal/provider/dayforce/{companies.yaml,companies.go,companies_test.go}`.

Row schema (mirrors ultipro's multi-key rows):

```yaml
- company: "Packaging Corporation of America"
  namespace: "pca"
  job_board_code: "CANDIDATEPORTAL"
  job_board_id: 1
  culture_code: "en-US"    # optional, defaults to en-US
```

Slug rule: the namespace, suffixed `-<lowercased xref>` when the board is not
`CANDIDATEPORTAL`, so a multi-board tenant like `mydayforce` can carry both
boards without colliding. Validated at init: non-empty company/namespace/code,
`job_board_id > 0`, unique slug and unique display name within this roster,
sorted by name.

Seed candidates, all confirmed live today (`200`, `jobBoardId 1`,
`maxCount` as shown). The API never returns the employer name, but every name
below was then read out of that tenant's **own live posting text**, which is
first-party enough to seed on — the quoted fragment is the evidence:

| namespace | maxCount | name | evidence in posting text |
|---|---|---|---|
| `pca` | 352 | Packaging Corporation of America | "As a Fortune 500 company, Packaging Corporation of America (PCA)…" |
| `jdemea` | 122 | JD Sports Fashion | "the JD Group is a leading omni-channel retailer of Sports Fashion"; another posting names "JD Sports Fashion plc" |
| `mymilacron` | 49 | Milacron | "leading and executing complex cross-functional projects across Milacron" |
| `wikoff` | 29 | Wikoff Color Corporation | "At Wikoff Color Corporation, we're proud to be employee-owned" |
| `avgroup` | 22 | AV Group NB | "AV Group NB Inc. Atholville Mill is currently recruiting…" |
| `nam` | 4 | National Association of Manufacturers | posting opens with the full name |

All six names were checked against the 39,247 display names already in the
other rosters (normalised to lowercase alphanumerics, the way the registry
does) and **none collides**.

Two otherwise-good candidates were deliberately left out of the seed for that
reason: `corpay` (141 jobs) collides with workday's "Corpay", and `kiinc` (75)
normalises onto workable's "Ki". Cross-adapter collisions are permitted in
principle — `Registry.Resolve` disambiguates by careers URL and they only need
to land in the collision golden file — but this repo already has a standing
cross-adapter display-name collision problem, so the seed deliberately avoids
adding to it. Both are fine candidates for the deferred bulk expansion once
that is settled.

One name still needs a human call in S2: `jdemea` is JD's **EMEA** board, so
decide whether the display name should carry that qualifier. It does not
block S1.

Also verified live and available if a seed row is dropped: `ipg` (131),
`medplast` (84), `paradigm` (45), `opta` (13), `thechronicle` (2, "The
Chronicle of Higher Education" — collision-free). `ipg`, `medplast`, `opta`
and `paradigm` postings do not name their employer in the text, so those need
a separate name source.

Include `mydayforce`+`alljobs` only if S1's non-default-board path should be
smoke-tested through MCP too.

The seed spans a 4 → 352 job range so pagination and single-page boards are
both exercised end to end.

Acceptance: init validation passes; no slug or display-name collision inside
this roster; cross-adapter name collisions are expected and only need to land
in the `cmd/openings-mcp/testdata/company_collisions.txt` golden file.

### S3 — debug CLI

Scope: `cmd/dayforce/main.go` (ff/v4; `search`, `detail`, `companies`),
mirroring `cmd/smartrecruiters` — validated pagination flags, stray positional
args rejected, `--format text|json`.

Acceptance: all three subcommands return live data, including a page-2 search.

### S4 — MCP surface

Scope: `internal/ats/dayforce.go` + `dayforce_test.go`; registration in
`atsAdapters`/`newATSRegistry` (`cmd/openings-mcp/main.go`); host pattern in
`careersHostPatternsByAdapter` (`internal/ats/registry.go`); `providerOrder`
and the `buildAdapters` switch in `cmd/verify-companies/main.go`; regenerated
collision golden file.

- `Search` → server-side search. `SearchParams.Page` → `paginationStart =
  (Page-1)*25`; `TotalCount` = `maxCount`; `TotalPages` = `ceil(maxCount/25)`.
  `Query` → `searchText`, `Location` → `locationString` (+ default distance),
  `Filters` → `departmentId` / `payClass` / `payType` / `travelRequired`.
  `JobSummary.URL` = `https://jobs.dayforcehcm.com/{culture}/{ns}/{xref}/jobs/{postingId}`;
  `JobID` = the posting id; `Location` from `postingLocations`
  (`hasVirtualLocation` → remote).
- `Filters` → department / pay-class / pay-type dimensions from the
  posting-attribute endpoints, keyed by `attributeValue` and sent back as
  `attributeId`.
- `Detail` → GET detail, `jobPostingContent` header+body+footer through
  html2text; `Company` from the roster row.
- `ParseCareersURL` accepts, case-insensitively:
  `jobs.dayforcehcm.com/{culture}/{ns}/{xref}`,
  `jobs.dayforcehcm.com/{ns}/{xref}` (locale-less form is live, e.g. `corpay`),
  either with a `/jobs/{id}` suffix, plus the legacy
  `us<NNN>.dayforcehcm.com/CandidatePortal/{culture}/{ns}[/Site/{xref}]` and
  `www.mydayforce.com/CandidatePortal/...` forms that redirect to the new host.
  Non-roster boards resolve to a canonical `{ns}/{xref}` slug, ultipro-style,
  with `jobBoardId` filled from `SiteInfo`. Must not swallow `/api/...`,
  `/_next/...`, `/profile`, or the bare host.

Acceptance: live MCP stdio smoke test — each seed company returns listings via
`search_jobs_by_company`, `get_filters_by_company` returns the three
dimensions, and `get_job_detail_by_company` resolves a returned `job_id`.

### S5 — docs

README provider list; `cmd/openings-mcp` server instructions only if
tool-selection guidance changes.

## Decode-strictness decision (added 2026-08-11, after S4)

The integration was refuted twice on the same class of defect, so the approach
changed. Recording it here so it is not re-litigated.

**What kept happening.** `openapi.yaml` is hand-derived, and ogen generates a
strict decoder: one field whose live value does not match its declared type
fails the **whole response**, so a single bad row takes out an entire 25-item
page or a whole detail call. Three rounds of "widen the field that threw" each
uncovered another field:

| round | field | how it failed |
|---|---|---|
| 1 | `SearchPostingLocation.isoCountryCode` | null on a continent-level location (mymilacron 2640) |
| 2 | `JobPostingDetail.isoCurrencyRegion` | null on 9 of 10 tenants; PCA, the only fixture, is the outlier |
| 2 | `SearchPosting.postingLocations` | the **array itself** null (corpay, jdemea, mymilacron) |
| 2 | `JobPostingAttributeValue` | `oneOf` lacked `boolean` (mymilacron 2684, `type: "bool"`) |
| 3 | `JobPostingContent.jobDescriptionFooter` | null on roster tenant avgroup (2482, 2442) |
| 3 | `JobPostingDetail.assessmentSentType` | declared nullable **string**, live returns integer `3` (corpay) |

**Why the sweeps kept missing them.** The sweeps detected *nulls*. That is the
wrong invariant twice over: a field null in every sampled tenant still has an
unknown non-null type (`assessmentSentType` was typed `string` on no evidence
at all), and a field non-null in the sample can be null elsewhere
(`jobDescriptionFooter`). What the strict decoder actually needs is the
**observed type set per JSON path**, over a corpus large enough to see the tail.

**Decision (repo owner, 2026-08-11): declare only the fields we consume.**
ogen ignores undeclared response fields, so a field absent from the spec cannot
break decoding. The candidate-portal's own apply/assessment/render state is
dropped from the schemas; job content and metadata stay and get their types
pinned from live evidence.

Options considered and rejected:

- *Full type-union sweep over a large corpus, keep every field.* Correct, and
  the only option that preserves the "surface every upstream field" habit, but
  it buys exhaustive fidelity for fields nothing reads, and every one of them
  stays a way for a future tenant's data to break a page.
- *Patch these two and ship, with the decode hunt as a guard.* Fastest, but
  leaves the residual risk with whoever hits it next.

The trade-off accepted: dropped fields are no longer visible to callers, which
runs against the repo's usual preference for surfacing everything upstream
returns. Anything dropped is one spec addition away from coming back — with the
type confirmed from live data rather than guessed.

### Verification recipe (the check that finds these)

The only thing that reliably found these defects was walking **every page of
many tenants** plus a spread of **detail** objects through the real client and
watching for decode errors — unit fixtures cannot, because the failures are
data-dependent. Sample well beyond the roster: the corpay, avgroup and
mymilacron rows that broke decoding were the only such rows on their boards.

## Deferred (not in this plan)

Bulk roster expansion via `unverified/dayforce.yaml` + `cmd/verify-companies`
(pipeline step 7). Harvesting is proven but manual (search-engine driven, no
tenant index), and display names need human curation — a separate roster PR
under the `roster:` commit convention.

## Risks noted

- **CSRF is the single point of failure.** Everything list-side depends on
  `/api/auth/csrf` + cookie. It is a stock next-auth double-submit token, not
  an anti-scraping measure, but it is one more thing that can change under us.
  The `403`-without-CSRF fixture exists so a regression fails loudly.
- **Fixed 25 page size** means a large board costs `ceil(n/25)` requests to
  walk. Fine for per-company search; a consideration if anyone later wants a
  full dump for caching.
- **Postings expire** (`postingExpiryTimestampUTC`), so detail fixtures rot.
  Same class of problem as the mynavi detail fixture.
- `jobs.dayforcehcm.com` sits behind Cloudflare (`__cf_bm` is set on every
  response). Plain curl with a browser user-agent is not challenged today;
  aggressive parallelism could change that.
