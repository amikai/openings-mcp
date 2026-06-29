package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/amikai/job-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error { return nil }

func TestServerListsJobTools(t *testing.T) {
	ctx := context.Background()
	server := newServer(
		job104.NewClient(job104.Config{HTTPClient: http.DefaultClient}),
		tsmc.NewClient(tsmc.Config{HTTPClient: http.DefaultClient}),
	)
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "v0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	res, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool, len(res.Tools))
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, name := range []string{
		"104_search_jobs",
		"104_get_job_detail",
		"tsmc_search_jobs",
		"tsmc_get_job_detail",
	} {
		if !got[name] {
			t.Fatalf("missing tool %q in %v", name, got)
		}
	}
}

func TestRunWithTransportTreatsStdinEOFAsCleanExit(t *testing.T) {
	transport := &mcp.IOTransport{
		Reader: io.NopCloser(strings.NewReader("")),
		Writer: writeCloser{Writer: io.Discard},
	}
	if err := runWithTransport(transport); err != nil {
		t.Fatalf("runWithTransport() error = %v, want nil", err)
	}
}
