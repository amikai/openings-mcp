package engage

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBoardNoJobList(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<html><body>not a board</body></html>`))
	require.NoError(t, err)

	_, err = parseBoard(doc)
	require.ErrorIs(t, err, ErrEmptyBoard)
}

func TestParseBoardCategoryWithNoJobs(t *testing.T) {
	// A dt.category with no dd.dataArea following it — everything else
	// being equal, this still means selector drift / a genuinely job-less
	// board, both of which engage never serves live (see doc.go).
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
		<dl class="jobList"><dt class="category">中途採用</dt></dl>
	`))
	require.NoError(t, err)

	_, err = parseBoard(doc)
	require.ErrorIs(t, err, ErrEmptyBoard)
}

func TestParseSalariesObject(t *testing.T) {
	raw := json.RawMessage(`{"@type":"MonetaryAmount","currency":"JPY","value":{"@type":"QuantitativeValue","unitText":"YEAR","minValue":"3600000"}}`)

	got, err := parseSalaries(raw)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, Salary{Currency: "JPY", UnitText: "YEAR", MinValue: "3600000"}, got[0])
}

func TestParseSalariesArray(t *testing.T) {
	// cookbiz_jobs publishes both an annual and a monthly figure for the
	// same posting — baseSalary is an array there, not a single object.
	raw := json.RawMessage(`[
		{"currency":"JPY","value":{"unitText":"YEAR","minValue":"2520000","maxValue":"4200000"}},
		{"currency":"JPY","value":{"unitText":"MONTH","minValue":"210000","maxValue":"350000"}}
	]`)

	got, err := parseSalaries(raw)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "YEAR", got[0].UnitText)
	assert.Equal(t, "MONTH", got[1].UnitText)
	assert.Equal(t, "4200000", got[0].MaxValue)
}

func TestParseSalariesAbsent(t *testing.T) {
	got, err := parseSalaries(nil)
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = parseSalaries(json.RawMessage(`null`))
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParseJobDetailNoLdJSON(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<html><body>not a detail page</body></html>`))
	require.NoError(t, err)

	_, err = parseJobDetail(doc)
	require.Error(t, err)
}

func TestParseJobDetailWrongType(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(
		`<script type="application/ld+json">{"@type":"Organization"}</script>`))
	require.NoError(t, err)

	_, err = parseJobDetail(doc)
	require.Error(t, err)
}

func TestParseBoardDate(t *testing.T) {
	assert.False(t, parseBoardDate("2026/7/16").IsZero())
	assert.True(t, parseBoardDate("").IsZero())
	assert.True(t, parseBoardDate("not a date").IsZero())
}

// TestParseJobDetailWithoutJSONLD covers the postings engage renders without a
// JSON-LD block. The HTML fallback must still recover the headline, employer,
// and section bodies; the JSON-LD-only fields stay zero.
func TestParseJobDetailWithoutJSONLD(t *testing.T) {
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, srv.Client())

	d, err := c.Job(t.Context(), MockNoJSONLDSlug, MockNoJSONLDWorkID)
	require.NoError(t, err)

	assert.Equal(t, MockNoJSONLDWorkID, d.WorkID, "work id is backfilled from the request")
	assert.Equal(t, "コーダーエンジニア", d.Title)
	assert.Equal(t, "株式会社アスパーク", d.Organization.Name)
	assert.NotEmpty(t, d.Sections)

	// Fields that exist only in the JSON-LD have no HTML equivalent.
	assert.Empty(t, d.DescriptionHTML)
	assert.True(t, d.DatePosted.IsZero())
	assert.Empty(t, d.EmploymentType)
	assert.Empty(t, d.Salaries)
}
