# Workable ATS Adapter — Design

## Surface choice

Multi-company ATS → `internal/ats.Adapter`, joining the unified
`search_jobs_by_company` tools. Careers pages live at
`apply.workable.com/<subdomain>/`.

## API surface (all unauthenticated, verified live 2026-07-18)

Workable has three public API generations behind `apply.workable.com`; we
use the v3 job-board API that the careers SPA itself calls, plus the v2
detail endpoint:

| Op | Endpoint |
|---|---|
| Search | `POST /api/v3/accounts/{account}/jobs` |
| Facets | `GET /api/v3/accounts/{account}/jobs/filters` |
| Detail | `GET /api/v2/accounts/{account}/jobs/{shortcode}` |

Rejected alternatives:

- `GET /api/v1/widget/accounts/{slug}` (the path ever-jobs' scraper takes):
  full dump with sparse fields, no server-side search — v3 gives real
  server-side keyword search plus structured filters.
- `{subdomain}.workable.com/spi/v3/jobs` (the official documented API):
  requires an account Bearer token; not usable for public listings.

## Search behavior

- Body fields, all optional: `query` (server-side full-text over title and
  body), `location` (array of `{country,region,city}` objects — any subset
  of fields), `department` (array of **numeric ids** from the facets
  endpoint), `remote` (`["true"]`/`["false"]`), `workplace`
  (`on_site|hybrid|remote`), `worktype` (`full|part|contract|temporary`),
  `token` (cursor).
- Unknown body fields are rejected (`{"bogus":"Not allowed"}`); there is no
  `limit` — page size is a fixed 10.
- Pagination is cursor-only: response `nextPage` token goes back in the body
  as `token`; the last page omits `nextPage` entirely.
- Errors: unknown account → 404 `text/plain` "Not Found"; unknown shortcode
  on detail → 404 `text/plain` "Job not found".
- Query residual filtering: Workable ORs query terms and also matches
  location text, so the adapter collects a bounded candidate set and re-
  applies unified AND semantics via `searchDump`. List rows carry no JD
  text, so residual filtering fans out to the detail endpoint (concurrency
  8) before matching; 404s between search and detail leave description
  blank rather than failing the whole search. Candidate walks are capped
  at 200 total hits.

## Detail behavior

The posting body is split across three HTML fields — `description`,
`requirements`, `benefits` — concatenated in that order for the unified
`JobDetail.Description`. All summary fields reappear on detail.

## Unified mapping decisions

- `SearchParams.Query` → body `query` (server-side), then residual AND on
  title + department + detail description.
- `SearchParams.Location` → resolved against the facets endpoint's
  `locations` (case-insensitive match on display/country/region/city); all
  matching facet entries are sent as `location` objects (OR). No facet
  match → empty result page, mirroring how structured filters behave.
- `Filters()` → from facets: `department` (names, resolved back to ids at
  search time), `workplace`, `worktype`.
- Unified page size is 20 but Workable serves fixed 10-item cursor pages, so
  one unified page = a cursor walk of 2×page upstream requests. Acceptable:
  requests are fast (<0.5s) and deep pages are rare in MCP use.
- Job URL: `https://apply.workable.com/<account>/j/<shortcode>/`.
- Location display: join the primary `location` with every distinct visible
  entry from `locations[]` (skip `hidden: true` and the primary duplicate),
  separated by `"; "`. Multi-site postings otherwise surface only the
  primary city (e.g. Tokyo while London is secondary).

## Quirk notes

- Cloudflare 403-blocks some client User-Agents (`Python-urllib` observed);
  Go's default UA and curl pass. Not worked around, just documented.
- `JobSummary.location` / `locations[]` fields (`region`, `city`, …) are
  nullable: live traffic from persado and zego returns `region: null` (and
  sometimes `city: ""`). The OpenAPI schema uses `type: [string, "null"]`
  so ogen decodes them as `OptNilString`.
