package ats

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/ultipro"
)

// mockUltiProSlug is a roster-shaped slug (lowercase company code) whose
// resolved Company happens to match the mock fixtures' company code and
// board id, so testUltiProAdapter's baseURL override always applies
// regardless of which company the slug names.
const mockUltiProSlug = "tec1006teser"

func testUltiProAdapter(t *testing.T) *UltiProAdapter {
	t.Helper()
	mock := ultipro.NewMockServer()
	t.Cleanup(mock.Close)
	a := NewUltiProAdapter(&http.Client{Timeout: 5 * time.Second})
	// Swap only the host; keep the real companyCode/boardId path so
	// requests still land on the mock's fixture routes (registered under
	// ultipro.MockCompanyCode/MockBoardID, the same values TechnoServe's
	// roster entry carries).
	a.baseURL = func(s ultipro.CareersSite) string {
		return mock.URL + "/" + s.CompanyCode + "/JobBoard/" + s.BoardID
	}
	return a
}

func TestUltiProRosterBuildsRegistry(t *testing.T) {
	_, err := NewRegistry(NewUltiProAdapter(http.DefaultClient))
	require.NoError(t, err)
}

func TestUltiProRosterReturnsCompanyNames(t *testing.T) {
	a := NewUltiProAdapter(http.DefaultClient)
	roster := a.Roster()
	require.NotEmpty(t, roster)
	found := false
	for _, c := range roster {
		if c.Slug == mockUltiProSlug {
			found = true
			assert.Equal(t, "TechnoServe", c.Name)
		}
	}
	assert.True(t, found, "expected %q in roster", mockUltiProSlug)
}

func TestUltiProParseCareersURL(t *testing.T) {
	a := NewUltiProAdapter(http.DefaultClient)
	cases := []struct {
		raw  string
		ok   bool
		slug string
	}{
		{"https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/", true, mockUltiProSlug},
		{"https://recruiting2.ultipro.com/SAL1002/JobBoard/bcc2e2d1-d94c-2041-4126-28086417eb0a/", true, "sal1002"},
		{
			// Not on the roster: falls back to the canonical URL slug.
			"https://recruiting.ultipro.com/UNKNOWNCODE/JobBoard/00000000-0000-0000-0000-000000000000/",
			true,
			"https://recruiting.ultipro.com/UNKNOWNCODE/JobBoard/00000000-0000-0000-0000-000000000000/",
		},
		{
			// Same roster company code, but a different board id: must NOT
			// fold onto the curated board — a tenant can run more than one
			// board under one code, and UltiPro addresses a board with all
			// three of host/code/board id together.
			"https://recruiting.ultipro.com/TEC1006TESER/JobBoard/00000000-0000-0000-0000-000000000000/",
			true,
			"https://recruiting.ultipro.com/TEC1006TESER/JobBoard/00000000-0000-0000-0000-000000000000/",
		},
		{
			// Same code and board id, but the curated host is recruiting2 —
			// must not collapse onto a roster entry with a different host.
			"https://recruiting.ultipro.com/SAL1002/JobBoard/bcc2e2d1-d94c-2041-4126-28086417eb0a/",
			true,
			"https://recruiting.ultipro.com/SAL1002/JobBoard/bcc2e2d1-d94c-2041-4126-28086417eb0a/",
		},
		{"https://boards.greenhouse.io/x", false, ""},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.raw)
		require.NoError(t, err)
		slug, ok := a.ParseCareersURL(u)
		assert.Equal(t, tc.ok, ok, tc.raw)
		assert.Equal(t, tc.slug, slug, tc.raw)
	}
}

func TestUltiProSearch(t *testing.T) {
	a := testUltiProAdapter(t)
	res, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{})
	require.NoError(t, err)
	assert.Equal(t, 90, res.TotalCount)
	assert.Len(t, res.Jobs, 20)
	first := res.Jobs[0]
	assert.NotEmpty(t, first.JobID)
	assert.NotEmpty(t, first.Title)
	assert.NotEmpty(t, first.Location)
	assert.Len(t, first.PostedAt, len("2006-01-02"))
	assert.Equal(t,
		"https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/OpportunityDetail?opportunityId="+first.JobID,
		first.URL,
	)
}

func TestUltiProSearchPageOverflow(t *testing.T) {
	a := testUltiProAdapter(t)
	_, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{Page: math.MaxInt})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestUltiProSearchFilterDepartment(t *testing.T) {
	a := testUltiProAdapter(t)
	res, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Filters: FilterSet{"department": {"Finance"}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jobs)
}

