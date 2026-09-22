# adp_wfn provider — ADP Workforce Now career centers

Design record for integrating ADP Workforce Now (`workforcenow.adp.com`) as
an ATS adapter. Written 2026-08-14, before implementation.

`internal/provider/adp_myjobs/doc.go` and `internal/ats/adp_myjobs.go`
reserved the name `adp_wfn` for this surface; this plan makes it real.

Every fact below was verified live against the public endpoints. Where a
first-pass conclusion was later corrected, the correction is noted — several
of the traps here were only found because an earlier reading was wrong.

## Scope

- **In**: the public career-center JSON API behind
  `workforcenow.adp.com/mascsr/default/mdf/recruitment/recruitment.html`.
- **Out**: ADP MyJobs (`myjobs.adp.com`, already served by `adp_myjobs`),
  developers.adp.com partner/HCM APIs, and the internal employee career
  center (`ccId=19000101_000002`, which answers HTTP 403 "Job listing is not
  allowed for external candidates").

## The surface

Public JSON REST. No auth, no token, no cookies, `Access-Control-Allow-Origin: *`.
Empty UA, `curl`, and `ClaudeBot` all receive HTTP 200. The HTML page is an
empty SPA shell — byte-identical across tenants, `<title>Recruitment</title>`,
the `cid` does not appear in it — so JSON is the only surface. No WAF, no bot
detection, no rate limiting observed (30 sequential and 12 concurrent requests
all returned 200).

Base: `https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing`

| Purpose | Path |
|---|---|
| Job list | `GET /v1/job-requisitions` |
| Job detail | `GET /v1/job-requisitions/{itemID\|ExternalJobID}` |
| Location + job-type facets | `GET /v1/job-requisitions/getSearchFilters` |
| Tenant identity and features | `GET /client-features` |
| Branding, locale set | `GET /v1/content-links/career-center` |

`client-features` sits directly under `staffing/`, **not** under `/v1/`. That
is why an endpoint sweep of the `/v1/` prefix missed it; it was found by hand.

### robots.txt

`workforcenow.adp.com/robots.txt` 302s to the SiteMinder login page for both
a browser UA and Googlebot, and `static.workforcenow.adp.com` returns 404. No
crawl policy is published on this host — there is no `Disallow` to honor and
no permission to point at. The data is unauthenticated public job postings.
Decision: proceed with polite pacing (Q11).

## Tenant identity

`cid` (a GUID) is the only required parameter and the only real tenant key.
The JSON API rejects everything else: `?client=novae` without a `cid` returns
HTTP 500 on both `job-requisitions` and `client-features`.

`ccId` is a career-center id. It is **not** always `19000101_000001` —
PolicyLink publishes `19000101_000003`, and for that tenant `_000001` and
`_000003` return the same board while `_000004` returns zero rows. Omitting
`ccId` worked on every tenant tested. A *wrong* `ccId` returns HTTP 200 with
an empty array rather than an error.

`locale` is a silent per-tenant filter — see the locale section below.

### Slug → cid resolution (curation only)

The legacy career center was decommissioned 2026-06-26 and
`posting.html?client=<slug>` appears dead: without a cookie jar it redirects
to itself until the client gives up. **With** a cookie jar it completes a
two-hop handshake and lands on
`intermediateRedirect.html?cid=<guid>` — a working slug → cid resolver.

Measured 1.19–1.54 s over five runs. Coverage is partial: `novae`,
`asd-1817`, `xip39`, `hyattlegal`, and `1euxj` resolve; `pcliving` does not
(it stops at `posting.html` after ~0.7 s).

Combined with `client-features` (cid → `ClientID`), tenants can be discovered
in both directions. This is the only scalable WFN discovery path known —
`site:workforcenow.adp.com` dorks return nothing because the shell has no
indexable content, no sitemap is served, and Google's index carries only
generic titles like "Job Details | Recruitment".

## Search shape

**Server-side paging, not a full dump.** The board paginates natively, so the
adapter passes pagination through rather than synthesizing a dump (Q13).
`$top` caps at 20, which happens to equal the repo's uniform `pageSize`, so
unfiltered paging is a 1:1 mapping.

- `$skip` is **1-based**. `$skip=0` silently drops a row.
- Correct paging is `$skip = (page-1)*20 + 1` → 1, 21, 41, …
- `$top` is honored **only when unfiltered**. Under any filter it is ignored
  entirely, so always step by exactly 20 or rows are refetched.
- Stop on an empty `jobRequisitions` array.

### What filters actually work

Every query-param filter is silently ignored — `$search`, `searchText`,
`keyword`, `$filter=…`, and `fooBarBaz=quux` all return the identical
unfiltered board with HTTP 200. The same trap `adp_myjobs` documents.

Real filtering is split between one query param and two HTTP **headers**,
found in the SPA bundle's `getJobRequisition` and confirmed live:

| Control | Where | Behavior |
|---|---|---|
| `userQuery` | query param | full-text over description, tokenizes with OR, quoted phrases work, case-insensitive, unmatched → 0 rows |
| `locationsList` | header | works; pair-based grammar, see below |
| `workerCategoriesList` | header | works; takes facet `oid` values |
| `sortBy` / `sortOrder` | header | no observable effect |

All three compose as a true AND.

`workerCategoriesList` was first read as broken. It is not — Novae has
exactly one job type, so filtering by it returns the whole board and looks
inert. Verified working on two other tenants.

### The two silent-fallback hazards

`locationsList` is a flat comma-separated token stream consumed in
`<value>,<qualifier>` **pairs**; multiple pairs OR together, and a dangling
unpaired token is dropped.

| Header | Result |
|---|---|
| `Markle, IN` / `IN,LOCATION_STATE` / `CA,LOCATION_COUNTRY` | filtered rows |
| `Atlantis, ZZ` / `ZZ,LOCATION_STATE` / `IN,LOCATION_BOGUS` | 0 rows — **safe** |
| `Markle` (no comma) / `!!!!` / `12345` / empty | **whole unfiltered board** |

`workerCategoriesList` has a related but worse hazard, corrected during
implementation — recon had reported it as a whole-board fallback, and it is
not. An oid the tenant does not publish returns neither an error, nor an empty
set, nor the whole board: on a 21-posting board, `9:999`, `garbage`, and a
display label each returned the **identical 19 rows**, with an inflated total
of 23. The tenant's own four oids partition that board exactly (16+2+2+1=21).

A whole-board fallback is at least recognisable as unfiltered. A 19-of-21
result looks like a filter that worked, which makes validating against the
published oids more important here, not less.

So a value that fails to parse is answered with an unfiltered board
masquerading as a filtered result. **Never send an unvalidated value.** For
locations, require a comma-formed pair; for job types, require an oid from
`getSearchFilters`.

Qualifiers: `LOCATION_CITY`, `LOCATION_STATE`, `LOCATION_COUNTRY` are real;
`LOCATION_POSTAL` is not. `LOCATION_STATE` needs the two-letter code
(`Indiana,LOCATION_STATE` → 0). A bare two-letter code also works as a
qualifier (`Markle, IN` ≡ `Markle,LOCATION_CITY`). Case and surrounding
whitespace are tolerated.

### getSearchFilters inverts its two facets

Verbatim from the SPA bundle:

```js
// FILTER LOCATION      → send .value
transformSearchLocationData: a.label=t.oid, a.value=t.value
// FILTER JOB TYPE      → send .oid
transformSearchWorkerCategoryData: a.label=t.value, a.value=t.oid
```

Location values ship already pair-formed (`"Markle, IN"`, `"IN,LOCATION_STATE"`),
so they go into the header verbatim. Job types send the `oid` (`7:132`,
`9200345894316_1`); sending the human label returns the whole board.

Job-type oids are tenant-specific and unjoinable across tenants, and their
labels are free text (`"FT"`, `"Full Time"`, `"Active - Regular full-time"`),
so no cross-tenant normalization is possible. Same per-tenant shape as
`adp_myjobs`.

A tenant may publish **no** `FILTER LOCATION` facet and still answer
`locationsList` — Bingemans does. Absence of the facet is not absence of the
capability; it only means there is no published list to validate against.

### userQuery cannot be paged

Ordering is **nondeterministic across identical calls** — ten identical
requests produced two distinct orderings of the same stable set, consistent
with tie-breaking on equal relevance scores across backend nodes. Windows
therefore overlap: walking `$skip=1,21,41,61` on `userQuery=manager` fetched
40 rows containing only 39 unique ids. Precision also collapses — page 1 is
Managers, page 3 is Welders, page 4 is Painters.

Decision (Q22): keyword search serves **page 1 only**, up to 20
relevance-ranked hits, `total_pages: 1`. Paging with dedupe would be silent
incompleteness, which this repo does not do. `location` and `job_type` page
soundly and are the tool for completeness.

### meta.totalNumber lies under filters

Trustworthy only on an unfiltered call. Under any filter it becomes a
relevance tally that routinely exceeds the board: `userQuery=manager` → 5
rows with `total=82`; `userQuery=IN` → 0 rows with `total=50`;
`locationsList: Markle, IN` → 11 rows with `total=28`; a no-op header → 133
against a 48-row board.

Decision (Q18): use the avature lower-bound pattern
(`internal/ats/avature.go:152`) — report the count the walk proved, `+1` when
a next page exists so `total_pages` still signals more. Filtered paging
terminates cleanly on an empty array, so this is implementable.

## Locale

A tenant's jobs live in **exactly one** locale. Every other locale the tenant
advertises returns an empty array — not partitioned, not translated, empty.

| Tenant | Job-bearing | Other advertised |
|---|---|---|
| Novae | `en_US` (48) | `es_US` → 0 |
| Bingemans | `en_CA` (15) | `en_US` → 0, `fr_CA` → 0 |
| NSU | `en_US` (11) | `es_US` → 0 |

Intersections are empty, so reading only the primary locale loses nothing.
Store a **single** locale string, not a list (Q29) — a list would encode
locales guaranteed to return nothing and would tempt a fan-out that triples
request volume for zero additional jobs.

Two behaviors, and the difference matters:

- A recognized locale is applied as a **strict filter**. Novae with `de_DE`,
  `es_US`, `fr_CA`, or `en_CA` → 0 rows, HTTP 200, no error. Sending the
  wrong locale is a silent total loss, indistinguishable from a tenant with
  no open jobs.
- An unrecognized or absent locale **falls back to the server default**,
  which is `en_US`. So omitting `locale` is *not* a safe default: it silently
  returns zero for any tenant whose board is not `en_US`.

`locale` is therefore **required** in `companies.yaml`, not optional with a
default — `mustLoadCompanies` panics when it is missing.

### Discovering the locale of an unlisted tenant

`content-links/career-center` repeats a `Locale` string field once per
supported locale, and the **first** entry was the job-bearing one in all four
tenants checked. It needs only `cid`.

`client-features.ClientDefaultLocale` is **not** usable for this: it reports
`en_US` for Bingemans, whose jobs are `en_CA`-only.

## Field inventory

**List row**: `itemID`, `requisitionTitle`, `postDate` (ISO-8601 with
offset), `clientRequisitionID`, `requisitionLocations[]`
(`address.cityName`, `address.countrySubdivisionLevel1.codeValue`,
`address.postalCode`, `nameCode.shortName` = `"Site, City, ST, US"`),
`payGradeRange.{minimum,maximum}Rate.{amountValue,currencyCode}`,
`workLevelCode.shortName`, and a `customFieldGroup` bag keyed by name
(`ExternalJobID`, `HomeDepartment`, `JobClass`, `SalaryRange`, `SalaryType`,
`InternalPostingFlag`, `ApplicantCount`, …).

**Detail** adds exactly one field over the list row: `requisitionDescription`,
an HTML blob. That is the only reason to make a detail call.
`organizationalUnits`, `postingInstructions`, `links`, and
`screeningRequirements` are empty arrays on both list and detail, on every
record checked.

`payGradeRange` and `workLevelCode` are per-row optional and **identical
between list and detail** (an earlier reading called them list-only; that was
a comparison against a row that simply lacked them). Sparse — 6/48 on Novae,
0/21 on ASD. There is no way to conjure salary onto a row that lacks it:
`$select`, `$expand`, and `includeSalary` are all silently ignored like every
other query param.

**No company name anywhere** in `job-requisitions`. It comes from
`client-features` or the roster.

### Ids

`ExternalJobID` is present on 100% of rows on both tenants swept, so it is
safe as the public-URL key with no fallback. Detail accepts either `itemID`
or `ExternalJobID`; `clientRequisitionID` is a decoy that returns a hollow
stub with no `itemID` rather than an error.

Canonical human URL, from the bundle's `getShareUrl`:

```
https://workforcenow.adp.com/mascsr/default/mdf/recruitment/recruitment.html?cid=<cid>&ccId=<ccId>&jobId=<ExternalJobID>&lang=<locale>
```

`jobId` is the `ExternalJobID`. There is no separate apply URL — Apply is an
in-SPA action.

## Tenant naming

`client-features` returns `ClientName`, `ClientID`, and feature flags for a
bare `cid`:

| Tenant | `ClientName` | `ClientID` |
|---|---|---|
| Novae | `NOVAE LLC` | `novae` |
| American School for the Deaf | `American School for the Deaf` | `asd-1817` |
| Bingemans | `Bingemans Inc` | `1euxj` |
| On-Site Health & Safety | `On-Site Health & Safety` | `xip39` |
| NSU | `Applied Water Managemen` | `awminc` |
| MetLife Legal Plans | `Metlife Legal Plans` | `hyattlegal` |
| PCL | `Partnerships In Community` | `pcliving` |

Quality is uneven: truncation (`Applied Water Managemen`,
`Partnerships In Community`), casing (`Metlife` for MetLife, `NOVAE LLC` all
caps), and one entry that is a different legal name than the trading name.

Decision (Q42): emit `ClientName` **verbatim**, no prettifying. It is ADP's
own record of the client, so this is the same "pass the upstream's name
through" model as `engage`, `mokahr`, and `smartrecruiters`. Correcting it
would be manufacturing.

### Paths that do not work, and why they were considered

Recorded so this is not re-litigated:

- `content-links/career-center` has **no** name field at any depth — verified
  by key-path union across seven tenants, by grepping the whole SPA bundle
  for a dozen candidate key names (zero hits), and by the observation that
  the page renders its company header as an **image**.
- Twenty-one guessed `/v1/` paths (`/client`, `/organization`, `/company`,
  `/branding`, …) all return HTTP 500. A name-bearing `/v1/` endpoint does
  not exist.
- The HTML shell is byte-identical across tenants; no sitemap; no
  server-rendered content anywhere.
- The `IMG_LOGO` blob renders a legible wordmark for 8/8 tenants and is the
  only source that identifies the two tenants no text field covers. It
  requires OCR, so it is a **curation-time** tool, not a runtime path — the
  Go server will not carry a vision dependency to fill one `omitempty` field.
- LinkedIn slug (5/8), logo filename (4/8), and the legacy `client=` slug
  (4/8, and only 1 of 4 resembles its company name — `xip39` is On-Site
  Health & Safety, `awminc` is Natural Systems Utilities) were all considered
  and dropped once `client-features` was found.

A misidentification during recon — an Oregon nonprofit read as PCL
Construction from a `facebook.com/pclrecruiting` link — is why roster names
are cross-checked rather than taken from a single signal (Q41).

## Decisions

| # | Decision |
|---|---|
| Q1 | Target is ADP Workforce Now, package `adp_wfn` |
| Q2 | New package + new adapter; `adp_myjobs` untouched. Options surveyed: one merged `adp` package serving both surfaces — rejected, different tenant identity, endpoints, and filter models |
| Q3 | `internal/ats.Adapter`, joins the unified company tools |
| Q4 | Done means wired into `cmd/openings-mcp` and smoke-tested over stdio |
| Q5 | Seed roster only; bulk discovery is a separate session and a separate `roster:` PR |
| Q6 | `doc.go` **and** `CONTEXT.md` **and** an ADR |
| Q7 | `CONTEXT.md` is repo-wide, seeded from vocabulary the code already uses |
| Q8 | `docs/adr/` for cross-cutting hard-to-reverse calls; `docs/superpowers/specs/` for per-provider surveys |
| Q9 | ADR 0001 covers the two-ADP-packages split |
| Q10 | One plan doc, one branch, one PR |
| Q11 | Proceed; no robots policy exists on this host, data is public, pacing stays polite |
| Q12 | ogen from a hand-written `openapi.yaml`; header filters declared as header params |
| Q13 | Server-side paging, **not** a synthesized full dump |
| Q14 | Superseded by Q43 (slug is no longer the bare cid) |
| Q15 | `JobID` is `ExternalJobID`; no fallback needed (100% coverage) |
| Q16 | Seed roster from the six tenants verified with live jobs |
| Q17 | Superseded by Q19 |
| Q18 | avature lower-bound pattern for `total_count` under filters |
| Q19 | Prefix the description with a salary line from `payGradeRange` when present; extending `ats.JobDetail` with a real salary field is follow-up work |
| Q20 | `Location` → facet value verbatim when it matches, else construct a pair; never emit an unpaired token |
| Q21 | `Filters()` exposes `location` + `job_type`, rejects unknown values loudly; values OR within a key |
| Q22 | Keyword search is page 1 only |
| Q23 | Superseded by Q44 |
| Q24 | Call `content-links` for unlisted tenants — later refined by Q42 once `client-features` was found |
| Q25 | Capture all six trap fixtures as separate hurl pairs |
| Q26 / Q28 | **No** cache. Correctness never needed it, and the cost is a constant +1 request, not an N+1. `DumpCache` exists for full-dump adapters, which Q13 made us not be. Adding caching later is a pure optimization with no API change |
| Q27 | A failed branding call degrades to an empty `Company`; it never fails `Detail` |
| Q29 | Roster stores a single required `locale`; unlisted tenants read `lang=` from the URL, else ask `content-links` |
| Q30 | Superseded by Q40 |
| Q31 | `CareersURL` and `JobDetail.URL` use the tenant's own locale, not a hardcoded `en_US` |
| Q32 / Q36 / Q37 | Superseded by Q42 |
| Q33 | `content-links` is called for unlisted tenants, for locale |
| Q34 / Q41 | Seed roster names are cross-checked against at least two independent sources; single-source entries are dropped, not guessed |
| Q35 | A roster candidate returning zero jobs fails **verification**; runtime `Search` still returns empty normally |
| Q38 | Roster tenants resolve to their curated slug; unlisted tenants get a canonical URL as their slug, carrying cid/ccId/lang — the workday/ultipro/oracle shape |
| Q39 / Q43 | Curated readable slug in `companies.yaml`; `cid`, `client_id`, `ccId`, `locale` are fields |
| Q40 | Omit `ccId` from API requests; keep it in the roster for rendering human URLs |
| Q42 | `Company` for unlisted tenants is `ClientName`, verbatim |
| Q44 | `ParseCareersURL` accepts the legacy `posting.html?client=<slug>` form and resolves it |
| Q45 | 3-second timeout on that resolution |

### Why no cache (Q26 / Q28)

Worth recording because the neighbouring adapter does the opposite.
`adp_myjobs` borrows `DumpCache` for its filter catalog. Here the cost of not
caching is one extra request on a filtered search and one on a detail call —
a constant, not a multiplier. The one place it compounds is paging a filtered
search, where the catalog is re-fetched per page. Starting without a cache is
strictly simpler and adding one later changes no API; starting with one bakes
in the `DumpCache` coupling and makes `--dump-cache-ttl` govern two unrelated
things.

## Roster schema

```yaml
- company: Novae Corporation
  slug: novae
  cid: d16ba13d-474e-4326-b628-74c87f0b289a
  client_id: novae
  ccId: '19000101_000001'
  locale: en_US
```

`slug` is curated by us, not taken from ADP. The reason is
`Registry.suggest` (`internal/ats/registry.go:300`): the suggestion pool is
built from **slugs only, never names** (`registry.go:177`). A GUID slug would
put every WFN company into that pool as a 32-char hex string, so a user who
typos a company name would never see a WFN suggestion, and the error message
— "pass one of the suggested slugs" — would be telling a human to type a
GUID. ADP's own `ClientID` is no better for three of seven tenants (`1euxj`,
`xip39`, `awminc`).

