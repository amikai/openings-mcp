---
name: integrate-new-provider
description: Use when adding a new job-listings provider to openings-mcp — a new ATS platform (like Workday, Greenhouse, Lever, Ashby, SmartRecruiters) or a dedicated job board or careers site (like 104, Cake, Google, NVIDIA, TSMC) — or when wiring an existing provider package into the MCP server.
---

# Integrate a New Provider

## Overview

Every provider follows the same pipeline: hunt for an official API spec →
capture real API traffic as fixtures → minimal OpenAPI spec →
ogen-generated client → provider package
with fixture-replaying tests → debug CLI → user-facing surface (ATS adapter
or dedicated MCP tools). Land each stage as its own PR; don't skip ahead.
SmartRecruiters (`internal/provider/smartrecruiters`, `cmd/smartrecruiters`)
is the most recent worked example.

## Pick the Surface

- **Multi-company ATS** (one API, many tenants/boards): implement
  `internal/ats.Adapter` so companies join the unified
  `search_jobs_by_company` tools. Examples: workday, greenhouse, lever, ashby.
- **Single site or job board**: dedicated `<name>_search_jobs` /
  `<name>_get_job_detail` MCP tools in `internal/openingsmcp/<name>.go`.
  Examples: job104, cake, google, nvidia, tsmc, linkedin.

## Pipeline

1. **Spec hunt** — WebSearch for an official OpenAPI/Swagger spec or
   developer API docs before reverse-engineering anything (e.g.
   "<provider> API openapi spec", "<provider> developer docs"). An
   official spec beats a hand-derived one: fewer wrong guesses about
   types, nullability, and pagination. If one exists, trim it down to
   the endpoints you need rather than writing from scratch. Official or
   not, the spec still gets verified against captured traffic in the
   next step — vendor specs drift from what the public endpoints
   actually return.
2. **Recon + fixtures** — find the site's public JSON endpoints. When the
   endpoint isn't guessable or refuses direct requests, drive a real
   browser with your browser-automation tool: load the careers page,
   perform a search or open a posting, then read the network requests it
   fired to recover the underlying API calls — URL, query params, and any
   required headers (in Claude Code: the Browser pane's `navigate` /
   `computer` plus `read_network_requests`). Replay the recovered request
   outside the browser to confirm it works standalone. Capture each
   operation as a hurl request + JSON response pair in
   `internal/provider/<name>/testdata/` (happy path, filtered search,
   not-found, unknown company). `make hurl-test` replays them live;
   `make hurl-fmt` before committing.
3. **OpenAPI + generated client** — first classify the API's shape; it
   decides which endpoints the spec covers and how the adapter searches:
   - **Server-side search**: a list/search endpoint with query params and
     pagination, plus a detail-by-id endpoint (workday, smartrecruiters).
   - **Full dump**: one endpoint returns the whole board and search happens
     in our code — dump-style adapters share `searchDump`
     (`internal/ats/filter.go`) instead of mapping params upstream
     (greenhouse, lever, ashby). A separate detail endpoint may still
     exist, or the dump may already carry full descriptions.

   Then produce a minimal
   `internal/provider/<name>/openapi.yaml` covering only the endpoints you
   use: trimmed from the official spec when step 1 found one, otherwise
   written from the captured traffic. Mark fields nullable per what
   responses actually contain (see
   docs/superpowers/plans/2026-07-11-provider-schema-nullable-sweep.md for
   the failure mode). Add `gen.go` with the ogen `go:generate` line, run
   `go generate ./internal/provider/<name>`, add the spec to
   `OPENAPI_SPECS` in the Makefile, run `make validate-openapi`.
4. **Provider package** — `mocksrv.go` replays the testdata fixtures;
   `client_test.go` exercises the generated client against it. Roster-based
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
6. **Surface**
   - ATS adapter: `internal/ats/<name>.go` implementing `Adapter`
     (Name, Roster, ParseCareersURL, Search, Filters, Detail) + tests.
     Register it in `newATSRegistry` (`cmd/openings-mcp/main.go`), add its
     careers-URL host pattern to `careersHostPatternsByAdapter`
     (`internal/ats/registry.go`), and add it to `providerOrder`
     (`cmd/verify-companies/main.go`).
   - Dedicated tools: `internal/openingsmcp/<name>.go` with
     `Register<Name>` + tests; wire the client in `newServer`.
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
- Fixtures are captured real responses, never hand-written JSON.

## Common Mistakes

- Forgetting to add the new `openapi.yaml` to `OPENAPI_SPECS`, so
  `make validate-openapi` silently skips it.
- Roster slug or display-name collisions across adapters:
  `ats.NewRegistry` fails at startup by design — check the other
  `companies.yaml` files before adding entries.
- Building the whole integration in one PR; each pipeline stage is
  independently reviewable and should land separately.
