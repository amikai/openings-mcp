package job104

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/search/api/jobs", serveTestdata("testdata/jobs_rsp.json"))
	mux.HandleFunc("/job/ajax/content/", serveTestdata("testdata/job_detail_rsp.json"))
	mux.HandleFunc("/company/ajax/list", serveTestdata("testdata/companies_rsp.json"))
	mux.HandleFunc("/api/companies/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/content") {
			serveTestdata("testdata/company_detail_rsp.json")(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/jobs") {
			serveTestdata("testdata/company_jobs_rsp.json")(w, r)
		}
	})
	return httptest.NewServer(mux)
}

func serveTestdata(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func TestJobs(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	r, err := c.Jobs(t.Context(), &JobsRequest{Keyword: "Golang", Area: AreaTaipei})
	if err != nil {
		t.Fatal(err)
	}
	p := r.Metadata.Pagination
	if p.Total == 0 {
		t.Fatal("got 0 total")
	}
	if len(r.Data) == 0 {
		t.Fatal("got 0 jobs in page")
	}
	if p.CurrentPage == 0 || p.LastPage == 0 {
		t.Fatalf("bad pagination: page=%d last=%d", p.CurrentPage, p.LastPage)
	}
	for i, job := range r.Data {
		if job.JobName == "" {
			t.Errorf("data[%d].JobName empty", i)
		}
		if job.CustName == "" {
			t.Errorf("data[%d].CustName empty", i)
		}
		if job.Link.Job == "" {
			t.Errorf("data[%d].Link.Job empty", i)
		}
	}
}

func TestJobDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	r, err := c.Jobs(t.Context(), &JobsRequest{})
	if err != nil || len(r.Data) == 0 {
		t.Skip("no jobs in testdata")
	}
	parts := strings.Split(r.Data[0].Link.Job, "/")
	code := parts[len(parts)-1]

	d, err := c.JobDetail(t.Context(), code)
	if err != nil {
		t.Fatal(err)
	}
	if d.Data.Header.JobName == "" {
		t.Error("Data.Header.JobName empty")
	}
	if d.Data.Header.CustName == "" {
		t.Error("Data.Header.CustName empty")
	}
	if d.Data.JobDetail.JobDescription == "" {
		t.Error("Data.JobDetail.JobDescription empty")
	}
	if d.Data.CustNo == "" {
		t.Error("Data.CustNo empty")
	}
}

func TestCompanies(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	r, err := c.Companies(t.Context(), &CompaniesRequest{Keyword: "科技"})
	if err != nil {
		t.Fatal(err)
	}
	p := r.Metadata.Pagination
	if p.Total == 0 {
		t.Fatal("got 0 total")
	}
	if len(r.Data) == 0 {
		t.Fatal("got 0 companies in page")
	}
	for i, co := range r.Data {
		if co.Name == "" {
			t.Errorf("data[%d].Name empty", i)
		}
		if co.EncodedCustNo == "" {
			t.Errorf("data[%d].EncodedCustNo empty", i)
		}
	}
}

func TestCompanyDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	// TSMC
	d, err := c.CompanyDetail(t.Context(), "a5h92m0")
	if err != nil {
		t.Fatal(err)
	}
	if d.Data.CustName == "" {
		t.Error("Data.CustName empty")
	}
	if d.Data.IndustryDesc == "" {
		t.Error("Data.IndustryDesc empty")
	}
}

func TestCompanyJobs(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}

	// TSMC
	jobs, err := c.CompanyJobs(t.Context(), "a5h92m0")
	if err != nil {
		t.Fatal(err)
	}
	list := append(jobs.Data.List.TopJobs, jobs.Data.List.NormalJobs...)
	for i, job := range list {
		if job.JobName == "" {
			t.Errorf("jobs[%d].JobName empty", i)
		}
		if job.EncodedJobNo == "" {
			t.Errorf("jobs[%d].EncodedJobNo empty", i)
		}
	}
}
