// Package jobindex searches public job listings on Jobindex.dk, Denmark's
// largest commercial job board.
//
// Search is HTML: results live in a `var Stash = {...}` blob on /jobsoegning
// (the former /jobsoegning.json endpoint returns 204). There is no stable
// public search DTO — only reverse-engineered Stash — so search results are
// []map[string]any pass-through bags with light slimming (see SearchResponse
// and slimJobResult for why). Card "html" is dropped; we do not re-parse
// fullcard markup when structured keys already exist.
//
// Detail uses /vis-job/{tid} so the client stays on Jobindex instead of
// following /jobannonce/{tid}'s off-site employer redirects; detail fields are
// scraped HTML with the same key names as search where concepts match — no
// invented merged deadline fields. Detail is a typed struct because we own
// that scrape surface; search is a map because we do not own Stash's shape.
//
// See also: https://github.com/MadsLorentzen/ai-job-search
package jobindex
