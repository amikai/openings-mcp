# Plan v2: move the workable CLI to `internal/cli/provider/workable` (urfave/cli v3, JSON only)

Revision of plan v1 after a plan-verifier REVISE (5 blockers) plus empirical
verification of urfave/cli v3.10.1 against a scratch module.

## Scope

- Only the workable provider. The other 47 `cmd/*` binaries stay on ff/v4, untouched.
- Keep four subcommands: `companies` / `search` / `get` / `filters`.
- Do not touch `internal/provider/workable` (client, roster, mocksrv are reused
  as-is), do not touch the MCP server.

## 0. Verified facts about urfave/cli v3.10.1

These were checked empirically in a throwaway module (`go get
github.com/urfave/cli/v3@v3.10.1`, then a program exercising each case), not
assumed. They are recorded here because plan v1 got one of them wrong.

- `v3.10.1` exists and is the latest release.
- **There is no `Persistent` field.** `FlagBase` has `Local bool` —
  "whether the flag needs to be applied to subcommands as well". Root flags are
  inherited by subcommands **by default**; `Local: true` opts *out*. Plan v1's
  `Persistent: true` would not compile.
- Root flags are readable from a subcommand action via `cmd.String("company")`,
  and accepted both before and after the subcommand name
  (`workable --company X search` and `workable search --company X` both work).
- `cmd.IsSet("remote")` correctly distinguishes unset / `--remote=false` /
  `--remote`, so a `BoolFlag` does carry the tri-state.
- `cmd.Root().Writer` exists (`Command.Writer io.Writer`) and is honored;
  `Command.ErrWriter` is separate, so tests must set both to capture everything.
- `cmd.Run(ctx, osArgs)` takes the full `os.Args` (including argv[0]).
- `Required: true` works and produces `Required flag "shortcode" not set`.
- `FlagBase.Validator func(T) error` exists and works, but a validator failure
  prints `Incorrect Usage: invalid value ... for flag -workplace: ...` **plus a
  full help dump**. See decision D3.
- **Stray positional arguments are silently ignored.** `workable get B02DA69C8F`
  does not error on the unconsumed arg. See decision D1.
- **A root command with no `Action` and no subcommand prints help and returns
  `nil`** → exit 0. See decision D2.

## 1. Dependency

`go get github.com/urfave/cli/v3@v3.10.1`. `peterbourgon/ff/v4` stays in go.mod
(the other cmds use it); `jaytaylor/html2text` stays too (other cmds use it),
workable just stops importing it.

## 2. File layout

```
internal/cli/provider/workable/
  doc.go       package doc
  cli.go       NewCommand() *cli.Command — root + 4 subcommands + flag definitions
  run.go       normalizeCompany, newClient, the four action implementations
  summary.go   jobSummaryJSON / searchResultJSON / jobURL / summarize / encodeJSON
  cli_test.go  unit + end-to-end tests
  testdata/    golden JSON for the e2e assertions (see section 5)
cmd/workable/main.go   ~20 lines, only Run + exit code
```

Package name `workable`; inside it the API package is imported aliased as
`provider "github.com/amikai/openings-mcp/internal/provider/workable"`.

`cmd/workable/main.go`:

```go
// Command workable is a debug CLI for the Workable job board API.
package main

func main() {
	if err := workablecli.NewCommand().Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}
```

Repo convention: `cmd/<name>` gets no doc.go; the command comment sits atop main.go.

## 3. Flag changes

Root flags — inherited by subcommands by default, so **no `Local` field is set
on any of them**:

| current | new | note |
|---|---|---|
| `--company` | `--company` / `-c` | unchanged semantics |
| `--timeout` | `--timeout` | unchanged, default 60s |
| `--format text\|json` | **deleted** | always JSON |
| — | `--base-url` (`Hidden: true`, default `https://apply.workable.com`) | new; lets tests point at mocksrv, also useful for debugging |

`search`:

