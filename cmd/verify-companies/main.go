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
	"github.com/amikai/openings-mcp/internal/provider/eightfold"
)

// providerOrder fixes the --provider default and the report's grouping order.
var providerOrder = []string{"ashby", "eightfold", "greenhouse", "lever", "recruitee", "teamtailor", "workday"}

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
		providers = fs.StringLong(
			"provider",
			strings.Join(providerOrder, ","),
			"comma-separated subset of "+strings.Join(providerOrder, ","),
		)
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
		case "eightfold":
			// Eightfold's edge 403s Go's default User-Agent instead of
			// returning JSON, so it gets its own client rather than hc.
			a = ats.NewEightfoldAdapter(&http.Client{Transport: eightfold.BrowserTransport{}})
		case "greenhouse":
			a, err = ats.NewGreenhouseAdapter("https://boards-api.greenhouse.io/v1", hc)
		case "lever":
			a, err = ats.NewLeverAdapter("https://api.lever.co", hc)
		case "recruitee":
			a = ats.NewRecruiteeAdapter(hc)
		case "teamtailor":
			a = ats.NewTeamtailorAdapter(hc)
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
