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
//     "search" (without $) is ignored by the API. $top is not capped at 100,
//     but a page large enough to return a whole big board fails with HTTP 502.
//
//  3. Location filtering ($filter), see [GeoBox]:
//     Only a geographic bounding box over workLocations.geoLocation works, in
//     the exact malformed POLYGON shape ADP's own SPA sends. Every other
//     $filter — requisitionLocations/itemID, workLocations.city, or a field
//     that does not exist — returns HTTP 200 and the complete unfiltered board,
//     so a wrong expression is indistinguishable from a board with no location
//     filter. $search and a box compose, and both respect $skip/$orderby.
//
//     The SPA reaches this by geocoding its Location input through Google
//     Places and pairing the box with a radius= parameter; the radius is what
//     narrows its "25 mi" search, while the box does the location matching.
//     This package instead takes coordinates from the board's own
//     requisitionLocations, so no geocoder is involved. Boards that publish
//     locations without coordinates (or with a 0/0 placeholder) cannot be
//     location-filtered at all.
//
//     The facet catalog behind the board's "All Filters" dialog comes from a
//     sibling endpoint, job-requisitions/search-custom-filters, which returns
//     tenant-defined categories (State, City, Area of Interest, Brand, ...) in
//     one request. Those labels carry no coordinates and the parameter that
//     applies them upstream is not known, so this package does not use it.
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
