package ashby

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer wraps the fixture handlers with request assertions the
// reusable NewMockServer deliberately doesn't make.
func newTestServer(t *testing.T, wantIncludeComp bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/posting-api/job-board/"+MockBoardName, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		if wantIncludeComp {
			assert.Equal(t, "true", r.URL.Query().Get("includeCompensation"))
			serveMockJSON(mockBoardCompRsp)(w, r)
			return
		}
		assert.Empty(t, r.URL.Query().Get("includeCompensation"))
		serveMockJSON(mockBoardRsp)(w, r)
	})
	return httptest.NewServer(mux)
}

func TestGetJobBoard(t *testing.T) {
	srv := newTestServer(t, false)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobBoard(t.Context(), GetJobBoardParams{JobBoardName: MockBoardName})
	require.NoError(t, err)

	board, ok := res.(*JobBoardResponse)
	require.True(t, ok, "expected *JobBoardResponse, got %T", res)

	assert.Equal(t, "1", board.ApiVersion.Value)
	require.Len(t, board.Jobs, 5)

	job := board.Jobs[0]
	assert.Equal(t, NewOptString("7724fbe3-6a27-4418-9705-2dcc40751a16"), job.ID)
	assert.Equal(t, "Software Engineer (Agent Platform)", job.Title.Value)
	assert.Equal(t, NewOptNilString("Engineering"), job.Department)
	assert.Equal(t, NewOptNilString("Engineering"), job.Team)
	assert.Equal(t, NilJobPostingEmploymentType{Value: JobPostingEmploymentTypeFullTime}, job.EmploymentType)
	assert.Equal(t, NilJobPostingWorkplaceType{Value: JobPostingWorkplaceTypeOnSite}, job.WorkplaceType)
	assert.Equal(t, NewOptNilString("San Francisco"), job.Location)
	assert.Equal(t, NilBool{Value: false}, job.IsRemote)
	assert.True(t, job.IsListed.Value)
	assert.True(t, job.PublishedAt.Value.Equal(time.Date(2025, 8, 25, 20, 13, 34, 942_000_000, time.UTC)))
	assert.Equal(t, "https://jobs.ashbyhq.com/browserbase/7724fbe3-6a27-4418-9705-2dcc40751a16", job.JobUrl.Value)
	assert.Equal(t, "https://jobs.ashbyhq.com/browserbase/7724fbe3-6a27-4418-9705-2dcc40751a16/application", job.ApplyUrl.Value)
	assert.True(t, job.DescriptionHtml.Set)
	assert.True(t, job.DescriptionPlain.Set)

	addr := job.Address.Value.PostalAddress.Value
	assert.Equal(t, NewOptNilString("San Francisco"), addr.AddressLocality)
	assert.Equal(t, NewOptNilString("CA"), addr.AddressRegion)
	assert.Equal(t, NewOptNilString("United States"), addr.AddressCountry)
	assert.Equal(t, NewOptNilString("94104"), addr.PostalCode)
	assert.Equal(t, NewOptNilString("1 Post Street, Floor 15"), addr.StreetAddress)

	require.Len(t, job.SecondaryLocations, 1)
	assert.Equal(t, NewOptNilString("New York"), job.SecondaryLocations[0].Location)

	for _, j := range board.Jobs {
		assert.False(t, j.Compensation.Set, "compensation must be absent without includeCompensation")
		assert.False(t, j.ShouldDisplayCompensationOnJobPostings.Set)
	}
}

