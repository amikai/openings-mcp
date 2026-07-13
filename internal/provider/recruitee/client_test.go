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
	assert.Equal(t, 2675345, offer.ID)
	assert.Equal(t, recruitee.NewNilString("Fraud Ops Expert"), offer.Title)
	assert.Equal(t, recruitee.NewOptNilString("Support & Operations"), offer.Department)
	assert.Equal(t, recruitee.NewNilString("https://careers.bunq.com/o/fraud-ops-expert"), offer.CareersURL)
	assert.Equal(t, recruitee.NewOptNilString("Sofia, Sofia (stolitsa), Bulgaria"), offer.Location)
	assert.Equal(t, recruitee.NewOptNilString("2026-07-13 13:42:26 UTC"), offer.PublishedAt)
	assert.Equal(t, recruitee.NewOptNilString("banking"), offer.CategoryCode)
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
	assert.Equal(t, 12345, offer.ID)
	assert.Equal(t, recruitee.OptNilString{Null: true, Set: true}, offer.Description)
	assert.Equal(t, recruitee.OptNilString{Null: true, Set: true}, offer.Location)
	assert.Equal(t, recruitee.OptNilSalary{Set: true, Null: true}, offer.Salary)
	assert.Empty(t, offer.Locations)
}
