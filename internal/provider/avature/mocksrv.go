package avature

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"strings"
)

//go:embed testdata/search_rsp.html
var _mockSearchRsp []byte

//go:embed testdata/search_filtered_rsp.html
var _mockSearchFilteredRsp []byte

//go:embed testdata/search_offset_rsp.html
var _mockSearchOffsetRsp []byte

//go:embed testdata/search_no_results_rsp.html
var _mockSearchNoResultsRsp []byte

//go:embed testdata/search_no_legend_rsp.html
var _mockSearchNoLegendRsp []byte

//go:embed testdata/search_jobs_theme_rsp.html
var _mockSearchJobsThemeRsp []byte

//go:embed testdata/job_detail_rsp.html
var _mockJobDetailRsp []byte

//go:embed testdata/job_detail_fields_rsp.html
var _mockJobDetailFieldsRsp []byte

//go:embed testdata/job_detail_not_found_rsp.html
var _mockJobDetailNotFoundRsp []byte

// NewMockServer returns an httptest.Server that replays captured Avature
// portal HTML under /careers (Bloomberg captures, plus the Koch detail as
// id 161128), /nolegend (Koch listing), and /jobs-theme (Avature's own
// portal). The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/careers/SearchJobs", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("search") == "zzzznonexistentkeyword12345":
			serveHTML(_mockSearchNoResultsRsp)(w, r)
		case q.Get("search") == "engineer":
			serveHTML(_mockSearchFilteredRsp)(w, r)
		case q.Get("jobOffset") == "12":
			serveHTML(_mockSearchOffsetRsp)(w, r)
		default:
			serveHTML(_mockSearchRsp)(w, r)
		}
	})
	mux.HandleFunc("/nolegend/SearchJobs", serveHTML(_mockSearchNoLegendRsp))
	mux.HandleFunc("/jobs-theme/SearchJobs", serveHTML(_mockSearchJobsThemeRsp))
	mux.HandleFunc("/careers/JobDetail/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/20873"):
			serveHTML(_mockJobDetailRsp)(w, r)
		case strings.HasSuffix(r.URL.Path, "/161128"):
			serveHTML(_mockJobDetailFieldsRsp)(w, r)
		default:
			// Live portals 302 unknown ids to <base>/Error.
			http.Redirect(w, r, "/careers/Error", http.StatusFound)
		}
	})
	mux.HandleFunc("/careers/Error", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(_mockJobDetailNotFoundRsp)
	})
	return httptest.NewServer(mux)
}

func serveHTML(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}
}
