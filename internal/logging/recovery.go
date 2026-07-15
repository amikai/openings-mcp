package logging

import (
	"context"
	"log/slog"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RecoveryMiddleware returns an MCP middleware that recovers from panics
// raised anywhere in the wrapped handler chain, logs them with a stack
// trace, and turns them into a safe response instead of crashing the
// server — the MCP equivalent of gin's or chi's Recovery middleware.
//
// Register it as the outermost middleware (the last AddReceivingMiddleware
// call) so it also guards handlers registered by other middleware.
func RecoveryMiddleware(logger *slog.Logger) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (result mcp.Result, err error) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}

				stack := debug.Stack()
				logger.Error("panic recovered", "method", method, "panic", rec, "stack", string(stack))

				if p, ok := toolCallParams(req); ok {
					result = &mcp.CallToolResult{
						IsError: true,
						Content: []mcp.Content{&mcp.TextContent{Text: "tool " + p.Name + " panicked"}},
					}
					err = nil
					return
				}

				result = nil
				err = &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
			}()

			return next(ctx, method, req)
		}
	}
}

func toolCallParams(req mcp.Request) (*mcp.CallToolParamsRaw, bool) {
	if req == nil {
		return nil, false
	}
	p, ok := req.GetParams().(*mcp.CallToolParamsRaw)
	return p, ok
}
