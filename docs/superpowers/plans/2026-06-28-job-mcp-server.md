# Job MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the existing 104 and tsmc Go job-board clients as a single MCP server over stdio, with 4 precisely-typed per-board tools.

**Architecture:** New `internal/jobmcp` package holds one file per board. Each file declares typed input structs, board-local label→code maps, a pure `*ToRequest` mapper (unit-tested), and thin handlers that call the existing client and return text. `cmd/jobmcp/main.go` builds shared-`http.Client` clients, registers tools, runs stdio.

**Tech Stack:** Go 1.26, `github.com/modelcontextprotocol/go-sdk/mcp`, existing `internal/provider/job104` and `internal/provider/tsmc` clients.

## Global Constraints

- Module path: `github.com/amikai/job-mcp`. Import 104 as `job104 "github.com/amikai/job-mcp/internal/provider/job104"`, tsmc as `"github.com/amikai/job-mcp/internal/provider/tsmc"`.
- go-sdk handler signature: `func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error)`. `Out` is `any` (text-only output, no output schema).
- Tool registration: `mcp.AddTool(server, &mcp.Tool{Name, Description}, handler)`.
- Upstream/validation failures return `&mcp.CallToolResult{IsError: true, Content: [text]}`, NOT a Go `error` (a returned error aborts the call; we want the LLM to see the message).
- Per-call timeout via the shared `*http.Client{Timeout: 30 * time.Second}` (covers hung upstreams without per-handler context plumbing).
- Tool names exactly: `104_search_jobs`, `104_get_job_detail`, `tsmc_search_jobs`, `tsmc_get_job_detail`.
- Enums are plain `string` fields whose allowed values are listed in the `jsonschema` description; the mapper validates and errors on unknown values. (No custom JSON schema machinery in v1.)
- Do NOT commit unless the user explicitly asks.

---

### Task 1: jobmcp package + 104 tools

**Files:**
- Create: `internal/jobmcp/jobmcp.go` (shared result helpers)
- Create: `internal/jobmcp/job104.go` (104 input structs, maps, mapper, handlers, register)
- Create: `internal/jobmcp/job104_test.go`
- Modify: `go.mod` / `go.sum` (add go-sdk via `go get`)

**Interfaces:**
- Produces:
  - `func textResult(s string) *mcp.CallToolResult`
  - `func errorResult(err error) *mcp.CallToolResult`
  - `func RegisterJob104(s *mcp.Server, c *job104.Client)`
  - `func job104ToRequest(in job104SearchInput) (*job104.JobRequest, error)`
  - type `job104SearchInput`, `job104DetailInput`

- [ ] **Step 1: Add the go-sdk dependency**

Run:
```bash
cd /Users/amikai/Workspace/job-mcp
go get github.com/modelcontextprotocol/go-sdk@latest
```
Expected: `go.mod` gains a `github.com/modelcontextprotocol/go-sdk vX.Y.Z` require line (v1.x).

- [ ] **Step 2: Write shared result helpers**

Create `internal/jobmcp/jobmcp.go`:
```go
// Package jobmcp adapts the internal job-board clients into MCP tools.
package jobmcp

import "github.com/modelcontextprotocol/go-sdk/mcp"

// textResult wraps a plain string as a successful tool result.
func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

// errorResult reports a failure to the model without aborting the tool call.
func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}
```

- [ ] **Step 3: Write the failing 104 mapper test**

Create `internal/jobmcp/job104_test.go`:
```go
package jobmcp

import (
	"testing"

	job104 "github.com/amikai/job-mcp/internal/provider/job104"
)

func TestJob104ToRequest(t *testing.T) {
	in := job104SearchInput{
		Keyword: "golang",
		Area:    "taipei",
		JobType: "part",
		Sort:    "newest",
		Remote:  "full",
		Page:    2,
	}
	got, err := job104ToRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Keyword != "golang" {
		t.Errorf("Keyword = %q, want golang", got.Keyword)
	}
	if got.Area != job104.AreaTaipei {
		t.Errorf("Area = %q, want %q", got.Area, job104.AreaTaipei)
	}
	if got.RO == nil || *got.RO != 1 {
		t.Errorf("RO = %v, want 1", got.RO)
	}
	if got.Order == nil || *got.Order != 15 {
		t.Errorf("Order = %v, want 15", got.Order)
	}
	if got.RemoteWork == nil || *got.RemoteWork != 2 {
		t.Errorf("RemoteWork = %v, want 2", got.RemoteWork)
	}
	if got.Page == nil || *got.Page != 2 {
		t.Errorf("Page = %v, want 2", got.Page)
	}
}

func TestJob104ToRequestInvalidArea(t *testing.T) {
	_, err := job104ToRequest(job104SearchInput{Keyword: "x", Area: "atlantis"})
	if err == nil {
		t.Fatal("expected error for invalid area, got nil")
	}
}
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `go test ./internal/jobmcp/`
Expected: FAIL — `undefined: job104SearchInput` / `job104ToRequest`.

- [ ] **Step 5: Implement `internal/jobmcp/job104.go`**

```go
package jobmcp

