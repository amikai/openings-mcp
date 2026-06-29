package jobmcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func assertTools(t *testing.T, server *mcp.Server, names ...string) {
	t.Helper()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
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
	for _, name := range names {
		if !got[name] {
			t.Fatalf("missing tool %q in %v", name, got)
		}
	}
}
