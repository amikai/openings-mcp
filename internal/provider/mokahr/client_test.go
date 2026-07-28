package mokahr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T) *JobsClient {
	t.Helper()
	srv := NewMockServer()
	t.Cleanup(srv.Close)
	client, err := NewJobsClient(srv.URL, nil)
	require.NoError(t, err)
	return client
}

func TestListJobs(t *testing.T) {
	client := testClient(t)

	list, err := client.ListJobs(t.Context(), ListJobsRequest{
		OrgId:    MockOrgID,
		SiteId:   MockSiteID,
		Limit:    NewOptInt(5),
		Offset:   NewOptInt(0),
		NeedStat: NewOptBool(true),
	})
	require.NoError(t, err)

	stats, ok := list.JobStats.Get()
	require.True(t, ok)
	assert.Equal(t, NewOptInt(35), stats.Total)
	assert.Equal(t, NewOptString(MockOrgID), stats.OrgId)
	require.Len(t, list.Jobs, 5)

	first := list.Jobs[0]
	assert.Equal(t, MockJobID, first.ID)
	assert.Equal(t, "多模态理解（数据/算法）研究员", first.Title)
	assert.Equal(t, NewOptString("open"), first.Status)
	assert.Equal(t, NewOptString("2026-06-25T00:00"), first.OpenedAt)
	assert.Contains(t, first.JobDescription.Or(""), "<p><strong>【团队使命】</strong></p>")

	zhineng, ok := first.Zhineng.Get()
	require.True(t, ok)
	assert.Equal(t, NewOptString("深度学习研究员"), zhineng.Name)

	// locale defaults to en-US, so place names arrive romanized.
	firstLocations := first.Locations.Or(nil)
	require.Len(t, firstLocations, 2)
	assert.Equal(t, NewOptString("Gongshu"), firstLocations[0].CityName)
	assert.Equal(t, NewOptString("Zhejiang"), firstLocations[0].ProvinceName)
	assert.Equal(t, NewOptString("China"), firstLocations[0].Country)
	assert.Equal(t, NewOptInt(31373), firstLocations[0].ID)
}

func TestListJobsPaginates(t *testing.T) {
	client := testClient(t)

	page2, err := client.ListJobs(t.Context(), ListJobsRequest{
		OrgId:    MockOrgID,
		SiteId:   MockSiteID,
		Limit:    NewOptInt(5),
		Offset:   NewOptInt(5),
		NeedStat: NewOptBool(true),
	})
	require.NoError(t, err)

	// The total is the whole board, not the page.
	stats, ok := page2.JobStats.Get()
	require.True(t, ok)
	assert.Equal(t, NewOptInt(35), stats.Total)
	require.Len(t, page2.Jobs, 5)
	assert.Equal(t, "法务团队", page2.Jobs[0].Title)
	assert.NotEqual(t, MockJobID, page2.Jobs[0].ID)
}

func TestListJobsKeywordMatchesTitlesOnly(t *testing.T) {
	client := testClient(t)

	list, err := client.ListJobs(t.Context(), ListJobsRequest{
		OrgId:    MockOrgID,
		SiteId:   MockSiteID,
		Limit:    NewOptInt(5),
		NeedStat: NewOptBool(true),
		Keyword:  NewOptString(MockKeyword),
	})
	require.NoError(t, err)

	stats, ok := list.JobStats.Get()
	require.True(t, ok)
	assert.Equal(t, NewOptInt(1), stats.Total)
	require.Len(t, list.Jobs, 1)
	assert.Equal(t, "AI平台运维工程师", list.Jobs[0].Title)
}

func TestListJobsUnknownSite(t *testing.T) {
	client := testClient(t)

	_, err := client.ListJobs(t.Context(), ListJobsRequest{
		OrgId:  MockUnknownOrgID,
		SiteId: "1",
		Limit:  NewOptInt(5),
	})

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, CodeSiteNotFound, apiErr.Code)
	assert.Equal(t, "未找到对应的官网", apiErr.Msg)
}

func TestGetJob(t *testing.T) {
	client := testClient(t)

	job, err := client.GetJob(t.Context(), GetJobRequest{
		OrgId:  MockOrgID,
		SiteId: MockSiteID,
		JobId:  MockJobID,
	})
	require.NoError(t, err)

	assert.Equal(t, MockJobID, job.ID)
	assert.Equal(t, "多模态理解（数据/算法）研究员", job.Title)
	// The fields that only the detail endpoint carries.
	assert.Equal(t, NewOptString("全职"), job.Commitment)
	assert.Equal(t, NewOptString("2026-06-27T12:05:08"), job.PublishedAt)
	department, ok := job.Department.Get()
	require.True(t, ok)
	assert.Equal(t, NewOptString("DeepSeek"), department.Name)
	require.Len(t, job.Departments, 1)

	// Detail locations carry countryDescription and omit provinceId, unlike
	// the list endpoint's.
	jobLocations := job.Locations.Or(nil)
	require.Len(t, jobLocations, 2)
	assert.Equal(t, NewOptString("China"), jobLocations[0].CountryDescription)
	assert.False(t, jobLocations[0].ProvinceId.IsSet())
}