| current | new | reason |
|---|---|---|
| `--keyword` | `--keyword` / `-q` | short alias |
| `--country` / `--region` / `--city` | unchanged | |
| `--department` (int) | `--department-id` (int) | the name should say it takes a numeric facet id, not a display name |
| `--workplace` | `--workplace` | still restricted to `on_site\|hybrid\|remote` |
| `--worktype` | `--worktype` | unchanged |
| `--remote` (string `"true"`/`"false"`) | `--remote` (**BoolFlag**) | tri-state recovered via `cmd.IsSet("remote")` (verified) |
| `--token` | `--page-token` | it is the nextPage cursor, not an auth token |

`get`: `--shortcode` kept, marked `Required: true`.
`companies` / `filters`: no own flags.

## 4. Decisions the executor must not have to make

### D1 — positional arguments: keep rejecting them

Today all four subcommands reject stray positional args
(`cmd/workable/main.go:45-47, 71-73, 99-102, 114-117`). urfave/cli v3 ignores
them silently, so the guard must be reimplemented or the behavior is lost.

Decision: **keep it.** A shared helper in `run.go`, called as the first
statement of every action:

```go
// rejectArgs mirrors the ff/v4 CLI's refusal to silently ignore positional
// arguments: urfave/cli v3 accepts and discards them.
func rejectArgs(cmd *cli.Command) error {
	if cmd.NArg() == 0 {
		return nil
	}
	return fmt.Errorf("%s takes no positional arguments, got %v", cmd.Name, cmd.Args().Slice())
}
```

The `get` subcommand's extra hint (`did you mean --shortcode %q?`) and the
`search` hint (`did you forget a flag name?`) are **dropped**; one uniform
message replaces the four bespoke ones. This is an accepted, deliberate
downgrade in error-message helpfulness, not an oversight.

### D2 — root with no subcommand: keep exiting 1

Today `workable --company blueground` prints help plus
`err: a subcommand (companies, search, get, or filters) is required` and exits 1
(`cmd/workable/main.go:132-136`). v3 would print help and exit 0.

Decision: **keep exiting 1.** The root command gets an explicit `Action` that
calls `cli.ShowAppHelp(cmd)` and then returns
`errors.New("a subcommand (companies, search, get, or filters) is required")`.
`main.go` prints it and exits 1, preserving today's contract.

### D3 — flag validation stays in the action bodies

`Validator` is available but dumps the full help text on failure and reformats
the message as `Incorrect Usage: invalid value ...`.

Decision: **do not use `Validator`.** Keep `--workplace` value-set and
`--department-id >= 0` checks at the top of `runSearch`, reusing today's exact
error strings with only the flag name updated:

- `--workplace must be on_site, hybrid, or remote, got %q`
- `--department-id must be a positive facet id, got %d`

`--company` required-and-in-roster stays in `normalizeCompany`, message
unchanged: `company %q not found; run 'workable companies' to see supported companies`.
`--shortcode` is the one exception: it moves to `Required: true`, so its error
text changes from `--shortcode is required (take it from a search result's
Shortcode)` to urfave's `Required flag "shortcode" not set`. Accepted.

## 5. Behavior changes

- Output always JSON on stdout, 2-space indent. All text branches and
  human-facing lines (`Workable Jobs Report`, `Next page: --token X`) removed;
  `nextToken` is already in the JSON.
- Output goes to `cmd.Root().Writer` instead of `os.Stdout` so tests can capture it.
- Deleted code: `printSummary`, the text branch of `printDetail`,
  `printSection`, the `format` flag and the `format` fields of
  `searchFlags`/`getFlags`, the html2text import. `printDetail` collapses to a
  single encode call.
- JSON shapes stay identical: `companies` encodes `provider.Companies`;
  `filters` encodes `FiltersResponse`; `get` encodes `JobDetail` (raw HTML
  fields included); `search` keeps `{total, jobs[], nextToken}` and
  `summarize()`'s existing composition (location display fallback, department
  join, published → `2006-01-02`).

## 6. Tests

`cmd/workable` has no test today. All new tests live in
`internal/cli/provider/workable/cli_test.go`.

**Unit**
- `TestSummarize`: location `display` present; `display` absent → rebuilt from
  city/region/country; department join; `published` RFC3339 → `2006-01-02`;
  unparseable `published` → passed through verbatim.
- `TestNormalizeCompany`: unknown company → the `not found` message; mixed-case
  input → roster's stored account casing.

