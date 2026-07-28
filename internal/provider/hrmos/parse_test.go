package hrmos

import (
	"bytes"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJobsHTML(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(mockJobsP1Rsp))
	require.NoError(t, err)

	got, err := parseJobsHTML(doc)
	require.NoError(t, err)

	assert.Equal(t, 203, got.Total)
	require.Len(t, got.Jobs, 100)
	assert.Equal(t, 3, got.MaxPage)

	job := got.Jobs[0]
	assert.Equal(t, "00000des01", job.ID)
	assert.Equal(t, "https://hrmos.co/pages/moneyforward/jobs/00000des01", job.URL)
	assert.NotEmpty(t, job.Title)
	assert.NotEmpty(t, job.Description)
	assert.Contains(t, job.Chips, "東京")
	// A NAMED fixture job: locations must equal the sg-tag-location chip
	// verbatim, including the 他(N) suffix — never collapsed to a bare
	// facet chip like "東京" alone (fact 17).
	assert.Equal(t, "東京都港区芝浦3-1-21 msb Tamachi 田町ステーションタワーS 21F 他(3)", job.Location)
}

func TestParseJobsHTMLPastTheEnd(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(mockJobsP4Rsp))
	require.NoError(t, err)

	got, err := parseJobsHTML(doc)
	require.NoError(t, err)

	assert.Equal(t, 0, got.Total)
	assert.Empty(t, got.Jobs)
	assert.Equal(t, 3, got.MaxPage, "the pager still lists the real page range past the end (fact 14)")
}

func TestParseJobsHTMLNoFacets(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(mockJobsNofacetsRsp))
	require.NoError(t, err)

	got, err := parseJobsHTML(doc)
	require.NoError(t, err)

	assert.Equal(t, 86, got.Total)
	require.Len(t, got.Jobs, 86)
	assert.Empty(t, got.Facets)
	// Fact 17: the sg-tag-location chip occurs exactly once per job. Its
	// text is normally a full address, but at least one live posting in
	// this capture has the chip present with empty text (an employer left
	// the address field blank) — the parser must not error or drop the
	// job over that, so most jobs still carry a non-empty location.
	var nonEmpty int
	for _, j := range got.Jobs {
		if j.Location != "" {
			nonEmpty++
		}
	}
	assert.Greater(t, nonEmpty, len(got.Jobs)-2)
}

func TestParseJobDetailHTML(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(mockJobDetailSalaryRsp))
	require.NoError(t, err)

	got, err := parseJobDetailHTML(doc, MockJobIDSalary, "https://hrmos.co/pages/visional/jobs/0000381")
	require.NoError(t, err)

	assert.Equal(t, "JPY", got.SalaryCurrency)
	assert.Equal(t, "6000000", got.SalaryMin)
	assert.Equal(t, "10000000", got.SalaryMax)
	assert.Equal(t, "YEAR", got.SalaryUnit)
	require.Len(t, got.Locations, 1)
	// visional's addressLocality retains the bracketed office label prefix
	// (fact 17's detail-page counterpart).
	assert.Contains(t, got.Locations[0].Locality, "［渋谷本社］")
	assert.NotEmpty(t, got.JobInfo)
	assert.NotEmpty(t, got.CompanyInfo)
}

func TestParseJobDetailHTMLNilSalary(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(mockJobDetailRsp))
	require.NoError(t, err)

	got, err := parseJobDetailHTML(doc, MockJobID, "https://hrmos.co/pages/moneyforward/jobs/0000265")
	require.NoError(t, err)

	assert.Empty(t, got.SalaryCurrency)
	assert.Empty(t, got.SalaryMin)
	assert.Empty(t, got.SalaryMax)
	assert.Empty(t, got.SalaryUnit)
}

func TestParseJobCassetteMalformedHref(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(
		`<li class="pg-list-cassette"><a href="https://hrmos.co/other/path"><h2>t</h2></a></li>`,
	)))
	require.NoError(t, err)

	_, err = parseJobCassette(doc.Find("li.pg-list-cassette").First())
	assert.Error(t, err)
}