func TestGetJobNotFound(t *testing.T) {
	client := testClient(t)

	_, err := client.GetJob(t.Context(), GetJobRequest{
		OrgId:  MockOrgID,
		SiteId: MockSiteID,
		JobId:  MockUnknownJobID,
	})

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, CodeJobNotFound, apiErr.Code)
	assert.Contains(t, apiErr.Msg, "您访问的职位已不存在")
}

// TestListFilterAggregations also covers the plaintext branch of the
// deobfuscating transport: this endpoint answers without the envelope.
func TestListFilterAggregations(t *testing.T) {
	client := testClient(t)

	aggs, err := client.ListFilterAggregations(t.Context(), SiteRef{
		OrgId:  MockOrgID,
		SiteId: MockSiteID,
	})
	require.NoError(t, err)

	system, ok := aggs.SystemFieldsAggregations.Get()
	require.True(t, ok)

	locations, ok := system.LocationAggregation.Get()
	require.True(t, ok)
	require.Len(t, locations.LocationList, 3)
	assert.Equal(t, NewOptString("Beijing"), locations.LocationList[0].Label)
	require.Len(t, locations.LocationList[0].LocationRows, 1)
	assert.Equal(t, NewOptInt(530103), locations.LocationList[0].LocationRows[0].ID)

	zhineng, ok := system.ZhinengAggregation.Get()
	require.True(t, ok)
	require.NotEmpty(t, zhineng.ZhinengList)
	assert.Equal(t, NewOptString("全栈开发/算法"), zhineng.ZhinengList[0].Label)
	assert.Equal(t, NewOptInt(16851), zhineng.ZhinengList[0].ID)

	// 职能 is a two-level tree; the functional group carries children.
	var withChildren *ZhinengFacet
	for i := range zhineng.ZhinengList {
		if len(zhineng.ZhinengList[i].Children) > 0 {
			withChildren = &zhineng.ZhinengList[i]
			break
		}
	}
	require.NotNil(t, withChildren)
	assert.Equal(t, NewOptString("职能部门"), withChildren.Label)
	assert.Equal(t, NewOptInt(236610), withChildren.Children[0].ParentId)
}

func TestDeobfuscateRejectsWrongKey(t *testing.T) {
	// A payload whose declared key does not decrypt it must fail loudly rather
	// than yield garbage the JSON decoder then misreports.
	_, err := deobfuscate([]byte(`{"data":"AAAAAAAAAAAAAAAAAAAAAA==","necromancer":"0123456789abcdef"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "padding")
}

func TestDeobfuscatePassesThroughPlaintext(t *testing.T) {
	body := []byte(`{"code":0,"success":true}`)
	out, err := deobfuscate(body)
	require.NoError(t, err)
	assert.Equal(t, body, out)
}

func TestURLs(t *testing.T) {
	assert.Equal(t,
		"https://app.mokahr.com/social-recruitment/high-flyer/140576",
		CareersURL(MockOrgID, MockSiteID))
	assert.Equal(t,
		"https://app.mokahr.com/social-recruitment/high-flyer/140576#/job/"+MockJobID,
		JobURL(MockOrgID, MockSiteID, MockJobID))
}

// TestListJobsOmittedLocations covers the tenants that keep workplaces off the
// listing: the key is absent, not null, and has to decode to nothing rather
// than fail.
func TestListJobsOmittedLocations(t *testing.T) {
	client := testClient(t)

	list, err := client.ListJobs(t.Context(), ListJobsRequest{
		OrgId:    MockNoLocationOrgID,
		SiteId:   MockNoLocationSiteID,
		Limit:    NewOptInt(5),
		NeedStat: NewOptBool(true),
	})
	require.NoError(t, err)

	require.NotEmpty(t, list.Jobs)
	for _, j := range list.Jobs {
		assert.NotEmpty(t, j.ID)
		assert.NotEmpty(t, j.Title)
		assert.Empty(t, j.Locations.Or(nil), "the listing carries no workplace")
	}
}

// TestListJobsTolerantOfNullLocations pins why locations is modeled nullable.
// MokaHR has only ever been seen omitting the key, but an explicit null is
// what a plain array schema turns into a hard decode failure, and careers-URL
// input reaches tenants no roster sampling has covered.
func TestListJobsTolerantOfNullLocations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":0,"success":true,"msg":"成功","data":{"jobStats":{"orgId":"x","total":1},`+
			`"jobs":[{"id":"a","title":"T","locations":null}]}}`)
	}))
	t.Cleanup(srv.Close)
	client, err := NewJobsClient(srv.URL, nil)
	require.NoError(t, err)

	list, err := client.ListJobs(t.Context(), ListJobsRequest{OrgId: "x", SiteId: "1"})
	require.NoError(t, err)
	require.Len(t, list.Jobs, 1)
	assert.Empty(t, list.Jobs[0].Locations.Or(nil))
}
