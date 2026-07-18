---
name: integrate-new-provider
description: Use when adding a new job-listings provider to openings-mcp — a new ATS platform (like Workday, Greenhouse, Lever, Ashby, SmartRecruiters) or a dedicated job board or careers site (like 104, Cake, Google, NVIDIA, TSMC) — or when finishing a stalled integration by wiring an existing provider package into the MCP server, e.g. its client, tests, and debug CLI all work but its companies aren't reachable through the MCP tools.
---

# Integrate a New Provider

## Overview

Every provider follows the same pipeline: **recon** (find and rank the
site's real data surface) → capture fixtures → build a client that matches
that surface → provider package with fixture-replaying tests → debug CLI →
MCP surface (ATS adapter or dedicated tools, wired into the server).

For **JSON REST**, the client path is: minimal OpenAPI spec → ogen-generated
client. For **GraphQL, HTML, SSR-embedded state, RSS/Atom, or JSON Feed**,
write a hand-rolled client instead — do not force ogen. Work the stages in
order; each builds on the previous one's verified output. The integration is
done when the provider is reachable through the MCP server, not when the
debug CLI works — if a session stops before the surface stage, hand off the
remaining stages explicitly.

SmartRecruiters (`internal/provider/smartrecruiters`, `cmd/smartrecruiters`)
is the most recent ogen/JSON worked example. Spec-less hand-written clients
in-tree: LinkedIn, jobindex, join, iCIMS, SuccessFactors, UltiPro (HTML /
GraphQL / `__NEXT_DATA__`).

## Pick the MCP Surface

- **Multi-company ATS** (one API, many tenants/boards): implement
  `internal/ats.Adapter` so companies join the unified
  `search_jobs_by_company` tools. Examples: workday, greenhouse, lever, ashby.
- **Single site or job board**: dedicated `<name>_search_jobs` /
  `<name>_get_job_detail` MCP tools in `internal/openingsmcp/<name>.go`.
  Examples: job104, cake, google, nvidia, tsmc, linkedin.

RSS/Atom and most niche boards land on dedicated tools unless each tenant
has a stable, roster-able feed URL and multi-company routing is worth it.

## Pipeline

1. **Spec hunt** — WebSearch for an official OpenAPI/Swagger spec or
   developer API docs before reverse-engineering anything (e.g.
   "<provider> API openapi spec", "<provider> developer docs"). An
   official spec beats a hand-derived one: fewer wrong guesses about
   types, nullability, and pagination. If one exists, trim it down to
   the endpoints you need rather than writing from scratch. Official or
   not, the spec still gets verified against captured traffic in the
   next step — vendor specs drift from what the public endpoints
   actually return. No official docs does not mean no integration; it
   means recon (step 2) owns the surface choice.

2. **Recon + fixtures** — **Recon** means reconnaissance: before writing a
   client, discover how the site actually exposes listings, confirm the
   calls work outside a browser, and pick the best surface. Do not assume
   JSON REST.

   **Surface ranking** (prefer higher when it is public, stable, and
   replayable without login or a browser):

   1. Public **JSON REST** (OpenAPI + ogen path)
   2. Public **GraphQL** (hand-written client; see Indeed)
   3. **JSON Feed** / structured feed (e.g. Teamtailor `/jobs.json`)
   4. **RSS / Atom** (list dump; only when the quality bar below holds)
   5. SSR-embedded state (`__NEXT_DATA__`, `var Stash = {...}`, JSON
      smuggled in HTML — join, jobindex, UltiPro detail)
   6. Pure **HTML** / JSON-LD scrape (LinkedIn, iCIMS)
   7. **Browser automation** (Playwright) — openings-mcp avoids this
      unless a later decision explicitly allows it

   When the endpoint isn't guessable or refuses direct requests, drive a
   real browser with your browser-automation tool: load the careers page,
   perform a search or open a posting, then read the network requests it
   fired — URL, query params, and required headers (in Claude Code: the
   Browser pane's `navigate` / `computer` plus `read_network_requests`).
   Also check for `<link rel="alternate" type="application/rss+xml">`,
   Atom, or `application/feed+json` if no JSON API appears. Replay the
   recovered request outside the browser to confirm it works standalone.

   Capture each operation as a hurl request + response pair in
   `internal/provider/<name>/testdata/` (happy path, filtered search when
   applicable, not-found, unknown company). Fixtures are real captures:
   JSON, XML (RSS/Atom), or HTML — never hand-written bodies.
   `make hurl-test` replays them live; `make hurl-fmt` before committing.

   **RSS / Atom adoption bar** (all should hold; otherwise keep looking or
   treat the feed as a list-only MVP with an explicit detail gap):

   - Stable per-item **id** (`guid`, or a durable link used as id)
   - Enough fields for search (at least title + link); short descriptions
     need a **replayable detail** path (second request or linked HTML)
   - Fixed or constructible feed URL, no login
   - Prefer a higher-ranked surface when the same site also exposes one

   RSS is a legitimate surface for niche boards. It is usually a **full
   dump + client-side filter** shape, not server-side search. Do not choose
   RSS to save time when a stable public JSON API already exists.

3. **Client** — first classify the chosen surface's **search shape** (this
   decides adapter behavior regardless of transport):

   - **Server-side search**: list/search with query params and pagination,
     plus detail-by-id (workday, smartrecruiters).
   - **Full dump**: one response returns the whole board; search happens
     in our code via `searchDump` (`internal/ats/filter.go`) for ATS
     adapters (greenhouse, lever, ashby). Dump-style boards/feeds use the
     same idea locally. A separate detail endpoint may still exist, or the
     dump may already carry full descriptions.

   Then build the client for the surface:

   - **JSON REST** — minimal `internal/provider/<name>/openapi.yaml`
     covering only the endpoints you use (trimmed official spec or written
     from captures). Mark fields nullable per real responses (see
     docs/superpowers/plans/2026-07-11-provider-schema-nullable-sweep.md).
     Add `gen.go` with the ogen `go:generate` line, run
     `go generate ./internal/provider/<name>`, add the spec to
     `OPENAPI_SPECS` in the Makefile, run `make validate-openapi`.
   - **GraphQL, HTML, SSR blob, RSS/Atom, JSON Feed** — hand-written
     client (`client.go` / `parse.go`). Document the surface and quirks in
     `doc.go` and, when reverse-engineering is non-obvious, `API.md`.
     For RSS/Atom, default to
     [`github.com/mmcdole/gofeed`](https://github.com/mmcdole/gofeed) (one
     API for RSS and Atom, including common real-world feed variations).
     Reserve direct `encoding/xml` for unsupported vendor extensions or
     provider-specific fields gofeed does not surface. Use goquery for
     HTML / `__NEXT_DATA__`. Skip ogen and `OPENAPI_SPECS` unless you
     later gain a true REST OpenAPI surface.

4. **Provider package** — `mocksrv.go` replays the testdata fixtures;
   `client_test.go` exercises the client against it. Roster-based
   providers add `companies.yaml` + `companies.go` (embedded via
   `go:embed`, validated at init, sorted by name). Seed the initial
   roster with 3–5 companies: WebSearch for well-known companies hosted
   on this ATS (e.g. "site:<careers host> ..." or "<ATS> customers"),
   then confirm each against the live API before adding it — a real
   request must return 200 with jobs present and a matching company name
   (the smartrecruiters roster documents this bar). Bulk expansion comes
   later in step 7; the seed roster just has to prove the pipeline
   end-to-end.
5. **Debug CLI** — `cmd/<name>/main.go` using ff/v4 with `search`,
   `detail`, and `companies` subcommands for live manual checks. Validate
   pagination flags and reject stray positional args (mirror
   `cmd/smartrecruiters`).
6. **MCP surface**
   - ATS adapter: `internal/ats/<name>.go` implementing `Adapter`
     (Name, Roster, ParseCareersURL, Search, Filters, Detail) + tests.
     Register it in `newATSRegistry` (`cmd/openings-mcp/main.go`), add its
     careers-URL host pattern to `careersHostPatternsByAdapter`
     (`internal/ats/registry.go`), and add it to `providerOrder`
     (`cmd/verify-companies/main.go`).
   - Dedicated tools: `internal/openingsmcp/<name>.go` with
     `Register<Name>` + tests; wire the client in `newServer`.

   Finish the stage with a live smoke test through the real MCP path:
   sample 3–5 companies from the provider's `companies.yaml` and send
   actual MCP requests into the server — every sampled company must
   return live listings via `search_jobs_by_company`, and
   `get_job_detail_by_company` must work on at least one returned
   job_id. Over stdio that is initialize → notifications/initialized →
   tools/call, keeping stdin open (trailing `sleep`) so the server
   doesn't EOF before answering:

   ```bash
   (echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}'
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}'
    echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_jobs_by_company","arguments":{"company":"<roster company>"}}}'
    sleep 15) | go run ./cmd/openings-mcp
   ```

   Dedicated-tool providers have no roster: run the same requests
   against the new `<name>_search_jobs` / `<name>_get_job_detail`
   tools with a few real queries instead.
7. **Roster curation** — bulk-discovered candidates go in
   `unverified/<name>.yaml`; verify entries with `cmd/verify-companies`
   (runs the real adapter path) before promoting them into the curated
   `companies.yaml`. Follow the roster commit convention in CLAUDE.md.
8. **Docs** — update the README provider list and, if tool-selection
   guidance changes, the server instructions in `cmd/openings-mcp`.

## Conventions

- Brainstorm and plan each stage under `docs/superpowers/{plans,specs}`;
  the ashby documents there are the template (openapi → provider → cli).
- Never hand-edit `oas_*_gen.go`; change `openapi.yaml` and regenerate.
- Document provider-specific quirks (e.g. opaque params, odd value
  domains, soft filters) in `openapi.yaml` (then regenerate) when
  the provider has a spec, otherwise in the package's `doc.go`. A quirk
  scoped to a single operation in a spec-less provider may go in that
  method's godoc instead of `doc.go`.
- Fixtures are captured real responses, never hand-written JSON, XML, or
  HTML.

## Common Mistakes

- Stopping after the debug CLI (step 5): users only reach the provider
  through the MCP server, so a provider package that isn't registered in
  `cmd/openings-mcp` is invisible no matter how complete its client,
  tests, and CLI are.
- Forgetting to add a new **REST** `openapi.yaml` to `OPENAPI_SPECS`, so
  `make validate-openapi` silently skips it.
- Forcing ogen / OpenAPI onto GraphQL, HTML, RSS, or SSR-embedded blobs —
  those stay hand-written clients.
- Hand-rolling RSS/Atom with `encoding/xml` when `gofeed` already parses
  the feed; only drop to raw XML for fields gofeed cannot expose.
- Picking RSS (or HTML scrape) when a stable public JSON API exists on
  the same site.
- Adopting an RSS feed without stable item ids, or with title+link only
  and no plan for detail.
- Roster slug or display-name collisions across adapters:
  `ats.NewRegistry` fails at startup by design — check the other
  `companies.yaml` files before adding entries.
