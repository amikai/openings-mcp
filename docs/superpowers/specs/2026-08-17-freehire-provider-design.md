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

## Update 2026-08-18 — upstream openapi.yaml v1.1.0

Everything above describes the first implementation, against spec v1.0.0.
Upstream published v1.1.0 (585 → 1650 lines), which reverses four of the
decisions recorded above. The originals are kept as the record of why the
first shape was chosen; where they conflict with this section, this
section is what shipped.

**Filter exposure.** v1.1.0 grows the job filter set from 15 to 41 and the
company filter set from 1 to 16. All of them are exposed. The guiding rule
for this provider is now: mirror upstream, and do not go beyond it. Its
prose promises that every string facet supports `<facet>_exclude` and
`<facet>_mode=and`, and testing confirmed the undeclared ones work
(`seniority_exclude` returned exactly the complement of `seniority`), but
only the 6 `_exclude` parameters and `skills_mode` that the schema
*declares* are exposed — using the rest would mean editing
`openapi.yaml`, and "used unmodified" is worth more than the marginal
reach. The same rule retired this repo's own conventions here: `limit`
/`offset` replace the 1-based `page`, output echoes upstream's
`meta` instead of a computed `last_page`, and the invented `max_values`
knob is gone.

**Search endpoint.** ~~`GET /agent/jobs/search`, truncated to an
opening.~~ Now `GET /jobs/search`. The two take an identical query
surface, but the agent path ships 140 KB against 59 KB at `limit=20` and
does **not** attach `ghost`. Truncation is gone too: upstream already caps
the preview near 1000 characters, so the tool only converts it out of HTML.
`agentSearchJobs` stays generated but uncalled.

**`GET /companies/{slug}`.** ~~Unwired: duplicates `freehire_search_jobs`
with `company` set.~~ Now `freehire_get_company_detail`. That original
reasoning only ever covered its `jobs` half, which is still true — that
half takes no filters and no sort. What changed is that v1.1.0 added
company filters for `industries`, `maturity`, and
`yc_batch`/`yc_status`/`yc_stage`/`yc_flags`, and no facets endpoint
covers those vocabularies. `CompanyDetail` is the only place their real
values can be read, which is not academic: `openapi.yaml` documents
`yc_batch` as `W21`, but the API holds `Summer 2009` and the documented
form returns 0 rows. `freehire_search_cities` exists for the same reason —
the `cities` facet holds display names and carries ~1200 values.

**`visa_sponsorship`.** ~~A facet, not a query parameter, so not a
filter.~~ v1.1.0 declares it as a query parameter, and it is exposed. It
is tri-state on purpose: `false` is a value the posting stated, so an
unset filter must send nothing rather than `false`.

**The failure mode that shaped the rest.** An unrecognized filter *value*
matches nothing and is not an error. An unrecognized parameter *name* is
dropped and the entire catalogue comes back, reported only in
`meta.ignored_params`. The tools turn that report into an error instead of
returning the rows, and MCP parameter names match upstream
character-for-character so the report can only mean this build has drifted
from the spec. Geography is the caller-facing counterpart: `regions`,
`countries` and `cities` form one OR-group, so a second geography widens
the search. That is documented on all three parameters and deliberately
not policed in Go — upstream calls `?regions=eu&countries=br` a useful
query.

`getSimilarJobs` remains unwired; it has nothing to do with filtering.
