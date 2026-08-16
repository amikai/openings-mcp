package delta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAreaList(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetAreaList(t.Context(), GetAreaListParams{
		Lang: "en-US",
	})
	require.NoError(t, err)
	require.NotEmpty(t, res)

	first := res[0]
	assert.Equal(t, "A", first.Value)
	assert.Equal(t, "TW", first.Text)
}

func TestSearchJobList(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	// 1. Unfiltered search
	res, err := client.SearchJobList(t.Context(), SearchJobListParams{
		AreaID:     "",
		AddJobName: "",
		Lang:       "en-US",
	})
	require.NoError(t, err)
	require.NotEmpty(t, res)

	first := res[0]
	assert.Equal(t, "C20260814001", first.EmpAddID)
	assert.Equal(t, "PT4B001", first.JobCode)
	assert.Equal(t, "自動化資深工程師", first.JobName)

	// 2. Filtered by area
	areaRes, err := client.SearchJobList(t.Context(), SearchJobListParams{
		AreaID:     "A",
		AddJobName: "",
		Lang:       "zh-TW",
	})
	require.NoError(t, err)
	require.NotEmpty(t, areaRes)
	assert.Equal(t, "台灣", areaRes[0].AreaName.Value)

	// 3. Filtered by keyword
	kwRes, err := client.SearchJobList(t.Context(), SearchJobListParams{
		AreaID:     "",
		AddJobName: "軟體",
		Lang:       "zh-TW",
	})
	require.NoError(t, err)
	require.NotEmpty(t, kwRes)
	assert.Contains(t, kwRes[0].JobName, "軟體")

	// 4. Empty search results
	emptyRes, err := client.SearchJobList(t.Context(), SearchJobListParams{
		AreaID:     "",
		AddJobName: "NONEXISTENT_KEYWORD_XYZ",
		Lang:       "en-US",
	})
	require.NoError(t, err)
	assert.Empty(t, emptyRes)
}

func TestGetJobDetails(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	// 1. Existing job detail
	res, err := client.GetJobDetails(t.Context(), GetJobDetailsParams{
		EmpAddID: "C20260814001",
		Resumeid: "",
		Lang:     "en-US",
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.JobDetails)

	detail := res.JobDetails[0]
	assert.Equal(t, "C20260814001", detail.EmpAddID)
	assert.Equal(t, "PT4B001", detail.JobCode)
	assert.Equal(t, "自動化資深工程師", detail.JobName)
	assert.Contains(t, detail.JobResponsibility.Value, "自动化控制系统设计")

	// 2. Non-existent job detail
	notFoundRes, err := client.GetJobDetails(t.Context(), GetJobDetailsParams{
		EmpAddID: "INVALID123",
		Resumeid: "",
		Lang:     "en-US",
	})
	require.NoError(t, err)
	assert.Empty(t, notFoundRes.JobDetails)
}
