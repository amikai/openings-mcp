# Teamtailor Unified ATS Adapter — Design

## Surface

Teamtailor is a multi-company ATS, so it implements `internal/ats.Adapter` and
joins `search_jobs_by_company`, `get_filters_by_company`, and
`get_job_detail_by_company`. It does not add dedicated MCP tools.

## Addressing and URL parsing

The adapter slug is the lowercase career-site hostname. Roster hosts resolve by
name or host. `ParseCareersURL` also accepts unlisted hosted sites whose hostname
ends in `.teamtailor.com`, allowing a user to pass a public `/jobs` or posting
URL directly. Reserved product hosts such as `www`, `app`, `api`, and `docs` are
rejected. A custom-domain site is recognized when its host is curated in
`companies.yaml`; arbitrary custom domains cannot be identified as Teamtailor
from URL shape alone without a network probe, which the synchronous parser does
not perform.

## Full-dump mapping

`Search`, `Filters`, and `Detail` fetch `/jobs.json`. The adapter maps each feed
item to the shared `dumpJob` engine:

- JSON Feed item UUID -> `JobID`;
- `title`, public `url`, and ISO date -> summary fields;
- `content_html` converted to plain text -> query tier 3 and detail body;
- Schema.org postal addresses -> semicolon-joined display/fuzzy location;
- distinct `city`, `region`, and `country` address values -> structured filters;
- `date_published` -> deterministic newest-first sort key.

The feed does not expose Teamtailor's department, role, or division fields, so
the adapter does not invent them.

Detail refetches the dump and selects the exact item ID. The feed title supplies
the company display name even for non-roster hosted sites. Missing items and
upstream 404s return teaching errors naming the host and required retry input.

## Wiring

- Register `NewTeamtailorAdapter` in `cmd/openings-mcp/newATSRegistry`.
- Add the URL shape to `careersHostPatternsByAdapter`.
- Add `teamtailor` to `cmd/verify-companies/providerOrder` and adapter building.
- Update README's unified provider list.

## Tests

Fixture-backed tests cover roster mapping, URL parsing (EU/NA/AU, reserved and
custom hosts), query/location/filter search, filters, detail, unknown host/item,
deterministic ordering, and registry collision/startup behavior.
