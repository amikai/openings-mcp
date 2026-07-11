package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lever "github.com/amikai/openings-mcp/internal/provider/lever"
)

func TestToPostingJSON(t *testing.T) {
	p := lever.Posting{
		ID:               "id-1",
		Text:             lever.NewNilString("Backend Engineer"),
		CreatedAt:        lever.NewOptNilInt64(1553186035299),
		HostedUrl:        lever.NewOptNilString("https://jobs.lever.co/leverdemo/id-1"),
		DescriptionPlain: lever.NewOptNilString("plain description"),
		Categories: lever.NewOptPostingCategories(lever.PostingCategories{
			Location:     lever.NewOptNilString("Taipei"),
			Team:         lever.NewOptNilString("Engineering"),
			Commitment:   lever.NewOptNilString("Full-time"),
			AllLocations: []string{"Taipei", "Tokyo"},
		}),
	}

	want := postingJSON{
		ID:          "id-1",
		Title:       "Backend Engineer",
		URL:         "https://jobs.lever.co/leverdemo/id-1",
		CreatedAt:   "2019-03-21",
		Location:    "Taipei",
		Locations:   []string{"Taipei", "Tokyo"},
		Team:        "Engineering",
		Commitment:  "Full-time",
		Description: "plain description",
	}
	assert.Equal(t, want, toPostingJSON(p))
}

func TestToPostingJSONSingleLocationFallback(t *testing.T) {
	p := lever.Posting{
		ID:   "id-2",
		Text: lever.NewNilString("Designer"),
		Categories: lever.NewOptPostingCategories(lever.PostingCategories{
			Location: lever.NewOptNilString("Remote"),
		}),
	}

	want := postingJSON{
		ID:       "id-2",
		Title:    "Designer",
		Location: "Remote",
	}
	assert.Equal(t, want, toPostingJSON(p))
}

func TestToPostingJSONNoCategories(t *testing.T) {
	p := lever.Posting{ID: "id-3", Text: lever.NewNilString("PM")}

	want := postingJSON{ID: "id-3", Title: "PM"}
	assert.Equal(t, want, toPostingJSON(p))
}

func TestRunSearchMissingSite(t *testing.T) {
	err := runSearch(t.Context(), "", time.Second, nil, nil, nil, nil, "", 20, 0, "text")
	assert.ErrorContains(t, err, "--site is required")
}

func TestRunSearchUnknownSite(t *testing.T) {
	err := runSearch(t.Context(), "doesnotexist-site-xyz", time.Second, nil, nil, nil, nil, "", 20, 0, "text")
	require.ErrorContains(t, err, `site "doesnotexist-site-xyz" not found`)
	assert.ErrorContains(t, err, "lever companies")
}

func TestRunGetMissingSite(t *testing.T) {
	err := runGet(t.Context(), "", time.Second, "some-id", "text")
	assert.ErrorContains(t, err, "--site is required")
}

func TestRunGetUnknownSite(t *testing.T) {
	err := runGet(t.Context(), "doesnotexist-site-xyz", time.Second, "some-id", "text")
	require.ErrorContains(t, err, `site "doesnotexist-site-xyz" not found`)
	assert.ErrorContains(t, err, "lever companies")
}

func TestRunGetMissingPostingID(t *testing.T) {
	err := runGet(t.Context(), "leverdemo", time.Second, "", "text")
	assert.ErrorContains(t, err, "posting id argument is required")
}
