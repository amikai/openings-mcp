# Provider Schema Nullable Sweep Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop upstream `null` values from crashing job-search decoding across every ogen-generated ATS/job-board provider, by marking every non-identifier response-schema field `nullable: true` (or, for the one field known to vary in *type* not just nullability, loosening it to untyped) — closing out issue #123 with a fix that covers the whole provider surface, not just the two fields that were caught.

**Architecture:** Pure OpenAPI-spec edits + `go generate` regen. No adapter/tool Go logic changes: ogen's `OptString` → `OptNilString` transition keeps the same `.Value string` accessor (confirmed against Greenhouse's existing 177 `OptNilString` usages), so every existing call site keeps compiling and behaves the same way on `null` as it already does on "field absent."

**Tech Stack:** Go, ogen (`github.com/ogen-go/ogen/cmd/ogen`) OpenAPI codegen, existing `internal/ats` unified adapters and `internal/openingsmcp` per-provider MCP tools.

## Global Constraints

- **Identifier fields never become nullable.** An identifier is a field whose value is fed back into a follow-up API call as a lookup key (job ID, facet ID, path segment). A `null` there is a genuine data-integrity problem and should keep failing loudly. Every task below states its provider's identifier field(s) explicitly.
- **Only `components.schemas` properties are in scope.** Path/query parameters are request inputs, never response data — do not touch them.
- **Only scalar leaf properties** (`type: string`, `integer`, `number`, `boolean`) get `nullable: true` added directly. `$ref` properties and `type: array` properties are left alone — a `$ref`'s target schema gets its own scalar leaves swept when that schema is visited; an array being null (rather than merely empty) is a known gap **not covered by this pass** (see "Known limitation" below).
- **The edit pattern is always the same:** insert a `nullable: true` line immediately after the property's `type:` line, at the same indentation, before any `description:`/`example:`/`enum:`/`format:` lines that follow. This matches the pattern already used for the handful of fields in these files that are already nullable (e.g. Lever's `Posting.country`).
- **No adapter/tool Go code changes in this plan.** If regenerating a spec ever produces a compile error in `internal/ats/*.go` or `internal/openingsmcp/*.go` (meaning some call site treats a field as non-optional in a way `OptNilString` breaks), stop and report it — don't paper over it — since the design's safety argument depends on this not happening.
- **Known limitation, intentionally out of scope:** an array-typed field sent as `null` (rather than omitted or empty) would still crash decoding the same way a null scalar does — Lever's `PostingCategories.allLocations` is one such field that's actually read (`postingLocations()` in `cmd/lever/main.go`, `cats.AllLocations` in `internal/ats/lever.go`). This pass doesn't touch array nullability; if the full-roster verification in Task 9 turns up a real null-array crash, that's new evidence to bring back for a follow-up, not something to fix silently here.
- **Commit steps are listed per the plan template's convention, but do not run `git commit` without the user's explicit go-ahead for each one** — this repo's standing preference is no auto-commit even when a skill/plan's steps say to commit.
- **Providers excluded from this pass, confirmed by investigation, no task needed:**
  - **Google, LinkedIn, TSMC** (`internal/provider/{google,linkedin,tsmc}/openapi.yaml`): no `components.schemas` section at all. These are HTML-scraping providers — every response schema is a single opaque `type: string` (the raw page/fragment). There is nothing to mark nullable in the OpenAPI layer; hardening these would mean changing the hand-written Go scraper structs in `client.go`/`parse.go`, which is a different mechanism and out of scope for this design.
  - **Synopsys** (`internal/provider/synopsys/openapi.yaml`): confirmed unregistered — grep of `internal/openingsmcp/*.go` and `cmd/openings-mcp/main.go` for "synopsys" returns zero matches, it's not reachable via any MCP tool. Its `components.schemas` only defines `FacetFilter` (a request-side query-string helper), no job/response schema exists to harden. Nothing to do until it's wired in.

---

### Task 1: Add Ashby's spec to the Makefile's `validate-openapi` list

**Files:**
- Modify: `Makefile:3-12`

**Interfaces:** None — build-tooling only, no Go code involved.

- [ ] **Step 1: Add the missing line**

