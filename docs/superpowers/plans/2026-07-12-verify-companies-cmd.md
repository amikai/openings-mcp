# verify-companies cmd Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A standalone CLI `cmd/verify-companies` that verifies every curated companies.yaml entry by running a real search through the unified `internal/ats` adapters and reports each entry's total job count.

**Architecture:** One `main.go`. Adapters are constructed with the same base URLs as `cmd/openings-mcp/main.go`; rosters come from `Adapter.Roster()`, verification calls `Adapter.Search(slug, page 1)`, a bounded worker pool fans out, and results print as text or JSON with exit code 1 (any ERROR) / 0 (all OK).

**Tech Stack:** Go 1.26, `ff/v4` (repo's CLI convention), `internal/ats`.

Spec: `docs/superpowers/specs/2026-07-12-verify-companies-cmd-design.md`

## Global Constraints

- No test file — per user decision, `cmd/verify-companies/main.go` only.
- Do NOT git commit — the user commits manually (standing preference overrides this skill's commit steps).
- Classification is binary: Search success → OK with `TotalCount`; any Search error → ERROR with the error message as detail.
- The cmd imports `internal/ats` but no `internal/provider/*` package.

---

### Task 1: cmd/verify-companies/main.go

**Files:**
- Create: `cmd/verify-companies/main.go`

**Interfaces:**
- Consumes: `ats.NewLeverAdapter(baseURL, *http.Client)`, `ats.NewAshbyAdapter(baseURL, *http.Client)`, `ats.NewGreenhouseAdapter(baseURL, *http.Client)` (all return `(*XxxAdapter, error)`), `ats.NewWorkdayAdapter(*http.Client)`; `ats.Adapter` (`Name() string`, `Roster() []ats.CompanyInfo`, `Search(ctx, slug string, ats.SearchParams) (*ats.SearchResult, error)`); `ats.CompanyInfo{Slug, Name string}`; `ats.SearchResult.TotalCount int`.
- Produces: the `verify-companies` binary; nothing consumes it.

- [ ] **Step 1: Write `cmd/verify-companies/main.go`**

```go
// Command verify-companies verifies every curated companies.yaml entry by
// running a real search through the unified internal/ats adapters — the
// same code path the MCP server serves — and reports each entry's total
// job count. See
// docs/superpowers/specs/2026-07-12-verify-companies-cmd-design.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/ats"
)

// providerOrder fixes the --provider default and the report's grouping order.
var providerOrder = []string{"ashby", "greenhouse", "lever", "workday"}

// Result statuses. ERROR covers every failed check — a stale identifier
// (upstream 404) and a transient failure (timeout, 5xx) alike; Detail
// carries the error message for telling them apart.
const (
	statusOK    = "OK"
	statusError = "ERROR"
)

// check is one roster entry to verify against its adapter.
type check struct {
	adapter ats.Adapter
	company string
	slug    string
}

// result is one classified check: OK carries the company's total job
// count; ERROR carries the error message in Detail.
type result struct {
	Provider string `json:"provider"`
	Company  string `json:"company"`
	Slug     string `json:"slug"`
	Status   string `json:"status"`
	Jobs     int    `json:"jobs"`
	Detail   string `json:"detail,omitempty"`
}

func main() {
	fs := ff.NewFlagSet("verify-companies")
	var (
		providers   = fs.StringLong("provider", strings.Join(providerOrder, ","), "comma-separated subset of ashby,greenhouse,lever,workday")
		timeout     = fs.DurationLong("timeout", 300*time.Second, "per-request timeout")
		concurrency = fs.IntLong("concurrency", 8, "number of concurrent checks")
		format      = fs.StringEnumLong("format", "output format", "text", "json")
	)

	var errorCount int
	cmd := &ff.Command{
		Name:  "verify-companies",
		Usage: "verify-companies [--provider LIST] [--timeout D] [--concurrency N] [--format text|json]",
		Flags: fs,
		Exec: func(ctx context.Context, args []string) error {
			var err error
			errorCount, err = run(ctx, *providers, *timeout, *concurrency, *format)
			return err
		},
	}

	if err := cmd.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(cmd))
		if errors.Is(err, ff.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}

	if err := cmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
	if errorCount > 0 {
		os.Exit(1)
	}
}

func run(ctx context.Context, providerList string, timeout time.Duration, concurrency int, format string) (errs int, err error) {
	names, err := parseProviders(providerList)
	if err != nil {
		return 0, err
	}
	if concurrency < 1 {
		return 0, fmt.Errorf("--concurrency must be at least 1, got %d", concurrency)
	}

	adapters, err := buildAdapters(names)
	if err != nil {
		return 0, err
	}
	results := runChecks(ctx, buildChecks(adapters), timeout, concurrency)

	if format == "json" {
		err = printJSON(results)
	} else {
		printText(results)
	}
	_, errs, _ = tally(results)
	return errs, err
}

// parseProviders validates the --provider list and returns it in
// providerOrder so the report grouping is stable regardless of input order.
func parseProviders(list string) ([]string, error) {
	selected := map[string]bool{}
	for name := range strings.SplitSeq(list, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		if !slices.Contains(providerOrder, name) {
			return nil, fmt.Errorf("unknown provider %q (want any of %s)", name, strings.Join(providerOrder, ", "))
		}
		selected[name] = true
	}
	var names []string
	for _, name := range providerOrder {
		if selected[name] {
			names = append(names, name)
		}
	}
	return names, nil
}

// buildAdapters constructs the selected adapters with the same base URLs
// cmd/openings-mcp/main.go uses.
func buildAdapters(names []string) ([]ats.Adapter, error) {
	hc := &http.Client{}
	var adapters []ats.Adapter
	for _, name := range names {
		var (
			a   ats.Adapter
			err error
		)
		switch name {
		case "ashby":
			a, err = ats.NewAshbyAdapter("https://api.ashbyhq.com", hc)
		case "greenhouse":
			a, err = ats.NewGreenhouseAdapter("https://boards-api.greenhouse.io/v1", hc)
		case "lever":
			a, err = ats.NewLeverAdapter("https://api.lever.co", hc)
		case "workday":
			a = ats.NewWorkdayAdapter(hc)
		}
		if err != nil {
			return nil, fmt.Errorf("build %s adapter: %w", name, err)
		}
		adapters = append(adapters, a)
	}
	return adapters, nil
}

// buildChecks flattens the adapters' rosters into checks, in roster order.
func buildChecks(adapters []ats.Adapter) []check {
	var checks []check
	for _, a := range adapters {
		for _, c := range a.Roster() {
			checks = append(checks, check{adapter: a, company: c.Name, slug: c.Slug})
		}
	}
	return checks
}

// runChecks executes checks through a worker pool of size concurrency and
// returns results in check order.
func runChecks(ctx context.Context, checks []check, timeout time.Duration, concurrency int) []result {
	results := make([]result, len(checks))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = c.do(ctx, timeout)
		})
	}
	wg.Wait()
	return results
}

// do searches page 1 for the entry and classifies the outcome.
func (c check) do(ctx context.Context, timeout time.Duration) result {
	r := result{Provider: c.adapter.Name(), Company: c.company, Slug: c.slug}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := c.adapter.Search(ctx, c.slug, ats.SearchParams{Page: 1})
	if err != nil {
		r.Status, r.Detail = statusError, err.Error()
		return r
	}
	r.Status = statusOK
	r.Jobs = res.TotalCount
	return r
}

// printText writes one line per entry plus a summary. Jobs is shown only
// for OK entries; Detail only for ERROR entries, where it explains the
// failure.
func printText(results []result) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range results {
		jobs, detail := "", r.Detail
		if r.Status == statusOK {
			jobs, detail = strconv.Itoa(r.Jobs), ""
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Status, r.Provider, r.Company, r.Slug, jobs, detail)
	}
	w.Flush()
	ok, errs, zero := tally(results)
	fmt.Printf("\ntotal %d: ok %d, error %d, zero-job %d\n", len(results), ok, errs, zero)
}

func printJSON(results []result) error {
	ok, errs, zero := tally(results)
	out := struct {
		Results []result       `json:"results"`
		Summary map[string]int `json:"summary"`
	}{
		Results: results,
		Summary: map[string]int{"ok": ok, "error": errs, "zero_job": zero},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// tally counts results per status; zero counts the OK entries whose board
// is live but currently lists no jobs.
func tally(results []result) (ok, errs, zero int) {
	for _, r := range results {
		if r.Status == statusOK {
			ok++
			if r.Jobs == 0 {
				zero++
			}
			continue
		}
		errs++
	}
	return ok, errs, zero
}
```

- [ ] **Step 2: Format, vet, build**

Run:
```bash
gofmt -l cmd/verify-companies && go vet ./cmd/verify-companies && go build ./cmd/verify-companies
```
Expected: no gofmt output, vet and build succeed.

- [ ] **Step 3: Smoke run against the smallest roster (lever, 20 entries)**

Run:
```bash
go run ./cmd/verify-companies --provider lever
```
Expected: 20 lines `OK  lever  <company>  <slug>  <jobs>`, summary `total 20: ok 20, error 0, zero-job <n>`, exit code 0 (`echo $?`).

- [ ] **Step 4: Verify JSON format and flag validation**

Run:
```bash
go run ./cmd/verify-companies --provider lever --format json | head -20
go run ./cmd/verify-companies --provider nope; echo "exit=$?"
```
Expected: JSON object with `results` (each result has a `jobs` field) and `summary` (`ok`/`error`/`zero_job`); second command prints `err: unknown provider "nope" ...` and `exit=1`.

- [ ] **Step 5: Full sweep across all four providers**

Run:
```bash
go build ./cmd/verify-companies && ./verify-companies; echo "exit=$?"
```
Expected: ~353 lines with job counts (the workday roster's two shared-tenant duplicate rows are deduped by `Roster()`). Exit 0 if everything is live; exit 1 with ERROR lines otherwise — stale identifiers show upstream 404/not-found messages (that output is the audit deliverable for issue #91); rerun once if errors look transient.

- [ ] **Step 6: Do NOT commit**

Leave changes uncommitted; the user commits manually.

## Self-review notes

- Spec coverage: adapter-based verification, job counts, structure (ats-only imports), all four flags, text/JSON output with zero-job summary, exit codes 0/1 — all in Task 1.
- No placeholders; single task because the spec is one self-contained file.
- Signatures checked against `internal/ats`: constructor arities, `Adapter` methods, `CompanyInfo{Slug, Name}`, `SearchResult.TotalCount`.
