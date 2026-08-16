package ats

import (
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/herp"
)

// Postings from testdata/company_rsp.json, picked for the property each one
// pins.
const (
	_herpFullRemoteJobID = "ZluM2laebsdh" // FULL_REMOTEWORK, インサイドセールス, 東京都中央区 + 京都府
	_herpHybridJobID     = "KJeVI46CBl58" // HYBRID_REMOTEWORK, コンサルティング role
	_herpNoRemoteJobID   = "MGp32NANKleO" // no jobRemoteworkType at all
	_herpBodyOnlyJobID   = "WZPIQQCxEKFO" // mentions コンサルティング only in its body
	_herpSparseJobID     = "zCbE82jVP-8s" // company_sparse_rsp.json: no location of any kind
	_herpOptOutJobID     = "FfCSxynEKKev" // company_media_optout_rsp.json: open, but no media apply action
)

func testHerpAdapter(t *testing.T) *HerpAdapter {
	t.Helper()
	srv := herp.NewMockServer()
	t.Cleanup(srv.Close)
	a := NewHerpAdapter(&http.Client{Timeout: 5 * time.Second}, nil)
	a.baseURL = srv.URL
	return a
}

// herpAllJobIDs pages through a search so membership assertions are not
// truncated by pageSize.
func herpAllJobIDs(t *testing.T, a *HerpAdapter, slug string, p SearchParams) []string {
	t.Helper()
	var ids []string
	for page := 1; ; page++ {
		p.Page = page
		res, err := a.Search(t.Context(), slug, p)
		require.NoError(t, err)
		for _, j := range res.Jobs {
			ids = append(ids, j.JobID)
		}
		if page >= res.TotalPages {
			require.Len(t, ids, res.TotalCount)
			return ids
		}
	}
}

func TestHerpRoster(t *testing.T) {
	a := testHerpAdapter(t)
	assert.Len(t, a.Roster(), len(herp.Companies))
}

func TestHerpParseCareersURL(t *testing.T) {
	a := testHerpAdapter(t)

	tests := []struct {
		name     string
		rawURL   string
		wantSlug string
		wantOK   bool
	}{
		{name: "career board", rawURL: "https://herp.careers/careers/companies/notainc", wantSlug: "notainc", wantOK: true},
		{
			name:     "job page",
			rawURL:   "https://herp.careers/careers/companies/notainc/jobs/" + _herpFullRemoteJobID,
			wantSlug: "notainc",
			wantOK:   true,
		},
		{name: "herp hire career page", rawURL: "https://herp.careers/v1/herpinc", wantSlug: "herpinc", wantOK: true},
		// Slugs outside the roster resolve: one request settles whether the
		// company is listed, so there is no reason to reject them up front.
		{
			name:     "outside roster",
			rawURL:   "https://herp.careers/careers/companies/" + herp.MockSparseSlug,
			wantSlug: herp.MockSparseSlug,
			wantOK:   true,
		},
		{name: "host root", rawURL: "https://herp.careers/careers", wantOK: false},
		{name: "unrelated", rawURL: "https://careers.google.com/jobs", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.rawURL)
			require.NoError(t, err)
			slug, ok := a.ParseCareersURL(u)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantSlug, slug)
		})
	}
}

func TestHerpCareersHostPatternRegistered(t *testing.T) {
	// The registry only advertises careers-URL shapes for adapters listed
	// in careersHostPatternsByAdapter; a missing entry silently degrades
	// the "unrecognized careers URL" teaching error.
	assert.Contains(t, _careersHostPatternsByAdapter, "herp")
}

func TestHerpSearchAll(t *testing.T) {
	a := testHerpAdapter(t)
	res, err := a.Search(t.Context(), herp.MockSlug, SearchParams{})
	require.NoError(t, err)

	assert.Equal(t, 51, res.TotalCount)
	assert.Len(t, res.Jobs, _pageSize)
	for _, j := range res.Jobs {
		assert.NotEmpty(t, j.JobID)
		assert.NotEmpty(t, j.Title)
		assert.NotEmpty(t, j.PostedAt)
		assert.Contains(t, j.URL, "/careers/companies/"+herp.MockSlug+"/jobs/"+j.JobID)
	}
}

