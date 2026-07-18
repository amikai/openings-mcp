# JOIN Public API

Client-side API notes for join.com, an SMB-focused ATS/career-page SaaS
(the same platform join.com itself runs on). Reverse-engineered from the
public `job-ad-app` Next.js bundle and confirmed live while building this
package.

## Architecture

Two independent, unauthenticated surfaces, neither of them documented:

- **Search — GraphQL.** `POST https://join.com/candidate-api/graphql`,
  operation `publicJobs(input: PublicJobsQueryInput!)`. This is the exact
  query the public `/companies/{slug}` career page's frontend calls
  (extracted from its webpack chunk, `query PublicJobsList`). No
  authentication, no API key.
- **Detail — SSR HTML scrape.** `GET https://join.com/companies/{slug}/{idParam}`.
  `publicJobs` never populates `descriptionHtml` — confirmed null on every
  live query tried, list or single-item, regardless of requested fields —
  so the only public source for a job's full description is the
  server-rendered job page's embedded `__NEXT_DATA__` JSON blob (the same
  data Next.js hydrates the page from). See parse.go.

There is no public GraphQL field to resolve a company slug to its numeric
`companyId` (`companySlug`/`companyDomain`/`publicCompany`/`companyByDomain`
all confirmed absent via "Did you mean" validation errors), and no
companies-directory or search field either. `companyId` must be read out of
the same `__NEXT_DATA__` blob on the company's `/companies/{slug}` page.
This client resolves it once per roster entry during roster curation
(`companies.yaml` stores the resolved id), not on every request.

## Why dump-style, not server-side search

`PublicJobsQueryInput` has a `filters` argument (`PublicJobsFilters`, with
`categoryIds`/`placeIds`/`excludedIds` confirmed via the frontend's search
UI code) but no keyword/text field — probed live with `search`, `keyword`,
`query`, `text`, `title`, none exist. There's no way to ask JOIN's API for
"jobs matching this text," so this client always fetches the whole board
for a company (paginating if `pageInfo.pageCount > 1`) and lets
`internal/ats.searchDump` do query/location/filter matching client-side —
the same shape as the Greenhouse/Lever/Ashby adapters. Unlike those,
though, the dump this client builds carries **no job description** (see
above), so `searchDump`'s full-text tier only ever matches title and
category text, not body copy — a real, not theoretical, gap given
`descriptionHtml` is unconditionally null.

## Job identity

Search results carry two different job identifiers:

- `id` (Int) — the canonical database id.
- `idParam` (String) — a different-looking number-prefixed slug, e.g.
  `id: 16312205` but `idParam: "16425272-vp-revenue-m-f-d"`. The leading
  number in `idParam` is **not** `id`; it's some other internal value.
  Confirmed live: `GET /companies/{slug}/{id}` (bare numeric `id`) 301s to
  `/companies/{slug}/{id}-{slugified-title}` — a *different* URL than
  `idParam` — while `GET /companies/{slug}/{idParam}` (the value search
  results actually carry) 200s directly with no redirect.

This client uses `idParam` as the job identifier end to end (the
`JobSummary.JobID` field), since it's what search already returns and it
resolves directly without a redirect hop.

## Key Behaviors

- **No explicit not-found.** An unknown or invalid `companyId` returns
  `HTTP 200` with `publicJobs.items: []` — identical to a real company
  with zero open jobs. There's no way to distinguish "bad id" from "no
  jobs" from this call alone; curated-roster ids are trusted, not
  re-validated per request.
- **`descriptionHtml` is unconditionally null**, confirmed on both
  `publicJobs` (list) and the singular `publicJob(input: {id: ...})`
  query, across every job tried. Whatever populates it server-side (a
  "unified description" editor migration, per the frontend's
  `unifiedDescription` flag) isn't exposed over the public API.
- **Job detail has two description shapes**, both scraped off the SSR
  page (`initialState.job` in `__NEXT_DATA__`), gated by the job's own
  `unifiedDescription` boolean — both branches confirmed live:
  - `false` (legacy): the body is split across `intro`, `tasks`,
    `requirements`, `benefits` (rendered only if non-empty), and `outro`
    (rendered only if non-empty) — each a Markdown string, not HTML.
  - `true`: the whole body is one Markdown string in `description`.
- **`remoteType` is also present on the job detail page**, not just the
  search list — confirmed live on a `workplaceType: REMOTE` job
  (`remoteType: "ANYWHERE"`). Values observed across the roster:
  `ANYWHERE` (no location restriction) and `COUNTRY` (remote within the
  job's `country` only); `null` for non-remote jobs. A `REMOTE` job's
  `city`/`country` still carry the employer's base location even when
  `remoteType` is `ANYWHERE` — a caller that displays only `city` for
  such a job would misrepresent it as on-site there.
- **Job detail 404s cleanly** (`HTTP 404`) for a nonexistent `idParam` or
  a nonexistent company slug — unlike search's no-not-found behavior.
- **Pagination** is page-based (`page`, `pageSize`), 1-indexed;
  `pageInfo.pageCount` tells the caller when to stop. Tested at
  `pageSize: 200` against a real board with no error or truncation — JOIN
  imposes no observed per-request cap, but this client still loops pages
  rather than assuming one call always suffices.

## Response Shape

Always `HTTP 200` for `candidate-api/graphql`, whether the query matched
zero, some, or (on real GraphQL validation failures) no rows at all — a
successful `publicJobs` call always carries `data.publicJobs.{items,pageInfo}`.
Malformed queries return `HTTP 400` with an `errors` array and no `data`
key (this client's own hand-maintained query is fixed and pre-validated,
so a 400 in production would mean this schema has drifted from the live
API, not a bad caller input).

## Example Queries

Search — full board dump for one company, paginated:

```graphql
query GetCompanyJobs {
  publicJobs(input: {companyId: 172617, paginationPage: {page: 1, pageSize: 100}}) {
    items {
      id
      idParam
      title
      status
      workplaceType
      remoteType
      createdAt
      updatedAt
      city { cityName countryName }
      country { iso3166 name }
      category { id name }
      employmentType { id name }
    }
    pageInfo { page pageCount pageSize rowCount }
  }
}
```

Detail — not a GraphQL call. `GET https://join.com/companies/routinelabs/16397229-senior-software-engineer-backend-llm-infrastructure`
returns the full SSR HTML page; `parse.go` extracts `__NEXT_DATA__` and
reads `props.pageProps.initialState.job`.
