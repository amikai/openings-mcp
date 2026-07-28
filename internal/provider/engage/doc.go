// Package engage is a client for en-gage.net, エン・ジャパン's free ATS. Every
// tenant gets a hosted careers site at en-gage.net/<slug>/; there is no
// login and no browser required for any surface this package uses. See
// API.md for the cross-tenant aggregator search reverse-engineering.
//
// Surface:
//
//   - GET /<slug>/ — one tenant's board: each employment-type category (e.g.
//     中途採用, アルバイト・パート採用) renders as its own dl.jobList, holding one
//     dt.category label followed by its dd.dataArea listings — a
//     multi-category board carries multiple sibling dl.jobList elements, not
//     one dl with several dt.category children. An unknown slug 404s.
//   - GET /<slug>/work_<id>/ — one posting, carrying exactly one
//     application/ld+json schema.org JobPosting. An unknown id 404s.
//
// Quirks:
//
//   - The board is dump-shaped but capped, not paginable: each dt.category
//     truncates at [CategoryCap] jobs, and ?page/?p/?offset/?limit are all
//     no-ops returning a byte-identical id set (verified by hashing the
//     sorted id list). The truncation is a contiguous id window — jobs exist
//     outside it once a category exceeds the cap (measured on cookbiz_jobs:
//     its board covers a ~350-id range while the aggregator returns six
//     cookbiz_jobs ids entirely outside that window, all resolving 200).
//     [Category.AtCap] surfaces this per category so callers can present
//     board counts as a lower bound instead of an exact total.
//   - A 200 board that parses to zero jobs is [ErrEmptyBoard], not an empty
//     result. 641 slugs drawn from the company sitemap were fetched live:
//     every one returned 200 with dl.jobList present and at least one job.
//     The sitemap only enumerates tenants with a published posting, so a
//     genuinely job-less tenant isn't reachable through this surface at all
//     — a 200 with zero parsed jobs means selector drift.
//   - Detail pages render every description sub-section (仕事内容, 応募資格・条件,
//     募集人数・募集背景, 勤務地, 勤務時間, 給与, 休日休暇, 福利厚生, ...) both inside the
//     JSON-LD's single "description" HTML string and, separately, as
//     h2.item / dd.data blocks in the surrounding page. [JobDetail.Sections]
//     exposes the latter for callers that want one section in isolation
//     rather than re-splitting Description.
//   - baseSalary is a single MonetaryAmount object on most postings, but an
//     array of them on tenants that publish both an annual and a monthly
//     figure for the same posting (observed on cookbiz_jobs). Its
//     minValue/maxValue are JSON strings, not numbers, and maxValue is
//     frequently absent (open-ended pay). See [JobDetail.Salaries].
//   - hiringOrganization.sameAs (the employer's own site) is present on
//     larger tenants but absent on small storefronts with no company URL on
//     file.
package engage
