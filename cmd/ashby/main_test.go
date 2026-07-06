package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunSearchMissingBoard(t *testing.T) {
	err := runSearch(context.Background(), "", time.Second, "", "text")
	assert.ErrorContains(t, err, "--board is required")
}

func TestRunSearchUnknownBoard(t *testing.T) {
	err := runSearch(context.Background(), "doesnotexist-board-xyz", time.Second, "", "text")
	assert.ErrorContains(t, err, `board "doesnotexist-board-xyz" not found`)
	assert.ErrorContains(t, err, "ashby companies")
}

func TestRunGetMissingID(t *testing.T) {
	err := runGet(context.Background(), "openai", time.Second, "", "text")
	assert.ErrorContains(t, err, "--id is required")
}

func TestRunGetMissingBoard(t *testing.T) {
	err := runGet(context.Background(), "", time.Second, "some-id", "text")
	assert.ErrorContains(t, err, "--board is required")
}

func TestRunGetUnknownBoard(t *testing.T) {
	err := runGet(context.Background(), "doesnotexist-board-xyz", time.Second, "some-id", "text")
	assert.ErrorContains(t, err, `board "doesnotexist-board-xyz" not found`)
	assert.ErrorContains(t, err, "ashby companies")
}
