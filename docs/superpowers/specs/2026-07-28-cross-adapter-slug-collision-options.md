# Cross-adapter slug collision options

Date: 2026-07-28  
Status: **decided** — see [Decision](#decision) at the end. Everything above it is the options survey that led there, kept as written.  
Related:

- [2026-07-06 unified company tools](2026-07-06-unified-company-tools-design.md) — global slug/name uniqueness; ATS invisible
- [2026-07-10 careers URL resolve](2026-07-10-careers-url-resolve-design.md) — `company` may be a careers URL
- HERP roster growth (#251) and follow-up collision drops (#252)

## Problem

`ats.NewRegistry` requires **globally unique** roster slugs and **globally unique** normalized display names across every adapter. A collision fails process startup (and CI):

```text
ats: company slug "nature" from herp collides with "nature" from workday
```

That rule was reasonable when rosters were small. It does not scale:

| Pressure | Why it hurts |
|---|---|
| ~20k curated entries | Short brand tokens collide across ATS families |
| HERP short slugs | Large share of slugs are ≤6–8 chars (`nature`, `stable`, `chime`, …) |
| SmartRecruiters alone | ~11k identifiers in the same flat namespace |
| Enrich workflow | Discovery must drop live boards to keep CI green |

**Invariant under stress:** each provider’s identifier is only unique *inside that ATS*. Greenhouse board tokens, Workday tenants, HERP slugs, and SmartRecruiters company identifiers were never meant to be one global primary key.

**Product invariant to preserve:** unified tools expose **one `company` parameter** and keep the **ATS invisible**. Users and agents know company names and careers pages, not “this board is on HERP.”

## Current resolve flow

Tools: `search_jobs_by_company`, `get_filters_by_company`, `get_job_detail_by_company` → `Registry.Resolve(company)` → `(adapter, slug)` → adapter call.

```text
Resolve(input)
  1. normalize → exact roster slug hit
  2. normalize → exact display name hit
  3. careers-URL shaped input → adapter.ParseCareersURL (first match)
  4. miss → teaching error + closest roster slugs
```

Startup: any cross-adapter slug or name collision → `NewRegistry` error.

Filters and job IDs are **per company / per tenant**. They are not a shared vocabulary across boards.

---

## Option A — Keep global uniqueness (drop or rename on collide)

### Idea

Keep `NewRegistry` as today. When enrich finds a cross-adapter slug or name clash, **drop the new entry** (or rename display name / invent a non-colliding slug where the adapter allows it).

### Agent / user flow

Unchanged for non-colliding companies. Colliding companies simply never appear in the roster (URL resolve may still work for URL-capable adapters).

### Pros

- No tool or registry contract change
- Simple mental model: one string → one company, always
- Matches the original “collision = curation bug” story

### Cons

- Loses real companies as rosters grow (HERP alone dropped 17 on first large enrich)
- Encourages bad renames (slug must often match the live ATS path segment)
- Operational cost rises with every multi-thousand roster expand

### Fit

Stopgap only. Already used in #252.

---

## Option B — Provider-qualified keys (`herp:nature`)

### Idea

Allow the same bare slug on multiple adapters. Public disambiguation key becomes `provider:slug` (or similar). Teaching errors tell the agent to retry with a qualifier.

### Agent / user flow

```text
search_jobs_by_company(company="nature")
  → ambiguous; try herp:nature or workday:nature

search_jobs_by_company(company="herp:nature")
  → herp adapter + slug nature
```

### Pros

- Stable, compact identifiers for logs, tests, and debug CLIs
- Easy to implement as an extra Resolve branch
- No need for a full careers URL in the happy path after disambiguation

### Cons

- **Breaks “ATS invisible.”** Callers must know or learn which ATS hosts the company
- Users and most agents do not have that knowledge
- Teaching errors sound like jargon (`try herp:nature`) rather than company identity
- Poor match for the unified-tools product promise

### Fit

Reasonable as an **internal** key or CLI escape hatch. **Poor as the primary MCP disambiguation UX.**

---

## Option C — Multi-match Resolve (structured candidates)

### Idea

`NewRegistry` allows multiple entries per bare slug (and optionally per name). `Resolve` returns either a single `(adapter, slug)` or an **ambiguous set of candidates**. Tools decide how to surface that.

Important constraint from filters:

> Different companies have different filter dimensions and values.  
> Multi-match must **not** mean “one `filters` map applied to N boards.”

### Sub-options

#### C1 — Disambiguate only (recommended shape of C)

On ≥2 hits, return a teaching error (or structured error payload) listing candidates. Do **not** run Filters/Search until the next call uses a unique key.

```text
get_filters_by_company("nature")
  → ambiguous:
       Nature株式会社  → https://herp.careers/careers/companies/nature
       Nature Research → https://nature.wd….myworkdayjobs.com/…/Search

get_filters_by_company(<one careers URL or unique name>)
  → single FilterSet

search_jobs_by_company(<same unique key>, filters=…)
  → single SearchResult
```

**Is “two filters” the same as “search twice”?**  
No. Returning two filter tables is still only metadata. Searching both companies requires **two independent pipelines** (filters₁ + search₁, filters₂ + search₂). Prefer not to return two filter maps in one response; that invites mixed keys.

#### C2 — Fan-out search without filters

On ambiguous bare slug with **no** filters, optionally search all candidates and merge rows tagged with company/provider identity.

| Allowed | Not allowed |
|---|---|
| Coarse `query` / `location` explore | Passing `filters` across candidates |
| Explicit product choice to “search all matches” | Merging filter schemas |
| | Assuming `job_id` is comparable across adapters |

Even C2 should refuse fan-out when `filters` is non-empty, and should refuse ambiguous `get_job_detail_by_company`.

### Pros

- Stops deleting real roster rows for namespace accidents
- Aligns with “collision is runtime ambiguity,” not “startup corruption”
- Can stay ATS-invisible if candidates are described by **display name + careers URL**

### Cons

- Tool contract must define ambiguous responses (error string vs structured candidates)
- Extra agent round-trip on collisions
- C2 merge semantics (pagination, totals, ranking) are messy
- Roster-only adapters (e.g. Eightfold, SuccessFactors, JOIN) may lack a stable public URL in the candidate list

### Fit

Best **registry behavior**: allow multi-entry indexes; **force single-company** for filters, detail, and filtered search. Fan-out search is optional and secondary.

---

## Option D — Namespace slugs internally; keep display names global

### Idea

Internal map key becomes `(adapter, slug)` always. Display names remain globally unique so `Resolve("Acme Inc")` still returns one hit. Bare slug collisions are allowed; bare name collisions are not.

### Agent / user flow

- Unique name → one shot (same as today)
- Unique slug → one shot
- Colliding slug, unique names → miss on slug path is avoided if we store multi-slug; resolve by name still works
- Colliding names → still need B/C/E-style disambiguation

### Pros

- Matches how ATS identifiers actually work
- Names remain human-friendly primary keys when curated carefully

### Cons

- Global **name** uniqueness still fails for genuine distinct orgs with the same legal/marketing name in different countries or ATS tenants
- HERP-style short English slugs still collide on the slug path; name path only helps when names differ
- Incomplete alone; still needs a disambiguation story for names and short slugs

### Fit

Good **storage model** underneath C or E, not a full product answer by itself.

---

## Option E — Careers URL as authority; roster as directory / suggest

### Idea

Treat **careers page URL** as the unambiguous company identity (already partially implemented). On roster ambiguity, do **not** pick a winner: teach the client to retry with a **careers URL** (or a unique display name). Roster becomes an index for discovery and suggestions, not a global exclusive primary key.

### Agent / user flow

```text
                    company param
                         │
            ┌────────────┴────────────┐
            ▼                         ▼
     careers URL?              roster slug / name
            │                         │
            ▼                    0 / 1 / ≥2 hits
     ParseCareersURL                  │
     (authoritative)         0 → fuzzy suggest (+ optional URLs)
                             1 → use as today
                             ≥2 → teaching error with
                                  display names + careers URLs
                                        │
                                        ▼
                              retry with URL or unique name
                                        │
                                        ▼
                         Filters / Search / Detail
                         (always one company)
```

### Pros

- Preserves **ATS invisible** product surface (URL identifies the board, not the vendor name)
- Reuses existing `ParseCareersURL` investment
- Scales with enrich: colliding short slugs can all stay in roster
- Teaching errors are actionable with copy-pasteable keys

### Cons

- One extra round-trip on ambiguous bare inputs
- Suggest quality must improve (bare slug suggestions are useless under collision; need name + URL)
- Roster-only providers without a parseable public board URL need a fallback (unique display name only, or stay drop-on-collide for those families)
- Agents must be instructed (tool description) to retry with the listed URL

### Fit

Strong default product direction when combined with C1 (multi-entry registry + disambiguate-before-filters).

---

## Cross-cutting rules (any option beyond A)

### Filters

| Situation | Required behavior |
|---|---|
| Exactly one resolved company | `get_filters` / filtered `search` as today |
| Ambiguous company | Do **not** return a merged FilterSet |
| Filtered search while ambiguous | Reject; ask for unique company key first |
| Two companies intentionally | Two full pipelines: filters₁+search₁ and filters₂+search₂ |

Returning two filter maps in one `get_filters` call is **not** “search twice,” but it usually **leads** to two searches if the agent explores both. Prefer disambiguation-first to avoid double work and mixed schemas.

### Job detail

`job_id` is opaque and adapter-local. Ambiguous `company` must never run Detail.

### What to show in teaching errors

Prefer company identity, not vendor jargon:

| Good | Bad (as primary UX) |
|---|---|
| Display name | `herp:nature` as the only hint |
| Careers URL | “use the workday adapter” |
| Optional provider in parentheses for power users | Provider required to proceed |

### ATS-invisible test

If a successful happy path or a disambiguation step **requires** the user to know Greenhouse vs HERP vs Workday, the option fails the product invariant.

---

## Comparison

| Option | Startup on collide | Agent extra hop | ATS visible? | Keeps both companies | Filter-safe |
|---|---|---|---|---|---|
| **A** Drop/rename | Fail until drop | No | No | No | Yes |
| **B** `provider:slug` | OK if multi-index | Yes | **Yes (bad)** | Yes | Yes if disambiguated |
| **C1** Multi-match → disambiguate | OK | Yes | No if name/URL | Yes | Yes |
| **C2** Fan-out search | OK | No (but noisy) | Easy to leak | Yes | **Only without filters** |
| **D** Internal (adapter,slug) | OK for slugs | Sometimes | No | Names still exclusive | Partial |
| **E** URL authority + suggest | OK with multi-index | Yes on collide | No | Yes | Yes |

---

## Recommended composition (not a final decision)

A practical target that matches prior design docs:

1. **Registry storage (D + C):** index by `(adapter, slug)`; allow duplicate bare slugs; decide separately whether duplicate **names** are allowed.
2. **Resolve (E + C1):**
   - Careers URL first (or keep current order but treat multi-slug/name as ambiguous, never silent shadow).
   - Unique slug/name → single hit.
   - Multi hit → teaching error with **display name + careers URL** per candidate (not `provider:slug` as the main instruction).
3. **Tools:** filters, filtered search, and detail require a single resolved company.
4. **Option B** only as optional debug / internal formatting, not MCP-facing primary UX.
5. **Option A** remains the fallback for roster-only adapters that cannot emit a careers URL.

### Explicit non-goals for a first implementation slice

- Cross-company fan-out search as default behavior  
- Merged multi-company filter schemas  
- New MCP tools solely for disambiguation (prefer teaching errors + retry on the same tools)  
- Forcing ATS vendor names into the user-visible contract  

---

## Implementation sketch (if E+C1 is chosen later)

Rough touch points (for a future plan, not this options doc’s commitment):

| Area | Change |
|---|---|
| `internal/ats/registry.go` | Multi-map for slug/name; ambiguous error type; suggest includes careers URL when available |
| `CompanyInfo` | Optional `CareersURL` (or adapter helper) for teaching errors |
| `internal/openingsmcp/company.go` | Schema/description: on ambiguity, retry with listed careers URL or exact display name |
| Roster enrich / verify | Stop dropping on cross-adapter slug clash when URL disambiguation exists |
| Tests | Collision fixtures: unique hit, ambiguous slug, URL wins, filters rejected while ambiguous |

---

## Decision checklist

When choosing:

1. Is **ATS invisible** non-negotiable for MCP? (If yes: reject B as primary.)
2. Must both colliding companies remain searchable via roster, or is “URL only for the loser” enough?
3. Are duplicate **display names** allowed, or only duplicate **slugs**?
4. Is any unfiltered multi-company fan-out (C2) desirable, or always disambiguate first?
5. How do roster-only adapters participate in teaching errors without a public board URL?

---

## Summary

| Label | One-line |
|---|---|
| **A** | Keep global uniqueness; drop/rename collisions |
| **B** | Disambiguate with `provider:slug` (exposes ATS) |
| **C** | Multi-match resolve; disambiguate before filters (C1) or limited fan-out search (C2) |
| **D** | Internal namespaced slugs; global names only |
| **E** | Careers URL is authoritative; roster suggests; collisions teach URLs |

**Filters implication:** multi-company ambiguity never shares one filter set. Two companies means up to two full `get_filters` + `search` pipelines—only after each company is uniquely identified.

---

# Decision

Everything above was written before the roster was measured. This section records what was chosen, the evidence that moved it, and where it departs from the *Recommended composition* sketched above.

## Measured baseline

Every adapter's `Roster()` unioned and keyed through `normalize`, measured on `a9c1a05` (#251 merged, before the #252 collision drops):

| | Count |
|---|---|
| Roster entries across all ATS adapters | 20,367 |
| Cross-adapter **slug** collisions | 17 |
| Cross-adapter **display name** collisions | 0 |
| Intra-adapter collisions (either kind) | 0 |

Every slug collision is HERP against one of workday / ashby / greenhouse / smartrecruiters, and every one is the same shape — a short generic English token:

```text
adglobe believe chime fusion garage helios kipp mota nature
nudge omni pros sidekicks siro smartbank stable union
```

Display names survive because `normalize` keeps Unicode letters, so a Japanese legal suffix is part of the key: `Nature株式会社` normalizes to `nature株式会社`, not `nature`.

### The name-collision count did not stay at zero

Twenty-eight commits later, on `6520026`, it is no longer 0:

```text
ats: company name "BETA Technologies" from greenhouse collides with "BETA Technologies" from lever
```

`main` fails to build a registry today because of it, so `cmd/openings-mcp` is red on a clean checkout. This arrived within one batch of roster enrichment PRs — much sooner than "rosters keep growing, name collisions are inevitable" anticipated, and it removes the argument for the smaller D + C1 alternative entirely. Relaxing name uniqueness is no longer forward-looking work; it is the fix for a live breakage.

### The case the options survey missed

Three of the 17 have a sharper shape — **one company's slug equals a different company's display name**:

| Input | Slug index | Name index |
|---|---|---|
| `fusion` | herp `株式会社FUSION`, workday `Fusion` | workday `Fusion` |
| `stable` | herp `stable株式会社`, ashby `Stable` | ashby `Stable` |
| `chime` | herp `有限会社チャイム`, greenhouse `Chime` | greenhouse `Chime` |

`Resolve` checks slug before name, so Option D — and the *Recommended composition*, which inherits D's storage and keeps the current lookup order — resolves these to whichever entry the slug index happens to hold, silently shadowing the other. None of A–E addresses this. The decision below removes the precedence question instead of answering it.

## What was decided

1. **Registry storage (D):** index by `(adapter, slug)`. `bySlug` and `byName` map to `[]registryEntry`. **Cross-adapter** collisions of either kind stop failing `NewRegistry` — **display name uniqueness is relaxed along with slug uniqueness**, which is further than D goes.
2. **Intra-adapter duplicates stay fatal — slugs *and* names.** For slugs the reason is structural: two entries in one roster sharing a slug render the same careers URL, so the disambiguation retry could not terminate. For names it is narrower — two same-adapter entries with distinct slugs and the same normalized name *are* disambiguable by URL — but a roster is a single YAML file this repo owns, a duplicate name in it is a curation mistake with no upstream cause, and the measured count is 0. Both checks also carry the nine per-adapter tests that build `NewRegistry(oneAdapter)`; relaxing either would quietly turn those into no-ops.
3. **An entry with no renderable careers URL must have a name that resolves uniquely**, enforced at `NewRegistry`. Every candidate for a key shares that key, so a candidate reached through the name index normalizes back to it — telling the caller to retry with a name that is itself ambiguous returns the same error forever. The uniqueness must hold **in the same key space `Resolve` uses**, not just among names: an `ok=false` entry's normalized name must equal no other entry's normalized name **and no other entry's normalized slug**, across all adapters, so that its name key yields exactly one candidate under the point-4 union. Name-vs-slug is the shape the options survey missed (`fusion`, `stable`, `chime`) and is hidden today only by slug-before-name ordering, so checking names against names alone would leave the loop non-terminating. `CareersURL` is a pure roster lookup, so this is checkable inside `NewRegistry`. Expected violations against the current rosters: none.
4. **Resolve (E + C1), with one change:** careers URL is authoritative; roster lookup takes the **union** of slug and name hits, not slug-then-name; two or more distinct entries is a runtime ambiguity error, never a silent pick.
5. **Disambiguation key (E):** the candidate's **careers URL**, rendered by a new `Adapter.CareersURL(slug)`. Display name is the fallback only when an adapter cannot render one.
6. **Tools (C1):** filters, filtered search, and detail all require a single resolved company. This falls out of `Resolve` being the single entry point — no per-tool *resolution* logic. `companyDetail` additionally wraps the ambiguity text with a pointer back to the caller's previous key; that is the only tool-local handling.
7. **Option B stays internal:** kept as a field on the ambiguity error for logs and debug CLIs, never rendered in MCP-facing text.
8. **Option A is not the fallback for whole adapter families**, only for individual entries no adapter can render a URL for. The #252 drops are restored.
9. **Collision reporting survives.** `NewRegistry` stops failing on cross-adapter collisions, but a non-fatal report keeps new ones visible in CI — otherwise roster quality degrades silently, which is what the fatal check was protecting.

### Why relax name uniqueness now

Name uniqueness is currently intact, so keeping it would work today and let disambiguation quote display names instead of URLs — that alternative (D + C1 only, no E) was considered and would have closed 100% of the measured collisions with a smaller diff.

It was rejected as deferred cost rather than avoided cost. Rosters keep growing, name collisions are inevitable at this rate, and the fix at that point is the same `Adapter.CareersURL` interface change across all 18 adapters. Doing it once, now, avoids a second pass.

### Correction to Option E's cons

> Roster-only providers without a parseable public board URL need a fallback

This is backwards. All 18 adapters already implement `ParseCareersURL`, including the roster-gated ones (Eightfold, SuccessFactors, JOIN) named in Option C's cons. The missing direction is the **reverse** — slug → URL — which is what a teaching error needs and which `ats.Adapter` has no method for. Thirteen provider packages already have a `CareersURL() string` on their roster type, reachable today only from the join and oracle adapters and two debug CLIs. So E's real cost is lower than written, and lands somewhere else.

## Resolve contract

```text
key := normalize(company)

① careers-URL shaped input → adapter.ParseCareersURL (authoritative, single answer)
     matched   → (adapter, slug)
     unmatched → fall through to ②

② candidates := dedupe(bySlug[key] ∪ byName[key])   // identity: (adapter, slug)
     1  → (adapter, slug)
     ≥2 → *AmbiguousCompanyError
     0  → unrecognized-careers-URL error if ① was URL-shaped, else fuzzy suggest
```

The union is the load-bearing part: dropping slug-before-name is what makes `fusion` / `stable` / `chime` list both companies instead of shadowing one. Its cost is that an input which resolves to a single company today becomes ambiguous — accepted, since that resolution is an artifact of index order, not of the input being unambiguous.

**The ①→② fallthrough is mandatory, not a nicety.** Two roster families mint slugs that are themselves careers-URL shaped, and `parseCareersInput` only requires a dot and a slash:

| Family | Roster slug | Its own `ParseCareersURL` |
|---|---|---|
| Oracle | `<host>/<site_number>`, e.g. `abc.fa.us2.oraclecloud.com/CX_1` | Rejects it — requires the full `/hcmUI/CandidateExperience/<lang>/sites/<site>/jobs` path |
| Avature | `<host>/<portal>` (the careers URL minus its scheme) | Rejects custom-domain portals — requires a `.avature.net` host |

These resolve today only because the URL branch runs *after* the slug index. Promoting it to ① without a fallthrough — and the current branch hard-errors instead of falling through — would break every Oracle roster entry and every custom-domain Avature entry.

**Identity for dedupe is `(adapter, slug)`.** It cannot be looser: an entry whose slug and display name normalize to the same key (`bunq`/`bunq`, `9fin`/`9fin`, and many more) appears in both index lists and must collapse to one candidate rather than becoming ambiguous with itself. It cannot be tighter either — that is why intra-adapter duplicate slugs stay a fatal `NewRegistry` error, since this identity would otherwise silently merge two distinct companies.

```go
type AmbiguousCompanyError struct {
    Input      string
    Candidates []CompanyCandidate // Name, CareersURL, Provider
}
```

`Provider` is populated but not rendered in the user-facing message.

## Tool contract

Ambiguity reaches the client through `errorResult` as `IsError: true` text content. **No input schema, output struct, or tool is added or changed** — only the `company` parameter descriptions, the error text, and `serverInstructions`. The change is invisible to clients that never hit a collision.

### Ambiguity message

```text
ambiguous company "nature": 2 companies match. Retry with the careers URL of the
one you want:
  Nature株式会社     https://herp.careers/careers/companies/nature
  Nature Research   https://nature.wd108.myworkdayjobs.com/ExternalCareers
```

Candidates whose adapter returns `ok=false` degrade to the display name:

```text
  Foo Corp   (no public careers URL; retry with company="Foo Corp")
```

That fallback only terminates because of decision point 3: an `ok=false` entry's normalized name is required to collide with no other entry's name **and no other entry's slug**, so the quoted name yields exactly one candidate under the union. Without that rule — or with it scoped to names only — the suggested retry would normalize back to the same key and return the same error indefinitely.

Instructing the URL rather than the name is what relaxing name uniqueness costs: with names no longer guaranteed unique, quoting a name is not always a resolvable retry, and the message would otherwise have to distinguish which candidate names still are.

### `search_jobs_by_company`

| Case | Behavior |
|---|---|
| Unique hit | Unchanged |
| Careers URL | Unchanged (now the primary disambiguation path) |
| Miss | Fuzzy suggest, with two repairs below |
| Ambiguous | Ambiguity message; no upstream call |

The miss path is *not* unchanged, because multi-entry indexes break two of its assumptions:

- `r.slugs` is appended once per roster entry, so a restored collision makes `suggest` able to return the same slug twice — `closest matches: nature, nature, …` — precisely when the collision it is describing matters. Suggestions must be de-duplicated by slug.
- The error advertises how many companies are supported using `len(r.bySlug)`, which under multi-entry maps counts distinct normalized keys, not companies. It must count roster entries.

### `get_filters_by_company`

Same ambiguity message, and **no `FilterSet` is fetched for any candidate**. This settles the *"Is two filters the same as search twice?"* question posed under C1 — returning one table per candidate is rejected outright:

- It defers the ambiguity by one hop rather than resolving it — the next `search_jobs_by_company` call still needs a single `company`.
- Filter keys are tenant-specific and do not overlap (`{職種, 雇用形態}` vs `{Location, Job Family}`). A merged view invites a filter map mixing both, which the chosen adapter then rejects or, worse, ignores.
- `companyFiltersOutput.Filters` is a `map[string][]string`. Returning several would mean a breaking output change on every call, including the overwhelming majority with no ambiguity.

After disambiguation the careers URL is the handle for the whole pipeline — the same string goes to filters, search, and detail.

### `get_job_detail_by_company`

Same ambiguity message plus a pointer back to the caller's own previous key: *use the same company value that produced this job_id*. The useful answer here is not "which company do you want" but "which one did you just search".

Ambiguity must hard-fail rather than pick. `job_id` is adapter-local and opaque with no cross-provider format guarantee, so a wrong pick can return a complete, plausible-looking posting belonging to the other company. Using `job_id` to auto-disambiguate is also rejected: verifying it costs one upstream `Detail` per candidate, and the case where several candidates accept the same id is exactly the case where guessing is most dangerous.

## Implementation plan

Three PRs. Slice 1 is additive apart from one named exception; slice 2 carries the resolution behavior change and its tests. Keeping them apart keeps each independently verifiable.

**Ordering correction, found in implementation.** PR 1's code is independent of PR 2, but its acceptance is not: the sweep builds the production registry, and `NewRegistry` currently hard-fails on the live rosters — `nature` before the #252 drops, `BETA Technologies` on `main` today. So the sweep cannot execute a single assertion until PR 2 relaxes the collision checks. PR 1 therefore lands with its implementation complete and its sweep red, and **PR 2 is what turns it green**. Five rounds of plan review missed this because they read the spec and the code without ever constructing a registry from the real rosters.

### PR 1 — `Adapter.CareersURL`

```go
// CareersURL renders the public careers page for a curated company.
// slug must come from Roster; it returns ok=false when this ATS has no
// stable public URL for that company.
CareersURL(slug string) (url string, ok bool)
```

Scoping the contract to **roster slugs only** is what keeps this cheap. Ambiguity candidates are always roster entries — a careers-URL input resolves uniquely and never produces candidates — so the method never sees a slug an adapter minted from a URL. Workday in particular does not have to handle its dual-shaped slug; it composes tenant/instance/site from the roster entry.

Renderer inventory at the provider layer:

| State | Providers |
|---|---|
| Has `CareersURL()` | avature, bamboohr, eightfold, herp, icims, join, oracle, recruitee, smartrecruiters, successfactors, teamtailor, ultipro, workable |
| Has the same thing named `BoardURL()` | ashby, greenhouse, rippling |
| Has no renderer | lever, workday |

So only two need writing, not five. The three `BoardURL()` methods are renamed to `CareersURL()` so each roster type exposes exactly one public careers-page renderer; leaving both names would invite the adapter layer to bind to the wrong one. Each adapter then wires slug → roster company through its existing lookup (`resolveSlug`, or the package's `CompaniesBySlug` / `CompaniesByHost` / `CompaniesByAccount` / … index) and delegates.

**Avature custom-domain portals — the one behavior change in this slice.** 35 of the 100 Avature roster entries are custom domains (`jobs.ea.com`, `careers.unifiservice.com`, …), and `AvatureAdapter.ParseCareersURL` only accepts `.avature.net` hosts, so their rendered URL cannot be pasted back. Two dispositions were available: return `ok=false` for those 35, or extend Avature's `ParseCareersURL` to recognize roster hosts — the pattern Teamtailor already uses.

**Chosen: extend the parser.** Returning `ok=false` for 35 companies that do have a public careers page would degrade them to name-only disambiguation, which is exactly the capability this design is being adopted for. The cost is that PR 1 is no longer purely additive, which is why it is named here rather than discovered in review.

**Scope rule, both directions.** Avature is the *only* `ParseCareersURL` this slice may touch; when the invariant sweep below fails anywhere else, the fix is to adjust the renderer to match the existing parser, never the reverse. Symmetrically, **returning `ok=false` is not an available fix for a failing sweep.** Every roster row of all 18 adapters is renderable today — 16 provider types already have a renderer, and lever and workday both carry every field their URL needs — so **the expected `ok=false` set for this slice is empty**. Any addition to it must be named in this plan, with its provider and the reason its company has no public careers page, before the slice is implemented. Without both directions closed, an implementer can turn any awkward entry into a name-only candidate and still show a green sweep — the disposition this slice explicitly rejected for the 35 Avature custom domains.

Two cases the implementer will hit, both named here so neither becomes an invented convention:

- **Avature `jobs.deutschebahngroup.careers/jobsGlobal`** has a mixed-case portal segment while `ParseCareersURL` lowercases it, so the extension must fold back through `avature.CompaniesBySlug` and return the roster's own slug rather than the lowercased path. Inside the permitted Avature scope.
- **Workday must render the locale segment.** `https://<tenant>.<instance>.myworkdayjobs.com/<site>` does not round-trip for every row: Johnson & Johnson is tenant `jj` / site `JJ`, and with no locale present a two-letter site trips the parser's locale-only-path guard, so `Resolve` returns the unrecognized-careers-URL error. The renderer therefore emits `https://<tenant>.<instance>.myworkdayjobs.com/en-US/<site>`. This is the renderer-side fix the scope rule intends — the parser is not touched, and `jj` does not become an `ok=false` exemption.

**Required invariant test.** The ambiguity message tells the agent to paste the URL back into `company`, and that input goes through `Registry.Resolve` — a first-match poll across all 18 adapters — not through the originating adapter. So the invariant must be stated at the registry level, or a URL can round-trip within its own adapter while a different adapter registered earlier claims it:

```text
for every roster entry, over the production registry:
    url, ok := adapter.CareersURL(slug)
    ok      → Resolve(url) returns the same adapter and the same slug
    !ok     → the entry is in the test's enumerated ok=false set
```

**The sweep must use the production wiring, not its own adapter list.** First-match polling makes the answer depend on registration order, and that order exists in exactly one place — `newATSRegistry` in `cmd/openings-mcp`. A sweep that re-declares the list proves round-tripping under an order production may not use, which is the failure mode the registry-level framing exists to rule out.

Enumeration needs one small extraction, because `newATSRegistry` returns only `*ats.Registry` and `Registry` exports nothing that walks its entries. **In scope for this slice:** pull the adapter literal out of `newATSRegistry` into an unexported `atsAdapters(hc, hcEightfold) ([]ats.Adapter, error)` in `cmd/openings-mcp/main.go`, which `newATSRegistry` then passes to `ats.NewRegistry`. The sweep lives in `cmd/openings-mcp/main_test.go`, iterates that same slice for `Roster()` and `CareersURL`, resolves through a registry built from it, and compares adapter identity by `Name()`.

This deliberately adds **no exported symbol to `internal/ats`** — an enumeration accessor there would be API surface PR 2 owns, added for a test. Reordering the single list in `atsAdapters` must be able to turn the sweep red.

The sweep asserts the enumerated `ok=false` set **exactly** — currently empty — so both a renderer regressing to `ok=false` and a newly added unnamed exemption fail the test rather than being absorbed by it. Workday needs care: two `companies.yaml` rows can share one tenant slug, so the sweep asserts per distinct slug, not per row.

Nothing guarantees any of this today. The two directions were written independently — `fmt.Sprintf` on one side, a regexp on the other — and their formats already differ in detail (trailing slashes on workable and ultipro, `?ss=1` on icims, `/search/` on successfactors against `/search` in the host-pattern table). This whole-roster sweep is cheap and is what makes the disambiguation loop trustworthy.

**Source-breaking.** Adding a method to `ats.Adapter` breaks every implementor and every test stub — `internal/openingsmcp/company_test.go`, `internal/ats/registry_test.go`, and `cmd/verify-companies/main_test.go`. "Additive" refers to runtime behavior, not to compilation.

### PR 2 — Registry and Resolve

Multi-entry `bySlug` / `byName`; `NewRegistry` stops erroring on cross-adapter collisions; union resolve with the ①→② fallthrough; `AmbiguousCompanyError`; the ambiguity message; de-duplicated suggestions and an entry-count in the miss error.

**In scope beyond `internal/ats/registry.go`**, because no other slice claims them:

- `internal/openingsmcp/company.go` — the three `company` parameter descriptions, and `companyDetail`'s ambiguity wrap
- `cmd/openings-mcp/main.go` — `serverInstructions`

All four texts must teach one thing: **when a company is ambiguous, retry with one of the careers URLs listed in the error.** That instruction is what makes the loop terminate in practice — without it the agent has no reason to prefer the listed URL over re-typing the name it started with. Their existing pinned substrings stay asserted.

**Rendering and detection.** `(*ats.AmbiguousCompanyError).Error()` renders the message block verbatim as specified in the Tool contract, so the existing `errorResult` path needs no change. The type is exported from `internal/ats`; `companyDetail` detects it with `errors.As` and appends its previous-key sentence to that text. That `errors.As` branch is the only tool-local handling anywhere — `companySearch` and `companyFilters` propagate unchanged. `Provider` is rendered by neither.

Fixtures:

- unique hit; ambiguous slug; ambiguous name; slug-versus-name cross (`fusion`)
- careers URL wins over an ambiguous bare token
- a candidate whose adapter returns `ok=false`
- filters and detail rejected while ambiguous
- the detail path's ambiguity text contains the previous-key sentence and the search and filters paths' text does not
- an entry whose slug and name normalize to the same key yields exactly **one** candidate
- intra-adapter duplicate slug **and** duplicate name still fail `NewRegistry`, so the nine per-adapter `NewRegistry(oneAdapter)` tests still assert something
- an `ok=false` entry whose normalized name equals another entry's normalized **name** fails `NewRegistry`, and so does one whose normalized name equals another adapter's roster **slug** (point 3)
- a roster slug that is URL-shaped but unparseable by its own adapter (Oracle `<host>/CX_n`, custom-domain Avature) still resolves to its entry
- a miss whose nearest slug is a colliding token: no repeated slug in the suggestions
- a miss against a roster with a duplicated key: the advertised company count equals roster entries, not distinct keys
- the retry instruction is present in `serverInstructions` and in each of the three `company` parameter descriptions

`internal/ats/registry_test.go`'s `TestNewRegistryRejectsDuplicateSlug` asserts today that a cross-adapter duplicate slug errors — the exact behavior this slice inverts. It is rewritten to the intra-adapter form rather than deleted, so the surviving fatal case stays covered.

**Acceptance.** `newATSRegistry` succeeds on the live rosters, `cmd/openings-mcp` goes green, and PR 1's `TestATSCareersURLRoundTripsThroughRegistry` passes — that sweep is PR 1's acceptance and PR 2 is what unblocks it.

The measured "intra-adapter collisions: 0" predates the enrichment batch that introduced `BETA Technologies`, so treat it as unverified until this slice actually builds the registry. If one surfaces, decision point 2 keeps the check fatal and the `companies.yaml` de-duplication lands **inside PR 2**, or in a roster-only PR merged before it — never deferred to PR 3, which would leave PR 2 unable to close and PR 1's sweep red behind it. PR 3 keeps only policy, the restored #252 drops, and the non-fatal report. Both branches — duplicate found or not — end with the production registry building at PR 2's merge commit.

### PR 3 — Roster policy

**Outcome:** the rosters stop being distorted to satisfy a uniqueness rule that no longer exists, and new collisions stay visible without blocking startup.

**Owned files:** `internal/provider/herp/companies.yaml`; the header comments of `internal/provider/{ultipro,icims,avature,oracle}/companies.yaml`; `.agents/skills/{discover-companies,verify-companies,integrate-new-provider}/SKILL.md`; the collision report test, its golden file under `cmd/openings-mcp/testdata/`, and a new `.gitattributes`.

**Non-goals:** no change to `internal/ats` or any adapter; no roster additions beyond the restored rows; PR 1's `ok=false` set stays empty. `f33d617` swapped HRMOS seed companies to dodge HERP collisions — those are different companies rather than dropped ones, so re-sourcing them is discover-companies work, not this slice.

### 1. Restore the dropped HERP rows

`c949038` ("roster: drop HERP slugs that collide across adapters") removed exactly 17 rows from `internal/provider/herp/companies.yaml`. Restore them; a reviewer diffs the result against `git show c949038^:internal/provider/herp/companies.yaml` and expects an exact match on those rows.

They have not been live-checked in roughly 28 commits. Run `go run ./cmd/verify-companies --provider herp` over the restored rows; a row that no longer resolves is dropped again and **named in the PR body**, not re-sourced by hand.

A restored row must clear three live gates, and failing any of them blocks the slice rather than being absorbed: the intra-adapter duplicate check (still fatal), decision point 3's rule for `ok=false` entries, and PR 1's `TestATSCareersURLRoundTripsThroughRegistry`. No restored row may be turned into an `ok=false` exemption.

### 2. Collision report

`NewRegistry` no longer fails, so without a report roster quality degrades silently — decision point 9.

**Form:** a Go test in `cmd/openings-mcp/main_test.go`, over the same `atsAdapters` enumeration PR 1's sweep uses. It runs under the `go test` step CI already has — `cmd/verify-companies` is not a candidate, since it performs live HTTP per entry.

**It keys by `Resolve`, not by a normalizer.** `normalize` is unexported and this slice may not touch `internal/ats`, so the report must not reimplement it — a private copy that drifts would pin a baseline describing collisions the resolver does not actually have. Instead, for every roster entry the report resolves its slug and its display name through the production registry and records a finding whenever either returns an `*ats.AmbiguousCompanyError`. That is exactly the set of inputs a caller experiences as ambiguous, expressed entirely in exported API.

**Golden file:** `cmd/openings-mcp/testdata/company_collisions.txt`. One finding per line:

```text
<probed input>\t<provider>|<display name>|<careers URL>\t<provider>|<display name>|<careers URL>…
```

Every field comes from something already exported: the probed input is the raw slug or display name as it appears in the roster (not a normalized key), and each candidate is `CompanyCandidate`'s `Provider`, `Name` and `CareersURL`. **There is deliberately no slug field** — `CompanyCandidate` does not carry one, and adding it would breach this slice's non-goal against touching `internal/ats`. An empty `CareersURL` renders as an empty field rather than being omitted.

Candidates follow the order `Resolve` returns them, which is adapter registration order. Findings are de-duplicated by probed input and the file is sorted by it, so the two entries of one collision — whose slug probes yield the same input — produce one line, not two, while two genuinely different inputs that happen to share a candidate set stay separate lines.

Records are **LF-terminated**. CI runs `go test ./...` on Windows runners and the repo has no `.gitattributes`, so a `.txt` golden generated with `\n` would be checked out CRLF-converted there and fail a byte comparison for no roster reason. This slice adds a `.gitattributes` pinning `cmd/openings-mcp/testdata/*.txt` to LF. The repo has no golden-file convention yet and `cmd/openings-mcp` has no `testdata/`; this slice establishes both.

**Regeneration:** the report test takes an `-update` flag. `go test ./cmd/openings-mcp -run TestCompanyCollisionReport -update` rewrites the file; on mismatch without it, the test prints the full expected content so the diff is actionable from CI output alone.

**Pass/fail:** the findings are written sorted and compared against a golden file under `cmd/openings-mcp/testdata/`. **Any** difference fails — added or removed — so a class growing while another shrinks cannot cancel out, and reverting the HERP restore cannot leave a baseline quietly admitting seventeen future collisions. Regenerating the golden file is a deliberate edit whose every line shows up in review. The report never blocks startup.

### 3. Curation rules

Four rosters carry a "Display names must stay unique across all ATS rosters" header — ultipro, icims, avature, oracle. Three skills assert the rule for **slugs as well as names**: discover-companies, verify-companies, and integrate-new-provider (which additionally states that `ats.NewRegistry` fails at startup by design).

Replace all of them with the rule that now holds: cross-adapter slug and display-name collisions are allowed and surface as runtime ambiguity; duplicates inside one roster stay a fatal `NewRegistry` error; an entry whose adapter cannot render a careers URL needs a name colliding with no other entry's name or slug.

`discover-companies` also tells contributors that `go test ./...` catches collisions — which after this slice is how they will meet a **red** report on a legitimate addition. Both it and `verify-companies` gain the remedy: when a new company genuinely collides across adapters, regenerate the golden file with `-update` and show its diff in the PR. Without that sentence the slice leaves its own documented workflow broken.

### Acceptance

- `go test ./...` green, explicitly including `TestATSCareersURLRoundTripsThroughRegistry` and the per-adapter `NewRegistry(oneAdapter)` tests
- `go run ./cmd/verify-companies --provider herp` clean over the restored rows, or every failure named in the PR body
- introducing a synthetic cross-adapter collision in any roster turns the report test red, **and so does removing one** — the golden file is compared both ways
- each of the seven named files carries a positive statement of the new rule: the four roster headers (`internal/provider/{ultipro,icims,avature,oracle}/companies.yaml`) and the three skills (`.agents/skills/{discover-companies,verify-companies,integrate-new-provider}/SKILL.md`)
- `discover-companies` and `verify-companies` each name the golden file path and the `-update` step as the remedy for a red report — without this the slice documents `go test ./...` as a gate contributors cannot legitimately clear
- the report test passes on the Windows legs of the existing CI matrix with the golden file committed as generated

Check the curation rewrite per file, not with a line-based grep: `discover-companies` wraps the phrase across a line break ("…globally" / "unique across all adapters…"), so any single-line pattern reports green on text it never inspected. Use `rg -U --multiline` with a whitespace-tolerant pattern, or simply read all seven.

**Rollback**, per part, with everything that must revert alongside it:

| Revert | Must revert with it |
|---|---|
| Curation-rule statements (four roster headers, three skills) | nothing |
| The report (test, golden file, `.gitattributes`) | every skill sentence naming the golden file or `TestCompanyCollisionReport` — the `-update` remedy in discover-companies and verify-companies, and integrate-new-provider's reference to the golden file — all of which would otherwise dangle |
| The HERP restore | the golden file — it records the collisions those rows create, so it must be reverted or regenerated in the same commit |

Only the curation-rule statements are freely independent. The other two couplings are the price of a golden set that fails in both directions, and it is worth paying: the alternative, a bare count, lets one collision class grow while another shrinks.

## Decision checklist, answered

The five questions posed above:

1. **Is ATS invisible non-negotiable?** Yes. B is rejected as primary UX and survives only as an internal error field.
2. **Must both colliding companies stay searchable via roster?** Yes. Both stay indexed; neither is dropped, and neither wins silently.
3. **Are duplicate display names allowed?** Across adapters, yes — relaxed together with slugs, which makes the careers URL the disambiguation key rather than the name. Within one adapter's own roster, no: both name and slug duplicates stay a fatal `NewRegistry` error (point 2). And an entry whose adapter cannot render a careers URL must have a name that collides with no other entry's name *or slug*, so the name-only retry it offers still resolves (point 3).
4. **Is unfiltered fan-out (C2) desirable?** No. It relocates the ambiguity to `get_job_detail_by_company`, where it is harder to handle, and it breaks the single-entry-point property that gives all three tools consistent behavior for free.
5. **How do roster-only adapters participate?** They already can — see the correction above. `ok=false` remains the escape hatch for individual entries, degrading that candidate to its display name.

## Non-goals

- Cross-company fan-out search
- Merged multi-company filter schemas
- A dedicated disambiguation tool — teaching errors and a retry on the same tool are enough
- ATS vendor names anywhere in the user-visible contract
