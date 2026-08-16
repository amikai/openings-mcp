# MediaTek Careers Provider — Design

## Scope and surface

MediaTek Careers is a single, self-hosted careers site rather than a
multi-tenant ATS. This change deliberately stops before the MCP surface: it
adds a reusable provider client and debug CLI, but does not register tools in
`cmd/openings-mcp`.

The public site exposes two useful surfaces:

- `GET /api/trpc/job.getJobs?input=...` — JSON search with server-side
  keyword/filter handling and one-based pagination.
- `GET /en/jobs/{jobID}` — server-rendered HTML detail page. The page embeds
  the job data in Next.js Flight state, but the visible HTML contains stable
  headings and metadata that can be parsed without a browser.

The Next.js locale middleware redirects cookie-less HTML requests back to the
same path, so the stateless client sends the public `NEXT_LOCALE=en` cookie.

No official API or OpenAPI specification was found. The client is therefore
hand-written, as required for an undocumented JSON/SSR surface.

## Search contract

The tRPC input is wrapped as `{ "json": ... }` and URL-encoded in the
`input` query parameter. The inner object contains `locales`, `page`,
`jobQueryInfo`, `filters`, `sortBy`, `order`, and `limit`. The public English
site uses locale `en_US`; filter values are opaque site codes.

The response is wrapped as `result.data.json` and contains `jobs`, `message`,
`status`, and `pagination`. Search summaries include stable `MTK...` IDs,
title, description, publication timestamp, category, experience, location,
program, and education metadata.

Search is server-side and supports keyword, category, work-experience,
location, and program filters. The client keeps the provider request in site
code form; the debug CLI resolves human-readable labels through the captured
filter maps in `ids.go`.

## Detail contract

Detail pages are fetched directly as HTML. The parser extracts the first main
heading, the Category/Location/Experience/Education metadata row, Job
Description, and Main Requirements and Qualifications. The requested job ID
is retained as the canonical ID because the page does not expose a stable
canonical link in the visible HTML. Unknown IDs currently return an upstream
HTTP 500 Next.js error page; the client reports that status as an error.

## Testing and deferred work

Fixtures are real captures for a filtered search, keyword search, empty
search, one detail page, and an unknown detail page. `NewMockServer` replays
those fixtures. MCP tools, server wiring, README provider lists, and live MCP
smoke tests are intentionally deferred to a later change.
