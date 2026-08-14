// Package adp_wfn accesses public ADP Workforce Now career centers
// (workforcenow.adp.com), not the MyJobs boards served by package adp_myjobs
// (myjobs.adp.com) and not the partner/HCM APIs on developers.adp.com.
//
// The two ADP surfaces share a vendor and nothing else; see
// docs/adr/0001-separate-adp-provider-packages.md.
//
// The endpoint shapes, parameters, and their quirks are documented in
// openapi.yaml and carried into the generated client's godoc. What follows is
// what a reader needs before touching this package at all.
//
// # No authentication
//
// Every operation is a bare GET. No token, no cookie, no session. The one
// exception is [BoardClient.ResolveLegacySlug], which needs a throwaway cookie
// jar to complete a redirect handshake.
//
// # Silent failures outnumber loud ones
//
// Almost nothing on this surface reports a problem with a status code. The
// traps, in the order they bite:
//
//  1. Every filter-shaped query parameter other than userQuery is ignored,
//     and answers HTTP 200 with the whole unfiltered board. Real filtering
//     happens through the locationsList and workerCategoriesList request
//     headers.
//  2. A locationsList value that forms no "<value>,<qualifier>" pair — a
//     bare "Markle" — also answers the whole board. One that is well formed
//     but matches nothing answers zero rows instead, which is safe.
//  3. A workerCategoriesList oid the tenant does not publish is worse: it
//     answers a large arbitrary subset. On a verified 21-posting board three
//     unrelated bogus values each returned the same 19 rows. A whole-board
//     fallback is at least recognisable as unfiltered; this is not.
//     Together, 2 and 3 mean every filter value must be validated against
//     the tenant's own catalog before it is sent, and caller input must
//     never be forwarded unchecked.
//  4. meta.totalNumber is a row count only on an unfiltered request. Under
//     any filter it is a relevance tally that routinely exceeds the board.
//  5. $skip is one-based; $skip=0 drops a row. $top is honored only when
//     unfiltered, so paging must always step by [PageSize].
//  6. Naming a locale the tenant does not publish postings under answers an
//     empty board, and omitting locale falls back to the server's en_US.
//     Both look exactly like a tenant with no openings.
//  7. The detail endpoint answers HTTP 200 for an unknown id, returning a
//     record with no itemID. That absence is the only not-found signal.
//
// Only an unknown cid fails loudly, with HTTP 404 and an HTML body rather
// than JSON.
//
// # Keyword search cannot be paged
//
// userQuery returns a stable result set in an unstable order: identical
// requests come back ordered differently, so $skip windows overlap and drop
// rows. Precision also degrades sharply after the first page. It is a
// relevance probe over the description text, not a filter, and callers take
// page one only. Location and job-type filtering page soundly and are the
// tool when completeness matters.
//
// # A tenant's identity is not in its jobs
//
// Neither the job list nor the job detail carries a company name; the career
// center renders its header as a logo image, so the SPA never receives one.
// [BoardClient.TenantInfo] reads the only public field that has it. The name
// there is ADP's own client record and is passed through verbatim — it shows
// truncation and inconsistent casing, but reconstructing a tidier name from
// slugs, domains, or logo filenames would be inventing one.
//
// # Locale discovery
//
// [BoardClient.PrimaryLocale] reads the career-center content links, whose
// first advertised locale matched the job-bearing locale on every tenant
// checked. The similarly named ClientDefaultLocale on client-features is not
// usable for this: it reports en_US for a tenant whose postings exist only
// under en_CA.
//
// # Out of scope
//
// The internal employee career center (ccId 19000101_000002) is not served;
// it answers HTTP 403 for external candidates. ADP MyJobs and the ADP
// developer APIs are likewise out of scope.
package adp_wfn
