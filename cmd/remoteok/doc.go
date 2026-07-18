// Command remoteok is a debug CLI for Remote OK's public job feed.
//
//	go run ./cmd/remoteok search --tag golang --tag react --format json
//	go run ./cmd/remoteok search --keyword engineer --limit 5
//	go run ./cmd/remoteok detail --id 1134996 --tag golang
//
// The feed only serves the ~100 most recent jobs (per tag set), so detail
// re-fetches the feed and needs the same --tag filter the job was found
// with when that job is outside the unfiltered window.
package main
