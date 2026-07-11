package logging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"regexp"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	middleware := LoggingMiddleware(logger)
	dummyHandler := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		res := &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: "api lookup failed"}},
		}
		return res, errors.New("handler level failure")
	}

	wrapped := middleware(dummyHandler)
	_, err := wrapped(t.Context(), "test_method", nil)
	require.Error(t, err)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "tool call error")
	assert.Contains(t, logOutput, "error=\"api lookup failed\"")
	assert.Contains(t, logOutput, "MCP protocol handler error")
	assert.Contains(t, logOutput, "error=\"handler level failure\"")
	assert.Contains(t, logOutput, "req_id=")
}

func TestLoggingMiddlewareSuccessLogsNothing(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	middleware := LoggingMiddleware(logger)
	dummyHandler := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	}

	wrapped := middleware(dummyHandler)
	_, err := wrapped(t.Context(), "test_method", nil)
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestLoggingMiddlewareDebugLogsRequests(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	middleware := LoggingMiddleware(logger)
	dummyHandler := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	}

	wrapped := middleware(dummyHandler)
	_, err := wrapped(t.Context(), "test_method", nil)
	require.NoError(t, err)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "request received")
	assert.Contains(t, logOutput, "request completed")
	assert.Contains(t, logOutput, "method=test_method")
	assert.Contains(t, logOutput, "duration=")

	// Both entries of one request carry the same req_id.
	ids := regexp.MustCompile(`req_id=(\S+)`).FindAllStringSubmatch(logOutput, -1)
	require.Len(t, ids, 2)
	assert.Equal(t, ids[0][1], ids[1][1])

	// A second request gets a different req_id.
	buf.Reset()
	_, err = wrapped(t.Context(), "test_method", nil)
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "req_id="+ids[0][1])
}
