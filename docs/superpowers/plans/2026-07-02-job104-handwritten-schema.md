# job104 Hand-Written Input Schema Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the reflection-built input schema of the `104_search_jobs` MCP tool with a hand-written raw JSON schema, drop the `shift` filter from the tool layer, and make `area` required alongside `keyword`.

**Architecture:** A backtick raw-JSON string constant in `internal/jobmcp/job104.go` is unmarshaled once at package init into `*jsonschema.Schema` (panic on error). The `ids.go` label maps stay the single source of label→code conversion; the schema's enum labels are those map keys, hand-typed in yaml code order. Spec: `docs/superpowers/specs/2026-07-02-job104-handwritten-schema-design.md`.

**Tech Stack:** Go, `github.com/google/jsonschema-go` v0.4.3, `github.com/modelcontextprotocol/go-sdk/mcp`, testify.

## Global Constraints

- No provider-layer changes: `internal/provider/job104` (openapi.yaml, generated client, `ids.go` including `S9IDs`) is untouched.
- Tool property names stay friendly: `keyword`, `area`, `job_type`, `sort`, `remote`, `edu`, `page` (never `ro`/`order`/`remoteWork`/`s9`).
- Descriptions carry semantics only, never id=label tables.
- Do NOT commit anything under `docs/` (user instruction); commit only code changes.
- Test style: golden whole-value `want` + single `assert.Equal`; expected values hand-typed literals. Exception granted for `area`'s 74-label enum — strip it from `got` and spot-check instead.

---

### Task 1: Hand-written search input schema

**Files:**
- Modify: `internal/jobmcp/job104.go` (delete `labelEnum` + `job104SearchSchema`, add raw JSON const + `mustSchema`)
- Test: `internal/jobmcp/job104_test.go` (`TestJob104SearchJobsSchema`)

**Interfaces:**
- Consumes: `job104SearchInput` struct (unchanged in this task), `mcp.AddTool` registration (unchanged).
- Produces: `var job104SearchInputSchema *jsonschema.Schema` (same name as today, now built by `mustSchema(job104SearchSchemaJSON)`); `func mustSchema(raw string) *jsonschema.Schema`.

- [ ] **Step 1: Update the golden schema test**

Replace the entire `TestJob104SearchJobsSchema` function in `internal/jobmcp/job104_test.go` with the version below. Changes from the old golden: `required` gains `"area"`, the `shift` property is gone, `edu`'s type is plain `"array"` (the `["null","array"]` was a reflection artifact), and `area`'s 74-label enum is stripped from `got` after hand-typed spot checks instead of being compared in full (so the `labelEnum` helper is no longer used here).

```go
func TestJob104SearchJobsSchema(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	client, err := job104.NewClient("https://www.104.com.tw")
	require.NoError(t, err)
	RegisterJob104(server, client)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer clientSession.Close()

	res, err := clientSession.ListTools(ctx, nil)
	require.NoError(t, err)

	var searchTool *mcp.Tool
	for _, tool := range res.Tools {
		if tool.Name == "104_search_jobs" {
			searchTool = tool
			break
		}
	}
	require.NotNil(t, searchTool)

	schema, ok := searchTool.InputSchema.(map[string]any)
	require.True(t, ok)

	// area's 74-label enum is impractical to hand-type; spot-check the ends
	// of the list, then strip it so the golden compare below stays a single
	// hand-typed whole-value assertion.
	area, ok := schema["properties"].(map[string]any)["area"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, area["enum"], "Taipei")
	assert.Contains(t, area["enum"], "WestAfrica")
	delete(area, "enum")

	// Full golden schema: LLM-facing names only (no ro/order/remoteWork/s9),
	// label enums instead of raw codes, keyword and area required.
	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword": map[string]any{
				"type":        "string",
				"description": "Free-text keyword search.",
			},
			"area": map[string]any{
				"type":        "string",
				"description": "City/region filter.",
			},
			"job_type": map[string]any{
				"type":        "string",
				"description": "Employment basis. Soft filter — verify each result's jobRo.",
				"enum":        []any{"Full-time", "Part-time", "Senior", "Dispatch"},
			},
			"sort": map[string]any{
				"type":        "string",
				"description": "Result order.",
				"enum":        []any{"Relevance", "Newest"},
			},
			"remote": map[string]any{
				"type":        "string",
				"description": "Remote work. Soft filter — verify each result's remoteWorkType. Omit for on-site.",
				"enum":        []any{"Full", "Partial"},
			},
			"edu": map[string]any{
				"type":        "array",
				"description": "Education levels, OR'd together.",
				"uniqueItems": true,
				"items": map[string]any{
					"type": "string",
					"enum": []any{"HighSchoolBelow", "HighSchool", "College", "University", "Master", "Doctorate"},
				},
			},
			"page": map[string]any{
				"type":        "integer",
				"description": "1-based page number.",
				"minimum":     float64(1),
			},
		},
		"required":             []any{"keyword", "area"},
		"additionalProperties": false,
	}
	assert.Equal(t, want, schema)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/jobmcp/ -run TestJob104SearchJobsSchema -v`
