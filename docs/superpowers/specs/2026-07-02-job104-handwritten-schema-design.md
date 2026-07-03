# job104 search tool: hand-written input schema

2026-07-02

## Goal

Replace the reflection-built input schema of the `104_search_jobs` MCP tool
(`jsonschema.For` + enum patching in `internal/jobmcp/job104.go`) with a
hand-written raw JSON schema unmarshaled into `jsonschema.Schema`. At the same
time, stop exposing the `s9` (shift) filter and make `area` required alongside
`keyword`.

## Non-goals

- No provider-layer changes: `internal/provider/job104` (openapi.yaml, generated
  client, `ids.go` label maps including `S9IDs`) stays as-is. `s9` remains a
  provider capability; only the tool stops exposing it.
- No renaming of tool input properties: the LLM-facing friendly names stay
  (`keyword`, `area`, `job_type`, `sort`, `remote`, `edu`, `page`). Only types,
  enums, and descriptions align semantically with openapi.yaml.

## Design

### Schema construction

- A backtick raw-JSON string constant in `job104.go`, unmarshaled once at
  package init into `*jsonschema.Schema`; unmarshal failure panics (same
  failure mode as today's `jsonschema.For` panic).
- Inline in the Go file (no separate `.json` file / `go:embed`).
- `labelEnum` and `job104SearchSchema()` are deleted from production code.

### Schema content

- `"type": "object"`, `"additionalProperties": false`,
  `"required": ["keyword", "area"]`.
- `keyword`: string. Free-text keyword search.
- `area`: string, enum of the human-readable labels from `job104.AreaIDs`
  (all 74), ordered by their 104 code — same order as openapi.yaml.
- `job_type`: string, enum `Full-time | Part-time | Senior | Dispatch`,
  soft-filter caveat in description.
- `sort`: string, enum `Relevance | Newest`.
- `remote`: string, enum `Full | Partial`, soft-filter caveat, omit for
  on-site.
- `edu`: `"type": "array"` (plain — the old `["null","array"]` was a
  reflection artifact), `uniqueItems: true`, items enum
  `HighSchoolBelow | HighSchool | College | University | Master | Doctorate`.
- `page`: integer, `minimum: 1`, 1-based.
- Descriptions carry semantics only, never id=label tables (unchanged rule).

### Input struct and conversion

- `job104SearchInput`: drop `Shift`; `Area` loses `omitempty` (required).
- `job104ToRequest`: drop the S9 conversion; add an `area` empty-string guard
  mirroring the existing `keyword` guard (protects direct callers that skip
  schema validation).
- `lookupCode`/`lookupCodes` and the `ids.go` maps remain the single source of
  label→code conversion. The hand-written enum labels must match those map
  keys; drift shows up as `invalid <field>` errors at call time.

### Tests

- Golden whole-value schema test (one `want` + single `assert.Equal`) with
  hand-typed literals, updated for: `required` = `["keyword","area"]`, no
  `shift` property, `edu` type `"array"`.
- Exception: `area`'s 74-label enum is not asserted in full (impractical to
  hand-type; per-enum coverage is not required). The whole-value compare runs
  with `area.enum` stripped from `got`, plus a hand-typed spot check that the
  enum contains a few known labels (e.g. `Taipei`).
- `TestJob104ToRequest`: drop `Shift`/`S9` from input and want.
- Minimal-input test becomes keyword+area; add a missing-`area` error case
  next to the missing-`keyword` one.
- Invalid-labels test: drop the `shift` case.

## Error handling

- Bad raw JSON → panic at init (programmer error, caught by any test run).
- Unknown enum label reaching the handler → `invalid <field> "<label>"` error
  result, unchanged.
- Missing `keyword`/`area` → schema validation rejects at the SDK layer;
  `job104ToRequest` guards double as defense for direct callers.
