package teamtailor

import (
	"testing"
	"time"

	"github.com/google/uuid"
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
	require.Len(t, feed.Items, 3)

	wantFeed := &CareerFeed{
		Version:     "https://jsonfeed.org/version/1.1",
		Title:       "Knauf Belgium",
		HomePageURL: "https://knaufsemea.teamtailor.com/jobs",
		FeedURL:     "https://knaufsemea.teamtailor.com/jobs.json",
	}
	gotFeed := &CareerFeed{
		Version:     feed.Version,
		Title:       feed.Title,
		HomePageURL: feed.HomePageURL,
		FeedURL:     feed.FeedURL,
	}
	assert.Equal(t, wantFeed, gotFeed)

	job := feed.Items[0]
	assert.NotEmpty(t, job.ContentHTML)
	assert.True(t, job.DatePublished.Equal(time.Date(2026, 6, 11, 6, 19, 40, 0, time.UTC)))

	want := CareerItem{
		ID:            uuid.MustParse("a97df59d-7a99-4387-8956-8e032e8bf793"),
		Title:         "Electromécanicien de maintenance (H/F/X)",
		URL:           "https://knaufsemea.teamtailor.com/jobs/7890179-electromecanicien-de-maintenance-h-f-x",
		DatePublished: job.DatePublished,
		ContentHTML:   job.ContentHTML,
		Jobposting: JobPosting{
			JobLocation: []Place{
				{
					Address: PostalAddress{
						StreetAddress:   NewNilString("Rue du Parc Industriel 1"),
						AddressLocality: "Engis",
						PostalCode:      NewNilString("4480"),
						AddressCountry:  "BE",
						AddressRegion:   NewNilString("WE&I"),
					},
				},
			},
		},
	}
	assert.Equal(t, want, job)
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
