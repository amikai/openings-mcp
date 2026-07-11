// Package logging provides MCP server middleware for request logging and
// panic recovery.
package logging

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LoggingMiddleware returns an MCP middleware that logs request handling at
// levels matching severity: each request's start and completion (with
// duration) at debug, tool results flagged as errors and protocol-level
// handler errors at error. The logger's level decides which entries appear.
// Every entry from one request carries the same req_id, so interleaved
// entries from concurrent requests can be told apart.
func LoggingMiddleware(logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			reqLogger := logger.With("req_id", uuid.NewString())

			reqLogger.Debug("request received", "method", method)
			start := time.Now()

			res, err := next(ctx, method, request)

			reqLogger.Debug("request completed", "method", method, "duration", time.Since(start))

			if r, ok := res.(*mcp.CallToolResult); ok && r.IsError {
				for _, c := range r.Content {
					if tc, ok := c.(*mcp.TextContent); ok {
						reqLogger.Error("tool call error", "method", method, "error", tc.Text)
					}
				}
			}

			if err != nil {
				reqLogger.Error("MCP protocol handler error", "method", method, "error", err)
			}

			return res, err
		}
	}
}
