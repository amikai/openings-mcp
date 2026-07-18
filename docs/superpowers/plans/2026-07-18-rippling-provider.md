# Rippling Provider — Plan

Executed 2026-07-18 in one session, following the integrate-new-provider
pipeline:

1. ~~Spec hunt~~ — official Job Board API docs exist
   (developer.rippling.com/documentation/job-board-api) but the reference
   pages are JS-rendered; the contract was derived from captured traffic
   against the documented endpoints and cross-checked against the
   ever-jobs `source-ats-rippling` plugin.
2. ~~Recon + fixtures~~ — probed 7 live boards. Two public surfaces exist:
   the official v1 API (api.rippling.com/platform/api/ats/v1) and the
   unofficial v2 the site itself uses (ats.rippling.com/api/v2); v1 covers
   everything the adapter needs, so v2 was left alone. Key quirk: the v1
   list emits one entry per (job, workLocation) pair — dedupe by uuid.
   Captured jobs/detail/not-found/unknown-board fixtures from Pythian's
   board (33 entries, 12 jobs — exercises the dedup); `hurl --test` green.
3. ~~OpenAPI + client~~ — full-dump shape (no pagination, no server-side
   search) with a separate detail endpoint, like Greenhouse. Minimal
   openapi.yaml over the two endpoints; ogen client. Nullability came from
   live evidence: `employmentType.label` null on a Cars & Bids posting
   (caught by verify-companies, not the fixtures), `jsonLd` null on every
   board but Rippling's own.
4. ~~Provider package~~ — mocksrv + client tests; seed roster of 5
   live-verified boards (Rippling, Boom Supersonic, Pythian, Cars & Bids,
   Plenful). Root Insurance was dropped: its old Workday tenant slug
   "joinroot" is still in the Workday roster and would collide in the
   registry.
5. ~~Debug CLI~~ — `cmd/rippling` companies/search/get (greenhouse shape,
   dump-with-client-side-filters); verified live including error paths.
6. ~~MCP surface~~ — `internal/ats/rippling.go` dump-style adapter
   (searchDump); registered in the server, careers-host patterns, and
   verify-companies. Live stdio smoke test: search on 3 roster companies,
   query+location filtering, get_filters, detail, and a careers-URL input
   for a non-roster board — all through the real server.
7. Roster curation — seed only; bulk discovery left for a later
   discover-companies session.
8. ~~Docs~~ — README ATS list; this plan.
