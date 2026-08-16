package dayforce

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

const (
	_mockCSRFToken = "33e493ffd99180b626498ce539f01c92fdb6f0bf6c4f083209b6fb7621440194"
	_mockCSRFValue = "33e493ffd99180b626498ce539f01c92fdb6f0bf6c4f083209b6fb7621440194%7Cc140f3a1832ef33a64a626452e79531f01c5b96f18340401e4ecd73a95996e7a"

	// MockJobID and MockNotFoundJobID are dayforce/testdata/detail_*.json's
	// posting ids, for tests exercising [BoardClient.Job].
	MockJobID         = 62374
	MockNotFoundJobID = 99999999

	// MockNullLocationJobID is search_null_location_rsp.json's posting id,
	// a continent-level location ("Europe") with isoCountryCode, stateCode,
	// and cityName all null at once — for tests exercising [BoardClient.Search]
	// against that shape.
	MockNullLocationJobID = 2640

	// MockNullPostingLocationsJobID is search_null_postinglocations_rsp.json's
	// posting id, whose postingLocations array itself (not just the fields
	// within it) is null on the search row — for tests exercising
	// [BoardClient.Search] against that shape.
	MockNullPostingLocationsJobID = 2418

	// MockNullCurrencyJobID is detail_null_currency_rsp.json's posting id
	// (jdemea 35762), whose detail response has isoCurrencyRegion: null and
	// hasVirtualLocation: false with zero postingLocations — for tests
	// exercising [BoardClient.Job] against that shape.
	MockNullCurrencyJobID = 35762

	// MockBoolAttributeJobID is detail_bool_attribute_rsp.json's posting id
	// (mymilacron 2684), whose TravelRequired attribute is a JSON bool with
	// type "bool".
	MockBoolAttributeJobID = 2684

	// MockNullDescriptionHeaderJobID is detail_null_description_header_rsp.json's
	// posting id (ara 8), whose jobDescriptionHeader is null while
	// jobDescription is present — for tests exercising [BoardClient.Job]
	// against that shape.
	MockNullDescriptionHeaderJobID = 8
)

//go:embed testdata/search_rsp.json
var _mockSearchRsp []byte

//go:embed testdata/search_filtered_rsp.json
var _mockSearchFilteredRsp []byte

//go:embed testdata/search_page2_rsp.json
var _mockSearchPage2Rsp []byte

//go:embed testdata/search_alljobs_rsp.json
var _mockSearchAllJobsRsp []byte

//go:embed testdata/search_unknown_tenant_rsp.json
var _mockSearchUnknownTenantRsp []byte

//go:embed testdata/search_null_location_rsp.json
var _mockSearchNullLocationRsp []byte

//go:embed testdata/search_null_postinglocations_rsp.json
var _mockSearchNullPostingLocationsRsp []byte

//go:embed testdata/detail_rsp.json
var _mockDetailRsp []byte

//go:embed testdata/detail_null_currency_rsp.json
var _mockDetailNullCurrencyRsp []byte

//go:embed testdata/detail_bool_attribute_rsp.json
var _mockDetailBoolAttributeRsp []byte

//go:embed testdata/detail_null_description_header_rsp.json
var _mockDetailNullDescriptionHeaderRsp []byte

//go:embed testdata/detail_not_found_rsp.json
var _mockDetailNotFoundRsp []byte

//go:embed testdata/attributes_departments_rsp.json
var _mockDepartmentsRsp []byte

//go:embed testdata/attributes_payclasses_rsp.json
var _mockPayClassesRsp []byte

//go:embed testdata/attributes_paytypes_rsp.json
var _mockPayTypesRsp []byte

//go:embed testdata/siteinfo_rsp.html
var _mockSiteInfoRsp []byte

