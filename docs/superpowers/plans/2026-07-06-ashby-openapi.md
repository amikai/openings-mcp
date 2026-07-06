# Ashby Provider OpenAPI Spec Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `internal/provider/ashby/openapi.yaml`, a hand-written OpenAPI 3.1 spec for Ashby's public job posting API, validated against ogen codegen and live captured responses.

**Architecture:** Ashby's API is one unauthenticated endpoint, `GET https://api.ashbyhq.com/posting-api/job-board/{jobBoardName}`, returning every listed job for an organization (full descriptions included) in a single response. The spec models that endpoint plus its optional `includeCompensation=true` compensation payload. Validation is two-stage: ogen must generate a compiling client from the spec, and the generated types must decode + validate real captured responses. Generated code is validation scaffolding only and is deleted before commit — codegen lands in a later round.

**Tech Stack:** OpenAPI 3.1 YAML, ogen v1.22 (already a `go tool` dependency of this repo), Go test for the decode check, `curl` + `python3` for fixture capture.

**Spec:** `docs/superpowers/specs/2026-07-06-ashby-openapi-design.md`

## Global Constraints

- **Never run `git commit` without the user's explicit go-ahead.** At each commit step, stop and ask the user first (standing user preference; overrides any skill default).
- ogen v1.22 rejects OpenAPI 3.1 `type: [x, "null"]` unions — express nullability with the OAS 3.0 `nullable: true` keyword (precedent: comment in `internal/provider/workday/openapi.yaml:237`).
- `workplaceType` and `employmentType` are strict enums (docs enumerate them exhaustively); `compensationType` and `interval` stay plain strings (docs mark those lists open with "and others").
- Only `internal/provider/ashby/openapi.yaml` and the plan/spec docs are committed this round. No `gen.go`, no generated `oas_*_gen.go`, no testdata.
- `$SCRATCH` below means the session scratchpad directory. Set it once per shell: `SCRATCH=<your session scratchpad path>`.

---

### Task 1: Capture live fixtures and write the spec

**Files:**
- Create: `internal/provider/ashby/openapi.yaml`
- Scratch (not committed): `$SCRATCH/ashby-with-comp.json`, `$SCRATCH/ashby-no-comp.json`

**Interfaces:**
- Produces: `internal/provider/ashby/openapi.yaml` with `operationId: getJobBoard` and component schemas `JobBoardResponse`, `JobPosting`, `SecondaryLocation`, `Address`, `PostalAddress`, `Compensation`, `CompensationTier`, `CompensationComponent`. Task 2 decodes fixtures into the ogen-generated `JobBoardResponse` Go type.
- Produces: the two fixture JSON files consumed by Task 2's decode test.

- [ ] **Step 1: Capture live responses (these are the test fixtures)**

```bash
curl -sf "https://api.ashbyhq.com/posting-api/job-board/ashby?includeCompensation=true" -o "$SCRATCH/ashby-with-comp.json"
curl -sf "https://api.ashbyhq.com/posting-api/job-board/ashby" -o "$SCRATCH/ashby-no-comp.json"
```

Expected: both commands exit 0 and the files are non-empty JSON.

- [ ] **Step 2: Sanity-check the fixtures**

```bash
python3 - "$SCRATCH" <<'EOF'
import json, sys
scratch = sys.argv[1]
for name, with_comp in [("ashby-with-comp.json", True), ("ashby-no-comp.json", False)]:
    d = json.load(open(f"{scratch}/{name}"))
    assert isinstance(d["apiVersion"], str), "apiVersion must be a JSON string"
    jobs = d["jobs"]
    assert jobs, "expected at least one job"
    required = {"title", "isRemote", "workplaceType", "employmentType",
                "publishedAt", "jobUrl", "applyUrl", "isListed"}
    for j in jobs:
        missing = required - j.keys()
        assert not missing, f"{name}: job missing required fields {missing}"
        assert ("compensation" in j) == with_comp, f"{name}: unexpected compensation presence"
    print(f"{name}: OK ({len(jobs)} jobs)")
EOF
```

