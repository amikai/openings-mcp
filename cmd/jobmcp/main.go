package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/job-mcp/internal/jobmcp"
	"github.com/amikai/job-mcp/internal/logging"
	"github.com/amikai/job-mcp/internal/provider/cake"
	"github.com/amikai/job-mcp/internal/provider/google"
	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/amikai/job-mcp/internal/provider/nvidia"
	"github.com/amikai/job-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	fs := ff.NewFlagSet("jobmcp")
	var (
		logFile              = fs.StringLong("log-file", "", "path to the log file (defaults to empty, outputs to stderr)")
		enableCommandLogging = fs.BoolLong("enable-command-logging", "log raw JSON-RPC traffic to the log output")
	)
	if err := ff.Parse(fs, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Flags(fs))
		if errors.Is(err, ff.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}

	logOutput := io.Writer(os.Stderr)
	if *logFile != "" {
		file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			log.Fatalf("failed to open log file: %v", err)
		}
		defer file.Close()
		logOutput = file
	}
	logger := slog.New(slog.NewTextHandler(logOutput, nil))

	var transport mcp.Transport = &mcp.StdioTransport{}
	if *enableCommandLogging {
		transport = &mcp.LoggingTransport{Transport: transport, Writer: logOutput}
	}

	if err := runWithTransport(transport, logger); err != nil {
		log.Fatal(err)
	}
}

func runWithTransport(transport mcp.Transport, logger *slog.Logger) error {
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

	server := newServer(c104, cCake, cNvidia, cTsmc, cGoogle, logger)

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
- When the user names a site or company, use that provider's tools.
- When the user has no target in mind, offer them the provider choices; if they don't pick one, start with the job boards (104 and Cake.me) rather than a single company's careers site.

Query construction:
- Listen carefully to the user's stated criteria and map each one onto a search parameter when a matching parameter exists; enforce criteria the parameters cannot express by filtering the results yourself.
- Keep the keyword parameter to role titles, skills, or technologies. Location, job type, seniority, and other constraints go in their dedicated parameters, never embedded in the keyword string.
- Every provider follows the same search-then-detail flow: <provider>_search_jobs returns summaries carrying an identifier (job code, ID, or path), and <provider>_get_job_detail exchanges that identifier for the full posting. Identifiers are provider-specific and not interchangeable. The detail step is conditional, not automatic: when a summary from the search step fails the user's criteria, drop it and never call get_job_detail for it.

Context management:
- Search results are paginated; fetch additional pages rather than broadening the query.
- Fetch job details in small batches of the most promising postings (around 5-10 at a time), only for postings you intend to present.`

func newServer(c104 *job104.Client, cCake *cake.Client, cNvidia *nvidia.Client, cTsmc *tsmc.Client, cGoogle *google.Client, logger *slog.Logger) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "job-mcp"}, &mcp.ServerOptions{Instructions: serverInstructions, Logger: logger})
	server.AddReceivingMiddleware(logging.ErrorLoggingMiddleware(logger))
	jobmcp.RegisterJob104(server, c104)
	jobmcp.RegisterCake(server, cCake)
	jobmcp.RegisterNvidia(server, cNvidia)
	jobmcp.RegisterTsmc(server, cTsmc)
	jobmcp.RegisterGoogle(server, cGoogle)
	return server
}
