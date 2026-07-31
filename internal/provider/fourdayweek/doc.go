// Package fourdayweek provides a client for the 4dayweek.io public jobs API.
//
// The package is named for the site rather than spelled after it, since a Go
// identifier cannot start with a digit.
//
// See openapi.yaml for the API surface, the fields the official spec promises
// but never sends, and the response quirks worth knowing before reading a
// listing (salary in cents, Markdown descriptions, remoteness carried only by
// work_arrangement).
package fourdayweek
