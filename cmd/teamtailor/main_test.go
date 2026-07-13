package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeHost(t *testing.T) {
	host, err := normalizeHost("CAREER.TEAMTAILOR.COM")
	require.NoError(t, err)
	assert.Equal(t, "career.teamtailor.com", host)
}

func TestRunSearchMissingHost(t *testing.T) {
	err := runSearch(t.Context(), "", time.Second, "", "text")
	assert.ErrorContains(t, err, "--host is required")
}

func TestRunSearchUnknownHost(t *testing.T) {
	err := runSearch(t.Context(), "does-not-exist.teamtailor.com", time.Second, "", "text")
	assert.ErrorContains(t, err, "teamtailor companies")
}

func TestRunGetMissingID(t *testing.T) {
	err := runGet(t.Context(), "career.teamtailor.com", time.Second, "", "text")
	assert.ErrorContains(t, err, "--id is required")
}
