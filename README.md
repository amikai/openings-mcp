# job-mcp

MCP server for searching job listings across job boards and company career sites (104, TSMC, and more).

## Install

With Go:

```
go install github.com/amikai/job-mcp/cmd/jobmcp@latest
```

Upgrade by rerunning the same command; pin a version with `@vX.Y.Z`.

Without Go: download the archive for your platform from
[Releases](https://github.com/amikai/job-mcp/releases) and put `jobmcp` on
your PATH.

## Add the MCP server to your tool

With `jobmcp` on your PATH (`$(go env GOPATH)/bin`):

**Claude Code**

```
claude mcp add job-mcp -- jobmcp
```

**Codex**

```
codex mcp add job-mcp -- jobmcp
```

**Gemini CLI**

```
gemini mcp add job-mcp jobmcp
```

## Supported platforms

macOS and Linux.
