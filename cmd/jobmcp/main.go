package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/amikai/job-mcp/internal/jobmcp"
	"github.com/amikai/job-mcp/internal/provider/cake"
	"github.com/amikai/job-mcp/internal/provider/google"
	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/amikai/job-mcp/internal/provider/nvidia"
	"github.com/amikai/job-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if err := runWithTransport(&mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func runWithTransport(transport mcp.Transport) error {
	// One connection pool, with a ceiling so a hung upstream fails that call
	// instead of stalling the MCP session.
	hc104 := &http.Client{Timeout: 30 * time.Second, Transport: job104.BrowserTransport{}}

	c104, err := job104.NewClient("https://www.104.com.tw", job104.WithClient(hc104))
	if err != nil {
		return err
	}

	hcCake := &http.Client{Timeout: 30 * time.Second}
	cCake, err := cake.NewClient("https://api.cake.me", cake.WithClient(hcCake))
	if err != nil {
		return err
	}

	hcNvidia := &http.Client{Timeout: 30 * time.Second}
	cNvidia, err := nvidia.NewClient("https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite", nvidia.WithClient(hcNvidia))
	if err != nil {
		return err
	}

	hcTsmc := &http.Client{Timeout: 30 * time.Second}
	cTsmc := tsmc.NewClient("https://careers.tsmc.com", hcTsmc)

	hcGoogle := &http.Client{Timeout: 30 * time.Second}
	cGoogle := google.NewClient("https://www.google.com/about/careers/applications", hcGoogle)

	server := newServer(c104, cCake, cNvidia, cTsmc, cGoogle)

	if err := server.Run(context.Background(), transport); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func newServer(c104 *job104.Client, cCake *cake.Client, cNvidia *nvidia.Client, cTsmc *tsmc.Client, cGoogle *google.Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "job-mcp"}, nil)
	jobmcp.RegisterJob104(server, c104)
	jobmcp.RegisterCake(server, cCake)
	jobmcp.RegisterNvidia(server, cNvidia)
	jobmcp.RegisterTsmc(server, cTsmc)
	jobmcp.RegisterGoogle(server, cGoogle)
	return server
}
