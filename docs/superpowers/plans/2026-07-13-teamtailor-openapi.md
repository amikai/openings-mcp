# Teamtailor Career Site Feed OpenAPI — Implementation Plan

**Goal:** Add an ogen-generated client for Teamtailor's unauthenticated
`/jobs.json` career-site feed.

**Architecture:** A per-career-host client calls one full-dump endpoint. The
minimal OpenAPI schema models JSON Feed identity/content fields and Schema.org
locations; generated code is committed but never hand-edited.

**Spec:** `docs/superpowers/specs/2026-07-13-teamtailor-openapi-design.md`

## Tasks

- [x] Capture `jobs_req.hurl` + `jobs_rsp.json` from a live board and an
  unknown-tenant 404 pair under `internal/provider/teamtailor/testdata/`.
- [x] Write `internal/provider/teamtailor/openapi.yaml` and `gen.go`.
- [x] Run `go generate ./internal/provider/teamtailor`.
- [x] Add a fixture-replaying `mocksrv.go` and `client_test.go` covering the
  successful feed and empty-body 404 union response.
- [x] Add the spec to `OPENAPI_SPECS` in `Makefile`.
- [x] Run hurl formatting/lint, OpenAPI validation, and provider tests.