func TestUltiProSearchFilterTeachingErrors(t *testing.T) {
	a := testUltiProAdapter(t)

	_, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Filters: FilterSet{"schedule": {"FullTime"}},
	})
	require.ErrorContains(t, err, "unknown filter key")

	_, err = a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Filters: FilterSet{"department": {"Nonexistent Department"}},
	})
	require.ErrorContains(t, err, `filter value "Nonexistent Department" not found`)
	assert.Contains(t, err.Error(), "available:")

	_, err = a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Filters: FilterSet{"location_type": {"nowhere"}},
	})
	require.ErrorContains(t, err, `filter value "nowhere" not found`)
	assert.Contains(t, err.Error(), "Hybrid, Onsite, Remote")
}

func TestUltiProFilters(t *testing.T) {
	a := testUltiProAdapter(t)
	fs, err := a.Filters(t.Context(), mockUltiProSlug)
	require.NoError(t, err)
	assert.Equal(t, []string{"Hybrid", "Onsite", "Remote"}, fs["location_type"])
	assert.Contains(t, fs["department"], "Finance")
}

func TestUltiProDetail(t *testing.T) {
	a := testUltiProAdapter(t)
	d, err := a.Detail(t.Context(), mockUltiProSlug, ultipro.MockOpportunityID)
	require.NoError(t, err)
	assert.Equal(t, ultipro.MockOpportunityID, d.JobID)
	assert.Equal(t, "Conseiller Senior en Partenariat-BeniBiz", d.Title)
	assert.Equal(t, "TechnoServe", d.Company)
	assert.NotEmpty(t, d.Location)
	assert.NotEmpty(t, d.Description)
	assert.Contains(t, d.URL, "opportunityId="+ultipro.MockOpportunityID)
}

