# SmartRecruiters ATS Adapter Design

2026-07-13

## Goal

Wire the existing SmartRecruiters provider package
(`internal/provider/smartrecruiters`) into the unified company tools:
implement `internal/ats.Adapter` and register it with the MCP server, so
the 52 rostered companies (and any `jobs.smartrecruiters.com` careers URL)
join `search_jobs_by_company`, `get_filters_by_company`, and
`get_job_detail_by_company`.

This is pipeline stage 6 of the integrate-new-provider skill. Stages 1–5
(spec, fixtures, generated client, roster, debug CLI) are already landed;
this design changes nothing in the provider package.

## Scope

One PR:

- `internal/ats/smartrecruiters.go` — the adapter — plus
  `internal/ats/smartrecruiters_test.go`.
- Register in `newATSRegistry` (`cmd/openings-mcp/main.go`).
- Add `"smartrecruiters": "jobs.smartrecruiters.com/<company>"` to
  `careersHostPatternsByAdapter` (`internal/ats/registry.go`).
- Add `smartrecruiters` to `providerOrder` and `buildAdapters`
  (`cmd/verify-companies/main.go`).
- Add SmartRecruiters to the README provider list.

## Adapter shape

`SmartRecruitersAdapter`, constructed as
`NewSmartRecruitersAdapter(baseURL string, hc *http.Client)` with
production base `https://api.smartrecruiters.com` — the Lever/Ashby style,
because the API host is fixed (unlike Workday's per-tenant bases).
`Name()` returns `"smartrecruiters"`.

## Slug model

The slug is the lowercased `CompanyIdentifier`; the API accepts it
case-insensitively as its `companyIdentifier` path parameter, so the slug
alone fully addresses a company.

- `Roster()` maps `smartrecruiters.Companies` to
  `CompanyInfo{Slug: lower(CompanyIdentifier), Name}`.
- `ParseCareersURL` recognizes `jobs.smartrecruiters.com/<identifier>[/...]`
  and returns the lowercased first path segment — for roster and
  non-roster companies alike. No canonical-URL slug form is needed
  (Workday needs one because its config is three values; here one value
  suffices).
- Unknown identifiers cannot be validated: the list endpoint returns
  HTTP 200 with `totalFound: 0` for them (see the openapi.yaml no-404
  note), so a typo'd careers URL degrades to an empty search result, not
  an "unknown company" error. Accepted trade-off — it mirrors the raw API.
- `JobDetail.Company` prefers the detail response's own `company.name`,
  falling back to the roster name, then the slug.

## Search

- With no `Query` or `Location`, paging stays entirely server-side:
  `limit=PageSize` (20), `offset=(page-1)*PageSize`, with the same
  overflow guard as Workday.
- SmartRecruiters `q` ORs terms and searches both titles and location
  text. It is therefore used only as a candidate selector, never as the
  final unified match:
  - Query-only search requests `q=Query`, then removes postings whose
    title does not contain every query word.
  - Location-only search requests `q=Location`, then removes postings
    whose `location.fullLocation` does not contain the requested text.
  - When both are present, the adapter requests the first 100 results for
    each value separately, keeps the smaller candidate set, and applies
    Query AND Location locally.
  - Candidate collection is capped at 2,000 postings. A broader search
    returns a teaching error asking for a narrower query, location, or
    structured filter rather than silently truncating totals.
- Text-search totals and pages are computed after local matching via the
  shared `searchDump` engine. Candidate pages use the API maximum
  `limit=100` to minimize upstream calls.
- Filters (keys as `Filters()` reports them):
  - `department` — labels resolved to ids via one `listDepartments` call
    when the filter is set (stateless probe, Workday-style). Labels match
    case-insensitively after trimming; an unknown label errors listing the
    valid labels (truncated past 20, Workday-style). Duplicate labels keep
    every underlying id. Multiple resolved ids join comma-separated into
    the single `department` query param — verified live against Equinox to
    OR (129 + 23 = 152 postings).
  - `location_type` — values match Remote/Hybrid/Onsite
    case-insensitively and map to the `locationType` enum
    (REMOTE/HYBRID/ONSITE, passed as an array). An unknown value errors
    listing the three.
  - Any other filter key errors naming the two valid keys.
- Summary mapping: `JobID` = posting `id`, `Title` = `name`,
  `Location` = `location.fullLocation`, `PostedAt` =
  `isoDate(releasedDate)`, `URL` =
  `https://jobs.smartrecruiters.com/<CompanyIdentifier>/<id>` (list items
  carry no `postingUrl`; slug-less public URLs verified to return 200).
  Postings missing an `id` are skipped rather than emitted with an
  un-detailable `job_id`.
- With no text constraints, `TotalCount` = upstream `totalFound`. With
  Query or Location, `TotalCount` and `TotalPages` reflect the locally
  matched set.

## Filters()

One `listDepartments` call:

- `department`: labels of non-archived, non-empty-label departments.
  (Companies that don't use departments return empty or unlabeled
  entries; they get no `department` dimension.) Some listed departments
  legitimately have zero live postings; that is fine — filtering by them
  returns an empty page. Labels are trimmed and deduplicated
  case-insensitively; Search resolves one display label to every matching
  department id.
- `location_type`: static `[Hybrid, Onsite, Remote]`.

## Detail

`getPosting(identifier, jobID)`.

- A 404 (`RESOURCE_NOT_FOUND`) maps to an error telling the caller to
  pass a `job_id` exactly as returned by the job search.
- `Description`: the jobAd sections — companyDescription, jobDescription,
  qualifications, additionalInformation — each converted with html2text;
  non-empty sections joined with their section titles as headings.
- `Location` = `fullLocation`, `PostedAt` = `isoDate(releasedDate)`,
  `URL` = `postingUrl`.

## Error handling

- All upstream errors wrap with a `smartrecruiters:` prefix and the slug,
  matching the other adapters.
- Filter-resolution failures are teaching errors that name the valid
  alternatives (labels, location types, or filter keys).
- An unfiltered Search has no unknown-slug error path: the postings list
  endpoint returns an empty 200 response. `Filters()` and a
  department-filtered Search call the departments endpoint, which may
  surface a 404 for an unknown company. Detail's 404 still surfaces for
  bad job ids.

## Testing

`internal/ats/smartrecruiters_test.go` drives the adapter against the
provider's `mocksrv.go` (fixture-replaying), like the other adapter
tests:

- Search happy path: mapping of id/title/fullLocation/releasedDate,
  derived public URL, totals and page math.
- Residual text matching: Query excludes location-only hits, Location
  excludes title-only hits, and Query + Location use AND semantics even
  though upstream `q` uses OR semantics.
- Candidate collection: smaller-set selection, multi-page collection,
  post-filter totals/pages, and the 2,000-candidate bound.
- Filter resolution: department label → id (single and multiple,
  comma-joined), location_type mapping, teaching errors for unknown
  label, unknown location_type, and unknown filter key.
- `Filters()`: department labels exclude archived/empty entries, fold
  case and whitespace, preserve all ids behind duplicate labels, and
  include the static location_type dimension.
- `ParseCareersURL`: roster identifier, non-roster identifier, non-SR
  host, bare host without path.
- Detail: happy path (sections joined, postingUrl, company name from
  response) and 404 → teaching error.
- Registry integration: registration in `ats.NewRegistry` succeeds (the
  existing collision checks cover slug/name clashes) and the careers-URL
  path resolves through the registry.
