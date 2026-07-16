package main

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/ats"
)

func TestParseProvidersIncludesTeamtailor(t *testing.T) {
	got, err := parseProviders("workday,teamtailor")
	require.NoError(t, err)
	assert.Equal(t, []string{"teamtailor", "workday"}, got)
}

func TestBuildTeamtailorAdapter(t *testing.T) {
	adapters, err := buildAdapters([]string{"teamtailor"})
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, "teamtailor", adapters[0].Name())
}

func TestParseProvidersIncludesOracle(t *testing.T) {
	got, err := parseProviders("workday,oracle")
	require.NoError(t, err)
	assert.Equal(t, []string{"oracle", "workday"}, got)
}

func TestBuildOracleAdapter(t *testing.T) {
	adapters, err := buildAdapters([]string{"oracle"})
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, "oracle", adapters[0].Name())
}

// fakeAdapter serves canned Search/Detail outcomes and records the Detail
// probe's arguments.
type fakeAdapter struct {
	searchRes   *ats.SearchResult
	searchErr   error
	detailErr   error
	detailCalls int
	detailSlug  string
	detailJobID string
}

func (f *fakeAdapter) Name() string                            { return "fake" }
func (f *fakeAdapter) Roster() []ats.CompanyInfo               { return nil }
func (f *fakeAdapter) ParseCareersURL(*url.URL) (string, bool) { return "", false }

func (f *fakeAdapter) Search(context.Context, string, ats.SearchParams) (*ats.SearchResult, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.searchRes, nil
}

func (f *fakeAdapter) Filters(context.Context, string) (ats.FilterSet, error) { return nil, nil }

func (f *fakeAdapter) Detail(_ context.Context, slug, jobID string) (*ats.JobDetail, error) {
	f.detailCalls++
	f.detailSlug, f.detailJobID = slug, jobID
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	return &ats.JobDetail{JobID: jobID}, nil
}

func searchResult(total int, jobIDs ...string) *ats.SearchResult {
	res := &ats.SearchResult{TotalCount: total}
	for _, id := range jobIDs {
		res.Jobs = append(res.Jobs, ats.JobSummary{JobID: id})
	}
	return res
}

func TestCheckProbesDetailOnFirstJob(t *testing.T) {
	fake := &fakeAdapter{searchRes: searchResult(42, "job-1", "job-2")}
	c := check{adapter: fake, company: "Acme", slug: "acme"}

	r := c.do(t.Context(), time.Minute)

	assert.Equal(t, statusOK, r.Status)
	assert.Equal(t, 42, r.Jobs)
	assert.Empty(t, r.Detail)
	assert.Equal(t, 1, fake.detailCalls)
	assert.Equal(t, "acme", fake.detailSlug)
	assert.Equal(t, "job-1", fake.detailJobID)
}

func TestCheckReportsDetailFailure(t *testing.T) {
	fake := &fakeAdapter{
		searchRes: searchResult(42, "job-1"),
		detailErr: errors.New("unrecognized detail page"),
	}
	c := check{adapter: fake, company: "Acme", slug: "acme"}

	r := c.do(t.Context(), time.Minute)

	assert.Equal(t, statusDetailError, r.Status)
	assert.Equal(t, 42, r.Jobs)
	assert.Contains(t, r.Detail, "job-1")
	assert.Contains(t, r.Detail, "unrecognized detail page")
}

func TestCheckSkipsDetailWhenZeroJobs(t *testing.T) {
	fake := &fakeAdapter{searchRes: searchResult(0)}
	c := check{adapter: fake, company: "Acme", slug: "acme"}

	r := c.do(t.Context(), time.Minute)

	assert.Equal(t, statusOK, r.Status)
	assert.Zero(t, r.Jobs)
	assert.Zero(t, fake.detailCalls)
}

func TestCheckReportsUnprobeablePage(t *testing.T) {
	// TotalCount > 0 with an empty page 1: the adapter dropped every
	// summary (e.g. Workday entries without externalPath), so the detail
	// path cannot be verified and must not pass silently.
	fake := &fakeAdapter{searchRes: searchResult(42)}
	c := check{adapter: fake, company: "Acme", slug: "acme"}

	r := c.do(t.Context(), time.Minute)

	assert.Equal(t, statusDetailError, r.Status)
	assert.Equal(t, 42, r.Jobs)
	assert.NotEmpty(t, r.Detail)
	assert.Zero(t, fake.detailCalls)
}

func TestCheckSkipsDetailWhenSearchFails(t *testing.T) {
	fake := &fakeAdapter{searchErr: errors.New("boom")}
	c := check{adapter: fake, company: "Acme", slug: "acme"}

	r := c.do(t.Context(), time.Minute)

	assert.Equal(t, statusError, r.Status)
	assert.Zero(t, fake.detailCalls)
}

func TestTallyCountsDetailErrorAsError(t *testing.T) {
	ok, errs, zero := tally([]result{
		{Status: statusOK, Jobs: 3},
		{Status: statusOK, Jobs: 0},
		{Status: statusDetailError, Jobs: 7},
		{Status: statusError},
	})
	assert.Equal(t, 2, ok)
	assert.Equal(t, 2, errs)
	assert.Equal(t, 1, zero)
}
