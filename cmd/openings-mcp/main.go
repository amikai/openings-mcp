package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/ats"
	"github.com/amikai/openings-mcp/internal/logging"
	"github.com/amikai/openings-mcp/internal/openingsmcp"
	"github.com/amikai/openings-mcp/internal/provider/cake"
	"github.com/amikai/openings-mcp/internal/provider/google"
	"github.com/amikai/openings-mcp/internal/provider/job104"
	"github.com/amikai/openings-mcp/internal/provider/linkedin"
	"github.com/amikai/openings-mcp/internal/provider/nvidia"
	"github.com/amikai/openings-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	version = "version"
	commit  = "commit"
	date    = "date"
)

// serverInstructions carries the cross-tool guidance for host LLMs: provider
// routing and the shared search→detail flow. Per-tool behavior stays in each
// tool's description.
const serverInstructions = `openings-mcp exposes job-search tools in two families: (1) per-provider tools for the job boards 104 and Cake.me (Taiwan-centric) and LinkedIn (global), plus the careers sites of Google, NVIDIA, and TSMC; (2) unified company tools — search_jobs_by_company, get_filters_by_company, get_job_detail_by_company — covering hundreds of companies behind one company parameter.

Tool selection:
- When the user names a specific company, try search_jobs_by_company first; it covers hundreds of companies and its error message suggests close matches when a name isn't recognized. Fall back to the per-provider tools (linkedin, 104, ...) when the company isn't covered.
- When the user names a site or company, use that provider's tools.
- When the user has no target in mind, offer them the provider choices; if they don't pick one, start with the job boards (104, Cake.me, and LinkedIn) rather than a single company's careers site.
- search_jobs_by_company also accepts a company's public careers-page URL. When a company isn't in the supported list, find its careers page URL (e.g. via web search) and pass that URL as the company argument.

Query construction:
- Listen carefully to the user's stated criteria and map each one onto a search parameter when a matching parameter exists; enforce criteria the parameters cannot express by filtering the results yourself.
- Keep the keyword parameter to role titles, skills, or technologies. Location, job type, seniority, and other constraints go in their dedicated parameters, never embedded in the keyword string.
- Every provider follows the same search-then-detail flow: <provider>_search_jobs returns summaries carrying an identifier (job code, ID, or path), and <provider>_get_job_detail exchanges that identifier for the full posting. Identifiers are provider-specific and not interchangeable. The detail step is conditional, not automatic: when a summary from the search step fails the user's criteria, drop it and never call get_job_detail for it.

Context management:
- Search results are paginated; fetch additional pages rather than broadening the query.
- After filtering, fetch details when both hold: the user's criteria include something summaries can't answer (tech stack, remote policy, overtime culture, education requirements written in the posting body, etc.), and the filtered set is small enough to fetch economically (roughly 5-10 postings). If either condition fails, present summaries and let the user decide whether to go deeper.`

func main() {
	os.Exit(run())
}

// run carries main's body so the deferred log-file cleanup survives every
// exit path; only main itself calls os.Exit.
func run() int {
	fs := ff.NewFlagSet("openings-mcp")
	var (
		logFile              = fs.StringLong("log-file", "", "path to the log file (defaults to empty, outputs to stderr)")
		logLevel             = fs.StringLong("log-level", "info", "minimum log level: debug, info, warn, or error")
		enableCommandLogging = fs.BoolLong("enable-command-logging", "log raw JSON-RPC traffic to the log output")
		versionFlag          = fs.BoolLong("version", "print version information and exit")
	)
	cmd := &ff.Command{
		Name:      "openings-mcp",
		ShortHelp: "MCP server exposing job-search tools for job boards and company careers sites",
		Flags:     fs,
	}
	if err := cmd.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(cmd))
		if errors.Is(err, ff.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "err:", err)
		return 1
	}

	if *versionFlag {
		fmt.Printf("Version: %s\nCommit: %s\nBuild Date: %s\n", version, commit, date)
		return 0
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		log.Fatalf("invalid log-level %q: %v", *logLevel, err)
	}

	logOutput := io.Writer(os.Stderr)
	if *logFile != "" {
		file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
			return 1
		}
		defer file.Close()
		logOutput = file
	}
	logger := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{Level: level}))

	var transport mcp.Transport = &mcp.StdioTransport{}
	if *enableCommandLogging {
		transport = &mcp.LoggingTransport{Transport: transport, Writer: logOutput}
	}

	if err := runWithTransport(transport, logger); err != nil {
		logger.Error("server terminated", "error", err)
		return 1
	}
	return 0
}

