# Core

- Go module `github.com/amikai/job-mcp` contains provider clients and MCP adapters for job search sources.
- Main source map: `internal/provider/*` holds board/company clients and parsers; `internal/jobmcp` adapts providers to MCP tools; `cmd/jobmcp` builds the stdio MCP server; other `cmd/*` packages are provider-specific CLIs/tests.
- Generated provider code exists under `internal/provider/cake/oas_*_gen.go`; avoid hand-editing generated files unless regenerating.
- Related memories: tech stack in `mem:tech_stack`, commands in `mem:suggested_commands`, coding conventions in `mem:conventions`, completion checks in `mem:task_completion`.