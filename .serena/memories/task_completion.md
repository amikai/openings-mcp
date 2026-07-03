# Task Completion

- For MCP adapter/server changes, run `go test ./internal/jobmcp ./cmd/jobmcp` at minimum.
- For broader Go changes, run `go test ./...`; `make ut` intentionally excludes `/cmd` packages.
- Run `go build ./cmd/jobmcp` after changing server construction or MCP registration.
- Use `gofmt` on edited Go files before final verification.
- User can run `serena memories check` from the project root to validate memory references.