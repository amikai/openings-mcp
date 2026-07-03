# Suggested Commands

- Run root non-cmd Go tests: `make ut`.
- Run all root Go packages including cmd tests: `go test ./...`.
- Run MCP adapter tests only: `go test ./internal/jobmcp ./cmd/jobmcp`.
- Build stdio MCP server: `go build ./cmd/jobmcp`.
- Validate OpenAPI specs with Docker: `make validate-openapi`.
- Preferred search command: `rg` / `rg --files`.