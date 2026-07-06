# Lever Postings API Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `internal/provider/lever` — an ogen-generated client for the public Lever Postings API (list + detail, JSON only), with mock-server tests and a curated list of 16 verified Lever tenants.

**Architecture:** Hand-written `openapi.yaml` (the contract) → ogen codegen (the client) → mock server replaying real captured `leverdemo` fixtures (the tests). Same stopping point as the Workday/LinkedIn providers: no MCP wiring, no CLI. Spec: `docs/superpowers/specs/2026-07-06-lever-provider-design.md`.

**Tech Stack:** Go, ogen (already a `go.mod` tool), `github.com/goccy/go-yaml` (already a dependency), testify, `net/http/httptest`.

## Global Constraints

- **Never commit without asking.** The repo owner requires explicit confirmation before any `git commit`. At every "Commit" step below, show the staged diff summary and ask first. This overrides any skill instruction to commit automatically.
- Package: `internal/provider/lever`, package name `lever`. All new files live there unless a step says otherwise.
- Generated files (`oas_*_gen.go`) are committed, never hand-edited. Regenerate with `go generate ./internal/provider/lever/...`.
- CI runs `go test $RACE -vet=all ./...` and `make validate-openapi` (docker-based). Both must pass.
- Follow the `golang-style` skill when writing Go.
- **Generated-name check:** ogen derives Go names (`ID` vs `Id`, `HostedURL`, `ListPostingsModeJson`, Opt types) from the spec. Test code in Tasks 3–5 uses best-guess names; each task has a grep step to confirm them against the generated code before running tests. Adjusting a name to match generated code is expected, not a deviation.
- Fixture values in test assertions (posting id `33538a2f-...`, text `AbelsonTaylor Writer`, etc.) were verified against the live API on 2026-07-06. Task 2 re-captures fixtures; if leverdemo's content changed, update the assertion values from the fresh fixtures as instructed there.

---

### Task 1: OpenAPI spec + ogen codegen

**Files:**
- Create: `internal/provider/lever/openapi.yaml`
- Create: `internal/provider/lever/gen.go`
- Create (generated): `internal/provider/lever/oas_*_gen.go`
- Modify: `Makefile:3-12` (add the new spec to `OPENAPI_SPECS`)

**Interfaces:**
- Consumes: nothing (first task).
- Produces: generated client — `NewClient(serverURL string, opts ...ClientOption) (*Client, error)`, `(*Client).ListPostings(ctx, ListPostingsParams) (Postings, error)`, `(*Client).GetPosting(ctx, GetPostingParams) (*Posting, error)`, types `Posting`, `Postings` (`[]Posting`), `PostingCategories`, `PostingListEntry`, `SalaryRange`, `ErrorResponse`, `*ErrorResponseStatusCode` (typed error for non-200), `ListPostingsModeJson` enum constant, `WithClient(*http.Client)` option. Exact spellings confirmed in Step 4.

- [ ] **Step 1: Write the spec**

Create `internal/provider/lever/openapi.yaml`:

