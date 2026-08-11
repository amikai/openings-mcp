// Package adp_myjobs accesses public ADP MyJobs career boards
// (myjobs.adp.com), not the partner/HCM APIs on developers.adp.com and not
// Workforce Now (workforcenow.adp.com; future package adp_wfn).
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
//     "search" (without $) is ignored by the API.
//
//  3. Job detail:
//     GET .../job-requisitions/search-meta/{reqId}
//
//  4. Apply URL (constructed):
//     https://myjobs.adp.com/{slug}/cx/job/{reqId}
//
// # Out of scope
//
// ADP Workforce Now careercenter APIs and official ADP developer staffing APIs
// are out of scope. Location facets are discovered from requisitionLocations
// and sent upstream as OData $filter expressions on itemID.
package adp_myjobs