Expected: two `OK` lines, no assertion errors. If an assertion fails, the upstream API changed since the spec was designed — stop and report instead of writing the yaml.

- [ ] **Step 3: Write `internal/provider/ashby/openapi.yaml`**

```yaml
openapi: 3.1.0

info:
  title: Ashby Public Job Posting API
  description: >
    Client surface for Ashby's public job posting API
    (https://developers.ashbyhq.com/docs/public-job-posting-api). Ashby is a
    multi-tenant ATS: each customer organization hosts a public job board at
    `https://jobs.ashbyhq.com/{jobBoardName}`, and the same `jobBoardName`
    path segment selects the organization here.

    The API is a single unauthenticated endpoint that returns every listed
    job for the organization in one response, each job already carrying its
    full HTML and plain-text description. There is no server-side search,
    filtering, pagination, or separate detail endpoint - callers filter
    client-side.

    Observed behaviors (live `ashby` board, 2026-07-06): `apiVersion` is the
    JSON string "1"; an unknown board name returns HTTP 404 with the
    plain-text body `Not Found`; fields without data are omitted from the
    response rather than sent as null.
  version: "1.0.0"

servers:
  - url: https://api.ashbyhq.com
    description: Production

tags:
  - name: Jobs
    description: Public job board listing

paths:
  /posting-api/job-board/{jobBoardName}:
    get:
      tags: [Jobs]
      summary: List all job postings on an organization's job board
      description: >
        Returns every listed job posting for the organization in a single
        response. Each posting includes the full description, so there is no
        follow-up detail request.
      operationId: getJobBoard
      parameters:
        - name: jobBoardName
          in: path
          required: true
          description: >
            The organization's job board slug - the trailing path segment of
            its Ashby-hosted board URL
            `https://jobs.ashbyhq.com/{jobBoardName}`.
          schema:
            type: string
          example: ashby
        - name: includeCompensation
          in: query
          required: false
          description: >
            When true, each job posting additionally carries a
            `compensation` object with structured salary/equity/bonus data.
          schema:
            type: boolean
      responses:
        "200":
          description: The organization's job board.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/JobBoardResponse"
        "404":
          description: No job board exists with the given name.
          content:
            text/plain:
              schema:
                type: string
                example: Not Found

