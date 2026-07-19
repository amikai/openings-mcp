// Package avature is a client for public Avature career-portal HTML pages.
//
// Avature hosts one or more portals per tenant at
// https://<tenant>.avature.net/<portal> (custom domains exist, e.g.
// careers.unifiservice.com). The customer REST API is auth-gated and the
// portal RSS feed (SearchJobs/feed/) is capped at 20 items with empty
// descriptions, so the server-rendered HTML is the only public surface.
//
// Surface:
//
//   - GET <base>/SearchJobs?search=<kw>&jobOffset=<n> — one listing page.
//     "search" is a platform-level full-text query (honored even on portals
//     without a visible keyword box); it covers titles and descriptions but
//     is not a location filter. "jobOffset" is a true arbitrary zero-based
//     offset. Page size is portal-configured (6-20 observed) and cannot be
//     raised via jobRecordsPerPage.
//   - GET <base>/JobDetail/<slug>/<id> — one posting. The slug segment is
//     cosmetic; only the numeric id matters. An unknown id 302s to
//     <base>/Error, which itself serves HTTP 404.
//
// A locale path segment (e.g. /en_US) is optional: locale-less URLs 302 to
// the portal's default locale with the query string preserved, so base URLs
// omit it.
//
// Quirks:
//
//   - The "1-12 of 436 results" legend is optional per portal; when hidden,
//     [SearchResponse.Total] is -1 and callers fall back to
//     [SearchResponse.HasNext].
//   - Facet filters (location, category, ...) use per-tenant numeric field
//     ids whose options load only via JS autocomplete — there is no portable
//     server-side facet surface.
//   - List-item markup varies by portal theme (article--result,
//     article--jobs, list__item, ...); parsing anchors on JobDetail links
//     and applies per-theme location heuristics, so [Job.Location] can be
//     empty on unrecognized themes.
//   - Posted dates are not rendered on list or detail pages in the observed
//     themes.
//   - Bot protection varies by tenant: some portals answer non-browser
//     clients with an empty HTTP 202 challenge (e.g. Delta) or require
//     login (e.g. Maximus). Roster entries must be verified live.
package avature
