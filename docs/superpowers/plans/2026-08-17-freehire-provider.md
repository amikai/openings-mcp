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
