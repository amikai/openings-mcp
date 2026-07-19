# NoDesk data surface

Reverse-engineered 2026-07-19. NoDesk (https://nodesk.co) is a static
(Hugo-style) site; there is no first-party JSON REST API. The search box
is Algolia InstantSearch, and that index is the machine-readable surface.

## Recovering the Algolia credentials

The listings page loads `/js/algoliasearch.min.js`, `/js/instantsearch.min.js`,
and the site's own `/js/search.min.js`. The last one contains:

```js
algoliasearch("0586L1SOK8","8dacb58c6f375cba28e19ecf1f03e9e1")
indexName:"jobPosts"
initIndex("geographicRegions")
```

- Application ID: `0586L1SOK8`
- Search-only API key: `8dacb58c6f375cba28e19ecf1f03e9e1` (public by
  design — every visitor's browser downloads it)
- Job index: `jobPosts`; a second index `geographicRegions` only backs the
  region autocomplete widget and is not used here.

If a request starts failing with 403 `Invalid Application-ID or API key`,
re-extract both values from the current `search.min.js`.

## Query endpoint

```
POST https://0586L1SOK8-dsn.algolia.net/1/indexes/jobPosts/query
X-Algolia-Application-Id: 0586L1SOK8
X-Algolia-API-Key: 8dacb58c6f375cba28e19ecf1f03e9e1
Referer: https://nodesk.co/

{"params": "query=golang&hitsPerPage=20&page=0"}
```

The key is **referer-locked**: without a nodesk.co `Referer` header the
response is 403 `Method not allowed with this referer`
(`testdata/search_missing_referer_req.hurl` captures this).

`params` is a URL-encoded query string inside the JSON body — standard
Algolia. Relevant parameters:

- `query` — full-text search; empty string matches everything.
- `page` — zero-based.
- `hitsPerPage` — values above 100 are clamped to 100 by the index
  (observed: requesting 186 returns 100 with `nbPages: 2`).
- `facetFilters` — URL-encoded JSON array, e.g.
  `["searchFilter:remote-jobs/engineering","applicantLocationRegions:Remote - Europe"]`.
  Top-level entries AND; nest an inner array for OR.
- `facets` — e.g. `["searchFilter","applicantLocationRegions"]` with
  `hitsPerPage=0` to enumerate all facet values and counts.

## Facets

- `searchFilter`: the site's category paths — role
  (`remote-jobs/engineering`), employment type (`remote-jobs/full-time`),
  technology (`remote-jobs/golang`), region shorthand (`remote-jobs/us`),
  and the catch-all `remote-jobs`. One job carries several.
- `applicantLocationRegions`: display labels (`Remote - Europe`,
  `Worldwide`, …).

## Hit record

Job fields: `objectID`, `title`, `company{name,url}`, `permalink`
(`/remote-jobs/<slug>/` — the durable job identity), `role{name,url}`,
`employmentTypes[]`, `keywords[]`, `applicantLocations[]`,
`applicantLocationRegions[]`, `baseSalary` (display string like
`"$150K – $250K"` or literal `false`), `date` (display label: `Featured`,
`Today`, `1d`, …), `datePublished` (zone-less ISO timestamp), `logo`,
`highlight` (featured flag).

Quirks:

- Fields the index publishes as `false` instead of null: `baseSalary`,
  each `keywords[].url`.
- Search-UI internals with no extra job data: `weight`, `index`,
  `searchFilter[]` (duplicated by the facets), `_highlightResult`,
  `{name,url,comma}` sub-shapes' `url`/`comma`.
- One injected advertisement record (`is_ad: true`) with `ctaUrl`,
  `ctaLabel`, `badges[]`, `verified`, an external `permalink`, and none of
  the job fields. The client drops it.

## Job detail page

`GET https://nodesk.co/remote-jobs/<slug>/` (any User-Agent works; no
Cloudflare challenge observed). The page embeds:

- One `<script type="application/ld+json">` block: a schema.org
  `JobPosting` with the full HTML `description`, `datePosted`,
  `validThrough`, `employmentType[]` (schema codes like `FULL_TIME`),
  `hiringOrganization{name,sameAs[],logo}`, `jobLocationType`
  (`TELECOMMUTE`), `applicantLocationRequirements`, and `baseSalary`
  (MonetaryAmount) when listed. Valid JSON (unlike e.g. We Work
  Remotely's equivalent block).
- The outbound apply link in `data-apply-url` on the apply buttons
  (`a.js-apply-btn`), pointing at the employer's own application page.

Caveat: `applicantLocationRequirements` is boilerplate
`{"@type":"Country","name":"Anywhere"}` even for postings whose index
record restricts to one region (verified against a `Remote - US`-only
posting). The hit's `applicantLocationRegions` is the reliable signal.

Unknown/expired slugs return a clean 404 HTML page.

## Rejected surfaces

- `https://nodesk.co/api/jobs/` — 404; an Apify/ever-jobs reference
  scraper invents this endpoint.
- `https://nodesk.co/remote-jobs/index.xml` — real RSS, but only the ~10
  newest postings.
