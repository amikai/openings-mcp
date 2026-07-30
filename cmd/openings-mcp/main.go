package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/amikai/openings-mcp/internal/ats"
	amazoncli "github.com/amikai/openings-mcp/internal/cli/provider/amazon"
	applecli "github.com/amikai/openings-mcp/internal/cli/provider/apple"
	ashbycli "github.com/amikai/openings-mcp/internal/cli/provider/ashby"
	avaturecli "github.com/amikai/openings-mcp/internal/cli/provider/avature"
	bamboohrcli "github.com/amikai/openings-mcp/internal/cli/provider/bamboohr"
	cakecli "github.com/amikai/openings-mcp/internal/cli/provider/cake"
	eightfoldcli "github.com/amikai/openings-mcp/internal/cli/provider/eightfold"
	engagecli "github.com/amikai/openings-mcp/internal/cli/provider/engage"
	flowxtracli "github.com/amikai/openings-mcp/internal/cli/provider/flowxtra"
	foxconncli "github.com/amikai/openings-mcp/internal/cli/provider/foxconn"
	googlecli "github.com/amikai/openings-mcp/internal/cli/provider/google"
	greenhousecli "github.com/amikai/openings-mcp/internal/cli/provider/greenhouse"
	herpcli "github.com/amikai/openings-mcp/internal/cli/provider/herp"
	himalayascli "github.com/amikai/openings-mcp/internal/cli/provider/himalayas"
	hrmoscli "github.com/amikai/openings-mcp/internal/cli/provider/hrmos"
	icimscli "github.com/amikai/openings-mcp/internal/cli/provider/icims"
	indeedcli "github.com/amikai/openings-mcp/internal/cli/provider/indeed"
	job104cli "github.com/amikai/openings-mcp/internal/cli/provider/job104"
	jobicycli "github.com/amikai/openings-mcp/internal/cli/provider/jobicy"
	jobindexcli "github.com/amikai/openings-mcp/internal/cli/provider/jobindex"
	joincli "github.com/amikai/openings-mcp/internal/cli/provider/join"
	levercli "github.com/amikai/openings-mcp/internal/cli/provider/lever"
	linkedincli "github.com/amikai/openings-mcp/internal/cli/provider/linkedin"
	metacli "github.com/amikai/openings-mcp/internal/cli/provider/meta"
	mokahrcli "github.com/amikai/openings-mcp/internal/cli/provider/mokahr"
	mtkcli "github.com/amikai/openings-mcp/internal/cli/provider/mtk"
	mynavicli "github.com/amikai/openings-mcp/internal/cli/provider/mynavi"
	nodeskcli "github.com/amikai/openings-mcp/internal/cli/provider/nodesk"
	nvidiacli "github.com/amikai/openings-mcp/internal/cli/provider/nvidia"
	oraclecli "github.com/amikai/openings-mcp/internal/cli/provider/oracle"
	quantacli "github.com/amikai/openings-mcp/internal/cli/provider/quanta"
	realtekcli "github.com/amikai/openings-mcp/internal/cli/provider/realtek"
	recruiteecli "github.com/amikai/openings-mcp/internal/cli/provider/recruitee"
	remotefirstjobscli "github.com/amikai/openings-mcp/internal/cli/provider/remotefirstjobs"
	remoteokcli "github.com/amikai/openings-mcp/internal/cli/provider/remoteok"
	remotivecli "github.com/amikai/openings-mcp/internal/cli/provider/remotive"
	ripplingcli "github.com/amikai/openings-mcp/internal/cli/provider/rippling"
	smartrecruiterscli "github.com/amikai/openings-mcp/internal/cli/provider/smartrecruiters"
	successfactorscli "github.com/amikai/openings-mcp/internal/cli/provider/successfactors"
	synopsyscli "github.com/amikai/openings-mcp/internal/cli/provider/synopsys"
	teamtailorcli "github.com/amikai/openings-mcp/internal/cli/provider/teamtailor"
	tsmccli "github.com/amikai/openings-mcp/internal/cli/provider/tsmc"
	ultiprocli "github.com/amikai/openings-mcp/internal/cli/provider/ultipro"
	weworkremotelycli "github.com/amikai/openings-mcp/internal/cli/provider/weworkremotely"
	workablecli "github.com/amikai/openings-mcp/internal/cli/provider/workable"
	workdaycli "github.com/amikai/openings-mcp/internal/cli/provider/workday"
	workingnomadscli "github.com/amikai/openings-mcp/internal/cli/provider/workingnomads"
	"github.com/amikai/openings-mcp/internal/cli/verifycompanies"
	"github.com/amikai/openings-mcp/internal/logging"
	"github.com/amikai/openings-mcp/internal/openingsmcp"
	"github.com/amikai/openings-mcp/internal/provider/amazon"
	"github.com/amikai/openings-mcp/internal/provider/apple"
	"github.com/amikai/openings-mcp/internal/provider/cake"
	"github.com/amikai/openings-mcp/internal/provider/eightfold"
	"github.com/amikai/openings-mcp/internal/provider/flowxtra"
	"github.com/amikai/openings-mcp/internal/provider/google"
	"github.com/amikai/openings-mcp/internal/provider/indeed"
	"github.com/amikai/openings-mcp/internal/provider/job104"
	"github.com/amikai/openings-mcp/internal/provider/jobindex"
	"github.com/amikai/openings-mcp/internal/provider/linkedin"
	"github.com/amikai/openings-mcp/internal/provider/meta"
	"github.com/amikai/openings-mcp/internal/provider/mokahr"
	"github.com/amikai/openings-mcp/internal/provider/mynavi"
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
const serverInstructions = `openings-mcp exposes job-search tools in two families: (1) per-provider tools for the job boards 104, Cake.me (Taiwan-centric), Jobindex (Denmark), Mynavi Tenshoku (Japan), Flowxtra (board-wide across every company on the Flowxtra careers platform, Europe-leaning), LinkedIn and Indeed (global), plus the careers sites of Amazon, Apple, Google, Meta, NVIDIA, and TSMC; (2) unified company tools — search_jobs_by_company, get_filters_by_company, get_job_detail_by_company — covering thousands of companies behind one company parameter.

Tool selection:
- When the user names a specific company, try search_jobs_by_company first; it covers thousands of companies and its error message suggests close matches when a name isn't recognized. Fall back to the per-provider tools (linkedin, indeed, 104, jobindex, mynavi, ...) when the company isn't covered.
- When the user explicitly names a job board or careers site as the desired source (for example LinkedIn, Indeed, 104, Cake.me, Jobindex, マイナビ転職/Mynavi, Flowxtra, Amazon Jobs, Apple Careers, Google Careers, Meta Careers, NVIDIA Careers, or TSMC Careers), use that source's dedicated tools. A company name by itself is not a source selection.
- When the user has no target in mind, offer them the provider choices; if they don't pick one, start with the job boards (104, Cake.me, LinkedIn, Indeed, Jobindex for Denmark, and Mynavi for Japan) rather than a single company's careers site.
- search_jobs_by_company also accepts recognized public careers-page URLs from the career systems this server supports. Do not pass other careers sites; some career systems accept URLs only for companies already in the curated roster.
- When a company is ambiguous, the unified company tools reject the call and list the matching companies by name with their public careers URLs; retry the same tool with one of the listed careers URLs, not with the original name.

Query construction:
- Use dedicated parameters for structured criteria whenever available. Use keyword only for free-text terms that have no better matching parameter, and evaluate unsupported criteria from the results or job details.
- Every provider follows the same search-then-detail flow: <provider>_search_jobs returns summaries carrying an identifier (job code, ID, or path), and <provider>_get_job_detail exchanges that identifier for the full posting. Identifiers are provider-specific and not interchangeable. The detail step is conditional, not automatic: when a summary from the search step fails the user's criteria, drop it and never call get_job_detail for it.

Context management:
- Search results are paginated; fetch additional pages rather than broadening the query.
- After filtering, fetch details when both hold: the user's criteria include something summaries can't answer (tech stack, remote policy, overtime culture, education requirements written in the posting body, etc.), and the filtered set is small enough to fetch economically (roughly 5-10 postings). If either condition fails, present summaries and let the user decide whether to go deeper.`

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

