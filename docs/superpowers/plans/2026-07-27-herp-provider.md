# HERP (herp.careers) Provider — Plan

Design: [specs/2026-07-27-herp-provider-design.md](../specs/2026-07-27-herp-provider-design.md)

Executed 2026-07-27 in one session, following the integrate-new-provider
pipeline:

1. ~~Spec hunt~~ — no official API. HERP publishes none; both public
   surfaces were found by reading the site's own traffic.
2. ~~Recon + fixtures~~ — compared the two public JSON APIs on `herp.careers`
   and took the richer one (see the design doc's comparison table); probed
   pagination, the `limit` cap, the no-op filter params, and the 404 body;
   measured field coverage over 991 postings before writing the spec.
   Captured company / sparse-company / 404 / jobs fixtures; `hurl --test`
   green.
3. ~~Client~~ — `internal/provider/herp/openapi.yaml` + ogen. The generated
   schema is `CompanyBoard`, not `Company`: the roster type already owns that
   name in this package. Spec added to `OPENAPI_SPECS`; validated with
   `openapi-spec-validator` (the Makefile target needs Docker, which was not
   running).
4. ~~Provider package~~ — client/companies/mocksrv + tests; 27 seed companies,
   every one confirmed live.
5. ~~Debug CLI~~ — `cmd/herp` companies/search/get; verified live including
   both error paths and stray-positional rejection.
6. ~~MCP surface~~ — `internal/ats/herp.go` (`HerpAdapter`) + tests; wired
   into `newATSRegistry`, `careersHostPatternsByAdapter`, and
   `cmd/verify-companies` (both `providerOrder` and `buildAdapters`). Live
   stdio smoke test: keyword search, romanized-location search, careers-URL
   resolution, `get_filters_by_company`, `get_job_detail_by_company`, and both
   error paths — all through the real server.
   `verify-companies --provider herp`: 27 OK, 0 error.
7. Roster curation — seeded only. The `/careers/api/v1/jobs` enumeration
   reaches 1000 of the 2,198 listed companies (it stops at an offset of 1000
   whatever `limit` is), so bulk expansion has a known ceiling; it belongs in
   a separate `roster:` PR via `unverified/herp.yaml`.
8. ~~Docs~~ — README ATS list; this plan + design doc.

Not done, deliberately: structured salary has no home in `ats.JobSummary`, so
it cannot be searched or sorted on. HERP is the first provider in the tree
that could feed one. Adding the field would touch all 18 adapters and is its
own change.
