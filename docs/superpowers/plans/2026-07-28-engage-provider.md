# engage (エン・ジャパン) provider integration

Date: 2026-07-28
Provider key: `engage`
Host: `en-gage.net`

## Recon summary

engage is エン・ジャパン's free ATS. Every tenant gets a hosted careers site at
`en-gage.net/<slug>/`, and all tenants are also pooled into one aggregator
search. Structurally this is HERP: one adapter, many tenants, slug discovery
from a central index.

### Surfaces found (all public, no login, no browser required)

| Surface | Shape | Verdict |
|---|---|---|
| `GET /user/api/search/result_work_list/` | JSON, 60/page, 1.03M jobs | aggregator search; **no slug filter** |
| `GET /<slug>/` | HTML board, **capped at 100/category** | **adapter Search source** |
| `GET /<slug>/work_<id>/` | HTML + schema.org JSON-LD `JobPosting` | **adapter Detail source** |
| `GET /sitemap_company_000{1,2}.xml.gz` | sitemap | roster discovery |
| `GET /user/search/desc/<id>/` | HTML | duplicate of tenant detail; unused |

### Board cap — measured, not assumed

The tenant board is **not** a complete dump. Measured by counting distinct
`/<slug>/work_<id>/` links per `dt.category` block:

| slug | per-category counts |
|---|---|
| `cookbiz_jobs` | 中途採用 **100** |
| `nkk-group_jobs` | アルバイト・パート採用 **100** |
| `nova_career` | 中途採用 **100**, アルバイト・パート採用 7 |
| `aspark-tokyo` | 派遣社員 **100**, 中途採用 10 |

100 is a hard per-employment-category ceiling. There is **no pagination**:
`?page=2`, `?p=2`, `?offset=100`, `?limit=200`, `?work_page=2` all return a
byte-identical id-set (verified by hashing the sorted id list).

The truncation is a contiguous id window, and jobs exist outside it: the
`cookbiz_jobs` board covers ids 16410854–16411201, while the aggregator
returns six `cookbiz_jobs` ids (16409440, 16414567, 16414576, 16415062,
16590480, 16650089) that fall outside that window — **zero overlap** — and all
of them resolve `200` at `/cookbiz_jobs/work_<id>/`. So the board silently
drops real postings once a category exceeds 100.

**Impact is small but real.** In a 40-slug sample drawn from
`sitemap_company_0001.xml.gz`, median board size is **3 jobs** and exactly
**1 of 40 (2.5%)** hit the cap. The ceiling only bites large staffing firms.

**Decision: accept the cap, and surface it doc-side only.** This follows
existing repo precedent for lower-bound totals (avature; herp's roster
enumeration capping at 1000 of 2198).

Concretely — and this is the single mechanism, chosen so no shared type
changes:

- `ats.SearchResult.TotalCount` stays exactly what `searchDump` computes
  (`len(matched)`). **No new field, no flag, no `internal/ats` type edit**, so
  the rollback statement below stays accurate.
- No phantom-page nudge. Avature's `total++` works because a real next page
  exists; here nothing lies beyond the cap that we can fetch, so inflating the
  count would invent a page that returns nothing.
- `Board` returns a per-category "hit the ceiling" signal. Its consumers are
  (a) `doc.go`, which states the ceiling and that counts are lower bounds for
  capped tenants, and (b) `cmd/engage`, which prints an explicit cap warning
  in `search` output. Unit tests assert the signal fires for `cookbiz_jobs`
  and not for `benefitone_saiyoujyouhou`.

If a future slice decides the cap must be MCP-visible, that is a change to
shared `internal/ats` types and belongs in its own plan.

**Rejected alternative — union the aggregator into Search.** The aggregator
could be queried by `companyName=<roster name>` and filtered client-side to
the slug (for cookbiz this does return exactly the 6 missing jobs). Rejected
because: `companyName` is a fuzzy server-side name match, not a slug filter,
so it over-returns for multi-slug companies (`NOVA` → 375 across several
tenants); it doubles the request count on every search; and compensating for
a soft server-side filter inside Go contradicts the repo's standing rule
against server-side soft-filter compensation. Recorded here so the option is
not silently re-litigated.

