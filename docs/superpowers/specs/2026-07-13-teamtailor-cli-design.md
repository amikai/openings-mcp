# Teamtailor Debug CLI — Design

## Goal

Add `cmd/teamtailor`, a live diagnostic CLI matching the other ATS provider
commands. It is for provider debugging and roster checks, not the MCP surface.

## Command tree

```text
teamtailor companies [--format text|json]
teamtailor --host HOST search [--keyword TEXT] [--format text|json]
teamtailor --host HOST get --id ITEM-ID [--format text|json]
```

Global `--timeout` defaults to 60 seconds. `--host` must name a curated roster
host; `companies` is the discovery path. Every command rejects stray positional
arguments. Search and get fetch `https://<host>/jobs.json` once; search filters
titles case-insensitively and get finds the JSON Feed item UUID. Both full-dump
operations are client-side because the upstream has no parameters or detail
endpoint.

Text search output is compact (title, location, date, item ID, URL). JSON has a
stable wrapper with total count and summaries. Get converts `content_html` to
plain text and emits the full posting.

## Tests

Unit tests cover missing/unknown host, missing ID, and argument/pagination-style
validation without making live requests. Provider fixture tests cover decoding.
