package freehire

// MaxResultWindow is the deepest row freehire's search will page to.
// The API answers 400 {"error":"pagination too deep"} once offset+limit
// exceeds it: offset=9999&limit=1 succeeds, offset=9999&limit=100 does
// not. openapi.yaml declares no maximum on offset, so this is measured
// against the live API rather than generated (see
// testdata/jobs_deep_page_req.hurl).
const MaxResultWindow = 10000
