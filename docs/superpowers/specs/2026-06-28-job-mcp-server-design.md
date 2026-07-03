# Job MCP Server — Design

Date: 2026-06-28
Status: Approved (design), pending implementation plan

## Goal

Expose the existing Go job-board clients as a single MCP server over **stdio**,
using the official SDK `github.com/modelcontextprotocol/go-sdk`.

Tool surface follows the **per-board** design (option B): each board gets its own
precisely-typed tools, instead of one unified tool with a `board` parameter or a
fan-out aggregator. Rationale: each board's search request struct is genuinely
different, so a shared schema would lie to the LLM. The LLM is the orchestrator —
give it precise tools and let it fan out / pick boards itself.

**This iteration ships 104 and tsmc only** (4 tools). synopsys, google, cake are
deferred — same pattern, added later board-by-board.

## Field exposure principle

Expose every search field that is **meaningful to a human** searching for a job.
Do NOT expose fields whose values are opaque codes with no human-readable mapping
available in the repo — a raw code an LLM must guess is not human-meaningful.

- A concept that is human-meaningful AND has a code mapping (e.g. 104 `Area` →
  city, `RO` → full/part-time) is exposed as a **human-labeled enum** that the
  handler maps to the underlying code.
- A concept whose code table does not exist in the repo (104 `Edu`, `S9`
  experience codes) is **omitted** until a label↔code map is added. The
  underlying client keeps the field; the MCP tool just doesn't surface it yet.

## Non-Goals (deliberately deferred)

- synopsys / google / cake tools (next iterations).
- HTTP / Streamable-HTTP transport. Stdio only. Handlers are transport-decoupled,
  so HTTP can be added later without rewriting them.
- Rate limiter, singleflight, TTL cache. Add when upstream blocking is observed.
- Fan-out / aggregating search across boards.
- 104 company tools (`search_companies`, `company_detail`, `company_jobs`).
- 104 `Edu` / `S9` filters (no human-readable code map in repo yet).

## Tool Inventory (4 tools)

| Board | Tools |
|-------|-------|
| 104   | `104_search_jobs`, `104_get_job_detail` |
| tsmc  | `tsmc_search_jobs`, `tsmc_get_job_detail` |

### 104_search_jobs — input

| Field | Type | Required | Maps to | Notes |
|-------|------|----------|---------|-------|
| `keyword` | string | yes | `JobRequest.Keyword` | |
| `area` | enum | no | `JobRequest.Area` | `taipei`/`new_taipei`/`taoyuan`/`taichung`/`tainan`/`kaohsiung` → area code const. Only the 6 cities with consts defined today. |
| `job_type` | enum | no | `JobRequest.RO` (*int) | `full`→0, `part`→1 |
| `sort` | enum | no | `JobRequest.Order` (*int) | `newest`→15, `relevance`→1 |
| `remote` | enum | no | `JobRequest.RemoteWork` (*int) | `none`→0, `partial`→1, `full`→2 |
| `page` | int | no | `JobRequest.Page` (*int) | |

Handler holds small board-local label→code maps for area/job_type/sort/remote.

### 104_get_job_detail — input

| Field | Type | Required | Maps to |
|-------|------|----------|---------|
| `job_code` | string | yes | `Client.JobDetail(ctx, jobCode)` |

### tsmc_search_jobs — input

tsmc's list fields take opaque facet **codes**, but the client already defines
readable const maps for all four — so expose them as human enums mapped to codes
(same approach as 104 `area`). Handler holds label→code maps built from the
existing `tsmc` consts; unknown value → tool error.

| Field | Type | Required | Maps to | Enum labels → const |
|-------|------|----------|---------|---------------------|
| `keyword` | string | yes | `JobRequest.Keyword` | |
| `locations` | []string | no | `JobRequest.Locations` | `taiwan`→`LocTaiwan`, `canada`→`LocCanada`, `china`→`LocChina`, `germany_dresden`→`LocGermanyDresden`, `germany_munich`→`LocGermanyMunich`, `japan_yokohama`→`LocJapanYokohama`, `japan_osaka`→`LocJapanOsaka`, `japan_tsukuba`→`LocJapanTsukuba`, `japan_kumamoto`→`LocJapanKumamoto`, `korea`→`LocKorea`, `netherlands`→`LocNetherlands`, `usa_arizona`→`LocUSAArizona`, `usa_california`→`LocUSACalifornia`, `usa_massachusetts`→`LocUSAMassachusetts`, `usa_texas`→`LocUSATexas`, `usa_washington`→`LocUSAWashington`, `usa_washington_dc`→`LocUSAWashingtonDC` |
| `categories` | []string | no | `JobRequest.Categories` | `rd`→`CatRD`, `specialty_technology`→`CatSpecialtyTechnology`, `ic_design_technology`→`CatICDesignTechnology`, `manufacturing`→`CatManufacturing`, `facility_and_safety`→`CatFacilityAndSafety`, `product_development`→`CatProductDevelopment`, `ic_packaging_technology`→`CatICPackagingTechnology`, `testing_development`→`CatTestingDevelopment`, `quality_and_reliability`→`CatQualityAndReliability`, `it`→`CatIT`, `internal_audit`→`CatInternalAudit`, `business_development`→`CatBusinessDevelopment`, `customer_service`→`CatCustomerService`, `corporate_planning`→`CatCorporatePlanning`, `finance`→`CatFinance`, `human_resources`→`CatHumanResources`, `legal`→`CatLegal`, `materials_management`→`CatMaterialsManagement`, `corporate_sustainability`→`CatCorporateSustainability`, `administration`→`CatAdministration`, `accessibility_inclusion`→`CatAccessibilityInclusion` |
| `job_types` | []string | no | `JobRequest.JobTypes` | `technician`→`JobTypeTechnician`, `associate_engineer`→`JobTypeAssociateEngineer`, `engineer`→`JobTypeEngineer`, `manager`→`JobTypeManager`, `others`→`JobTypeOthers` |
| `employment_types` | []string | no | `JobRequest.EmploymentTypes` | `regular`→`EmployRegular`, `temporary`→`EmployTemporary`, `intern`→`EmployIntern`, `apprenticeship`→`EmployApprenticeship` |
| `page` | int | no | `JobRequest.Page` | |

