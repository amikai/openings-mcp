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
	err := runSearch(t.Context(), searchFlags{timeout: time.Second, format: "text"})
	assert.ErrorContains(t, err, "--host is required")
}

func TestRunSearchUnknownHost(t *testing.T) {
	err := runSearch(t.Context(), searchFlags{host: "does-not-exist.teamtailor.com", timeout: time.Second, format: "text"})
	assert.ErrorContains(t, err, "teamtailor companies")
}

func TestRunGetMissingID(t *testing.T) {
	err := runGet(t.Context(), getFlags{host: "career.teamtailor.com", timeout: time.Second, format: "text"})
	assert.ErrorContains(t, err, "--id is required")
}
