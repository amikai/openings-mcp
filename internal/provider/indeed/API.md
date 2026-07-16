# Indeed Mobile GraphQL API

Client-side API notes for Indeed's undocumented GraphQL endpoint, the same
one the official iOS/Android apps use. Reverse-engineered from the
python-jobspy reference implementation
(https://github.com/Bunsly/JobSpy, `jobspy/indeed/`) and confirmed live
while building this package.

- Base URL: `https://apis.indeed.com`
- Endpoint: `POST /graphql`

## Architecture

A single POST endpoint, one GraphQL query type covering both operations
this package needs:

- `jobSearch` — keyword + location + filter search, cursor-paginated.
  Callers choose which subfields to request; this client asks for a lean
  field set for summaries (no `description`) to keep search responses
  small.
- `jobData` — look up one or more jobs directly by opaque key (the same
  `key` field `jobSearch` results carry), requesting the full field set
  including `description.html`. This is the "detail" call; there is no
  separate REST-style `/jobs/{id}` path.

Both operations share the same URL, headers, and auth — only the
`query`/variables in the POST body differ — so unlike a typical multi-path
REST API this can't be modeled as an OpenAPI spec turned into an `ogen`
client (`ogen` models distinct paths per operation). The Go client is built
with Khan/genqlient against a reverse-engineered `schema.graphql` (Indeed
has no public SDL). Live-validated type names (via GraphQL validation
errors and the `cmd/indeed` CLI): `JobSearchSortOrder`,
`filters: [JobSearchFilterInput!]!`, `JobSearchLocationInput`,
`jobData(jobKeys: [ID!])`, `Salary.range` → `RangeType` (`Range` | `AtLeast` | `AtMost` | `Exactly`).

## Headers

Every request carries these:

| Header | Required | Notes |
| --- | --- | --- |
| `indeed-api-key` | yes | Static key, reused verbatim from python-jobspy's `constant.py`; long-lived and shared publicly across every jobspy installation, not a per-caller secret. Omitting it or sending an arbitrary value returns an auth error — not a value to invent yourself. |
| `indeed-co` | yes | Two-letter country catalogue selector (e.g. `TW`). Must match the country implied by `location`'s `where` text — see Key Behaviors. |
| `indeed-locale` | yes | Defaults to `en-US`. |
| `user-agent` | yes | Must resemble the official mobile app's UA string, e.g. `Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Indeed App 193.1`. A generic Go/library UA was not tested and jobspy's reference implementation always sends the app UA, so this client does too. |
| `indeed-app-info` | yes | e.g. `appv=193.1; appid=com.indeed.jobsearch; osv=16.6.1; os=ios; dtype=phone`. |

## Key Behaviors

- **`indeed-co` header selects the country catalogue.** It must match the
  country implied by `location`'s `where` text (e.g. `indeed-co: TW` when
  searching `"Taipei"`); sending an unrelated `indeed-co` still returns
  `HTTP 200` but with irrelevant or empty results, not an error — there is
  no server-side validation to catch a mismatch. See `country.go` for the
  name→(domain, indeed-co) table this client derives both values from
  together, so they can't drift apart independently.
- **GraphQL validation is real and strict.** Unknown fields/arguments
  return `HTTP 200` with a `GRAPHQL_VALIDATION_FAILED` error array and no
  `data` key — never a 4xx. A caller must check for a populated `errors`
  array before trusting an empty `data.jobSearch.results`.
- **No dedicated "not found" response.** Requesting `jobData` with a job
  key that doesn't exist (expired, removed, or never valid) returns
  `HTTP 200` with `data.jobData.results: []` — an empty list, not a 404 or
  an error entry.
- **`jobSearch` cannot filter by key.** There is no `jobKeys`/`jobKey`
  argument on `jobSearch` (confirmed live: it 400s as an unknown
  argument) — `jobData` is the only way to fetch a specific job, and it
  takes a list of keys in one call rather than one key per request.
- **Compensation is usually null.** Both `compensation.baseSalary` and
  `compensation.estimated` are commonly `null` on real postings; treat
  absence as "not disclosed", not as a parsing failure.
- **Pagination** is cursor-based, not offset-based: `pageInfo.nextCursor`
  from one response feeds back as the `cursor` argument on the next
  request; `null` means no more pages. `limit` caps out at 100 per call in
  the reference implementation.
- **Filters are mutually exclusive in the reference query shape**: the
  python-jobspy implementation this client mirrors sends at most one of a
  date filter (`dateOnIndeed`), an Easy-Apply keyword filter, or a
  composite job-type/remote keyword filter per request — never several
  combined — because the API's `filters` argument was only ever exercised
  that way building the reference implementation, not because combining
  them is known to fail.

## Response Shape

Always `HTTP 200`, whether the query succeeded, matched nothing, or failed
GraphQL validation — see Key Behaviors above.

- A successful `jobSearch` response carries `data.jobSearch.{pageInfo,results}`.
- A successful `jobData` response carries `data.jobData.results[].job`,
  possibly an empty list.
- A failed one carries an `errors` array (each with a `message`) and no
  `data.jobSearch`/`jobData` key.

## Example Queries

`jobSearch` — keyword + location search, lean fields:

```graphql
query GetJobData {
  jobSearch(what: "software engineer", location: {where: "Taipei", radius: 25, radiusUnit: MILES}, limit: 5, sort: RELEVANCE) {
    pageInfo { nextCursor }
    results {
      trackingKey
      job {
        key
        title
        datePublished
        location { countryCode admin1Code city formatted { long } }
        compensation { estimated { currencyCode baseSalary { unitOfWork range { ... on Range { min max } ... on AtLeast { min } ... on AtMost { max } ... on Exactly { value } } } } baseSalary { unitOfWork range { ... on Range { min max } ... on AtLeast { min } ... on AtMost { max } ... on Exactly { value } } } currencyCode }
        attributes { key label }
        employer { relativeCompanyPageUrl name }
      }
    }
  }
}
```

`jobData` — detail-by-key lookup, full fields:

```graphql
query GetJobDetail {
  jobData(jobKeys: ["9d503ca7fe211430"]) {
    results {
      job {
        key
        title
        description { html }
        location { countryName countryCode admin1Code city postalCode streetAddress formatted { short long } }
        employer { relativeCompanyPageUrl name dossier { employerDetails { addresses industry employeesLocalizedLabel revenueLocalizedLabel briefDescription } images { squareLogoUrl } links { corporateWebsite } } }
        recruit { viewJobUrl detailedSalary workSchedule }
      }
    }
  }
}
```
