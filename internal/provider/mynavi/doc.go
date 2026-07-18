// Package mynavi reads public Mynavi Tenshoku (マイナビ転職) job listings and
// posting details from https://tenshoku.mynavi.jp. Mynavi publishes no
// official API; this surface was reverse-engineered from live traffic
// (2026-07-18). Both endpoints are server-side rendered HTML with no
// auth, cookie, or fingerprint gate; robots.txt allows both paths.
//
// # Search: the /list/ path-token DSL
//
// Filters are URL path segments, not query parameters, combinable in any
// order under /list/ (the client emits min/kw/pg to match the site's own
// URLs):
//
//   - kw{keywords} — free text; space-separated terms AND together,
//     Japanese and Latin terms both work. A literal "/" cannot be
//     expressed: its escaped form is HTTP 404 upstream, so the client
//     rejects it with a clearer error first.
//   - min{NNNN} — first-year-income (初年度年収) floor in units of 10,000
//     JPY, zero-padded to 4 digits. Only the fixed steps in [MinSalaries]
//     are valid; anything else is HTTP 404 upstream (also rejected
//     client-side).
//   - pg{N} — 1-based page. Page 1 is expressed by omitting the segment.
//     A page past the last one is HTTP 200 with zero cassettes, not an
//     error.
//
// A full page carries 50 job cassettes; the total match count is rendered
// on the page. A zero-hit search is HTTP 200 with count 0 and no
// cassettes (no recommend/PR cards are injected in their place).
//
// Unknown /list/ segments are a clean HTTP 404 — with one trap: an
// unknown path prefix (e.g. /tokyo/list/...) is HTTP 200 but silently
// serves an unfiltered legacy Shift_JIS page with different markup and
// all ~55k jobs. The parser guards against ever reading such a page as
// results by erroring when the result counter is missing.
//
// # Detail: /jobinfo-{a}-{b}-{c}-{d}/
//
// The four hyphen-separated numbers from a search result's [Job.ID] are
// the canonical posting ID; treat the whole string as opaque. The page
// embeds a complete schema.org JobPosting JSON-LD block (alongside a
// BreadcrumbList one — parsing selects by @type), so detail extraction is
// structured-data parsing, not CSS scraping. Postings expire:
// validThrough is typically ~4 weeks after datePosted, after which the
// URL is a clean HTTP 404.
//
// # Verified but deliberately unsupported
//
//   - Regional editions (/hokkaido/list/... and 11 more region slugs,
//     with prefecture p{NN} / city c{NNNNN} sub-tokens): functional, but
//     they render a separate legacy Shift_JIS template that would need a
//     second parser. Put place names in the keywords instead.
//   - Sort (?soff=ficf|pv): present in the results page's markup, but a
//     plain GET returns the identical default order (confirmed by
//     comparing full first pages), so it is not exposed.
//   - The /search/list/ POST form (structured filters): responds 302
//     into a session-backed flow; the stateless GET DSL above is the
//     robots-allowed surface.
package mynavi