`PerPage` and `Organization` not exposed (size tuning / no search field on the
client).

### tsmc_get_job_detail — input

| Field | Type | Required | Maps to |
|-------|------|----------|---------|
| `job_id` | string | yes | `Client.JobDetail(ctx, jobID)` |

## Architecture / Layers

```
internal/provider/job104/, internal/provider/tsmc/    existing clients — UNCHANGED (full power retained)
internal/jobmcp/                 NEW — MCP glue, one file per board
  ├ job104.go    registerJob104(s *mcp.Server, c *job104.Client)
  └ tsmc.go      registerTSMC(s *mcp.Server, c *tsmc.Client)
cmd/jobmcp/main.go               build clients → register all tools → server.Run(stdio)
```

Each `register*` file:
- defines the typed input struct(s) for that board's tools (with `json` +
  `jsonschema` description tags),
- holds any board-local label→code maps,
- implements handlers mapping input struct → existing client call → result text,
- calls `mcp.AddTool(server, &mcp.Tool{Name, Description}, handler)`.

### Handler signature (go-sdk current API)

```go
func(ctx context.Context, req *mcp.CallToolRequest, in Job104SearchInput) (*mcp.CallToolResult, any, error)
```

- Return value: **text content** built from the result. Reuse existing 104
  helpers `FormatSearchJobResponse` / `FormatJobDetail`. tsmc has no helper →
  format inline in `tsmc.go` (small, board-local).
- `Out` type param = `any` (empty) → text-only output, no output schema.
  Structured output is a future option, not needed for v1.
- Errors: wrap upstream failures into `&mcp.CallToolResult{IsError: true, ...}`
  so the LLM sees the message, instead of returning a Go error that kills the call.
- Use the request `ctx` with a per-call timeout (e.g. 30s) so a slow/hung board
  call fails that call without stalling the session.

### Client construction (in `main.go`)

Both 104 and tsmc use `NewClient(Config{HTTPClient, BaseURL})`. Share one
`*http.Client` across both (connection pooling; built once for the process
lifetime). `BaseURL` left default.

### Server lifetime

```go
server := mcp.NewServer(&mcp.Implementation{Name: "job-mcp", Version: "v0.1.0"}, nil)
registerJob104(server, c104)
registerTSMC(server, cTSMC)
server.Run(ctx, &mcp.StdioTransport{})  // blocks until stdin EOF, then clean exit
```

One process per client session (stdio is a persistent pipe, not per-call). No
background goroutines, no daemonizing — read stdin, exit on EOF.

## Dependencies to add

- `github.com/modelcontextprotocol/go-sdk` (MCP server + stdio transport).
- No others (rate/singleflight/cache deferred).

## Testing

- `job104.go` / `tsmc.go` each get one test: call the handler with a sample input
  backed by the board's existing `testdata/*_rsp.*` fixtures (clients already have
  httptest-style tests), assert the tool returns non-error text containing
  expected fields. Also assert one label→code mapping for 104 (e.g. `area=taipei`
  produces the Taipei code in the outgoing request).
- A `cmd/jobmcp` smoke test: build the server, list tools, assert all 4 names
  present.
- Reuse existing fixtures; do not hit live upstreams.

## Future (explicitly out of scope now)

- synopsys / google / cake tools (next iterations, same pattern).
- 104 `Edu` / `S9` filters once a label↔code map exists.
- HTTP transport + auth for multi-user/public deployment.
- Shared TTL cache + per-host rate limiter + singleflight (as an
  `http.RoundTripper` wrapping the shared client — transparent to client code)
  once blocking is real.
- `search_all_jobs` fan-out convenience tool on top of B.
- 104 company tools.
- Structured (typed) tool output in addition to text.