func TestUltiProDetailNotFound(t *testing.T) {
	a := testUltiProAdapter(t)
	_, err := a.Detail(t.Context(), mockUltiProSlug, ultipro.MockNotFoundOpportunityID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// recordedFilter mirrors LoadSearchResults' wire Filters entries, just
// enough to assert on FieldName/Values.
type recordedFilter struct {
	FieldName int      `json:"fieldName"`
	Values    []string `json:"values"`
}

// newUltiProRecordingServer serves a fixed two-category catalog and
// records every LoadSearchResults call's Filters array (nil if the
// endpoint is never hit — used to prove the "remote" + conflicting
// location_type guard short-circuits before any request).
func newUltiProRecordingServer(t *testing.T) (srv *httptest.Server, calls *int, lastFilters *[]recordedFilter) {
	t.Helper()
	calls = new(int)
	lastFilters = new([]recordedFilter)
	mux := http.NewServeMux()
	mux.HandleFunc("/JobBoardViewMore/ViewMoreJobCategories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"categories": []map[string]string{
				{"Id": "cat-eng", "DisplayName": "Engineering"},
				{"Id": "cat-sales", "DisplayName": "Sales"},
			},
			"totalCount": 2,
		})
	})
	mux.HandleFunc("/JobBoardViewMore/ViewMorePhysicalLocations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"locations": []map[string]string{
				{"Id": "loc-austin", "LocalizedDescription": "US/Austin"},
				{"Id": "loc-houston", "LocalizedDescription": "US/Houston"},
				{"Id": "loc-tokyo", "LocalizedDescription": "Japan/Tokyo"},
			},
			"totalCount": 3,
		})
	})
	mux.HandleFunc("/JobBoardView/LoadSearchResults", func(w http.ResponseWriter, r *http.Request) {
		*calls++
		var body struct {
			OpportunitySearch struct {
				Filters []recordedFilter `json:"Filters"`
			} `json:"opportunitySearch"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		*lastFilters = body.OpportunitySearch.Filters
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"opportunities": []any{}, "totalCount": 0})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, calls, lastFilters
}

func newUltiProRecordingAdapter(t *testing.T) (*UltiProAdapter, *int, *[]recordedFilter) {
	t.Helper()
	srv, calls, lastFilters := newUltiProRecordingServer(t)
	a := NewUltiProAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(ultipro.CareersSite) string { return srv.URL }
	return a, calls, lastFilters
}

func TestUltiProSearchFilterValuesCombineIntoOneObject(t *testing.T) {
	a, _, lastFilters := newUltiProRecordingAdapter(t)

	_, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Filters: FilterSet{"department": {"Engineering", "Sales"}},
	})
	require.NoError(t, err)
	require.Len(t, *lastFilters, 1, "department values must combine into one filter object, not one per value")
	assert.Equal(t, 5, (*lastFilters)[0].FieldName)
	assert.ElementsMatch(t, []string{"cat-eng", "cat-sales"}, (*lastFilters)[0].Values)

	_, err = a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Filters: FilterSet{"location_type": {"Hybrid", "Onsite"}},
	})
	require.NoError(t, err)
	require.Len(t, *lastFilters, 1)
	assert.Equal(t, 37, (*lastFilters)[0].FieldName)
	assert.ElementsMatch(t, []string{"0", "1"}, (*lastFilters)[0].Values)
}

func TestUltiProSearchRemoteLocationUsesLocationType(t *testing.T) {
	a, _, lastFilters := newUltiProRecordingAdapter(t)

	_, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{Location: "Remote"})
	require.NoError(t, err)
	require.Len(t, *lastFilters, 1, "remote location must resolve through location_type, not a field-4 physical-location filter")
	assert.Equal(t, 37, (*lastFilters)[0].FieldName)
	assert.Equal(t, []string{"2"}, (*lastFilters)[0].Values)
}

func TestUltiProSearchRemoteLocationIntersectsExplicitLocationType(t *testing.T) {
	a, _, lastFilters := newUltiProRecordingAdapter(t)

	// location="remote" and location_type=["Remote","Hybrid"] are separate,
	// ANDed criteria (a Remote location intersected with a Remote-or-Hybrid
	// type), not something that ORs Hybrid into the result — narrow to
	// Remote only rather than sending both codes.
	_, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Location: "remote",
		Filters:  FilterSet{"location_type": {"Remote", "Hybrid"}},
	})
	require.NoError(t, err)
	require.Len(t, *lastFilters, 1)
	assert.Equal(t, 37, (*lastFilters)[0].FieldName)
	assert.Equal(t, []string{"2"}, (*lastFilters)[0].Values)
}

func TestUltiProSearchRemoteLocationConflictShortCircuits(t *testing.T) {
	a, calls, _ := newUltiProRecordingAdapter(t)

	res, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Location: "remote",
		Filters:  FilterSet{"location_type": {"Onsite"}},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.TotalCount)
	assert.Equal(t, 0, *calls, "a location_type filter excluding remote makes remote+location contradictory; must not round-trip to the API")
}

// TestUltiProSearchRemoteLocationValidatesFiltersFirst covers the bug
// where the remote/location_type conflict check ran on raw, unvalidated
// filter input: an invalid location_type value or an unknown filter key
// combined with location="remote" must still produce buildFilters' normal
// teaching error, not silently fall through as "excludes remote" -> empty.
func TestUltiProSearchRemoteLocationValidatesFiltersFirst(t *testing.T) {
	a, calls, _ := newUltiProRecordingAdapter(t)

	_, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Location: "remote",
		Filters:  FilterSet{"location_type": {"nowhere"}},
	})
	require.ErrorContains(t, err, `filter value "nowhere" not found`)
	assert.Equal(t, 0, *calls)

	_, err = a.Search(t.Context(), mockUltiProSlug, SearchParams{
		Location: "remote",
		Filters:  FilterSet{"schedule": {"FullTime"}},
	})
	require.ErrorContains(t, err, "unknown filter key")
	assert.Equal(t, 0, *calls)
}

func TestUltiProSearchLocationFuzzyMatchesAllHits(t *testing.T) {
	a, _, lastFilters := newUltiProRecordingAdapter(t)

	// "US" substring-matches both Austin and Houston (via LocalizedDescription
	// "US/Austin", "US/Houston") but not Tokyo; both must OR together in one
	// field-4 filter rather than erroring or picking just one.
	_, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{Location: "US"})
	require.NoError(t, err)
	require.Len(t, *lastFilters, 1)
	assert.Equal(t, 4, (*lastFilters)[0].FieldName)
	assert.ElementsMatch(t, []string{"loc-austin", "loc-houston"}, (*lastFilters)[0].Values)
}

func TestUltiProSearchLocationNoMatchErrors(t *testing.T) {
	a, _, _ := newUltiProRecordingAdapter(t)

	_, err := a.Search(t.Context(), mockUltiProSlug, SearchParams{Location: "Nowhereville"})
	require.ErrorContains(t, err, `no location matching "Nowhereville"`)
	assert.Contains(t, err.Error(), "available:")
}

func TestUltiProDetailPropagatesNonNotFoundErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/OpportunityDetail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	a := NewUltiProAdapter(&http.Client{Timeout: 5 * time.Second})
	a.baseURL = func(ultipro.CareersSite) string { return srv.URL }

	_, err := a.Detail(t.Context(), mockUltiProSlug, ultipro.MockOpportunityID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "not found", "a 500 must surface as a fetch failure, not a misleading not-found teaching error")
	assert.Contains(t, err.Error(), "500")
}