```yaml
openapi: 3.1.0

info:
  title: Lever Postings API (JSON consumer surface)
  version: "1.0.0"
  description: >
    Minimal JSON-consumer surface of the public, unauthenticated Lever
    Postings API (https://github.com/lever/postings-api, mirrored at
    case-study/postings-api). Lever is a multi-tenant ATS: every company's
    published postings live under a site slug (e.g. jobs.lever.co/leverdemo
    -> site "leverdemo"), and this one contract serves all tenants. A
    company lives on exactly one instance, global or EU; switch instances
    with the client's WithServerURL option.

    Deliberately out of scope: the POST apply endpoint (a write operation
    that needs an API key and submits real applications), the `group`
    parameter (changes the list response's top-level shape to
    `[{title, postings}]`; grouping is trivial client-side), and the
    HTML/iframe embedding features (`mode=html`, `mode=iframe`, `css`,
    `resize`). `mode` is modeled as a required single-value enum (`json`)
    so a response can never fall back to HTML.

    Response fields follow the official field table, corrected against
    live leverdemo responses: `createdAt` (epoch milliseconds) is returned
    by the API but missing from the official table; `salaryRange`,
    `salaryDescription`, and `salaryDescriptionPlain` are documented but
    absent on postings without salary data, so everything except `id` and
    `text` is optional.

servers:
  - url: https://api.lever.co
    description: Global instance (default).
  - url: https://api.eu.lever.co
    description: EU instance.

tags:
  - name: Postings
    description: Published job postings, namespaced by company site slug.

paths:
  /v0/postings/{site}:
    get:
      tags: [Postings]
      summary: List published postings for a site
      description: >
        Filter values are case-sensitive. Repeating a filter parameter ORs
        the values within that field, e.g.
        `?location=Oakland&location=Boston`.
      operationId: listPostings
      parameters:
        - name: site
          in: path
          required: true
          description: Company site slug, e.g. `leverdemo`.
          schema:
            type: string
          example: "leverdemo"
        - name: mode
          in: query
          required: true
          description: >
            Output mode, pinned to `json`. The upstream API also accepts
            `html` and `iframe`; those are web-embedding features and not
            modeled here.
          schema:
            type: string
            enum: [json]
        - name: skip
          in: query
          required: false
          description: Skip N postings from the start.
          schema:
            type: integer
            minimum: 0
        - name: limit
          in: query
          required: false
          description: Return at most N postings.
          schema:
            type: integer
            minimum: 1
        - name: location
          in: query
          required: false
          description: Filter by location; repeatable, values OR'ed.
          schema:
            type: array
            items:
              type: string
        - name: commitment
          in: query
          required: false
          description: Filter by commitment; repeatable, values OR'ed.
          schema:
            type: array
            items:
              type: string
        - name: team
          in: query
          required: false
          description: Filter by team; repeatable, values OR'ed.
          schema:
            type: array
            items:
              type: string
        - name: department
          in: query
          required: false
          description: >
            Filter by department, if the company uses departments;
            repeatable, values OR'ed.
          schema:
            type: array
            items:
              type: string
        - name: level
          in: query
          required: false
          description: Filter by level.
          schema:
            type: string
      responses:
        "200":
          description: Published postings matching the filters.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Postings"
        default:
          description: >
            Any non-200 status. A 404 with `{ok: false, error: "Document
            not found"}` has been observed for an unknown site.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ErrorResponse"

  /v0/postings/{site}/{postingId}:
    get:
      tags: [Postings]
      summary: Get one posting by id
      description: >
        Same schema as a list element. This endpoint only serves JSON
        upstream, so it takes no `mode` parameter.
      operationId: getPosting
      parameters:
        - name: site
          in: path
          required: true
          description: Company site slug, e.g. `leverdemo`.
          schema:
            type: string
          example: "leverdemo"
        - name: postingId
          in: path
          required: true
          description: Posting id from a list element's `id` field.
          schema:
            type: string
          example: "33538a2f-d27d-4a96-8f05-fa4b0e4d940e"
      responses:
        "200":
          description: The posting.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Posting"
        default:
          description: >
            Any non-200 status. A 404 with `{ok: false, error: "Document
            not found"}` has been observed for an unknown posting id.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ErrorResponse"

components:
  schemas:
    Postings:
      type: array
      items:
        $ref: "#/components/schemas/Posting"

    Posting:
      type: object
      required: [id, text]
      properties:
        id:
          type: string
          description: Unique posting id.
        text:
          type: string
          description: Posting title.
        categories:
          $ref: "#/components/schemas/PostingCategories"
        country:
          type: ["string", "null"]
          description: >
            ISO 3166-1 alpha-2 country code, or null when unknown. Not
            filterable.
        createdAt:
          type: integer
          format: int64
          description: >
            Creation timestamp in epoch milliseconds. Returned by the live
            API; missing from the official field table.
        workplaceType:
          type: string
          enum: [unspecified, on-site, remote, hybrid]
          description: Primary workplace environment. Not filterable.
        opening:
          type: string
          description: Description opening (styled HTML).
        openingPlain:
          type: string
          description: Description opening (plaintext).
        description:
          type: string
          description: Combined opening and body (styled HTML).
        descriptionPlain:
          type: string
          description: Combined opening and body (plaintext).
        descriptionBody:
          type: string
          description: Body without opening (styled HTML).
        descriptionBodyPlain:
          type: string
          description: Body without opening (plaintext).
        lists:
          type: array
          description: Extra lists such as requirements and benefits.
          items:
            $ref: "#/components/schemas/PostingListEntry"
        additional:
          type: string
          description: Optional closing content (styled HTML); may be empty.
        additionalPlain:
          type: string
          description: Optional closing content (plaintext); may be empty.
        hostedUrl:
          type: string
          description: Lever-hosted posting page URL.
        applyUrl:
          type: string
          description: Lever-hosted application form URL.
        salaryRange:
          $ref: "#/components/schemas/SalaryRange"
        salaryDescription:
          type: string
          description: Optional salary range description (styled HTML).
        salaryDescriptionPlain:
          type: string
          description: Optional salary range description (plaintext).

    PostingCategories:
      type: object
      description: >
        The primary posting location is `location` and also appears in
        `allLocations`.
      properties:
        location:
          type: string
        commitment:
          type: string
        team:
          type: string
        department:
          type: string
        allLocations:
          type: array
          items:
            type: string

    PostingListEntry:
      type: object
      required: [text, content]
      properties:
        text:
          type: string
          description: List section name, e.g. "Qualifications".
        content:
          type: string
          description: Unstyled HTML of the list elements.

    SalaryRange:
      type: object
      properties:
        currency:
          type: string
        interval:
          type: string
        min:
          type: number
        max:
          type: number

    ErrorResponse:
      type: object
      required: [ok, error]
      properties:
        ok:
          type: boolean
          description: Always false on errors.
        error:
          type: string
      description: >
        Observed error body, e.g. `{"ok": false, "error": "Document not
        found"}` with a 404 for an unknown site or posting id.
```

- [ ] **Step 2: Write the codegen directive**

Create `internal/provider/lever/gen.go`:

```go
//go:generate go tool github.com/ogen-go/ogen/cmd/ogen --target . -package lever --clean openapi.yaml

package lever
```

- [ ] **Step 3: Generate and build**

Run:
```bash
go generate ./internal/provider/lever/... && go build ./... && go vet ./...
```
Expected: `oas_*_gen.go` files appear in `internal/provider/lever/`, build and vet pass. If ogen rejects the spec (e.g. the 3.1 nullable `country`), fix the spec, not the generated code, and regenerate.

- [ ] **Step 4: Confirm generated names**

Run:
```bash
grep -n "func (c \*Client)" internal/provider/lever/oas_client_gen.go
grep -n "ListPostingsMode\|type Postings\|type Posting struct\|ErrorResponseStatusCode" internal/provider/lever/oas_schemas_gen.go internal/provider/lever/oas_parameters_gen.go | head -20
grep -n "ID\|HostedURL\|CreatedAt\|Country " internal/provider/lever/oas_schemas_gen.go | head -20
```
Expected: method signatures matching the Interfaces block above. Note the exact spellings (`ID`/`PostingID`, `ListPostingsModeJson`, `OptNilString` for `country`, `OptInt64` for `createdAt`) — Tasks 3–5 use them.

- [ ] **Step 5: Register the spec in the Makefile**

In `Makefile`, add one line to `OPENAPI_SPECS` between `job104` and `linkedin`:

```makefile
	internal/provider/lever/openapi.yaml \
```

If docker is available, run `make validate-openapi` — expected: every spec validates. If docker is unavailable, note that and rely on CI.

- [ ] **Step 6: Commit (ask first)**

```bash
git add internal/provider/lever/ Makefile
git commit -m "feat(lever): add OpenAPI spec and ogen client for Lever Postings API"
```

---

### Task 2: Capture leverdemo fixtures

**Files:**
- Create: `internal/provider/lever/testdata/postings_req.sh`
- Create: `internal/provider/lever/testdata/postings_rsp.json`
- Create: `internal/provider/lever/testdata/posting_detail_req.sh`
- Create: `internal/provider/lever/testdata/posting_detail_rsp.json`

**Interfaces:**
- Consumes: nothing (live API access only).
- Produces: `testdata/postings_rsp.json` (JSON array of 3 postings) and `testdata/posting_detail_rsp.json` (one posting object), embedded by Task 3's mock server.

- [ ] **Step 1: Write the capture scripts**

Create `internal/provider/lever/testdata/postings_req.sh`:

```bash
#!/bin/bash
curl -s "https://api.lever.co/v0/postings/leverdemo?mode=json&limit=3" \
  | jq . > postings_rsp.json
```

Create `internal/provider/lever/testdata/posting_detail_req.sh`:

```bash
#!/bin/bash
# Fetches the first posting in postings_rsp.json; run postings_req.sh first.
id=$(jq -r '.[0].id' postings_rsp.json)
curl -s "https://api.lever.co/v0/postings/leverdemo/${id}?mode=json" \
  | jq . > posting_detail_rsp.json
```

- [ ] **Step 2: Capture**

Run:
```bash
cd internal/provider/lever/testdata && chmod +x postings_req.sh posting_detail_req.sh && ./postings_req.sh && ./posting_detail_req.sh
```
Expected: both `*_rsp.json` files exist; `jq length postings_rsp.json` prints `3`; `jq -r .id posting_detail_rsp.json` prints a UUID.

- [ ] **Step 3: Verify assertion values still hold**

Run:
```bash
cd internal/provider/lever/testdata && jq -r '.[0] | .id, .text, .country, .workplaceType, .createdAt, (.categories.location)' postings_rsp.json
```
Expected (as of 2026-07-06):
```
33538a2f-d27d-4a96-8f05-fa4b0e4d940e
AbelsonTaylor Writer
US
hybrid
1553186035299
Arlington, TX
```
If any line differs (leverdemo content changed), the fixture wins: substitute the fresh values everywhere Task 3's test code hardcodes them.

- [ ] **Step 4: Commit (ask first)**

```bash
git add internal/provider/lever/testdata/
git commit -m "test(lever): capture leverdemo fixtures for mock server"
```

---

### Task 3: Mock server + happy-path client tests

**Files:**
- Create: `internal/provider/lever/mocksrv.go`
- Create: `internal/provider/lever/client_test.go`

**Interfaces:**
- Consumes: generated client from Task 1; fixtures from Task 2.
- Produces: `NewMockServer() *httptest.Server`, `serveMockJSON([]byte) http.HandlerFunc`, `serveMockError(w, status int, msg string)`, embedded vars `mockPostingsRsp`, `mockPostingDetailRsp`, constants `MockNotFoundSite = "mock-404-site"`, `MockNotFoundPostingID = "mock-404-posting"` — all reused by Task 4.

- [ ] **Step 1: Write the failing happy-path tests**

Create `internal/provider/lever/client_test.go` (field spellings per Task 1 Step 4; fixture values per Task 2 Step 3):

```go
package lever

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockServer wraps the package mock server with per-request assertions
// on the exact query encoding the generated client produces.
func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/postings/{site}", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "leverdemo", r.PathValue("site"))
		q := r.URL.Query()
		assert.Equal(t, "json", q.Get("mode"))
		assert.Equal(t, "0", q.Get("skip"))
		assert.Equal(t, "3", q.Get("limit"))
		assert.Equal(t, []string{"Arlington, TX", "New York, NY"}, q["location"])
		assert.Equal(t, []string{"Customer Success"}, q["department"])
		serveMockJSON(mockPostingsRsp)(w, r)
	})
	mux.HandleFunc("/v0/postings/{site}/{postingId}", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "leverdemo", r.PathValue("site"))
		assert.Equal(t, "33538a2f-d27d-4a96-8f05-fa4b0e4d940e", r.PathValue("postingId"))
		serveMockJSON(mockPostingDetailRsp)(w, r)
	})
	return httptest.NewServer(mux)
}

