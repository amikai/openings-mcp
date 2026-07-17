package jobindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractStashAndSearch(t *testing.T) {
	html := string(mockJobsRsp)
	resp, err := parseSearchHTML(html, 1)
	require.NoError(t, err)
	require.Len(t, resp.Jobs, 5)
	assert.Equal(t, 70, resp.TotalCount)
	assert.Equal(t, "h1683131", resp.Jobs[0].ID)
}

func TestExtractStashMissing(t *testing.T) {
	_, err := parseSearchHTML("<html><body>no stash</body></html>", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Stash")
}

func TestTidFromURL(t *testing.T) {
	assert.Equal(t, "h1683131", tidFromURL("https://www.jobindex.dk/vis-job/h1683131"))
	assert.Equal(t, "r13911770", tidFromURL("https://www.jobindex.dk/jobannonce/r13911770?x=1"))
	assert.Equal(t, "", tidFromURL("https://example.com/other"))
}

func TestTruncateDate(t *testing.T) {
	assert.Equal(t, "2026-07-15", truncateDate("2026-07-15T00:00:00"))
	assert.Equal(t, "ASAP", truncateDate("ASAP"))
}
