# RemoteFirstJobs Provider Implementation Plan

**Goal:** Add `internal/provider/remotefirstjobs` (OpenAPI spec, ogen client, fixture-replaying tests) and `cmd/remotefirstjobs` (debug CLI) for RemoteFirstJobs' public JSON API. The MCP surface is deliberately deferred to a later session (user request); this plan ends at the debug CLI + live smoke check.

**Surface decision (for the later wiring session):** RemoteFirstJobs is a single job board, not a multi-company ATS → dedicated `remotefirstjobs_search_jobs` / `remotefirstjobs_get_job_detail` tools in `internal/openingsmcp/remotefirstjobs.go`, following remotive/jobindex rather than an `ats.Adapter`.

**Reference:** the user pointed at `case-study/ever-jobs`'s `source-remotefirstjobs` plugin, which scrapes `https://remotefirstjobs.com/remote-jobs.rss` with a regex parser. That surface is stale and strictly worse than what the site exposes today: the RSS URL now 301s to `/rss/jobs.rss`, the feed carries only `title`/`link`/`guid`/`description`/`pubDate`, and the site publishes an official, documented **public JSON API** at `/jobs-api` that the ever-jobs plugin predates. Per the recon surface ranking, JSON REST wins — the RSS feed is not used.

## API shape (captured 2026-07-19)

Official docs: https://remotefirstjobs.com/jobs-api (prose only; no OpenAPI document)

- `GET https://remotefirstjobs.com/api/search-jobs` — the whole public surface: one operation with `query` (full-text), `category`, and `page` (0–4, 100 jobs/page) parameters, server-side search shape. No per-job detail endpoint exists (`/api/jobs/<id>` and variants 404), but every search hit already carries the **full HTML description** (1.7k–15k chars observed), so detail = resolve an id from search pages.
- Response envelope: `{_README_, page, jobs_count, jobs}`. Job fields: `id` (slug ending in a numeric suffix, e.g. `senior-product-manager-ai-platform-837752`), `url`, `company_name`, `company_logo` (nullable per docs), `title`, `category`, `seniority`, `description` (full HTML), `salary_min`/`salary_max` (nullable per docs), `locations` (list of country names, max 3, **may be empty** — 84/500 observed), `published_at`.

Observations grounding the spec (500 jobs sampled across 5 captures):

- **Empty result quirk:** zero hits returns HTTP 200 with `"jobs": null` (not `[]`) — `jobs` must be modeled nullable.
- **Error shape:** out-of-range `page` (>4 or <0) and unknown `category` return HTTP 400 `{"kind": "invalid_argument", "message": "..."}`.
- `category` accepts dash and underscore forms interchangeably (`software-development` ≡ `software_development`, byte-identical responses); responses always carry the underscore form. 15 category values observed in responses; there is no category-list endpoint, so the known values are documented in the spec but the parameter stays an open string.
- `seniority` observed values: `entry_level`, `intern`, `middle`, `senior`, `manager`, `director`, `principal`, `executive` — open string, not a closed enum.
- `published_at` has no timezone offset (`2026-07-17T21:33:30`) — modeled as plain string, not `format: date-time` (ogen's RFC3339 decoder would reject it).
- `query` + `page` compose (golang page 0 and page 1 share zero ids); pagination caps the reachable window at 500 jobs per query.
- Freshness: jobs appear in the API 24h after publication; upstream refreshes every 10 minutes. No User-Agent requirement, no auth, no observed rate limit.
- ToS (from `_README_` and /jobs-api): credit RemoteFirstJobs with a link; do not republish jobs to third-party job boards.

## Tasks

- [x] Capture fixtures → `internal/provider/remotefirstjobs/testdata/` (default search, `query=golang&page=1`, category req-only proof, no-results, invalid-category 400, page-out-of-range 400)
- [x] `openapi.yaml` (one operation, quirks documented inline) + `gen.go` + `go generate` + add to `OPENAPI_SPECS` + `make validate-openapi`
- [x] `mocksrv.go` + `client_test.go` (decode fixtures via generated client, incl. jobs:null and 400 paths) + `detail.go` (`Client.FindJob` page scan) + `doc.go`
- [x] `cmd/remotefirstjobs` — `search` (server-side query/category/page), `detail --id` (scan search pages for the id, optional `--query`/`--category` narrowing); live smoke check
- [ ] Hand off MCP wiring (`internal/openingsmcp/remotefirstjobs.go` registering `remotefirstjobs_search_jobs`/`remotefirstjobs_get_job_detail`, wire in `newServer`, README provider list) to a follow-up session
