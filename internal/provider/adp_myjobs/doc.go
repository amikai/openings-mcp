// Package adp_myjobs accesses public ADP MyJobs career boards
// (myjobs.adp.com), not the partner/HCM APIs on developers.adp.com and not
// Workforce Now (workforcenow.adp.com), which package adp_wfn serves.
//
// # Public surface
//
//  1. Career-site config (no login):
//     GET https://myjobs.adp.com/public/staffing/v1/career-site/{slug}
//     Returns clientName, domain, orgoid, myJobsToken (short-lived public
//     board session used by the career SPA).
//
//  2. Job requisitions (OData-style list + free-text search):
//     GET https://my.adp.com/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters
//     Headers: myjobstoken, rolecode (manager), orgoid; Origin/Referer
//     https://myjobs.adp.com. Query: $top, $skip, $orderby, $select, tz, and
//     optional $search for server-side keyword filtering. The bare name
//     "search" (without $) is ignored by the API. $top is not capped at 100,
//     but a page large enough to return a whole big board fails with HTTP 502.
//
//  3. Custom filters (the tenant's own dimensions):
//     GET .../job-requisitions/search-custom-filters
//     Returns every dimension the tenant configured with its values, in one
//     request. Each carries an opaque slot code ("FIELD1".."FIELD5") plus a
//     display label; the codes are positional, so FIELD3 is "Full-Time/Part-Time"
//     on one board, "Area of Interest" on another and "Compensation Range" on a
//     third, and a label may repeat within one board. Apply them on the listing
//     endpoint as $filter=FIELD1 eq 'Value':
//
//     - the slot code is the field name; naming the label instead returns the
//     whole unfiltered board with HTTP 200, as does any unconfigured code
//     - the value must equal the published one exactly, case included; a
//     near-miss returns zero jobs rather than an error
//     - clauses AND with "&&"; there is no OR ("a || b" matches nothing and
//     "(a or b)" is ignored in favour of the whole board)
//     - filters compose with $search, $skip and $orderby
//     - spaces inside $filter must be percent-encoded: a "+" is read as part of
//     the value and silently returns the whole board
//
//     Geography is only available where a tenant configured it. Boards that
//     file jobs by store (Guitar Center, Follett) offer no city or state
//     dimension, which is why free-text location search is not supported.
//
//  4. Job detail:
//     GET .../job-requisitions/search-meta/{reqId}
//
//  5. Apply URL (constructed):
//     https://myjobs.adp.com/{slug}/cx/job/{reqId}
//
// # Out of scope
//
// ADP Workforce Now careercenter APIs and official ADP developer staffing APIs
// are out of scope.
package adp_myjobs
