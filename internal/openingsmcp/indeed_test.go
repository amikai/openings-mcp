package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/indeed"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndeedMCPToHTTPRequest(t *testing.T) {
	radius := 50
	in := indeedSearchInput{
		Keyword:     "software engineer",
		Location:    "Taipei",
		Country:     "Taiwan",
		RadiusMiles: &radius,
		Cursor:      "cur1",
		JobType:     "Full-time",
		Remote:      true,
	}
	got, err := indeedMCPToHTTPRequest(&in)
	require.NoError(t, err)

	want := &indeed.JobsRequest{
		Keywords:    "software engineer",
		Location:    "Taipei",
		Country:     "Taiwan",
		RadiusMiles: &radius,
		Cursor:      "cur1",
		JobType:     indeed.JobTypeFullTime,
		Remote:      true,
	}
	assert.Equal(t, want, got)
}

func TestIndeedMCPToHTTPRequestMinimal(t *testing.T) {
	got, err := indeedMCPToHTTPRequest(&indeedSearchInput{})
	require.NoError(t, err)
	assert.Equal(t, &indeed.JobsRequest{}, got)
}

func TestIndeedMCPToHTTPRequestZeroRadius(t *testing.T) {
	zero := 0
	got, err := indeedMCPToHTTPRequest(&indeedSearchInput{RadiusMiles: &zero})
	require.NoError(t, err)
	require.NotNil(t, got.RadiusMiles)
	assert.Equal(t, 0, *got.RadiusMiles)
}

