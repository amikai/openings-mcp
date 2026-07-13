---
name: discover-companies
description: Use when growing one ATS provider's curated roster with net-new companies — "find more <provider> companies", "add <industry/region/theme> companies to <provider>", or a roster is thin for a segment. Input is one existing roster-based provider. Not for pruning or promoting entries already on file (verify-companies) or adding a new ATS platform (integrate-new-provider).
---

# Discover Companies

## Overview

Given one roster-based provider, source companies hosted on that ATS
that no roster has yet, confirm each against the live adapter path, and
land them in `internal/provider/<provider>/companies.yaml`. This skill
covers the sourcing; verification and promotion mechanics are the
verify-companies skill, and the roster commit convention is in
CLAUDE.md. Scope a session to one provider — rosters land one per PR.

The providers in scope are the `--provider` values of
`cmd/verify-companies` (`providerOrder`). SmartRecruiters keeps a
roster too but has no adapter yet; spot-check its candidates with
`go run ./cmd/smartrecruiters --company <id> search` instead.

## Sourcing channels

Work from the provider's public board-host shape — the entry for the
provider in `careersHostPatternsByAdapter`
(`internal/ats/registry.go`). Mix channels; each one alone dries up:

- **Site dorks** — WebSearch `site:<board host> <theme keywords>`
  (e.g. `site:jobs.lever.co fintech`,
  `site:job-boards.greenhouse.io robotics` — for greenhouse the legacy
  `boards.greenhouse.io` host is indexed too and redirects). Dork the
  public board host, never the API host — search engines index board
  pages, not API responses. Vary keywords by industry, region, and
  role; page past the first results.
- **Theme-first probing** — start from a company list (industry
  roundups, accelerator batches, "companies using <ATS>" articles),
  websearch `<company> careers`, and keep the ones whose careers URL
  matches the provider's host shape. The slug comes from that URL,
  never from guessing the company name.
- **Migration finds** — verify-companies audits surface companies that
  moved ATS; their new host names the provider and slug directly.

A careers URL matching a *different* supported provider's host shape is
still a find — park it in `unverified/<other>.yaml` rather than mixing
rosters in one session.

## Candidate to entry

1. Dedupe first: `grep -ri "<name-or-slug>"
   internal/provider/*/companies.yaml unverified/*.yaml`.
   `ats.NewRegistry` requires display names and slugs to be globally
   unique across all adapters, so a hit anywhere disqualifies or
   renames the candidate.
2. Copy the YAML key shape from the target `companies.yaml` — every
   provider names its identifier differently, and workday needs the
   tenant/instance/site triple from the careers URL, one row per
   tenant (the adapter dedupes by tenant and serves only the first).
3. Display name comes from an authoritative source (the company's own
   site or the board's page title). A slug with an unconfirmed name
   goes to `unverified/<provider>.yaml`, not the curated roster.

## Verify the batch

1. Append the batch to `companies.yaml`, then
   `go run ./cmd/verify-companies --provider <name>` — rosters are
   go:embed'd, so `go run` picks the additions up. Keep the default
   timeout; big full-dump boards need it.
2. Drop confirmed-stale ERRORs; retry transients first (the
   verify-companies skill's audit steps). `zero-job` entries are fine —
   board live, no current openings.
3. `go test ./...` — registry tests catch collisions and shape
   mistakes.
4. Commit per the `roster:` convention in CLAUDE.md: one batch, one
   commit, one PR.

## Common mistakes

- Probing a guessed slug and accepting any 200 — a wrong slug can be a
  *different* company's live board. The board's company name must match
  the candidate.
- Promoting a slug straight into `companies.yaml` with the slug as its
  display name — that's what `unverified/` quarantines.
- Dorking the API host (`api.lever.co`, `api.ashbyhq.com`) and finding
  nothing.
- Adding several rows for one workday tenant; only the first is served.
- Sourcing across providers in one session and batching multiple
  rosters into one PR.
