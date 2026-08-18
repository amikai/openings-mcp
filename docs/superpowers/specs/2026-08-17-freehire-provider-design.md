# freehire.me provider — design

Dedicated job-board tools, not an `ats.Adapter`.

freehire is an ingest-time catalogue (Meilisearch over ~3.3M IT postings)
with a keyless JSON API. Company search is a facet (`company_slug`), not
a tenant. Stripe on freehire is a crawled Greenhouse subset of a company
already on the Greenhouse roster, so putting it behind
`search_jobs_by_company` would collide and return an incomplete board.

MCP surface: `freehire_search_jobs` + `freehire_get_job_facets` +
`freehire_search_companies` + `freehire_get_job_detail`.

Every filter needs somewhere to get a legal value from. `freehire_get_job_facets`
covers `skills`, `category`, `countries`, and `source`; the schema enums the
rest. `company` is the exception — the facets response carries no company
facet under any filter combination, and `company_slug` matches exactly, so
`freehire_search_companies` wraps `GET /companies` to resolve a name. Matching
there is fuzzy despite the spec calling it a substring (`strpie` finds Stripe),
and a name can match several companies (`adria-solutions` and
`adria-solutions-ltd` are different employers), so it returns a list with
`job_count` as the tiebreaker. `GET /companies/{slug}` stays unwired: it
duplicates `freehire_search_jobs` with `company` set.
`company` is an optional filter next to `query`, `skills`, `seniority`,
`work_mode`, `regions`, `country`, `source`, and `category`. Filter
fields that the official spec enums (`seniority`, `work_mode`,
`regions`, `sort`, `order`) stay enums on both the OpenAPI client and
the MCP tool. `visa_sponsorship` is a facet, not a query parameter, so
it is not a filter.

Client: official `openapi.yaml` from https://freehire.me/openapi.yaml,
unmodified. Search uses `GET /agent/jobs/search`; detail uses
`GET /jobs/{slug}`. Search ships every row's whole posting whatever
`description_format` asks for, so MCP search truncates it to an opening
and leaves the rest to detail. There is no company facet: `company_slug`
rides on each search row.

No roster. Server-side search shape.
