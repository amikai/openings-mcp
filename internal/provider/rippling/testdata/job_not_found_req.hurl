# A well-formed but unknown job uuid 404s with a generic {"ok": false, ...}
# validation-error body, not a structured RESOURCE_NOT_FOUND like the board
# endpoint.
GET https://api.rippling.com/platform/api/ats/v1/board/pythian/jobs/1b2c3d4e-5f60-4789-8abc-def012345678

HTTP 404