func TestListPostings(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	got, err := c.ListPostings(t.Context(), ListPostingsParams{
		Site:       "leverdemo",
		Mode:       ListPostingsModeJson,
		Skip:       NewOptInt(0),
		Limit:      NewOptInt(3),
		Location:   []string{"Arlington, TX", "New York, NY"},
		Department: []string{"Customer Success"},
	})
	require.NoError(t, err)
	require.Len(t, got, 3)

	first := got[0]
	assert.Equal(t, "33538a2f-d27d-4a96-8f05-fa4b0e4d940e", first.ID)
	assert.Equal(t, "AbelsonTaylor Writer", first.Text)
	assert.Equal(t, NewOptNilString("US"), first.Country)
	assert.Equal(t, NewOptInt64(1553186035299), first.CreatedAt)
	assert.Equal(t, NewOptPostingWorkplaceType(PostingWorkplaceTypeHybrid), first.WorkplaceType)
	assert.Equal(t, NewOptString("https://jobs.lever.co/leverdemo/33538a2f-d27d-4a96-8f05-fa4b0e4d940e"), first.HostedURL)
	assert.Equal(t, NewOptString("https://jobs.lever.co/leverdemo/33538a2f-d27d-4a96-8f05-fa4b0e4d940e/apply"), first.ApplyURL)

	cats := first.Categories.Value
	assert.Equal(t, NewOptString("Arlington, TX"), cats.Location)
	assert.Equal(t, NewOptString("Regular Full Time (Salary)"), cats.Commitment)
	assert.Equal(t, NewOptString("Professional Services"), cats.Team)
	assert.Equal(t, NewOptString("Customer Success"), cats.Department)
	assert.Equal(t, []string{"Arlington, TX"}, cats.AllLocations)

	require.NotEmpty(t, first.Lists)
	assert.Equal(t, "Qualifications", first.Lists[0].Text)
}

