// Package weworkremotely reads public job listings from We Work Remotely's
// RSS feeds at https://weworkremotely.com. WWR does publish a JSON API
// (weworkremotely.com/api), but it is write-only for employers — Create/
// Update/Show a job you posted, gated behind a partner token issued by WWR
// staff on request — with no listing or search route at all, so it cannot
// serve this package's read use case. The RSS feeds are the public,
// unauthenticated read surface; this package's contract was
// reverse-engineered from them directly (2026-07-19).
//
// # Feeds: one per category, no query-string search
//
// There is no single "all jobs" or "search" endpoint with real filtering.
// Instead each of the 10 fixed categories in [Categories] has its own feed
// at /categories/<slug>.rss, and that feed is a full, unfiltered dump of
// every live posting in that category — same shape as Remotive's single
// dump, just partitioned by category instead of merged into one endpoint.
// [Client.AllJobs] fetches and merges all 10; [Client.Search] fetches only
// the matching feed when a recognized category is given, which is a real
// request-count saving unlike a no-op query param.
//
// A combined https://weworkremotely.com/remote-jobs.rss exists but is
// NOT used: it caps at ~10 items per category (confirmed by counting
// items against the per-category feeds), so it is a "recently posted"
// digest, not a superset. Query parameters on any feed URL, including
// ?page=N, are ignored — confirmed by diffing two fetches that differed
// only in the ?page value; the response body was byte-identical.
//
// # Job identity and detail
//
// Each item's <link>/<guid> is the canonical posting URL,
// e.g. https://weworkremotely.com/remote-jobs/<slug>; [Job.ID] is that
// trailing slug. The individual posting page itself exists and embeds a
// schema.org JobPosting JSON-LD block, but it is deliberately NOT fetched
// for [Client.Detail]:
//
//   - It sits behind Cloudflare bot management: a bare HTTP client with a
//     minimal User-Agent gets a 403 challenge page on some request shapes
//     (observed on HEAD requests to feed URLs too), and reliably passing
//     it needs a full browser-like header set that is fragile to keep
//     working over time.
//   - Its JobPosting JSON-LD is not valid JSON — the description field
//     contains raw, unescaped control characters — so a standards-conforming
//     decoder (encoding/json included) rejects it outright.
//
// The RSS item already carries the complete HTML job description, so
// [Client.Detail] instead resolves the ID against a fresh [Client.AllJobs]
// fetch, the same "no detail endpoint, resolve from the dump" shape used
// by the Remotive provider.
//
// A single feed can list the same job twice under identical <link>/<guid>
// values (observed live in the Full-Stack Programming feed); [Client.Jobs]
// returns a feed as-is, duplicates included, but every other exported path
// ([Client.AllJobs], [Client.Search], [Client.Detail]) deduplicates by
// [Job.ID].
//
// # Fields
//
// <region>, <country>, and <state> are independent, mostly-empty location
// hints (e.g. region "Anywhere in the World" with country/state blank, or
// region "Massachusetts" with country/state blank, or country "🇺🇸 United
// States of America" with state "Georgia") — there is no single reliable
// location field, so [FilterOptions.Region] matches across all three.
// <skills> is a free-text, comma-joined tag list, often empty. <type> was
// observed as only "Full-Time" or "Contract" across a live sample, but is
// treated as an open string, not a closed enum.
//
// # Verified but deliberately unsupported
//
//   - Keyword search via the site's own /remote-jobs/search HTML page:
//     works interactively, but returns rendered HTML, not RSS, and would
//     need its own scraper. [FilterOptions.Keyword] covers this client-side
//     over the dump instead.
package weworkremotely
