# HERP (herp.careers) Provider — Design

HERP Hire is a Japanese ATS (2,000+ tenants). Its public job pages live on
`herp.careers`, and HERP Career — HERP's own job media — republishes every
opted-in tenant's board on the same host. Both are reachable without login.

## Surface choice

Two public JSON APIs exist on `herp.careers`. We use **B**.

| | A — HERP Hire career page | B — HERP Career media (**chosen**) |
|---|---|---|
| Endpoint | `GET /api/v1/{slug}` | `GET /careers/api/v1/companies/{slug}` |
| Shape | `{requisitionList: [...]}` | `{company: {…, jobs: [...]}, statusCode}` |
| Job title | `name` | `name` |
| Posted date | ✗ | `jobPublishedAt`, `updatedAt` |
| Salary | ✗ | `salary{text,minimum,maximum,period}` |
| Structured location | ✗ (free text only) | `jobLocations[]{prefCode,prefName,cityCode,cityName}` |
| Employment type | `formOfEmployment` (JA text) | `jobEmploymentTypeIds[]` + `formOfEmployment` |
| Remote | ✗ | `jobRemoteworkType` |
| Job taxonomy | free-text group names | `jobRoles[]{id,name,parentJobRoleId,parentJobRoleName}` |
| JD sections | `summary` only | `summary`, `requiredSkills`, `preferredSkills`, `personality`, `workingConditions`, `welfare`, `trial` |
| Company profile | ✗ | funding, valuation, headcount history, investors, directors, tags |
| Coverage | every HERP Hire career page | HERP Career-listed tenants (2,198 companies) |

B is strictly richer on every field that the ATS layer can use. Its narrower
coverage does not matter: the roster is built *from* B's own company
enumeration, so every roster entry is guaranteed to resolve.

Both surfaces are plain JSON REST → OpenAPI + ogen, per the pipeline.

### Search shape

**Full dump.** `/companies/{slug}` returns the tenant's whole board with all
JD text in one response; there is no server-side search and no per-job detail
endpoint (`/careers/api/v1/companies/{slug}/jobs/{id}` 404s). So the adapter
follows the greenhouse/lever/ashby/recruitee family: fetch dump →
`searchDump` in `internal/ats/filter.go`; `Detail` re-fetches and scans by id.

## Endpoints modeled in `openapi.yaml`

Server: `https://herp.careers`.

1. `GET /careers/api/v1/companies/{slug}`
   → `{company: Company, statusCode}` where `Company` carries the 22
   `company*` fields plus `jobs[]`, `directors[]`, `fundingHistories[]`,
   `numberOfEmployeesHistories[]`. 404 → `{"error":{"message":"Function"}}`.

2. `GET /careers/api/v1/jobs?page=&limit=`
   → `{companies: [Company-without-profile-history], total, page, totalPages,
   statusCode}` — the cross-tenant enumeration used to build and refresh the
   roster. Not called by the adapter: each row carries at most **two** preview
   jobs, so it enumerates companies, not postings.

Every upstream field is modeled and marked nullable per observed responses.
Measured over the 991 preview postings on the first five `/jobs` pages:
`jobRemoteworkType` is null on 50%, `salary.minimum` on 25%,
`jobLocations[].cityName` on 29% (`prefName` is always present),
`title` is empty on 44%; `companyCumulativeFunding`, `companyValuation`, and
`companyMembersVoiceCount` are null on most companies. `jobPublishedAt`,
`updatedAt`, `jobRoles`, and `salary` (the object) are always present.

## Quirks to document in `openapi.yaml`

- `/careers/api/v1/jobs` paginates by **company**, not by job: `total` (2,198)
  and `totalPages` count companies, and each row nests at most two preview
  jobs regardless of how many that company actually has.
- `limit` caps at 100; `limit=200` → HTTP 400. Default page size is 20.
- Filter-looking query params on `/jobs` (`employment-type-id`,
  `parent-job-role-ids`, `remote-work-type-ids` — the ones the web UI puts in
  its URLs) are **no-ops on the API**: the response is byte-identical.
  Filtering is client-side, which is exactly what `searchDump` does.
