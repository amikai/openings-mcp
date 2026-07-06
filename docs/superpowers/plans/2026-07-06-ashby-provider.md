# Ashby Provider (Codegen, Tests, Company Roster) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete `internal/provider/ashby/` with committed ogen codegen, a fixture-backed mock server and client tests, and a verified company → board-slug roster. No MCP wiring.

**Architecture:** The ogen-generated `Client` is the client (no hand-written wrapper), exactly as in cake/workday. Tests hit an `httptest` mock serving raw fixtures captured from the real `browserbase` board (5 jobs; exercises secondaryLocations, streetAddress, compensation tiers, and null tier titles). `companies.go` mirrors workday's `companies.go` with Ashby's simpler single-slug addressing.

**Tech Stack:** Go, ogen v1.22 (`go tool`), testify, goccy/go-yaml (already a dependency via workday), curl + jq for fixtures.

**Spec:** `docs/superpowers/specs/2026-07-06-ashby-provider-design.md`

## Global Constraints

- **Never run `git commit` without the user's explicit go-ahead.** At each commit step, stop and ask the user first (standing user preference; overrides any skill default).
- Exported package vars (`Companies`, `CompaniesByBoard`), no getter functions (standing user preference).
- Generated `oas_*_gen.go` files are committed and must be reproducible: `go generate ./internal/provider/ashby/` must yield no diff.
- All commands run from the repo root.
- Fixture-derived assertion values below were extracted from live captures on 2026-07-06. Job boards drift; each task says how to re-derive a value if a fixture re-capture changed it.

---

### Task 1: Spec touch-up and committed codegen

**Files:**
- Modify: `internal/provider/ashby/openapi.yaml` (PostalAddress schema)
- Create: `internal/provider/ashby/gen.go`
- Create (generated): `internal/provider/ashby/oas_*_gen.go`

**Interfaces:**
- Produces: generated package `ashby` with `NewClient(serverURL string, ...) (*Client, error)`, `(*Client).GetJobBoard(ctx context.Context, params GetJobBoardParams) (GetJobBoardRes, error)`, `GetJobBoardParams{JobBoardName string, IncludeCompensation OptBool}`, res variants `*JobBoardResponse` (200) and `*GetJobBoardNotFound{Data io.Reader}` (404), enum consts `JobPostingEmploymentTypeFullTime`, `JobPostingWorkplaceTypeOnSite`, option types `OptString`/`OptBool`/`OptNilString{Value,Set,Null}`/`OptNilFloat64{Value,Set,Null}` with `NewOptString`/`NewOptBool` constructors. Tasks 2–3 compile against these.

- [ ] **Step 1: Add the observed `streetAddress` field to PostalAddress**

Live `browserbase` data carries `address.postalAddress.streetAddress`, which the spec (written against a board without street addresses) lacks. ogen skips unknown fields, so this is accuracy, not a bug fix. In `internal/provider/ashby/openapi.yaml`, replace:

```yaml
        postalCode:
          type: string
          description: Postal code.
```

with:

```yaml
        postalCode:
          type: string
          description: Postal code.
        streetAddress:
          type: string
          description: >
            Street address. Not listed in the official field reference;
            observed on boards that publish full office addresses.
          example: 1 Post Street, Floor 15
```

- [ ] **Step 2: Write `internal/provider/ashby/gen.go`**

```go
//go:generate go tool github.com/ogen-go/ogen/cmd/ogen --target . -package ashby --clean openapi.yaml

package ashby
```

- [ ] **Step 3: Generate**

```bash
go generate ./internal/provider/ashby/
```

Expected: exit 0 (an INFO line about "Convenient errors" is normal); `oas_*_gen.go` files appear next to `openapi.yaml`.

- [ ] **Step 4: Build and vet**

```bash
go build ./internal/provider/ashby/ && go vet ./internal/provider/ashby/
```

Expected: both exit 0 with no output.

- [ ] **Step 5: Commit (only after the user explicitly approves)**

```bash
git add internal/provider/ashby/
git commit -m "feat(ashby): generate client from OpenAPI spec"
```

---

### Task 2: Fixtures, mock server, client tests

**Files:**
- Create: `internal/provider/ashby/testdata/board_req.sh`
- Create (captured): `internal/provider/ashby/testdata/board_rsp.json`, `internal/provider/ashby/testdata/board_comp_rsp.json`
- Create: `internal/provider/ashby/mocksrv.go`
- Test: `internal/provider/ashby/client_test.go`

