// Package jobmcp adapts the internal job-board clients into MCP tools.
package jobmcp

import "github.com/modelcontextprotocol/go-sdk/mcp"

// errorResult reports a failure to the model without aborting the tool call.
func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}
