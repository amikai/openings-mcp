package hrmos

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/jobs_p1_rsp.html
var mockJobsP1Rsp []byte

//go:embed testdata/jobs_p2_rsp.html
var mockJobsP2Rsp []byte

//go:embed testdata/jobs_p3_rsp.html
var mockJobsP3Rsp []byte

//go:embed testdata/jobs_p4_rsp.html
var mockJobsP4Rsp []byte

//go:embed testdata/jobs_small_rsp.html
var mockJobsSmallRsp []byte

//go:embed testdata/jobs_nofacets_rsp.html
var mockJobsNofacetsRsp []byte

//go:embed testdata/job_detail_rsp.html
var mockJobDetailRsp []byte

//go:embed testdata/job_detail_salary_rsp.html
var mockJobDetailSalaryRsp []byte

//go:embed testdata/job_detail_shinsotsu_rsp.html
var mockJobDetailShinsotsuRsp []byte

// Mock tenant slugs and job IDs, matching the captured fixtures.
const (
	MockSlugPaged    = "moneyforward" // 203 jobs across 3 pages, 5 facet groups
	MockSlugSmall    = "visional"     // 9 jobs, single page, 2 facet groups
	MockSlugNoFacets = "hrmos"        // 86 jobs, single page, no facet nav
	// MockSlugShinsotsu is the tenant serving the 新卒 detail fixture.
	MockSlugShinsotsu = "raksul"
	MockSlugNotFound  = "no-such-tenant"

	MockJobID       = "0000265" // moneyforward, baseSalary null
	MockJobIDSalary = "0000381" // visional, baseSalary populated
	// MockJobIDShinsotsu is a 新卒 posting: same page template, no JSON-LD.
	MockJobIDShinsotsu = "2167257001778999340"
	MockJobIDNotFound  = "9999999"
)

// NewMockServer returns an httptest.Server that mimics hrmos.co with canned
// fixture responses, so tests never hit the real site. The caller owns the
// server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/pages/{slug}/jobs", func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		switch slug {
		case MockSlugPaged:
			serveMockJobsPage(w, r)
		case MockSlugSmall:
			serveMockHTML(w, mockJobsSmallRsp)
		case MockSlugNoFacets:
			serveMockHTML(w, mockJobsNofacetsRsp)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/pages/{slug}/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		slug, id := r.PathValue("slug"), r.PathValue("id")
		switch {
		case slug == MockSlugPaged && id == MockJobID:
			serveMockHTML(w, mockJobDetailRsp)
		case slug == MockSlugSmall && id == MockJobIDSalary:
			serveMockHTML(w, mockJobDetailSalaryRsp)
		case slug == MockSlugShinsotsu && id == MockJobIDShinsotsu:
			serveMockHTML(w, mockJobDetailShinsotsuRsp)
		default:
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(mux)
}

// serveMockJobsPage routes moneyforward's paged dump by ?page=.
func serveMockJobsPage(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("page") {
	case "", "1":
		serveMockHTML(w, mockJobsP1Rsp)
	case "2":
		serveMockHTML(w, mockJobsP2Rsp)
	case "3":
		serveMockHTML(w, mockJobsP3Rsp)
	default:
		serveMockHTML(w, mockJobsP4Rsp)
	}
}

func serveMockHTML(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