func TestGetPosting(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	got, err := c.GetPosting(t.Context(), GetPostingParams{
		Site:      "leverdemo",
		PostingId: "33538a2f-d27d-4a96-8f05-fa4b0e4d940e",
	})
	require.NoError(t, err)
	assert.Equal(t, "33538a2f-d27d-4a96-8f05-fa4b0e4d940e", got.ID)
	assert.Equal(t, "AbelsonTaylor Writer", got.Text)
}
```

Note: `GetPosting` may return `*Posting`; if the generated signature returns a different wrapper, follow the generated code. Same for `PostingId` vs `PostingID`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/provider/lever/ -run 'TestListPostings|TestGetPosting' -v`
Expected: FAIL to compile — `undefined: serveMockJSON`, `undefined: mockPostingsRsp`, `undefined: mockPostingDetailRsp`.

- [ ] **Step 3: Write the mock server**

Create `internal/provider/lever/mocksrv.go`:

```go
package lever

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/postings_rsp.json
var mockPostingsRsp []byte

//go:embed testdata/posting_detail_rsp.json
var mockPostingDetailRsp []byte

// MockNotFoundSite and MockNotFoundPostingID trigger the mock server's
// error path so tests can exercise non-200 handling: listing
// MockNotFoundSite's postings or requesting MockNotFoundPostingID's detail
// returns a 404 with the same JSON error body the real API sends for an
// unknown site or posting.
const (
	MockNotFoundSite      = "mock-404-site"
	MockNotFoundPostingID = "mock-404-posting"
)

// NewMockServer returns an httptest.Server that mimics the Lever Postings
// API with canned leverdemo fixture responses, so tests never hit the real
// site. The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/postings/{site}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("site") == MockNotFoundSite {
			serveMockError(w, http.StatusNotFound, "Document not found")
			return
		}
		serveMockJSON(mockPostingsRsp)(w, r)
	})
	mux.HandleFunc("/v0/postings/{site}/{postingId}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("postingId") == MockNotFoundPostingID {
			serveMockError(w, http.StatusNotFound, "Document not found")
			return
		}
		serveMockJSON(mockPostingDetailRsp)(w, r)
	})
	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func serveMockError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"ok":false,"error":%q}`, msg)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/provider/lever/ -run 'TestListPostings|TestGetPosting' -v`
Expected: PASS (2 tests). If a struct field or Opt constructor name mismatches, fix the test to match the generated code (Task 1 Step 4's grep output), not the other way around.

- [ ] **Step 5: Commit (ask first)**

```bash
git add internal/provider/lever/mocksrv.go internal/provider/lever/client_test.go
git commit -m "test(lever): add mock server and happy-path client tests"
```

---

### Task 4: Error-path tests

**Files:**
- Modify: `internal/provider/lever/client_test.go` (append two tests)

**Interfaces:**
- Consumes: `NewMockServer()`, `MockNotFoundSite`, `MockNotFoundPostingID` from Task 3; `ErrorResponse`, `*ErrorResponseStatusCode` from Task 1.
- Produces: nothing new.

- [ ] **Step 1: Write the failing error-path tests**

Append to `internal/provider/lever/client_test.go` (add `"errors"` to the imports):

```go
func TestListPostingsNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	_, err = c.ListPostings(t.Context(), ListPostingsParams{
		Site: MockNotFoundSite,
		Mode: ListPostingsModeJson,
	})
	require.Error(t, err)

	ue, ok := errors.AsType[*ErrorResponseStatusCode](err)
	require.True(t, ok, "expected *ErrorResponseStatusCode in %v", err)
	want := &ErrorResponseStatusCode{
		StatusCode: 404,
		Response:   ErrorResponse{Ok: false, Error: "Document not found"},
	}
	assert.Equal(t, want, ue)
}

func TestGetPostingNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	_, err = c.GetPosting(t.Context(), GetPostingParams{
		Site:      "leverdemo",
		PostingId: MockNotFoundPostingID,
	})
	require.Error(t, err)

	ue, ok := errors.AsType[*ErrorResponseStatusCode](err)
	require.True(t, ok, "expected *ErrorResponseStatusCode in %v", err)
	want := &ErrorResponseStatusCode{
		StatusCode: 404,
		Response:   ErrorResponse{Ok: false, Error: "Document not found"},
	}
	assert.Equal(t, want, ue)
}
```

- [ ] **Step 2: Run the new tests**

Run: `go test ./internal/provider/lever/ -run 'NotFound' -v`
Expected: PASS (the mock error path already exists from Task 3; these tests pin the decoded error shape). If they fail on the `ErrorResponse` field names, check `oas_schemas_gen.go` — with `required: [ok, error]` they should be plain `Ok bool` / `Error string`.

- [ ] **Step 3: Run the whole package**

Run: `go test ./internal/provider/lever/ -v`
Expected: all 4 tests PASS.

- [ ] **Step 4: Commit (ask first)**

```bash
git add internal/provider/lever/client_test.go
git commit -m "test(lever): pin 404 error decoding for unknown site and posting"
```

---

### Task 5: Curated company list

**Files:**
- Create: `internal/provider/lever/companies.yaml`
- Create: `internal/provider/lever/companies.go`
- Create: `internal/provider/lever/companies_test.go`

**Interfaces:**
- Consumes: nothing from other tasks (independent of the generated client).
- Produces: `type Company struct { Name, Site string }`, `var Companies []Company` (sorted by name), `var CompaniesBySite map[string]Company`.

- [ ] **Step 1: Write the failing test**

Create `internal/provider/lever/companies_test.go`:

```go
package lever

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompanies(t *testing.T) {
	require.NotEmpty(t, Companies)
	assert.True(t, sort.SliceIsSorted(Companies, func(i, j int) bool {
		return Companies[i].Name < Companies[j].Name
	}), "Companies must be sorted by name")

	seen := make(map[string]bool, len(Companies))
	for _, c := range Companies {
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.Site)
		assert.Equal(t, strings.ToLower(c.Site), c.Site, "site slugs are lowercase")
		assert.False(t, seen[c.Site], "duplicate site %q", c.Site)
		seen[c.Site] = true
	}

	demo, ok := CompaniesBySite["leverdemo"]
	require.True(t, ok)
	assert.Equal(t, "Lever (demo)", demo.Name)
	assert.Len(t, CompaniesBySite, len(Companies))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/provider/lever/ -run TestCompanies -v`
Expected: FAIL to compile — `undefined: Companies`, `undefined: CompaniesBySite`.

- [ ] **Step 3: Write the company list**

Create `internal/provider/lever/companies.yaml`. All 16 sites were verified on 2026-07-06 by `GET https://api.lever.co/v0/postings/<site>?limit=1&mode=json` returning 200 with a non-empty array:

