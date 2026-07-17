// Package jobindex searches public job listings on Jobindex.dk, Denmark's
// largest commercial job board.
//
// Search is HTML: results live in a `var Stash = {...}` blob on /jobsoegning
// (the former /jobsoegning.json endpoint returns 204). Detail uses /vis-job/{tid}
// so the client stays on Jobindex instead of following /jobannonce/{tid}'s
// off-site employer redirects.
//
// Reference implementation: case-study/ai-job-search jobindex-search skill.
package jobindex