`client_id` is stored anyway because it is the key to slug → cid resolution
for future bulk discovery.

Exact-name lookup via `byName` is unaffected by any of this; only the
near-miss path is.

**Follow-up worth doing separately**: teach `Registry.suggest` to rank names
as well as slugs. `byName` already exists, so a suggestion pool that ignores
it is a latent gap for every adapter, not just this one. It should not ride
along in this PR.

## Seed roster

Six tenants, live-verified, each name cross-checked against at least two
independent sources:

| Company | cid | locale | Jobs |
|---|---|---|---|
| Novae Corporation | `d16ba13d-474e-4326-b628-74c87f0b289a` | en_US | 48 |
| American School for the Deaf | `55a9d987-168e-4cd9-9f4d-bc3c10fb6e90` | en_US | 21 |
| On-Site Health & Safety | `284521e1-407f-4b01-862b-9b67eff6cd18` | en_US | 16 |
| Bingemans | `5a312740-8c7b-44b7-80ad-50a35fbb25d3` | **en_CA** | 15 |
| Natural Systems Utilities | `fb30d7bb-6b35-4ece-97a2-d094abe83a7f` | en_US | 11 |
| MetLife Legal Plans | `269ead05-4042-4cc5-b325-75a73a6e0439` | en_US | 2 |

