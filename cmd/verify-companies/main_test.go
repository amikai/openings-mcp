package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProvidersIncludesTeamtailor(t *testing.T) {
	got, err := parseProviders("workday,teamtailor")
	require.NoError(t, err)
	assert.Equal(t, []string{"teamtailor", "workday"}, got)
}

func TestBuildTeamtailorAdapter(t *testing.T) {
	adapters, err := buildAdapters([]string{"teamtailor"})
	require.NoError(t, err)
	require.Len(t, adapters, 1)
	assert.Equal(t, "teamtailor", adapters[0].Name())
}