The JSON API's own condition fields (`keyword`, `companyName`, `area`, `job`,
`employ`, `income`, `span`, `page`) were recovered from the search SPA bundle
and verified server-side: `keyword=エンジニア` → 39,784 (the UI's live counter
showed 39,779). Every slug/company_id/work_id parameter name tried was a
**no-op returning the unfiltered total** — the API cannot be narrowed to one
tenant, so it is not usable as the adapter's per-company Search. It is
retained only for roster discovery (below).

Pagination quirk: `page=N` alone returns `{"result":"error"}`. It requires
`p_t=<totalCount observed on page 1>` and `f_t=0`; `p_t=0` yields an empty
page. This is a stateful stitch like Avature's `jobOffset`.

### Why an ATS adapter, not dedicated tools

Per-tenant boards with stable slugs and a central slug index is exactly the
`ats.Adapter` shape (herp, lever, ashby). Search is **dump-shaped** — one
request returns everything the tenant board will give — so it routes through
`searchDump`. The 100/category ceiling caps the dump but does not change its
shape: there is no pagination to stitch, so `Board` stays page-less.

### Roster discovery mechanism

Every `searchResult` record carries `company_url_root_dir` (slug) +
`official_corporate_name` + `company_id`. Paging the aggregator API harvests
verified slug↔name pairs 60 at a time, with no HTML parsing. 180 records
sampled → 106 distinct slugs, zero slug or name collisions against the 5,246
values already in the other rosters.

### Roster constraints discovered

1. **One company owns many slugs.** `copro-group_saiyo7/9/10/11` are all
   株式会社コプロコンストラクション; `agekke*` are all 株式会社エイジェック.
2. **Distinct companies share a display name.** 株式会社アクセル is both
   `accel-qcmd` (company_id 277480) and `axcell_saiyo1` (565578).

`ats.NewRegistry` fails at startup on duplicate display names, so the roster
must carry disambiguated names. Rule: `Name` is the corporate name; when a
name is already taken within engage, qualify it and keep `Slug` canonical.
Prefer one slug per company unless the boards genuinely differ.

## Envelope (shared constraints)

- Hand-written client (`client.go` / `parse.go` / `model.go`), **goquery** for
  HTML and `encoding/json` for the JSON-LD block. No ogen, no `openapi.yaml`,
  no `OPENAPI_SPECS` entry — this is not a REST surface.
- Fixtures are real captures committed under
  `internal/provider/engage/testdata/` as hurl req/rsp pairs. `mocksrv.go`
  replays them; `make hurl-fmt` before commit.
- Quirks documented in `internal/provider/engage/doc.go` (no `openapi.yaml` to
  hold them), with the aggregator-API reverse-engineering in `API.md`.
- Japanese strings appear only as site terms / example values, never as
  identifiers, comments, or help text.
- No `cmd/engage/doc.go`; the command comment sits atop `main.go`.

### Request headers (client and every fixture)

Follows `internal/provider/join/client.go:27,134`. A package-level
`userAgent` var, set on every request, and mirrored verbatim into each
`.hurl` req so live replay matches the client:

```
user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8   # HTML surfaces
accept: application/json                                                  # aggregator API
```

### Stop conditions (halt and report; do not improvise)

- Unknown slug or unknown work_id stops returning `404` (e.g. `200` + a soft
  error page, or a redirect) — the not-found signal must be re-derived.
- A tenant board proves paginable after all, or the 100/category ceiling
  moves — the dump characterization and `TotalCount` semantics change.
- A live tenant board returns `200` with no `dl.jobList` or zero parsed jobs —
  contradicts the 641-slug observation that engage serves no empty boards, and
  makes the selector-drift invariant unsafe.
- The host returns `403`/`429`, or blocks the configured user-agent.
- The JSON-LD `JobPosting` block is absent or not `@type: JobPosting` on a
  detail page that renders normally.

### Rollback

S1–S3 add only `internal/provider/engage/` and `cmd/engage/`; nothing else
references them, so rollback is deleting those directories. S4 is the first
slice that mutates shared files (`cmd/openings-mcp/main.go`,
`internal/ats/registry.go`, `cmd/verify-companies/main.go`); rollback there
is reverting those three edits plus deleting `internal/ats/engage.go`.

## Slices

### S1 — provider package: client + parsers + fixtures

Scope: `internal/provider/engage/{doc.go,API.md,model.go,client.go,parse.go,mocksrv.go,client_test.go,parse_test.go}`
plus `testdata/`.

