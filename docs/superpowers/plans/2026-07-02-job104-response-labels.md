# job104 Response Label Conversion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert 104 tool responses from raw numeric codes to the same labels the tool inputs use (`jobRo`→`job_type`, `remoteWorkType`/`remoteWork`→`remote`), for both `104_search_jobs` and `104_get_job_detail`.

**Architecture:** MCP-facing output structs in `internal/jobmcp/job104.go` mirror the provider (ogen) response types 1:1; two pure converter functions map provider → output; reverse label maps are inverted from `job104.RoIDs` / `job104.RemoteWorkIDs` so `ids.go` stays the single source of truth. Unknown codes leave the label empty and `omitempty` drops the field.

**Tech Stack:** Go, testify, modelcontextprotocol/go-sdk, ogen-generated provider in `internal/provider/job104`.

## Global Constraints

- Work on branch `job104-response-labels`, never main.
- Test wants are hand-typed literals; golden whole-value compare: one full `want` + a single `assert.Equal` per test.
- No shared flow helpers: keep CallTool → require → decode → assert inline in each test.
- Struct init style: omit zero-value fields except in test literals where names clarify intent.
- Provider package (`internal/provider/job104`) and its `client_test.go` are untouched.
- Spec: `docs/superpowers/specs/2026-07-02-job104-response-labels-design.md`.

---

### Task 1: Search output types + reverse maps + converter

**Files:**
- Modify: `internal/jobmcp/job104.go` (append after `job104MCPToHTTPRequest`)
- Test: `internal/jobmcp/job104_test.go`

**Interfaces:**
- Consumes: `job104.JobsResponse`, `job104.RoIDs`, `job104.RemoteWorkIDs`, `job104.SearchJobsRo` (int), `job104.SearchJobsRemoteWork` (int).
- Produces: `job104SearchOutput` / `job104JobSummary` / `job104JobSummaryLink` / `job104SearchMetadata` / `job104Pagination` structs; `job104HTTPToMCPResponse(*job104.JobsResponse) *job104SearchOutput`; package vars `job104RoLabels map[job104.SearchJobsRo]string`, `job104RemoteWorkLabels map[job104.SearchJobsRemoteWork]string`. Tasks 2–4 rely on these exact names.

- [ ] **Step 1: Write the failing test** (append to `internal/jobmcp/job104_test.go`)

