// Package herp provides a HERP Career client and curated roster.
//
// HERP Career (herp.careers/careers) is the job media that republishes the
// boards of companies using the HERP Hire ATS. Everything the client needs
// comes from one request per company; the surface, its quirks, and the
// rejected alternative are documented in openapi.yaml.
//
// Searching a HERP board is Japanese-first. Job text — titles, skills,
// descriptions — is Japanese, so a Japanese query matches best, the same
// expectation the Mynavi tools state at their own boundary. Locations are
// the exception: the ATS adapter appends romanized prefecture names to each
// job's searchable location text, so both "東京都" and "Tokyo" find a Tokyo
// posting even though the displayed location stays Japanese.
package herp