**Interfaces:**
- Consumes: the generated client surface from Task 1.
- Produces: `const MockBoardName = "browserbase"` and `func NewMockServer() *httptest.Server` (serves both fixtures by `includeCompensation` query param, plain-text 404 for unknown boards), plus unexported `serveMockJSON(data []byte) http.HandlerFunc` — same shape as workday's `mocksrv.go`.

- [ ] **Step 1: Write the capture script `internal/provider/ashby/testdata/board_req.sh`**

```bash
#!/bin/bash
# Captures Ashby public job-board fixtures from the real `browserbase` board
# (small - 5 jobs at capture time - but exercises secondaryLocations,
# streetAddress, compensation tiers, and null tier titles).
BASE="https://api.ashbyhq.com/posting-api/job-board/browserbase"
curl -s "$BASE" | jq . > board_rsp.json
curl -s "$BASE?includeCompensation=true" | jq . > board_comp_rsp.json
```

Make it executable: `chmod +x internal/provider/ashby/testdata/board_req.sh`

- [ ] **Step 2: Capture the fixtures**

```bash
cd internal/provider/ashby/testdata && ./board_req.sh && cd -
jq '.jobs | length' internal/provider/ashby/testdata/board_rsp.json
jq '[.jobs[] | has("compensation")] | all' internal/provider/ashby/testdata/board_comp_rsp.json
```

Expected: both files created; job count 5; second jq prints `true`. If the count is no longer 5 or the first job is no longer "Software Engineer (Agent Platform)", re-derive the assertion values used in Step 3 from the fresh fixture:

```bash
jq '.jobs[0] | {id, title, department, team, employmentType, location, publishedAt, workplaceType, address, secondaryLocations}' internal/provider/ashby/testdata/board_comp_rsp.json
jq '.jobs[0].compensation | {compensationTierSummary, scrapeableCompensationSalarySummary, compensationTiers}' internal/provider/ashby/testdata/board_comp_rsp.json
```

and adjust the `want` values in the test code accordingly (same fields, fresh values).

- [ ] **Step 3: Write the failing tests `internal/provider/ashby/client_test.go`**

