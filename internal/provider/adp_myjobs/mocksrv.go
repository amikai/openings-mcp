package adp_myjobs

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

// MockSlug is the Guitar Center tenant captured in testdata.
const MockSlug = "guitarcenterexternal"

// MockSecondSlug is the Church Mutual tenant config fixture.
const MockSecondSlug = "churchmutual"

// MockUnknownSlug is a non-existent career-site slug (HTTP 400 upstream).
const MockUnknownSlug = "this-company-does-not-exist-xyz-openings-mcp"

//go:embed testdata/career_site_guitarcenterexternal.json
var mockCareerSiteGC []byte

//go:embed testdata/career_site_churchmutual.json
var mockCareerSiteCM []byte

//go:embed testdata/career_site_unknown_rsp.json
var mockCareerSiteUnknown []byte

// NewMockServer serves career-site, list (+ optional $search), and search-meta.
func NewMockServer() *httptest.Server {
	const listAll = `{"count":2,"jobRequisitions":[
		{"reqId":"1001","jobTitle":"Engineer A","publishedJobTitle":"Engineer A","jobDescription":"<p>A</p>","workLevelCode":"Full Time","postingDate":"2026-01-01","requisitionLocations":[{"itemID":"loc-1","primaryIndicator":true,"address":{"cityName":"Langhorne","countrySubdivisionLevel1":{"codeValue":"PA"},"countryCode":{"codeValue":"USA"}},"nameCode":{"codeValue":"1","longName":"GC - Langhorne-1"}}]},
		{"reqId":"1002","jobTitle":"Music Teacher Store 1","publishedJobTitle":"Music Teacher Store 1","jobDescription":"<p>Teach music</p>","workLevelCode":"Part Time","postingDate":"2026-01-02","requisitionLocations":[{"itemID":"loc-2","primaryIndicator":true,"address":{"cityName":"Orem","countrySubdivisionLevel1":{"codeValue":"UT"},"countryCode":{"codeValue":"USA"}},"nameCode":{"codeValue":"2","longName":"GC - Orem-2"}}]}
	]}`
	const listTeacher = `{"count":1,"jobRequisitions":[
		{"reqId":"1002","jobTitle":"Music Teacher Store 1","publishedJobTitle":"Music Teacher Store 1","jobDescription":"<p>Teach music</p>","workLevelCode":"Part Time","postingDate":"2026-01-02","requisitionLocations":[{"itemID":"loc-2","primaryIndicator":true,"address":{"cityName":"Orem","countrySubdivisionLevel1":{"codeValue":"UT"},"countryCode":{"codeValue":"USA"}},"nameCode":{"codeValue":"2","longName":"GC - Orem-2"}}]}
	]}`
	const detail = `{"jobRequisitions":[{
		"itemID":"1002",
		"requisitionTitle":"Music Teacher Store 1",
		"requisitionDescription":"<p>Teach music full description</p>",
		"clientRequisitionID":"c1",
		"requisitionLocations":[],
		"postingInstructions":[{"timestampLastPosted":"2026-01-02T00:00:00Z"}]
	}]}`

	mux := http.NewServeMux()
	mux.HandleFunc("/public/staffing/v1/career-site/", func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimPrefix(r.URL.Path, "/public/staffing/v1/career-site/")
		slug = strings.Trim(slug, "/")
		switch strings.ToLower(slug) {
		case MockSlug:
			serveJSON(w, http.StatusOK, mockCareerSiteGC)
		case MockSecondSlug:
			serveJSON(w, http.StatusOK, mockCareerSiteCM)
		default:
			serveJSON(w, http.StatusBadRequest, mockCareerSiteUnknown)
		}
	})
	mux.HandleFunc("/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/apply-custom-filters", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("myjobstoken") == "" || r.Header.Get("rolecode") == "" {
			http.Error(w, "bad headers", http.StatusBadRequest)
			return
		}
		search := strings.ToLower(r.URL.Query().Get("$search"))
		filter := strings.ToLower(r.URL.Query().Get("$filter"))
		if strings.Contains(filter, "requisitionlocations/itemid eq loc-2") {
			serveJSON(w, http.StatusOK, []byte(listTeacher))
			return
		}
		if search == "" {
			serveJSON(w, http.StatusOK, []byte(listAll))
			return
		}
		if strings.Contains(search, "teacher") {
			serveJSON(w, http.StatusOK, []byte(listTeacher))
			return
		}
		serveJSON(w, http.StatusOK, []byte(`{"count":0,"jobRequisitions":[]}`))
	})
	mux.HandleFunc("/myadp_prefix/mycareer/public/staffing/v1/job-requisitions/search-meta/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("myjobstoken") == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		serveJSON(w, http.StatusOK, []byte(detail))
	})
	return httptest.NewServer(mux)
}

// NewTinyBoardMockServer is an alias of NewMockServer for older tests.
func NewTinyBoardMockServer() *httptest.Server { return NewMockServer() }

func serveJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
