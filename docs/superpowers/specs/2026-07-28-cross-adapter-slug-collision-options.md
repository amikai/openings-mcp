# Cross-adapter slug collision options

Date: 2026-07-28  
Status: options (not decided)  
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
