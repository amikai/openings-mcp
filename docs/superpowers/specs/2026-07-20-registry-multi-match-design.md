# Registry multi-match and result merging

## Problem

`ats.Registry` keeps two maps (`bySlug`, `byName`) and fails startup when a
normalized slug or display name collides across adapters. This blocks two real
roster situations:

- the same company listed on more than one ATS (e.g. a main board plus a
  regional board on another provider)
- unrelated companies whose slug or name normalize to the same key

`Resolve` also returns a single `(Adapter, slug)`, so even if collisions were
allowed, callers could only ever reach one of the colliding entries.

## Decision summary

- Multi-match is always merged — the registry does not try to distinguish
  "same company on two ATSes" from "two companies sharing a name".
- Data structure: one `map[string][]registryEntry` (approach B below), not a
  composite `{adapter}|{name}|{slug}` key.
- Fan-out tolerates partial failure: failed adapters are skipped; only
  all-fail returns an error.

## Design

### 1. Registry data structure

Replace `bySlug` and `byName` with a single map:

```go
entries map[string][]registryEntry // key: normalize(slug) and normalize(name)
```

Build rules:

- Each roster company is inserted under `normalize(slug)` and
  `normalize(name)`; when the two keys are equal, insert once.
- A key collision appends to the slice instead of failing startup.
- A duplicate slug **within one adapter's roster** still fails startup — that
  is a curation bug in the roster file, unlike a cross-adapter collision.
- Slice order under a key follows build order, i.e. adapter registration
  order, so lookups are deterministic.
- The company count used in the "unknown company" teaching error is
  accumulated during build (can no longer use `len(bySlug)`).

`slugs []slugEntry` (suggestion index) is unchanged.

Example — roster:

| adapter | slug | name |
|---|---|---|
| workday | `nvidia` | NVIDIA |
| smartrecruiters | `nvidia-jp` | NVIDIA |
| lever | `apple-tree` | Apple Tree Learning |

produces:

```go
map[string][]registryEntry{
    "nvidia": {
        {adapter: workday,         slug: "nvidia",    name: "NVIDIA"},
        {adapter: smartrecruiters, slug: "nvidia-jp", name: "NVIDIA"},
    },
    "nvidiajp":          {{adapter: smartrecruiters, slug: "nvidia-jp", name: "NVIDIA"}},
    "appletree":         {{adapter: lever, slug: "apple-tree", name: "Apple Tree Learning"}},
    "appletreelearning": {{adapter: lever, slug: "apple-tree", name: "Apple Tree Learning"}},
}
```

### 2. Resolve signature

```go
type ResolvedCompany struct {
    Adapter Adapter
    Slug    string
}

func (r *Registry) Resolve(company string) ([]ResolvedCompany, error)
```

- Map hit → return every entry under the key. The old slug-before-name
  two-step lookup disappears; a key holds slug-sourced and name-sourced
  entries together, which is exactly the always-merge semantics.
- Careers-URL fallback is unchanged and returns a single-element slice.
- Full miss → the existing teaching error (closest slugs + company count) is
  unchanged.

### 3. Tool merge behavior

All three tools in `internal/openingsmcp/company.go` fan out sequentially over
the resolved entries. Single match — the overwhelmingly common case — behaves
exactly as today.

- **search_jobs_by_company**: call `Search` on each adapter with the same
  params; concatenate `jobs` in entry order, sum `total_count`, take the max
  `total_pages`, pass `page` through.
- **get_company_filters**: union the filter maps; values under the same
  dimension are concatenated, deduplicated, order-preserving.
- **get_job_detail_by_company**: the job_id belongs to exactly one adapter;
  try each in order and return the first success.

Failure policy (all three tools): a failed adapter is skipped; if every
adapter fails, return the errors combined with `errors.Join`.

### 4. Non-goals

- No same-company-vs-collision heuristic — always merge.
- No warnings field for partial failure — successful results are returned
  as-is with no side channel.
- No concurrent fan-out — multi-match is rare and usually 2 entries;
  sequential until proven slow.

### 5. Testing

- Registry: multi-match returns all entries; slug==name inserts once;
  duplicate slug within one adapter still fails startup; careers URL returns
  one entry; teaching error unchanged.
- Company tools, with two fake adapters: search merging (concat / sum / max),
  filters union with dedup, detail first-success, partial failure skipped,
  all-fail returns joined error.