```go
package ashby

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer wraps the fixture handlers with request assertions the
// reusable NewMockServer deliberately doesn't make.
func newTestServer(t *testing.T, wantIncludeComp bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/posting-api/job-board/"+MockBoardName, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		if wantIncludeComp {
			assert.Equal(t, "true", r.URL.Query().Get("includeCompensation"))
			serveMockJSON(mockBoardCompRsp)(w, r)
			return
		}
		assert.Empty(t, r.URL.Query().Get("includeCompensation"))
		serveMockJSON(mockBoardRsp)(w, r)
	})
	return httptest.NewServer(mux)
}

func TestGetJobBoard(t *testing.T) {
	srv := newTestServer(t, false)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobBoard(context.Background(), GetJobBoardParams{JobBoardName: MockBoardName})
	require.NoError(t, err)

	board, ok := res.(*JobBoardResponse)
	require.True(t, ok, "expected *JobBoardResponse, got %T", res)

	assert.Equal(t, "1", board.ApiVersion)
	require.Len(t, board.Jobs, 5)

	job := board.Jobs[0]
	assert.Equal(t, NewOptString("7724fbe3-6a27-4418-9705-2dcc40751a16"), job.ID)
	assert.Equal(t, "Software Engineer (Agent Platform)", job.Title)
	assert.Equal(t, NewOptString("Engineering"), job.Department)
	assert.Equal(t, NewOptString("Engineering"), job.Team)
	assert.Equal(t, JobPostingEmploymentTypeFullTime, job.EmploymentType)
	assert.Equal(t, JobPostingWorkplaceTypeOnSite, job.WorkplaceType)
	assert.Equal(t, NewOptString("San Francisco"), job.Location)
	assert.False(t, job.IsRemote)
	assert.True(t, job.IsListed)
	assert.True(t, job.PublishedAt.Equal(time.Date(2025, 8, 25, 20, 13, 34, 942_000_000, time.UTC)))
	assert.Equal(t, "https://jobs.ashbyhq.com/browserbase/7724fbe3-6a27-4418-9705-2dcc40751a16", job.JobUrl)
	assert.Equal(t, "https://jobs.ashbyhq.com/browserbase/7724fbe3-6a27-4418-9705-2dcc40751a16/application", job.ApplyUrl)
	assert.True(t, job.DescriptionHtml.Set)
	assert.True(t, job.DescriptionPlain.Set)

	addr := job.Address.Value.PostalAddress.Value
	assert.Equal(t, NewOptString("San Francisco"), addr.AddressLocality)
	assert.Equal(t, NewOptString("CA"), addr.AddressRegion)
	assert.Equal(t, NewOptString("United States"), addr.AddressCountry)
	assert.Equal(t, NewOptString("94104"), addr.PostalCode)
	assert.Equal(t, NewOptString("1 Post Street, Floor 15"), addr.StreetAddress)

	require.Len(t, job.SecondaryLocations, 1)
	assert.Equal(t, NewOptString("New York"), job.SecondaryLocations[0].Location)

	for _, j := range board.Jobs {
		assert.False(t, j.Compensation.Set, "compensation must be absent without includeCompensation")
		assert.False(t, j.ShouldDisplayCompensationOnJobPostings.Set)
	}
}

func TestGetJobBoardWithCompensation(t *testing.T) {
	srv := newTestServer(t, true)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobBoard(context.Background(), GetJobBoardParams{
		JobBoardName:        MockBoardName,
		IncludeCompensation: NewOptBool(true),
	})
	require.NoError(t, err)

	board, ok := res.(*JobBoardResponse)
	require.True(t, ok, "expected *JobBoardResponse, got %T", res)
	require.Len(t, board.Jobs, 5)

	job := board.Jobs[0]
	assert.Equal(t, NewOptBool(true), job.ShouldDisplayCompensationOnJobPostings)
	require.True(t, job.Compensation.Set)

	comp := job.Compensation.Value
	assert.Equal(t, NewOptString("$132K – $330K • Offers Equity"), comp.CompensationTierSummary)
	assert.Equal(t, NewOptString("$132K - $330K"), comp.ScrapeableCompensationSalarySummary)

	require.Len(t, comp.CompensationTiers, 1)
	tier := comp.CompensationTiers[0]
	assert.Equal(t, OptNilString{Set: true, Null: true}, tier.Title, "unnamed tier decodes as null title")
	assert.Equal(t, NewOptString("Estimated base salary $132K – $330K • Offers Equity"), tier.TierSummary)

	require.Len(t, tier.Components, 2)
	salary := tier.Components[0]
	assert.Equal(t, NewOptString("Salary"), salary.CompensationType)
	assert.Equal(t, NewOptString("1 YEAR"), salary.Interval)
	assert.Equal(t, OptNilString{Value: "USD", Set: true}, salary.CurrencyCode)
	assert.Equal(t, OptNilFloat64{Value: 132000, Set: true}, salary.MinValue)
	assert.Equal(t, OptNilFloat64{Value: 330000, Set: true}, salary.MaxValue)

	equity := tier.Components[1]
	assert.Equal(t, NewOptString("EquityPercentage"), equity.CompensationType)
	assert.Equal(t, OptNilString{Set: true, Null: true}, equity.CurrencyCode)
	assert.Equal(t, OptNilFloat64{Set: true, Null: true}, equity.MinValue)
	assert.Equal(t, OptNilFloat64{Set: true, Null: true}, equity.MaxValue)
}

func TestGetJobBoardNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobBoard(context.Background(), GetJobBoardParams{JobBoardName: "no-such-board"})
	require.NoError(t, err)

	nf, ok := res.(*GetJobBoardNotFound)
	require.True(t, ok, "expected *GetJobBoardNotFound, got %T", res)
	body, err := io.ReadAll(nf)
	require.NoError(t, err)
	assert.Equal(t, "Not Found", strings.TrimSpace(string(body)))
}
```

- [ ] **Step 4: Run tests to verify they fail**

```bash
go test ./internal/provider/ashby/
```

Expected: FAIL to compile — `undefined: MockBoardName`, `undefined: serveMockJSON`, `undefined: mockBoardRsp`, `undefined: NewMockServer`.

- [ ] **Step 5: Write `internal/provider/ashby/mocksrv.go`**