Bingemans earns its place specifically because it is the only `en_CA` tenant
— it turns the locale trap into a test case instead of a comment. MetLife
Legal at two jobs guards the small-board edge.

`599f5d1f-…` (Partnerships in Community Living) is excluded: its board is
empty, which per Q35 fails verification.

These are small firms rather than household names. A recognizable roster is
the later bulk-discovery session's job.

## Fixtures

Beyond the standard happy path / detail / not-found / unknown-tenant, capture
each trap as an executable fixture, in the dayforce style of separate small
`_req.hurl` / `_rsp.json` pairs:

1. `locationsList` no-op — a bare unpaired token returning the whole board
2. a well-formed pair with bogus values returning zero rows
3. `$skip=1` vs the `$skip=0` off-by-one
4. inflated `meta.totalNumber` under a filter
5. the `en_CA` board (Bingemans) returning zero on `en_US`
6. `getSearchFilters` for a tenant publishing no location facet

Assert on structure rather than exact counts where the count is volatile, so
a drifting live board does not invalidate the shape being tested.

## Implementation stages

1. `openapi.yaml` covering the five endpoints, with the header filters as
   header params and every trap documented in operation descriptions. Add to
   `OPENAPI_SPECS`, `make validate-openapi`, `go generate`.
   - 404 on `content-links` returns **nginx HTML, not JSON** — the strict
     decoder must not be handed it.
   - A bad `ccId` returns HTTP 200 with `contentLinks: []` and
     `meta…PublishedIndicator: false`, which is a usable sentinel.
