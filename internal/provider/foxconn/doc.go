// Package foxconn provides a client for Hon Hai / Foxconn's Taiwan careers
// API — the public, unauthenticated JSON REST surface at
// https://recruit.foxconn.com/hh_recruit_tw_api/portal_api/ that backs the
// Vue SPA at https://recruit.foxconn.com/isite-web-tw/main/jobsearch.
//
// This is a single careers site (Hon Hai's own board), not a multi-tenant
// ATS, so there is no company roster — search and detail address one board
// directly. The two server-side list filters, workplaceCode and
// talentZoneCode, draw from small, rarely-changing enums that are
// hand-transcribed as static reference data in codes.go (WorkplaceCodes and
// TalentZoneCodes) rather than fetched at runtime.
//
// The wire surface, its quirks, and the schemas are documented in
// openapi.yaml (the source for the generated client). The notable behaviors
// worth knowing before reading the generated code: the list endpoint has no
// pagination and returns its full matching array in one response; an unknown
// filter value or zero-match keyword is HTTP 200 with an empty array rather
// than a 404; the detail endpoint 404s with an RFC 7807 problem+json body;
// and job_type/job_type_name are populated on the list endpoint but come
// back null on the detail endpoint for the same job.
//
// Foxconn runs sibling regional portals under other host paths (e.g.
// isite-web-vn for Vietnam); only the Taiwan portal is modeled here, but the
// same hh_recruit_*_api path shape is the entry point for extending to them.
package foxconn
