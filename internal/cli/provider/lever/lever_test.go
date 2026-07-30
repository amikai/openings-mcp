package lever

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	leverprovider "github.com/amikai/openings-mcp/internal/provider/lever"
)

func TestToPostingJSON(t *testing.T) {
	p := leverprovider.Posting{
		ID:               "id-1",
		Text:             leverprovider.NewNilString("Backend Engineer"),
		CreatedAt:        leverprovider.NewOptNilInt64(1553186035299),
		HostedUrl:        leverprovider.NewOptNilString("https://jobs.lever.co/leverdemo/id-1"),
		DescriptionPlain: leverprovider.NewOptNilString("plain description"),
		Categories: leverprovider.NewOptPostingCategories(leverprovider.PostingCategories{
			Location:     leverprovider.NewOptNilString("Taipei"),
			Team:         leverprovider.NewOptNilString("Engineering"),
			Commitment:   leverprovider.NewOptNilString("Full-time"),
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
	assert.Equal(t, want, toPostingJSON(&p))
}

func TestToPostingJSONSingleLocationFallback(t *testing.T) {
	p := leverprovider.Posting{
		ID:   "id-2",
		Text: leverprovider.NewNilString("Designer"),
		Categories: leverprovider.NewOptPostingCategories(leverprovider.PostingCategories{
			Location: leverprovider.NewOptNilString("Remote"),
		}),
	}

	want := postingJSON{
		ID:       "id-2",
		Title:    "Designer",
		Location: "Remote",
	}
	assert.Equal(t, want, toPostingJSON(&p))
}

func TestToPostingJSONNoCategories(t *testing.T) {
	p := leverprovider.Posting{ID: "id-3", Text: leverprovider.NewNilString("PM")}

	want := postingJSON{ID: "id-3", Title: "PM"}
	assert.Equal(t, want, toPostingJSON(&p))
}

func TestRunSearchMissingSite(t *testing.T) {
	err := runSearch(t.Context(), searchFlags{timeout: time.Second, limit: 20, format: "text"})
	assert.ErrorContains(t, err, "--site is required")
}

func TestRunSearchUnknownSite(t *testing.T) {
	err := runSearch(t.Context(), searchFlags{site: "doesnotexist-site-xyz", timeout: time.Second, limit: 20, format: "text"})
	require.ErrorContains(t, err, `site "doesnotexist-site-xyz" not found`)
	assert.ErrorContains(t, err, "lever companies")
}

func TestRunGetMissingSite(t *testing.T) {
	err := runGet(t.Context(), getFlags{timeout: time.Second, postingID: "some-id", format: "text"})
	assert.ErrorContains(t, err, "--site is required")
}

func TestRunGetUnknownSite(t *testing.T) {
	err := runGet(t.Context(), getFlags{site: "doesnotexist-site-xyz", timeout: time.Second, postingID: "some-id", format: "text"})
	require.ErrorContains(t, err, `site "doesnotexist-site-xyz" not found`)
	assert.ErrorContains(t, err, "lever companies")
}

func TestRunGetMissingPostingID(t *testing.T) {
	err := runGet(t.Context(), getFlags{site: "leverdemo", timeout: time.Second, format: "text"})
	assert.ErrorContains(t, err, "posting id argument is required")
}