2. `internal/provider/adp_wfn`: client, `companies.yaml` + `companies.go`,
   `mocksrv.go`, `client_test.go`, `doc.go`, fixtures.
3. `cmd/adp_wfn` with `search`, `detail`, `companies` subcommands, mirroring
   `cmd/smartrecruiters`. No `cmd/adp_wfn/doc.go`.
4. `internal/ats/adp_wfn.go` implementing `Adapter` + tests. Register in
   `newATSRegistry`, add the host pattern to `careersHostPatternsByAdapter`,
   and add to `cmd/verify-companies` in **both** places it needs: the
   `providerOrder` list *and* the `buildAdapters` switch. Listing it in only
   the first leaves a nil adapter, and because `providerOrder` is also the
   `--provider` default, that panics every invocation of the tool rather than
   only the new provider's — which is exactly what happened here.
5. Live MCP smoke test over stdio across the sampled roster companies.
6. README provider list; update the two `adp_myjobs` comments that call
   `adp_wfn` a future package.

### ParseCareersURL

```
recruitment.html?cid=<guid>[&ccId=][&lang=][&jobId=]   → current form, jobId ignored
posting.html?client=<slug>                             → resolved, see below
```

Roster tenants (matched by cid via a `CompaniesByCID` index) resolve to their
curated slug. Unlisted tenants get a canonical URL as their slug so that
`cid`, `ccId`, and `lang` survive into `Search` and `Detail` — without this
the `lang=` on the input URL is lost the moment `ParseCareersURL` returns,
and a tenant like Bingemans silently reports an empty board.

The legacy form is resolved inside `ParseCareersURL` with
`context.WithTimeout(context.Background(), 3*time.Second)` — the interface
takes no `ctx`, and changing it would touch every adapter. A temporary client
carrying a `cookiejar` wraps the shared `Transport`; the jar must not be
attached to the shared client or cookies leak into every other provider's
requests. A slug that does not resolve returns `false`, and the error message
directs the caller to the current URL form. Two consequences are accepted:
the call is uncancellable by the MCP client, and the worst case adds ~3 s to
an input form ADP has already decommissioned.

## Follow-up work, deliberately not in this PR

- Extend `ats.JobDetail` (and possibly `JobSummary`) with a structured salary
  field. WFN publishes `payGradeRange` on every row that has it and the
  unified schema has nowhere to put it, so today it is flattened into the
  description text. This touches every adapter.
- Teach `Registry.suggest` to rank names as well as slugs.
- Bulk roster discovery using the `ClientID` → cid resolver plus
  `client-features`, with logo OCR for the tail of tenants no text field
  identifies.