type serverConfig struct {
	logFile              string
	logLevel             string
	enableCommandLogging bool
	dumpCacheTTL         time.Duration
}

func newRootCmd() *cobra.Command {
	cfg := &serverConfig{}

	rootCmd := &cobra.Command{
		Use:          "openings-mcp",
		Short:        "MCP server exposing job-search tools for job boards and company careers sites",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cfg)
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfg.logFile, "log-file", "", "path to the log file (defaults to empty, outputs to stderr)")
	rootCmd.PersistentFlags().StringVar(&cfg.logLevel, "log-level", "info", "minimum log level: debug, info, warn, or error")
	rootCmd.PersistentFlags().BoolVar(&cfg.enableCommandLogging, "enable-command-logging", false, "log raw JSON-RPC traffic to the log output")
	rootCmd.PersistentFlags().DurationVar(&cfg.dumpCacheTTL, "dump-cache-ttl", ats.DefaultDumpCacheTTL, "TTL for full-board dump cache; <=0 disables the cache")

	rootCmd.Version = fmt.Sprintf("%s\nCommit: %s\nBuild Date: %s", version, commit, date)
	rootCmd.SetVersionTemplate("Version: {{.Version}}\n")

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Launch the openings-mcp stdio MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cfg)
		},
	}

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(verifycompanies.NewCommand())
	rootCmd.AddCommand(job104cli.NewCommand())
	rootCmd.AddCommand(amazoncli.NewCommand())
	rootCmd.AddCommand(applecli.NewCommand())
	rootCmd.AddCommand(ashbycli.NewCommand())
	rootCmd.AddCommand(avaturecli.NewCommand())
	rootCmd.AddCommand(bamboohrcli.NewCommand())
	rootCmd.AddCommand(cakecli.NewCommand())
	rootCmd.AddCommand(eightfoldcli.NewCommand())
	rootCmd.AddCommand(engagecli.NewCommand())
	rootCmd.AddCommand(flowxtracli.NewCommand())
	rootCmd.AddCommand(foxconncli.NewCommand())
	rootCmd.AddCommand(googlecli.NewCommand())
	rootCmd.AddCommand(greenhousecli.NewCommand())
	rootCmd.AddCommand(herpcli.NewCommand())
	rootCmd.AddCommand(himalayascli.NewCommand())
	rootCmd.AddCommand(hrmoscli.NewCommand())
	rootCmd.AddCommand(icimscli.NewCommand())
	rootCmd.AddCommand(indeedcli.NewCommand())
	rootCmd.AddCommand(jobicycli.NewCommand())
	rootCmd.AddCommand(jobindexcli.NewCommand())
	rootCmd.AddCommand(joincli.NewCommand())
	rootCmd.AddCommand(levercli.NewCommand())
	rootCmd.AddCommand(linkedincli.NewCommand())
	rootCmd.AddCommand(metacli.NewCommand())
	rootCmd.AddCommand(mokahrcli.NewCommand())
	rootCmd.AddCommand(mtkcli.NewCommand())
	rootCmd.AddCommand(mynavicli.NewCommand())
	rootCmd.AddCommand(nodeskcli.NewCommand())
	rootCmd.AddCommand(nvidiacli.NewCommand())
	rootCmd.AddCommand(oraclecli.NewCommand())
	rootCmd.AddCommand(quantacli.NewCommand())
	rootCmd.AddCommand(realtekcli.NewCommand())
	rootCmd.AddCommand(recruiteecli.NewCommand())
	rootCmd.AddCommand(remotefirstjobscli.NewCommand())
	rootCmd.AddCommand(remoteokcli.NewCommand())
	rootCmd.AddCommand(remotivecli.NewCommand())
	rootCmd.AddCommand(ripplingcli.NewCommand())
	rootCmd.AddCommand(smartrecruiterscli.NewCommand())
	rootCmd.AddCommand(successfactorscli.NewCommand())
	rootCmd.AddCommand(synopsyscli.NewCommand())
	rootCmd.AddCommand(teamtailorcli.NewCommand())
	rootCmd.AddCommand(tsmccli.NewCommand())
	rootCmd.AddCommand(ultiprocli.NewCommand())
	rootCmd.AddCommand(weworkremotelycli.NewCommand())
	rootCmd.AddCommand(workablecli.NewCommand())
	rootCmd.AddCommand(workdaycli.NewCommand())
	rootCmd.AddCommand(workingnomadscli.NewCommand())

	return rootCmd
}

