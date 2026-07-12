package smartrecruiters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPostings(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.ListPostings(t.Context(), ListPostingsParams{CompanyIdentifier: "equinox"})
	require.NoError(t, err)

	assert.Equal(t, 0, res.Offset)
	assert.Equal(t, 5, res.Limit)
	assert.Equal(t, 662, res.TotalFound)
	require.Len(t, res.Content, 5)

	first := res.Content[0]
	assert.Equal(t, NewOptString("744000137225639"), first.ID)
	assert.Equal(t, NewOptNilString("Female Locker Room Associate, Houston"), first.Name)
	assert.Equal(t, NewOptNilString("REF3410P"), first.RefNumber)
	assert.Equal(t, NewOptCompany(Company{
		Identifier: NewOptNilString("Equinox"),
		Name:       NewOptNilString("Equinox"),
	}), first.Company)
	assert.Equal(t, NewOptLocation(Location{
		City:         NewOptNilString("Houston"),
		Region:       NewOptNilString("TX"),
		Country:      NewOptNilString("us"),
		Remote:       NewOptBool(false),
		Hybrid:       NewOptBool(false),
		Latitude:     NewOptNilString("29.7604267"),
		Longitude:    NewOptNilString("-95.3698028"),
		FullLocation: NewOptNilString("Houston, TX, United States"),
	}), first.Location)
	assert.Equal(t, NewOptIdLabel(IdLabel{
		ID:    NewOptNilString("health_wellness_fitness"),
		Label: NewOptNilString("Health, Wellness And Fitness"),
	}), first.Industry)

	// department.id is a quoted string on the list endpoint — the opposite
	// shape from the same posting's detail response (see TestGetPosting).
	dep, ok := first.Department.Get()
	require.True(t, ok)
	depID, ok := dep.ID.Get()
	require.True(t, ok)
	s, isStr := depID.GetString()
	assert.True(t, isStr, "want department.id to decode as string on the list endpoint")
	assert.Equal(t, "660916", s)
	assert.Equal(t, NewOptNilString("Club - Staff"), dep.Label)

	assert.Equal(t, NewOptIdLabel(IdLabel{ID: NewOptNilString("other"), Label: NewOptNilString("Other")}), first.Function)
	assert.Equal(t, NewOptIdLabel(IdLabel{ID: NewOptNilString("part-time"), Label: NewOptNilString("Part-time")}), first.TypeOfEmployment)
	assert.Equal(t, NewOptIdLabel(IdLabel{ID: NewOptNilString("not_applicable"), Label: NewOptNilString("Not Applicable")}), first.ExperienceLevel)
	require.Len(t, first.CustomField, 5)
	assert.Equal(t, CustomField{
		FieldId:    NewOptNilString("58b7e4d3e4b09a6d37a0cdc3"),
		FieldLabel: NewOptNilString("Department"),
		ValueId:    NewOptNilString("660916"),
		ValueLabel: NewOptNilString("Club - Staff"),
	}, first.CustomField[3])
	assert.Equal(t, NewOptNilString("PUBLIC"), first.Visibility)
	assert.Equal(t, NewOptNilString("https://api.smartrecruiters.com/v1/companies/equinox/postings/744000137225639"), first.Ref)
	assert.Equal(t, NewOptLanguage(Language{
		Code:        NewOptNilString("en"),
		Label:       NewOptNilString("English"),
		LabelNative: NewOptNilString("English (US)"),
	}), first.Language)
}

// TestListPostingsFiltered proves q= is modeled as a real server-side
// filter rather than an ignored parameter: the fixture's totalFound narrows
// from 662 to 138, and every returned title contains "trainer".
func TestListPostingsFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.ListPostings(t.Context(), ListPostingsParams{
		CompanyIdentifier: "equinox",
		Q:                 NewOptString("trainer"),
	})
	require.NoError(t, err)

	assert.Equal(t, 138, res.TotalFound)
	require.Len(t, res.Content, 3)
	for _, p := range res.Content {
		name, ok := p.Name.Get()
		require.True(t, ok)
		assert.Contains(t, name, "Trainer")
	}
}

// TestListPostingsUnknownCompany guards the no-404 quirk: an unrecognized
// companyIdentifier is HTTP 200 with the same empty shape a real company
// with zero open postings would return, not an error.
func TestListPostingsUnknownCompany(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.ListPostings(t.Context(), ListPostingsParams{CompanyIdentifier: MockUnknownCompany})
	require.NoError(t, err)

	assert.Equal(t, 0, res.TotalFound)
	assert.Empty(t, res.Content)
}

func TestGetPosting(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetPosting(t.Context(), GetPostingParams{
		CompanyIdentifier: "equinox",
		PostingId:         "744000137225639",
	})
	require.NoError(t, err)

	got, ok := res.(*PostingDetail)
	require.True(t, ok, "want *PostingDetail, got %T", res)

	assert.Equal(t, NewOptString("744000137225639"), got.ID)
	assert.Equal(t, NewOptNilString("Female Locker Room Associate, Houston"), got.Name)
	assert.True(t, got.Active.Value)
	assert.Equal(t, NewOptNilString("PUBLIC"), got.Visibility)
	assert.Equal(t, NewOptNilString(
		"https://jobs.smartrecruiters.com/Equinox/744000137225639-female-locker-room-associate-houston",
	), got.PostingUrl)
	assert.Equal(t, NewOptNilString(
		"https://jobs.smartrecruiters.com/Equinox/744000137225639-female-locker-room-associate-houston?oga=true",
	), got.ApplyUrl)
	require.Len(t, got.CustomField, 5)

	// department.id is an unquoted integer on the detail endpoint — the
	// opposite shape from the same posting's list response (TestListPostings).
	dep, ok := got.Department.Get()
	require.True(t, ok)
	depID, ok := dep.ID.Get()
	require.True(t, ok)
	i, isInt := depID.GetInt()
	assert.True(t, isInt, "want department.id to decode as int on the detail endpoint")
	assert.Equal(t, 660916, i)
	assert.Equal(t, NewOptNilString("Club - Staff"), dep.Label)

	sections, ok := got.JobAd.Value.Sections.Get()
	require.True(t, ok)

	companyDesc, ok := sections.CompanyDescription.Get()
	require.True(t, ok)
	assert.Equal(t, NewOptNilString("Company Description"), companyDesc.Title)
	assert.Contains(t, companyDesc.Text.Value, "Equinox Group is a high growth collective")

	jobDesc, ok := sections.JobDescription.Get()
	require.True(t, ok)
	assert.Contains(t, jobDesc.Text.Value, "Female Locker Room Associates")

	qualifications, ok := sections.Qualifications.Get()
	require.True(t, ok)
	assert.Contains(t, qualifications.Text.Value, "clean and sanitary environment")

	additional, ok := sections.AdditionalInformation.Get()
	require.True(t, ok)
	assert.Contains(t, additional.Text.Value, "AS A MEMBER OF THE EQUINOX TEAM")
}

func TestGetPostingNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetPosting(t.Context(), GetPostingParams{
		CompanyIdentifier: "equinox",
		PostingId:         "000000000000",
	})
	require.NoError(t, err)

	got, ok := res.(*PostingErrorResponse)
	require.True(t, ok, "want *PostingErrorResponse, got %T", res)

	assert.Equal(t, NewOptNilInt(404), got.HttpCode)
	assert.Equal(t, NewOptNilString("RESOURCE_NOT_FOUND"), got.Code)
}
