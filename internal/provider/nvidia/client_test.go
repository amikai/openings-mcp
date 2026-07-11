package nvidia

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		var req JobsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if !assert.NoError(t, err) {
			return
		}

		wantReq := JobsRequest{
			AppliedFacets: AppliedFacets{},
			Limit:         20,
			Offset:        0,
			SearchText:    "golang",
		}
		assert.Equal(t, wantReq, req)

		serveMockJSON(mockJobsRsp)(w, r)
	})

	mux.HandleFunc("/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		serveMockJSON(mockJobDetailRsp)(w, r)
	})

	return httptest.NewServer(mux)
}

func TestSearchJobs(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	got, err := c.SearchJobs(t.Context(), &JobsRequest{
		AppliedFacets: AppliedFacets{},
		Limit:         20,
		Offset:        0,
		SearchText:    "golang",
	})
	require.NoError(t, err)

	want := &JobsResponse{
		Total: NewNilInt(27),
		JobPostings: []JobSummary{
			{
				Title:         NewOptNilString("Senior Software Golang Kubernetes Engineer"),
				ExternalPath:  NewOptString("/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916"),
				LocationsText: NewOptNilString("3 Locations"),
				PostedOn:      NewOptNilString("Posted 30+ Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Software Golang Kubernetes Engineer"),
				ExternalPath:  NewOptString("/job/Israel-Tel-Aviv/Senior-Software-Golang-Kubernetes-Engineer_JR2016621"),
				LocationsText: NewOptNilString("3 Locations"),
				PostedOn:      NewOptNilString("Posted 30+ Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Software Engineer, GoLang - DSX MaxQ"),
				ExternalPath:  NewOptString("/job/US-CA-Santa-Clara/Senior-Software-Engineer--GoLang---DSX-MaxQ_JR2017740-1"),
				LocationsText: NewOptNilString("3 Locations"),
				PostedOn:      NewOptNilString("Posted 4 Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior C++ Software Engineer - Chip Design Tools"),
				ExternalPath:  NewOptString("/job/US-CA-Santa-Clara/Senior-C---Software-Engineer---Chip-Design-Tools_JR2009389"),
				LocationsText: NewOptNilString("4 Locations"),
				PostedOn:      NewOptNilString("Posted 30+ Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Full Stack Software Engineer - DGX Cloud"),
				ExternalPath:  NewOptString("/job/US-NC-Remote/Senior-Full-Stack-Software-Engineer---DGX-Cloud_JR2017922"),
				LocationsText: NewOptNilString("5 Locations"),
				PostedOn:      NewOptNilString("Posted 15 Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior System Software Engineer – GeForce NOW Cloud"),
				ExternalPath:  NewOptString("/job/US-CA-Santa-Clara/Senior-System-Software-Engineer---GeForce-NOW-Cloud_JR2018465"),
				LocationsText: NewOptNilString("US, CA, Santa Clara"),
				PostedOn:      NewOptNilString("Posted 20 Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior System Software Engineer for Cloud – GeForce NOW"),
				ExternalPath:  NewOptString("/job/US-CA-Santa-Clara/Senior-System-Software-Engineer-for-Cloud---GeForce-NOW_JR2013549"),
				LocationsText: NewOptNilString("US, CA, Santa Clara"),
				PostedOn:      NewOptNilString("Posted 30+ Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Software Development Tech Lead – AI Developer Experiences"),
				ExternalPath:  NewOptString("/job/China-Shanghai/Senior-Software-Development-Tech-Lead---AI-Developer-Experiences_JR2017783"),
				LocationsText: NewOptNilString("China, Shanghai"),
				PostedOn:      NewOptNilString("Posted 30+ Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior C++ Software Engineer - Infrastructure Tools"),
				ExternalPath:  NewOptString("/job/US-CA-Santa-Clara/Senior-C---Software-Engineer---Infrastructure-Tools_JR2018693"),
				LocationsText: NewOptNilString("4 Locations"),
				PostedOn:      NewOptNilString("Posted 30+ Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior DevOps Engineer"),
				ExternalPath:  NewOptString("/job/India-Pune/Senior-DevOps-Engineer_JR2019008"),
				LocationsText: NewOptNilString("India, Pune"),
				PostedOn:      NewOptNilString("Posted 20 Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Software Engineer, Cloud Automation"),
				ExternalPath:  NewOptString("/job/Poland-Warsaw/Senior-Software-Engineer--Cloud-Automation_JR2019580-1"),
				LocationsText: NewOptNilString("2 Locations"),
				PostedOn:      NewOptNilString("Posted 20 Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Data Backend Engineer"),
				ExternalPath:  NewOptString("/job/US-OR-Hillsboro/Senior-Data-Backend-Engineer_JR2020354"),
				LocationsText: NewOptNilString("2 Locations"),
				PostedOn:      NewOptNilString("Posted 2 Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Full-Stack Software Engineer - VLSI Tools"),
				ExternalPath:  NewOptString("/job/US-CA-Santa-Clara/Senior-Full-Stack-Software-Engineer---VLSI-Tools_JR2012368"),
				LocationsText: NewOptNilString("4 Locations"),
				PostedOn:      NewOptNilString("Posted 23 Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Infrastructure Engineer – Bazel Remote Execution"),
				ExternalPath:  NewOptString("/job/US-CA-Santa-Clara/Senior-Infrastructure-Engineer---Bazel-Remote-Execution_JR2019387"),
				LocationsText: NewOptNilString("US, CA, Santa Clara"),
				PostedOn:      NewOptNilString("Posted 16 Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Manager, Engineering - AI Developer Tools"),
				ExternalPath:  NewOptString("/job/US-CA-Santa-Clara/Senior-Manager--Engineering---AI-Developer-Tools_JR2019726-1"),
				LocationsText: NewOptNilString("2 Locations"),
				PostedOn:      NewOptNilString("Posted 15 Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Data Engineer - Financial Transactions & Automation"),
				ExternalPath:  NewOptString("/job/US-CA-Santa-Clara/Senior-Data-Engineer---Financial-Transactions---Automation_JR2009512"),
				LocationsText: NewOptNilString("2 Locations"),
				PostedOn:      NewOptNilString("Posted 30+ Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior System Software Engineer for Cloud – GeForce NOW Platform"),
				ExternalPath:  NewOptString("/job/US-CA-Santa-Clara/Senior-System-Software-Engineer-for-Cloud---GeForce-NOW-Platform_JR2018467"),
				LocationsText: NewOptNilString("US, CA, Santa Clara"),
				PostedOn:      NewOptNilString("Posted 30+ Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Systems Software Engineer, Kubernetes Scale - DGX Cloud"),
				ExternalPath:  NewOptString("/job/Germany-Remote/Senior-Systems-Software-Engineer--Kubernetes-Scale---DGX-Cloud_JR2020234-1"),
				LocationsText: NewOptNilString("6 Locations"),
				PostedOn:      NewOptNilString("Posted 6 Days Ago"),
			},
			{
				Title:         NewOptNilString("Senior Cloud Software Engineer"),
				ExternalPath:  NewOptString("/job/India-Bengaluru/Senior-Cloud-Software-Engineer_JR2020094"),
				LocationsText: NewOptNilString("2 Locations"),
				PostedOn:      NewOptNilString("Posted 2 Days Ago"),
			},
			{
				Title:         NewOptNilString("Systems Software Engineer, Kubernetes Scale - DGX Cloud"),
				ExternalPath:  NewOptString("/job/Germany-Remote/Systems-Software-Engineer--Kubernetes-Scale---DGX-Cloud_JR2020236"),
				LocationsText: NewOptNilString("6 Locations"),
				PostedOn:      NewOptNilString("Posted 5 Days Ago"),
			},
		},
	}

	assert.Equal(t, want, got)
}

func TestGetJobDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	got, err := c.GetJobDetail(t.Context(), GetJobDetailParams{
		Location:  "Israel-Yokneam",
		TitleSlug: "Senior-Software-Golang-Kubernetes-Engineer_JR2015916",
	})
	require.NoError(t, err)

	want := &JobDetailResponse{
		JobPostingInfo: JobPostingInfo{
			Title:               NewNilString("Senior Software Golang Kubernetes Engineer"),
			JobDescription:      NewNilString(`<p>NVIDIA Networking is looking for an excellent Software Developer to work on NVIDIA cloud platforms based on Kubernetes. We are seeking an experienced engineer who is deeply technical, hands-on, and has a wide system view. You will design, build and deploy high-performance and scalable clouds based on NVIDIA&#39;s superior ConnectX and Bluefield NICs and SpectrumX AI platform. We want to grow our teams with the smartest people in the world. If you&#39;re creative and autonomous, we want to hear from you!</p><p></p><p><b>What you&#39;ll be doing:</b></p><ul><li><p>Design and implement new features to accelerate Network and Storage</p></li><li><p>Work closely with open source communities, participate in the relevant working groups</p></li><li><p>Work with different teams across NVIDIA</p></li><li><p>Mentor members of the team, enabling them to deliver high-quality software</p></li></ul><p></p><p><b>What we need to see:</b></p><ul><li><p>BSc in Computer Science or equivalent program</p></li><li><p>5&#43; years of hands-on experience in software development, preferably with Python/Golang</p></li><li><p>Highly motivated with strong communication skills, the ability to work successfully with multi-functional teams, developers, and architects</p></li><li><p>Coordinate effectively across organizational boundaries and geographies</p></li><li><p>Strong self-initiative, independence, and flexibility to a new technology</p></li><li><p>Deep understanding of network protocols, virtualization, and containers</p></li><li><p>Strong background in designing, implementing, and debugging complex software</p></li><li><p><span>Hands-on experience with Kubernetes</span></p></li></ul><p></p><p><b>Ways to stand out from the crowd:</b></p><ul><li><p>Experience with working on open source projects</p></li><li><p>Background with SR-IOV, DPDK, ROCE technologies</p></li><li><p>Experience in developing Kubernetes Operators, CSI plugins, CNI Plugins</p></li></ul><p><br />We are an equal opportunity employer and value diversity at our company. We do not discriminate on the basis of race, religion, color, national origin, sex, gender, gender expression, sexual orientation, age, marital status, veteran status, or disability status. We will ensure that individuals with disabilities are provided reasonable accommodation to participate in the job application or interview process, perform essential job functions, and receive other benefits and privileges of employment.</p><p style="text-align:inherit"></p><p style="text-align:inherit"></p><p style="text-align:inherit"></p><p style="text-align:inherit"></p>`),
			Location:            NewOptNilString("Israel, Yokneam"),
			AdditionalLocations: []string{"Israel, Raanana", "Israel, Tel Aviv"},
			PostedOn:            NewOptNilString("Posted 30+ Days Ago"),
			TimeType:            NewOptNilString("Full time"),
			JobReqId:            NewOptNilString("JR2015916"),
			ExternalUrl:         NewOptNilString("https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916"),
		},
	}

	assert.Equal(t, want, got)
}
