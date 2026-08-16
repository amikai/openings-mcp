package meta

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

const (
	// MockJobID is the requisition ID captured in testdata/job_detail_rsp.json.
	MockJobID = "1063741453022215"
	// MockNotFoundJobID is a requisition ID with no posting.
	MockNotFoundJobID = "999999999999999"
	// mockFilteredOffice selects the filtered search fixture.
	mockFilteredOffice = "Singapore"
)

//go:embed testdata/jobs_rsp.json
var mockJobsResponse []byte

//go:embed testdata/jobs_filtered_rsp.json
var mockFilteredJobsResponse []byte

//go:embed testdata/job_detail_rsp.json
var mockJobDetailResponse []byte

//go:embed testdata/job_detail_notfound_rsp.json
var mockJobDetailNotFoundResponse []byte

//go:embed testdata/filters_rsp.json
var mockFiltersResponse []byte

//go:embed testdata/locations_rsp.json
var mockLocationsResponse []byte

// NewMockServer returns an httptest.Server that replays captured Meta
// Careers GraphQL responses, dispatching on doc_id and variables and
// enforcing the endpoint's presence-only lsd + header contract. The caller
// owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		// The live endpoint 400s without these; keep the mock as strict.
		if r.PostFormValue("lsd") == "" ||
			r.Header.Get("X-FB-LSD") == "" ||
			r.Header.Get("Origin") == "" ||
			r.Header.Get("Referer") == "" ||
			r.Header.Get("Sec-Fetch-Mode") != "cors" {
			http.Error(w, "missing lsd or browser headers", http.StatusBadRequest)
			return
		}
		variables := r.PostFormValue("variables")
		var body []byte
		switch r.PostFormValue("doc_id") {
		case searchDocID:
			if strings.Contains(variables, mockFilteredOffice) {
				body = mockFilteredJobsResponse
			} else {
				body = mockJobsResponse
			}
		case detailDocID:
			if strings.Contains(variables, MockJobID) {
				body = mockJobDetailResponse
			} else {
				body = mockJobDetailNotFoundResponse
			}
		case filtersDocID:
			body = mockFiltersResponse
		case locationsDocID:
			body = mockLocationsResponse
		default:
			http.Error(w, "unknown doc_id", http.StatusBadRequest)
			return
		}
		// The live endpoint labels JSON bodies as HTML with a quoted charset;
		// replay that too so clients never grow a dependency on a sane value.
		w.Header().Set("Content-Type", `text/html; charset="utf-8"`)
		w.Write(body)
	})
	return httptest.NewServer(mux)
}