import (
	"context"
	"fmt"

	job104 "github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type job104SearchInput struct {
	Keyword string `json:"keyword" jsonschema:"search keyword, required"`
	Area    string `json:"area,omitempty" jsonschema:"city filter; one of: taipei, new_taipei, taoyuan, taichung, tainan, kaohsiung"`
	JobType string `json:"job_type,omitempty" jsonschema:"employment basis; one of: full, part"`
	Sort    string `json:"sort,omitempty" jsonschema:"result order; one of: newest, relevance"`
	Remote  string `json:"remote,omitempty" jsonschema:"remote work; one of: none, partial, full"`
	Page    int    `json:"page,omitempty" jsonschema:"1-based page number"`
}

type job104DetailInput struct {
	JobCode string `json:"job_code" jsonschema:"104 job code (jobNo), required"`
}

var (
	job104Areas = map[string]string{
		"taipei":     job104.AreaTaipei,
		"new_taipei": job104.AreaNewTaipei,
		"taoyuan":    job104.AreaTaoyuan,
		"taichung":   job104.AreaTaichung,
		"tainan":     job104.AreaTainan,
		"kaohsiung":  job104.AreaKaohsiung,
	}
	job104JobType = map[string]int{"full": 0, "part": 1}
	job104Sort    = map[string]int{"newest": 15, "relevance": 1}
	job104Remote  = map[string]int{"none": 0, "partial": 1, "full": 2}
)

func job104ToRequest(in job104SearchInput) (*job104.JobRequest, error) {
	r := &job104.JobRequest{Keyword: in.Keyword}
	if in.Area != "" {
		code, ok := job104Areas[in.Area]
		if !ok {
			return nil, fmt.Errorf("invalid area %q (want taipei|new_taipei|taoyuan|taichung|tainan|kaohsiung)", in.Area)
		}
		r.Area = code
	}
	if in.JobType != "" {
		v, ok := job104JobType[in.JobType]
		if !ok {
			return nil, fmt.Errorf("invalid job_type %q (want full|part)", in.JobType)
		}
		r.RO = &v
	}
	if in.Sort != "" {
		v, ok := job104Sort[in.Sort]
		if !ok {
			return nil, fmt.Errorf("invalid sort %q (want newest|relevance)", in.Sort)
		}
		r.Order = &v
	}
	if in.Remote != "" {
		v, ok := job104Remote[in.Remote]
		if !ok {
			return nil, fmt.Errorf("invalid remote %q (want none|partial|full)", in.Remote)
		}
		r.RemoteWork = &v
	}
	if in.Page > 0 {
		p := in.Page
		r.Page = &p
	}
	return r, nil
}

// RegisterJob104 registers the 104 search and job-detail tools.
func RegisterJob104(s *mcp.Server, c *job104.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_search_jobs",
		Description: "Search jobs on 104 (Taiwan's largest job board) by keyword, with optional city/job-type/remote/sort filters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in job104SearchInput) (*mcp.CallToolResult, any, error) {
		req, err := job104ToRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, err := c.Jobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(job104.FormatSearchJobResponse(resp)), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "104_get_job_detail",
		Description: "Get the full job description for a 104 job code (jobNo from search results).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in job104DetailInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.JobDetail(ctx, in.JobCode)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(job104.FormatJobDetail(resp, in.JobCode)), nil, nil
	})
}
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./internal/jobmcp/`
Expected: PASS.

- [ ] **Step 7: Vet the package builds**

Run: `go vet ./internal/jobmcp/`
Expected: no output (success).

---

### Task 2: tsmc tools

**Files:**
- Create: `internal/jobmcp/tsmc.go`
- Create: `internal/jobmcp/tsmc_test.go`

**Interfaces:**
- Consumes: `textResult`, `errorResult` from Task 1.
- Produces:
  - `func RegisterTSMC(s *mcp.Server, c *tsmc.Client)`
  - `func tsmcToRequest(in tsmcSearchInput) (*tsmc.JobRequest, error)`
  - type `tsmcSearchInput`, `tsmcDetailInput`

- [ ] **Step 1: Write the failing tsmc mapper test**

Create `internal/jobmcp/tsmc_test.go`:
```go
package jobmcp

