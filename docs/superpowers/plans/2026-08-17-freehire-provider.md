# freehire.me Provider — Plan

Design: [specs/2026-08-17-freehire-provider-design.md](../specs/2026-08-17-freehire-provider-design.md)

Executed 2026-08-17, following the integrate-new-provider pipeline:

1. ~~Spec hunt and recon~~ — official OpenAPI at https://freehire.me/openapi.yaml;
   live `/jobs/search` and `/jobs/{slug}` verified 2026-08-17.
2. ~~Capture fixtures~~ — search, company filter, unknown company, detail, 404.
3. ~~Client~~ — official OpenAPI as published, ogen, fixture-replaying mock.
4. ~~Debug CLI~~ — `cmd/freehire` search + detail; live JSON search + detail checked.
5. ~~MCP surface~~ — dedicated tools, wired in `newServer`; live stdio smoke
   (`golang`+remote, `company=stripe`, one detail) returned listings.
6. ~~Docs~~ — README job-board list + server instructions.

Second pass 2026-08-18, for upstream `openapi.yaml` v1.1.0 (see the
design's update section):

1. ~~Regenerate~~ — v1.1.0 pulled in unmodified; ogen now emits shared enum
   types (`RegionsItem`, `SeniorityItem`, …) rather than per-operation ones.
2. ~~Recapture fixtures~~ — all 12 hurl files, retargeted to `/jobs/search`;
   added `jobs_ignored_params`, `geo_cities`, `company_detail`.
3. ~~MCP surface~~ — 6 tools; 41 job filters and 16 company filters exposed,
   `limit`/`offset` paging, `meta.ignored_params` promoted to a tool error.
4. ~~Debug CLI~~ — all 41 flags mirrored, plus `cities` and `company`
   subcommands; `--semantic-ratio` dropped.
5. ~~Live verification~~ — every filter exercised against the API: all 41
   parameters together produce no `ignored_params`, and each of the 27
   shared filters, 6 excludes, and 14 company filters narrows from the
   1,476,927-row baseline.
6. ~~Docs~~ — six spec-vs-live-API disagreements recorded in `doc.go`;
   provider README repointed at it.
