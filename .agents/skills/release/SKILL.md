---
name: release
description: Use when the user asks to cut, tag, or ship an openings-mcp release (e.g. "release v0.8.0"), to run pre-release provider smoke tests, or to rewrite a fresh release's auto-generated notes.
---

# Release

## Overview

Cut a release end to end: validate the version, live-smoke-test every
provider at HEAD, push the tag, wait for the Release workflow
(goreleaser publishes the GitHub release with a raw commit-list body),
then rewrite the notes in the house style and report back.

## Input

Exactly one version matching `^v\d+\.\d+\.\d+$` (all numeric, e.g.
`v0.8.0`). Reject anything else — `0.8.0`, `v0.8`, `v0.8.0-rc1` — and
ask for a corrected version instead of guessing. It must not already
exist and must sort above the latest tag
(`git tag --sort=-v:refname | head -1` — that latest tag is also the
"previous version" used in steps 2, 4, and 5). Require a clean tree on
main, in sync with origin/main.

## 1. Smoke-test every provider (before tagging)

Test HEAD through the real MCP path, never an installed binary or the
session's connected openings-mcp server — those run the previous
release. Build once (`go build -o <scratchpad>/openings-mcp
./cmd/openings-mcp`) and drive it over stdio; the request script is in
the integrate-new-provider skill's MCP-surface step.

- **ATS adapters** (the unified-search set: `providerOrder` in
  `internal/cli/verifycompanies`): per provider, sample 3–5 companies
  from `internal/provider/<name>/companies.yaml`;
  `search_jobs_by_company` must return live listings for each, and
  `get_job_detail_by_company` must succeed on at least one returned
  job_id.
- **Dedicated-tool providers** (job boards and careers sites: the
  `Register*` calls in `newServer`, `cmd/openings-mcp/main.go`): per
  provider, 3–5 varied `<name>_search_jobs` queries exercising the
  filters it supports (keyword, location, ...) plus one
  `<name>_get_job_detail` on a returned identifier.

A failure stops the release: retry once to rule out upstream
flakiness, then report to the user instead of tagging. Also run
`go test ./...` — cheaper to catch breakage here than across six CI
matrix legs after the tag is public.

## 2. Tag

Annotated, message in the same shape as the release title:

```bash
git tag -a vX.Y.Z -m "vX.Y.Z: <one-line summary of the release>"
git push origin vX.Y.Z
```

Skim `git log <prev>..HEAD --oneline` first to write the summary line.

## 3. Wait for CI

The tag push triggers `.github/workflows/release.yml`: unit tests on
six platforms, then goreleaser (GitHub release, archives, Homebrew
cask, Docker images). Watch it to completion:

```bash
gh run watch --exit-status \
  "$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')"
```

On failure, report the failing job; never delete the tag or release to
retry without asking.

## 4. Rewrite the release notes

Collect what shipped since the previous tag: `git log <prev>..vX.Y.Z
--oneline` plus the merged PRs it references — read enough of each PR
to state its user-facing impact, not just its subject line. Then
replace goreleaser's title and commit-list body:

```bash
gh release edit vX.Y.Z --title "..." --notes-file <scratchpad>/notes.md
```

House style (see v0.5.5, v0.6.0, v0.7.0 for worked examples):

- Title `vX.Y.Z: <one-line story of the release>`.
- Only the `##` sections that apply: `Highlights` (a minor release's
  lead story, opening phrase bolded), `Added`, `Changed`, `Fixed`.
- Bullets are full sentences about user-visible outcomes, not commit
  restatements; concrete numbers (roster counts) when they carry the
  story; each bullet ends with its PR refs `(#N)` — GitHub autolinks
  them in release notes, no full URL needed.
- Internal-only changes (CI, refactors, docs, skills) are omitted
  unless users can observe the difference.
- Last line: `**Full Changelog**:
  https://github.com/amikai/openings-mcp/compare/<prev>...vX.Y.Z`.
- English throughout.

## 5. Report

End the run by showing the user the full changelog (the compare link
plus the commit/PR list gathered in step 4) and the rewritten release
note verbatim.

## Common Mistakes

- Smoke-testing through a released binary or the session's MCP server —
  that validates the previous release, not the one being cut.
- Tagging before the smoke tests finish: the tag push immediately
  triggers a public release build.
- Writing notes from commit subjects alone — the style demands
  user-facing impact, which usually lives in the PR diff or
  description.
- Reciting provider lists from memory: derive the ATS set and the
  dedicated tools from the wiring named in step 1; both change from
  release to release.
