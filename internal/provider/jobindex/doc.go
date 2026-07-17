// Package jobindex searches public job listings on Jobindex.dk, Denmark's
// largest commercial job board.
//
// Search is HTML: results live in a `var Stash = {...}` blob on /jobsoegning
// (the former /jobsoegning.json endpoint returns 204). The client returns that
// searchResponse payload with upstream field names (tid, headline, company,
// …), stripping only each result's card "html" markup. Detail uses
// /vis-job/{tid} so the client stays on Jobindex instead of following
// /jobannonce/{tid}'s off-site employer redirects; detail fields are scraped
// HTML with the same key names as search where concepts match — no invented
// merged deadline fields.
package jobindex