import (
	"testing"

	"github.com/amikai/job-mcp/internal/tsmc"
)

func TestTSMCToRequest(t *testing.T) {
	in := tsmcSearchInput{
		Keyword:         "process engineer",
		Locations:       []string{"taiwan", "japan_osaka"},
		Categories:      []string{"rd"},
		JobTypes:        []string{"engineer"},
		EmploymentTypes: []string{"regular"},
		Page:            3,
	}
	got, err := tsmcToRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Keyword != "process engineer" || got.Page != 3 {
		t.Errorf("Keyword/Page = %q/%d", got.Keyword, got.Page)
	}
	wantLoc := []string{tsmc.LocTaiwan, tsmc.LocJapanOsaka}
	if len(got.Locations) != 2 || got.Locations[0] != wantLoc[0] || got.Locations[1] != wantLoc[1] {
		t.Errorf("Locations = %v, want %v", got.Locations, wantLoc)
	}
	if len(got.Categories) != 1 || got.Categories[0] != tsmc.CatRD {
		t.Errorf("Categories = %v, want [%s]", got.Categories, tsmc.CatRD)
	}
	if len(got.JobTypes) != 1 || got.JobTypes[0] != tsmc.JobTypeEngineer {
		t.Errorf("JobTypes = %v, want [%s]", got.JobTypes, tsmc.JobTypeEngineer)
	}
	if len(got.EmploymentTypes) != 1 || got.EmploymentTypes[0] != tsmc.EmployRegular {
		t.Errorf("EmploymentTypes = %v", got.EmploymentTypes)
	}
}