- `Board(ctx, slug)` → `GET /<slug>/`, parses the board: each
  `dd.dataArea > a.linkKoma` gives work_id (from `/<slug>/work_<id>/`), title
  (`h2.catch`), salary (`.label--income`), area (`.label--area`), last-updated
  (`div.date`); the owning `dt.category` gives the employment type
  (中途採用 / アルバイト・パート採用) as a filter dimension.

  **Corrected during S1 (recon error).** This plan originally said one
  `dl.jobList` holds several `dt.category` children. It does not: each
  employment category renders as its **own sibling `<dl class="jobList">`**,
  so a two-category board has two `dl.jobList` elements. A parser taking
  `dl.jobList` `.First()` silently drops every category after the first. The
  committed `nova_career` fixture contains 2 such elements and its
  `board_req.hurl` asserts that count, so the mistake cannot recur silently.
- `Job(ctx, slug, workID)` → `GET /<slug>/work_<id>/`, parses the
  `application/ld+json` `JobPosting` (title, description, datePosted,
  baseSalary, employmentType, jobLocation, hiringOrganization) and falls back
  to the section headings for fields JSON-LD omits (応募資格・条件, 選考プロセス).
- 404 is the not-found signal for both unknown slug and unknown work_id —
  distinct typed errors.
- `Board` reports, per category, whether that category returned exactly 100.
  Consumed by `doc.go` and `cmd/engage` only — see the cap decision above.
- A `200` board that parses to zero jobs is an **error** (selector drift), not
  an empty result; engage serves no empty boards.
- `Companies(ctx, page, prevTotal)` → aggregator API, returns slug↔name pairs
  for roster tooling only. The `prevTotal` argument carries the `p_t`/`f_t`
  paging quirk explicitly rather than hiding it in client state.

Fixtures:

| fixture | slug / observed | covers |
|---|---|---|
| board happy path | `nova_career` — 200, 2 `dt.category` blocks (100 + 7) | multi-category parse |
| board at cap | `cookbiz_jobs` — 200, 1 category at exactly 100 | the 100-per-category ceiling |
| board minimal | `2918` — 200, `dl.jobList` present, exactly 1 job | the smallest board engage actually serves (see below) |
| board 404 | `definitely-not-a-real-slug-xyz` — **404**, ~76 KB error page, no `dl.jobList` | unknown slug |
| detail happy path | `nova_career/work_17046487` — 200, one `application/ld+json` `JobPosting` | JSON-LD detail parse |
| detail 404 | `nova_career/work_999999999` — **404** | unknown work_id |
| aggregator page 1 → page 2 | one chained `.hurl` | the `p_t`/`f_t` stitch: capture `totalCount` from page 1, feed it into page 2 as `p_t` |

**There is no empty-board fixture, because engage serves no empty boards.**
641 slugs drawn from `sitemap_company_0001.xml.gz` were fetched: all returned
`200`, all contained `dl.jobList`, and the minimum board size observed was
**1** job. The company sitemap enumerates only tenants with at least one
published posting. This yields a stronger invariant than an empty-board
fixture would: a `200` board that parses to zero jobs means **selector drift**,
not an empty tenant, and `Board` must return an error in that case rather than
an empty slice. Unknown tenants are already separated by a hard `404`.

The chained aggregator fixture is achievable as described — `[Captures]`
feeding a second request within one file is already in-tree at
`internal/provider/workable/testdata/jobs_req.hurl`, with the second response
committed separately and replayed by `mocksrv.go`.

The aggregator fixture must be a single chained file using hurl `[Captures]`
(already used in-tree: `internal/provider/apple/testdata/jobs_req.hurl`,
`internal/provider/workable/testdata/jobs_req.hurl`). A hardcoded `p_t` would
break under live replay because `totalCount` drifts continuously (~1.03M and
moving), and `make hurl-test` runs every `testdata/*.hurl` against the live
host.

Acceptance: `go test ./internal/provider/engage/...` green against mocksrv;
`make hurl-test` green — including the chained aggregator file actually
traversing page 1 → page 2 — and `make hurl-lint` clean.

### S2 — roster

Scope: `internal/provider/engage/{companies.yaml,companies.go,companies_test.go}`.

Seed roster, all six verified live (200 + jobs present + name match).
"board" is jobs visible on the tenant page; † marks a board at the
100/category cap, where the true count is higher:

