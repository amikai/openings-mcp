# `cmd/workday` TenantClient replacement

## Goal

Replace the direct `workday.Client` usage in `cmd/workday` with the existing
`workday.TenantClient`, while keeping the command-line interface and observable
behavior unchanged.

## Scope

The change covers the `facets` and `search` command paths, including the
per-result job-detail fetch. It does not change flags, output formats, request
parameters, error wording, concurrency, or fallback URL behavior.

## Design

Each command creates one `workday.TenantClient` and passes the tenant slug to
the tenant-aware methods:

- `runFacets` calls `JobsByTenant(ctx, tenant, request)`.
- `runSearch` calls `JobsByTenant(ctx, tenant, request)`.
- `fetchJobResult` calls `JobDetailByTenant(ctx, tenant, location, titleSlug)`.

The existing `CompaniesByTenant` lookup remains in the command functions. It
continues to provide the current user-facing unknown-tenant error and supplies
the confirmed base URL used to construct a best-effort fallback link when a
detail request fails. The `TenantClient` itself remains unchanged.

## Error handling and data flow

Tenant validation, facet parsing, timeout setup, request bodies, result
ordering, detail-fetch concurrency, and per-job fallback handling remain as
they are today. Only the transport client and the way its server URL is
selected change: the tenant slug is passed to `TenantClient`, which resolves
the confirmed tenant URL internally.

## Testing

Existing command validation tests must continue to pass. Add a focused
no-network detail-path test using a custom HTTP transport and
`TenantClient`, verifying that the tenant-aware client returns the same detail
result shape. Run the package tests and the full Go test suite after the
change.