```yaml
- company: "Binance"
  site: "binance"
- company: "Crypto.com"
  site: "crypto"
- company: "Immutable"
  site: "immutable"
- company: "Lever (demo)"
  site: "leverdemo"
- company: "Match Group"
  site: "matchgroup"
- company: "Mistral AI"
  site: "mistral"
- company: "MoonPay"
  site: "moonpay"
- company: "Ninja Van"
  site: "ninjavan"
- company: "Nium"
  site: "nium"
- company: "Offchain Labs"
  site: "offchainlabs"
- company: "Outreach"
  site: "outreach"
- company: "Palantir Technologies"
  site: "palantir"
- company: "SwissBorg"
  site: "swissborg"
- company: "Veeva Systems"
  site: "veeva"
- company: "WHOOP"
  site: "whoop"
- company: "Zoox"
  site: "zoox"
```

To re-verify (optional, hits the live API 16 times):

```bash
yq -r '.[].site' internal/provider/lever/companies.yaml | while read -r s; do
  n=$(curl -s "https://api.lever.co/v0/postings/$s?limit=1&mode=json" | jq 'if type == "array" then length else -1 end')
  echo "$s: $n"
done
```
Expected: every line ends in `1`. Drop any site that errors or returns 0.

- [ ] **Step 4: Write the loader**

