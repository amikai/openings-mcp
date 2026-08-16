package icims

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSearchHTMLFixture(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(mockSearchRsp)))
	require.NoError(t, err)

	res, err := parseSearchHTML(doc)
	require.NoError(t, err)
	assert.Equal(t, 1, res.TotalPages)
	assert.Equal(t, 3, res.PageSize)
	require.Len(t, res.Jobs, 3)
	assert.Equal(t, Job{
		ID:       "1977",
		Slug:     "senior-product-manager",
		Title:    "Senior Product Manager",
		Location: "US-TX-Austin",
	}, res.Jobs[0])
	require.NotEmpty(t, res.Locations)
	assert.Equal(t, "12781-12827-Austin", res.Locations[0].Value)
	assert.Contains(t, res.Locations[0].Label, "Austin")
	assert.Contains(t, res.Categories, SelectOption{Value: "36942", Label: "Technology - Product Management"})
	assert.Contains(t, res.PositionTypes, SelectOption{Value: "2049", Label: "Full-Time"})
}

func TestMatchLocationOptions(t *testing.T) {
	opts := []SelectOption{
		{Value: "12781-12827-Austin", Label: "TX Austin US"},
		{Value: "12781-12830-Lorton", Label: "VA Lorton US"},
	}
	assert.Equal(t, []string{"12781-12827-Austin"}, MatchLocationOptions(opts, "Austin"))
	assert.Equal(t, []string{"12781-12830-Lorton"}, MatchLocationOptions(opts, "12781-12830-Lorton"))
	// Broad country match must retain every US option, not collapse to one city.
	assert.Equal(t, []string{"12781-12827-Austin", "12781-12830-Lorton"}, MatchLocationOptions(opts, "US"))
	assert.Empty(t, MatchLocationOptions(opts, "Seattle"))

	// Substring trap: "us" is embedded in "Austin" but must not match the
	// value alone when the label has no separate US token.
	assert.Empty(t, MatchLocationOptions([]SelectOption{
		{Value: "1-1-Austin", Label: "Austin TX"},
	}, "US"))
	// State code must be a full token, not a substring of a city name.
	assert.Empty(t, MatchLocationOptions([]SelectOption{
		{Value: "1-1-Orlando", Label: "Orlando FL"},
	}, "OR"))
}

func TestMatchLocationOptionsRealisticLabels(t *testing.T) {
	opts := []SelectOption{
		{Value: "1-1-Titusville", Label: "FL,Titusville"},
		{Value: "1-2-Columbus", Label: "OH,Columbus"},
		{Value: "1-3-King-of-Prussia", Label: "PA,King of Prussia"},
		{Value: "1-4-Los-Angeles", Label: "CA,Los Angeles"},
		{Value: "1-5-Indianapolis", Label: "IN,Indianapolis"},
	}

	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "country is not embedded substring", text: "US", want: nil},
		{name: "state token CA", text: "CA", want: []string{"1-4-Los-Angeles"}},
		{name: "state token IN", text: "IN", want: []string{"1-5-Indianapolis"}},
		{name: "multi-word city", text: "Los Angeles", want: []string{"1-4-Los-Angeles"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MatchLocationOptions(opts, tt.text))
		})
	}
}

func TestLooksLikeLocationValue(t *testing.T) {
	assert.True(t, LooksLikeLocationValue("12781-12827-Austin"))
	assert.False(t, LooksLikeLocationValue("Austin"))
	assert.False(t, LooksLikeLocationValue("TX Austin US"))
}

func TestParseSearchPostedFixture(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(mockSearchPostedRsp)))
	require.NoError(t, err)

	res, err := parseSearchHTML(doc)
	require.NoError(t, err)
	require.Len(t, res.Jobs, 1)
	assert.Equal(t, "167924", res.Jobs[0].ID)
	// The date span's title attribute carries the absolute timestamp; the
	// visible text is only relative ("3 weeks ago").
	assert.Equal(t, "6/25/2026 4:15 PM", res.Jobs[0].PostedAt)
}

func TestParseSearchNoResults(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(mockSearchNoResultsRsp)))
	require.NoError(t, err)

	res, err := parseSearchHTML(doc)
	require.NoError(t, err)
	assert.Empty(t, res.Jobs)
	assert.Equal(t, 0, res.PageSize)
	assert.Equal(t, 1, res.TotalPages)
}

func TestParseJobDetailFixture(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(mockJobDetailRsp)))
	require.NoError(t, err)

	d, ok := parseJobDetailHTML(doc, "1977")
	require.True(t, ok)
	assert.Equal(t, "Senior Product Manager", d.Title)
	assert.Contains(t, d.Location, "Austin")
	assert.Contains(t, d.DescriptionHTML, "Overview")
	assert.Equal(t, "FULL_TIME", d.EmploymentType)
}

func TestParseJobDetailMultipleLocations(t *testing.T) {
	// Mirrors live Peraton job 168635: two jobLocation entries must both
	// survive, not just the first.
	page := `<html><head><script type="application/ld+json">
	{"@context":"https://schema.org","@type":"JobPosting","title":"Systems Engineer",
	 "jobLocation":[
	   {"@type":"Place","address":{"addressLocality":"Herndon","addressRegion":"VA","addressCountry":"US"}},
	   {"@type":"Place","address":{"addressLocality":"Annapolis Junction","addressRegion":"MD","addressCountry":"US"}}
	 ]}</script></head><body></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	require.NoError(t, err)

	d, ok := parseJobDetailHTML(doc, "168635")
	require.True(t, ok)
	assert.Equal(t, "Herndon, VA, US; Annapolis Junction, MD, US", d.Location)
}

func TestParseJobDetailNotFoundBody(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(mockJobDetailNotFoundRsp)))
	require.NoError(t, err)

	_, ok := parseJobDetailHTML(doc, "999999999")
	assert.False(t, ok)
	assert.True(t, isSearchLikeDetailBody(doc))
}

// Portals without JobPosting JSON-LD still render the iframe chrome
// (h1.iCIMS_Header + expandable description sections).
func TestParseJobDetailPortalHTMLFallback(t *testing.T) {
	const page = `<html><body>
<div class="container-fluid iCIMS_JobsTable">
  <div class="row">
    <div class="col-xs-12 title">
      <h1 class="iCIMS_Header">Event Bartender | Part-Time</h1>
    </div>
    <div class="col-xs-6 header left"><span class="sr-only field-label">Location</span><span>US-TX-Lubbock</span></div>
    <div class="col-xs-6 header right"><span title="7/16/2026 5:21 PM">15 minutes ago</span></div>
  </div>
  <div class="iCIMS_Expandable_Text"><p>Oak View Group (OVG) is the global leader.</p></div>
  <div class="iCIMS_Expandable_Text"><p>Responsibilities include setup.</p></div>
</div>
</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	require.NoError(t, err)

	d, ok := parseJobDetailHTML(doc, "33322")
	require.True(t, ok)
	assert.Equal(t, "Event Bartender | Part-Time", d.Title)
	assert.Equal(t, "US-TX-Lubbock", d.Location)
	assert.Equal(t, "7/16/2026 5:21 PM", d.PostedAtRaw)
	assert.Contains(t, d.DescriptionHTML, "Oak View Group")
	assert.Contains(t, d.DescriptionHTML, "Responsibilities include setup")
}

func TestJobIDAndSlugFromHref(t *testing.T) {
	id, slug := jobIDAndSlugFromHref("https://careers-buspatrol.icims.com/jobs/1977/senior-product-manager/job?in_iframe=1")
	assert.Equal(t, "1977", id)
	assert.Equal(t, "senior-product-manager", slug)
}