```go
package ashby

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

// MockBoardName is the board slug the mock server serves fixtures for — the
// real board the testdata was captured from (see testdata/board_req.sh).
const MockBoardName = "browserbase"

//go:embed testdata/board_rsp.json
var mockBoardRsp []byte

//go:embed testdata/board_comp_rsp.json
var mockBoardCompRsp []byte

// NewMockServer returns an httptest.Server serving canned Ashby job-board
// fixture responses captured from a real board (see testdata/board_req.sh),
// so tests never hit the live API. The compensation fixture is served when
// the request sets includeCompensation=true; unknown boards get the same
// plain-text 404 the real API returns. The caller owns the server and must
// Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/posting-api/job-board/"+MockBoardName, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("includeCompensation") == "true" {
			serveMockJSON(mockBoardCompRsp)(w, r)
			return
		}
		serveMockJSON(mockBoardRsp)(w, r)
	})
	mux.HandleFunc("/posting-api/job-board/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	})
	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/provider/ashby/ -v
```

Expected: PASS for `TestGetJobBoard`, `TestGetJobBoardWithCompensation`, `TestGetJobBoardNotFound`. If an assertion fails on a concrete value, the board drifted since capture — re-derive values per Task 2 Step 2 and update only those assertions.

- [ ] **Step 7: Commit (only after the user explicitly approves)**

```bash
git add internal/provider/ashby/
git commit -m "test(ashby): add mock server and client tests backed by live fixtures"
```

---

### Task 3: Company roster

**Files:**
- Create: `internal/provider/ashby/companies.yaml`
- Create: `internal/provider/ashby/testdata/verify_companies.sh`
- Create: `internal/provider/ashby/companies.go`
- Test: `internal/provider/ashby/companies_test.go`

**Interfaces:**
- Produces: `type Company struct { Name, Board string }` with `BoardURL() string`, exported vars `Companies []Company` (sorted by name) and `CompaniesByBoard map[string]Company` (keyed by lowercased board slug).

- [ ] **Step 1: Write `internal/provider/ashby/companies.yaml`**

Every entry below was verified live on 2026-07-06: HTTP 200 from the posting API AND ≥1 published job AND the public board page title names the expected company. Candidates that 404ed (e.g. `runwayml`, `anysphere`) were dropped; so were live-but-empty boards (`airtable`, `deel`, `hex`, `mercury`, `snyk`, `vercel` — an empty board is indistinguishable from an abandoned one) and `hightouch` (board page title says "[old]"). Note `runway` on Ashby is Runway Financial, not Runway ML — deliberately excluded.

```yaml
- company: "Alchemy"
  board: "alchemy"
- company: "Ashby"
  board: "ashby"
- company: "Astronomer"
  board: "astronomer"
- company: "Benchling"
  board: "benchling"
- company: "Browserbase"
  board: "browserbase"
- company: "Character.AI"
  board: "character"
- company: "ClickHouse"
  board: "clickhouse"
- company: "Cohere"
  board: "cohere"
- company: "Cursor"
  board: "cursor"
- company: "Docker"
  board: "docker"
- company: "Eight Sleep"
  board: "eightsleep"
- company: "ElevenLabs"
  board: "elevenlabs"
- company: "Granola"
  board: "granola"
- company: "Harvey"
  board: "harvey"
- company: "Kalshi"
  board: "kalshi"
- company: "LangChain"
  board: "langchain"
- company: "Linear"
  board: "linear"
- company: "Lovable"
  board: "lovable"
- company: "Midjourney"
  board: "midjourney"
- company: "Modal"
  board: "modal"
- company: "Neon"
  board: "neon"
- company: "Notion"
  board: "notion"
- company: "OpenAI"
  board: "openai"
- company: "Patreon"
  board: "patreon"
- company: "Perplexity"
  board: "perplexity"
- company: "Pinecone"
  board: "pinecone"
- company: "Plaid"
  board: "plaid"
- company: "Polymarket"
  board: "polymarket"
- company: "PostHog"
  board: "posthog"
- company: "Quora"
  board: "quora"
- company: "Railway"
  board: "railway"
- company: "Ramp"
  board: "ramp"
- company: "Render"
  board: "render"
- company: "Replit"
  board: "replit"
- company: "Sierra"
  board: "sierra"
- company: "Stytch"
  board: "stytch"
- company: "Substack"
  board: "substack"
- company: "Suno"
  board: "suno"
- company: "Supabase"
  board: "supabase"
- company: "Temporal Technologies"
  board: "temporal"
- company: "Uniswap Labs"
  board: "uniswap"
- company: "Vanta"
  board: "vanta"
- company: "Warp"
  board: "warp"
- company: "Weaviate"
  board: "weaviate"
- company: "Zapier"
  board: "zapier"
- company: "Zed"
  board: "zed"
```

- [ ] **Step 2: Write `internal/provider/ashby/testdata/verify_companies.sh`**

