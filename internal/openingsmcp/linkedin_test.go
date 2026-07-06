package openingsmcp

import (
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/linkedin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkedinMCPToHTTPRequest(t *testing.T) {
	in := linkedinSearchInput{
		Keyword:       "software engineer",
		Location:      "Taiwan",
		Distance:      25,
		WorkplaceType: "Remote",
		JobType:       "Full-time",
		EasyApply:     true,
		CompanyIDs:    "1441, 162479",
		PostedWithin:  "Past week",
		Start:         10,
	}
	got, err := linkedinMCPToHTTPRequest(&in)
	require.NoError(t, err)

	want := &linkedin.JobsRequest{
		Keywords:            "software engineer",
		Location:            "Taiwan",
		Distance:            25,
		WorkplaceType:       linkedin.WorkplaceRemote,
		JobType:             linkedin.JobTypeFullTime,
		EasyApply:           true,
		CompanyIDs:          []string{"1441", "162479"},
		PostedWithinSeconds: 604800,
		Start:               10,
	}
	assert.Equal(t, want, got)
}

func TestLinkedinMCPToHTTPRequestMinimal(t *testing.T) {
	got, err := linkedinMCPToHTTPRequest(&linkedinSearchInput{})
	require.NoError(t, err)
	assert.Equal(t, &linkedin.JobsRequest{}, got)
}

func TestLinkedinMCPToHTTPRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   linkedinSearchInput
		want string
	}{
		{"workplace_type", linkedinSearchInput{WorkplaceType: "Space"}, `invalid workplace_type "Space"`},
		{"job_type", linkedinSearchInput{JobType: "Volunteer"}, `invalid job_type "Volunteer"`},
		{"posted_within", linkedinSearchInput{PostedWithin: "Past year"}, `invalid posted_within "Past year"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := linkedinMCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
