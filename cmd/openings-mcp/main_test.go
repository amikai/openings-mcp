package main

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/cake"
	"github.com/amikai/openings-mcp/internal/provider/google"
	"github.com/amikai/openings-mcp/internal/provider/job104"
	"github.com/amikai/openings-mcp/internal/provider/linkedin"
	"github.com/amikai/openings-mcp/internal/provider/nvidia"
	"github.com/amikai/openings-mcp/internal/provider/tsmc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error { return nil }

func TestServerListsJobTools(t *testing.T) {
	ctx := t.Context()
	c104, err := job104.NewClient("https://www.104.com.tw", job104.WithClient(http.DefaultClient))
	require.NoError(t, err)
	cCake, err := cake.NewClient("https://api.cake.me", cake.WithClient(http.DefaultClient))
	require.NoError(t, err)
	cNvidia, err := nvidia.NewClient("https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite", nvidia.WithClient(http.DefaultClient))
	require.NoError(t, err)
	cTsmc := tsmc.NewClient("https://careers.tsmc.com", http.DefaultClient)
	cGoogle := google.NewClient("https://www.google.com/about/careers/applications", http.DefaultClient)
	cLinkedin := linkedin.NewClient("https://www.linkedin.com", http.DefaultClient)
	registry, err := newATSRegistry(http.DefaultClient)
	require.NoError(t, err)
	server := newServer(providerClients{
		job104:   c104,
		cake:     cCake,
		nvidia:   cNvidia,
		tsmc:     cTsmc,
		google:   cGoogle,
		linkedin: cLinkedin,
	}, registry, slog.New(slog.NewTextHandler(io.Discard, nil)))
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "v0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer clientSession.Close()

	res, err := clientSession.ListTools(ctx, nil)
	require.NoError(t, err)
	got := make(map[string]bool, len(res.Tools))
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, name := range []string{
		"104_search_jobs",
		"104_get_job_detail",
		"cake_search_jobs",
		"cake_get_job_detail",
		"nvidia_search_jobs",
		"nvidia_get_job_detail",
		"tsmc_search_jobs",
		"tsmc_get_job_detail",
		"google_search_jobs",
		"google_get_job_detail",
		"linkedin_search_jobs",
		"linkedin_get_job_detail",
		"search_jobs_by_company",
		"get_filters_by_company",
		"get_job_detail_by_company",
	} {
		assert.Contains(t, got, name)
	}
}

func TestRunWithTransportTreatsStdinEOFAsCleanExit(t *testing.T) {
	transport := &mcp.IOTransport{
		Reader: io.NopCloser(strings.NewReader("")),
		Writer: writeCloser{Writer: io.Discard},
	}
	err := runWithTransport(transport, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
}

func TestATSRegistryIncludesTeamtailor(t *testing.T) {
	registry, err := newATSRegistry(http.DefaultClient)
	require.NoError(t, err)

	adapter, slug, err := registry.Resolve("Teamtailor")
	require.NoError(t, err)
	assert.Equal(t, "teamtailor", adapter.Name())
	assert.Equal(t, "career.teamtailor.com", slug)

	adapter, slug, err = registry.Resolve("https://unlisted.na.teamtailor.com/jobs")
	require.NoError(t, err)
	assert.Equal(t, "teamtailor", adapter.Name())
	assert.Equal(t, "unlisted.na.teamtailor.com", slug)
}