| slug | name | board |
|---|---|---|
| `nova_career` | ＮＯＶＡホールディングス株式会社 | 107 † |
| `cookbiz_jobs` | クックビズ株式会社 | 100 † |
| `aspark-tokyo` | 株式会社アスパーク | 110 † |
| `accel-qcmd` | 株式会社アクセル | 84 |
| `japan_concentrix` | 日本コンセントリクス株式会社 | 38 |
| `hama-eng` | 株式会社ハマエンジニアリング | 12 |

**Seed changed during S4 (cross-adapter collision).** The seed originally
carried `benefitone_saiyoujyouhou` (株式会社ベネフィット・ワン, 4 jobs). That
company is already on the **herp** roster as `benefitone`, and both boards are
live — herp 35 jobs, engage 4 — so it is a genuine dual-platform employer, not
a stale entry. `ats.NewRegistry` rejects a display name shared across adapters,
so keeping it would have failed server startup. Replaced with `hama-eng`, which
serves the same purpose (a small, uncapped board) and collides with nothing.
The roster now collides with none of the ~34k names or slugs in the other
rosters, checked under the registry's own `normalize` (lowercase, letters and
digits only).

The seed deliberately mixes capped and uncapped boards so S4's lower-bound
`TotalCount` path is exercised by the smoke test.

### JSON-LD is not guaranteed (recon error, corrected during S4)

This plan asserted that a detail page contains exactly one
`application/ld+json` `JobPosting`. **It does not.** Roughly 1 detail page in
34 sampled across tenants renders the normal layout with **zero** JSON-LD
blocks — `aspark-tokyo/work_17068421` and `japan_concentrix/work_13152877` are
two. This tripped the stop condition "the JSON-LD block is absent on a
normally-rendering detail page", and was caught by the live roster
verification, not by the fixtures.

The HTML on those pages is identical in structure, so the parser now falls
back to it: the `h1` carries `<title> / <company>`, the `dl.dataSet` sections
carry the bodies, and the 会社名 table row backs up the employer. Fields that
exist only in the JSON-LD — `datePosted`, `employmentType`, structured salary
bounds, postal address — stay zero, while the salary and location text survive
as sections. `testdata/job_detail_no_jsonld_rsp.html` pins the case.

`go:embed`, validated at init, sorted by name.

Acceptance: init validation passes; no slug/name collision across all rosters.

### S3 — debug CLI

Scope: `cmd/engage/main.go` (ff/v4; `search`, `detail`, `companies`),
mirroring `cmd/smartrecruiters` — validated pagination flags, stray
positional args rejected.

Acceptance: all three subcommands return live data.

### S4 — MCP surface

Scope: `internal/ats/engage.go` + `engage_test.go`; registration in
`newATSRegistry` (`cmd/openings-mcp/main.go`); host pattern in
`careersHostPatternsByAdapter` (`internal/ats/registry.go`); `providerOrder`
in `cmd/verify-companies/main.go`.

- `Search` → `Board` → `searchDump` (dump-shaped). When any category came back
  at the 100 ceiling, `TotalCount` is a lower bound — surfaced as such, never
  presented as an exact total.
- `Filters` → employment-type dimension from `dt.category`.
- `Detail` → `Job`.
- `ParseCareersURL` → `en-gage.net/<slug>/` and `en-gage.net/<slug>/work_<id>/`.
  Must not swallow non-tenant paths (`/user/...`, `/sitemap*`, static roots).

Acceptance: live MCP stdio smoke test — every one of the six roster companies
returns listings through `search_jobs_by_company`, and
`get_job_detail_by_company` resolves a returned `job_id`.

### S5 — docs

README provider list; `cmd/openings-mcp` server instructions only if
tool-selection guidance changes.

## Deferred (not in this plan)

Bulk roster expansion via `unverified/engage.yaml` + `cmd/verify-companies`
(pipeline step 7). The harvest mechanism is proven; running it to scale is a
separate roster PR under the `roster:` commit convention.

## Risk noted

`en-gage.net/robots.txt` carries `User-agent: ClaudeBot / Disallow: /`. There
is no `User-agent: *` section. Raised with the repo owner on 2026-07-28; owner
elected to proceed. Recorded here so the decision is not silently re-litigated.

engage is also spinning out as a separate operating entity ("engage Inc.",
April 2026) and its Indeed syndication was scheduled to end 2026-06-11, so the
Indeed XML feed route was not pursued — the aggregator + tenant pages are
first-party and unaffected.
