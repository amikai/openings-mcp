# `cmd/lever` — Lever Postings CLI

## Purpose

A CLI that searches and inspects job postings on Lever-hosted career sites
(`jobs.lever.co/<site>`), built on the `internal/provider/lever` package
(added in [PR #80](https://github.com/amikai/openings-mcp/pull/80)). It's
the CLI companion to that provider, mirroring `cmd/workday`'s shape: one
binary, `ff/v4` subcommands, `--format text|json`.

Not wired into `internal/openingsmcp` (no MCP tool layer) — a separate,
later decision.

## Key difference from `cmd/workday`

Lever's list response already carries the full posting content
(`descriptionPlain` and friends), so `search` needs no per-result detail
fetches — no errgroup fan-out, no `html2text`, no fallback-URL logic. The
CLI is a thin render layer over one API call per invocation.

## Architecture

One binary, `cmd/lever/main.go` (single file, matching `cmd/workday`),
plus `cmd/lever/main_test.go`. No provider-package changes: everything the
CLI needs is already exported (`Companies`, `CompaniesBySite`,
`NewClient`, `ListPostings`, `GetPosting`).

```
lever --site SITE [--timeout DUR] [--format text|json] <companies|search|get> [flags]
```

- Root `Command` owns shared flags via `Flags.SetParent`: `--site`
  (curated site slug), `--timeout` (default 60s), `--format text|json`
  (default text). No `Exec` of its own; missing subcommand prints help and
  exits 1, exactly like `cmd/workday`.
- `companies` — lists `lever.Companies` (name and site slug), no network
  call, no `--site` needed.
- `search` — flags `--location`, `--commitment`, `--team`, `--department`
  (all `StringListLong`, repeatable, passed through as the OR'ed repeated
  query parameters), `--level`, `--limit` (default 20), `--skip`
  (default 0). Calls `ListPostings` with `Mode: ListPostingsModeJSON`.
  One page per invocation, no auto-pagination.
- `get` — takes one positional argument, the posting id (from a search
  result). Calls `GetPosting`. Errors if the positional arg is missing.

## Site validation

`--site` is validated against `lever.CompaniesBySite` (lowercased before
lookup), same policy as `cmd/workday --tenant`:

- empty → `--site is required`
- unknown → `site %q not found; run 'lever companies' to see supported sites`

All curated sites live on the global instance (`https://api.lever.co`), so
the CLI always uses the generated client's default server and has no
`--eu` flag. When an EU-instance company is ever added to
`companies.yaml`, that's the moment to add an `instance` field there and
switch servers per company — not before.

## Output

**Text mode** (`search`):

```
Lever Jobs Report (site: leverdemo)
Showing N postings

1. <title>
Created: 2019-03-21
URL: <hostedUrl>
Location: <single location>        # or a "Locations:" bullet list when >1
Team: <categories.team>            # omitted when unset
Commitment: <categories.commitment># omitted when unset
Description:
<descriptionPlain>
```

- The Lever list API returns no total count, so the report says "Showing
  N postings" for the fetched page only.
- `createdAt` (epoch milliseconds) renders as `2006-01-02` via
  `time.UnixMilli(...).UTC().Format`.
- Locations come from `categories.allLocations`, falling back to
  `categories.location` when `allLocations` is empty; singular/plural
  rendering mirrors `cmd/workday`'s `printResultLocations`.

`get` renders one posting with the same block (no numbering).

**JSON mode**: a stable hand-defined shape, not a dump of generated types —
mirrors `cmd/workday`'s `jobResultJSON` idea:

```go
type postingJSON struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	URL         string   `json:"url,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"` // 2006-01-02
	Location    string   `json:"location,omitempty"`
	Locations   []string `json:"locations,omitempty"`
	Team        string   `json:"team,omitempty"`
	Commitment  string   `json:"commitment,omitempty"`
	Description string   `json:"description,omitempty"` // descriptionPlain
}
```

`search --format json` emits `{"postings": [...]}` (wrapped for future
side-channel fields); `get --format json` emits one `postingJSON` object.
`companies --format json` encodes `lever.Companies` directly, like
`cmd/workday`'s `runCompanies`.

## Error handling

- Flag/argument validation errors and API errors both surface as returned
  errors from the `run*` functions; `main` prints `err: ...` and exits 1
  (the `cmd/workday` pattern verbatim).
- An API 404 (unknown posting id on `get`) arrives as the provider's
  `*ErrorResponseStatusCode`; its `Error()` string is what the user sees.
  No custom unwrapping.

## Testing

`cmd/lever/main_test.go`, same altitude as `cmd/workday/main_test.go`:
error-path tests on the `run*` functions — `runSearch`/`runGet` with a
missing site, an unknown site, and `runGet` with a missing posting id.
Rendering helpers that are pure functions (`toPostingJSON`, location
fallback) get direct unit tests against a literal `lever.Posting`. No
network, no mock server wiring in the CLI tests — request/response
behavior is already covered by the provider's own tests.

## Out of scope / later decisions

- MCP tool registration in `internal/openingsmcp`.
- Auto-pagination (`--all`).
- EU instance support (gated on an EU company entering `companies.yaml`).
- Free-form `--site` values outside the curated list.
