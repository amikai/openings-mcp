// Command jobindex is a debug CLI for Jobindex.dk search and detail.
//
//	go run ./cmd/jobindex search --keyword backend --jobage 14 --format json
//	go run ./cmd/jobindex detail --tid h1683131 --format json
//
// JSON output keeps Jobindex Stash field names (tid, headline, hitcount, …).
package main
