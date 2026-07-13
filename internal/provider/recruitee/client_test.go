package recruitee_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/recruitee"
)

func TestGetOffers(t *testing.T) {
	srv := recruitee.NewMockServer()
	defer srv.Close()

	client, err := recruitee.NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetOffers(t.Context())
	require.NoError(t, err)

	feed, ok := res.(*recruitee.OffersResponse)
	require.True(t, ok, "want *OffersResponse, got %T", res)
	require.NotEmpty(t, feed.Offers)

	// Verify first item is parsed correctly
	offer := feed.Offers[0]

	wantID := 2675345
	assert.Equal(t, wantID, offer.ID)

	wantTitle := recruitee.NewNilString("Fraud Ops Expert")
	assert.Equal(t, wantTitle, offer.Title)

	wantDepartment := recruitee.NewOptNilString("Support & Operations")
	assert.Equal(t, wantDepartment, offer.Department)

	wantCareersURL := recruitee.NewNilString("https://careers.bunq.com/o/fraud-ops-expert")
	assert.Equal(t, wantCareersURL, offer.CareersURL)

	wantLocation := recruitee.NewOptNilString("Sofia, Sofia (stolitsa), Bulgaria")
	assert.Equal(t, wantLocation, offer.Location)

	wantPublishedAt := recruitee.NewOptNilString("2026-07-13 13:42:26 UTC")
	assert.Equal(t, wantPublishedAt, offer.PublishedAt)

	wantCategoryCode := recruitee.NewOptNilString("banking")
	assert.Equal(t, wantCategoryCode, offer.CategoryCode)

	assert.Contains(t, offer.Tags, "Promise Keeper")
}

func TestGetOffersNotFound(t *testing.T) {
	srv := recruitee.NewMockServer()
	defer srv.Close()

	client, err := recruitee.NewClient(srv.URL + "/missing")
	require.NoError(t, err)

	res, err := client.GetOffers(t.Context())
	require.NoError(t, err)
	_, ok := res.(*recruitee.GetOffersNotFound)
	assert.True(t, ok, "want *GetOffersNotFound, got %T", res)
}

func TestGetOffersNullFields(t *testing.T) {
	srv := recruitee.NewNullMockServer()
	defer srv.Close()

	client, err := recruitee.NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetOffers(t.Context())
	require.NoError(t, err)

	feed, ok := res.(*recruitee.OffersResponse)
	require.True(t, ok, "want *OffersResponse, got %T", res)
	require.Len(t, feed.Offers, 1)

	offer := feed.Offers[0]

	wantID := 12345
	assert.Equal(t, wantID, offer.ID)

	wantDescription := recruitee.OptNilString{Null: true, Set: true}
	assert.Equal(t, wantDescription, offer.Description)

	wantLocation := recruitee.OptNilString{Null: true, Set: true}
	assert.Equal(t, wantLocation, offer.Location)

	wantSalary := recruitee.OptNilSalary{Set: true, Null: true}
	assert.Equal(t, wantSalary, offer.Salary)

	assert.Empty(t, offer.Locations)
}