```go
func TestJob104HTTPToMCPResponse(t *testing.T) {
	in := job104.JobsResponse{
		Data: []job104.JobSummary{
			{JobNo: "1", JobName: "onsite", CustName: "c1", CustNo: "n1", Link: job104.JobSummaryLink{Job: "j1", Cust: "u1"}, SalaryHigh: 2, SalaryLow: 1, JobAddrNoDesc: "a1", AppearDate: "20260101", ApplyCnt: 3, RemoteWorkType: 0, JobRo: 1},
			{JobNo: "2", JobName: "full-remote", CustName: "c2", CustNo: "n2", Link: job104.JobSummaryLink{Job: "j2", Cust: "u2"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "a2", AppearDate: "20260102", ApplyCnt: 4, RemoteWorkType: 1, JobRo: 2},
			{JobNo: "3", JobName: "hybrid", CustName: "c3", CustNo: "n3", Link: job104.JobSummaryLink{Job: "j3", Cust: "u3"}, SalaryHigh: 9, SalaryLow: 5, JobAddrNoDesc: "a3", AppearDate: "20260103", ApplyCnt: 5, RemoteWorkType: 2, JobRo: 4},
			{JobNo: "4", JobName: "unknown-codes", CustName: "c4", CustNo: "n4", Link: job104.JobSummaryLink{Job: "j4", Cust: "u4"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "a4", AppearDate: "20260104", ApplyCnt: 6, RemoteWorkType: 9, JobRo: 9},
		},
		Metadata: job104.JobsResponseMetadata{
			Pagination: job104.JobsResponseMetadataPagination{CurrentPage: 1, LastPage: 2, Total: 34},
		},
	}
	got := job104HTTPToMCPResponse(&in)

	// Unknown codes (jobRo 9, remoteWorkType 9) map to no label at all.
	want := &job104SearchOutput{
		Data: []job104JobSummary{
			{JobNo: "1", JobName: "onsite", CustName: "c1", CustNo: "n1", Link: job104JobSummaryLink{Job: "j1", Cust: "u1"}, SalaryHigh: 2, SalaryLow: 1, JobAddrNoDesc: "a1", AppearDate: "20260101", ApplyCnt: 3, JobType: "Full-time"},
			{JobNo: "2", JobName: "full-remote", CustName: "c2", CustNo: "n2", Link: job104JobSummaryLink{Job: "j2", Cust: "u2"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "a2", AppearDate: "20260102", ApplyCnt: 4, Remote: "Full", JobType: "Part-time"},
			{JobNo: "3", JobName: "hybrid", CustName: "c3", CustNo: "n3", Link: job104JobSummaryLink{Job: "j3", Cust: "u3"}, SalaryHigh: 9, SalaryLow: 5, JobAddrNoDesc: "a3", AppearDate: "20260103", ApplyCnt: 5, Remote: "Partial", JobType: "Dispatch"},
			{JobNo: "4", JobName: "unknown-codes", CustName: "c4", CustNo: "n4", Link: job104JobSummaryLink{Job: "j4", Cust: "u4"}, SalaryHigh: 0, SalaryLow: 0, JobAddrNoDesc: "a4", AppearDate: "20260104", ApplyCnt: 6},
		},
		Metadata: job104SearchMetadata{
			Pagination: job104Pagination{CurrentPage: 1, LastPage: 2, Total: 34},
		},
	}
	assert.Equal(t, want, got)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/jobmcp/ -run TestJob104HTTPToMCPResponse`
Expected: FAIL (build error: `undefined: job104HTTPToMCPResponse`, `undefined: job104SearchOutput`)

- [ ] **Step 3: Write minimal implementation** (append to `internal/jobmcp/job104.go`)