func runServer(cfg *serverConfig) error {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.logLevel)); err != nil {
		return fmt.Errorf("invalid log-level %q: %w", cfg.logLevel, err)
	}

	logOutput := io.Writer(os.Stderr)
	if cfg.logFile != "" {
		file, err := os.OpenFile(cfg.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer file.Close()
		logOutput = file
	}
	logger := slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{Level: level}))

	// Process-local dump cache for full-dump ATS adapters. Injected into
	// adapters; nil when --dump-cache-ttl <= 0 (opt out by not constructing).
	ttl := cfg.dumpCacheTTL
	var dumpCache *ats.DumpCache
	if ttl > 0 {
		dumpCache = ats.NewDumpCache(ats.DumpCacheConfig{TTL: ttl})
	}
	logger.Info("dump cache configured",
		"enabled", dumpCache != nil,
		"ttl", ttl.String(),
		"max_entries", ats.DefaultDumpCacheMaxEntries,
	)

	var transport mcp.Transport = &mcp.StdioTransport{}
	if cfg.enableCommandLogging {
		transport = &mcp.LoggingTransport{Transport: transport, Writer: logOutput}
	}

	if err := runWithTransport(transport, logger, dumpCache); err != nil {
		logger.Error("server terminated", "error", err)
		return err
	}
	return nil
}