- Unknown slug → HTTP 404 with the constant body `{"error":{"message":"Function"}}`
  — no usable message, so the adapter supplies its own.
- `name` is the public heading; `title` is a separate marketing headline that
  is often empty. Both are surfaced (title as the JD's first line).
- Image URLs come back with an explicit port: `https://herp.careers:443/...`.
- `source` is `herp-hire` on every observed row, so it is modeled but unused.
- A tenant's HERP Career job count can differ slightly from its HERP Hire
  career page (e.g. `crassone`: 10 vs 8) — the media is the authority here.

## `internal/ats.Adapter` mapping (`internal/ats/herp.go`)

- `Name()` → `"herp"`.
- `ParseCareersURL` → host `herp.careers`, slug from either
  `/careers/companies/<slug>[/jobs/<id>]` or the HERP Hire form `/v1/<slug>[/…]`.
  Slugs are **not** restricted to the roster: unlike join — which cannot
  resolve an arbitrary slug without a network call it has no context for
  (`internal/ats/join.go:47-61`) — one `/companies/{slug}` request settles it,
  so an arbitrary slug is accepted the way recruitee and greenhouse accept an
  arbitrary board. Because the two URL forms address different tenant sets,
  a `/v1/<slug>` URL for a HERP Hire tenant with no HERP Career listing
  parses but then 404s; the adapter turns the message-less upstream body into
  `herp: company %q is not listed on HERP Career (herp.careers/careers)`.
  Registry pattern: `herp.careers/careers/companies/<company>`
  (also `herp.careers/v1/<company>`).
- `dumpJob` mapping:

  | `dumpJob` | source |
  |---|---|
  | `summary.JobID` | `id` (the 12-char career-page id, e.g. `MoiFIPmUM9g4`) |
  | `summary.Title` | `name` |
  | `summary.Location` | composed — see below |
  | `summary.PostedAt` | `jobPublishedAt` → `YYYY-MM-DD` |
  | `summary.URL` | `/careers/companies/{slug}/jobs/{id}`, or `/v1/{slug}/{id}` when `companyIsApplicationEnabled` is false — see below |
  | `sortKey` | `jobPublishedAt` |
  | `orgUnit` | `jobRoles[]` `name` + `parentJobRoleName` (query tier 2) |
  | `description` | labelled JA sections — see below |
  | `locations` | `summary.Location` plus Latin aliases — see below |
  | `isRemote` | `jobRemoteworkType != null` — see below |
  | `fields` | see below |

- Structured filter dimensions (`Filters` = `distinctFilters(dump)`):

  | key | values | source |
  |---|---|---|
  | `jobRole` | エンジニア, カスタマーサクセス, … | `jobRoles[].name` |
  | `jobCategory` | セールス・事業開発, … | `jobRoles[].parentJobRoleName` |
  | `prefecture` | 東京都, 京都府, … | `jobLocations[].prefName` |
  | `city` | … | `jobLocations[].cityName` |
  | `employmentType` | `FULL_TIME`, `CONTRACT`, `FREELANCE`, `INTERNSHIP`, `PART_TIME` | `jobEmploymentTypeIds[]` |
  | `workplaceType` | `Remote`, `Hybrid` | `jobRemoteworkType` |

  `FilterSet` is documented as display **labels** (`internal/ats/ats.go:88`)
  and `distinctFilters` returns `fields` values verbatim, so the taxonomy
  keys carry the Japanese names, not `jobRoles[].id`/`parentJobRoleId` —
  otherwise `get_filters_by_company` would hand back opaque ids with no
  label anywhere in the response. `matchFilters` compares with
  `strings.EqualFold`, so CJK values are fine. `employmentType` keeps its
  upstream enum because upstream has no label form for it, matching
  recruitee, whose values are `Offer.EmploymentTypeCode` codes
  (`internal/ats/recruitee.go:172-174`).

  `workplaceType` is the key ashby, lever, and bamboohr already use for this
  dimension, so herp uses it too rather than minting a second name for the
  same thing — the whole point of `internal/ats` is that the caller cannot
  tell which ATS answered. Its values follow bamboohr's normalization
  (`internal/provider/bamboohr/workmode.go`), which maps that upstream's
  equally opaque `"0"/"1"/"2"` codes to `On-site`/`Remote`/`Hybrid`. herp
  never emits `On-site`: a null `jobRemoteworkType` means the company said
  nothing, not that the role is on-site. Since it is null on ~50% of
  postings, the key is absent entirely for a company that specified none —
  the per-company availability every full-dump adapter has, since
  `validateFilterKeys` derives valid keys from that company's dump.

### Location composition

`summary.Location` is built so it is reproducible from this spec alone:

1. Each `jobLocations` entry renders as `prefName + cityName` concatenated
   with no separator when `cityName` is non-null (`東京都渋谷区` — Japanese
   address order, so both `東京都` and `渋谷区` are substrings), else
   `prefName` alone.
2. Entries join with `"; "`, the separator recruitee already uses
   (`internal/ats/recruitee.go:244`), after dropping duplicates.
3. When `jobLocations` is empty, the first non-empty line of the free-text
   `location` is used instead.
4. When `jobRemoteworkType` is set, its label (`FULL_REMOTEWORK` →
   `フルリモート`, `HYBRID_REMOTEWORK` → `ハイブリッドリモート`) joins as one
   more `"; "` element — and stands alone when steps 1–3 produced nothing.
5. Otherwise `summary.Location` is empty. It is never synthesized.

`locations` — the string `matchLocation` actually searches — is
`summary.Location` plus two `"; "`-joined additions. First, Latin aliases:
the romanized prefecture for each distinct `prefCode` (`13` → `Tokyo`) from
a static 47-entry table in `herp.go`, and `Remote` / `Hybrid remote` for the
remote label. Second, the **whole** free-text `location`, whitespace
collapsed — the structured entries are the concise display but they routinely
drop what the company wrote out, so a posting can name 京都市上京区 in free
text while its Kyoto entry carries a null city, and indexing only the display
form would make `location: "上京区"` return nothing. Everything displayed
stays searchable and the additions only add reach; that is why `locations` is
a superset rather than a copy.

Without them, herp would be the first `internal/ats` adapter whose location
text is CJK-only, and `location: "tokyo"` would return zero rows with no
teaching error against a shared, adapter-agnostic tool description whose
examples are Latin-script (`internal/openingsmcp/company.go:26-28`). The
alias table fixes that inside the adapter instead of adding herp-specific
caveats to a schema all 17 adapters share. `query` text is left alone: it
searches the JD body, which is Japanese, and the same expectation is stated
at mynavi's tool boundary (`internal/openingsmcp/mynavi.go:15`). Both
behaviors are documented in `internal/provider/herp/doc.go`.

**Remote semantics.** `isRemote` covers both `FULL_REMOTEWORK` and
`HYBRID_REMOTEWORK`, so `location: "remote"` is the broad cut and
`workplaceType: Remote` is the precise one. Restricting
`isRemote` to full remote would drop the 396 hybrid postings (vs 97 full
remote) out of the sampled 991 from any "remote" search while leaving them
labelled ハイブリッドリモート on screen — a worse default than making the
narrower cut one filter away.
- `Detail` re-fetches the dump and scans by `id`, returning the same teaching
  error as the other full-dump adapters when the id is unknown.

### What the JD body carries

`Description` is the only free-form channel in `ats.JobDetail`, and HERP
Career exposes far more than the unified struct has fields for. Rather than
drop it, the adapter renders it as labelled Japanese sections, in the order
the site itself uses:

1. 求人タイトル (`title`, when non-empty — 56% of rows)
2. 仕事概要 (`summary`)
3. 必須スキル (`requiredSkills`) / 歓迎スキル (`preferredSkills`) /
   求める人物像 (`personality`)
4. 給与 — `salary.text` plus a normalized line derived from
   `minimum`/`maximum`/`period` (`ANNUAL`/`MONTHLY`/`HOURLY`), so the model
   never has to parse the free-text range. `minimum` is present on 75% of
   rows even though `salary` itself is always present.
5. 勤務地 (`location`) / 雇用形態 (`formOfEmployment`) /
   勤務体系 (`workingConditions`) / 試用期間 (`trial`) / 福利厚生 (`welfare`)
6. 最終更新 (`updatedAt`) and 応募受付 (`isApplicable` **alone**) — freshness
   and whether the opening is still live, neither of which fits `JobSummary`.
   `companyIsApplicationEnabled` is deliberately not part of this: it picks
   the application surface, not the posting's state (see below).
7. 企業情報 — the startup due-diligence block no other provider in this repo
   has: 累計調達額 / 評価額 (`companyCumulativeFunding`, `companyValuation`,
   both in 億円), 投資家 (`companyInvestors`), 従業員数 with its recent
   trajectory (`companyNumberOfEmployees` + `numberOfEmployeesHistories`),
   設立 (`companyFoundedIn`, `companyYearsSinceFounded`), 本社
   (`companyHeadquarterLocation`), 資本金 (`companyShareCapital`), 事業内容
   (`companySynopsis`, `companyWhat`/`Why`/`Where`/`How`), タグ
   (`companyTags`), 企業サイト (`companyUrl`), 経営陣 (`directors`). The
   founder badge requires `isFounder` **and** `isInside`: `isFounder` sits on
   a single history entry that usually names a previous company, so the flag
   alone would credit a director with founding whichever startup they came
   from.

The company block is emitted by `Detail` only; `Search` results stay at
`JobSummary` size.

### Which URL the applicant gets

`companyIsApplicationEnabled` governs the HERP Career **media** pages only.
A company that opts out still has a working HERP Hire career page: its media
job page renders with no apply action while `/v1/{slug}/{id}` shows 応募する.
Since `JobSummary.URL` is the applicant handoff, the adapter links to
`/careers/companies/{slug}/jobs/{id}` normally and falls back to
`/v1/{slug}/{id}` when the flag is false — otherwise valid search results
lead to a dead end. `cmd/herp` applies the same rule.

Deliberately dropped: `coverImageUrl` and `companyLogoUrl` (images),
`requisitionId` and `companyId` (internal ids with no public page),
`source` (constant `herp-hire`), `companyPrivacyPolicyUrl`,
`companyMembersVoiceCount`.

Structured salary still has no home in `ats.JobSummary`, so it cannot be
searched or sorted on — a unified-schema change that would touch all 17
adapters, out of scope here.

## Files

New:

- `internal/provider/herp/{openapi.yaml,gen.go,doc.go,companies.go,companies.yaml,mocksrv.go,client_test.go}`
- `internal/provider/herp/testdata/` — `company_req.hurl` + `company_rsp.json`,
  `company_not_found_req.hurl` + `_rsp.json` (404), `company_sparse_rsp.json`,
  `jobs_req.hurl` + `jobs_rsp.json` (`page=1&limit=2`).

  Fixtures are real captures, never edited, so each tenant is **selected** by
  scanning `/careers/api/v1/jobs` for the shape its tests need. Selection
  criteria, which are exactly the union of what the assertions below depend on:

  `company_rsp.json` — one tenant whose postings include at least: one
  `FULL_REMOTEWORK`, one `HYBRID_REMOTEWORK`, one null `jobRemoteworkType`;
  one posting in `prefCode` `13` (東京都), so the Latin-alias assertion has a
  known target; postings spanning two distinct `prefName`, so the
  `prefecture` filter demonstrably narrows; one `jobLocations` entry with a
  non-null `cityName`; one posting with a non-empty `jobEmploymentTypeIds`;
  two distinct `jobRoles[].parentJobRoleName`; and two postings that separate
  the query tiers — one whose `jobRoles[].name` contains a term absent from
  its title, and one where the same term appears only in the JD body.

  `company_media_optout_rsp.json` — a tenant with
  `companyIsApplicationEnabled: false` and open postings, so the URL-surface
  rule and the 応募受付 status can be pinned against a real capture rather
  than a hand-built one.

  `company_sparse_rsp.json` — a tenant with at least one posting that has
  empty `jobLocations`, empty free-text `location`, and null
  `jobRemoteworkType` simultaneously, plus a null `salary.minimum`. This is
  the null-coverage fixture (recruitee's `offers_nulls_rsp.json` shape) and
  carries no `.hurl` of its own, same as recruitee's.

  Selected: `notainc` (株式会社Helpfeel, 51 postings) meets every
  `company_rsp.json` clause — 45 `FULL_REMOTEWORK` / 5 `HYBRID_REMOTEWORK` /
  1 null, 東京都 and 京都府, 49 postings in prefCode `13`, 49 non-null
  `cityName`, `FULL_TIME`/`FREELANCE`/`INTERNSHIP`, 12 role parents, and the
  query-tier pair on コンサルティング. `resmahr` carries the sparse posting;
  `worx`, `ohmyteeth`, `gloe`, and `tohohouse` were the alternates. If a
  tenant drifts, select another by the same criteria rather than trimming a
  capture.

  `notainc`'s capture is 577 KB, comparable to recruitee's 593 KB
  `offers_rsp.json`.
- `internal/ats/herp.go`, `internal/ats/herp_test.go`. The tests pin the
  mapping decisions, not just the wiring — following
  `internal/ats/recruitee_test.go:79-171`. Each case names its fixture:
  - *(`company_rsp.json`)* `Filters` returns all six keys, with
    `jobRole`/`jobCategory`/`prefecture`/`city` carrying Japanese labels,
    `workplaceType` carrying the shared `Remote`/`Hybrid` labels, and
    `employmentType` carrying upstream enum values
  - *(`company_rsp.json`)* `Search` narrows correctly under a `jobRole`
    filter and under a `prefecture` filter, and an unknown key is rejected by
    `validateFilterKeys` with the teaching error
  - *(`company_rsp.json`)* `location: "remote"` returns both the
    `FULL_REMOTEWORK` and the `HYBRID_REMOTEWORK` posting and excludes the
    null-`jobRemoteworkType` one, while `workplaceType: Remote` narrows to
    full remote only
  - *(`company_rsp.json`)* `jobRoles` names reach `orgUnit`: the role-name
    match outranks the body-only match for the same query
  - *(`company_rsp.json`)* the Latin alias path — `location: "tokyo"` matches
    a posting whose displayed `Location` is `東京都…`, and that posting's
    `city` filter value is also accepted as a `location` value
  - *(`company_sparse_rsp.json`)* `summary.Location` is empty — not
    `Remote` — for the posting with no location data and no remote type
  - *(no fixture)* both `ParseCareersURL` forms, including the accept
    decision for a slug not on the roster, and
    `assert.Contains(t, careersHostPatternsByAdapter, "herp")` (the guard from
    `internal/ats/smartrecruiters_test.go:506-511`)
  - *(`company_not_found_rsp.json`)* the not-listed-on-HERP-Career error;
    *(`company_rsp.json`)* the unknown-`job_id` teaching error
- `cmd/herp/main.go` — ff/v4 `companies` / `search` / `get`, no `doc.go`

Edited:

- `cmd/openings-mcp/main.go` — `ats.NewHerpAdapter(hc)` in `newATSRegistry`
- `internal/ats/registry.go` — `careersHostPatternsByAdapter["herp"]`
- `cmd/verify-companies/main.go` — `"herp"` in `providerOrder` **and** a
  `case "herp": a = ats.NewHerpAdapter(hc)` arm in `buildAdapters`. Both are
  required: the switch has no `default`, so a name in `providerOrder` with no
  arm appends a nil `ats.Adapter` that `buildChecks` then dereferences
  (`cmd/verify-companies/main.go:179-229`).
- `Makefile` — `internal/provider/herp/openapi.yaml` in `OPENAPI_SPECS`
- `README.md` — ATS list

## Roster

Seed 20–30 companies drawn from `/careers/api/v1/jobs` and confirmed live
(HTTP 200 with a non-empty `jobs` array and a matching `companyName`), checked
against the other `companies.yaml` files for slug/display-name collisions.
Bulk expansion of the remaining ~2,170 companies is a separate `roster:` PR
per CLAUDE.md, staged through `unverified/herp.yaml` + `cmd/verify-companies`.

## Verification

`go generate` → `make validate-openapi` → `go test ./...` → `make hurl-test`
→ live `cmd/herp` checks → MCP stdio smoke test (`search_jobs_by_company` on
3–5 roster companies, `get_job_detail_by_company` on one returned `job_id`).
