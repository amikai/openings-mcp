// Package nodesk reads public job listings from NoDesk
// (https://nodesk.co), a remote-jobs board. NoDesk is a static site with
// no JSON REST API of its own; its search box is Algolia InstantSearch,
// and that Algolia index is the read surface here. The contract was
// reverse-engineered from the site's own JavaScript (2026-07-19); see
// API.md for the recovery steps.
//
// # Search: the Algolia jobPosts index
//
// [Client.Search] POSTs to the standard Algolia query endpoint
//
//	POST {algolia}/1/indexes/jobPosts/query
//
// with the site's public application ID and search-only API key (both
// ship in nodesk.co's /js/search.min.js — they are page-visible by
// design, not secrets). The key is referer-locked: requests must carry a
// nodesk.co Referer header or Algolia answers 403 "Method not allowed
// with this referer". The client always sends it.
//
// This is real server-side search — full-text query, zero-based page,
// hitsPerPage (values above 100 are clamped to 100 by the index), and
// facetFilters over two facets:
//
//   - searchFilter: the site's category paths, e.g.
//     "remote-jobs/engineering", "remote-jobs/full-time",
//     "remote-jobs/golang". One job carries several.
//   - applicantLocationRegions: region labels, e.g. "Remote - Europe",
//     "Worldwide".
//
// [Client.Facets] enumerates both value sets with live job counts.
//
// The index also serves one injected advertisement record (is_ad: true, an
// external CTA with no role, date, or NoDesk job page). It is not a job
// posting, so [Client.Search] drops it from results.
//
// A hit's date field is a display label ("Featured", "Today", "1d"), kept
// in [Job.DateLabel]; datePublished is the real timestamp. Featured
// postings sort first and repeat the label on every page of results.
//
// # Job identity and detail
//
// A hit's permalink is the job page path, /remote-jobs/<slug>/; [Job.ID]
// is that slug. [Client.Detail] fetches the page and reads its schema.org
// JobPosting JSON-LD block (full HTML description, salary, employment
// types) plus the outbound apply link from the page's data-apply-url
// attribute. An unknown or expired slug is a clean HTTP 404.
//
// The JSON-LD applicantLocationRequirements is unreliable boilerplate: it
// says Country "Anywhere" even for postings whose index record restricts
// to a single region (verified against a "Remote - US"-only posting). Use
// the search hit's [Job.Regions] for location instead.
//
// # Verified but deliberately unsupported
//
//   - https://nodesk.co/api/jobs/ does not exist (404); a reference
//     scraper in circulation invents it.
//   - /remote-jobs/index.xml is a real RSS feed but carries only the ~10
//     newest postings — a digest, not the board.
//   - A second Algolia index, geographicRegions, backs the site's region
//     autocomplete; the applicantLocationRegions facet already covers the
//     same values with counts.
//   - Hit fields weight, index, highlight-markup (_highlightResult), and
//     the per-keyword site-navigation URLs are search-UI internals with no
//     job data beyond what [Job] already carries.
package nodesk