Expected: FAIL — the current reflection-built schema still has a `shift` property, `required` is only `["keyword"]`, and `edu`'s type is `["null","array"]`.

- [ ] **Step 3: Replace the schema construction in job104.go**

In `internal/jobmcp/job104.go`:

3a. Change the imports: drop `cmp` and `slices`, add `encoding/json`. The block becomes:

```go
import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)
```

3b. Delete the `labelEnum` function and the whole `job104SearchSchema` function together with the old `var job104SearchInputSchema = job104SearchSchema()` declaration and its doc comment.

3c. In their place, add the hand-written schema. The property names are the LLM-facing friendly names; enum labels are exactly the `ids.go` map keys, ordered by their 104 code (same order as openapi.yaml); descriptions are unchanged from the old builder. Do not touch the `job104SearchInput` struct in this task.

```go
// job104SearchInputSchema is hand-written JSON kept aligned with openapi.yaml's
// searchJobs parameters: friendly property names, human labels instead of 104
// codes (the ids.go maps translate labels back to codes — enum labels here
// must match those map keys). Descriptions carry semantics only, never
// id=label tables.
var job104SearchInputSchema = mustSchema(job104SearchSchemaJSON)

// mustSchema unmarshals a raw JSON schema, panicking on malformed JSON —
// a programmer error, same failure mode as jsonschema.For before it.
func mustSchema(raw string) *jsonschema.Schema {
	var s jsonschema.Schema
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		panic(fmt.Sprintf("job104 search schema: %v", err))
	}
	return &s
}

const job104SearchSchemaJSON = `{
	"type": "object",
	"properties": {
		"keyword": {
			"type": "string",
			"description": "Free-text keyword search."
		},
		"area": {
			"type": "string",
			"description": "City/region filter.",
			"enum": [
				"Taipei", "NewTaipei", "Yilan", "Keelung", "Taoyuan",
				"Hsinchu", "Miaoli", "Taichung", "Changhua", "Nantou",
				"Yunlin", "Chiayi", "Tainan", "Kaohsiung", "Pingtung",
				"Taitung", "Hualien", "Penghu", "Kinmen", "Lienchiang",
				"Beijing", "Tianjin", "Shanghai", "Chongqing", "Guangdong",
				"Fujian", "Hainan", "Zhejiang", "Jiangsu", "Shandong",
				"Hebei", "Liaoning", "Jilin", "Heilongjiang", "Hunan",
				"Hubei", "Jiangxi", "Anhui", "Henan", "Shanxi",
				"Shaanxi", "Gansu", "Qinghai", "Sichuan", "Guizhou",
				"Yunnan", "InnerMongolia", "Tibet", "Ningxia", "Xinjiang",
				"Guangxi", "HongKong", "Macao",
				"NortheastAsia", "SoutheastAsia", "OtherAsia",
				"AustraliaNZ", "OtherOceania",
				"Canada", "EasternUS", "WesternUS", "MidwesternUS",
				"CentralAmerica", "SouthAmerica",
				"NorthernEurope", "SouthernEurope", "EasternEurope",
				"WesternEurope", "CentralEurope",
				"NorthAfrica", "CentralAfrica", "SouthAfrica",
				"EastAfrica", "WestAfrica"
			]
		},
		"job_type": {
			"type": "string",
			"description": "Employment basis. Soft filter — verify each result's jobRo.",
			"enum": ["Full-time", "Part-time", "Senior", "Dispatch"]
		},
		"sort": {
			"type": "string",
			"description": "Result order.",
			"enum": ["Relevance", "Newest"]
		},
		"remote": {
			"type": "string",
			"description": "Remote work. Soft filter — verify each result's remoteWorkType. Omit for on-site.",
			"enum": ["Full", "Partial"]
		},
		"edu": {
			"type": "array",
			"description": "Education levels, OR'd together.",
			"uniqueItems": true,
			"items": {
				"type": "string",
				"enum": ["HighSchoolBelow", "HighSchool", "College", "University", "Master", "Doctorate"]
			}
		},
		"page": {
			"type": "integer",
			"description": "1-based page number.",
			"minimum": 1
		}
	},
	"required": ["keyword", "area"],
	"additionalProperties": false
}`
```