func TestTSMCToRequestInvalidLocation(t *testing.T) {
	_, err := tsmcToRequest(tsmcSearchInput{Keyword: "x", Locations: []string{"mars"}})
	if err == nil {
		t.Fatal("expected error for invalid location, got nil")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/jobmcp/`
Expected: FAIL — `undefined: tsmcSearchInput` / `tsmcToRequest`.

- [ ] **Step 3: Implement `internal/jobmcp/tsmc.go`**

```go
package jobmcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/amikai/job-mcp/internal/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type tsmcSearchInput struct {
	Keyword         string   `json:"keyword" jsonschema:"search keyword, required"`
	Locations       []string `json:"locations,omitempty" jsonschema:"sites; any of: taiwan, canada, china, germany_dresden, germany_munich, japan_yokohama, japan_osaka, japan_tsukuba, japan_kumamoto, korea, netherlands, usa_arizona, usa_california, usa_massachusetts, usa_texas, usa_washington, usa_washington_dc"`
	Categories      []string `json:"categories,omitempty" jsonschema:"job families; any of: rd, specialty_technology, ic_design_technology, manufacturing, facility_and_safety, product_development, ic_packaging_technology, testing_development, quality_and_reliability, it, internal_audit, business_development, customer_service, corporate_planning, finance, human_resources, legal, materials_management, corporate_sustainability, administration, accessibility_inclusion"`
	JobTypes        []string `json:"job_types,omitempty" jsonschema:"seniority; any of: technician, associate_engineer, engineer, manager, others"`
	EmploymentTypes []string `json:"employment_types,omitempty" jsonschema:"any of: regular, temporary, intern, apprenticeship"`
	Page            int      `json:"page,omitempty" jsonschema:"1-based page number"`
}

type tsmcDetailInput struct {
	JobID string `json:"job_id" jsonschema:"tsmc job id (from search results), required"`
}

var (
	tsmcLocations = map[string]string{
		"taiwan": tsmc.LocTaiwan, "canada": tsmc.LocCanada, "china": tsmc.LocChina,
		"germany_dresden": tsmc.LocGermanyDresden, "germany_munich": tsmc.LocGermanyMunich,
		"japan_yokohama": tsmc.LocJapanYokohama, "japan_osaka": tsmc.LocJapanOsaka,
		"japan_tsukuba": tsmc.LocJapanTsukuba, "japan_kumamoto": tsmc.LocJapanKumamoto,
		"korea": tsmc.LocKorea, "netherlands": tsmc.LocNetherlands,
		"usa_arizona": tsmc.LocUSAArizona, "usa_california": tsmc.LocUSACalifornia,
		"usa_massachusetts": tsmc.LocUSAMassachusetts, "usa_texas": tsmc.LocUSATexas,
		"usa_washington": tsmc.LocUSAWashington, "usa_washington_dc": tsmc.LocUSAWashingtonDC,
	}
	tsmcCategories = map[string]string{
		"rd": tsmc.CatRD, "specialty_technology": tsmc.CatSpecialtyTechnology,
		"ic_design_technology": tsmc.CatICDesignTechnology, "manufacturing": tsmc.CatManufacturing,
		"facility_and_safety": tsmc.CatFacilityAndSafety, "product_development": tsmc.CatProductDevelopment,
		"ic_packaging_technology": tsmc.CatICPackagingTechnology, "testing_development": tsmc.CatTestingDevelopment,
		"quality_and_reliability": tsmc.CatQualityAndReliability, "it": tsmc.CatIT,
		"internal_audit": tsmc.CatInternalAudit, "business_development": tsmc.CatBusinessDevelopment,
		"customer_service": tsmc.CatCustomerService, "corporate_planning": tsmc.CatCorporatePlanning,
		"finance": tsmc.CatFinance, "human_resources": tsmc.CatHumanResources,
		"legal": tsmc.CatLegal, "materials_management": tsmc.CatMaterialsManagement,
		"corporate_sustainability": tsmc.CatCorporateSustainability, "administration": tsmc.CatAdministration,
		"accessibility_inclusion": tsmc.CatAccessibilityInclusion,
	}
	tsmcJobTypes = map[string]string{
		"technician": tsmc.JobTypeTechnician, "associate_engineer": tsmc.JobTypeAssociateEngineer,
		"engineer": tsmc.JobTypeEngineer, "manager": tsmc.JobTypeManager, "others": tsmc.JobTypeOthers,
	}
	tsmcEmploymentTypes = map[string]string{
		"regular": tsmc.EmployRegular, "temporary": tsmc.EmployTemporary,
		"intern": tsmc.EmployIntern, "apprenticeship": tsmc.EmployApprenticeship,
	}
)

// mapCodes translates human enum labels to tsmc facet codes, erroring on any unknown label.
func mapCodes(field string, labels []string, m map[string]string) ([]string, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		code, ok := m[l]
		if !ok {
			return nil, fmt.Errorf("invalid %s %q", field, l)
		}
		out = append(out, code)
	}
	return out, nil
}

func tsmcToRequest(in tsmcSearchInput) (*tsmc.JobRequest, error) {
	r := &tsmc.JobRequest{Keyword: in.Keyword, Page: in.Page}
	var err error
	if r.Locations, err = mapCodes("location", in.Locations, tsmcLocations); err != nil {
		return nil, err
	}
	if r.Categories, err = mapCodes("category", in.Categories, tsmcCategories); err != nil {
		return nil, err
	}
	if r.JobTypes, err = mapCodes("job_type", in.JobTypes, tsmcJobTypes); err != nil {
		return nil, err
	}
	if r.EmploymentTypes, err = mapCodes("employment_type", in.EmploymentTypes, tsmcEmploymentTypes); err != nil {
		return nil, err
	}
	return r, nil
}

func formatTSMCSearch(r *tsmc.SearchResponse) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d jobs (showing %d)\n\n", r.Total, len(r.Jobs))
	for i, j := range r.Jobs {
		fmt.Fprintf(&sb, "%d. [%s] %s\n   Location: %s | Area: %s | %s | Posted: %s\n\n",
			i+1, j.ID, j.Title, j.Location, j.CareerArea, j.EmploymentType, j.Posted)
	}
	return sb.String()
}

func formatTSMCDetail(d *tsmc.JobDetail) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\nCompany: %s | Location: %s | Area: %s\nType: %s | Employment: %s | Posted: %s\n\n",
		d.Title, d.Company, d.Location, d.CareerArea, d.JobType, d.EmploymentType, d.Posted)
	fmt.Fprintf(&sb, "Responsibilities:\n%s\n\nQualifications:\n%s\n", d.Responsibilities, d.Qualifications)
	return sb.String()
}