func runWithTransport(transport mcp.Transport, logger *slog.Logger, dumpCache *ats.DumpCache) error {
	// One connection pool, with a ceiling so a hung upstream fails that call
	// instead of stalling the MCP session.
	hc104 := &http.Client{Timeout: 30 * time.Second, Transport: job104.BrowserTransport{}}

	c104, err := job104.NewClient("https://www.104.com.tw", job104.WithClient(hc104))
	if err != nil {
		return err
	}

	hc := &http.Client{Timeout: 30 * time.Second}
	cAmazon, err := amazon.NewClient("https://www.amazon.jobs", amazon.WithClient(hc))
	if err != nil {
		return fmt.Errorf("create Amazon client: %w", err)
	}

	// Eightfold's edge 403s Go's default User-Agent instead of returning
	// JSON, so it gets its own client rather than sharing hc.
	hcEightfold := &http.Client{Timeout: 30 * time.Second, Transport: eightfold.BrowserTransport{}}

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

	cApple, err := apple.NewJobsClient(apple.DefaultBaseURL, hc)
	if err != nil {
		return fmt.Errorf("create Apple client: %w", err)
	}

	jarLinkedin, _ := cookiejar.New(nil)
	cLinkedin := linkedin.NewClient("https://www.linkedin.com", &http.Client{Timeout: 30 * time.Second, Jar: jarLinkedin})

	cIndeed := indeed.NewClient("https://apis.indeed.com/graphql", hc)

	cFlowxtra, err := flowxtra.NewClient("https://app.flowxtra.com/api", flowxtra.WithClient(hc))
	if err != nil {
		return fmt.Errorf("create Flowxtra client: %w", err)
	}

	cJobindex := jobindex.NewClient("https://www.jobindex.dk", hc)

	cMynavi := mynavi.NewClient("https://tenshoku.mynavi.jp", hc)

	cMeta := meta.NewClient("https://www.metacareers.com", hc)

	registry, err := newATSRegistry(hc, hcEightfold, dumpCache)
	if err != nil {
		return err
	}

	server := newServer(&providerClients{
		amazon:   cAmazon,
		job104:   c104,
		apple:    cApple,
		cake:     cCake,
		nvidia:   cNvidia,
		tsmc:     cTsmc,
		google:   cGoogle,
		linkedin: cLinkedin,
		indeed:   cIndeed,
		flowxtra: cFlowxtra,
		jobindex: cJobindex,
		mynavi:   cMynavi,
		meta:     cMeta,
	}, registry, logger)

	if err := server.Run(context.Background(), transport); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// newATSRegistry wires all unified company adapters over one shared
// connection pool, against the providers' production endpoints.
// hcEightfold is separate because Eightfold's edge requires a browser-shaped
// User-Agent that the other adapters don't need.
func newATSRegistry(hc, hcEightfold *http.Client, dumpCache *ats.DumpCache) (*ats.Registry, error) {
	adapters, err := atsAdapters(hc, hcEightfold, dumpCache)
	if err != nil {
		return nil, err
	}
	return ats.NewRegistry(adapters...)
}

// atsAdapters builds the production adapter list newATSRegistry registers,
// in registration order. Pulled out of newATSRegistry so tests can walk the
// same list the server actually runs, rather than a redeclared copy that
// could drift from it. dumpCache is injected into full-dump adapters (nil is fine).
func atsAdapters(hc, hcEightfold *http.Client, dumpCache *ats.DumpCache) ([]ats.Adapter, error) {
	leverAdapter, err := ats.NewLeverAdapter("https://api.lever.co", hc, dumpCache)
	if err != nil {
		return nil, fmt.Errorf("create Lever ATS adapter: %w", err)
	}
	ashbyAdapter, err := ats.NewAshbyAdapter("https://api.ashbyhq.com", hc, dumpCache)
	if err != nil {
		return nil, fmt.Errorf("create Ashby ATS adapter: %w", err)
	}
	greenhouseAdapter, err := ats.NewGreenhouseAdapter("https://boards-api.greenhouse.io/v1", hc, dumpCache)
	if err != nil {
		return nil, fmt.Errorf("create Greenhouse ATS adapter: %w", err)
	}
	smartrecruitersAdapter, err := ats.NewSmartRecruitersAdapter("https://api.smartrecruiters.com", hc)
	if err != nil {
		return nil, fmt.Errorf("create SmartRecruiters ATS adapter: %w", err)
	}
	workableAdapter, err := ats.NewWorkableAdapter("https://apply.workable.com", hc)
	if err != nil {
		return nil, fmt.Errorf("create Workable ATS adapter: %w", err)
	}
	ripplingAdapter, err := ats.NewRipplingAdapter("https://api.rippling.com/platform/api/ats/v1", hc, dumpCache)
	if err != nil {
		return nil, fmt.Errorf("create Rippling ATS adapter: %w", err)
	}
	mokahrAdapter, err := ats.NewMokaHRAdapter(mokahr.DefaultBaseURL, hc)
	if err != nil {
		return nil, fmt.Errorf("create MokaHR ATS adapter: %w", err)
	}

	return []ats.Adapter{
		ats.NewWorkdayAdapter(hc),
		leverAdapter,
		ashbyAdapter,
		greenhouseAdapter,
		ats.NewTeamtailorAdapter(hc, dumpCache),
		ats.NewRecruiteeAdapter(hc, dumpCache),
		ats.NewHerpAdapter(hc, dumpCache),
		ats.NewEngageAdapter(hc, dumpCache),
		ats.NewBambooHRAdapter(hc, dumpCache),
		ats.NewEightfoldAdapter(hcEightfold),
		ats.NewSuccessFactorsAdapter(hc),
		smartrecruitersAdapter,
		workableAdapter,
		ripplingAdapter,
		ats.NewICIMSAdapter(hc),
		ats.NewAvatureAdapter(hc),
		ats.NewOracleAdapter(hc),
		ats.NewJoinAdapter("https://join.com", hc, dumpCache),
		ats.NewUltiProAdapter(hc),
		ats.NewHrmosAdapter("https://hrmos.co", hc, dumpCache),
		mokahrAdapter,
	}, nil
}

// providerClients bundles one client per per-provider tool family, so
// newServer's signature doesn't grow with every provider added.
type providerClients struct {
	amazon   *amazon.Client
	job104   *job104.Client
	apple    *apple.JobsClient
	cake     *cake.Client
	nvidia   *nvidia.Client
	tsmc     *tsmc.Client
	google   *google.Client
	linkedin *linkedin.Client
	indeed   *indeed.Client
	flowxtra *flowxtra.Client
	jobindex *jobindex.Client
	mynavi   *mynavi.Client
	meta     *meta.Client
}

func newServer(clients *providerClients, registry *ats.Registry, logger *slog.Logger) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "openings-mcp", Version: version},
		&mcp.ServerOptions{Instructions: serverInstructions, Logger: logger},
	)
	server.AddReceivingMiddleware(logging.LoggingMiddleware(logger))
	// Registered last so it wraps outermost, catching panics from tool
	// handlers and from other middleware alike.
	server.AddReceivingMiddleware(logging.RecoveryMiddleware(logger))
	openingsmcp.RegisterAmazon(server, clients.amazon)
	openingsmcp.RegisterJob104(server, clients.job104)
	openingsmcp.RegisterApple(server, clients.apple)
	openingsmcp.RegisterCake(server, clients.cake)
	openingsmcp.RegisterNvidia(server, clients.nvidia)
	openingsmcp.RegisterTsmc(server, clients.tsmc)
	openingsmcp.RegisterGoogle(server, clients.google)
	openingsmcp.RegisterLinkedin(server, clients.linkedin)
	openingsmcp.RegisterIndeed(server, clients.indeed)
	openingsmcp.RegisterFlowxtra(server, clients.flowxtra)
	openingsmcp.RegisterJobindex(server, clients.jobindex)
	openingsmcp.RegisterMynavi(server, clients.mynavi)
	openingsmcp.RegisterMeta(server, clients.meta)
	openingsmcp.RegisterCompany(server, registry)
	return server
}
