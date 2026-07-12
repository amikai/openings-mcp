---
name: verify-companies
description: Use when auditing the curated ATS rosters for stale entries, pruning dead boards or tenants, promoting candidates from unverified/ into internal/provider/*/companies.yaml, or when search_jobs_by_company errors on a company that used to work.
---

# Verify Companies

## Overview

`cmd/verify-companies` sweeps every curated `companies.yaml` entry
through the real `internal/ats` adapter path — the same code the MCP server serves —
and reports each entry as OK with its total job count, or ERROR with the
error message. Two workflows build on it: **audit** (find and prune
stale roster entries) and **promotion** (move `unverified/<provider>.yaml`
candidates into the curated roster).

## The sweep

```
go run ./cmd/verify-companies [--provider ashby,greenhouse,lever,workday] \
    [--format text|json] [--concurrency N] [--timeout D]
```

- Classification is binary: any Search failure is ERROR — a stale
  identifier's 404 and a transient timeout or 5xx look the same at the
  status level; the detail column tells them apart.
- `zero-job` in the summary is informational: board live, no current
  openings. Not a failure.
- Keep the default 300s `--timeout`: the largest full-dump boards
  (e.g. Palantir, Veeva on lever) take minutes to download, and 60s
  produced false ERRORs.
- Exits non-zero when any entry ERRORs, so it doubles as a CI check.

## Audit workflow

1. Sweep, scoped with `--provider` when you already know the target;
   `--format json` when you'll process the results.
2. Re-run the failures once before touching the roster — transient
   timeouts and 5xx clear on retry; stale identifiers (404, unknown
   board) don't. Read the detail column, don't just count statuses.
3. For each confirmed-stale ERROR and each OK entry with 10 or fewer
   jobs, web-search the company's current careers page
   (`<company> careers`) — a dead or near-empty board is often a
   leftover after the company moved to another ATS. Then, by what the
   careers URL shows (adapter host shapes:
   `careersHostPatternsByAdapter`, `internal/ats/registry.go`):
   - Another supported adapter's host → the company migrated. Move the
     entry to that provider's `companies.yaml`, slug taken from the
     URL, and verify with `--provider <new>`.
   - Same provider, different slug → fix the slug.
   - Same provider, same slug → small company; leave it.
   - No working careers page or unsupported ATS → remove the entry. If
     only the display name is in doubt, move it to
     `unverified/<provider>.yaml`.
4. `go test ./...` — registry tests catch roster mistakes.
5. Commit per the `roster:` convention in AGENTS.md.

## Promotion workflow

`unverified/` entries have a live-verified board/tenant but an
unconfirmed display name — often just the slug. The directory is not
go:embed'd, so it never affects the built binary.

1. Confirm the real display name from an authoritative source (the
   company's own site or careers page). Never promote a guessed name;
   that's the whole reason the entry is quarantined.
2. Move the entries into the provider's `companies.yaml` (same YAML
   shape) and delete them from `unverified/<provider>.yaml`.
3. `go run ./cmd/verify-companies --provider <name>` — rosters are
   go:embed'd, so `go run` picks the additions up. Drop entries that
   ERROR (re-check stale vs transient as in the audit).
4. `go test ./...` — `ats.NewRegistry` requires every display name and
   slug to be globally unique across all adapters combined. Move
   collisions back to `unverified/` for a human to sort out rather than
   inventing disambiguated names.
5. One roster per PR, one `roster:` commit (AGENTS.md).

## Common mistakes

- Pruning on a transient error — a timeout on a huge full-dump board is
  not a stale entry. Retry first.
- Removing a stale entry without checking for an ATS migration — when
  the company moved providers, the fix is a move, not a removal.
- Workday roster rows sharing one tenant: `WorkdayAdapter.Roster()`
  dedupes by tenant and serves only the first row, so the rest sit in
  the file looking valid while being unreachable. One row per tenant.
- Promoting entries whose display name was never confirmed.
- Batching several providers' rosters into one PR — AGENTS.md wants one
  roster per PR.
