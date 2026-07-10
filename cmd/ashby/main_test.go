package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunSearchMissingBoard(t *testing.T) {
	err := runSearch(t.Context(), "", time.Second, "", "text")
	assert.ErrorContains(t, err, "--board is required")
}

func TestRunSearchUnknownBoard(t *testing.T) {
	err := runSearch(t.Context(), "doesnotexist-board-xyz", time.Second, "", "text")
	assert.ErrorContains(t, err, `board "doesnotexist-board-xyz" not found`)
	assert.ErrorContains(t, err, "ashby companies")
}

func TestRunGetMissingID(t *testing.T) {
	err := runGet(t.Context(), "openai", time.Second, "", "text")
	assert.ErrorContains(t, err, "--id is required")
}

func TestRunGetMissingBoard(t *testing.T) {
	err := runGet(t.Context(), "", time.Second, "some-id", "text")
	assert.ErrorContains(t, err, "--board is required")
}

func TestRunGetUnknownBoard(t *testing.T) {
	err := runGet(t.Context(), "doesnotexist-board-xyz", time.Second, "some-id", "text")
	assert.ErrorContains(t, err, `board "doesnotexist-board-xyz" not found`)
	assert.ErrorContains(t, err, "ashby companies")
}
