// Package hrmos reads public HRMOS 採用 (job posting) listings and posting
// details from https://hrmos.co. HRMOS publishes no official API; this
// surface was reverse-engineered from live traffic against several tenants
// (2026-07-28). Both endpoints are server-side rendered HTML with no auth,
// cookie, header, or fingerprint gate.
//
// # Search: /pages/{slug}/jobs, a full dump
//
// There is no server-side keyword search — ?word=/?keyword=/?q= all return
// byte-identical responses — so a search is a full-dump fetch (Client.
// AllJobs) followed by client-side filtering, one layer up in
// internal/ats. A page carries at most 100 cassettes; p.pg-count's
// "全 N 件中 M 件 を表示しています" gives the true total N. A page past the
// last one is HTTP 200 with zero cassettes and no p.pg-count — but
// nav.pg-pagenation is still present and still lists the real page range,
// which is why AllJobs trusts the pager as its stop condition rather than
// probing for an empty page.
//
// Each cassette (li.pg-list-cassette) already carries the FULL job
// description as flattened plain text, not a truncated summary — unusual
// among this repo's HTML-scrape providers.
//
// # Facet nav: tenant-defined, tenant-optional, and not a function
//
// nav.sg-job-filters holds one table row per tenant-defined facet group
// (moneyforward: 雇用形態/職種/勤務地/求人言語（Language）/職種（詳細）;
// visional: 雇用形態/求人カテゴリー; the hrmos tenant: none — the nav is
// entirely absent). It is tenant-global, not page-scoped: byte-identical
// across every page of a tenant's dump, unlike its hidden per-option job-ID
// lists, which ARE page-scoped and use a different, unrelated numeric ID
// space than cassette URLs — unusable for filtering or counting.
//
// A facet option label does not map to a single group: on moneyforward,
// プロダクトマネージャー is a value under BOTH 職種 and 職種（詳細）. And a
// cassette's own chip can carry no facet option at all (マーケティングアシスタント
// appears on a job but in no group). [internal/ats.HrmosAdapter] resolves
// this by classifying each chip into every group that claims it and
// dropping the rest — see that package for the resulting search semantics.
//
// # Location: the sg-tag-location chip, not the facet groups
//
// Every cassette carries exactly one ul.sg-tags > li.sg-tag-location chip
// (verified across all 298 jobs of five captured tenant pages) holding a full
// street address, sometimes prefixed with a bracketed office label
// (e.g. ［渋谷本社］) and sometimes suffixed with "他(N)" meaning N further
// worksites the list page does not name — those appear only on the detail
// page, so list-derived locations are a lower bound. The chip element is
// always present but its text is not always filled in: employers who left the
// address blank render as <li class="sg-tag-location"></li>, so a job can have
// no location at all and will not match any location search. The list page carries
// no posted date and no remote signal anywhere (zero <time> elements, zero
// リモート/在宅/テレワーク/remote matches in chips or the facet nav across
// every tenant observed); such wording occurs only inside free JD prose.
//
// # Detail: /pages/{slug}/jobs/{id}
//
// The detail page embeds exactly one schema.org JobPosting JSON-LD block,
// stable across every mid-career tenant observed:
// @context @type baseSalary datePosted description employmentType
// hiringOrganization identifier jobLocation title validThrough — notably no
// "url" key. jobLocation is always a list of Place{address: PostalAddress}.
// baseSalary is null when the tenant wrote 応相談 (negotiable) instead of a
// figure, otherwise a full MonetaryAmount{currency, value:{minValue,
// maxValue, unitText}}.
//
// section.pg-descriptions appears twice per detail page: the job's own
// condition table (職種/募集ポジション, 雇用形態, 給与, 勤務地, ...) and the
// employer's 会社情報 table. Client.JobDetail parses both into
// [JobDetailResponse.JobInfo] / [JobDetailResponse.CompanyInfo] so that
// data isn't lost even though [internal/ats.JobDetail] only surfaces a
// subset of it.
//
// # The sonar (新卒) surface has no JSON-LD
//
// HRMOS absorbed the sonar ATS (new-graduate hiring) in late 2025, and those
// postings are already served from these same /pages/{slug}/jobs/{id} URLs —
// e.g. raksul's 【28新卒/内定直結3days】 entry. They use the same page template
// but omit the JobPosting JSON-LD block entirely, so Client.JobDetail reads
// those fields from the surrounding markup instead: the title from
// h1.sg-corporate-name, the description from .pg-markdown, the employer from
// the 会社情報 table's 会社名 row, and the address from the 勤務地 row. Both
// pg-descriptions tables are present and parse normally.
//
// Two traps live here. The LAST sg-breadcrumbs entry is the posting title,
// not the employer — only the first is the company — and each 勤務地 entry
// ends with a "地図で確認" maps link that is not part of the address.
//
// The posting date comes from the #jsi-published-date-start hidden input,
// which carries the same instant as the JSON-LD's datePosted and is present
// on both page variants, so it covers this surface too.
//
// Treating a missing JSON-LD as an error would make every new-graduate
// posting on a mixed tenant unreachable.
//
// Unknown tenant slug and unknown job ID are both a clean HTTP 404.
package hrmos
