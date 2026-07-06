# Ashby Provider — Codegen, Tests, and Company Roster — Design

## Context

[PR #82](https://github.com/amikai/openings-mcp/pull/82) landed
`internal/provider/ashby/openapi.yaml` (see
`docs/superpowers/specs/2026-07-06-ashby-openapi-design.md`). This round
completes the provider package: committed ogen codegen, a mock server and
client tests, and a curated company → board-slug roster. These are items 1
and 2 of the previous design's follow-up list; item 3 (MCP tool wiring in
`internal/openingsmcp/`) is intentionally dropped — no MCP integration in
this round or a planned later one.

Everything follows existing provider conventions; nothing here invents a
new shape. The reference implementation is `internal/provider/workday/`
(the other multi-tenant provider), with cake as a secondary reference.

## 1. Codegen

- `gen.go` — the standard one-liner, identical to every other provider:
  `//go:generate go tool github.com/ogen-go/ogen/cmd/ogen --target . -package ashby --clean openapi.yaml`
- Generated `oas_*_gen.go` files are committed. The previous round already
  proved the spec generates and compiles cleanly and that the generated
  `JobBoardResponse` decodes live data.
- No hand-written client wrapper: the generated `Client`
  (`NewClient("https://api.ashbyhq.com")`, method `GetJobBoard`) is the
  client, exactly as in cake/workday. Ashby needs no equivalent of
  workday's `path.go` — one fixed production server, and the board slug is
  an ordinary path parameter the generated client already handles.

## 2. Mock server, fixtures, tests

- `testdata/` — raw captures from one real, small board (chosen at
  implementation time from the verified roster: prefer ~5–20 jobs so each
  fixture stays around or under ~100K; repo precedent allows up to ~960K):
  - `board_rsp.json` — without compensation
  - `board_comp_rsp.json` — with `includeCompensation=true`
  - `board_req.sh` — workday-style capture script recording exactly how
    the fixtures were produced (both curl commands, piped through `jq .`).
- `mocksrv.go` — embeds both fixtures; `NewMockServer()` serves
  `/posting-api/job-board/{board}` and picks the fixture by the
  `includeCompensation` query parameter. Exported like the other
  providers' mock servers so later rounds (or other packages) can reuse it.
- `client_test.go` — generated client pointed at the mock server, workday
  style with exact assertions:
  - `TestGetJobBoard`: no-compensation call; assert the decoded response
    matches the fixture exactly (apiVersion, job count, and a
    representative job's fields including enums and optional fields).
  - `TestGetJobBoardWithCompensation`: assert the mock saw
    `includeCompensation=true` and the decoded compensation structure
    (tiers, components, a nullable field such as a null tier `title` if
    the chosen board exhibits one — otherwise nullable coverage rests on
    the schema's existing live validation from the previous round).
  - `TestGetJobBoardNotFound`: mock returns 404 `Not Found` text/plain for
    an unknown board; assert the generated client surfaces the typed 404
    response rather than an error.

## 3. Company roster

- `companies.yaml` — curated list of organizations hosting public Ashby
  boards. Entry shape:

  ```yaml
  - company: "OpenAI"
    board: "openai"
  ```

  Curation process (runs during implementation): assemble a candidate list
  of notable tech companies believed to use Ashby (OpenAI, Ramp, Notion,
  Linear, Deel, Replit, Supabase, Modal, PostHog, Runway, …), probe
  `https://api.ashbyhq.com/posting-api/job-board/{slug}` for each, and keep
  only candidates answering HTTP 200 with valid job-board JSON. Target
  30–50 verified entries; candidates that 404 are dropped, not guessed at.
  The probe script is kept in `testdata/verify_companies.sh` so the roster
  can be re-verified later.
- `companies.go` — workday's `companies.go` adapted to Ashby's simpler
  addressing (one slug instead of tenant/instance/site):
  - `//go:embed companies.yaml`
  - `type Company struct { Name string; Board string }` with yaml/json tags
  - `func (c Company) BoardURL() string` →
    `https://jobs.ashbyhq.com/{board}` (the human-facing board page; API
    calls take `Board` directly as the path parameter)
  - `var Companies = mustLoadCompanies()` — sorted by company name
  - `var CompaniesByBoard = buildBoardIndex(Companies)` — keyed by
    lowercased board slug
  - Exported package vars, no getter functions (standing user preference).
- `companies_test.go` — asserts the embedded roster is non-empty and board
  slugs are unique (a duplicate would be silently swallowed by the
  `CompaniesByBoard` map). Workday documents its known duplicates in a
  comment instead; Ashby slugs have no such exception, so the test
  enforces uniqueness outright.

## Error handling

Unknown boards are already modeled in the spec as a 404 text/plain
response, so ogen generates a typed result for it — no extra client-side
error handling is needed. Other upstream failures surface as ogen's
standard unexpected-status errors, same as every other generated provider.

## Verification

- `go generate ./internal/provider/ashby/` reproduces the committed
  generated code with no diff.
- `go test ./internal/provider/ashby/` passes.
- `go vet ./internal/provider/ashby/` is clean.
- Every `companies.yaml` entry verified live by the probe script at
  implementation time.

## Out of scope

- MCP tool wiring (`internal/openingsmcp/`) — dropped per user decision,
  not deferred.
- Client-side search/filter helpers — nothing consumes them yet (YAGNI);
  the API's fetch-everything semantics are documented in the spec for
  whoever builds on the client.
