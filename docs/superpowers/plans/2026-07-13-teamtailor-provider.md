# Teamtailor Provider Package — Implementation Plan

**Goal:** Add a fixture-backed Teamtailor provider package and a verified
five-company seed roster.

**Spec:** `docs/superpowers/specs/2026-07-13-teamtailor-provider-design.md`

## Tasks

- [x] Add `companies.yaml` with the five live-verified seed companies.
- [x] Add `companies.go` with embedded parsing, sorting, host indexing, and
  `CareersURL`.
- [x] Add roster tests for sorting/indexing, duplicate hosts, and URL output.
- [x] Add the reusable fixture mock and generated-client tests.
- [x] Run `go test ./internal/provider/teamtailor`.
