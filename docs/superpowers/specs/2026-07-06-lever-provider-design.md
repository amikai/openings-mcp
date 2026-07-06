# `internal/provider/lever` — Lever Postings API provider

## Purpose

A provider package for the [Lever Postings API](https://github.com/lever/postings-api)
(mirrored locally at `case-study/postings-api/`), the public, unauthenticated
JSON API behind every `jobs.lever.co/<site>` career page. Like Workday, Lever
is a multi-tenant ATS: one OpenAPI contract serves every company, with the
company's `site` slug selecting the tenant. Unlike Workday there is no
per-company pod or site name to discover — the base URL is fixed and `site`
is a plain path parameter.

Scope is the provider layer only: hand-written `openapi.yaml`, ogen-generated
client, mock server, tests, and a small curated company list. Not wired into
`internal/openingsmcp` (no MCP tool layer), and no `cmd/` CLI — same stopping
point as the Workday, LinkedIn, and Synopsys providers.

## API surface (deliberately minimal)

The upstream API has three methods (list, detail, apply) plus HTML/iframe
embedding modes. This spec models only what a JSON API consumer needs:

- **`GET /v0/postings/{site}`** — list published postings.
  Query parameters: `skip`, `limit`, `location`, `commitment`, `team`,
  `department` (the four filters are repeatable and OR'ed within a field,
  e.g. `?location=Oakland&location=Boston`; values are case-sensitive),
  `level`, and `mode` pinned to the single enum value `json` so the API
  never falls back to HTML output.
- **`GET /v0/postings/{site}/{postingId}`** — one posting by id; same
  schema as a list element. JSON-only upstream, no extra parameters.

Excluded, with a note in the spec's top-level description:

- `POST /v0/postings/{site}/{postingId}` (apply) — a write operation that
  needs an API key and submits real applications; out of scope for a
  job-search tool.
- `group` — changes the list response's top-level shape to
  `[{title, postings}]`; grouping is trivial client-side.
- `mode=html|iframe`, `css`, `resize` — web-embedding features, not API
  consumption.

## OpenAPI spec shape

- **servers**: two fixed entries, global `https://api.lever.co` (default)
  and EU `https://api.eu.lever.co`. A company lives on exactly one
  instance; the ogen client's `WithServerURL` option switches between them,
  so no region variable and no extra code.
- **`Posting` schema**: all response fields from the official field table —
  `id`, `text`, `categories` (object: `location`, `commitment`, `team`,
  `department`, `allLocations`), `country`, `workplaceType`
  (`unspecified` / `on-site` / `remote` / `hybrid`), `opening`,
  `openingPlain`, `description`, `descriptionPlain`, `descriptionBody`,
  `descriptionBodyPlain`, `lists` (array of `{text, content}`),
  `additional`, `additionalPlain`, `hostedUrl`, `applyUrl`, `salaryRange`
  (object: `currency`, `interval`, `min`, `max`), `salaryDescription`,
  `salaryDescriptionPlain`. Only `id` and `text` are required; everything
  else is optional (`salaryRange` and others are documented as omittable).
- **Ground truth is the live API, not the README**: during implementation,
  captured `leverdemo` responses correct the spec — fields the API returns
  but the README omits (e.g. `createdAt`) get added; documented fields the
  API never returns get flagged rather than silently kept.
- **Codegen**: `gen.go` with the standard directive —
  `go tool github.com/ogen-go/ogen/cmd/ogen --target . -package lever --clean openapi.yaml`.

## Curated company list

`companies.yaml`, format like Workday's but with a single key besides the
name, since the site slug is all Lever needs:

```yaml
- company: "Lever (demo)"
  site: "leverdemo"
```

Collection rule: 10–20 companies, each verified at implementation time by
`GET https://api.lever.co/v0/postings/<slug>?limit=1&mode=json` returning
200 with a non-empty array. Unverified guesses don't ship.

`companies.go` embeds the YAML with `go:embed` and exports the parsed slice
directly as a package variable — no wrapper getter.

## Testing

Mirrors `job104`:

- `testdata/postings_req.sh` and `testdata/posting_detail_req.sh` — curl
  scripts that captured real `leverdemo` responses, kept so fixtures can be
  refreshed.
- `testdata/postings_rsp.json`, `testdata/posting_detail_rsp.json` — the
  captured responses, served verbatim by `mocksrv.go`.
- `client_test.go` against the mock server verifies:
  - request construction: path shape, `mode=json` always present, repeated
    filter parameters encode as `location=A&location=B` (not comma-joined);
  - response decoding of both endpoints against the real captured JSON;
  - error paths: 404 for an unknown site and an unknown posting id.

## Out of scope / later decisions

- MCP tool registration in `internal/openingsmcp`.
- A `cmd/lever` CLI.
- The apply endpoint, if programmatic applications are ever wanted.
- Growing `companies.yaml` beyond the initial verified batch.