func TestIndeedMCPToHTTPRequestInvalidJobType(t *testing.T) {
	_, err := indeedMCPToHTTPRequest(&indeedSearchInput{JobType: "Volunteer"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid job_type "Volunteer"`)
}

func TestIndeedHTTPToMCPResponse(t *testing.T) {
	in := indeed.JobsResponse{
		Jobs: []indeed.Job{
			{Key: "1", Title: "t1", Company: "c1", CompanyURL: "cu1", Location: "l1", Country: "Taiwan", JobURL: "u1", PostedDate: "d1", JobTypes: []string{"Full-time"}, Compensation: &indeed.Compensation{MinAmount: 22.5, MaxAmount: 27.5, Currency: "USD", Interval: "HOUR"}},
			{Key: "2", Title: "t2"},
		},
		NextCursor: "next1",
	}
	got := indeedHTTPToMCPResponse(&in)

	want := &indeedSearchOutput{
		Data: []indeedJobSummary{
			{Key: "1", Title: "t1", Company: "c1", CompanyURL: "cu1", Location: "l1", Country: "Taiwan", URL: "u1", PostedDate: "d1", JobTypes: []string{"Full-time"}, Compensation: &indeedCompensation{MinAmount: 22.5, MaxAmount: 27.5, Currency: "USD", Interval: "HOUR"}},
			{Key: "2", Title: "t2"},
		},
		NextCursor: "next1",
	}
	assert.Equal(t, want, got)
}

func TestIndeedHTTPToMCPDetail(t *testing.T) {
	in := indeed.JobDetail{
		Key:                "7",
		Title:              "t",
		Company:            "c",
		CompanyURL:         "cu",
		Location:           indeed.Location{Country: "Taiwan", CountryCode: "TW", State: "TPE", City: "Taipei", Formatted: "Taipei, Taiwan"},
		JobURL:             "u",
		PostedDate:         "d",
		Description:        "desc",
		JobTypes:           []string{"Full-time"},
		Compensation:       &indeed.Compensation{MinAmount: 1, MaxAmount: 2, Currency: "USD", Interval: "YEAR"},
		Source:             "src",
		DateIndexed:        "d2",
		CompanyWebsite:     "web",
		CompanyIndustry:    "ind",
		CompanyEmployees:   "10,000+",
		CompanyRevenue:     "rev",
		CompanyDescription: "cdesc",
		CompanyLogo:        "logo-url",
		CompanyAddresses:   []string{"addr1"},
		CompanyCEO:         "ceo",
		CompanyCEOPhoto:    "ceo-photo",
		CompanyBannerImage: "banner-url",
		ApplyURL:           "apply-url",
		DetailedSalary:     "detailed-salary",
		WorkSchedule:       "schedule",
	}
	got := indeedHTTPToMCPDetail(&in)

	want := &indeedDetailOutput{
		Key:                "7",
		URL:                "u",
		Title:              "t",
		Company:            "c",
		CompanyURL:         "cu",
		Location:           &indeedLocation{Country: "Taiwan", CountryCode: "TW", State: "TPE", City: "Taipei", Formatted: "Taipei, Taiwan"},
		PostedDate:         "d",
		Description:        "desc",
		JobTypes:           []string{"Full-time"},
		Compensation:       &indeedCompensation{MinAmount: 1, MaxAmount: 2, Currency: "USD", Interval: "YEAR"},
		Source:             "src",
		DateIndexed:        "d2",
		CompanyWebsite:     "web",
		CompanyIndustry:    "ind",
		CompanyEmployees:   "10,000+",
		CompanyRevenue:     "rev",
		CompanyDescription: "cdesc",
		CompanyLogo:        "logo-url",
		CompanyAddresses:   []string{"addr1"},
		CompanyCEO:         "ceo",
		CompanyCEOPhoto:    "ceo-photo",
		CompanyBannerImage: "banner-url",
		ApplyURL:           "apply-url",
		DetailedSalary:     "detailed-salary",
		WorkSchedule:       "schedule",
	}
	assert.Equal(t, want, got)
}

func TestIndeedLocationFromHTTPZeroValue(t *testing.T) {
	assert.Nil(t, indeedLocationFromHTTP(indeed.Location{}))
}

func testIndeedMCPClientServer(t *testing.T) (*mcp.ClientSession, *mcp.ServerSession) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := indeed.NewMockServer()
	t.Cleanup(srv.Close)
	client := indeed.NewClient(srv.URL+"/graphql", srv.Client())
	RegisterIndeed(server, client)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		serverSession.Close()
	})

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := mcpClient.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		clientSession.Close()
	})
	return clientSession, serverSession
}

func TestRegisterIndeed(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client := indeed.NewClient("https://apis.indeed.com/graphql", nil)
	RegisterIndeed(server, client)

	assertTools(t, server, "indeed_search_jobs", "indeed_get_job_detail")
}

func TestIndeedSearchJobsE2E(t *testing.T) {
	clientSession, _ := testIndeedMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "indeed_search_jobs",
		Arguments: map[string]any{
			"keyword":  "software engineer",
			"location": "Taipei",
			"country":  "Taiwan",
		},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got indeedSearchOutput
	require.NoError(t, json.Unmarshal(data, &got))

	require.Len(t, got.Data, 5)
	first := got.Data[0]
	assert.Equal(t, "9d503ca7fe211430", first.Key)
	assert.Equal(t, "Senior Staff Engineer System Application Engineering", first.Title)
	assert.Equal(t, "Infineon Technologies", first.Company)
	assert.Equal(t, "Taiwan", first.Country)
	assert.Equal(t, "https://tw.indeed.com/viewjob?jk=9d503ca7fe211430", first.URL)
	assert.NotEmpty(t, got.NextCursor)

	// Fractional Range + one-sided variants must survive end-to-end.
	assert.Equal(t, 22.5, got.Data[1].Compensation.MinAmount)
	assert.Equal(t, 27.5, got.Data[1].Compensation.MaxAmount)
	assert.Equal(t, 15.0, got.Data[2].Compensation.MinAmount)
	assert.Equal(t, 17.5, got.Data[3].Compensation.MinAmount)
	assert.Equal(t, 17.5, got.Data[3].Compensation.MaxAmount)
	assert.Equal(t, 30.0, got.Data[4].Compensation.MaxAmount)
}

func TestIndeedSearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession, _ := testIndeedMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "indeed_search_jobs",
		Arguments: map[string]any{"job_type": "valueNotInEnum"},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, `validating /properties/job_type: enum`)
}

func TestIndeedGetJobDetailE2E(t *testing.T) {
	clientSession, _ := testIndeedMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "indeed_get_job_detail",
		Arguments: map[string]any{"job_key": "9d503ca7fe211430", "country": "Taiwan"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got indeedDetailOutput
	require.NoError(t, json.Unmarshal(data, &got))

	description := got.Description
	got.Description = ""

	want := indeedDetailOutput{
		Key:                "9d503ca7fe211430",
		URL:                "https://tw.indeed.com/viewjob?jk=9d503ca7fe211430",
		Title:              "Senior Staff Engineer System Application Engineering",
		Company:            "Infineon Technologies",
		CompanyURL:         "https://tw.indeed.com/cmp/Infineon-Technologies",
		Location:           &indeedLocation{Country: "台灣", CountryCode: "TW", State: "TPE", City: "台北市", Formatted: "台北市"},
		PostedDate:         "2026-06-04",
		JobTypes:           []string{"Permanent", "Full-time"},
		Source:             "Infineon Technologies",
		DateIndexed:        "2026-07-14",
		CompanyWebsite:     "https://www.infineon.com/",
		CompanyEmployees:   "10,000+",
		CompanyRevenue:     "more than $10B (USD)",
		CompanyDescription: "Infineon designs, develops, manufactures, and markets a broad range of semiconductors and semiconductor-based solutions, focusing on key markets in the automotive, industrial, and consumer sectors.",
		CompanyLogo:        "https://d2q79iu7y748jz.cloudfront.net/s/_squarelogo/256x256/d6d998121efc1f38f34bb1678e486bcb",
		CompanyAddresses:   []string{"Neubiberg"},
		CompanyCEO:         "Jochen Hanebeck",
		CompanyCEOPhoto:    "https://d2q79iu7y748jz.cloudfront.net/s/_ceophoto/512x512/e164f653df3541221b0c25a3f6610e5c",
		CompanyBannerImage: "https://d2q79iu7y748jz.cloudfront.net/s/_headerimage/1960x400/e9a83e0c63c4266c44c12762aed91d6c",
		ApplyURL:           "https://jobs.infineon.com/careers/job/563808971122356?utm_source=indeed&domain=infineon.com",
	}
	assert.Equal(t, want, got)
	assert.Contains(t, description, "Your Role")
}

func TestIndeedGetJobDetailRequiresCountryE2E(t *testing.T) {
	clientSession, _ := testIndeedMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "indeed_get_job_detail",
		Arguments: map[string]any{"job_key": "9d503ca7fe211430"},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
}

func TestIndeedGetJobDetailNotFoundE2E(t *testing.T) {
	clientSession, _ := testIndeedMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "indeed_get_job_detail",
		Arguments: map[string]any{"job_key": indeed.MockNotFoundJobKey, "country": "Taiwan"},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
}
