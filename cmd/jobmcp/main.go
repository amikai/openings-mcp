package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	job104 "github.com/amikai/job-mcp/internal/104"
	"github.com/amikai/job-mcp/internal/jobmcp"
	"github.com/amikai/job-mcp/internal/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if err := runWithTransport(&mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func runWithTransport(transport mcp.Transport) error {
	// Shared client: one connection pool, with a ceiling so a hung upstream
	// fails that call instead of stalling the MCP session.
	hc := &http.Client{Timeout: 30 * time.Second}

	c104 := job104.NewClient(job104.Config{HTTPClient: hc})
	cTSMC := tsmc.NewClient(tsmc.Config{HTTPClient: hc})
	server := newServer(c104, cTSMC)

	if err := server.Run(context.Background(), transport); err != nil && !isCleanClose(err) {
		return err
	}
	return nil
}

func newServer(c104 *job104.Client, cTSMC *tsmc.Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "job-mcp", Version: "v0.1.0"}, nil)
	jobmcp.RegisterTW104(server, c104)
	jobmcp.RegisterTSMC(server, cTSMC)
	return server
}

func isCleanClose(err error) bool {
	return errors.Is(err, io.EOF) || strings.Contains(err.Error(), "EOF")
}