// RegisterTSMC registers the tsmc search and job-detail tools.
func RegisterTSMC(s *mcp.Server, c *tsmc.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "tsmc_search_jobs",
		Description: "Search TSMC careers by keyword, with optional location/category/seniority/employment filters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in tsmcSearchInput) (*mcp.CallToolResult, any, error) {
		req, err := tsmcToRequest(in)
		if err != nil {
			return errorResult(err), nil, nil
		}
		resp, err := c.Jobs(ctx, req)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(formatTSMCSearch(resp)), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "tsmc_get_job_detail",
		Description: "Get the full TSMC job description for a job id (from search results).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in tsmcDetailInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.JobDetail(ctx, in.JobID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(formatTSMCDetail(resp)), nil, nil
	})
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/jobmcp/`
Expected: PASS (all four tests).

- [ ] **Step 5: Vet**

Run: `go vet ./internal/jobmcp/`
Expected: no output.

---

### Task 3: cmd/jobmcp entrypoint + stdio smoke test

**Files:**
- Create: `cmd/jobmcp/main.go`

**Interfaces:**
- Consumes: `jobmcp.RegisterJob104`, `jobmcp.RegisterTSMC`.

- [ ] **Step 1: Implement `cmd/jobmcp/main.go`**

```go
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	job104 "github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/amikai/job-mcp/internal/jobmcp"
	"github.com/amikai/job-mcp/internal/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Shared client: one connection pool, 30s ceiling per request so a hung
	// upstream fails that call instead of stalling the session.
	hc := &http.Client{Timeout: 30 * time.Second}

	c104 := job104.NewClient(job104.Config{HTTPClient: hc})
	cTSMC := tsmc.NewClient(tsmc.Config{HTTPClient: hc})

	server := mcp.NewServer(&mcp.Implementation{Name: "job-mcp", Version: "v0.1.0"}, nil)
	jobmcp.RegisterJob104(server, c104)
	jobmcp.RegisterTSMC(server, cTSMC)

	// Run over stdin/stdout until the client disconnects (stdin EOF).
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Build the binary**

Run: `go build ./cmd/jobmcp/`
Expected: builds, produces `jobmcp` binary, no errors.

- [ ] **Step 3: Smoke-test the stdio server lists all 4 tools**

Run:
```bash
printf '%s\n%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | go run ./cmd/jobmcp/
```
Expected: JSON-RPC responses; the `tools/list` result contains all four names: `104_search_jobs`, `104_get_job_detail`, `tsmc_search_jobs`, `tsmc_get_job_detail`. (Process exits on stdin EOF.)

- [ ] **Step 4: Full build + test sweep**

Run: `go build ./... && go test ./...`
Expected: everything builds; all tests pass (existing client tests + new jobmcp tests).

---

## Self-Review

**Spec coverage:**
- Per-board option B, stdio, go-sdk → Tasks 1–3. ✓
- 4 tools with exact names → Constraints + Tasks 1–2. ✓
- 104 human enums (area/job_type/sort/remote) mapped to codes → Task 1 `job104ToRequest`. ✓
- 104 Edu/S9 omitted → not in `job104SearchInput`. ✓
- tsmc human enums for all 4 facets → Task 2 maps. ✓ PerPage/Organization omitted. ✓
- Reuse `Format*` for 104, inline format for tsmc → Tasks 1–2. ✓
- Errors as `IsError` result → `errorResult`, used in every handler. ✓
- Shared `*http.Client`, built once, 30s timeout → Task 3. ✓
- stdio lifetime, exit on EOF → Task 3 `server.Run`. ✓
- Tests reuse fixtures / no live upstream → mapper unit tests are pure; smoke test hits no upstream (only lists tools). ✓

**Placeholder scan:** No TBD/TODO; all code blocks complete. ✓

**Type consistency:** `job104SearchInput`/`job104DetailInput`/`job104ToRequest`, `tsmcSearchInput`/`tsmcDetailInput`/`tsmcToRequest`, `RegisterJob104`/`RegisterTSMC`, `textResult`/`errorResult` used consistently across tasks. Handler signature identical everywhere. ✓

**Deferred (not in this plan, per spec):** synopsys/google/cake tools, cache/limiter, HTTP transport, fan-out, structured output, 104 company tools.
