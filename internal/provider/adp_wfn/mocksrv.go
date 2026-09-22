package adp_wfn

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

// Mock identifiers captured in testdata. They are the real tenants the
// fixtures came from, so a test that passes here is exercising the same
// values the live API answers to.
const (
	// MockCID is Novae, a 48-posting en_US board — big enough to page.
	MockCID = "d16ba13d-474e-4326-b628-74c87f0b289a"
	// MockSlug is the roster slug for MockCID.
	MockSlug = "novae"
	// MockSmallCID is MetLife Legal Plans, a two-posting board.
	MockSmallCID = "269ead05-4042-4cc5-b325-75a73a6e0439"
	// MockEnCACID is Bingemans, whose postings exist only under en_CA and
	// whose board is empty under the en_US the server would otherwise default
	// to.
	MockEnCACID = "5a312740-8c7b-44b7-80ad-50a35fbb25d3"
	// MockEnCALocale is the only locale MockEnCACID publishes postings under.
	MockEnCALocale = "en_CA"
	// MockUnknownCID is a syntactically valid GUID no career center answers
	// to; upstream returns HTML, not JSON.
	MockUnknownCID = "00000000-0000-0000-0000-000000000000"
	// MockUnlistedCID is a tenant this repository's roster does not carry, so
	// it can only be reached by careers URL. It serves the same en_CA-only
	// board as MockEnCACID, which is what makes it useful: a caller reaching
	// it by URL has to carry the locale along or find the board empty.
	MockUnlistedCID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
	// MockJobID is an ExternalJobID on MockSmallCID's board.
	MockJobID = "584587"
	// MockUnknownJobID is answered with a record stripped of its itemID.
	MockUnknownJobID = "99999999"
	// MockLegacySlug resolves through the retired career center to MockCID.
	MockLegacySlug = "novae"
	// MockUnresolvableLegacySlug is a real client slug the retired career
	// center never redirects away from, so resolution fails without erroring.
	MockUnresolvableLegacySlug = "pcliving"

	// MockLocationValue is a wire-ready pair published by MockCID's filters.
	MockLocationValue = "Clarinda, IA"
	// MockUnpairedLocation is the trap: a bare token forms no pair, so the
	// upstream ignores the filter and answers the whole board.
	MockUnpairedLocation = "Clarinda"

	// MockJobTypeCID is American School for the Deaf, whose 21-posting board
	// its four published job types partition exactly.
	MockJobTypeCID = "55a9d987-168e-4cd9-9f4d-bc3c10fb6e90"
	// MockJobTypeOID is a job-type oid MockJobTypeCID publishes.
	MockJobTypeOID = "7:132"
	// MockBogusJobTypeOID is not published by any tenant. It is answered with
	// a 19-row subset of the 21-row board rather than an error, an empty set,
	// or the whole board.
	MockBogusJobTypeOID = "9:999"
)

//go:embed testdata/search_rsp.json
var mockSearchSmall []byte

//go:embed testdata/search_page1_rsp.json
var mockSearchPage1 []byte

//go:embed testdata/search_page3_rsp.json
var mockSearchPage3 []byte

//go:embed testdata/search_location_rsp.json
var mockSearchLocation []byte

//go:embed testdata/search_location_nomatch_rsp.json
var mockSearchLocationNoMatch []byte

//go:embed testdata/search_query_rsp.json
var mockSearchQuery []byte

//go:embed testdata/search_en_ca_rsp.json
var mockSearchEnCA []byte

//go:embed testdata/search_asd_rsp.json
var mockSearchASD []byte

//go:embed testdata/search_jobtype_rsp.json
var mockSearchJobType []byte

//go:embed testdata/search_jobtype_bogus_rsp.json
var mockSearchJobTypeBogus []byte

//go:embed testdata/search_empty_rsp.json
var mockSearchEmpty []byte

//go:embed testdata/search_unknown_tenant_rsp.json
var mockUnknownTenant []byte

//go:embed testdata/filters_rsp.json
var mockFilters []byte

//go:embed testdata/filters_nolocation_rsp.json
var mockFiltersNoLocation []byte

//go:embed testdata/detail_rsp.json
var mockDetail []byte

//go:embed testdata/detail_not_found_rsp.json
var mockDetailNotFound []byte

//go:embed testdata/clientfeatures_rsp.json
var mockClientFeatures []byte

//go:embed testdata/contentlinks_rsp.json
var mockContentLinks []byte