components:
  schemas:
    JobBoardResponse:
      type: object
      required: [apiVersion, jobs]
      properties:
        apiVersion:
          type: string
          description: API version. Observed as the JSON string "1".
          example: "1"
        jobs:
          type: array
          description: Every listed job posting for the organization.
          items:
            $ref: "#/components/schemas/JobPosting"

    JobPosting:
      type: object
      description: >
        One published job posting. Per the official docs, fields without
        data are omitted from the response rather than sent as null.
      required:
        - title
        - isRemote
        - workplaceType
        - employmentType
        - publishedAt
        - jobUrl
        - applyUrl
        - isListed
      properties:
        id:
          type: string
          description: >
            Job posting UUID. Not listed in the official field reference but
            observed on every job; `jobUrl` and `applyUrl` embed it.
          example: 7458d4e9-da2e-47bd-98cb-adfda43d42b2
        title:
          type: string
          description: Job title.
          example: Engineering Manager, EU
        department:
          type: string
          description: Department the job belongs to.
          example: Engineering
        team:
          type: string
          description: Immediate team the job belongs to.
          example: EMEA Engineering
        employmentType:
          type: string
          enum: [FullTime, PartTime, Intern, Contract, Temporary]
          description: Employment type. The docs enumerate this set exhaustively.
        location:
          type: string
          description: Primary location display name.
          example: Remote - European Union
        secondaryLocations:
          type: array
          description: Additional locations the job can be worked from.
          items:
            $ref: "#/components/schemas/SecondaryLocation"
        publishedAt:
          type: string
          format: date-time
          description: When the job was (last) published.
          example: "2024-03-04T14:29:08.532+00:00"
        isListed:
          type: boolean
          description: Whether the job appears on the public job board listing.
        isRemote:
          type: boolean
          description: Whether the job is remote.
        workplaceType:
          type: string
          enum: [OnSite, Remote, Hybrid]
          description: Workplace arrangement. The docs enumerate this set exhaustively.
        address:
          $ref: "#/components/schemas/Address"
        jobUrl:
          type: string
          description: URL of the Ashby-hosted job posting page.
          example: https://jobs.ashbyhq.com/ashby/7458d4e9-da2e-47bd-98cb-adfda43d42b2
        applyUrl:
          type: string
          description: URL of the Ashby-hosted application page.
          example: https://jobs.ashbyhq.com/ashby/7458d4e9-da2e-47bd-98cb-adfda43d42b2/application
        descriptionHtml:
          type: string
          description: Full job description as HTML.
        descriptionPlain:
          type: string
          description: Full job description as plain text.
        shouldDisplayCompensationOnJobPostings:
          type: boolean
          description: >
            Not listed in the official field reference; observed on every
            job when `includeCompensation=true` is requested.
        compensation:
          allOf:
            - $ref: "#/components/schemas/Compensation"
          description: >
            Structured compensation data. Present only when the request set
            `includeCompensation=true`.

    SecondaryLocation:
      type: object
      properties:
        location:
          type: string
          description: Location display name.
          example: Spain
        address:
          $ref: "#/components/schemas/Address"

    Address:
      type: object
      description: Wrapper object holding a postal address.
      properties:
        postalAddress:
          $ref: "#/components/schemas/PostalAddress"

    PostalAddress:
      type: object
      description: >
        Schema.org-style postal address. Observed to use empty strings for
        parts the organization did not fill in.
      properties:
        addressLocality:
          type: string
          description: City.
        addressRegion:
          type: string
          description: State / province.
        addressCountry:
          type: string
          description: Country.
        postalCode:
          type: string
          description: Postal code.

    Compensation:
      type: object
      description: Structured compensation data for one job posting.
      properties:
        compensationTierSummary:
          type: string
          description: Human-readable summary across all tiers.
          example: "€76K – €185K • Offers Equity • Offers Bonus"
        scrapeableCompensationSalarySummary:
          type: string
          description: Salary-only summary intended for scrapers.
          example: "€76K - €185K"
        compensationTiers:
          type: array
          description: One entry per compensation tier (e.g. per geographic zone).
          items:
            $ref: "#/components/schemas/CompensationTier"
        summaryComponents:
          type: array
          description: >
            Components summarizing compensation across all tiers. Entries
            omit `id` and `summary`.
          items:
            $ref: "#/components/schemas/CompensationComponent"

    CompensationTier:
      type: object
      properties:
        id:
          type: string
          description: Tier identifier (UUID).
        title:
          type: string
          description: Tier name.
          example: EU
        tierSummary:
          type: string
          description: Human-readable summary of this tier.
          example: "€76K – €185K • Offers Equity • Offers Bonus"
        additionalInformation:
          type: string
          nullable: true
          description: Extra details. Observed as null when absent.
        components:
          type: array
          items:
            $ref: "#/components/schemas/CompensationComponent"

    CompensationComponent:
      type: object
      description: >
        One compensation component. `id` and `summary` appear on tier
        components but are omitted from `summaryComponents` entries.
      properties:
        id:
          type: string
          description: Component identifier (UUID).
        summary:
          type: string
          description: Human-readable summary of this component.
          example: "€76K – €185K"
        compensationType:
          type: string
          description: >
            Component kind. Known values include Salary, EquityPercentage,
            and Bonus, but the official docs mark the list as open ("and
            others"), so this is deliberately not an enum.
        interval:
          type: string
          description: >
            Payout interval. Known values include "1 YEAR" and "NONE", but
            the official docs mark the list as open ("and others"), so this
            is deliberately not an enum.
          example: 1 YEAR
        currencyCode:
          type: string
          nullable: true
          description: ISO 4217 currency code; null for non-monetary components.
          example: EUR
        minValue:
          type: number
          nullable: true
          description: Minimum amount; null when the component has no range.
        maxValue:
          type: number
          nullable: true
          description: Maximum amount; null when the component has no range.
```

- [ ] **Step 4: Generate the client from the spec (this is the spec's compile test)**

Generate in-tree so the output compiles against the repo module's existing ogen dependencies (the generated files are validation scaffolding, removed in Task 2):

```bash
cd <repo root>
go tool github.com/ogen-go/ogen/cmd/ogen --target internal/provider/ashby -package ashby --clean internal/provider/ashby/openapi.yaml
go build ./internal/provider/ashby/
```

Expected: ogen exits 0 (no schema/IR errors), `oas_*_gen.go` files appear next to `openapi.yaml`, and `go build` exits 0. If ogen rejects a construct, fix the yaml (staying within the design decisions above) and rerun — do not delete features to silence errors.

---

### Task 2: Decode-check against live data, clean up, commit

**Files:**
- Create then delete: `internal/provider/ashby/ashby_decode_test.go`
- Delete: `internal/provider/ashby/oas_*_gen.go` (from Task 1 Step 4)
- Keep: `internal/provider/ashby/openapi.yaml`

**Interfaces:**
- Consumes: the ogen-generated `JobBoardResponse` type (methods `UnmarshalJSON([]byte) error` and `Validate() error`, field `Jobs`) and the fixtures `$SCRATCH/ashby-with-comp.json`, `$SCRATCH/ashby-no-comp.json` from Task 1.
- Produces: the committed `internal/provider/ashby/openapi.yaml` — the round's sole deliverable.

- [ ] **Step 1: Write the decode test**

Create `internal/provider/ashby/ashby_decode_test.go` (ephemeral — deleted in Step 3):

```go
package ashby

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDecodeCapturedResponses proves the spec's schemas accept real API
// responses: required fields are truly always present, enums cover the
// observed values, and nullable fields decode.
func TestDecodeCapturedResponses(t *testing.T) {
	scratch := os.Getenv("ASHBY_FIXTURE_DIR")
	if scratch == "" {
		t.Fatal("set ASHBY_FIXTURE_DIR to the directory holding the captured fixtures")
	}
	for _, name := range []string{"ashby-with-comp.json", "ashby-no-comp.json"} {
		data, err := os.ReadFile(filepath.Join(scratch, name))
		if err != nil {
			t.Fatal(err)
		}
		var resp JobBoardResponse
		if err := resp.UnmarshalJSON(data); err != nil {
			t.Fatalf("%s: decode: %v", name, err)
		}
		if err := resp.Validate(); err != nil {
			t.Fatalf("%s: validate: %v", name, err)
		}
		if len(resp.Jobs) == 0 {
			t.Fatalf("%s: decoded zero jobs", name)
		}
	}
}
```

- [ ] **Step 2: Run the decode test**

```bash
cd <repo root>
ASHBY_FIXTURE_DIR="$SCRATCH" go test ./internal/provider/ashby/ -run TestDecodeCapturedResponses -v
```

Expected: PASS. A decode error means a field the spec marks required is missing or mistyped; a validate error means an enum value outside the spec's set. Either way fix `openapi.yaml`, regenerate (Task 1 Step 4 command), and rerun until green.

- [ ] **Step 3: Remove the validation scaffolding**

```bash
cd <repo root>
rm internal/provider/ashby/oas_*_gen.go internal/provider/ashby/ashby_decode_test.go
```

- [ ] **Step 4: Verify only the spec remains**

```bash
ls internal/provider/ashby/
git status --short
```

Expected: the directory contains exactly `openapi.yaml`; `git status` shows only `internal/provider/ashby/openapi.yaml` plus the spec/plan docs as new files.

- [ ] **Step 5: Commit (only after the user explicitly approves — see Global Constraints)**

Ask the user for the go-ahead, then:

```bash
git add internal/provider/ashby/openapi.yaml docs/superpowers/specs/2026-07-06-ashby-openapi-design.md docs/superpowers/plans/2026-07-06-ashby-openapi.md
git commit -m "feat(ashby): add public job posting API OpenAPI spec"
```