func TestHerpFilters(t *testing.T) {
	a := testHerpAdapter(t)
	fs, err := a.Filters(t.Context(), herp.MockSlug)
	require.NoError(t, err)

	// The taxonomy dimensions carry the labels a caller can read; only the
	// two upstream enums, which have no label form, stay as codes.
	assert.Contains(t, fs["jobRole"], "インサイドセールス")
	assert.Contains(t, fs["jobCategory"], "セールス・事業開発")
	assert.ElementsMatch(t, []string{"東京都", "京都府"}, fs["prefecture"])
	assert.Equal(t, []string{"中央区"}, fs["city"])
	assert.Contains(t, fs["employmentType"], "FULL_TIME")
	// The shared workplaceType key and bamboohr's label vocabulary, not this
	// upstream's FULL_REMOTEWORK/HYBRID_REMOTEWORK enum.
	assert.ElementsMatch(t, []string{"Remote", "Hybrid"}, fs["workplaceType"])
}

func TestHerpSearchFilters(t *testing.T) {
	a := testHerpAdapter(t)

	byRole, err := a.Search(t.Context(), herp.MockSlug, SearchParams{
		Filters: FilterSet{"jobRole": {"インサイドセールス"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 4, byRole.TotalCount)
	assert.Contains(t, jobIDs(byRole.Jobs), _herpFullRemoteJobID)

	byPrefecture, err := a.Search(t.Context(), herp.MockSlug, SearchParams{
		Filters: FilterSet{"prefecture": {"京都府"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 49, byPrefecture.TotalCount)
	for _, j := range byPrefecture.Jobs {
		assert.Contains(t, j.Location, "京都府")
	}
}

func TestHerpSearchRejectsUnknownFilter(t *testing.T) {
	a := testHerpAdapter(t)
	_, err := a.Search(t.Context(), herp.MockSlug, SearchParams{
		Filters: FilterSet{"department": {"engineering"}},
	})
	require.ErrorContains(t, err, `unknown filter key "department"`)
	require.ErrorContains(t, err, "jobRole")
}

func TestHerpRemoteSearchCoversHybrid(t *testing.T) {
	a := testHerpAdapter(t)

	// "remote" is the broad cut: full remote and hybrid both answer to it,
	// because hybrid outnumbers full remote on Japanese boards and silently
	// hiding it would be the worse default.
	remote := herpAllJobIDs(t, a, herp.MockSlug, SearchParams{Location: _remoteLocation})
	assert.Contains(t, remote, _herpFullRemoteJobID)
	assert.Contains(t, remote, _herpHybridJobID)
	assert.NotContains(t, remote, _herpNoRemoteJobID)

	// workplaceType is the precise cut.
	full := herpAllJobIDs(t, a, herp.MockSlug, SearchParams{
		Filters: FilterSet{"workplaceType": {"Remote"}},
	})
	assert.Contains(t, full, _herpFullRemoteJobID)
	assert.NotContains(t, full, _herpHybridJobID)
}

func TestHerpLocationAcceptsRomanizedPrefecture(t *testing.T) {
	a := testHerpAdapter(t)

	// The board's location text is Japanese-only, so the adapter also
	// indexes the romanized prefecture — otherwise an English location
	// search returns nothing at all, with no error to explain why.
	tokyo := herpAllJobIDs(t, a, herp.MockSlug, SearchParams{Location: "tokyo"})
	assert.Contains(t, tokyo, _herpFullRemoteJobID)

	japanese := herpAllJobIDs(t, a, herp.MockSlug, SearchParams{Location: "東京都"})
	assert.ElementsMatch(t, japanese, tokyo)

	// A value offered as a city filter also works as a location, so the two
	// discovery paths agree.
	byCity := herpAllJobIDs(t, a, herp.MockSlug, SearchParams{Location: "中央区"})
	assert.Contains(t, byCity, _herpFullRemoteJobID)
}

func TestHerpJobRolesMapToOrganizationUnit(t *testing.T) {
	a := testHerpAdapter(t)

	// コンサルティング is a job-role name on some postings and body text on
	// others; the role match must outrank the body match.
	res, err := a.Search(t.Context(), herp.MockSlug, SearchParams{Query: "コンサルティング"})
	require.NoError(t, err)

	ids := jobIDs(res.Jobs)
	require.Contains(t, ids, _herpHybridJobID)
	require.Contains(t, ids, _herpBodyOnlyJobID)
	assert.Less(t, slices.Index(ids, _herpHybridJobID), slices.Index(ids, _herpBodyOnlyJobID))
}

func TestHerpDetail(t *testing.T) {
	a := testHerpAdapter(t)
	d, err := a.Detail(t.Context(), herp.MockSlug, _herpFullRemoteJobID)
	require.NoError(t, err)

	assert.Equal(t, _herpFullRemoteJobID, d.JobID)
	assert.Equal(t, "100｜インサイドセールス", d.Title)
	assert.Equal(t, "株式会社Helpfeel", d.Company)
	assert.Equal(t, "2026-02-03", d.PostedAt)

	// Everything the unified struct has no field for lands in the body: the
	// company's own salary wording plus a normalized range, the sections the
	// site renders, and the startup profile.
	assert.Contains(t, d.Description, "年収 400万円〜")
	assert.Contains(t, d.Description, "必須スキル")
	assert.Contains(t, d.Description, "福利厚生")
	assert.Contains(t, d.Description, "最終更新\n2026-02-24")
	// The company block is a section like every other one, blank line and all.
	assert.Contains(t, d.Description, "応募受付\n受付中\n\n企業情報\n")
	assert.Contains(t, d.Description, "累計調達額: 33.3億円")
	assert.Contains(t, d.Description, "従業員数: 245名")
}

func TestHerpMediaOptOutLinksToApplicableSurface(t *testing.T) {
	a := testHerpAdapter(t)

	// companyIsApplicationEnabled governs the media pages only. When it is
	// false the media page has no apply action, so the applicant handoff has
	// to point at the HERP Hire career page instead of a dead end.
	res, err := a.Search(t.Context(), herp.MockMediaOptOutSlug, SearchParams{})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
	for _, j := range res.Jobs {
		assert.Contains(t, j.URL, "/v1/"+herp.MockMediaOptOutSlug+"/"+j.JobID)
	}

	d, err := a.Detail(t.Context(), herp.MockMediaOptOutSlug, _herpOptOutJobID)
	require.NoError(t, err)
	assert.Contains(t, d.URL, "/v1/"+herp.MockMediaOptOutSlug+"/"+_herpOptOutJobID)

	// The company opting out of media applications says nothing about whether
	// this posting is open; isApplicable does.
	assert.Contains(t, d.Description, "応募受付\n受付中")
}

func TestHerpFounderBadgeIsScopedToThisCompany(t *testing.T) {
	a := testHerpAdapter(t)

	// The inside founder keeps the badge.
	d, err := a.Detail(t.Context(), herp.MockSlug, _herpFullRemoteJobID)
	require.NoError(t, err)
	assert.Contains(t, d.Description, "経営陣: 永田 周一（創業者）")

	// The regression this guards: isFounder rides on one history entry, which
	// often names a previous company. On this board 原 康紘 founded TRIDENT
	// (isInside false) and 諸岡 裕人 founded this company (isInside true), so
	// a check on isFounder alone would badge both.
	res, err := a.Search(t.Context(), herp.MockOutsideFounderSlug, SearchParams{})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
	d, err = a.Detail(t.Context(), herp.MockOutsideFounderSlug, res.Jobs[0].JobID)
	require.NoError(t, err)
	assert.Contains(t, d.Description, "経営陣: 原 康紘, 諸岡 裕人（創業者）, 河内 佑介")
	assert.NotContains(t, d.Description, "原 康紘（創業者）")
}

func TestHerpFreeTextLocationStaysSearchable(t *testing.T) {
	a := testHerpAdapter(t)

	// The structured entry for 京都府 carries a null city, so 上京区 exists
	// only in the free-text location. Losing it would make a perfectly
	// reasonable city search return nothing.
	ids := herpAllJobIDs(t, a, herp.MockSlug, SearchParams{Location: "上京区"})
	assert.Contains(t, ids, _herpFullRemoteJobID)

	// The concise structured form is still what gets displayed.
	res, err := a.Search(t.Context(), herp.MockSlug, SearchParams{Location: "上京区"})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
	assert.NotContains(t, res.Jobs[0].Location, "上京区")
}

func TestHerpDetailNotFound(t *testing.T) {
	a := testHerpAdapter(t)
	_, err := a.Detail(t.Context(), herp.MockSlug, "000000000000")
	require.ErrorContains(t, err, "pass a job_id exactly as returned by the job search")
}

func TestHerpCompanyNotListed(t *testing.T) {
	a := testHerpAdapter(t)
	_, err := a.Search(t.Context(), herp.MockUnknownSlug, SearchParams{})
	require.ErrorContains(t, err, "is not listed on HERP Career")
}

func TestHerpPostingWithoutAnyLocation(t *testing.T) {
	a := testHerpAdapter(t)
	d, err := a.Detail(t.Context(), herp.MockSparseSlug, _herpSparseJobID)
	require.NoError(t, err)

	// No structured location, no free text, no remote type: the location
	// stays empty rather than being invented as "Remote".
	assert.Empty(t, d.Location)
}

func jobIDs(jobs []JobSummary) []string {
	ids := make([]string, 0, len(jobs))
	for _, j := range jobs {
		ids = append(ids, j.JobID)
	}
	return ids
}
