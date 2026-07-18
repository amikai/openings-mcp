# RemoteOK Provider — Plan

Scope for this session: pipeline stages 1–5 (fixtures → openapi → client →
provider package → debug CLI). The MCP surface (stage 6) is explicitly
deferred at the user's request — see Handoff below.

## Surface

RemoteOK (remoteok.com) is a single remote-jobs board, not a multi-tenant
ATS → dedicated `remoteok_search_jobs` / `remoteok_get_job_detail` tools
(stage 6, deferred). No roster.

## API shape (from live recon, 2026-07-19)

No official spec; `https://remoteok.com/api` is the whole public API and
its terms ride inside the response itself.

- `GET /api` — full dump of the ~100 most recent jobs. No pagination, no
  detail endpoint; each element already carries the full HTML description.
- `?tags=a,b` — server-side filter, comma-separated values AND-ed, scoped
  to the ~100 most recent matches of that tag set (a tag search can
  surface jobs older than the unfiltered window). Unknown tag → only the
  legal element.
- Response is a JSON array whose first element is a legal/attribution
  notice `{last_updated, legal}`; the rest are jobs.
- No auth, no User-Agent requirement (Go default UA gets 200).

Job fields observed on every job: slug, id (numeric string), epoch, date,
company, company_logo, position, tags, description, location, apply_url,
salary_min, salary_max, logo, url. `original` (bool) appears on rare
elements only. Absent values are empty strings / 0, not null.

Quirks to document in the spec:

- Every description ends with an anti-spam "Please mention the word …"
  blurb; some descriptions are only that blurb.
- `apply_url` equals `url` on every observed job.
- `salary_min`/`salary_max` are 0 when unknown.
- API ToS (in the legal element): consumers must credit Remote OK and
  link back to the job's `url` directly.

## Stages

1. ~~Spec hunt~~ — no official OpenAPI; third-party go-api-libs spec
   exists but is thinner than live traffic; contract derived from
   captured responses.
2. ~~Recon + fixtures~~ — dump / tags-filtered / unknown-tag (legal-only)
   hurl pairs; large dumps trimmed to the legal element + first jobs with
   jq (elements themselves stay byte-real).
3. ~~OpenAPI + client~~ — single `getJobs` operation with optional `tags`.
   Array items are `oneOf: [LegalNotice, Job]` — ogen infers the variant
   from unique required fields (`legal` vs `id`). Conservative
   requiredness on Job (`id`, `slug` only) so one field missing upstream
   can't fail the whole dump (see 2026-07-11 nullable sweep).
4. ~~Provider package~~ — mocksrv replaying the fixtures (tags-aware:
   unknown tag serves the legal-only fixture), client tests, doc.go.
5. ~~Debug CLI~~ — `cmd/remoteok` with `search` (--tags server-side,
   --keyword/--limit client-side) and `detail <id>` (re-fetches the dump,
   optional --tags to reach jobs outside the latest-100 window); verified
   live.

## Handoff (not done in this session)

- Stage 6, MCP surface: `internal/openingsmcp/remoteok.go` with
  `RegisterRemoteOK` + tests, wire the client in `newServer`
  (`cmd/openings-mcp/main.go`), then the live stdio smoke test with real
  queries against the new tools.
- Stage 8, docs: README provider list entry; server instructions mention
  if tool-selection guidance changes. Surface the attribution ToS in the
  tool description (mention Remote OK, link the job `url`).
