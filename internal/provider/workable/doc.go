// Package workable provides a public job board API client and curated
// company roster for Workable-hosted careers pages (apply.workable.com).
//
// Quirks observed live (2026-07-18):
//
//   - Search is POST /api/v3/accounts/{account}/jobs with a fixed page size of
//     10 and cursor pagination via nextPage → body token. There is no limit.
//   - Location objects frequently ship null region (and empty city) even when
//     country/city are set; the OpenAPI schema marks those fields nullable.
//   - Facet location display is account-inconsistent — present for blueground,
//     absent on zego — so consumers rebuild labels from structured fields.
//   - Cloudflare 403-blocks some client User-Agents (Python-urllib observed);
//     Go's default UA and curl pass.
package workable
