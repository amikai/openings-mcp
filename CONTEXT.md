# openings-mcp

An MCP server that answers job-listing questions across many hiring
platforms, so a client names a company and never learns which system serves
it.

## Language

### Platforms and code

**Provider**:
One upstream job-listing source we integrate, and the package under
`internal/provider/` that talks to it.
_Avoid_: source, site, vendor, integration

**Adapter**:
A provider's implementation of `ats.Adapter`, which is what lets its
companies answer the unified company-parameterized tools. Not every provider
has one — a single-site provider gets its own dedicated tools instead.
_Avoid_: driver, connector, backend

**Unified company tools**:
`search_jobs_by_company`, `get_filters_by_company`, and
`get_job_detail_by_company` — the tools that take a company and dispatch to
whichever adapter owns it.
_Avoid_: generic tools, shared tools

**Dedicated tools**:
A single provider's own `<name>_search_jobs` / `<name>_get_job_detail`
tools, for sources that serve one site rather than many companies.
_Avoid_: custom tools, per-provider tools

### Companies

**Company**:
An employer as a client names it. The unit a caller asks about.
_Avoid_: employer, organization, client, account

**Tenant**:
One company's installation on a multi-company platform, as that platform
identifies it. A company is who the caller means; a tenant is how the
upstream addresses them.
_Avoid_: instance, account, org

**Slug**:
A company's unique key within one provider's roster, and the value adapter
methods take. Its shape is the provider's business — a bare tenant name, a
composite of tenant and site, or a canonical careers URL for a company the
roster does not list.
_Avoid_: id, key, handle

**Roster**:
A provider's curated list of companies, in its `companies.yaml`. Entries are
live-verified before they are added.
_Avoid_: list, catalog, directory, registry

**Unverified**:
A bulk-discovered candidate company under `unverified/`, not yet confirmed
against the live API and not yet reachable through the server. It becomes a
roster entry only after `cmd/verify-companies` passes it.
_Avoid_: pending, staged, draft

**Seed roster**:
The handful of companies added when a provider is first built, enough to
prove the pipeline end to end. Distinct from the bulk expansion that follows
later.

**Careers URL**:
A company's public job-listing page on its platform. It is also an accepted
input wherever a company is: it lets a caller reach a company the roster does
not list, and it disambiguates two rosters that use the same name.
_Avoid_: job board URL, careers page link

### Search

**Server-side search**:
A provider whose upstream applies the query, filters, and pagination itself.
_Avoid_: remote search, native search

**Full dump**:
A provider whose upstream returns its whole board in one response, so
matching happens in our code. A provider that merely paginates is not a full
dump — paging it ourselves to synthesize one is a choice, not a description.
_Avoid_: bulk fetch, crawl, scrape

**Filter dimension**:
One axis a company's board can be narrowed by, with the values it accepts.
Dimensions are per-company and discovered at runtime, because platforms let
each tenant configure their own.
_Avoid_: facet, category, attribute, filter type

**Soft filter**:
An upstream filter that is accepted and then not fully applied, so results
leak rows that do not match. We report what the upstream returned and label
the leakage; we do not silently drop rows to make the response look clean.
_Avoid_: fuzzy filter, loose filter

### Fixtures

**Fixture**:
A captured real request and response pair under a provider's `testdata/`,
replayed by its mock server in tests and re-run live by `make hurl-test`.
Fixtures are always captures, never hand-written bodies.
_Avoid_: mock, stub, sample, golden file
