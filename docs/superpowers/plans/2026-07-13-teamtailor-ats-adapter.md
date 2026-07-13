# Teamtailor Unified ATS Adapter — Implementation Plan

**Goal:** Expose Teamtailor career sites through the unified company tools and
the roster verifier.

**Spec:** `docs/superpowers/specs/2026-07-13-teamtailor-ats-adapter-design.md`

## Tasks

- [x] Implement `TeamtailorAdapter` with roster, hosted/custom URL parsing,
  full-dump search/filter mapping, and detail lookup.
- [x] Add fixture-backed adapter tests for success, filtering, URL parsing,
  unknown hosts/items, and stable ordering.
- [x] Register the adapter in the MCP server and URL teaching patterns.
- [x] Add it to `verify-companies` provider parsing/building and tests.
- [x] Update README provider coverage.
- [x] Run focused tests, `go test ./...`, formatting, vet/lint targets available
  in the repository, OpenAPI validation, and live hurl replay.
