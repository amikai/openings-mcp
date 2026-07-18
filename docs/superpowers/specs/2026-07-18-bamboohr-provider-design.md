# BambooHR Provider Design

2026-07-18

## Goal

Add BambooHR as a multi-company ATS behind the unified company tools:
`search_jobs_by_company`, `get_filters_by_company`, and
`get_job_detail_by_company`. Public careers boards at
`https://{subdomain}.bamboohr.com/careers` become searchable by roster name
or by careers-page URL.

## Surface

Multi-company ATS → `internal/ats.Adapter` (not dedicated MCP tools).

## API shape

Full dump + separate detail, no server-side search or pagination:

| Endpoint | Role |
|---|---|
| `GET /careers/list` | Every open posting as a sparse summary |
| `GET /careers/{id}/detail` | One full posting (description HTML, compensation, datePosted, share URL) |

The authenticated BambooHR product API is out of scope; only the public
careers-site surface is used.

## Adapter shape

`BambooHRAdapter` constructed as `NewBambooHRAdapter(hc *http.Client)`.

- `Name()` → `"bamboohr"`.
- Per-tenant origin: `https://{slug}.bamboohr.com`.
- `NewBambooHRAdapter` copies `hc` and sets `CheckRedirect` to
  `http.ErrUseLastResponse` so unknown-tenant 302s stay diagnosable instead
  of landing on marketing HTML with HTTP 200.
- `baseURL` is overridable for fixture-replaying tests.

## Slug model

The slug is the lowercased careers-site subdomain.

- `Roster()` maps `bamboohr.Companies` to `CompanyInfo{Slug, Name}`.
- `ParseCareersURL` matches `(?i)^(?P<slug>[^.]+)\.bamboohr\.com$` and
  rejects reserved product hosts (`www`, `api`, `app`, `documentation`, …).
- Non-roster URLs are accepted; Search/Detail hit the live tenant. An
  unknown tenant returns a "not found upstream" error (302 observed).
- `JobDetail.Company` prefers the roster name, then the slug.

## Search / Filters

`Search` and `Filters` call `ListJobs`, map each row into a `dumpJob`, then
delegate to the shared `searchDump` / `distinctFilters` engine.

When `Search` receives a non-empty `Query`, the adapter fans out concurrent
`GetJobDetail` calls (cap 8) to fill each dumpJob's description so tier-3
skill/technology matching works. Filters and empty-query Search skip that
fan-out and stay list-only.

Filter keys emitted from the dump:

| Key | Source |
|---|---|
| `department` | `departmentLabel` |
| `employmentType` | `employmentStatusLabel` |
| `workplaceType` | `locationType` via `WorkModeLabel` (`0` On-site, `1` Remote, `2` Hybrid) |

List-feed limitations the adapter exposes:

- No posting date on summaries (`PostedAt` empty; detail carries `datePosted`).
- Display location prefers structured `location` (city/state) and falls back
  to `atsLocation` (city/state/country) when `location` is all-null.
- Search location strings also include the work-mode label so "remote" /
  "hybrid" queries hit rows whose only locality signal is `locationType`.
- `isRemote` is true when `locationType == "1"`.

## Detail

`Detail` calls `GetJobDetail` and maps:

- title ← `jobOpeningName`
- location ← city/state/country from detail `location`, falling back to
  `atsLocation` when `location` is all-null (same rule as the list path)
- postedAt ← `datePosted`
- URL ← `jobOpeningShareUrl` or constructed `https://{slug}.bamboohr.com/careers/{id}`
- description ← HTML-stripped `description` via html2text

404 → teaching error asking for a job_id from search. 302 → not-found
upstream.

## Wiring

- `cmd/openings-mcp/main.go` — `newATSRegistry` includes `NewBambooHRAdapter(hc)`.
- `internal/ats/registry.go` — `careersHostPatternsByAdapter["bamboohr"] =
  "<company>.bamboohr.com/careers"`.
- `cmd/verify-companies/main.go` — `providerOrder` + `buildAdapters` case.
- `Makefile` — `OPENAPI_SPECS` includes `internal/provider/bamboohr/openapi.yaml`.

## Seed roster

Five live-verified boards (name / slug):

- Aroa Biosurgery / `aroabio`
- Ashtead Technology / `ashteadtechnology`
- Concept2 / `concept2`
- Curtin Maritime / `curtinmaritime`
- Giatec Scientific / `giatecscientific`

Bulk expansion is out of scope for this PR (discover-companies later).

## Non-goals

- Authenticated BambooHR API (employees, time-off, etc.).
- Server-side search mapping (the public list endpoint has none).
- Application-form submission or any write path.
- Large roster curation.
