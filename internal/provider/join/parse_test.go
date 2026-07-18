package join

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCompany(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.ResolveCompany(t.Context(), MockJobSlug)
	require.NoError(t, err)
	assert.Equal(t, MockCompanyID, got.ID)
	assert.Equal(t, "Routine Labs", got.Name)
	assert.Equal(t, MockJobSlug, got.Slug)
}

func TestResolveCompanyNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.ResolveCompany(t.Context(), "nonexistent-company-xyz")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestBuildDescriptionLegacy(t *testing.T) {
	j := &nextDataJob{
		Intro:        "Intro text.",
		Tasks:        "Do things.",
		Requirements: "Know things.",
		Benefits:     "",
		Outro:        "",
	}
	got := buildDescription(j)
	assert.Contains(t, got, "Intro text.")
	assert.Contains(t, got, "## Tasks\n\nDo things.")
	assert.Contains(t, got, "## Skills\n\nKnow things.")
	assert.NotContains(t, got, "## Benefits")
}

func TestBuildDescriptionUnified(t *testing.T) {
	j := &nextDataJob{
		UnifiedDescription: true,
		Description:        "Whole body in one field.",
		Intro:              "should be ignored",
	}
	assert.Equal(t, "Whole body in one field.", buildDescription(j))
}
