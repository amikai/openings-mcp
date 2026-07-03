# Release Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automate releases of the `jobmcp` MCP server so customers can install it as a Gemini CLI extension or Claude Code plugin, per [issue #26](https://github.com/amikai/job-mcp/issues/26).

**Architecture:** GitHub Releases (built by GoReleaser on `v*` tags) is the single source of binaries. The Gemini extension is the release archive itself (manifest + binary, platform-named per Gemini's convention). The Claude Code plugin lives in this repo as a thin wrapper whose MCP command is a bootstrap script that downloads the right binary from GitHub Releases on first run.

**Tech Stack:** Go 1.26, GoReleaser v2, GitHub Actions, POSIX sh.

## Global Constraints

- Platforms: darwin + linux only, amd64 + arm64 (no Windows).
- Gemini asset naming: `{platform}.{arch}.job-mcp.tar.gz` where platform ∈ {darwin, linux}, arch ∈ {x64, arm64} — note **x64, not amd64**.
- Each release archive must contain `gemini-extension.json` at archive root; its `version` must equal the git tag.
- Repo: `github.com/amikai/job-mcp`; MCP server binary: `jobmcp` (from `./cmd/jobmcp`).
- Work on a new branch off `main` (e.g. `release-pipeline`) — the current working tree has unrelated changes on `cake-openapi-enums`; do not touch them.
- Bootstrap script must be POSIX sh (no bashisms), depend only on `curl`, `tar`, `uname`.

---

### Task 1: Inject version at build time

The server version is hardcoded as `"v0.1.0"` in `cmd/jobmcp/main.go`. Make it a package variable so GoReleaser can stamp it via ldflags.

**Files:**
- Modify: `cmd/jobmcp/main.go` (the `newServer` function, ~line 71)
- Test: `cmd/jobmcp/main_test.go`

**Interfaces:**
- Produces: package var `version string = "dev"` in `package main`; GoReleaser (Task 2) sets it with `-X main.version={{.Tag}}`.

- [ ] **Step 1: Write the failing test**

Add to `cmd/jobmcp/main_test.go` (adapt imports to the existing file):

```go
func TestServerVersion(t *testing.T) {
	// version defaults to "dev"; release builds override it via ldflags.
	assert.Equal(t, "dev", version)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/jobmcp -run TestServerVersion -v`
Expected: FAIL — `undefined: version`

- [ ] **Step 3: Implement**

In `cmd/jobmcp/main.go`, add a package-level var and use it in `newServer`:

```go
// version is stamped by GoReleaser via -ldflags "-X main.version=...".
var version = "dev"
```

and change the `mcp.Implementation` literal:

```go
server := mcp.NewServer(&mcp.Implementation{Name: "job-mcp", Version: version},
	&mcp.ServerOptions{Logger: logger})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/jobmcp -v`
Expected: PASS (all tests, including existing ones)

- [ ] **Step 5: Commit**

```bash
git add cmd/jobmcp/main.go cmd/jobmcp/main_test.go
git commit -m "feat(release): stamp server version via ldflags"
```

---

### Task 2: GoReleaser config + Gemini extension manifest

GoReleaser builds the 4 binaries and packages each into a Gemini-convention archive containing the binary plus a generated `gemini-extension.json`.

**Files:**
- Create: `.goreleaser.yaml`
- Create: `scripts/gen-gemini-manifest.sh`
- Modify: `.gitignore` (add `dist/`)

**Interfaces:**
- Consumes: `main.version` var from Task 1.
- Produces: release assets named `darwin.arm64.job-mcp.tar.gz`, `darwin.x64.job-mcp.tar.gz`, `linux.arm64.job-mcp.tar.gz`, `linux.x64.job-mcp.tar.gz`, each containing `jobmcp` + `gemini-extension.json` at archive root; plus `checksums.txt`. Task 3's workflow runs `goreleaser release`; Task 5 documents these names.

- [ ] **Step 1: Write the manifest generator script**

Create `scripts/gen-gemini-manifest.sh`:

```sh
#!/bin/sh
# Generates the Gemini CLI extension manifest for a release tag.
# Usage: gen-gemini-manifest.sh <tag>
set -eu

TAG="$1"
OUT_DIR="dist/manifest"
mkdir -p "$OUT_DIR"

# ${extensionPath} and ${/} are Gemini CLI variables, expanded at load
# time on the customer's machine — they must land literally in the JSON.
cat > "$OUT_DIR/gemini-extension.json" <<EOF
{
  "name": "job-mcp",
  "version": "${TAG}",
  "description": "MCP server for searching Taiwan job listings (104, TSMC, and more)",
  "mcpServers": {
    "job-mcp": {
      "command": "\${extensionPath}\${/}jobmcp"
    }
  }
}
EOF
```

Then: `chmod +x scripts/gen-gemini-manifest.sh`

- [ ] **Step 2: Verify the script output is valid JSON with the tag**

Run:
```bash
./scripts/gen-gemini-manifest.sh v9.9.9 && python3 -m json.tool dist/manifest/gemini-extension.json
```
Expected: pretty-printed JSON, `"version": "v9.9.9"`, and `"command": "${extensionPath}${/}jobmcp"` (literal, unexpanded).

- [ ] **Step 3: Write `.goreleaser.yaml`**

```yaml
version: 2

project_name: jobmcp

before:
  hooks:
    - ./scripts/gen-gemini-manifest.sh {{ .Tag }}

builds:
  - main: ./cmd/jobmcp
    binary: jobmcp
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{ .Tag }}

archives:
  # Gemini CLI extension convention: {platform}.{arch}.{name}.tar.gz with
  # arch spelled x64/arm64, and gemini-extension.json at the archive root.
  - formats: [tar.gz]
    name_template: '{{ .Os }}.{{ if eq .Arch "amd64" }}x64{{ else }}{{ .Arch }}{{ end }}.job-mcp'
    files:
      - src: dist/manifest/gemini-extension.json
        strip_parent: true

checksum:
  name_template: checksums.txt

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs'
      - '^test'
      - '^chore'
```

- [ ] **Step 4: Add `dist/` to `.gitignore`**

Append to `.gitignore`:

```
# goreleaser output
dist/
```

- [ ] **Step 5: Validate config and do a full local snapshot build**

Run:
```bash
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```
Expected: `check` reports the config is valid; snapshot build produces 4 archives under `dist/`.

- [ ] **Step 6: Verify archive contents and naming**

Run:
```bash
ls dist/*.tar.gz
tar -tzf dist/darwin.arm64.job-mcp.tar.gz
```
Expected: exactly `darwin.arm64.job-mcp.tar.gz`, `darwin.x64.job-mcp.tar.gz`, `linux.arm64.job-mcp.tar.gz`, `linux.x64.job-mcp.tar.gz`; each archive lists `jobmcp` and `gemini-extension.json` at the root (no directory prefix). If `gemini-extension.json` appears under a `manifest/` prefix, `strip_parent` didn't apply — fix the `files` entry before proceeding.

- [ ] **Step 7: Commit**

```bash
git add .goreleaser.yaml scripts/gen-gemini-manifest.sh .gitignore
git commit -m "feat(release): goreleaser config producing gemini extension archives"
```

---

### Task 3: Release GitHub Actions workflow

**Files:**
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: `.goreleaser.yaml` from Task 2.
- Produces: a GitHub Release with the 4 archives + checksums on every `v*` tag push.

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: stable

      - name: Unit tests
        run: make ut

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: Lint the workflow**

Run: `actionlint .github/workflows/release.yml` if available; otherwise validate YAML parses:
```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: release workflow running goreleaser on v* tags"
```

---

### Task 4: Claude Code plugin (manifest + bootstrap script)

The plugin lives in this repo. Its MCP server command is a bootstrap script that resolves the latest release, downloads the platform archive into a user cache dir, and execs the binary. Installed via `/plugin marketplace add amikai/job-mcp`.

**Files:**
- Create: `.claude-plugin/plugin.json`
- Create: `.claude-plugin/marketplace.json`
- Create: `bin/run.sh`

**Interfaces:**
- Consumes: release asset names from Task 2 (`{platform}.{x64|arm64}.job-mcp.tar.gz`).
- Produces: installable Claude Code plugin named `job-mcp`.

- [ ] **Step 1: Write the bootstrap script**

Create `bin/run.sh`:

```sh
#!/bin/sh
# Bootstrap for the job-mcp Claude Code plugin: downloads the release
# binary for this platform on first run, caches it, and execs it.
# Env: JOBMCP_BOOTSTRAP_DRYRUN=1 prints the resolved URL and exits
# (used by tests; no network download, no exec).
set -eu

REPO="amikai/job-mcp"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/job-mcp"

platform=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$platform" in
  darwin|linux) ;;
  *) echo "job-mcp: unsupported platform: $platform" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64) arch=x64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "job-mcp: unsupported arch: $arch" >&2; exit 1 ;;
esac

tag=$(curl -fsSL --max-time 10 "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
  | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' || true)

# Offline or rate-limited: fall back to the newest cached version.
if [ -z "$tag" ]; then
  tag=$(ls -1 "$CACHE_DIR" 2>/dev/null | sort -V | tail -1)
fi
if [ -z "$tag" ]; then
  echo "job-mcp: cannot reach GitHub and no cached binary exists" >&2
  exit 1
fi

url="https://github.com/${REPO}/releases/download/${tag}/${platform}.${arch}.job-mcp.tar.gz"

if [ "${JOBMCP_BOOTSTRAP_DRYRUN:-}" = "1" ]; then
  echo "$url"
  exit 0
fi

bin="$CACHE_DIR/$tag/jobmcp"
if [ ! -x "$bin" ]; then
  mkdir -p "$CACHE_DIR/$tag"
  curl -fsSL "$url" | tar -xz -C "$CACHE_DIR/$tag" jobmcp
fi

exec "$bin" "$@"
```

Then: `chmod +x bin/run.sh`

- [ ] **Step 2: Verify the script (syntax + dry run)**

Run:
```bash
sh -n bin/run.sh
JOBMCP_BOOTSTRAP_DRYRUN=1 sh bin/run.sh
```
Expected: `sh -n` silent; dry run prints a URL like `https://github.com/amikai/job-mcp/releases/download/<tag>/darwin.arm64.job-mcp.tar.gz` (tag from the latest GitHub release — if no release exists yet and the cache is empty, it exits 1 with the "cannot reach GitHub and no cached binary" message, which is also acceptable at this stage). Run `shellcheck bin/run.sh` too if installed.

- [ ] **Step 3: Write the plugin manifest**

Create `.claude-plugin/plugin.json`:

```json
{
  "name": "job-mcp",
  "description": "MCP server for searching Taiwan job listings (104, TSMC, and more)",
  "author": { "name": "amikai" },
  "repository": "https://github.com/amikai/job-mcp",
  "mcpServers": {
    "job-mcp": {
      "command": "${CLAUDE_PLUGIN_ROOT}/bin/run.sh"
    }
  }
}
```

- [ ] **Step 4: Write the marketplace manifest**

Create `.claude-plugin/marketplace.json`:

```json
{
  "name": "job-mcp",
  "owner": { "name": "amikai" },
  "plugins": [
    {
      "name": "job-mcp",
      "source": "./",
      "description": "MCP server for searching Taiwan job listings (104, TSMC, and more)"
    }
  ]
}
```

- [ ] **Step 5: Validate both manifests parse**

Run:
```bash
python3 -m json.tool .claude-plugin/plugin.json
python3 -m json.tool .claude-plugin/marketplace.json
```
Expected: both pretty-print without error.

- [ ] **Step 6: Commit**

```bash
git add .claude-plugin bin/run.sh
git commit -m "feat(release): claude code plugin with release-download bootstrap"
```

---

### Task 5: Installation docs

**Files:**
- Create: `README.md`

**Interfaces:**
- Consumes: install commands enabled by Tasks 2–4.

- [ ] **Step 1: Write the README**

Create `README.md`:

```markdown
# job-mcp

MCP server for searching Taiwan job listings (104, TSMC, and more).

## Install

### Gemini CLI

```
gemini extensions install https://github.com/amikai/job-mcp
```

Upgrade with `gemini extensions update job-mcp`.

### Claude Code

```
/plugin marketplace add amikai/job-mcp
/plugin install job-mcp@job-mcp
```

The plugin downloads the release binary for your platform on first run
(cached under `~/.cache/job-mcp`). New releases are picked up automatically
the next time the server starts.

### Codex / manual

Download the archive for your platform from
[Releases](https://github.com/amikai/job-mcp/releases), extract `jobmcp`
somewhere on your PATH, then:

```
codex mcp add job-mcp -- jobmcp
```

With Go installed: `go install github.com/amikai/job-mcp/cmd/jobmcp@latest`

## Supported platforms

macOS and Linux, amd64/arm64.

## Configuration

`JOBMCP_LOG_LEVEL` — log level (`debug`/`info`/`warn`/`error`, default `info`).
Logs go to stderr.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: installation instructions for gemini, claude code, codex"
```

---

### Task 6: First release verification (manual, after merge)

Not code — an operator checklist once the PR merges to `main`.

- [ ] **Step 1: Tag and push**

```bash
git checkout main && git pull
git tag v0.2.0
git push origin v0.2.0
```

- [ ] **Step 2: Watch the workflow**

Run: `gh run watch` (or check the Actions tab).
Expected: Release workflow succeeds; the release page shows 4 archives + `checksums.txt`.

- [ ] **Step 3: Verify Gemini install**

```bash
gemini extensions install https://github.com/amikai/job-mcp
gemini extensions list
```
Expected: `job-mcp` installed at `v0.2.0`; MCP server starts in a Gemini session.

- [ ] **Step 4: Verify Claude Code plugin install**

In Claude Code:
```
/plugin marketplace add amikai/job-mcp
/plugin install job-mcp@job-mcp
```
Expected: plugin installs; on first MCP start the bootstrap downloads `{platform}.{arch}.job-mcp.tar.gz` and the `job-mcp` tools appear.

- [ ] **Step 5: Close issue #26**

```bash
gh issue close 26 --comment "Shipped: goreleaser + release workflow, gemini extension archives, claude code plugin. Verified installs on v0.2.0."
```
