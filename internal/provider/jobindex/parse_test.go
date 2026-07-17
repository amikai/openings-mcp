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
	require.Len(t, resp.Results, 5)
	assert.Equal(t, 70, resp.Hitcount)
	assert.Greater(t, resp.TotalPages, 0) // pass-through from Stash, not synthesized
	assert.Equal(t, "h1683131", resp.Results[0]["tid"])
	_, hasHTML := resp.Results[0]["html"]
	assert.False(t, hasHTML)
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
