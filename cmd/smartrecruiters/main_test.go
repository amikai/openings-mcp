package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	smartrecruiters "github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

func TestSummarize(t *testing.T) {
	p := smartrecruiters.PostingSummary{
		ID:           smartrecruiters.NewOptString("744000137225639"),
		Name:         smartrecruiters.NewOptNilString("Female Locker Room Associate, Houston"),
		Location:     smartrecruiters.NewOptLocation(smartrecruiters.Location{FullLocation: smartrecruiters.NewOptNilString("Houston, TX, United States")}),
		Department:   smartrecruiters.NewOptDepartment(smartrecruiters.Department{Label: smartrecruiters.NewOptNilString("Club - Staff")}),
		ReleasedDate: smartrecruiters.NewOptNilDateTime(time.Date(2026, 7, 10, 23, 49, 3, 0, time.UTC)),
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
	s := summarize(smartrecruiters.PostingSummary{
		ID:   smartrecruiters.NewOptString("1"),
		Name: smartrecruiters.NewOptNilString("X"),
	})
	assert.Equal(t, postingSummaryJSON{ID: "1", Title: "X"}, s)
}

// TestSummarizeNullReleasedDate guards a present-but-explicitly-null
// releasedDate: PostedAt must stay empty, not format the zero time as if
// it were real data.
func TestSummarizeNullReleasedDate(t *testing.T) {
	p := smartrecruiters.PostingSummary{
		ID:           smartrecruiters.NewOptString("1"),
		Name:         smartrecruiters.NewOptNilString("X"),
		ReleasedDate: smartrecruiters.OptNilDateTime{Set: true, Null: true},
	}
	assert.Empty(t, summarize(p).PostedAt)
}

func TestRunSearchMissingCompany(t *testing.T) {
	err := runSearch(t.Context(), "", time.Second, "", "", "", "", "", 20, 0, "text")
	assert.ErrorContains(t, err, "--company is required")
}

func TestRunGetMissingCompany(t *testing.T) {
	err := runGet(t.Context(), "", time.Second, "744000137225639", "text")
	assert.ErrorContains(t, err, "--company is required")
}

func TestRunGetMissingID(t *testing.T) {
	err := runGet(t.Context(), "equinox", time.Second, "", "text")
	assert.ErrorContains(t, err, "--id is required")
}
