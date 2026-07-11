// Package logging provides MCP server middleware for error auditing and
// panic recovery.
package logging

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ErrorLoggingMiddleware returns an MCP middleware that logs tool results
// flagged as errors and any protocol-level handler errors.
func ErrorLoggingMiddleware(logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			res, err := next(ctx, method, request)

			if r, ok := res.(*mcp.CallToolResult); ok && r.IsError {
				for _, c := range r.Content {
					if tc, ok := c.(*mcp.TextContent); ok {
						logger.Error("tool call error", "method", method, "error", tc.Text)
					}
				}
			}

			if err != nil {
				logger.Error("MCP protocol handler error", "method", method, "error", err)
			}

			return res, err
		}
	}
}
