# Ashby Provider OpenAPI Spec — Design

## Context

Ashby is a multi-tenant ATS: each customer organization hosts a public job
board at `jobs.ashbyhq.com/{jobBoardName}` (e.g. `jobs.ashbyhq.com/openai`),
backed by the unauthenticated Public Job Posting API documented at
<https://developers.ashbyhq.com/docs/public-job-posting-api>. This design
covers the first step of an Ashby provider: a hand-written
`internal/provider/ashby/openapi.yaml` describing that API, in the same style
as the existing cake/workday specs. ogen codegen, the client wrapper, a
`companies.yaml` roster, and MCP tool wiring are follow-up rounds.

## The upstream API (verified 2026-07-06)

One endpoint, no authentication:

```
GET https://api.ashbyhq.com/posting-api/job-board/{jobBoardName}[?includeCompensation=true]
```

It returns every listed job for the organization in a single response —
no server-side search, filtering, or pagination — and each job already
carries the full `descriptionHtml`/`descriptionPlain`. There is no separate
detail endpoint. Consequences recorded for the follow-up rounds: a future
`ashby_search_jobs` tool must fetch the whole board and filter client-side,
and "detail" is a lookup into the same payload rather than a second HTTP
call.

Observed behaviors (against the live `ashby` board):

- Unknown board → `404` with plain-text body `Not Found`.
- `apiVersion` is the JSON **string** `"1"`, not a number.
- Two fields appear in real responses but not in the official field table:
  `id` (UUID; `jobUrl`/`applyUrl` are built from it) and
  `shouldDisplayCompensationOnJobPostings` (boolean). Both are modeled and
  annotated as observed-only.
- The docs state missing data is **omitted**, not sent as `null`, so
  documented-optional fields are simply non-required properties.

## Spec shape

- `openapi: 3.1.0`, `info.description` explains the minimal-surface intent
  and where a `jobBoardName` comes from, matching the tone of
  `internal/provider/cake/openapi.yaml`.
- Server: `https://api.ashbyhq.com`.
- Single path `GET /posting-api/job-board/{jobBoardName}`
  (`operationId: getJobBoard`):
  - path parameter `jobBoardName` (string, required)
  - query parameter `includeCompensation` (boolean, optional; adds the
    `compensation` object to each job)
  - `200` → `JobBoardResponse`
  - `404` → `text/plain` string (unknown board)

### Schemas

`JobBoardResponse`: `apiVersion` (string) + `jobs` (array of `JobPosting`),
both required.

`JobPosting` — required per the official field table: `title`, `isRemote`,
`workplaceType`, `employmentType`, `publishedAt` (date-time), `jobUrl`,
`applyUrl`, `isListed`. Optional: `location`, `department`, `team`,
`descriptionHtml`, `descriptionPlain`, `address`, `secondaryLocations`,
`compensation`. Observed-only additions `id` and
`shouldDisplayCompensationOnJobPostings` are optional with descriptions
noting they are absent from the official docs.

Enum policy:

- `workplaceType` (`OnSite`, `Remote`, `Hybrid`) and `employmentType`
  (`FullTime`, `PartTime`, `Intern`, `Contract`, `Temporary`) are strict
  enums — the docs enumerate these sets exhaustively. Trade-off accepted:
  if Ashby ever adds a value, the generated client fails decoding and the
  spec must be updated.
- `compensationType` and `interval` stay plain strings with known values
  listed in the description (`Salary`, `EquityPercentage`, `Bonus`, … /
  `1 YEAR`, `NONE`, …) — the docs explicitly mark these lists as open
  ("and others"), so a strict enum would be a decode time bomb.

Compensation (full structure, present only with `includeCompensation=true`):

- `Compensation`: `compensationTierSummary`,
  `scrapeableCompensationSalarySummary` (both nullable — observed null on
  jobs that publish no compensation ranges, with empty tier/component
  arrays), `compensationTiers[]`, `summaryComponents[]`.
- `CompensationTier`: `id`, `title` (nullable — observed null on unnamed
  tiers), `tierSummary`, `additionalInformation` (nullable), `components[]`.
- `CompensationComponent` (shared by `components[]` and
  `summaryComponents[]`, where the latter omits `id`/`summary`):
  `compensationType`, `interval`, and nullable `currencyCode` (ISO 4217),
  `minValue`, `maxValue` — nullability as observed live, expressed with the
  OAS 3.0 `nullable: true` keyword: ogen v1.22 rejects 3.1-style
  `type: [string, "null"]` unions (precedent documented inline in
  `internal/provider/workday/openapi.yaml`).

Addresses: a shared `PostalAddress` schema (`addressLocality`,
`addressRegion`, `addressCountry`, `postalCode`, all optional strings —
observed as empty strings when unset) wrapped as `{postalAddress: {...}}`,
reused by `JobPosting.address` and `SecondaryLocation.address`;
`SecondaryLocation` adds the display `location` string.

## Validation

- Run ogen against the spec into the scratchpad (generated code not
  committed this round) to prove the file is codegen-clean.
- Cross-check the schema against a captured live response from the `ashby`
  board (with and without `includeCompensation`).

## Out of scope (follow-up rounds)

- `gen.go` + committed generated client, mock server, and tests.
- `companies.yaml` company → board-slug roster (Workday pattern, per
  discussion), with a direct-slug escape hatch.
- `ashby_search_jobs` / `ashby_get_job_detail` MCP tools; both are served
  by the single endpoint with client-side filtering, and payload size
  management (full descriptions for every job in one response) is a design
  point for that round.
