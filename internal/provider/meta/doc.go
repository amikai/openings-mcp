// Package meta reads job listings and posting details from Meta Careers
// (www.metacareers.com).
//
// # Surface
//
// The site is a Facebook Comet app: all data flows through POST /graphql
// using persisted queries. A request is form-encoded and needs only three
// fields — lsd, variables (JSON), and doc_id — plus browser-shaped headers.
// There is no public schema; doc_ids pin server-side query documents:
//
//   - CareersJobSearchResultsDataQuery (search): variables
//     {isLoggedIn, search_input, viewasUserID}
//   - CandidatePortalJobDetailsViewQuery (detail): variables
//     {renderLoggedInView, requisitionID, viewasUserID}
//   - CareersJobSearchFiltersV3Query and
//     CareersJobSearchLocationFilterV3Query (filter value lists): empty
//     variables. The lists are dynamic — teams, technologies, roles, and
//     offices change as Meta reorganizes — so clients enumerate them at
//     call time instead of hardcoding values.
//
// doc_ids drift when Meta redeploys the site. If both operations start
// failing, re-derive them from the site: search-page doc_ids live in JS
// bundle modules named <QueryName>_candidate_portalRelayOperation, and the
// detail doc_id is in the "expectedPreloaders" descriptor embedded in a
// job-details page's HTML.
//
// # Anti-bot gates (all presence-only, no real session needed)
//
//   - The lsd form field and X-FB-LSD header must be present, but any
//     value passes — the token is not validated for logged-out requests.
//   - Origin, Referer, and Sec-Fetch-Dest/-Mode/-Site (empty/cors/same-origin)
//     request headers are required; without them the endpoint answers
//     HTTP 400 with a Facebook error page.
//   - No cookies are needed.
//
// # Quirks
//
//   - Responses are JSON but carry Content-Type: text/html; charset="utf-8"
//     (quoted charset), which some strict parsers reject — testdata hurl
//     fixtures assert on raw bytes for this reason.
//   - Search is server-side filtered but unpaginated: the response carries
//     every matching job, and search_input's page / results_per_page keys
//     are no-ops (the website slices results client-side). Filter keys are
//     documented on [SearchRequest].
//   - Detail lookups for unknown requisition IDs return HTTP 200 with a
//     null xcp_requisition_job_description, surfaced as [ErrJobNotFound].
//   - Rich-text detail fields arrive double-encoded: a JSON string whose
//     content is itself a JSON object {"__html": "<fragment>"}.
//   - A public job URL is https://www.metacareers.com/jobs/<id>/ , which
//     redirects to /profile/job_details/<id>/.
package meta
