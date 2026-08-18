package main

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/freehire"
)

func TestSplitCSV(t *testing.T) {
	assert.Nil(t, splitCSV(""))
	assert.Equal(t, []string{"go", "rust"}, splitCSV("go,rust"))
	assert.Equal(t, []string{"go", "rust"}, splitCSV(" go, rust "))
	assert.Equal(t, []string{"go"}, splitCSV("go,,"))
}

func TestSearchParams(t *testing.T) {
	params, err := searchParams(searchFlags{
		query:     "golang",
		company:   "stripe",
		skills:    "go,rust",
		seniority: "senior",
		workMode:  "remote",
		region:    "eu",
		country:   "de",
		source:    "greenhouse",
		category:  "backend",
		sortField: "posted_at",
		order:     "desc",
		page:      2,
		limit:     20,
	})
	require.NoError(t, err)
	assert.Equal(t, "golang", params.Q.Or(""))
	assert.Equal(t, []string{"stripe"}, params.CompanySlug)
	assert.Equal(t, []string{"go", "rust"}, params.Skills)
	assert.Equal(t, []freehire.AgentSearchJobsSeniorityItem{freehire.AgentSearchJobsSeniorityItemSenior}, params.Seniority)
	assert.Equal(t, []freehire.AgentSearchJobsWorkModeItem{freehire.AgentSearchJobsWorkModeItemRemote}, params.WorkMode)
	assert.Equal(t, []freehire.AgentSearchJobsRegionsItem{freehire.AgentSearchJobsRegionsItemEu}, params.Regions)
	assert.Equal(t, []string{"de"}, params.Countries)
	assert.Equal(t, []string{"greenhouse"}, params.Source)
	assert.Equal(t, []string{"backend"}, params.Category)
	assert.Equal(t, freehire.AgentSearchJobsSortPostedAt, params.Sort.Or(""))
	assert.Equal(t, freehire.AgentSearchJobsOrderDesc, params.Order.Or(""))
	assert.Equal(t, freehire.AgentSearchJobsDescriptionFormatText, params.DescriptionFormat.Or(""))
	assert.Equal(t, 20, params.Offset.Or(-1))
	assert.Equal(t, 20, params.Limit.Or(0))
}

func TestSearchParamsRejectsInvalidSort(t *testing.T) {
	_, err := searchParams(searchFlags{page: 1, limit: 20, sortField: "nope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --sort")
	assert.Contains(t, err.Error(), "nope")
}

func TestSearchParamsRejectsDeepPage(t *testing.T) {
	_, err := searchParams(searchFlags{page: 501, limit: 20})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10000-result window")

	// The window is a row count, not a page count: a bigger --limit
	// runs out of pages sooner.
	_, err = searchParams(searchFlags{page: 101, limit: 100})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10000-result window")
}

func TestSearchParamsRejectsInvalidSeniority(t *testing.T) {
	_, err := searchParams(searchFlags{page: 1, limit: 20, seniority: "staff,nope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --seniority")
	assert.Contains(t, err.Error(), "nope")
}

func TestSummarize(t *testing.T) {
	u, err := url.Parse("https://example.com/job")
	require.NoError(t, err)
	job := freehire.Job{
		PublicSlug:  "golang-developer-sequoiaat-jfzun3rb",
		Title:       "Golang Developer",
		Company:     "sequoiaat",
		CompanySlug: freehire.NewOptString("sequoiaat"),
		Location:    freehire.NewOptString("Tamil Nadu, Chennai, India"),
		WorkMode:    freehire.NewOptString("remote"),
		Source:      freehire.NewOptString("freshteam"),
		Skills:      []string{"go", "docker"},
		PostedAt:    freehire.NewOptDateTime(time.Date(2026, time.August, 16, 19, 35, 12, 0, time.UTC)),
		URL:         freehire.NewOptURI(*u),
	}
	assert.Equal(t, jobSummaryJSON{
		Slug:        "golang-developer-sequoiaat-jfzun3rb",
		Title:       "Golang Developer",
		Company:     "sequoiaat",
		CompanySlug: "sequoiaat",
		Location:    "Tamil Nadu, Chennai, India",
		WorkMode:    "remote",
		Source:      "freshteam",
		Skills:      []string{"go", "docker"},
		PostedAt:    "2026-08-16",
		URL:         "https://example.com/job",
	}, summarize(job))
}

func TestRenderHTML(t *testing.T) {
	assert.Equal(t, "one\ntwo", renderHTML("one<br/>two"))
}