Create `internal/provider/lever/companies.go` (mirrors `internal/provider/workday/companies.go`; exports the slice and index directly, no wrapper getters):

```go
package lever

import (
	_ "embed"
	"fmt"
	"sort"

	"github.com/goccy/go-yaml"
)

//go:embed companies.yaml
var companiesYAML []byte

// Company is a confirmed Lever tenant from the curated
// internal/provider/lever/companies.yaml. Site is the slug that namespaces
// the company's postings, e.g. "leverdemo" for jobs.lever.co/leverdemo —
// slugs are unique and lowercase, unlike display names. Every entry was
// verified to return a non-empty postings array from the global instance
// (api.lever.co) at collection time.
type Company struct {
	Name string `yaml:"company" json:"company"`
	Site string `yaml:"site" json:"site"`
}

// Companies holds every confirmed Lever tenant, sorted by company name.
var Companies = mustLoadCompanies()

// CompaniesBySite looks up a confirmed tenant by site slug. Keys are
// lowercase slugs as they appear in companies.yaml.
var CompaniesBySite = buildSiteIndex(Companies)

// mustLoadCompanies parses the embedded companies.yaml. A parse failure is
// a build-time bug in a file this package owns, not a runtime condition to
// recover from.
func mustLoadCompanies() []Company {
	var cs []Company
	if err := yaml.Unmarshal(companiesYAML, &cs); err != nil {
		panic(fmt.Sprintf("lever: parse companies.yaml: %v", err))
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name })
	return cs
}

func buildSiteIndex(cs []Company) map[string]Company {
	m := make(map[string]Company, len(cs))
	for _, c := range cs {
		m[c.Site] = c
	}
	return m
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/provider/lever/ -run TestCompanies -v`
Expected: PASS.

- [ ] **Step 6: Commit (ask first)**

```bash
git add internal/provider/lever/companies.yaml internal/provider/lever/companies.go internal/provider/lever/companies_test.go
git commit -m "feat(lever): add curated list of 16 verified Lever tenants"
```

---

### Task 6: Final verification

**Files:** none (verification only).

**Interfaces:**
- Consumes: everything above.
- Produces: a clean CI-equivalent run.

- [ ] **Step 1: Run the full suite the way CI does**

Run:
```bash
go generate ./internal/provider/lever/... && git status --porcelain internal/provider/lever/
```
Expected: no diff from regeneration (generated code is in sync with the committed spec).

Run:
```bash
go test -race -vet=all ./...
```
Expected: PASS across all packages.

If docker is available:
```bash
make validate-openapi
```
Expected: every spec, including `internal/provider/lever/openapi.yaml`, validates.

- [ ] **Step 2: Report**

No commit here. Summarize test results and any deviations from the plan (renamed generated identifiers, refreshed fixture values) to the user.
