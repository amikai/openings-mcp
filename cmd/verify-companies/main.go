// Command verify-companies verifies every curated companies.yaml entry by
// running a real search through the unified internal/ats adapters — the
// same code path the MCP server serves — and reports each entry's total
// job count. Each successful search is followed by one Detail probe on a
// sampled job, so detail-template divergence surfaces here rather than at
// release smoke testing. See
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
	"github.com/amikai/openings-mcp/internal/provider/mokahr"
)

// providerOrder fixes the --provider default and the report's grouping order.
var _providerOrder = []string{
	"adp_myjobs",
	"ashby",
	"avature",
	"bamboohr",
	"dayforce",
	"eightfold",
	"engage",
	"greenhouse",
	"herp",
	"hrmos",
	"icims",
	"join",
	"lever",
	"mokahr",
	"oracle",
	"recruitee",
	"rippling",
	"smartrecruiters",
	"successfactors",
	"teamtailor",
	"ultipro",
	"workable",
	"workday",
}

// Result statuses. ERROR covers every failed search — a stale identifier
// (upstream 404) and a transient failure (timeout, 5xx) alike.
// DETAIL_ERROR means the search succeeded but the follow-up Detail probe
// on a sampled job failed — usually a detail-template divergence the
// search path never exercises. Detail carries the error message either way.
const (
	_statusOK          = "OK"
	_statusError       = "ERROR"
	_statusDetailError = "DETAIL_ERROR"
)

// check is one roster entry to verify against its adapter.
type check struct {
	adapter ats.Adapter
	company string
	slug    string
}

// result is one classified check: OK and DETAIL_ERROR carry the company's
// total job count; ERROR and DETAIL_ERROR carry the error message in
// Detail.
type result struct {
	Provider string `json:"provider"`
	Company  string `json:"company"`
	Slug     string `json:"slug"`
	Status   string `json:"status"`
	Jobs     int    `json:"jobs"`
	Detail   string `json:"detail,omitempty"`
}

func main() {
	os.Exit(runMain())
}

