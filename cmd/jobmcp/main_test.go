package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/cake"
	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error { return nil }

func TestServerListsJobTools(t *testing.T) {
	ctx := context.Background()
	c104, err := job104.NewClient("https://www.104.com.tw", job104.WithClient(http.DefaultClient))
	if err != nil {
		t.Fatal(err)
	}
	cCake, err := cake.NewClient("https://api.cake.me", cake.WithClient(http.DefaultClient))
	if err != nil {
		t.Fatal(err)
	}
	server := newServer(c104, cCake)
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
		"cake_search_jobs",
		"cake_get_job_detail",
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
