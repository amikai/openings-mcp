package main

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	greenhouse "github.com/amikai/openings-mcp/internal/provider/greenhouse"
)

func mustURL(t *testing.T, s string) url.URL {
	t.Helper()
	u, err := url.Parse(s)
	assert.NoError(t, err)
	return *u
}

func TestSummarize(t *testing.T) {
	j := greenhouse.JobSummary{
		ID:             greenhouse.NewOptInt(4425455),
		Title:          greenhouse.NewOptString("Staff Engineer"),
		FirstPublished: greenhouse.NewOptDateTime(time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)),
		UpdatedAt:      greenhouse.NewOptDateTime(time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)),
		Location:       greenhouse.NewOptLocation(greenhouse.Location{Name: greenhouse.NewOptString("Taipei, Taiwan")}),
		AbsoluteURL:    greenhouse.NewOptURI(mustURL(t, "https://boards.greenhouse.io/acme/jobs/4425455")),
	}
	assert.Equal(t, jobSummaryJSON{
		ID:        4425455,
		Title:     "Staff Engineer",
		Location:  "Taipei, Taiwan",
		PostedAt:  "2026-05-01",
		UpdatedAt: "2026-06-20",
		URL:       "https://boards.greenhouse.io/acme/jobs/4425455",
	}, summarize(j))
}

func TestSummarizeEmptyOptionals(t *testing.T) {
	s := summarize(greenhouse.JobSummary{ID: greenhouse.NewOptInt(1), Title: greenhouse.NewOptString("X")})
	assert.Equal(t, jobSummaryJSON{ID: 1, Title: "X"}, s)
}

func TestMatches(t *testing.T) {
	s := jobSummaryJSON{Title: "Senior Software Engineer", Location: "Taipei, Taiwan"}
	assert.True(t, matches(s, "", ""), "empty filters match everything")
	assert.True(t, matches(s, "software", ""), "keyword is case-insensitive substring on title")
	assert.True(t, matches(s, "", "taipei"), "location is case-insensitive substring")
	assert.True(t, matches(s, "engineer", "taiwan"), "both filters AND together")
	assert.False(t, matches(s, "manager", ""))
	assert.False(t, matches(s, "engineer", "london"), "one failing filter fails the AND")
}

func TestFormatCents(t *testing.T) {
	assert.Equal(t, "136000", formatCents(13600000), "whole units drop the decimals")
	assert.Equal(t, "1359.99", formatCents(135999), "fractional cents keep two decimals")
}

func TestPayRangeLine(t *testing.T) {
	r := greenhouse.PayInputRange{
		MinCents:     greenhouse.NewOptInt(13600000),
		MaxCents:     greenhouse.NewOptInt(20000000),
		CurrencyType: greenhouse.NewOptString("USD"),
		Title:        greenhouse.NewOptString("Base Salary"),
	}
	assert.Equal(t, "Base Salary: 136000 – 200000 USD", payRangeLine(r))

	untitled := greenhouse.PayInputRange{
		MinCents:     greenhouse.NewOptInt(5000000),
		MaxCents:     greenhouse.NewOptInt(7000000),
		CurrencyType: greenhouse.NewOptString("EUR"),
	}
	assert.Equal(t, "50000 – 70000 EUR", payRangeLine(untitled))
}

func TestRenderDescription(t *testing.T) {
	// Greenhouse sends entity-encoded HTML: decode first, then strip tags.
	got := renderDescription("&lt;p&gt;Build &amp;amp; ship things.&lt;/p&gt;")
	assert.Contains(t, got, "Build & ship things.")
	assert.NotContains(t, got, "<p>")
}

func TestRunSearchMissingBoard(t *testing.T) {
	err := runSearch(t.Context(), "", time.Second, "", "", "text")
	assert.ErrorContains(t, err, "--board is required")
}

func TestRunSearchUnknownBoard(t *testing.T) {
	err := runSearch(t.Context(), "doesnotexist-board-xyz", time.Second, "", "", "text")
	assert.ErrorContains(t, err, `board "doesnotexist-board-xyz" not found`)
	assert.ErrorContains(t, err, "greenhouse companies")
}

func TestRunGetMissingID(t *testing.T) {
	err := runGet(t.Context(), "anthropic", time.Second, 0, "text")
	assert.ErrorContains(t, err, "--id is required")
}

func TestRunGetMissingBoard(t *testing.T) {
	err := runGet(t.Context(), "", time.Second, 123, "text")
	assert.ErrorContains(t, err, "--board is required")
}

func TestRunGetUnknownBoard(t *testing.T) {
	err := runGet(t.Context(), "doesnotexist-board-xyz", time.Second, 123, "text")
	assert.ErrorContains(t, err, `board "doesnotexist-board-xyz" not found`)
	assert.ErrorContains(t, err, "greenhouse companies")
}