Fallback (only if Step 4 shows `additionalProperties` failing to round-trip as boolean `false` through jsonschema-go v0.4.3): remove the `"additionalProperties": false` line from the raw JSON and set it in Go after unmarshal — in `mustSchema`'s caller is wrong (shared helper); instead set it once right after the var, i.e. replace the var line with:

```go
var job104SearchInputSchema = func() *jsonschema.Schema {
	s := mustSchema(job104SearchSchemaJSON)
	s.AdditionalProperties = &jsonschema.Schema{Not: &jsonschema.Schema{}}
	return s
}()
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/jobmcp/ -run TestJob104SearchJobsSchema -v`
Expected: PASS

Then run the whole package to confirm nothing else compiles against the deleted helpers:

Run: `go test ./internal/jobmcp/`
Expected: PASS (the conversion tests still pass — the struct and `job104ToRequest` are untouched so far)

- [ ] **Step 5: Commit**

```bash
git add internal/jobmcp/job104.go internal/jobmcp/job104_test.go
git commit -m "refactor: hand-write job104 search input schema"
```

---

### Task 2: Require area, drop shift from the tool input

**Files:**
- Modify: `internal/jobmcp/job104.go` (`job104SearchInput` struct, `job104ToRequest`)
- Test: `internal/jobmcp/job104_test.go` (conversion tests)

**Interfaces:**
- Consumes: `job104.AreaIDs` / `lookupCode` / `lookupCodes` (unchanged), `job104SearchInputSchema` from Task 1.
- Produces: `job104SearchInput` without `Shift`; `job104ToRequest` returning `fmt.Errorf("area is required")` when `Area` is empty and no longer populating `SearchJobsParams.S9`.

- [ ] **Step 1: Update the conversion tests**

In `internal/jobmcp/job104_test.go`, replace the four conversion test functions (`TestJob104ToRequest`, `TestJob104ToRequestMissingKeyword`, `TestJob104ToRequestKeywordOnly`, `TestJob104ToRequestInvalidLabels`) with the versions below. Changes: no `Shift`/`S9` anywhere; the minimal valid input is now keyword+area; a missing-`area` case appears next to missing-`keyword`; the invalid-`shift` case is gone.

