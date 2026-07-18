# マイナビ転職 Provider — Plan

Design: [specs/2026-07-18-mynavi-provider-design.md](../specs/2026-07-18-mynavi-provider-design.md)

Executed 2026-07-18 in one session, following the integrate-new-provider
pipeline:

1. ~~Spec hunt~~ — no official API (confirmed the 2026-07-18 survey).
2. ~~Recon + fixtures~~ — probed the /list/ DSL live (token ordering,
   min-salary steps, multi-keyword, zero-hit, 404s, the /tokyo/ legacy-page
   trap, `?soff=` no-op); captured jobs/empty/detail fixtures + five hurl
   files; `hurl --test` green.
3. ~~API contract~~ — no generated client (hand-written goquery scraper),
   so no openapi.yaml: the DSL and page contract are documented in the
   package doc comment (`internal/provider/mynavi/doc.go`) instead. (A
   doc-grade openapi.yaml was written first and folded into doc.go on
   review — nothing consumed it.)
4. ~~Provider package~~ — client/parse/mocksrv + tests.
5. ~~Debug CLI~~ — `cmd/mynavi` search/detail (jobindex shape); verified
   live including error paths.
6. ~~MCP surface~~ — `internal/openingsmcp/mynavi.go` + tests; wired into
   `cmd/openings-mcp` (clients, registry, serverInstructions, tool-list
   test); live stdio smoke test: filtered search (134 hits), page 2,
   detail, not-found error — all through the real server.
7. Roster curation — n/a (no roster; dedicated-tools provider).
8. ~~Docs~~ — README job-board list; this plan + design doc.
