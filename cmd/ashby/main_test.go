package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSearchMissingBoard(t *testing.T) {
	err := runSearch(t.Context(), searchFlags{timeout: time.Second, format: "text"})
	assert.ErrorContains(t, err, "--board is required")
}

func TestRunSearchUnknownBoard(t *testing.T) {
	err := runSearch(t.Context(), searchFlags{board: "doesnotexist-board-xyz", timeout: time.Second, format: "text"})
	require.ErrorContains(t, err, `board "doesnotexist-board-xyz" not found`)
	assert.ErrorContains(t, err, "ashby companies")
}

func TestRunGetMissingID(t *testing.T) {
	err := runGet(t.Context(), getFlags{board: "openai", timeout: time.Second, format: "text"})
	assert.ErrorContains(t, err, "--id is required")
}

func TestRunGetMissingBoard(t *testing.T) {
	err := runGet(t.Context(), getFlags{timeout: time.Second, jobID: "some-id", format: "text"})
	assert.ErrorContains(t, err, "--board is required")
}

func TestRunGetUnknownBoard(t *testing.T) {
	err := runGet(t.Context(), getFlags{board: "doesnotexist-board-xyz", timeout: time.Second, jobID: "some-id", format: "text"})
	require.ErrorContains(t, err, `board "doesnotexist-board-xyz" not found`)
	assert.ErrorContains(t, err, "ashby companies")
}
