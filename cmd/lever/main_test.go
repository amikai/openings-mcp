package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	lever "github.com/amikai/openings-mcp/internal/provider/lever"
)

func TestToPostingJSON(t *testing.T) {
	p := lever.Posting{
		ID:               "id-1",
		Text:             "Backend Engineer",
		CreatedAt:        lever.NewOptInt64(1553186035299),
		HostedUrl:        lever.NewOptString("https://jobs.lever.co/leverdemo/id-1"),
		DescriptionPlain: lever.NewOptString("plain description"),
		Categories: lever.NewOptPostingCategories(lever.PostingCategories{
			Location:     lever.NewOptString("Taipei"),
			Team:         lever.NewOptString("Engineering"),
			Commitment:   lever.NewOptString("Full-time"),
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
		Text: "Designer",
		Categories: lever.NewOptPostingCategories(lever.PostingCategories{
			Location: lever.NewOptString("Remote"),
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
	p := lever.Posting{ID: "id-3", Text: "PM"}

	want := postingJSON{ID: "id-3", Title: "PM"}
	assert.Equal(t, want, toPostingJSON(p))
}

func TestRunSearchMissingSite(t *testing.T) {
	err := runSearch(context.Background(), "", time.Second, nil, nil, nil, nil, "", 20, 0, "text")
	assert.ErrorContains(t, err, "--site is required")
}

func TestRunSearchUnknownSite(t *testing.T) {
	err := runSearch(context.Background(), "doesnotexist-site-xyz", time.Second, nil, nil, nil, nil, "", 20, 0, "text")
	assert.ErrorContains(t, err, `site "doesnotexist-site-xyz" not found`)
	assert.ErrorContains(t, err, "lever companies")
}

func TestRunGetMissingSite(t *testing.T) {
	err := runGet(context.Background(), "", time.Second, "some-id", "text")
	assert.ErrorContains(t, err, "--site is required")
}

func TestRunGetUnknownSite(t *testing.T) {
	err := runGet(context.Background(), "doesnotexist-site-xyz", time.Second, "some-id", "text")
	assert.ErrorContains(t, err, `site "doesnotexist-site-xyz" not found`)
	assert.ErrorContains(t, err, "lever companies")
}

func TestRunGetMissingPostingID(t *testing.T) {
	err := runGet(context.Background(), "leverdemo", time.Second, "", "text")
	assert.ErrorContains(t, err, "posting id argument is required")
}