```bash
#!/bin/bash
# Re-verifies every board in ../companies.yaml against the live Ashby
# posting API: each must answer HTTP 200 with a non-empty jobs array.
# Boards that fail (org left Ashby / renamed its board) or report zero jobs
# (possibly abandoned) are flagged for manual review.
set -u
cd "$(dirname "$0")/.."
fail=0
while read -r board; do
  rsp=$(curl -s --max-time 60 "https://api.ashbyhq.com/posting-api/job-board/$board")
  n=$(printf '%s' "$rsp" | jq '.jobs | length' 2>/dev/null)
  if [ -z "$n" ] || [ "$n" = "null" ]; then
    echo "BAD  $board: not a job-board response"
    fail=1
  elif [ "$n" -eq 0 ]; then
    echo "WARN $board: 0 jobs (possibly abandoned board)"
  else
    echo "OK   $board: $n jobs"
  fi
done < <(awk '/^ *board:/ {gsub(/"/, "", $2); print $2}' companies.yaml)
exit $fail
```

Make it executable: `chmod +x internal/provider/ashby/testdata/verify_companies.sh`

- [ ] **Step 3: Run the verification script**

```bash
internal/provider/ashby/testdata/verify_companies.sh
```

Expected: 46 `OK` lines, exit 0. On `BAD` (board vanished since 2026-07-06): remove that entry from companies.yaml and rerun. On `WARN` (board emptied): remove it too, per the empty-board policy above.

- [ ] **Step 4: Write the failing test `internal/provider/ashby/companies_test.go`**

```go
package ashby

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompanies(t *testing.T) {
	assert.NotEmpty(t, Companies)

	seen := make(map[string]string, len(Companies))
	for _, c := range Companies {
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.Board)
		if prev, dup := seen[c.Board]; dup {
			t.Errorf("duplicate board %q used by %q and %q (CompaniesByBoard silently drops one)", c.Board, prev, c.Name)
		}
		seen[c.Board] = c.Name
	}

	assert.Equal(t, "https://jobs.ashbyhq.com/openai", CompaniesByBoard["openai"].BoardURL())
}
```

- [ ] **Step 5: Run test to verify it fails**

```bash
go test ./internal/provider/ashby/ -run TestCompanies
```

Expected: FAIL to compile — `undefined: Companies`, `undefined: CompaniesByBoard`.

- [ ] **Step 6: Write `internal/provider/ashby/companies.go`**

```go
package ashby

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed organization hosting a public Ashby job board,
// drawn from a curated list (internal/provider/ashby/companies.yaml). Every
// entry was verified against the live posting API — HTTP 200 with a
// non-empty jobs array — and its board page title checked against the
// expected company name; testdata/verify_companies.sh re-verifies the
// roster. It's keyed by board slug (e.g. "openai"), the same identifier the
// API takes as its jobBoardName path parameter.
type Company struct {
	Name  string `yaml:"company" json:"company"`
	Board string `yaml:"board" json:"board"`
}

// BoardURL returns the company's human-facing job board page, e.g.
// https://jobs.ashbyhq.com/openai. API calls instead pass Board directly as
// the jobBoardName parameter.
func (c Company) BoardURL() string {
	return fmt.Sprintf("https://jobs.ashbyhq.com/%s", c.Board)
}

// Companies holds every confirmed Ashby board, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesByBoard looks up a confirmed company by board slug. Keys are
// lowercased, so callers must lowercase their input before indexing.
var CompaniesByBoard = buildBoardIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is a
// build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []Company {
	var cs []Company
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("ashby: parse companies.yaml: %v", err))
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name })
	return cs
}

func buildBoardIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[strings.ToLower(c.Board)] = c
	}
	return m
}
```

- [ ] **Step 7: Run tests to verify they pass**

```bash
go test ./internal/provider/ashby/ -run TestCompanies -v
```

Expected: PASS.

- [ ] **Step 8: Full package verification**

```bash
go generate ./internal/provider/ashby/ && git status --short internal/provider/ashby/
go test ./internal/provider/ashby/
go vet ./internal/provider/ashby/
```

Expected: `git status` shows no modifications to `oas_*_gen.go` after regeneration (only the untracked new files from this task, if not yet committed); tests PASS; vet clean.

- [ ] **Step 9: Commit (only after the user explicitly approves)**

```bash
git add internal/provider/ashby/
git commit -m "feat(ashby): add curated company board roster"
```
