# マイナビ転職 (Mynavi Tenshoku) Provider — Design

## Context

Mynavi Tenshoku was the one green light from the 2026-07-18 survey of
Japanese mainstream job platforms (Rikunabi = Indeed JP inventory; doda
Akamai-blocked; en転職 robots-blocks ClaudeBot). No official API exists; the
public surface is SSR HTML with a URL path-token search DSL and a complete
schema.org JobPosting JSON-LD on every detail page. robots.txt allows both
paths, and there is no cookie/session/fingerprint gate — unlike LinkedIn,
the closest existing provider in shape.

## Surface choice

Dedicated tools (`mynavi_search_jobs` / `mynavi_get_job_detail`), not an ATS
adapter: Mynavi is a single job board, not a multi-tenant ATS.

## Search scope: national /list/ only

Verified live (the full DSL contract lives in the package doc comment,
internal/provider/mynavi/doc.go):

- `kw{keywords}` — space-separated AND terms; `min{NNNN}` — fixed-step
  salary floor; `pg{N}` — 1-based pages of 50. Segment order free.
- The result page total lives in `p.result__num em`; cassettes are
  `div.cassetteRecruit__content`; zero hits renders count 0 and no
  cassettes (no PR-card injection).

Deliberately out:

- **Regional editions** (`/{region}/list/` with `p{NN}`/`c{NNNNN}` geo
  tokens): functional but render a second, legacy Shift_JIS template that
  would need its own parser. Location terms in `kw` are the alternative.
- **Sort** (`?soff=`): the markup advertises it but a plain GET does not
  reorder results (verified by diffing full first pages); LinkedIn's
  `distance` precedent — don't expose what live behavior doesn't confirm.
- **The `/search/list/` POST form**: session-backed (302), while the GET
  DSL is stateless and robots-allowed.

The one trap worth guarding in code: an unknown path *prefix* (e.g.
`/tokyo/list/…`) soft-falls back to an unfiltered legacy page instead of
404ing. The parser therefore errors when `p.result__num` is missing rather
than reading such a page as zero results.

## Provider package

Hand-written goquery client (`client.go`, `parse.go`), LinkedIn-style; no
ogen. Client-side validation mirrors what the site 404s on: off-step
`MinSalary` (exported `MinSalaries` steps), `/` inside keywords, malformed
job IDs. Detail parsing is JSON-LD extraction selected by `@type` (the page
carries a BreadcrumbList block too), with two tolerant JSON shapes:
`jobLocation` object-or-array, salary values string-or-number. HTML-valued
fields flatten to plain text; the flattener also converts the
double-escaped `&amp;lt;br&amp;gt;` artifacts some employers' section
dividers carry.

Every JobPosting field is surfaced (no forced schema loss): title, org
name/URL, employmentType, industry, occupationalCategory,
datePosted/validThrough, per-prefecture locations, salary
min/max/currency/unit, description, experienceRequirements, workHours,
jobBenefits. Search cards likewise surface all five tableCondition rows
(初年度年収 is optional — 49/50 cards in the fixture), the label tags, and
both dates.

## MCP surface

- `mynavi_search_jobs`: `keyword`, `min_salary` (integer enum mirroring
  `MinSalaries` so the SDK pre-rejects off-step values), `page`. Output:
  `total` + full card summaries with synthesized posting URLs.
- `mynavi_get_job_detail`: `job_id`. Output mirrors JobDetailResponse.
- Tool descriptions note the site is Japanese; there is no rate-limit
  caveat because recon never hit one.

## Testing

Fixtures are captured real responses (50-card Python search, zero-hit
search, one detail page); five hurl files replay the live contract
including the 404 not-found path. `NewMockServer` routes the zero-hit
keyword to the empty fixture and `MockNotFoundJobID` to 404 so client and
MCP tests exercise those paths. The detail fixture's posting expires
~2026-07-30; job_detail_req.hurl documents recapturing with a fresh ID.