**End-to-end** — build the command with `NewCommand()`, set both `Writer` and
`ErrWriter` to a `bytes.Buffer`, run against `provider.NewMockServer()` via
`--base-url`.

Expected output source, per subcommand (this is what makes the assertions
non-tautological — the expectation is a checked-in literal, so changing a struct
tag or a `summarize` field fails the test):

| case | expectation |
|---|---|
| `search` (no filters) | golden file `testdata/search_golden.json`, `assert.JSONEq` |
| `search -q engineer` | golden file `testdata/search_keyword_golden.json` |
| `search --page-token <provider.MockPage2Token>` | golden file `testdata/search_page2_golden.json` |
| `get --shortcode <fixture shortcode>` | golden file `testdata/get_golden.json` |
| `filters` | golden file `testdata/filters_golden.json` |
| `companies` | **no golden file** — the roster is ~3900 entries. Assert instead: output is a JSON array, its length equals `len(provider.Companies)`, and its first element equals the literal `{"company": ..., "account": ...}` of `provider.Companies[0]`. |

> **Revision, post-implementation.** The first implementation of the `companies`
> assertion decoded the output back into `provider.RosterCompany` and compared it
> to `provider.Companies[0]`. An outcome verifier refuted it: encode and decode
> share the same struct tags, so renaming `json:"company"` shifted both sides
> together and the test still passed — exactly the tautology this row exists to
> prevent.
>
> The assertion now decodes into `[]map[string]string` and compares against a
> map whose **keys** are literals (`"company"`, `"account"`) and whose values
> come from `provider.Companies[0]`. This keeps the property the row is after — a
> json-tag rename fails the test, proven by mutation — while not pinning the test
> to a specific roster row, so adding a company that sorts first does not break
> it. That is a deliberate refinement of this row's original "first element
> equals the literal" wording, not a shortcut around it.

Golden files are authored by hand-checking the first run's output against the
fixture in `internal/provider/workable/testdata/`, not by blindly recording it.

**Error cases**
- unknown shortcode → `job %q not found for company %q`
- `--company <not in roster>` → the `not found` message. Note: this is a
  **roster-rejection** test, not an upstream-404 test. `provider.MockUnknownCompany`
  is deliberately absent from `companies.yaml`, so `normalizeCompany` rejects it
  before any HTTP call and `mocksrv.go`'s unknown-account handler is unreachable
  from this CLI. Consequently the `res.(*SearchResponse)` / `res.(*FiltersResponse)`
  type-assertion fallthrough (`company %q not found on Workable (account removed?)`)
  **stays untested**, exactly as it is today. Stated, not silently skipped.
- `--workplace bogus`, `--department-id -1`, missing `--company`, missing
  `--shortcode`
- stray positional arg on `get` and on one other subcommand → D1's message
- root with no subcommand → D2's error

**Commands**

```
go build ./... && go test ./internal/cli/... ./cmd/... && go vet ./...
```

plus a manual smoke: `go run ./cmd/workable --company blueground search | jq .`

## 7. Known trade-offs / open items

- go.mod carries both ff/v4 and urfave/cli v3 until (or unless) the remaining
  cmds migrate. Accepted cost of the pilot.
- `.agents/skills/integrate-new-provider/SKILL.md:141` still mandates ff/v4 for a
  new provider's debug CLI. **Not changed in this pass**; revisit after the pilot
  lands.
- Breaking changes for anyone scripting the CLI: `--format text` gone, `--token`
  → `--page-token`, `--department` → `--department-id`, and the `--shortcode` /
  positional-arg error texts change. Acceptable for a debug CLI.

## 8. Changes made after the plan landed

These were decided in review, after the implementation was merged. Recorded here
so the sections above are not read as the current state.

- **One file per subcommand.** `cli.go` holds only `NewCmd`; `companies.go`,
  `search.go`, `detail.go`, and `filters.go` each own their flags and action;
  the cross-command helpers live in `helper.go`. `run.go` and `summary.go` are
  gone. `NewCommand` → `NewCmd`, `<x>Command` → `<x>Cmd`.
