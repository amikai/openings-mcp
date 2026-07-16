# Oracle Recruiting Cloud Candidate Experience OpenAPI — Design

## Context

Oracle Recruiting Cloud (ORC) is a multi-company ATS. Its public external
career sites use URLs shaped like:

```text
https://<fusion-host>/hcmUI/CandidateExperience/<lang>/sites/<site>/jobs
```

The `<site>` path segment may be the internal site number (`CX_1001`) or a
configured URL name (`Mayo-US`). The career-site HTML identifies the API
origin and internal site number through the `<base>` element's
`data-apibaseurl` and `data-sitenumber` attributes. Older themes omit those
attributes; for them, the API origin is the page origin and the path segment
is the site number.

Oracle publishes an official OpenAPI 3.0 document for Fusion Cloud HCM at
<https://docs.oracle.com/en/cloud/saas/human-resources/farws/openapi.json>.
It includes the Candidate Experience resources, although their documentation
marks them as intended for Oracle internal use. Live public career sites call
the same resources anonymously, so this provider trims the official document
to the two operations needed for public job search and detail, then corrects
the model against captured traffic.

## Verified wire behavior (2026-07-17)

`GET /hcmRestApi/resources/latest/recruitingCEJobRequisitions`:

- requires no authentication, cookies, or CSRF token;
- uses `finder=findReqs;...` to carry the site number, pagination, keyword,
  and selected facet IDs;
- returns one search-state object in `items`, containing `TotalJobsCount`,
  `requisitionList`, and requested facet arrays;
- accepts an absolute result `offset` and caller-selected `limit`;
- expects keyword text quoted inside the finder value, with embedded quotes
  escaped;
- accepts semicolon-separated facet IDs in `selected*Facet` finder variables;
- returns facet IDs as integers for locations/categories/organizations/work
  locations/posting dates, and strings for title/workplace-type facets;
- accepts Oracle ADF `fields` projections as an alternative to `expand`,
  including semicolon-separated child-resource clauses;
- treats `facetsList=NONE` as a compact result request with empty facet arrays;
- does **not** validate `siteNumber`: an unknown number can fall back to the
  host's default career site, so tenant validation must happen from the public
  career-site URL before API calls.

`GET /hcmRestApi/resources/latest/recruitingCEJobRequisitionDetails`:

- uses `finder=ById;Id="<job-id>",siteNumber=<site-number>`;
- returns one expanded detail object in `items` for a current posting;
- returns HTTP 200 with `items: []` for an unknown or expired job ID;
- accepts non-numeric public IDs (observed values include numeric IDs,
  hyphenated IDs, and prefixed alphanumeric IDs);
- carries HTML description sections in `ExternalDescriptionStr`,
  `CorporateDescriptionStr`, `ExternalResponsibilitiesStr`, and
  `ExternalQualificationsStr`.

Live verification covered Mayo Clinic, JPMorgan Chase, KPMG India, KPMG
Global Services, University of Chicago Medicine, the International
Organization for Migration, and Gosport Borough Council across
`*.fa.ocs.oraclecloud.com`, `*.fa.oraclecloud.com`, and regional
`*.fa.<region>.oraclecloud.com` host shapes.

## Spec shape

`internal/provider/oracle/openapi.yaml` is a minimal OpenAPI 3.1 trim of
Oracle's official document:

```text
GET /hcmRestApi/resources/latest/recruitingCEJobRequisitions
  -> SearchCollection

GET /hcmRestApi/resources/latest/recruitingCEJobRequisitionDetails
  -> DetailCollection
```

Both operations expose only the query parameters the public career site uses:
`onlyData`, `expand`, `fields`, and `finder`, plus the two locale headers.
`expand` and `fields` are alternatives; the captured fixtures use projections
to stay compact. Finder construction remains provider-layer logic because
Oracle encodes a structured argument list inside one query-string value.

The schemas retain only fields consumed by the future provider and ATS
adapter:

- search totals, offset/limit, compact posting fields, and standard facets;
- detail identity, posting date, location, workplace type, and public HTML
  description sections;
- secondary-location display names.

Nullability follows the official Oracle schema and was checked against live
responses. Unknown Oracle fields are intentionally ignored by the generated
decoder.

## Family

**Server-side search** with exact totals, absolute-offset pagination,
server-side keyword search, and tenant-specific standard facets.

## Validation

- Capture unfiltered, keyword-filtered, facet-bearing, detail, and missing
  detail responses as hurl fixtures.
- Generate an ogen client and compile it.
- Replay the fixtures through `NewMockServer` in generated-client tests.
- Add the spec to `OPENAPI_SPECS` and run OpenAPI validation.

## Out of scope

Candidate accounts, applications, internal candidate experience, talent
community signup, events, AI recommendations, map endpoints, requisition
administration APIs, and the career-site HTML discovery/parser belong to later
provider stages.
