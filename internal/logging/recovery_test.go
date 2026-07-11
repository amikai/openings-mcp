package logging

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoveryMiddlewarePassesThroughWithoutPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	middleware := RecoveryMiddleware(logger)
	dummyHandler := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	}

	wrapped := middleware(dummyHandler)
	res, err := wrapped(t.Context(), "test_method", nil)
	require.NoError(t, err)
	assert.Equal(t, &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, res)
	assert.Empty(t, buf.String())
}

func TestRecoveryMiddlewareToolCallPanicReturnsErrorResult(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	middleware := RecoveryMiddleware(logger)
	panicHandler := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		panic("boom")
	}

	req := &mcp.ServerRequest[*mcp.CallToolParamsRaw]{Params: &mcp.CallToolParamsRaw{Name: "search_jobs"}}

	wrapped := middleware(panicHandler)
	res, err := wrapped(t.Context(), "tools/call", req)
	require.NoError(t, err)

	result, ok := res.(*mcp.CallToolResult)
	require.True(t, ok)
	assert.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	tc, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc.Text, "search_jobs")

	logOutput := buf.String()
	assert.Contains(t, logOutput, "panic recovered")
	assert.Contains(t, logOutput, "panic=boom")
	assert.Contains(t, logOutput, "method=tools/call")
}

func TestRecoveryMiddlewareNonToolCallPanicReturnsProtocolError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	middleware := RecoveryMiddleware(logger)
	panicHandler := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		panic("boom")
	}

	wrapped := middleware(panicHandler)
	res, err := wrapped(t.Context(), "resources/read", nil)

	require.Nil(t, res)
	require.Error(t, err)
	wireErr, ok := err.(*jsonrpc.Error)
	require.True(t, ok)
	assert.Equal(t, int64(jsonrpc.CodeInternalError), wireErr.Code)

	assert.Contains(t, buf.String(), "panic recovered")
}
