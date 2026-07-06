# Ashby CLI (`cmd/ashby`) — Design

## Context

`internal/provider/ashby` is complete (spec, generated client, mock server,
company roster — [PR #82](https://github.com/amikai/openings-mcp/pull/82)).
This round adds the standalone CLI, the same kind of dev tool every other
provider has under `cmd/`. The reference implementation is `cmd/workday`
(the other multi-tenant, roster-backed provider). MCP wiring remains out of
scope permanently (user decision).

Ashby's API shape drives the CLI shape: one unauthenticated endpoint
returns every listed job for a board in a single response — full
descriptions included, no server-side search or pagination, no detail
endpoint. A board can be large (OpenAI: 718 jobs), so printing full
descriptions for every result (what cmd/workday and cmd/cake do for their
10–20-per-page results) would flood the terminal. Hence a summary/detail
split: `search` prints compact summaries, `get` prints one job in full.

## Command surface

Single `cmd/ashby/main.go` (~250–300 lines), ff/v4 subcommand structure
copied from cmd/workday:

```
ashby --board BOARD [--timeout 60s] [--format text|json] <companies|search|get>
```

- Root flags: `--board` (board slug from companies.yaml), `--timeout`
  (default 60s), `--format` (`text`|`json`, default text).
- `companies` — lists the embedded roster as `Name (board)` lines
  (json: the `[]ashby.Company` slice). No network call. Mirrors
  cmd/workday's `runCompanies` exactly.
- `search [--keyword TEXT]` — fetches the whole board, filters
  client-side, prints summaries.
- `get --id UUID` — fetches the whole board, finds the job by id, prints
  it in full.

Both `search` and `get` always request `includeCompensation=true`:
compensation is core job-hunting information, the payload difference is
negligible, and a flag for it would be noise.

## search

1. Reject empty `--board`; look the slug up in `ashby.CompaniesByBoard`
   (lowercased) and reject slugs outside the roster, message pointing at
   the `companies` subcommand — workday's exact policy and message shape.
2. `ashby.NewClient("https://api.ashbyhq.com")`, `GetJobBoard` with
   `IncludeCompensation: NewOptBool(true)`.
3. Client-side filter: case-insensitive substring match of `--keyword`
   against the job title. Empty keyword = no filter. That is the only
   filter (user decision — everything else is `--format json` + jq).
4. Output, text: `Found N jobs; showing M` (N = full board, M = after
   filtering), then one numbered block per job: title; department/team;
   location plus secondary locations; workplace type; posted date (date
   part only); compensation tier summary when present and non-null; job
   URL; job id (the input to `get`). No description.
5. Output, json: `{"total": N, "jobs": [...]}` where each entry carries
   the same summary fields (id, title, department, team, location,
   secondaryLocations, workplaceType, isRemote, publishedAt, compensation
   summary, url).

## get

1. Same board validation and fetch as `search`.
2. Find the job whose `id` equals `--id` (exact match). Missing `--id` →
   `--id is required`; no match → `job %q not found on board %q`.
3. Output, text: the summary fields plus employment type, apply URL, the
   full description via `html2text.FromString(descriptionHtml)` (fall back
   to `descriptionPlain` if conversion fails), and a compensation section
   listing each tier (`title` may be null → label it "(unnamed tier)") and
   its components (`compensationType`, `summary`, interval, currency,
   min–max range; null-valued fields omitted).
4. Output, json: the job's decoded struct re-encoded as JSON (one job,
   full fidelity, description included).

## Error handling

- Missing/unknown `--board`, missing `--id`: CLI-side errors before any
  network call, as above.
- `GetJobBoard` returning the typed `*GetJobBoardNotFound`: report
  `board %q not found upstream` — theoretically unreachable for roster
  boards but not silently swallowed.
- Transport/decode errors: propagate as-is (workday behavior).
- Keyword matching nothing is not an error: `Found N jobs; showing 0`.

## Testing

`cmd/ashby/main_test.go`, mirroring cmd/workday's main_test.go: no-network
argument-validation tests asserting error messages —

- `search` and `get` with empty `--board` → `--board is required`
- `search` and `get` with a slug outside the roster → `board %q not
  found` + mention of `ashby companies`
- `get` with empty `--id` → `--id is required`

Fetch-path behavior (decode, filtering, rendering) is already covered by
the provider package's mock-server tests; the CLI run functions stay thin
enough that arg validation is the meaningful unit to test, matching the
repo's existing cmd-test convention.

## Not touched

- `.goreleaser.yaml` — it releases only `cmd/openings-mcp`; provider CLIs
  are unreleased dev tools.
- CI — already runs the whole repo's tests.
- `internal/provider/ashby` — consumed as-is.
