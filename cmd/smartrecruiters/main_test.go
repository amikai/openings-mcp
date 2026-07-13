package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	smartrecruiters "github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

func TestSummarize(t *testing.T) {
	p := smartrecruiters.PostingItem{
		ID:           smartrecruiters.NewOptString("744000137225639"),
		Name:         smartrecruiters.NewOptString("Female Locker Room Associate, Houston"),
		Location:     smartrecruiters.NewOptPostingLocation(smartrecruiters.PostingLocation{FullLocation: smartrecruiters.NewOptString("Houston, TX, United States")}),
		Department:   smartrecruiters.NewOptDepartment(smartrecruiters.Department{Label: smartrecruiters.NewOptString("Club - Staff")}),
		ReleasedDate: smartrecruiters.NewOptDateTime(time.Date(2026, 7, 10, 23, 49, 3, 0, time.UTC)),
	}
	assert.Equal(t, postingSummaryJSON{
		ID:         "744000137225639",
		Title:      "Female Locker Room Associate, Houston",
		Location:   "Houston, TX, United States",
		Department: "Club - Staff",
		PostedAt:   "2026-07-10",
	}, summarize(p))
}

func TestSummarizeEmptyOptionals(t *testing.T) {
	s := summarize(smartrecruiters.PostingItem{
		ID:   smartrecruiters.NewOptString("1"),
		Name: smartrecruiters.NewOptString("X"),
	})
	assert.Equal(t, postingSummaryJSON{ID: "1", Title: "X"}, s)
}

func TestRunSearchMissingCompany(t *testing.T) {
	err := runSearch(t.Context(), searchFlags{timeout: time.Second, limit: 20, format: "text"})
	assert.ErrorContains(t, err, "--company is required")
}

func TestRunGetMissingCompany(t *testing.T) {
	err := runGet(t.Context(), getFlags{timeout: time.Second, postingID: "744000137225639", format: "text"})
	assert.ErrorContains(t, err, "--company is required")
}

func TestRunGetMissingID(t *testing.T) {
	err := runGet(t.Context(), getFlags{company: "equinox", timeout: time.Second, format: "text"})
	assert.ErrorContains(t, err, "--id is required")
}

func TestRunSearchLimitOutOfRange(t *testing.T) {
	for _, limit := range []int{0, -1, 101} {
		err := runSearch(t.Context(), searchFlags{company: "equinox", timeout: time.Second, limit: limit, format: "text"})
		assert.ErrorContainsf(t, err, "--limit must be between 1 and 100", "limit=%d", limit)
	}
}

func TestRunSearchOffsetNegative(t *testing.T) {
	err := runSearch(t.Context(), searchFlags{company: "equinox", timeout: time.Second, limit: 20, offset: -1, format: "text"})
	assert.ErrorContains(t, err, "--offset must be >= 0")
}

func TestNormalizeCompanyUnknown(t *testing.T) {
	_, err := normalizeCompany("doesnotexist-company-xyz")
	require.ErrorContains(t, err, `company "doesnotexist-company-xyz" not found`)
	assert.ErrorContains(t, err, "smartrecruiters companies")
}

// TestNormalizeCompanyCanonicalCasing guards that a case-insensitive match
// returns the roster's stored casing (e.g. "Equinox"), not whatever casing
// the caller typed — the API is case-insensitive, but report output and
// the params sent upstream should stay consistent with companies.yaml.
func TestNormalizeCompanyCanonicalCasing(t *testing.T) {
	got, err := normalizeCompany("equinox")
	require.NoError(t, err)
	assert.Equal(t, "Equinox", got)
}

func TestRunSearchUnknownCompany(t *testing.T) {
	err := runSearch(t.Context(), searchFlags{company: "doesnotexist-company-xyz", timeout: time.Second, limit: 20, format: "text"})
	require.ErrorContains(t, err, `company "doesnotexist-company-xyz" not found`)
	assert.ErrorContains(t, err, "smartrecruiters companies")
}

func TestRunGetUnknownCompany(t *testing.T) {
	err := runGet(t.Context(), getFlags{company: "doesnotexist-company-xyz", timeout: time.Second, postingID: "744000137225639", format: "text"})
	require.ErrorContains(t, err, `company "doesnotexist-company-xyz" not found`)
	assert.ErrorContains(t, err, "smartrecruiters companies")
}
