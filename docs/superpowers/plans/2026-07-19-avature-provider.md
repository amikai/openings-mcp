# Avature provider

Multi-company ATS → `internal/ats.Adapter` + roster, like iCIMS.

## Surface (recon 2026-07-19)

No public JSON/GraphQL/feed surface: the customer REST API is auth-gated,
and the portal RSS feed (`SearchJobs/feed/`) is capped at 20 items with
empty descriptions regardless of params. Chosen surface: server-rendered
portal HTML.

- List/search: `GET <base>/SearchJobs/?search=<kw>&jobOffset=<n>`.
  `search` is a platform-level full-text param (works even on portals
  without a visible keyword box). `jobOffset` is a true arbitrary offset;
  page size is portal-configured (Koch 6, Bloomberg 12) and
  `jobRecordsPerPage` cannot raise it.
- Total: `.list-controls__text__legend` "1-12 of 436 results" — optional
  per portal (Koch hides it). Legend absent → lower-bound total from
  offset + fetched + next-link presence.
- Detail: `GET <base>/JobDetail/<slug>/<id>` — slug segment is cosmetic,
  numeric id is the key. Bad id → 302 to `<base>/Error`.
- Locale prefix (`/en_US`) is optional; locale-less URLs 302 to it,
  query preserved.
- Bot protection varies per tenant: Delta 202-challenges, Maximus
  requires login. Roster entries must be curl-verified.

## Not supported (verified, not guessed)

- Facet filters: field names and option values are per-tenant numeric IDs
  and options load via JS autocomplete only — no server-side facets.
  `Filters` returns an empty set.
- Location: `search` is full-text over descriptions ("London" matches
  Dublin jobs), not a location filter. `SearchParams.Location` → teaching
  error.
- PostedAt: not rendered on list or detail in the common themes.

## Portal themes

Item markup varies (`article--result`, `article--jobs`, `list__item`,
sortable `<li>`); the stable anchor is same-origin `JobDetail/<slug>/<id>`
links deduped by id, first anchor per container wins (title precedes
Apply). Location is best-effort per theme: `.list-item-location`, a
field label containing "Location", or `.icon-address` sibling text.
Detail: title via `og:title` → `banner__text__title` → `<title>`;
description = `article--details` sections minus label/value field divs;
label/value pairs give Location.

## Stages

1. Fixtures: Bloomberg (legend theme + detail + not-found 302), Koch
   (no-legend theme + detail field variant), careers.avature.net/main
   (`article--jobs` theme).
2. Hand-written client (goquery), no ogen: `Search`, `JobDetail`.
3. Provider package: mocksrv replay, tests, `companies.yaml` seed
   (Bloomberg, Koch, OneCall, Unifi, Avature — all curl-verified live).
4. `cmd/avature` (ff/v4: search / detail / companies).
5. `ats.AvatureAdapter`: roster slug = `<host>/<portal>`; ParseCareersURL
   accepts `<tenant>.avature.net/[<locale>/]<portal>/...`; registry +
   careersHostPatterns + verify-companies wiring; MCP stdio smoke test.
6. README.
