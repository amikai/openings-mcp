# engage aggregator search — reverse-engineering notes

`GET https://en-gage.net/user/api/search/result_work_list/` is engage's
cross-tenant job search — the API backing its own site-wide search UI, not
documented anywhere public. Recovered from the search SPA's bundle and
confirmed live while building this package. It is used only by
[Client.Companies] for roster curation; it is **not** the adapter's
per-company Search (see below).

## Why not the adapter's Search

The endpoint's own condition fields — `keyword`, `companyName`, `area`,
`job`, `employ`, `income`, `span`, `page` — were recovered from the SPA
bundle and verified server-side (`keyword=エンジニア` returns 39,784, matching
the UI's live counter). But every parameter name tried for narrowing to one
tenant (`slug`, `company_id`, `work_id`, and several guessed variants) is a
no-op: it returns the identical unfiltered total regardless of value. There
is no way to ask this endpoint for "jobs at tenant X." `companyName` comes
closest but is a fuzzy server-side name match, not a slug filter — for a
multi-slug company like NOVA it over-returns across every tenant sharing
that name (375 results). Compensating for that soft match client-side would
mean silently dropping or misattributing rows, which the repo's standing
rule against server-side soft-filter compensation rules out. So this client
never uses the aggregator as a search path; `Board` (the tenant page) is the
adapter's only Search source, and the 100/category cap it carries is
accepted as-is (see doc.go).

## Response shape

```json
{
  "result": "success",
  "totalCount": 1031581,
  "fromCnt": 1,
  "toCnt": 60,
  "paginator": {"hasMorePages": true, ...},
  "searchResult": [
    {
      "work_id": 13004412,
      "company_id": 352522,
      "company_url_root_dir": "nova_career",
      "company_name": "ＮＯＶＡホールディングス株式会社",
      "official_corporate_name": "ＮＯＶＡホールディングス株式会社",
      ...
    }
  ]
}
```

`searchResult` carries 60 records per page and dozens of fields describing
the job itself; this package reads only the four identity fields above —
`company_url_root_dir` is the tenant slug, verified against the board it
serves for every roster entry.

## Pagination — the p_t/f_t stitch

Paging is stateful, not offset-based:

- `page=1` succeeds with any `p_t` (including it omitted) as long as `f_t=0`
  is present. `page=1` **without** `f_t=0` returns `{"result":"error"}`.
- `page=N` for `N >= 2` requires `p_t` to equal the exact `totalCount`
  observed on page 1 (or whichever page was fetched immediately before).
  `page=2` without `p_t` returns `{"result":"error"}`; `p_t=0` returns
  `{"result":"success", "totalCount":0, ...}` — silently empty, not an
  error, so a client that forgets to carry `p_t` forward gets a page that
  looks valid but carries nothing.

This client's [Client.Companies] takes `prevTotal` as an explicit parameter
rather than hiding it in client state — the same shape as Avature's
`jobOffset` stitch — so the caller (roster tooling) owns the loop and can't
silently drop the value between calls. `totalCount` drifts continuously
(~1.03M and climbing), so a hardcoded `p_t` in a test fixture would break on
replay; the committed hurl fixture captures page 1's `totalCount` and
templates it into page 2's `p_t` rather than hardcoding it.

## Roster discovery

Paging this endpoint harvests verified `slug ↔ name ↔ company_id` triples
without any HTML parsing — the mechanism behind S2's seed roster and any
later bulk expansion via `unverified/engage.yaml`. Two constraints this
surfaced:

- **One company owns many slugs.** `copro-group_saiyo7/9/10/11` are all
  株式会社コプロコンストラクション; `agekke*` are all 株式会社エイジェック.
- **Distinct companies share a display name.** 株式会社アクセル is both
  `accel-qcmd` (company_id 277480) and `axcell_saiyo1` (565578) — a roster
  built from `company_name` alone would collide.