```go
func TestJob104ToRequest(t *testing.T) {
	in := job104SearchInput{
		Keyword: "golang",
		Area:    "Taipei",
		JobType: "Part-time",
		Sort:    "Newest",
		Remote:  "Full",
		Edu:     []string{"University", "Master"},
		Page:    2,
	}
	got, err := job104ToRequest(in)
	require.NoError(t, err)

	want := job104.SearchJobsParams{
		Keyword:    job104.NewOptString("golang"),
		Area:       job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
		Ro:         job104.NewOptSearchJobsRo(job104.SearchJobsRo2),
		Order:      job104.NewOptSearchJobsOrder(job104.SearchJobsOrder2),
		RemoteWork: job104.NewOptSearchJobsRemoteWork(job104.SearchJobsRemoteWork1),
		Page:       job104.NewOptInt(2),
		Edu:        []job104.SearchJobsEduItem{job104.SearchJobsEduItem4, job104.SearchJobsEduItem5},
	}
	assert.Equal(t, want, got)
}

func TestJob104ToRequestMissingRequired(t *testing.T) {
	cases := []struct {
		name string
		in   job104SearchInput
		want string
	}{
		{"all empty", job104SearchInput{}, "keyword is required"},
		{"filters only", job104SearchInput{Area: "Taipei", Sort: "Newest", Page: 2}, "keyword is required"},
		{"keyword only", job104SearchInput{Keyword: "golang"}, "area is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := job104ToRequest(tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestJob104ToRequestMinimal(t *testing.T) {
	got, err := job104ToRequest(job104SearchInput{Keyword: "golang", Area: "Taipei"})
	require.NoError(t, err)
	want := job104.SearchJobsParams{
		Keyword: job104.NewOptString("golang"),
		Area:    job104.NewOptSearchJobsArea(job104.AreaIDs["Taipei"]),
	}
	assert.Equal(t, want, got)
}

func TestJob104ToRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   job104SearchInput
		want string
	}{
		{"area", job104SearchInput{Keyword: "x", Area: "Mars"}, `invalid area "Mars"`},
		{"job_type", job104SearchInput{Keyword: "x", Area: "Taipei", JobType: "full"}, `invalid job_type "full"`},
		{"sort", job104SearchInput{Keyword: "x", Area: "Taipei", Sort: "newest"}, `invalid sort "newest"`},
		{"remote", job104SearchInput{Keyword: "x", Area: "Taipei", Remote: "hybrid"}, `invalid remote "hybrid"`},
		{"edu", job104SearchInput{Keyword: "x", Area: "Taipei", Edu: []string{"University", "PhD"}}, `invalid edu "PhD"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := job104ToRequest(tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
```

(`TestJob104ToRequestMissingKeyword` and `TestJob104ToRequestKeywordOnly` are replaced by `TestJob104ToRequestMissingRequired` and `TestJob104ToRequestMinimal` — delete the old two.)

- [ ] **Step 2: Run the tests to verify the new case fails**

Run: `go test ./internal/jobmcp/ -run 'TestJob104ToRequest' -v`
Expected: FAIL — `TestJob104ToRequestMissingRequired/keyword_only` fails because `job104ToRequest` does not yet return "area is required" (it currently succeeds with keyword only). The other tests compile and pass because the `Shift` field still exists but is simply unused.

- [ ] **Step 3: Require area and drop shift in job104.go**

3a. The struct loses `Shift`, and `Area` loses `omitempty`:

```go
type job104SearchInput struct {
	Keyword string   `json:"keyword"` // required
	Area    string   `json:"area"`    // required
	JobType string   `json:"job_type,omitempty"`
	Sort    string   `json:"sort,omitempty"`
	Remote  string   `json:"remote,omitempty"`
	Edu     []string `json:"edu,omitempty"`
	Page    int      `json:"page,omitempty"`
}
```

3b. In `job104ToRequest`, the area block stops being conditional and gains a required guard mirroring keyword's, and the S9 block is deleted. The function becomes:

```go
func job104ToRequest(in job104SearchInput) (job104.SearchJobsParams, error) {
	var params job104.SearchJobsParams
	// The schema already marks keyword and area required; these guard direct
	// callers and clients that skip schema validation.
	if in.Keyword == "" {
		return params, fmt.Errorf("keyword is required")
	}
	params.Keyword = job104.NewOptString(in.Keyword)
	if in.Area == "" {
		return params, fmt.Errorf("area is required")
	}
	code, err := lookupCode("area", in.Area, job104.AreaIDs)
	if err != nil {
		return params, err
	}
	params.Area = job104.NewOptSearchJobsArea(code)
	if in.JobType != "" {
		code, err := lookupCode("job_type", in.JobType, job104.RoIDs)
		if err != nil {
			return params, err
		}
		params.Ro = job104.NewOptSearchJobsRo(code)
	}
	if in.Sort != "" {
		code, err := lookupCode("sort", in.Sort, job104.OrderIDs)
		if err != nil {
			return params, err
		}
		params.Order = job104.NewOptSearchJobsOrder(code)
	}
	if in.Remote != "" {
		code, err := lookupCode("remote", in.Remote, job104.RemoteWorkIDs)
		if err != nil {
			return params, err
		}
		params.RemoteWork = job104.NewOptSearchJobsRemoteWork(code)
	}
	if params.Edu, err = lookupCodes("edu", in.Edu, job104.EduIDs); err != nil {
		return params, err
	}
	if in.Page > 0 {
		params.Page = job104.NewOptInt(in.Page)
	}
	return params, nil
}
```

(Note the top-level `code, err :=` for area now declares `err`, so the later `if params.Edu, err = ...` keeps plain `=`; the former `var err error` line is gone.)

- [ ] **Step 4: Run the full package tests**

Run: `go test ./internal/jobmcp/`
Expected: PASS (all tests, including Task 1's schema golden)

Then confirm the whole module still builds and passes:

Run: `go build ./... && go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/jobmcp/job104.go internal/jobmcp/job104_test.go
git commit -m "feat: require area and drop shift in 104 search tool"
```