- **`apiBaseURL` moved to the provider package** as the exported
  `provider.DefaultBaseURL` (`internal/provider/workable/client.go`), following
  `internal/provider/mokahr`. Exported because the CLI is now a different
  package.
- **`--company` is per subcommand, not a root flag.** `companies` takes no
  account and should not advertise the flag. Consequence: `workable --company X
  search` no longer parses — the flag must follow the subcommand
  (`workable search --company X`). `--timeout` and `--base-url` deliberately
  stay on the root.
- **Short flag names dropped.** `-c` and `-q` are gone; long names only. The
  built-in `-h` remains, since removing it means mutating urfave's package-level
  `cli.HelpFlag`.
- **`companies` prints slugs, not JSON.** It emits one account slug per line so
  the output feeds straight into `--company`; the display names and the rest of
  companies.yaml were noise for that purpose. This makes `companies` the one
  exception to §5's "output always JSON", and retires the bounded-JSON
  assertion the revision note above describes — the test now asserts line count
  and exact line content instead.
- **`get` → `detail`.** `detail` says what comes back, matches the 24 other
  commands in `cmd/` that already use it (against 11 on `get`), matches this
  provider's own original design in
  `docs/superpowers/plans/2026-07-18-workable-ats-adapter.md`, and lines up with
  the MCP tool name `get_job_detail`. D2's error string is therefore now
  `a subcommand (companies, search, detail, or filters) is required`.

- **Help metadata corrected.** The port had mapped ff's `Usage` (a usage line)
  onto urfave's `Usage`, which is a *short description* — so the COMMANDS list
  repeated each command's own invocation and the four `ShortHelp` descriptions
  were lost. Usage lines now live in `UsageText` and the descriptions are back
  in `Usage`. `--remote`'s help text, still reading `true or false` from when it
  was a string flag, was rewritten for the bool tri-state.
- **The CLI stopped reshaping what it prints.** `summarize()` is gone: it picked
  seven fields out of each search result, joined department, reformatted the
  date, and synthesized a `url` the API never returns — while `detail` and
  `filters` passed their responses straight through, so the list view carried a
  URL the single view did not. That inconsistency was the tell that the shaping
  was product work, which belongs in the MCP layer, not in a tool whose job is
  showing what upstream sent. `search` now emits `SearchResponse` as-is.

  Deliberately *not* done: dumping the raw HTTP body. It was measured — the
  ogen-generated decoder silently drops any field `openapi.yaml` does not
  declare, so going through the client is lossy — but bypassing the client would
  show fields the provider package can never return, breaking the correspondence
  between what this CLI prints and what the MCP server can serve. Spec drift is
  the hurl fixtures' job.
- **Output is compact, not indented.** `SetIndent` is gone on the same grounds:
  pretty-printing is a presentation choice, and `jq` makes it better. With that,
  `encodeJSON` wrapped a single line and was inlined into its three call sites.
- **The golden files are gone.** Once `search` stopped reshaping, all five were
  byte-for-byte copies of the hurl fixtures in
  `internal/provider/workable/testdata` that `NewMockServer` already replays —
  verified by asserting equality before deleting them. The tests read those
  fixtures directly, so the expectation is bytes recorded from Workable rather
  than bytes produced by the code under test, and the assertion is now the
  property the CLI is built around: output equals the API response.
- **`cmd/workable` became `cmd/openings-cli workable`.** One binary with a
  subcommand per provider, rather than a binary and a `main.go` per provider;
  the next provider is one line. `cmd/workable` was removed rather than kept as
  a second way to run the same code.

  Mounting the command under a parent broke three things, each found by running
  it: `cli.ShowAppHelp` prints the *outermost* command's help, so
  `openings-cli workable` described the wrong command (now
  `ShowSubcommandHelp`); `UsageText` is printed verbatim, with urfave prefixing
  the parent chain onto the NAME line but not onto USAGE; and two user-facing
  strings still told the reader to run `workable companies` /
  `workable filters`. `--timeout` and `--base-url` needed nothing — on the
  intermediate command they still reach the leaf, from either side of it.
  `TestMounted` covers the arrangement, since every other test drives `NewCmd()`
  standalone and the two differ in what `cmd.Root()` resolves to.
