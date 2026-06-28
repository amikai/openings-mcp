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
