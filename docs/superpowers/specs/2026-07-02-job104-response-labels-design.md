# job104 response label conversion — design

Date: 2026-07-02
Status: approved (untracked per project convention)

## Problem

The MCP layer converts requests (labels → 104 codes via `job104MCPToHTTPRequest`)
but returns provider responses raw. LLMs see numeric codes (`jobRo: 1`,
`remoteWorkType: 2`) and the input schema descriptions tell them to "verify
each result's jobRo" — vocabulary that only exists on the wire, not in the
tool's input language.

## Decisions (confirmed with user)

- Scope: both `104_search_jobs` and `104_get_job_detail`.
- Unknown codes: skip the field (omitempty), never error, never leak raw codes.
- Shape: mirror provider responses 1:1 in new MCP-facing output structs; coded
  fields are replaced by label fields named after the input params
  (`job_type`, `remote`).

## Design

All new code lives in `internal/jobmcp/job104.go`, symmetric with the existing
request converter.

### Output types

- `job104SearchOutput` mirrors `job104.JobsResponse`: `data` (list) +
  `metadata.pagination`. Per-job fields keep their wire JSON names
  (`jobNo`, `custName`, `link`, `salaryHigh`, …) except:
  - `jobRo` (int) → `job_type` (string label from `RoIDs`: Full-time /
    Part-time / Senior / Dispatch), `omitempty`.
  - `remoteWorkType` (int) → `remote` (string label from `RemoteWorkIDs`:
    Full / Partial), `omitempty`; 0 (on-site) omits the field.
- `job104DetailOutput` mirrors `job104.JobDetailResponse` (header, contact,
  condition, welfare, jobDetail, industry, employees, custNo). Provider `Opt*`
  fields become plain fields with `omitempty`. Coded fields:
  - `jobDetail.jobType` (OptInt, same value set as `ro`) → `job_type`, `omitempty`.
  - `jobDetail.remoteWork` (nullable `{type, description}`) → `remote` label,
    `omitempty`; null/absent omits the field.

### Converters

- `job104HTTPToMCPResponse(*job104.JobsResponse) *job104SearchOutput`
- `job104HTTPToMCPDetail(*job104.JobDetailResponse) *job104DetailOutput`

Pure mapping, no error return. Reverse lookups are package-level maps in
jobmcp inverted from `job104.RoIDs` / `job104.RemoteWorkIDs` at init —
ids.go stays the single source of truth.

### Handler and schema updates

- Both tool handlers return the converted output as structured content.
- Input schema descriptions change: "verify each result's jobRo" →
  "verify each result's job_type"; same for remoteWorkType → remote.

### Testing

- `TestJob104SearchJobE2E`: `wantResp` becomes a hand-typed
  `job104SearchOutput` golden (e.g. `JobRo: 1` → `JobType: "Full-time"`;
  entries with `remoteWorkType: 0` have no `remote`). Structured content is
  re-unmarshaled into `job104SearchOutput` before the single assert.Equal.
- New `TestJob104GetJobDetailE2E`: calls `104_get_job_detail` against the
  mock server, hand-typed `job104DetailOutput` golden.
- Provider-level `client_test.go` untouched — it tests the HTTP client;
  conversion is an MCP-layer concern.

## Out of scope

- Trimming fields the LLM doesn't need (e.g. custNo) — separate discussion.
- Converting `specialty` / `jobCategory` code+description pairs — they already
  carry human-readable descriptions.