func TestGetJobBoardWithCompensation(t *testing.T) {
	srv := newTestServer(t, true)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobBoard(t.Context(), GetJobBoardParams{
		JobBoardName:        MockBoardName,
		IncludeCompensation: NewOptBool(true),
	})
	require.NoError(t, err)

	board, ok := res.(*JobBoardResponse)
	require.True(t, ok, "expected *JobBoardResponse, got %T", res)
	require.Len(t, board.Jobs, 5)

	job := board.Jobs[0]
	assert.Equal(t, NewOptNilBool(true), job.ShouldDisplayCompensationOnJobPostings)
	require.True(t, job.Compensation.Set)

	comp := job.Compensation.Value
	assert.Equal(t, OptNilString{Value: "$132K – $330K • Offers Equity", Set: true}, comp.CompensationTierSummary)
	assert.Equal(t, OptNilString{Value: "$132K - $330K", Set: true}, comp.ScrapeableCompensationSalarySummary)

	// Job 3 publishes no compensation ranges: the compensation object is
	// present but both summaries are null and the arrays are empty.
	unpriced := board.Jobs[3].Compensation.Value
	assert.Equal(t, OptNilString{Set: true, Null: true}, unpriced.CompensationTierSummary)
	assert.Equal(t, OptNilString{Set: true, Null: true}, unpriced.ScrapeableCompensationSalarySummary)
	assert.Empty(t, unpriced.CompensationTiers)

	require.Len(t, comp.CompensationTiers, 1)
	tier := comp.CompensationTiers[0]
	assert.Equal(t, OptNilString{Set: true, Null: true}, tier.Title, "unnamed tier decodes as null title")
	assert.Equal(t, NewOptNilString("Estimated base salary $132K – $330K • Offers Equity"), tier.TierSummary)

	require.Len(t, tier.Components, 2)
	salary := tier.Components[0]
	assert.Equal(t, NewOptNilString("Salary"), salary.CompensationType)
	assert.Equal(t, NewOptNilString("1 YEAR"), salary.Interval)
	assert.Equal(t, OptNilString{Value: "USD", Set: true}, salary.CurrencyCode)
	assert.Equal(t, OptNilFloat64{Value: 132000, Set: true}, salary.MinValue)
	assert.Equal(t, OptNilFloat64{Value: 330000, Set: true}, salary.MaxValue)

	equity := tier.Components[1]
	assert.Equal(t, NewOptNilString("EquityPercentage"), equity.CompensationType)
	assert.Equal(t, OptNilString{Set: true, Null: true}, equity.CurrencyCode)
	assert.Equal(t, OptNilFloat64{Set: true, Null: true}, equity.MinValue)
	assert.Equal(t, OptNilFloat64{Set: true, Null: true}, equity.MaxValue)
}

// TestGetJobBoardNullFields guards the fields the official docs claim are
// always present but many real boards null out: a job with isRemote: null
// and workplaceType: null must decode instead of failing the whole board
// (the bug that crashed `ashby --board openai search`).
func TestGetJobBoardNullFields(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobBoard(t.Context(), GetJobBoardParams{JobBoardName: MockNullsBoardName})
	require.NoError(t, err)

	board, ok := res.(*JobBoardResponse)
	require.True(t, ok, "expected *JobBoardResponse, got %T", res)
	require.Len(t, board.Jobs, 4)

	// Jobs 0-1 carry real values; jobs 2-3 null both fields.
	assert.Equal(t, NilBool{Value: true}, board.Jobs[0].IsRemote)
	assert.Equal(t, NilJobPostingWorkplaceType{Value: JobPostingWorkplaceTypeRemote}, board.Jobs[0].WorkplaceType)
	assert.Equal(t, NilBool{Null: true}, board.Jobs[2].IsRemote)
	assert.Equal(t, NilJobPostingWorkplaceType{Null: true}, board.Jobs[2].WorkplaceType)
	assert.Equal(t, NilBool{Null: true}, board.Jobs[3].IsRemote)
}

func TestGetJobBoardNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetJobBoard(t.Context(), GetJobBoardParams{JobBoardName: "no-such-board"})
	require.NoError(t, err)

	nf, ok := res.(*GetJobBoardNotFound)
	require.True(t, ok, "expected *GetJobBoardNotFound, got %T", res)
	body, err := io.ReadAll(nf)
	require.NoError(t, err)
	assert.Equal(t, "Not Found", strings.TrimSpace(string(body)))
}