func runMain() int {
	fs := ff.NewFlagSet("verify-companies")
	var (
		providers = fs.StringLong(
			"provider",
			strings.Join(_providerOrder, ","),
			"comma-separated subset of "+strings.Join(_providerOrder, ","),
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
			return 0
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}

	if err := cmd.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}
	if errorCount > 0 {
		return 1
	}
	return 0
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
	selected := make(map[string]bool)
	for name := range strings.SplitSeq(list, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		if !slices.Contains(_providerOrder, name) {
			return nil, fmt.Errorf("unknown provider %q (want any of %s)", name, strings.Join(_providerOrder, ", "))
		}
		selected[name] = true
	}
	var names []string
	for _, name := range _providerOrder {
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
		case "adp_myjobs":
			a = ats.NewADPMyJobsAdapter(hc, nil)
		case "ashby":
			a, err = ats.NewAshbyAdapter("https://api.ashbyhq.com", hc, nil)
		case "avature":
			a = ats.NewAvatureAdapter(hc)
		case "bamboohr":
			a = ats.NewBambooHRAdapter(hc, nil)
		case "dayforce":
			a, err = ats.NewDayforceAdapter("https://jobs.dayforcehcm.com", hc)
		case "eightfold":
			// Eightfold's edge 403s Go's default User-Agent instead of
			// returning JSON, so it gets its own client rather than hc.
			a = ats.NewEightfoldAdapter(&http.Client{Transport: eightfold.BrowserTransport{}})
		case "engage":
			a = ats.NewEngageAdapter(hc, nil)
		case "greenhouse":
			a, err = ats.NewGreenhouseAdapter("https://boards-api.greenhouse.io/v1", hc, nil)
		case "herp":
			a = ats.NewHerpAdapter(hc, nil)
		case "hrmos":
			a = ats.NewHrmosAdapter("https://hrmos.co", hc, nil)
		case "icims":
			a = ats.NewICIMSAdapter(hc)
		case "join":
			a = ats.NewJoinAdapter("https://join.com", hc, nil)
		case "lever":
			a, err = ats.NewLeverAdapter("https://api.lever.co", hc, nil)
		case "mokahr":
			a, err = ats.NewMokaHRAdapter(mokahr.DefaultBaseURL, hc)
		case "oracle":
			a = ats.NewOracleAdapter(hc)
		case "recruitee":
			a = ats.NewRecruiteeAdapter(hc, nil)
		case "rippling":
			a, err = ats.NewRipplingAdapter("https://api.rippling.com/platform/api/ats/v1", hc, nil)
		case "smartrecruiters":
			a, err = ats.NewSmartRecruitersAdapter("https://api.smartrecruiters.com", hc)
		case "successfactors":
			a = ats.NewSuccessFactorsAdapter(hc)
		case "teamtailor":
			a = ats.NewTeamtailorAdapter(hc, nil)
		case "ultipro":
			a = ats.NewUltiProAdapter(hc)
		case "workable":
			a, err = ats.NewWorkableAdapter("https://apply.workable.com", hc)
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
// returns results in check order. Recruitee's public API rate-limits hard
// around concurrent board fetches (HTTP 429 at the default pool size), so
// recruitee checks share a tighter sub-cap of at most 2 workers without
// raising overall concurrency past the user-requested limit.
func runChecks(ctx context.Context, checks []check, timeout time.Duration, concurrency int) []result {
	results := make([]result, len(checks))
	sem := make(chan struct{}, concurrency)
	recruiteeCap := min(2, concurrency)
	recruiteeSem := make(chan struct{}, recruiteeCap)
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Go(func() {
			// Provider-specific caps first so waiters on a tight sub-cap
			// (Recruitee) do not occupy global slots and starve other
			// providers while queued on the Recruitee semaphore.
			if c.adapter.Name() == "recruitee" {
				recruiteeSem <- struct{}{}
				defer func() { <-recruiteeSem }()
			}
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = c.do(ctx, timeout)
		})
	}
	wg.Wait()
	return results
}

// do searches page 1 for the entry, follows up with one Detail probe on
// the first job, and classifies the outcome. The probe catches
// detail-template divergence (e.g. issue #196) that the search path never
// exercises; zero-job boards have nothing to probe and stay OK. A nonzero
// total with an empty page 1 (the adapter dropped every summary, e.g.
// Workday entries without externalPath) is DETAIL_ERROR too: the detail
// path cannot be verified. Each of the two requests gets its own timeout.
func (c check) do(ctx context.Context, timeout time.Duration) result {
	r := result{Provider: c.adapter.Name(), Company: c.company, Slug: c.slug}

	res, err := c.search(ctx, timeout)
	if err != nil {
		r.Status, r.Detail = _statusError, err.Error()
		return r
	}
	r.Status = _statusOK
	r.Jobs = res.TotalCount

	switch {
	case len(res.Jobs) > 0:
		jobID := res.Jobs[0].JobID
		if err := c.probeDetail(ctx, timeout, jobID); err != nil {
			r.Status = _statusDetailError
			r.Detail = fmt.Sprintf("detail %s: %s", jobID, err)
		}
	case res.TotalCount > 0:
		r.Status = _statusDetailError
		r.Detail = fmt.Sprintf("search reported %d jobs but page 1 carried no probeable job", res.TotalCount)
	}
	return r
}

func (c check) search(ctx context.Context, timeout time.Duration) (*ats.SearchResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.adapter.Search(ctx, c.slug, ats.SearchParams{Page: 1})
}

func (c check) probeDetail(ctx context.Context, timeout time.Duration, jobID string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := c.adapter.Detail(ctx, c.slug, jobID)
	return err
}

// printText writes one line per entry plus a summary. Jobs is shown
// whenever the search succeeded (OK and DETAIL_ERROR); Detail only for
// failures, where it explains which request failed and why.
func printText(results []result) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range results {
		jobs, detail := "", r.Detail
		if r.Status != _statusError {
			jobs = strconv.Itoa(r.Jobs)
		}
		if r.Status == _statusOK {
			detail = ""
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
		if r.Status == _statusOK {
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
