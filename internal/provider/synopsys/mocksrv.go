package synopsys

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/jobs_rsp.json
var mockJobsRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

// MockJobID, MockCity, and MockSlug address the one posting the detail
// fixture covers; any other triple gets the 404 page.
const (
	MockJobID = "93498496944"
	MockCity  = "bengaluru"
	MockSlug  = "staff-software-engineer"
)

// MockLocationTerm is the only term the mock geocoder resolves. Every other
// term returns an empty array, mirroring the live typeahead's answer for a
// place it does not recognize.
const MockLocationTerm = "bengaluru"

// mockLocationsRsp is hand-built to the shape documented in openapi.yaml
// rather than captured live — the locations endpoint has no fixture, and one
// suggestion is enough to exercise both the resolve and no-match paths.
const mockLocationsRsp = `[{"id":1,"value":"Bengaluru, Karnataka, India","lat":12.9716,"lon":77.5946,"type":1,"city":"Bengaluru","division1":"Karnataka","country":"India","lp":"1-2-3","lt":4,"pc":""}]`

// NewMockServer returns an httptest.Server serving canned Synopsys careers
// site fixtures, so tests never hit the live site. The caller owns the server
// and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/search-jobs/results", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockJobsRsp)
	})

	// The live typeahead answers HTTP 200 with [] for an unknown term, not
	// an error; the empty array is what makes a location unresolvable.
	mux.HandleFunc("/search-jobs/locations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.EqualFold(r.URL.Query().Get("term"), MockLocationTerm) {
			w.Write([]byte(mockLocationsRsp))
			return
		}
		w.Write([]byte(`[]`))
	})

	mux.HandleFunc("/job/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/job/"+MockCity+"/"+MockSlug+"/"+orgID+"/"+MockJobID {
			w.Write(mockJobDetailRsp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("<html><body>Not Found</body></html>"))
	})

	return httptest.NewServer(mux)
}
