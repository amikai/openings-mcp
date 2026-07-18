package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/apple"
)

const cliTestCountryCode = "TWN"

func TestRunSearchValidation(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		flags searchFlags
	}{
		{name: "keyword", flags: searchFlags{country: cliTestCountryCode, page: 1}, want: "--keyword is required"},
		{name: "country", flags: searchFlags{keyword: "camera", page: 1}, want: "--country is required"},
		{name: "page", flags: searchFlags{keyword: "camera", country: cliTestCountryCode}, want: "--page must be >= 1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runSearch(t.Context(), test.flags, &bytes.Buffer{})
			assert.ErrorContains(t, err, test.want)
		})
	}
}

func TestRunDetailRequiresJobID(t *testing.T) {
	err := runDetail(t.Context(), detailFlags{timeout: time.Second}, &bytes.Buffer{})
	assert.ErrorContains(t, err, "--job-id is required")
}

func TestWriteSearch(t *testing.T) {
	server := apple.NewMockServer()
	t.Cleanup(server.Close)
	client, err := apple.NewJobsClient(server.URL, server.Client())
	require.NoError(t, err)
	response, err := client.SearchJobs(t.Context(), apple.SearchRequest{
		Keyword:     "software engineer",
		CountryCode: cliTestCountryCode,
	})
	require.NoError(t, err)

	var output bytes.Buffer
	require.NoError(t, writeSearch(&output, "text", 1, response))
	assert.Contains(t, output.String(), "total=11 page=1 jobs=11")
	assert.Contains(t, output.String(), "[200624996] SoC Packaging Engineer")
	assert.Contains(t, output.String(), "Taipei, Taiwan")
}

func TestWriteDetail(t *testing.T) {
	server := apple.NewMockServer()
	t.Cleanup(server.Close)
	client, err := apple.NewJobsClient(server.URL, server.Client())
	require.NoError(t, err)
	response, err := client.JobDetail(t.Context(), apple.MockJobID)
	require.NoError(t, err)

	var output bytes.Buffer
	require.NoError(t, writeDetail(&output, "text", response))
	assert.Contains(t, output.String(), "[200624996] SoC Packaging Engineer")
	assert.Contains(t, output.String(), "Responsibilities")
	assert.Contains(t, output.String(), "Minimum qualifications")
}

func TestLocationLabel(t *testing.T) {
	assert.Equal(t, "Taipei, Taiwan", locationLabel("Taipei", "Taiwan"))
	assert.Equal(t, "Taiwan", locationLabel("Taiwan", "Taiwan"))
	assert.Equal(t, "Taiwan", locationLabel("", "Taiwan"))
}
