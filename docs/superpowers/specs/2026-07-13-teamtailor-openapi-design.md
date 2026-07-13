# Teamtailor Career Site Feed OpenAPI — Design

## Context

Teamtailor is a multi-company ATS whose hosted career sites normally use
`<company>.teamtailor.com`, `<company>.na.teamtailor.com`, or
`<company>.au.teamtailor.com`; customers may also attach custom domains.
Teamtailor's authenticated account API is documented at
<https://docs.teamtailor.com/>, but it requires a secret issued separately
by every customer and therefore cannot power anonymous cross-company search.

Since May 2026, each career site advertises unauthenticated, agent-oriented
resources through `/.well-known/api-catalog`. The catalog names
`/jobs.json` as `application/feed+json` and `/jobs.md` as `text/markdown`.
The JSON endpoint is the useful machine surface: it follows JSON Feed 1.1,
returns every current posting in one response, embeds full HTML descriptions,
and includes a Schema.org `JobPosting` object under `_jobposting`.

There is no published OpenAPI document for this career-site feed. The provider
therefore gets a minimal hand-written spec grounded in the advertised JSON
Feed standard and captured live responses.

## Verified wire behavior (2026-07-13)

`GET https://<career-host>/jobs.json`:

- requires no authentication, cookies, query parameters, or special headers;
- returns `200 application/feed+json; charset=utf-8` for a live site;
- returns the complete board with no server-side search or pagination;
- returns `404 application/json` with an empty body for an unknown hosted
  tenant;
- advertises its API catalog in the response `Link` header;
- duplicates the description in `content_html` and
  `_jobposting.description`; the client only needs `content_html`;
- carries a JSON Feed item UUID in `id`, the public posting URL in `url`, and
  an ISO 8601 timestamp in `date_published`;
- carries locations in `_jobposting.jobLocation[].address`, where
  `streetAddress`, `addressLocality`, `postalCode`, `addressCountry`, and
  `addressRegion` may be null.

Live verification covered Teamtailor, bunny.net, Knauf Belgium, Tiptapp, THE/STUDIO,
and Village Automotive Group across the EU and North America host shapes.

## Spec shape

`internal/provider/teamtailor/openapi.yaml` uses OpenAPI 3.1 and models one
operation:

```text
GET /jobs.json -> 200 CareerFeed | 404 (empty)
```

The server URL is an example Teamtailor career host. Callers always construct
the generated client with the selected company's origin, so no company path
or query parameter appears in the operation itself.

Only fields consumed by the provider are modeled:

- `CareerFeed`: JSON Feed `version`, `title`, `home_page_url`, `feed_url`, and
  `items`.
- `CareerItem`: `id`, `title`, `url`, `date_published`, `content_html`, and
  `_jobposting`.
- `JobPosting`: `jobLocation`.
- `Place` / `PostalAddress`: the structured location fields used for display
  and filters.

Unknown JSON Feed and Schema.org properties remain forward-compatible and are
ignored by the generated decoder. The 404 response has no schema because the
observed body is zero bytes despite the `application/json` content type.

## Validation

- Capture a real non-empty feed and unknown-tenant response as hurl fixtures.
- Generate with ogen and compile the package.
- Decode the captured response in a fixture-replaying client test.
- Add the spec to `OPENAPI_SPECS` and run `make validate-openapi`.

## Out of scope

The authenticated Teamtailor account API, application submission, legacy RSS,
career-page HTML scraping, and server-side filters are not part of this client.
Search, detail lookup, and pagination are adapter concerns over the full dump.