// NewMockServer replays the captured fixtures, branching the same way the
// live API does rather than answering every request identically. The
// branching is the point: most of this surface's failure modes are HTTP 200
// responses that differ only in content, so a mock that ignored the request
// would let every one of them pass unnoticed.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		cid := q.Get("cid")
		if cid == "" {
			http.Error(w, "cid required", http.StatusInternalServerError)
			return
		}
		if cid == MockUnknownCID {
			writeHTML(w, http.StatusNotFound, mockUnknownTenant)
			return
		}

		// A locale the tenant publishes no postings under is answered with an
		// empty board and HTTP 200, never an error.
		if (cid == MockEnCACID || cid == MockUnlistedCID) && q.Get("locale") != MockEnCALocale {
			writeJSON(w, mockSearchEmpty)
			return
		}

		// Header filters come first: when one of them fails to parse the
		// upstream ignores it and answers the whole board, which is the
		// behaviour worth reproducing exactly.
		if loc := r.Header.Get("locationsList"); loc != "" {
			switch {
			case !strings.Contains(loc, ","):
				writeJSON(w, mockSearchPage1)
			case strings.EqualFold(loc, MockLocationValue):
				writeJSON(w, mockSearchLocation)
			default:
				writeJSON(w, mockSearchLocationNoMatch)
			}
			return
		}
		if cats := r.Header.Get("workerCategoriesList"); cats != "" {
			if cats == MockJobTypeOID {
				writeJSON(w, mockSearchJobType)
				return
			}
			// An oid the tenant does not publish is answered with a large
			// arbitrary subset rather than an error, an empty set, or the
			// whole board — 19 of this board's 21 rows, identically for every
			// bogus value tried.
			writeJSON(w, mockSearchJobTypeBogus)
			return
		}
		if q.Get("userQuery") != "" {
			writeJSON(w, mockSearchQuery)
			return
		}

		switch cid {
		case MockSmallCID:
			writeJSON(w, mockSearchSmall)
		case MockEnCACID, MockUnlistedCID:
			writeJSON(w, mockSearchEnCA)
		case MockJobTypeCID:
			writeJSON(w, mockSearchASD)
		default:
			if q.Get("$skip") == "41" {
				writeJSON(w, mockSearchPage3)
				return
			}
			writeJSON(w, mockSearchPage1)
		}
	})

	mux.HandleFunc("/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions/getSearchFilters", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cid") {
		case MockUnknownCID:
			writeHTML(w, http.StatusNotFound, mockUnknownTenant)
		case MockEnCACID, MockUnlistedCID:
			// This tenant publishes job types only, and still filters by
			// location correctly.
			writeJSON(w, mockFiltersNoLocation)
		default:
			writeJSON(w, mockFilters)
		}
	})

	mux.HandleFunc("/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cid") == MockUnknownCID {
			writeHTML(w, http.StatusNotFound, mockUnknownTenant)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions/")
		if id == MockJobID {
			writeJSON(w, mockDetail)
			return
		}
		// Every other id, known-but-unaddressable or simply absent, is
		// answered with a record carrying no itemID.
		writeJSON(w, mockDetailNotFound)
	})

	mux.HandleFunc("/mascsr/default/careercenter/public/events/staffing/client-features", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cid") == MockUnknownCID {
			writeHTML(w, http.StatusNotFound, mockUnknownTenant)
			return
		}
		writeJSON(w, mockClientFeatures)
	})

	mux.HandleFunc("/mascsr/default/careercenter/public/events/staffing/v1/content-links/career-center", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cid") == MockUnknownCID {
			writeHTML(w, http.StatusNotFound, mockUnknownTenant)
			return
		}
		writeJSON(w, mockContentLinks)
	})

	// The retired career center. Its first response sets a cookie and points
	// back at itself; only a client carrying that cookie is redirected onward
	// to a URL bearing the cid. A client without a jar loops until it gives
	// up, which is why this endpoint looks dead from the outside.
	mux.HandleFunc("/jobs/apply/posting.html", func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("client")
		if _, err := r.Cookie("mock-wfn-session"); err != nil {
			http.SetCookie(w, &http.Cookie{Name: "mock-wfn-session", Value: "1", Path: "/"})
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
			return
		}
		if slug != MockLegacySlug {
			// Some slugs never leave posting.html no matter how many times
			// they are followed.
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
			return
		}
		http.Redirect(w, r, "/mascsr/default/mdf/recruitment/intermediateRedirect.html?cid="+MockCID, http.StatusFound)
	})

	mux.HandleFunc("/mascsr/default/mdf/recruitment/intermediateRedirect.html", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, http.StatusOK, []byte("<html></html>"))
	})

	return httptest.NewServer(mux)
}

// MockBaseURL is the API root to point a [Config] at for a mock server.
func MockBaseURL(srvURL string) string {
	return srvURL + "/mascsr/default/careercenter/public/events/staffing"
}

func writeJSON(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func writeHTML(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
