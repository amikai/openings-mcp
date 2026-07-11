package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workday "github.com/amikai/openings-mcp/internal/provider/workday"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestFetchJobResultUsesTenantClient(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/wday/cxs/nvidia/NVIDIAExternalCareerSite/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916", r.URL.Path)
		header := make(http.Header)
		header.Set("Content-Type", "application/json")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader(`{"jobPostingInfo":{"title":"Senior Software Engineer","jobDescription":"<p>description</p>","location":"Israel, Yokneam","externalUrl":"https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916"}}`)),
		}, nil
	})

	client, err := workday.NewTenantClient(workday.WithClient(&http.Client{Transport: transport}))
	require.NoError(t, err)

	got := fetchJobResult(
		context.Background(),
		client,
		"nvidia",
		"https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite",
		workday.JobSummary{
			Title:        workday.NewOptNilString("Search title"),
			ExternalPath: workday.NewOptString("/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916"),
		},
	)

	assert.Equal(t, "Senior Software Engineer", got.Title)
	assert.Equal(t, "Israel, Yokneam", got.Location)
	assert.Equal(t, "https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916", got.URL)
	assert.Equal(t, "description", got.Description)
}
