# Workable ATS Adapter — Plan

Design: `docs/superpowers/specs/2026-07-18-workable-ats-adapter-design.md`.
Template example: SmartRecruiters (`internal/provider/smartrecruiters`,
`internal/ats/smartrecruiters.go`, `cmd/smartrecruiters`).

## Fixture facts (used by test assertions)

Captured 2026-07-18 against account `blueground` (29 open jobs):

- `jobs_rsp.json`: total 29, 10 results, has `nextPage`.
- `jobs_page2_rsp.json`: the page fetched with page 1's token; 10 results.
- `jobs_filtered_rsp.json`: `query=engineer` narrows total to 9.
- `jobs_filters_rsp.json`: 16 locations, 2 departments (City Core 435335,
  Shared Services 435343), worktypes `full|contract|temporary`, workplaces
  `on_site|hybrid|remote`.
- `job_detail_rsp.json`: shortcode `B02DA69C8F` (Senior Software Engineer,
  iOS — published 2026-07-14; recapture when it closes), with
  description/requirements/benefits.
- `job_not_found_rsp.txt`, `jobs_unknown_company_rsp.txt`: 404 text bodies.

## Tasks

1. `internal/provider/workable/openapi.yaml` (three endpoints above, nullable
   per fixtures) + `gen.go`, `go generate`, add to `OPENAPI_SPECS`,
   `make validate-openapi`.
2. Provider package: `mocksrv.go` replaying fixtures, `client_test.go`,
   `companies.yaml` (3–5 live-verified seeds) + `companies.go`, `doc.go`.
3. `cmd/workable`: ff/v4 `search` / `detail` / `companies` subcommands
   mirroring `cmd/smartrecruiters`; live manual checks.
4. `internal/ats/workable.go` + tests; register in `newATSRegistry`,
   `careersHostPatternsByAdapter`, verify-companies `providerOrder`.
5. Live MCP stdio smoke test over the roster; README provider list.

## Final verification

`go build ./... && go test ./... && make validate-openapi hurl-lint` plus the
stdio smoke test from the integrate-new-provider skill.
