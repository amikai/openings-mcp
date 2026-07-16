# verify-companies cmd design

Date: 2026-07-12
Issue: #91 — "verify tenant identifiers are still valid"

## Purpose

A standalone CLI, `cmd/verify-companies`, that sweeps the four curated ATS
rosters (`internal/provider/{ashby,greenhouse,lever,workday}/companies.yaml`,
~550 entries) and verifies each entry by running a real search through the
unified `internal/ats` adapters — the same code path the MCP server serves.
Each entry's report line includes its total job count, so the sweep doubles
as a roster health report (a valid tenant with 0 jobs is visible at a
glance). It prints a per-entry report and exits non-zero when entries have
gone stale, so it works both as a manual audit tool and in CI.

## Verification call

For every roster entry, call that adapter's
`Search(ctx, slug, ats.SearchParams{Page: 1})`:

- **ashby / greenhouse / lever** — full-dump adapters: one GET of the whole
  board, `TotalCount` = every listed job.
- **workday** — server-side search: one POST of page 1, `TotalCount` from
  the API.

Classification is binary: a successful Search is OK (with job count); any
Search error — a stale identifier's upstream 404 and a transient timeout
or 5xx alike — is ERROR, with the error message carried in the detail
column for telling them apart by eye. (An earlier revision split out an
INVALID status by unwrapping typed status-code errors; dropped as not
worth the classification machinery.)

## Structure

- `cmd/verify-companies/main.go` only. No test file.
- CLI built with `ff/v4`, matching the other `cmd/` tools.
- The cmd depends only on `internal/ats`: adapters are constructed with the
  same base URLs as `cmd/openings-mcp/main.go`, rosters come from each
  adapter's `Roster()`, and verification goes through `Search()`. The
  provider packages are not imported directly.
- Entries fan out through a bounded worker pool.

## Flags

- `--provider` — comma-separated subset of `ashby,greenhouse,lever,workday`;
  default all four.
- `--timeout` — per-request timeout, default 300s (the largest full-dump
  boards, e.g. Palantir and Veeva on lever, take minutes to download; 60s
  produced false ERRORs).
- `--concurrency` — worker pool size, default 8.
- `--format` — `text` (default) or `json`.

## Output and exit code

Text format: one line per entry, `STATUS  provider  company  slug  jobs
detail`, grouped by provider (`jobs` is the total job count for OK entries,
blank otherwise; `detail` is the error message for ERROR entries), followed
by a summary of counts per status plus a `zero-job` count (OK entries whose
board is live but currently lists no jobs). JSON format: one object
`{"results": [...], "summary": {"ok": N, "error": N, "zero_job": N}}` where
each result carries provider, company, slug, status, job count, and detail.

Exit codes: any ERROR → 1; all OK → 0.

## Addendum (2026-07-17): Detail probe

Issue: #196 — search-only verification let a tenant pass the sweep while
every `JobDetail` call failed on a divergent detail template.

Each successful Search is now followed by one `Detail` probe on the first
job of page 1, with the same slug. Classification gains a third status,
`DETAIL_ERROR`: search succeeded but the probe failed — adapter code to
fix, not the roster. It keeps the job count, carries the probed job ID
plus error in the detail column, and counts toward exit code 1.

- `TotalCount == 0` is the only zero-job case; no probe, stays OK.
- `TotalCount > 0` with an empty page 1 (the adapter dropped every
  summary, e.g. Workday entries without `externalPath`) is DETAIL_ERROR:
  the detail path cannot be verified.
- Search and probe each get their own `--timeout`, keeping the flag's
  per-request meaning. Cost: at most one extra request per entry.

This addendum supersedes "Classification is binary" and "No test file"
above: `main_test.go` covers the probe classification with a fake adapter.
