# Oracle Recruiting Cloud Candidate Experience OpenAPI — Implementation Plan

**Goal:** Add an ogen-generated client for Oracle Recruiting Cloud's
anonymous Candidate Experience job search and detail resources.

**Architecture:** A per-Fusion-origin client calls two JSON collection
endpoints. Callers construct Oracle `finder` values from a validated career
site's internal site number. The spec is trimmed from Oracle's official Fusion
Cloud HCM OpenAPI document and corrected against captured public traffic.

**Spec:** `docs/superpowers/specs/2026-07-17-oracle-openapi-design.md`

## Tasks

- [x] Locate Oracle's official Fusion Cloud HCM OpenAPI document.
- [x] Verify the public Candidate Experience request shape across live tenants.
- [x] Capture unfiltered, keyword-filtered, facet, detail, and missing-detail
  hurl + JSON fixtures under `internal/provider/oracle/testdata/`.
- [x] Write `internal/provider/oracle/openapi.yaml` and `gen.go`.
- [x] Run `go generate ./internal/provider/oracle`.
- [x] Add a fixture-replaying `mocksrv.go` and generated-client tests.
- [x] Add the spec to `OPENAPI_SPECS` in `Makefile`.
- [x] Run hurl formatting/lint, OpenAPI validation, and provider tests.
