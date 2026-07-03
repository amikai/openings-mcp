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

// serverInstructions carries the cross-tool guidance for host LLMs: provider
// routing and the shared search→detail flow. Per-tool behavior stays in each
// tool's description.
const serverInstructions = `job-mcp exposes job-search tools for five job boards: 104 and Cake.me (both Taiwan-centric), plus the official careers sites of Google, NVIDIA, and TSMC.

Tool selection:
- When the user names a site or company, use that provider's tools. Otherwise search 104 and Cake.me for jobs in Taiwan, and the company careers tools for roles at Google, NVIDIA, or TSMC.
- Every provider follows the same two-step flow: <provider>_search_jobs returns summaries carrying an identifier (job code, ID, or path), and <provider>_get_job_detail exchanges that identifier for the full posting. Identifiers are provider-specific and not interchangeable.

Context management:
- Search results are paginated; fetch additional pages rather than broadening the query.
- Fetch job details only for postings you intend to present.`

func newServer(c104 *job104.Client, cCake *cake.Client, cNvidia *nvidia.Client, cTsmc *tsmc.Client, cGoogle *google.Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "job-mcp"}, &mcp.ServerOptions{Instructions: serverInstructions})
	jobmcp.RegisterJob104(server, c104)
	jobmcp.RegisterCake(server, cCake)
	jobmcp.RegisterNvidia(server, cNvidia)
	jobmcp.RegisterTsmc(server, cTsmc)
	jobmcp.RegisterGoogle(server, cGoogle)
	return server
}
