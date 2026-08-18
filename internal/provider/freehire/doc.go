// Package freehire provides a client for freehire.me's public jobs API.
// openapi.yaml is https://freehire.me/openapi.yaml, used unmodified.
//
// The spec declares eight operations. The MCP tools use six of them and
// deliberately skip two:
//
//   - agentSearchJobs takes the same query surface as searchJobs but
//     returns every row's full body instead of the index preview, and
//     does not attach ghost. Measured at limit=20: 140 KB against
//     searchJobs' 59 KB, of which a tool result keeps the same handful
//     of leading characters either way. searchJobs is what the tools
//     call; getJob serves the full body for the one posting a caller
//     picked.
//   - getSimilarJobs is not wired into the MCP server yet.
//
// # Two silent failure modes
//
// A filter VALUE upstream does not recognize matches nothing and is not
// an error, so a zero-result search usually means a bad slug rather than
// an empty market. getJobFacets, searchCompanies, and searchCities are
// the resolvers for the open vocabularies.
//
// A parameter NAME no filter reads is dropped rather than refused, and
// the answer is the whole catalogue — indistinguishable from a search
// that legitimately matched everything. Only meta.ignored_params
// separates them, so both the MCP tools and cmd/freehire turn its
// presence into an error rather than returning the rows. See
// testdata/jobs_ignored_params_req.hurl.
//
// # Where the spec and the live API disagree
//
// Measured 2026-08-18. Each of these makes the documented behaviour the
// wrong thing to build on:
//
//   - yc_batch's description gives W21 as its example, but the values the
//     API holds are written out in full ("Summer 2009"). The documented
//     form matches nothing, so the tool descriptions carry the verified
//     spelling instead. Same for yc_status and yc_stage, which are
//     capitalized ("Active", "Growth").
//   - my_vote is documented as "Always null on this unauthenticated
//     schema" and comes back as 0.
//   - getCompany refs the shared Limit parameter, which declares
//     default: 10, but an unset limit returns 20 jobs. Nothing here sends
//     a default, so callers get upstream's 20.
//   - offset's prose caps offset+limit at 10000 on the job endpoints, but
//     the schema puts no maximum on it, so the ceiling reaches callers
//     only as a 400. testdata/jobs_deep_page_req.hurl pins it.
//   - semantic_ratio was dropped from the spec in v1.1.0, and the live
//     API now reports it under meta.ignored_params. It is gone from the
//     MCP tools and cmd/freehire.
//   - ghost is declared nullable and documented as carrying null on most
//     postings, but the API omits the key outright. It is also empty
//     across the live catalogue: 600 rows sampled over five different
//     filters, plus a getJob on an evergreen posting, returned no value
//     at all. No fixture can cover a populated one, so the field is
//     forwarded untyped and callers are told its absence means "no
//     signal", not "verified real".
//   - reality can differ between the search index and getJob for the same
//     posting (1 of 5 sampled: the index said likely-evergreen with
//     mass_posting_count 12, getJob said fresh with 2). The index snapshot
//     lags; the two are not the same read.
package freehire
