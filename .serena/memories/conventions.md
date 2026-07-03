# Conventions

- Keep provider packages focused on upstream HTTP clients, parsers, request/response domain types, constants, and provider-local formatting.
- Keep MCP glue in `internal/jobmcp`; it owns MCP input structs, label-to-code mapping for tool-facing enums, tool definitions, handlers, and MCP result helpers.
- MCP tool handlers should return `errorResult(err), nil, nil` for validation/upstream failures so the model sees an MCP tool error result instead of a protocol-level Go error.
- Use typed `mcp.ToolHandlerFor[In, Out]` handlers through generic `mcp.AddTool` so input schemas and JSON argument decoding are derived from Go structs.
- `cmd/jobmcp` should stay composition-oriented: build clients/server, register selected tools, run transport, and treat stdin EOF as clean close.