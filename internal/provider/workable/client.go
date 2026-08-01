package workable

// DefaultBaseURL is the origin behind Workable-hosted careers pages — the
// single production server in this package's openapi.yaml. Paths carry their
// own /api/v3 and /api/v2 prefixes, so pass this to NewClient unchanged.
// Tests pass NewMockServer().URL instead.
const DefaultBaseURL = "https://apply.workable.com"