// NewMockServer returns an httptest.Server that replays captured Dayforce
// candidate portal responses, including the next-auth CSRF cookie/header
// contract. It must be an httptest.NewTLSServer: the gate cookie,
// __Host-next-auth.csrf-token, is Secure, and Go's cookiejar stores but
// never sends a Secure cookie back to a plain http:// origin, so an
// httptest.NewServer mock would loop on 403 forever. The caller owns the
// server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/auth/csrf", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "__Host-next-auth.csrf-token",
			Value:    _mockCSRFValue,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
		})
		serveMockJSON(w, http.StatusOK, []byte(`{"csrfToken":"`+_mockCSRFToken+`"}`))
	})

	mux.HandleFunc("POST /api/geo/{ns}/jobposting/search", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("__Host-next-auth.csrf-token")
		if err != nil || cookie.Value != _mockCSRFValue || r.Header.Get("X-Csrf-Token") != _mockCSRFToken {
			// The live 403 body is a bare 9-byte "Forbidden" string with no
			// Content-Type; the spec models this response with no content,
			// so its exact bytes here are unchecked by the generated client.
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("Forbidden"))
			return
		}

		switch r.PathValue("ns") {
		case "nosuchtenantxyz":
			serveMockProblem(w, http.StatusNotFound, _mockSearchUnknownTenantRsp)
		case "mydayforce":
			serveMockJSON(w, http.StatusOK, _mockSearchAllJobsRsp)
		case "pca":
			serveMockJSON(w, http.StatusOK, pcaSearchFixture(r))
		case "mymilacron":
			serveMockJSON(w, http.StatusOK, mymilacronSearchFixture(r))
		case "emptyboard", "badculture":
			serveMockJSON(w, http.StatusOK, []byte(`{"jobPostings":[],"maxCount":0,"offset":0,"count":0}`))
		case "nossr":
			serveMockJSON(w, http.StatusOK, _mockSearchRsp)
		default:
			serveMockProblem(w, http.StatusNotFound, _mockSearchUnknownTenantRsp)
		}
	})

	mux.HandleFunc("GET /api/geo/{ns}/jobposting/{ns2}/{culture}/{jobBoardId}/{postingId}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("ns") == "pca" && r.PathValue("ns2") == "pca" &&
			r.PathValue("culture") == "en-US" && r.PathValue("jobBoardId") == "1" &&
			r.PathValue("postingId") == "62374" {
			serveMockJSON(w, http.StatusOK, _mockDetailRsp)
			return
		}
		if r.PathValue("ns") == "jdemea" && r.PathValue("ns2") == "jdemea" &&
			r.PathValue("culture") == "en-US" && r.PathValue("jobBoardId") == "1" &&
			r.PathValue("postingId") == "35762" {
			serveMockJSON(w, http.StatusOK, _mockDetailNullCurrencyRsp)
			return
		}
		if r.PathValue("ns") == "mymilacron" && r.PathValue("ns2") == "mymilacron" &&
			r.PathValue("culture") == "en-US" && r.PathValue("jobBoardId") == "1" &&
			r.PathValue("postingId") == "2684" {
			serveMockJSON(w, http.StatusOK, _mockDetailBoolAttributeRsp)
			return
		}
		if r.PathValue("ns") == "ara" && r.PathValue("ns2") == "ara" &&
			r.PathValue("culture") == "en-US" && r.PathValue("jobBoardId") == "1" &&
			r.PathValue("postingId") == "8" {
			serveMockJSON(w, http.StatusOK, _mockDetailNullDescriptionHeaderRsp)
			return
		}
		// Any other ns/culture/jobBoardId/postingId combination, including a
		// valid posting id requested under a different tenant, replays the
		// captured cross-tenant 404 fixture.
		serveMockProblem(w, http.StatusNotFound, _mockDetailNotFoundRsp)
	})

	mux.HandleFunc("GET /api/geo/{ns}/postingattributes/departments/{ns2}/{jobBoardId}/{culture}", serveMockJSONFunc(_mockDepartmentsRsp))
	mux.HandleFunc("GET /api/geo/{ns}/postingattributes/payclasses/{ns2}/{jobBoardId}/{culture}", serveMockJSONFunc(_mockPayClassesRsp))
	mux.HandleFunc("GET /api/geo/{ns}/postingattributes/paytypes/{ns2}/{jobBoardId}/{culture}", serveMockJSONFunc(_mockPayTypesRsp))

	mux.HandleFunc("GET /{culture}/{ns}/{xref}", func(w http.ResponseWriter, r *http.Request) {
		if ns := r.PathValue("ns"); ns == "badculture" || ns == "nossr" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(_mockSiteInfoRsp)
	})

	return httptest.NewTLSServer(mux)
}

// pcaSearchFixture picks the pca fixture matching the exact request body the
// live captures used: the base page, the searchText-filtered page, or page
// 2 via paginationStart.
func pcaSearchFixture(r *http.Request) []byte {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return _mockSearchRsp
	}
	if text, ok := req.SearchText.Get(); ok && text == "electrical engineer" {
		return _mockSearchFilteredRsp
	}
	// Location branch pins the captured geo payload. A request that has
	// locationString but not distance 50 / distanceUnit 0 falls through
	// to the unfiltered fixture so adapter tests fail closed.
	if loc, ok := req.LocationString.Get(); ok &&
		loc == "Chicago, Illinois, United States" {
		dist, distOK := req.Distance.Get()
		unit, unitOK := req.DistanceUnit.Get()
		if distOK && dist == 50 && unitOK && unit == 0 {
			return _mockSearchFilteredRsp
		}
		return _mockSearchRsp
	}
	if start, ok := req.PaginationStart.Get(); ok && start == 25 {
		return _mockSearchPage2Rsp
	}
	if start, ok := req.PaginationStart.Get(); ok && start > 25 {
		return []byte(`{"jobPostings":[],"maxCount":352,"offset":0,"count":0}`)
	}
	return _mockSearchRsp
}

// mymilacronSearchFixture picks between mymilacron's two captured fixtures:
// the base page (a null-field posting location, "Europe") and page 2
// (paginationStart 25), whose jobPostingId 2418 has a null postingLocations
// array itself.
func mymilacronSearchFixture(r *http.Request) []byte {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return _mockSearchNullLocationRsp
	}
	if start, ok := req.PaginationStart.Get(); ok && start == 25 {
		return _mockSearchNullPostingLocationsRsp
	}
	return _mockSearchNullLocationRsp
}

func serveMockJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// serveMockProblem matches the live API's 404 Content-Type, which is
// application/problem+json rather than plain application/json.
func serveMockProblem(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func serveMockJSONFunc(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		serveMockJSON(w, http.StatusOK, data)
	}
}
