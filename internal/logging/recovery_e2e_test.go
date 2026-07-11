package logging_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/amikai/openings-mcp/internal/logging"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This mirrors newServer's middleware wiring in cmd/openings-mcp/main.go
// exactly, wires a real client<->server pair over an in-memory transport,
// and calls a tool handler that genuinely panics -- no mocked
// mcp.MethodHandler, no fake req. If RecoveryMiddleware didn't actually
// work, this test would hang (the SDK's own request goroutine dies mid
// request) or crash the whole `go test` process.
func TestRecoveryMiddlewareRealPanicOverTheWire(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0"}, &mcp.ServerOptions{Logger: logger})
	server.AddReceivingMiddleware(logging.ErrorLoggingMiddleware(logger))
	server.AddReceivingMiddleware(logging.RecoveryMiddleware(logger))

	mcp.AddTool(server, &mcp.Tool{Name: "boom", Description: "always panics"},
		func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
			panic("simulated bug: nil map write")
		})
	mcp.AddTool(server, &mcp.Tool{Name: "ping", Description: "ok"},
		func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "pong"}}}, nil, nil
		})

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ctx := t.Context()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer clientSession.Close()

	// The actual call that triggers the real panic inside the SDK's own
	// goroutine (internal/jsonrpc2/conn.go handleAsync).
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "boom"})
	require.NoError(t, err, "the JSON-RPC round trip itself must succeed -- no crash, no hang")
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	tc, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc.Text, "boom")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "panic recovered")
	assert.Contains(t, logOutput, "simulated bug: nil map write")
	assert.Contains(t, logOutput, "goroutine") // proof debug.Stack() actually ran

	// Prove the session/server is still alive after the panic: a normal
	// call afterward must still work.
	result2, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "ping"})
	require.NoError(t, err)
	assert.False(t, result2.IsError)
}
