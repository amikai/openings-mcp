# Teamtailor Debug CLI — Implementation Plan

**Goal:** Add roster-aware Teamtailor `companies`, `search`, and `get`
subcommands over the generated Career Site Feed client.

**Spec:** `docs/superpowers/specs/2026-07-13-teamtailor-cli-design.md`

## Tasks

- [x] Build the ff/v4 command tree and reject stray positional arguments.
- [x] Validate `--host` through `CompaniesByHost` and `--id` before network I/O.
- [x] Implement text/JSON company, search-summary, and full-detail output.
- [x] Convert HTML descriptions to plain text and join structured locations.
- [x] Add focused validation tests and run `go test ./cmd/teamtailor`.
