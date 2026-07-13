package teamtailor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetJobs(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobs(t.Context())
	require.NoError(t, err)

	feed, ok := res.(*CareerFeed)
	require.True(t, ok, "want *CareerFeed, got %T", res)
	assert.Equal(t, "https://jsonfeed.org/version/1.1", feed.Version)
	assert.Equal(t, "Knauf Belgium", feed.Title)
	assert.Equal(t, "https://knaufsemea.teamtailor.com/jobs", feed.HomePageURL)
	assert.Equal(t, "https://knaufsemea.teamtailor.com/jobs.json", feed.FeedURL)
	require.Len(t, feed.Items, 3)

	job := feed.Items[0]
	assert.Equal(t, "a97df59d-7a99-4387-8956-8e032e8bf793", job.ID.String())
	assert.Equal(t, "Electromécanicien de maintenance (H/F/X)", job.Title)
	assert.Equal(t, "https://knaufsemea.teamtailor.com/jobs/7890179-electromecanicien-de-maintenance-h-f-x", job.URL)
	assert.True(t, job.DatePublished.Equal(time.Date(2026, 6, 11, 6, 19, 40, 0, time.UTC)))
	assert.NotEmpty(t, job.ContentHTML)

	require.Len(t, job.Jobposting.JobLocation, 1)
	address := job.Jobposting.JobLocation[0].Address
	assert.Equal(t, NewNilString("Rue du Parc Industriel 1"), address.StreetAddress)
	assert.Equal(t, "Engis", address.AddressLocality)
	assert.Equal(t, NewNilString("4480"), address.PostalCode)
	assert.Equal(t, "BE", address.AddressCountry)
	assert.Equal(t, NewNilString("WE&I"), address.AddressRegion)
}

func TestGetJobsNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL + "/missing")
	require.NoError(t, err)

	res, err := client.GetJobs(t.Context())
	require.NoError(t, err)
	_, ok := res.(*GetJobsNotFound)
	assert.True(t, ok, "want *GetJobsNotFound, got %T", res)
}

func TestGetJobsNullAddressFields(t *testing.T) {
	srv := NewNullMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	res, err := client.GetJobs(t.Context())
	require.NoError(t, err)

	feed, ok := res.(*CareerFeed)
	require.True(t, ok, "want *CareerFeed, got %T", res)
	require.Len(t, feed.Items, 1)
	require.Len(t, feed.Items[0].Jobposting.JobLocation, 1)
	address := feed.Items[0].Jobposting.JobLocation[0].Address
	assert.Equal(t, NilString{Null: true}, address.AddressRegion)
}
