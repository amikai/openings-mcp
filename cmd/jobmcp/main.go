package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/amikai/job-mcp/internal/jobmcp"
	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/amikai/job-mcp/internal/provider/tsmc"
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
	hc104 := &http.Client{Timeout: 30 * time.Second, Transport: job104.BrowserTransport{}}

	c104, err := job104.NewClient("https://www.104.com.tw", job104.WithClient(hc104))
	if err != nil {
		return err
	}
	cTSMC := tsmc.NewClient(hc)
	server := newServer(c104, cTSMC)

	if err := server.Run(context.Background(), transport); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func newServer(c104 *job104.Client, cTSMC *tsmc.Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "job-mcp"}, nil)
	jobmcp.RegisterJob104(server, c104)
	jobmcp.RegisterTSMC(server, cTSMC)
	return server
}