```go
// job104SearchOutput mirrors job104.JobsResponse for the LLM: identical
// fields and JSON names, except the coded jobRo/remoteWorkType become the
// job_type/remote labels used by the search input params. Unknown codes
// leave the label empty and omitempty drops the field.
type job104SearchOutput struct {
	Data     []job104JobSummary   `json:"data"`
	Metadata job104SearchMetadata `json:"metadata"`
}

type job104JobSummary struct {
	JobNo         string               `json:"jobNo"`
	JobName       string               `json:"jobName"`
	CustName      string               `json:"custName"`
	CustNo        string               `json:"custNo"`
	Link          job104JobSummaryLink `json:"link"`
	SalaryHigh    int                  `json:"salaryHigh"`
	SalaryLow     int                  `json:"salaryLow"`
	JobAddrNoDesc string               `json:"jobAddrNoDesc"`
	AppearDate    string               `json:"appearDate"`
	ApplyCnt      int                  `json:"applyCnt"`
	Remote        string               `json:"remote,omitempty"`
	JobType       string               `json:"job_type,omitempty"`
}

type job104JobSummaryLink struct {
	Job  string `json:"job"`
	Cust string `json:"cust"`
}

type job104SearchMetadata struct {
	Pagination job104Pagination `json:"pagination"`
}

type job104Pagination struct {
	CurrentPage int `json:"currentPage"`
	LastPage    int `json:"lastPage"`
	Total       int `json:"total"`
}

// job104RoLabels and job104RemoteWorkLabels invert the ids.go request maps
// for response conversion, keeping ids.go the single source of truth.
var job104RoLabels = func() map[job104.SearchJobsRo]string {
	m := make(map[job104.SearchJobsRo]string, len(job104.RoIDs))
	for label, code := range job104.RoIDs {
		m[code] = label
	}
	return m
}()

var job104RemoteWorkLabels = func() map[job104.SearchJobsRemoteWork]string {
	m := make(map[job104.SearchJobsRemoteWork]string, len(job104.RemoteWorkIDs))
	for label, code := range job104.RemoteWorkIDs {
		m[code] = label
	}
	return m
}()

func job104HTTPToMCPResponse(resp *job104.JobsResponse) *job104SearchOutput {
	out := &job104SearchOutput{
		Data: make([]job104JobSummary, 0, len(resp.Data)),
		Metadata: job104SearchMetadata{
			Pagination: job104Pagination{
				CurrentPage: resp.Metadata.Pagination.CurrentPage,
				LastPage:    resp.Metadata.Pagination.LastPage,
				Total:       resp.Metadata.Pagination.Total,
			},
		},
	}
	for _, j := range resp.Data {
		out.Data = append(out.Data, job104JobSummary{
			JobNo:         j.JobNo,
			JobName:       j.JobName,
			CustName:      j.CustName,
			CustNo:        j.CustNo,
			Link:          job104JobSummaryLink{Job: j.Link.Job, Cust: j.Link.Cust},
			SalaryHigh:    j.SalaryHigh,
			SalaryLow:     j.SalaryLow,
			JobAddrNoDesc: j.JobAddrNoDesc,
			AppearDate:    j.AppearDate,
			ApplyCnt:      j.ApplyCnt,
			Remote:        job104RemoteWorkLabels[job104.SearchJobsRemoteWork(j.RemoteWorkType)],
			JobType:       job104RoLabels[job104.SearchJobsRo(j.JobRo)],
		})
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/jobmcp/ -run TestJob104HTTPToMCPResponse -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/jobmcp/job104.go internal/jobmcp/job104_test.go
git commit -m "feat: convert 104 search response codes to labels"
```

---

### Task 2: Detail output types + converter