func runWithTransport(transport mcp.Transport, logger *slog.Logger) error {
	// One connection pool, with a ceiling so a hung upstream fails that call
	// instead of stalling the MCP session.
	hc104 := &http.Client{Timeout: 30 * time.Second, Transport: job104.BrowserTransport{}}

	c104, err := job104.NewClient("https://www.104.com.tw", job104.WithClient(hc104))
	if err != nil {
		return err
	}

	hc := &http.Client{Timeout: 30 * time.Second}

	cCake, err := cake.NewClient("https://api.cake.me", cake.WithClient(hc))
	if err != nil {
		return err
	}

	cNvidia, err := nvidia.NewClient("https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite", nvidia.WithClient(hc))
	if err != nil {
		return err
	}

	cTsmc := tsmc.NewClient("https://careers.tsmc.com", hc)

	cGoogle := google.NewClient("https://www.google.com/about/careers/applications", hc)

	jarLinkedin, _ := cookiejar.New(nil)
	cLinkedin := linkedin.NewClient("https://www.linkedin.com", &http.Client{Timeout: 30 * time.Second, Jar: jarLinkedin})

	registry, err := newATSRegistry(hc)
	if err != nil {
		return err
	}

	server := newServer(
		c104,
		cCake,
		cNvidia,
		cTsmc,
		cGoogle,
		cLinkedin,
		registry,
		logger,
	)

	if err := server.Run(context.Background(), transport); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// newATSRegistry wires all unified company adapters over one shared
// connection pool, against the providers' production endpoints.
func newATSRegistry(hc *http.Client) (*ats.Registry, error) {
	leverAdapter, err := ats.NewLeverAdapter("https://api.lever.co", hc)
	if err != nil {
		return nil, fmt.Errorf("create Lever ATS adapter: %w", err)
	}
	ashbyAdapter, err := ats.NewAshbyAdapter("https://api.ashbyhq.com", hc)
	if err != nil {
		return nil, fmt.Errorf("create Ashby ATS adapter: %w", err)
	}
	greenhouseAdapter, err := ats.NewGreenhouseAdapter("https://boards-api.greenhouse.io/v1", hc)
	if err != nil {
		return nil, fmt.Errorf("create Greenhouse ATS adapter: %w", err)
	}

	return ats.NewRegistry(
		ats.NewWorkdayAdapter(hc),
		leverAdapter,
		ashbyAdapter,
		greenhouseAdapter,
		ats.NewTeamtailorAdapter(hc),
	)
}

func newServer(
	c104 *job104.Client,
	cCake *cake.Client,
	cNvidia *nvidia.Client,
	cTsmc *tsmc.Client,
	cGoogle *google.Client,
	cLinkedin *linkedin.Client,
	registry *ats.Registry,
	logger *slog.Logger,
) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "openings-mcp", Version: version}, &mcp.ServerOptions{Instructions: serverInstructions, Logger: logger})
	server.AddReceivingMiddleware(logging.LoggingMiddleware(logger))
	// Registered last so it wraps outermost, catching panics from tool
	// handlers and from other middleware alike.
	server.AddReceivingMiddleware(logging.RecoveryMiddleware(logger))
	openingsmcp.RegisterJob104(server, c104)
	openingsmcp.RegisterCake(server, cCake)
	openingsmcp.RegisterNvidia(server, cNvidia)
	openingsmcp.RegisterTsmc(server, cTsmc)
	openingsmcp.RegisterGoogle(server, cGoogle)
	openingsmcp.RegisterLinkedin(server, cLinkedin)
	openingsmcp.RegisterCompany(server, registry)
	return server
}