Current (`Makefile:3-12`):
```makefile
OPENAPI_SPECS := \
	internal/provider/cake/openapi.yaml \
	internal/provider/google/openapi.yaml \
	internal/provider/greenhouse/openapi.yaml \
	internal/provider/job104/openapi.yaml \
	internal/provider/lever/openapi.yaml \
	internal/provider/linkedin/openapi.yaml \
	internal/provider/nvidia/openapi.yaml \
	internal/provider/synopsys/openapi.yaml \
	internal/provider/tsmc/openapi.yaml \
	internal/provider/workday/openapi.yaml
```

New:
```makefile
OPENAPI_SPECS := \
	internal/provider/ashby/openapi.yaml \
	internal/provider/cake/openapi.yaml \
	internal/provider/google/openapi.yaml \
	internal/provider/greenhouse/openapi.yaml \
	internal/provider/job104/openapi.yaml \
	internal/provider/lever/openapi.yaml \
	internal/provider/linkedin/openapi.yaml \
	internal/provider/nvidia/openapi.yaml \
	internal/provider/synopsys/openapi.yaml \
	internal/provider/tsmc/openapi.yaml \
	internal/provider/workday/openapi.yaml
```

- [ ] **Step 2: Verify**

Run: `make validate-openapi` (requires Docker; if Docker isn't available, at minimum confirm the file list resolves: `echo $(find internal/provider -name openapi.yaml | sort)` should list 11 files matching the new `OPENAPI_SPECS`).

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build: validate ashby's openapi spec alongside the others"
```

---

### Task 2: Greenhouse — nullable sweep + untyped `metadata` fix (the evidenced bug from issue #123)

**Files:**
- Modify: `internal/provider/greenhouse/openapi.yaml`
- Test: `internal/provider/greenhouse/openapi.yaml` (regen output compiles + existing tests pass)

**Interfaces:**
- Consumes: nothing new — same `greenhouse.NewClient` constructor internal/ats/greenhouse.go already uses.
- Produces: same public Go API surface. Fields listed below switch from `OptString`/`OptInt` to `OptNilString`/`OptNilInt` — both still expose `.Value`, so `internal/ats/greenhouse.go` (`j.Title.Value`, `o.Name.Value`, `r.Location.Value.Name.Value`, etc.) needs no changes.

**Identifier fields (do not touch):** `JobSummary.id`, `JobDetail.id` (both `type: integer`, used as the `job_id` argument to `GetJob` — see `internal/ats/greenhouse.go:82` `strconv.Atoi(jobID)` / `GetJobParams{JobID: id}`).

- [ ] **Step 1: Add `nullable: true` to these scalar properties**

| Schema | Property | Current definition |
|---|---|---|
| `Location` | `name` | `type: string` |
| `Department` | `name` | `type: string` |
| `Office` | `name` | `type: string` |
| `Office` | `location` | `type: string` (**this is the exact field from issue #123's crash**) |
| `JobSummary` | `title` | `type: string` |
| `JobSummary` | `company_name` | `type: string` |
| `JobSummary` | `first_published` | `type: string, format: date-time` |
| `JobSummary` | `updated_at` | `type: string, format: date-time` |
| `JobSummary` | `education` | `type: string` (has `enum:`) |
| `JobSummary` | `absolute_url` | `type: string, format: uri` |
| `JobSummary` | `language` | `type: string` |
| `JobSummary` | `content` | `type: string` |
| `JobDetail` | `title` | `type: string` |
| `JobDetail` | `company_name` | `type: string` |
| `JobDetail` | `first_published` | `type: string, format: date-time` |
| `JobDetail` | `updated_at` | `type: string, format: date-time` |
| `JobDetail` | `education` | `type: string` (has `enum:`) |
| `JobDetail` | `content` | `type: string` |
| `JobDetail` | `absolute_url` | `type: string, format: uri` |
| `JobDetail` | `language` | `type: string` |

Worked example — `Office.location` (the field from the actual bug report), before:
```yaml
    Office:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
        location:
          type: string
        parent_id:
          type: integer
          nullable: true
```
After:
```yaml
    Office:
      type: object
      properties:
        id:
          type: integer
        name:
          type: string
          nullable: true
        location:
          type: string
          nullable: true
        parent_id:
          type: integer
          nullable: true
```
Apply the same one-line insertion to every row in the table above, at that property's existing indentation.

- [ ] **Step 2: Loosen `JobDetail.metadata[].value` to untyped (rule 3 — unconsumed, and real payloads have shown both `null` and array values, per issue #123's Pinterest/Hootsuite/Duolingo/Databricks detail-fetch failures)**

`JobSummary.metadata` is already correctly freeform (`items: { type: object }`, no nested `properties`) — `JobDetail.metadata` should match it exactly. Before:
```yaml
        metadata:
          type: array
          nullable: true
          items:
            type: object
            properties:
              id:
                type: integer
              name:
                type: string
              value_type:
                type: string
              value:
                type: string
```
After:
```yaml
        metadata:
          type: array
          nullable: true
          items:
            type: object
```

- [ ] **Step 3: Regenerate**

```bash
cd internal/provider/greenhouse && go generate ./... && cd -
```

- [ ] **Step 4: Build and test**

```bash
go build ./... && go test ./internal/provider/greenhouse/... ./internal/ats/... ./internal/openingsmcp/...
```
Expected: all pass, zero compile errors (per the `.Value`-compatibility argument above).

- [ ] **Step 5: Commit**

```bash
git add internal/provider/greenhouse/
git commit -m "fix(greenhouse): mark response fields nullable so upstream null values don't crash decoding

Fixes #123"
```

---

### Task 3: Workday — nullable sweep

**Files:**
- Modify: `internal/provider/workday/openapi.yaml`
- Test: existing `internal/provider/workday/*_test.go`, `internal/ats/workday_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: same public Go API surface (`.Value` accessor preserved on every affected field).

**Identifier fields (do not touch):**
- `JobSummary.externalPath` — fed into `workday.JobDetailKeyFromPath` and then a follow-up `GET /job/{location}/{titleSlug}` call (`internal/ats/workday.go:112,124,171`).
- `FacetNode.id` — collected into `workday.AppliedFacets` and re-submitted in a follow-up `POST /jobs` facet-filtered search (`internal/ats/workday.go:270,294,307`).
- Note: `JobPostingInfo.jobReqId` and `JobPostingInfo.externalUrl` look identifier-ish by name but are never re-submitted anywhere — they're display-only. They get `nullable: true` like any other field.

- [ ] **Step 1: Add `nullable: true` to these scalar properties**

| Schema | Property | Current definition |
|---|---|---|
| `JobsResponse` | `total` | `type: integer` |
| `FacetNode` | `facetParameter` | `type: string` |
| `FacetNode` | `descriptor` | `type: string` |
| `FacetNode` | `count` | `type: integer` |
| `JobSummary` | `title` | `type: string` |
| `JobSummary` | `locationsText` | `type: string` |
| `JobSummary` | `postedOn` | `type: string` |
| `JobPostingInfo` | `title` | `type: string` |
| `JobPostingInfo` | `jobDescription` | `type: string` |
| `JobPostingInfo` | `location` | `type: string` |
| `JobPostingInfo` | `postedOn` | `type: string` |
| `JobPostingInfo` | `timeType` | `type: string` |
| `JobPostingInfo` | `jobReqId` | `type: string` |
| `JobPostingInfo` | `externalUrl` | `type: string` |
| `ErrorResponse` | `errorCode` | `type: string` |
| `ErrorResponse` | `httpStatus` | `type: integer` |
| `ErrorResponse` | `message` | `type: string` |

Worked example — `FacetNode.count`, before:
```yaml
        count:
          type: integer
          description: Live matching job count for this leaf value.
          example: 1778
```
After:
```yaml
        count:
          type: integer
          nullable: true
          description: Live matching job count for this leaf value.
          example: 1778
```
Apply the same insertion (right after `type:`, before `description:`/`example:`) to every row above.

- [ ] **Step 2: Regenerate**

```bash
cd internal/provider/workday && go generate ./... && cd -
```

- [ ] **Step 3: Build and test**

```bash
go build ./... && go test ./internal/provider/workday/... ./internal/ats/... ./internal/openingsmcp/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/provider/workday/
git commit -m "fix(workday): mark response fields nullable so upstream null values don't crash decoding"
```

---

### Task 4: Lever — nullable sweep

**Files:**
- Modify: `internal/provider/lever/openapi.yaml`
- Test: existing `internal/provider/lever/*_test.go`, `internal/ats/lever_test.go`, `cmd/lever/main_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: same public Go API surface.

**Identifier field (do not touch):** `Posting.id` — fed into `GetPosting` as `PostingId` for the follow-up detail call (`internal/ats/lever.go:75`, `cmd/lever/main.go`'s `get` subcommand).

- [ ] **Step 1: Add `nullable: true` to these scalar properties**

| Schema | Property | Current definition |
|---|---|---|
| `Posting` | `text` | `type: string` |
| `Posting` | `createdAt` | `type: integer, format: int64` |
| `Posting` | `workplaceType` | `type: string` |
| `Posting` | `opening` | `type: string` |
| `Posting` | `openingPlain` | `type: string` |
| `Posting` | `description` | `type: string` |
| `Posting` | `descriptionPlain` | `type: string` |
| `Posting` | `descriptionBody` | `type: string` |
| `Posting` | `descriptionBodyPlain` | `type: string` |
| `Posting` | `additional` | `type: string` |
| `Posting` | `additionalPlain` | `type: string` |
| `Posting` | `hostedUrl` | `type: string` |
| `Posting` | `applyUrl` | `type: string` |
| `Posting` | `salaryDescription` | `type: string` |
| `Posting` | `salaryDescriptionPlain` | `type: string` |
| `PostingCategories` | `location` | `type: string` |
| `PostingCategories` | `commitment` | `type: string` |
| `PostingCategories` | `team` | `type: string` |
| `PostingCategories` | `department` | `type: string` |
| `PostingListEntry` | `text` | `type: string` |
| `PostingListEntry` | `content` | `type: string` |
| `SalaryRange` | `currency` | `type: string` |
| `SalaryRange` | `interval` | `type: string` |
| `SalaryRange` | `min` | `type: number` |
| `SalaryRange` | `max` | `type: number` |
| `ErrorResponse` | `ok` | `type: boolean` |
| `ErrorResponse` | `error` | `type: string` |

Worked example — `Posting.hostedUrl`, before:
```yaml
        hostedUrl:
          type: string
          description: Lever-hosted posting page URL.
```
After:
```yaml
        hostedUrl:
          type: string
          nullable: true
          description: Lever-hosted posting page URL.
```
Apply the same insertion to every row above.

- [ ] **Step 2: Regenerate**

```bash
cd internal/provider/lever && go generate ./... && cd -
```

- [ ] **Step 3: Build and test**

```bash
go build ./... && go test ./internal/provider/lever/... ./internal/ats/... ./cmd/lever/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/provider/lever/
git commit -m "fix(lever): mark response fields nullable so upstream null values don't crash decoding"
```

---

### Task 5: Ashby — nullable sweep

**Files:**
- Modify: `internal/provider/ashby/openapi.yaml`
- Test: existing `internal/provider/ashby/*_test.go`, `internal/ats/ashby_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: same public Go API surface.

**Identifier field (do not touch):** `JobPosting.id` — compared against the inbound `jobID` and returned as `JobDetail.JobID` (`internal/ats/ashby.go:74,78`).

Note: `CompensationTier.id` and `CompensationComponent.id` are separate, unrelated UUIDs never read by `internal/ats/ashby.go` — they are **not** identifiers under this rule and get `nullable: true` like any other field.

- [ ] **Step 1: Add `nullable: true` to these scalar properties**

| Schema | Property | Current definition |
|---|---|---|
| `JobBoardResponse` | `apiVersion` | `type: string` |
| `JobPosting` | `title` | `type: string` |
| `JobPosting` | `department` | `type: string` |
| `JobPosting` | `team` | `type: string` |
| `JobPosting` | `employmentType` | `type: string` (has `enum:`) |
| `JobPosting` | `location` | `type: string` |
| `JobPosting` | `publishedAt` | `type: string, format: date-time` |
| `JobPosting` | `isListed` | `type: boolean` |
| `JobPosting` | `jobUrl` | `type: string` |
| `JobPosting` | `applyUrl` | `type: string` |
| `JobPosting` | `descriptionHtml` | `type: string` |
| `JobPosting` | `descriptionPlain` | `type: string` |
| `JobPosting` | `shouldDisplayCompensationOnJobPostings` | `type: boolean` |
| `SecondaryLocation` | `location` | `type: string` |
| `PostalAddress` | `addressLocality` | `type: string` |
| `PostalAddress` | `addressRegion` | `type: string` |
| `PostalAddress` | `addressCountry` | `type: string` |
| `PostalAddress` | `postalCode` | `type: string` |
| `PostalAddress` | `streetAddress` | `type: string` |
| `CompensationTier` | `id` | `type: string` |
| `CompensationTier` | `tierSummary` | `type: string` |
| `CompensationComponent` | `id` | `type: string` |
| `CompensationComponent` | `summary` | `type: string` |
| `CompensationComponent` | `compensationType` | `type: string` |
| `CompensationComponent` | `interval` | `type: string` |

Worked example — `JobPosting.descriptionPlain`, before:
```yaml
        descriptionPlain:
          type: string
```
After:
```yaml
        descriptionPlain:
          type: string
          nullable: true
```
Apply the same insertion to every row above.

- [ ] **Step 2: Regenerate**

```bash
cd internal/provider/ashby && go generate ./... && cd -
```

- [ ] **Step 3: Build and test**

```bash
go build ./... && go test ./internal/provider/ashby/... ./internal/ats/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/provider/ashby/
git commit -m "fix(ashby): mark response fields nullable so upstream null values don't crash decoding"
```

---

### Task 6: Cake.me — nullable sweep

**Files:**
- Modify: `internal/provider/cake/openapi.yaml`
- Test: existing `internal/provider/cake/*_test.go`, `internal/openingsmcp/cake_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: same public Go API surface.

**Identifier field (do not touch):** `JobSearchItem.path` — the value `internal/openingsmcp/cake.go` reads out of each search result and later hands back into `cake_get_job_detail` as the `path` argument (`cake.go:176`, `GetJobDetailParams.Path`, `cake.go:239`).

Judgment call: `JobDetail.path` carries the same real-world value but is only ever copied to the tool's own output in this codebase — never fed into another API call — so per the strict "used as a lookup key" rule it is **not** treated as an identifier here and gets `nullable: true` like the rest of `JobDetail`'s fields.

- [ ] **Step 1: Add `nullable: true` to these scalar properties**

| Schema | Property | Current definition |
|---|---|---|
| `JobSearchResponse` | `total_entries` | `type: integer` |
| `JobSearchResponse` | `total_pages` | `type: integer` |
| `JobSearchResponse` | `per_page` | `type: integer` |
| `JobSearchResponse` | `current_page` | `type: integer` |
| `JobSearchItem` | `title` | `type: string` |
| `JobSearchItem` | `description` | `type: string` |
| `JobSearchPage` | `path` | `type: string` |
| `JobDetail` | `id` | `type: integer` |
| `JobDetail` | `path` | `type: string` |
| `JobDetail` | `page_path` | `type: string` |
| `JobDetail` | `title` | `type: string` |
| `JobDetail` | `description` | `type: string` |
| `JobDetail` | `requirements` | `type: string` |
| `ErrorResponse` | `msg` | `type: string` |
| `ErrorResponse` | `error` | `type: string` |

`JobSearchRequest` and `JobSearchFilters` (including its nested `salary.type`/`salary.currency`/`salary.min`/`salary.max`) are request-only schemas — never used in a response — so they're excluded from this pass per the Global Constraints scope (response schemas only).

Worked example — `JobDetail.requirements`, before:
```yaml
        requirements:
          type: string
```
After:
```yaml
        requirements:
          type: string
          nullable: true
```
Apply the same insertion to every row above.

- [ ] **Step 2: Regenerate**

```bash
cd internal/provider/cake && go generate ./... && cd -
```

- [ ] **Step 3: Build, test, and one real-upstream smoke check**

```bash
go build ./... && go test ./internal/provider/cake/... ./internal/openingsmcp/...
```

Since Cake isn't part of the 360-company ATS roster verified in Task 9, add a one-off smoke check here: build the server (`go build -o /tmp/openings-mcp-smoke ./cmd/openings-mcp`) and call `cake_search_jobs` with a real keyword (e.g. `{"keyword":"engineer"}`) through an MCP client (see Task 9's harness pattern) to confirm the regenerated types decode a real response without error.

- [ ] **Step 4: Commit**

```bash
git add internal/provider/cake/
git commit -m "fix(cake): mark response fields nullable so upstream null values don't crash decoding"
```

---

### Task 7: 104 (job104) — nullable sweep

**Files:**
- Modify: `internal/provider/job104/openapi.yaml`
- Test: existing `internal/provider/job104/*_test.go`, `internal/openingsmcp/job104_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: same public Go API surface.

**Identifier field (do not touch):** `JobSummary.link.job` — `internal/openingsmcp/job104.go:301` derives the job code from this field via `job104.JobCodeFromURL(j.Link.Job)`, and that code is fed into the follow-up `GetJobDetail` call (`job104.go:390`).

Note: `JobSummary.jobNo` looks identifier-ish by name, but the spec itself documents it as "a different, unrelated internal id" that 404s if used as a lookup key, and `job104.go` never reads it — it is **not** an identifier and gets `nullable: true` like any other field.

- [ ] **Step 1: Add `nullable: true` to these scalar properties**

| Schema | Property | Current definition |
|---|---|---|
| `JobsResponse` | `metadata.pagination.currentPage` | `type: integer` |
| `JobsResponse` | `metadata.pagination.lastPage` | `type: integer` |
| `JobsResponse` | `metadata.pagination.total` | `type: integer` |
| `JobSummary` | `jobNo` | `type: string` |
| `JobSummary` | `jobName` | `type: string` |
| `JobSummary` | `custName` | `type: string` |
| `JobSummary` | `custNo` | `type: string` |
| `JobSummary` | `link.cust` | `type: string` |
| `JobSummary` | `salaryHigh` | `type: integer` |
| `JobSummary` | `salaryLow` | `type: integer` |
| `JobSummary` | `s10` | `type: integer` |
| `JobSummary` | `jobAddrNoDesc` | `type: string` |
| `JobSummary` | `appearDate` | `type: string` |
| `JobSummary` | `applyCnt` | `type: integer` |
| `JobSummary` | `remoteWorkType` | `type: integer` |
| `JobSummary` | `jobRo` | `type: integer` |
| `JobSummary` | `period` | `type: integer` |
| `JobDetail` | `header.jobName` | `type: string` |
| `JobDetail` | `header.custName` | `type: string` |
| `JobDetail` | `header.custUrl` | `type: string` |
| `JobDetail` | `header.appearDate` | `type: string` |
| `JobDetail` | `header.isSaved` | `type: boolean` |
| `JobDetail` | `header.isApplied` | `type: boolean` |
| `JobDetail` | `contact.hrName` | `type: string` |
| `JobDetail` | `contact.email` | `type: string` |
| `JobDetail` | `contact.reply` | `type: string` |
| `JobDetail` | `condition.workExp` | `type: string` |
| `JobDetail` | `condition.edu` | `type: string` |
| `JobDetail` | `welfare.welfare` | `type: string` |
| `JobDetail` | `jobDetail.jobDescription` | `type: string` |
| `JobDetail` | `jobDetail.salary` | `type: string` |
| `JobDetail` | `jobDetail.salaryMin` | `type: integer` |
| `JobDetail` | `jobDetail.salaryMax` | `type: integer` |
| `JobDetail` | `jobDetail.jobType` | `type: integer` |
| `JobDetail` | `jobDetail.addressRegion` | `type: string` |
| `JobDetail` | `jobDetail.addressDetail` | `type: string` |
| `JobDetail` | `jobDetail.manageResp` | `type: string` |
| `JobDetail` | `jobDetail.needEmp` | `type: string` |
| `JobDetail` | `jobDetail.remoteWork.type` | `type: integer` |
| `JobDetail` | `jobDetail.remoteWork.description` | `type: string` |
| `JobDetail` | `industry` | `type: string` |
| `JobDetail` | `employees` | `type: string` |
| `JobDetail` | `custNo` | `type: string` |
| `CodeDescription` | `code` | `type: string` |
| `CodeDescription` | `description` | `type: string` |
| `ErrorResponse` | `message` | `type: string` |

(The dotted names above are nested inline objects, e.g. `JobsResponse.metadata.pagination.currentPage` means: inside `JobsResponse`'s `metadata` object property, inside its `pagination` object property, the `currentPage` field — add `nullable: true` at that innermost level, not on the containing `metadata`/`pagination` objects themselves, which aren't scalars.)

Worked example — `JobDetail.jobDetail.salaryMin`, before:
```yaml
            salaryMin:
              type: integer
```
After:
```yaml
            salaryMin:
              type: integer
              nullable: true
```
Apply the same insertion to every row above, at each field's own indentation level.

- [ ] **Step 2: Regenerate**

```bash
cd internal/provider/job104 && go generate ./... && cd -
```

- [ ] **Step 3: Build, test, and one real-upstream smoke check**

```bash
go build ./... && go test ./internal/provider/job104/... ./internal/openingsmcp/...
```

104 isn't part of the 360-company ATS roster in Task 9 either — add a smoke check here the same way as Task 6: call `104_search_jobs` with a real keyword through an MCP client and confirm it decodes without error.

- [ ] **Step 4: Commit**

```bash
git add internal/provider/job104/
git commit -m "fix(job104): mark response fields nullable so upstream null values don't crash decoding"
```

---

### Task 8: NVIDIA — nullable sweep

**Files:**
- Modify: `internal/provider/nvidia/openapi.yaml`
- Test: existing `internal/provider/nvidia/*_test.go`, `internal/openingsmcp/nvidia_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: same public Go API surface.

**Identifier field (do not touch):** `JobSummary.externalPath` — split by `nvidia.SplitExternalPath` into `location`/`titleSlug` and fed into the follow-up `GetJobDetail` call (`internal/openingsmcp/nvidia.go:276-292`). Same Workday-derived shape and same reasoning as Task 3.

- [ ] **Step 1: Add `nullable: true` to these scalar properties**

| Schema | Property | Current definition |
|---|---|---|
| `JobsResponse` | `total` | `type: integer` |
| `JobSummary` | `title` | `type: string` |
| `JobSummary` | `locationsText` | `type: string` |
| `JobSummary` | `postedOn` | `type: string` |
| `JobPostingInfo` | `title` | `type: string` |
| `JobPostingInfo` | `jobDescription` | `type: string` |
| `JobPostingInfo` | `location` | `type: string` |
| `JobPostingInfo` | `postedOn` | `type: string` |
| `JobPostingInfo` | `timeType` | `type: string` |
| `JobPostingInfo` | `jobReqId` | `type: string` |
| `JobPostingInfo` | `externalUrl` | `type: string` |
| `ErrorResponse` | `errorCode` | `type: string` |
| `ErrorResponse` | `httpStatus` | `type: integer` |
| `ErrorResponse` | `message` | `type: string` |

This is the same field set/shape as Task 3 (Workday) — NVIDIA's provider is Workday-based under the hood with its own generated package. Apply the same insertion pattern (worked example in Task 3 applies verbatim here).

- [ ] **Step 2: Regenerate**

```bash
cd internal/provider/nvidia && go generate ./... && cd -
```

- [ ] **Step 3: Build, test, and one real-upstream smoke check**

```bash
go build ./... && go test ./internal/provider/nvidia/... ./internal/openingsmcp/...
```

NVIDIA is a single-company careers site, not part of the 360-company ATS roster in Task 9 — add a smoke check the same way as Task 6: call `nvidia_search_jobs` for real through an MCP client and confirm it decodes without error.

- [ ] **Step 4: Commit**

```bash
git add internal/provider/nvidia/
git commit -m "fix(nvidia): mark response fields nullable so upstream null values don't crash decoding"
```

---

### Task 9: Full-roster live verification, update issue #123

**Files:**
- Create (temporary, not committed): a throwaway Go harness under `cmd/mcptest-temp/main.go`, deleted at the end of this task — same disposable pattern used to originally catch the bug (see the earlier `sweep_test.go`/`sweep_results.md` artifacts from this investigation).

**Interfaces:**
- Consumes: `github.com/modelcontextprotocol/go-sdk/mcp` (`mcp.NewClient`, `mcp.CommandTransport`, `(*ClientSession).CallTool`) — same client-side API already proven out earlier in this investigation.
- Produces: a pass/fail report per company, used to decide whether to close issue #123.

- [ ] **Step 1: Build the binary with every Task above already merged**

```bash
go build -o /tmp/openings-mcp-verify ./cmd/openings-mcp
```

- [ ] **Step 2: Write the full-roster harness**

Same shape as the 30-company harness used earlier in this investigation, but reading the full curated roster instead of a sample:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/ashby"
	"github.com/amikai/openings-mcp/internal/provider/greenhouse"
	"github.com/amikai/openings-mcp/internal/provider/lever"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type searchOut struct {
	Data       []map[string]any `json:"data"`
	TotalCount int              `json:"total_count"`
}

func main() {
	binPath := os.Args[1]
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "ats-full-sweep", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(binPath)}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer session.Close()

	groups := map[string][]string{}
	for _, c := range lever.Companies {
		groups["Lever"] = append(groups["Lever"], c.Name)
	}
	for _, c := range ashby.Companies {
		groups["Ashby"] = append(groups["Ashby"], c.Name)
	}
	for _, c := range greenhouse.Companies {
		groups["Greenhouse"] = append(groups["Greenhouse"], c.Name)
	}
	// Workday's curated roster lives in JSON, not a Go slice — load it directly.
	wdRaw, err := os.ReadFile("internal/provider/workday/sp500_workday_confirmed.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, "read workday roster:", err)
		os.Exit(1)
	}
	var wdEntries []struct{ Company string `json:"company"` }
	if err := json.Unmarshal(wdRaw, &wdEntries); err != nil {
		fmt.Fprintln(os.Stderr, "parse workday roster:", err)
		os.Exit(1)
	}
	for _, e := range wdEntries {
		groups["Workday"] = append(groups["Workday"], e.Company)
	}

	totalPass, totalFail := 0, 0
	for _, group := range []string{"Workday", "Lever", "Ashby", "Greenhouse"} {
		companies := groups[group]
		pass, fail := 0, 0
		fmt.Printf("\n=== %s (%d companies) ===\n", group, len(companies))
		for _, company := range companies {
			cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			res, err := session.CallTool(cctx, &mcp.CallToolParams{
				Name:      "search_jobs_by_company",
				Arguments: map[string]any{"company": company},
			})
			cancel()
			if err != nil || res.IsError {
				fail++
				msg := ""
				if err != nil {
					msg = err.Error()
				} else {
					for _, c := range res.Content {
						if tc, ok := c.(*mcp.TextContent); ok {
							msg = tc.Text
						}
					}
				}
				fmt.Printf("FAIL   %-35s %s\n", company, msg)
				continue
			}
			var out searchOut
			b, _ := json.Marshal(res.StructuredContent)
			if err := json.Unmarshal(b, &out); err != nil {
				fail++
				fmt.Printf("FAIL   %-35s bad structured content: %v\n", company, err)
				continue
			}
			pass++
			fmt.Printf("OK     %-35s total_count=%d\n", company, out.TotalCount)
			time.Sleep(200 * time.Millisecond)
		}
		fmt.Printf("--- %s: %d/%d passed ---\n", group, pass, len(companies))
		totalPass += pass
		totalFail += fail
	}
	fmt.Printf("\n=== TOTAL: %d/%d passed ===\n", totalPass, totalPass+totalFail)
}
```

- [ ] **Step 3: Run it against all 360 curated companies**

Use the Write tool to save the Step 2 code verbatim to `cmd/mcptest-temp/main.go` (creating the `cmd/mcptest-temp` directory first if needed), then:

```bash
go run ./cmd/mcptest-temp /tmp/openings-mcp-verify > /tmp/full-sweep-results.txt 2>&1
cat /tmp/full-sweep-results.txt
```

Expected: `TOTAL: 360/360 passed`. If anything still fails, read the printed error — per Global Constraints, do not add adapter-side workarounds to force a pass; report the failure (which field, which company) so the classification rule can be revisited.

- [ ] **Step 4: Clean up the throwaway harness**

```bash
rm -rf cmd/mcptest-temp
```

- [ ] **Step 5: Update issue #123**

Read the `TOTAL:` line from `/tmp/full-sweep-results.txt` first, then pick exactly one of these two paths:

**If it reads `TOTAL: 360/360 passed`:**
```bash
gh issue comment 123 --body "Full-roster verification after the schema nullable sweep (Tasks 2-8): 360/360 curated companies passed \`search_jobs_by_company\` through the real MCP server. Closing."
gh issue close 123
```

**If it reads anything else** (e.g. `TOTAL: 358/360 passed`): open `/tmp/full-sweep-results.txt`, copy every `FAIL` line verbatim, and post them instead — do not close the issue:
```bash
gh issue comment 123 --body "Full-roster verification after the schema nullable sweep (Tasks 2-8): <N>/360 passed. Remaining failures:

<paste every FAIL line from /tmp/full-sweep-results.txt here, one per line>

Leaving open pending a look at these."
```

- [ ] **Step 6: Commit** (only the Makefile/spec changes from Tasks 1-8 should already be committed by this point; this task produces no file changes to commit — it's verification only)