**Files:**
- Modify: `internal/jobmcp/job104.go` (append after Task 1's code)
- Test: `internal/jobmcp/job104_test.go`

**Interfaces:**
- Consumes: `job104.JobDetailResponse` tree (`JobDetail`, `JobDetailHeader`, `JobDetailContact`, `JobDetailCondition`, `JobDetailWelfare`, `JobDetailJobDetail`, `CodeDescription`, `OptNilJobDetailJobDetailRemoteWork`), plus Task 1's `job104RoLabels` / `job104RemoteWorkLabels`. Opt access: `.Or(zero)`; `RemoteWork.Get()` returns `ok=false` for both null and absent.
- Produces: `job104DetailOutput` / `job104JobDetail` / `job104DetailHeader` / `job104DetailContact` / `job104DetailCondition` / `job104DetailWelfare` / `job104DetailJobDetail` / `job104CodeDescription` structs; `job104HTTPToMCPDetail(*job104.JobDetailResponse) *job104DetailOutput`. Tasks 3–4 rely on these exact names.

- [ ] **Step 1: Write the failing tests** (append to `internal/jobmcp/job104_test.go`)

```go
func TestJob104HTTPToMCPDetail(t *testing.T) {
	in := job104.JobDetailResponse{
		Data: job104.JobDetail{
			Header: job104.JobDetailHeader{JobName: "j", CustName: "c", CustUrl: "u", AppearDate: "2026/01/01", IsSaved: true, IsApplied: false},
			Contact: job104.JobDetailContact{
				HrName: job104.NewOptString("hr"),
				Email:  job104.NewOptString("e@x"),
				Reply:  job104.NewOptString(""),
			},
			Condition: job104.JobDetailCondition{
				WorkExp: job104.NewOptString("exp"),
				Edu:     job104.NewOptString("edu"),
				Major:   []string{"m1"},
				Specialty: []job104.CodeDescription{
					{Code: job104.NewOptString("s1"), Description: job104.NewOptString("d1")},
				},
			},
			Welfare: job104.JobDetailWelfare{Welfare: job104.NewOptString("w")},
			JobDetail: job104.JobDetailJobDetail{
				JobDescription: job104.NewOptString("desc"),
				JobCategory: []job104.CodeDescription{
					{Code: job104.NewOptString("k1"), Description: job104.NewOptString("kd1")},
				},
				Salary:        job104.NewOptString("sal"),
				SalaryMin:     job104.NewOptInt(10),
				SalaryMax:     job104.NewOptInt(20),
				JobType:       job104.NewOptInt(1),
				AddressRegion: job104.NewOptString("region"),
				AddressDetail: job104.NewOptString("detail"),
				ManageResp:    job104.NewOptString("mr"),
				NeedEmp:       job104.NewOptString("ne"),
				RemoteWork: job104.OptNilJobDetailJobDetailRemoteWork{
					Set:   true,
					Value: job104.JobDetailJobDetailRemoteWork{Type: job104.NewOptInt(1), Description: job104.NewOptString("遠端")},
				},
			},
			Industry:  "ind",
			Employees: "9人",
			CustNo:    "cn",
		},
	}
	got := job104HTTPToMCPDetail(&in)

	want := &job104DetailOutput{
		Data: job104JobDetail{
			Header:  job104DetailHeader{JobName: "j", CustName: "c", CustUrl: "u", AppearDate: "2026/01/01", IsSaved: true, IsApplied: false},
			Contact: job104DetailContact{HrName: "hr", Email: "e@x", Reply: ""},
			Condition: job104DetailCondition{
				WorkExp:   "exp",
				Edu:       "edu",
				Major:     []string{"m1"},
				Specialty: []job104CodeDescription{{Code: "s1", Description: "d1"}},
			},
			Welfare: job104DetailWelfare{Welfare: "w"},
			JobDetail: job104DetailJobDetail{
				JobDescription: "desc",
				JobCategory:    []job104CodeDescription{{Code: "k1", Description: "kd1"}},
				Salary:         "sal",
				SalaryMin:      10,
				SalaryMax:      20,
				JobType:        "Full-time",
				AddressRegion:  "region",
				AddressDetail:  "detail",
				ManageResp:     "mr",
				NeedEmp:        "ne",
				Remote:         "Full",
			},
			Industry:  "ind",
			Employees: "9人",
			CustNo:    "cn",
		},
	}
	assert.Equal(t, want, got)
}

func TestJob104HTTPToMCPDetailNullRemoteUnknownJobType(t *testing.T) {
	in := job104.JobDetailResponse{
		Data: job104.JobDetail{
			Header: job104.JobDetailHeader{JobName: "j", CustName: "c", CustUrl: "u", AppearDate: "2026/01/01"},
			JobDetail: job104.JobDetailJobDetail{
				JobType:    job104.NewOptInt(9),
				RemoteWork: job104.OptNilJobDetailJobDetailRemoteWork{Set: true, Null: true},
			},
			Industry:  "ind",
			Employees: "9人",
			CustNo:    "cn",
		},
	}
	got := job104HTTPToMCPDetail(&in)

	// Null remoteWork and unknown jobType code both drop their labels.
	want := &job104DetailOutput{
		Data: job104JobDetail{
			Header:    job104DetailHeader{JobName: "j", CustName: "c", CustUrl: "u", AppearDate: "2026/01/01"},
			Industry:  "ind",
			Employees: "9人",
			CustNo:    "cn",
		},
	}
	assert.Equal(t, want, got)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/jobmcp/ -run TestJob104HTTPToMCPDetail`
Expected: FAIL (build error: `undefined: job104HTTPToMCPDetail`, `undefined: job104DetailOutput`)

- [ ] **Step 3: Write minimal implementation** (append to `internal/jobmcp/job104.go`)

```go
// job104DetailOutput mirrors job104.JobDetailResponse for the LLM: Opt
// fields flatten to plain values with omitempty, and the coded
// jobType/remoteWork become the job_type/remote labels used by the search
// input params. Unknown codes and null remoteWork drop the label.
type job104DetailOutput struct {
	Data job104JobDetail `json:"data"`
}

type job104JobDetail struct {
	Header    job104DetailHeader    `json:"header"`
	Contact   job104DetailContact   `json:"contact"`
	Condition job104DetailCondition `json:"condition"`
	Welfare   job104DetailWelfare   `json:"welfare"`
	JobDetail job104DetailJobDetail `json:"jobDetail"`
	Industry  string                `json:"industry"`
	Employees string                `json:"employees"`
	CustNo    string                `json:"custNo"`
}

type job104DetailHeader struct {
	JobName    string `json:"jobName"`
	CustName   string `json:"custName"`
	CustUrl    string `json:"custUrl"`
	AppearDate string `json:"appearDate"`
	IsSaved    bool   `json:"isSaved"`
	IsApplied  bool   `json:"isApplied"`
}

type job104DetailContact struct {
	HrName string `json:"hrName,omitempty"`
	Email  string `json:"email,omitempty"`
	Reply  string `json:"reply,omitempty"`
}

type job104DetailCondition struct {
	WorkExp   string                  `json:"workExp,omitempty"`
	Edu       string                  `json:"edu,omitempty"`
	Major     []string                `json:"major,omitempty"`
	Specialty []job104CodeDescription `json:"specialty,omitempty"`
}

type job104DetailWelfare struct {
	Welfare string `json:"welfare,omitempty"`
}

type job104DetailJobDetail struct {
	JobDescription string                  `json:"jobDescription,omitempty"`
	JobCategory    []job104CodeDescription `json:"jobCategory,omitempty"`
	Salary         string                  `json:"salary,omitempty"`
	SalaryMin      int                     `json:"salaryMin,omitempty"`
	SalaryMax      int                     `json:"salaryMax,omitempty"`
	JobType        string                  `json:"job_type,omitempty"`
	AddressRegion  string                  `json:"addressRegion,omitempty"`
	AddressDetail  string                  `json:"addressDetail,omitempty"`
	ManageResp     string                  `json:"manageResp,omitempty"`
	NeedEmp        string                  `json:"needEmp,omitempty"`
	Remote         string                  `json:"remote,omitempty"`
}

type job104CodeDescription struct {
	Code        string `json:"code,omitempty"`
	Description string `json:"description,omitempty"`
}

func job104HTTPToMCPDetail(resp *job104.JobDetailResponse) *job104DetailOutput {
	d := resp.Data
	out := &job104DetailOutput{
		Data: job104JobDetail{
			Header: job104DetailHeader{
				JobName:    d.Header.JobName,
				CustName:   d.Header.CustName,
				CustUrl:    d.Header.CustUrl,
				AppearDate: d.Header.AppearDate,
				IsSaved:    d.Header.IsSaved,
				IsApplied:  d.Header.IsApplied,
			},
			Contact: job104DetailContact{
				HrName: d.Contact.HrName.Or(""),
				Email:  d.Contact.Email.Or(""),
				Reply:  d.Contact.Reply.Or(""),
			},
			Condition: job104DetailCondition{
				WorkExp:   d.Condition.WorkExp.Or(""),
				Edu:       d.Condition.Edu.Or(""),
				Major:     d.Condition.Major,
				Specialty: job104CodeDescriptions(d.Condition.Specialty),
			},
			Welfare: job104DetailWelfare{Welfare: d.Welfare.Welfare.Or("")},
			JobDetail: job104DetailJobDetail{
				JobDescription: d.JobDetail.JobDescription.Or(""),
				JobCategory:    job104CodeDescriptions(d.JobDetail.JobCategory),
				Salary:         d.JobDetail.Salary.Or(""),
				SalaryMin:      d.JobDetail.SalaryMin.Or(0),
				SalaryMax:      d.JobDetail.SalaryMax.Or(0),
				JobType:        job104RoLabels[job104.SearchJobsRo(d.JobDetail.JobType.Or(0))],
				AddressRegion:  d.JobDetail.AddressRegion.Or(""),
				AddressDetail:  d.JobDetail.AddressDetail.Or(""),
				ManageResp:     d.JobDetail.ManageResp.Or(""),
				NeedEmp:        d.JobDetail.NeedEmp.Or(""),
			},
			Industry:  d.Industry,
			Employees: d.Employees,
			CustNo:    d.CustNo,
		},
	}
	if rw, ok := d.JobDetail.RemoteWork.Get(); ok {
		out.Data.JobDetail.Remote = job104RemoteWorkLabels[job104.SearchJobsRemoteWork(rw.Type.Or(0))]
	}
	return out
}

func job104CodeDescriptions(in []job104.CodeDescription) []job104CodeDescription {
	if len(in) == 0 {
		return nil
	}
	out := make([]job104CodeDescription, 0, len(in))
	for _, cd := range in {
		out = append(out, job104CodeDescription{Code: cd.Code.Or(""), Description: cd.Description.Or("")})
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/jobmcp/ -run TestJob104HTTPToMCPDetail -v`
Expected: PASS (both tests)

- [ ] **Step 5: Commit**

```bash
git add internal/jobmcp/job104.go internal/jobmcp/job104_test.go
git commit -m "feat: convert 104 job detail response codes to labels"
```

---

### Task 3: Wire handlers, update schema descriptions, update search E2E golden

**Files:**
- Modify: `internal/jobmcp/job104.go` (both tool handlers + `job104SearchInputRawSchema` descriptions)
- Modify: `internal/jobmcp/job104_test.go` (`TestJob104SearchJobE2E` golden)

**Interfaces:**
- Consumes: Task 1's `job104HTTPToMCPResponse` + output types; Task 2's `job104HTTPToMCPDetail`.
- Produces: `104_search_jobs` structured content is `job104SearchOutput` JSON; `104_get_job_detail` structured content is `job104DetailOutput` JSON. Task 4 relies on the detail wiring.

- [ ] **Step 1: Update the search E2E test to the new output shape (failing first)**

In `TestJob104SearchJobE2E` (`internal/jobmcp/job104_test.go`) apply these exact replacements:

1. `var got job104.JobsResponse` → `var got job104SearchOutput`
2. `wantResp := &job104.JobsResponse{` → `wantResp := &job104SearchOutput{`
3. `Data: []job104.JobSummary{` → `Data: []job104JobSummary{`
4. All 30× `Link: job104.JobSummaryLink{` → `Link: job104JobSummaryLink{` (replace-all)
5. Trailing code pairs on every entry (replace-all; every entry has `JobRo: 1`):
   - `, RemoteWorkType: 0, JobRo: 1}` → `, JobType: "Full-time"}`  (26 entries)
   - `, RemoteWorkType: 1, JobRo: 1}` → `, Remote: "Full", JobType: "Full-time"}`  (1 entry: jobNo 13766806)
   - `, RemoteWorkType: 2, JobRo: 1}` → `, Remote: "Partial", JobType: "Full-time"}`  (3 entries: jobNo 15160106, 13761398, 15106548)
6. `Metadata: job104.JobsResponseMetadata{` → `Metadata: job104SearchMetadata{`
7. `Pagination: job104.JobsResponseMetadataPagination{CurrentPage: 1, LastPage: 7, Total: 189},` → `Pagination: job104Pagination{CurrentPage: 1, LastPage: 7, Total: 189},`

Also update the golden schema `want` in the same test — the two descriptions that name wire fields:

- `"description": "Employment basis. Soft filter — verify each result's jobRo.",` → `"description": "Employment basis. Soft filter — verify each result's job_type.",`
- `"description": "Remote work. Soft filter — verify each result's remoteWorkType. Omit for on-site.",` → `"description": "Remote work. Soft filter — verify each result's remote. Omit for on-site.",`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/jobmcp/ -run TestJob104SearchJobE2E`
Expected: FAIL — schema description mismatch AND structured content still carries `jobRo`/`remoteWorkType` (handler not wired yet)

- [ ] **Step 3: Wire both handlers and the raw schema** (`internal/jobmcp/job104.go`)

In the `104_search_jobs` handler replace:

```go
		return nil, resp, nil
```

with:

```go
		return nil, job104HTTPToMCPResponse(resp), nil
```

In the `104_get_job_detail` handler replace:

```go
		return nil, detail, nil
```

with:

```go
		return nil, job104HTTPToMCPDetail(detail), nil
```

In `job104SearchInputRawSchema` replace:

- `"description": "Employment basis. Soft filter — verify each result's jobRo.",` → `"description": "Employment basis. Soft filter — verify each result's job_type.",`
- `"description": "Remote work. Soft filter — verify each result's remoteWorkType. Omit for on-site.",` → `"description": "Remote work. Soft filter — verify each result's remote. Omit for on-site.",`

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/jobmcp/`
Expected: PASS (all tests in package)

- [ ] **Step 5: Commit**

```bash
git add internal/jobmcp/job104.go internal/jobmcp/job104_test.go
git commit -m "feat: return label-converted 104 responses from MCP tools"
```

---

### Task 4: Detail E2E test

**Files:**
- Test: `internal/jobmcp/job104_test.go` (new `TestJob104GetJobDetailE2E`)

**Interfaces:**
- Consumes: `testClientServer(t)` (mock-backed sessions), Task 2's `job104DetailOutput` types, Task 3's detail handler wiring. Mock fixture: `internal/provider/job104/testdata/job_detail_rsp.json` (jobType 1 → "Full-time"; remoteWork null → no remote; salaryMin/Max 0 → omitted; isSaved/isApplied false; reply empty → "").
- Produces: nothing downstream; final verification task.

- [ ] **Step 1: Write the test** (append to `internal/jobmcp/job104_test.go`; want values hand-typed from the fixture, matching `client_test.go`'s `TestGetJobDetail` literals converted to output shape)

```go
func TestJob104GetJobDetailE2E(t *testing.T) {
	clientSession, _ := testClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "104_get_job_detail",
		Arguments: map[string]any{"job_code": "624o1"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got job104DetailOutput
	require.NoError(t, json.Unmarshal(data, &got))

	want := &job104DetailOutput{
		Data: job104JobDetail{
			Header: job104DetailHeader{
				JobName:    "軟體工程師 (數位工程發展部)",
				CustName:   "亞新工程顧問股份有限公司",
				CustUrl:    "https://www.104.com.tw/company/264c9zc",
				AppearDate: "2026/06/22",
				IsSaved:    false,
				IsApplied:  false,
			},
			Contact: job104DetailContact{
				HrName: "Rachel Chiu 邱小姐",
				Email:  "personnel@maaconsultants.com,cj.yu@maaconsultants.com,eugene.shen@maaconsultants.com,fred.chou@maaconsultants.com",
			},
			Condition: job104DetailCondition{
				WorkExp: "不拘",
				Edu:     "大學以上",
				Major:   []string{"資訊工程相關"},
				Specialty: []job104CodeDescription{
					{Code: "12001003009", Description: "C#"},
					{Code: "12001003006", Description: "ASP.NET"},
					{Code: "12001004031", Description: "MS SQL"},
					{Code: "12001003045", Description: "Python"},
					{Code: "12003001003", Description: "GIS"},
					{Code: "12001003094", Description: "IoT"},
					{Code: "12002003010", Description: "Revit"},
				},
			},
			Welfare: job104DetailWelfare{
				Welfare: "在亞新，我們重視同仁的職涯成長與友善職場，透過全方位的福利與支持，推動以人為本、永續發展的職場環境，實現工作與生活的和諧平衡。\n\n【薪酬與獎金】\n  •  具市場競爭力的薪資水準\n  •  年節獎金與專案獎金，共享成果回饋\n\n【健康與保障】\n  •  勞健保及完整團體保險(意外、醫療、重大疾病、職災保障)\n  •  定期健康檢查、健康講座與員工關懷方案\n\n【休假與彈性】\n  •  彈性上下班、育兒友善措施，兼顧生活平衡\n\n【教育訓練與發展】\n  •  完善新人培訓與師徒制\n  •  E-learning 線上學習資源\n  •  專業證照補助（如 PMP、專業技師等）\n  •  外部訓練與國際研討會，拓展國際視野\n  •  參與國家級重大工程，累積獨特專業經驗\n\n【生活與休閒】\n  •  福委會關懷：生日禮金、節慶禮品或禮券、婚喪喜慶、傷病住院慰問與生育補助\n  •  部門聚餐、咖啡分享日、社團活動、Happy Hour，促進交流與凝聚力\n  •  舒適職場環境：明亮開放空間、零食吧、茶包與自助研磨咖啡機\n\n【招募流程】\n  1. 投遞履歷\n  2. HR初審履歷 → 部門主管面試\n  3. Final面談（含專案介紹與Q&A）\n  4. 錄取通知\n （流程清楚透明，讓你安心應徵!)",
			},
			JobDetail: job104DetailJobDetail{
				JobDescription: "無相關經驗可，大學以上資訊工程、資訊管理等相關科系畢業\n\n【工作內容】\n- 參與智慧工程數位平台的設計、開發與維運\n- 開發與維護 GIS、BIM 系統，並支援無人機地形數據應用\n- 參與 AI 工具與文件管理系統之開發 \n- 與跨領域團隊合作（工程、IoT、BIM、AI），推動數位轉型與自動化流程\n\n【希望條件】\n- 熟悉現代軟體系統研發流程與版本控制\n- 熟悉至少一種指令式程式設計語言（C#、JavaScript、Python、PHP 尤佳）\n- 具 ASP.NET、SQL、Vue.js、Laravel、Unity、GIS、IoT、Revit等開發經驗\n- 具軟體設計、開發、運營、開發、機器學習、AI 模型訓練 (Finetuning)、 AI 應用設計（OCR、RAG、LLM、Agentic 等）開發、導入經驗\n- 具 Azure DevOps、Docker、Kubernetes 經驗者優先\n\n＊我們期待具備高度邏輯思維、善於溝通系統需求與設計選擇，並能獨立完成軟體開發的夥伴加入，一起參與系統規劃與優化。",
				JobCategory: []job104CodeDescription{
					{Code: "2007001004", Description: "軟體工程師"},
				},
				Salary:        "待遇面議",
				JobType:       "Full-time",
				AddressRegion: "新北市汐止區",
				AddressDetail: "新台五路一段112號22樓",
				ManageResp:    "不需負擔管理責任",
				NeedEmp:       "2~3人",
			},
			Industry:  "建築及工程技術服務業",
			Employees: "1200人",
			CustNo:    "264c9zc",
		},
	}
	assert.Equal(t, want, &got)
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/jobmcp/ -run TestJob104GetJobDetailE2E -v`
Expected: PASS (wiring already landed in Task 3; this test locks the detail output contract)

If it fails on a field, compare against `internal/provider/job104/testdata/job_detail_rsp.json` — the fixture is the oracle; fix the hand-typed want, not the converter, unless the converter provably drops/mangles a value.

- [ ] **Step 3: Run the full suite**

Run: `go build ./... && go test ./...`
Expected: all packages PASS

- [ ] **Step 4: Commit**

```bash
git add internal/jobmcp/job104_test.go
git commit -m "test: add 104 job detail E2E golden"
```
