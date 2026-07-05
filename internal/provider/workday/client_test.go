package workday

import (
	"context"
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

		var req JobsRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		wantReq := JobsRequest{
			AppliedFacets: AppliedFacets{"jobFamilyGroup": {"0c40f6bd1d8f10ae43ffaefd46dc7e78"}},
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

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	resp, err := client.SearchJobs(context.Background(), &JobsRequest{
		AppliedFacets: AppliedFacets{"jobFamilyGroup": {"0c40f6bd1d8f10ae43ffaefd46dc7e78"}},
		Limit:         20,
		Offset:        0,
		SearchText:    "golang",
	})
	require.NoError(t, err)

	want := &JobsResponse{
		Total: 27,
		JobPostings: []JobSummary{
			{Title: NewOptString("Software Golang Kubernetes Engineer"), ExternalPath: NewOptString("/job/Israel-Tel-Aviv/Software-Golang-Kubernetes-Engineer_JR2020442"), LocationsText: NewOptString("3 Locations"), PostedOn: NewOptString("Posted 3 Days Ago"), BulletFields: []string{"JR2020442"}},
			{Title: NewOptString("Senior Software Golang Kubernetes Engineer"), ExternalPath: NewOptString("/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916"), LocationsText: NewOptString("3 Locations"), PostedOn: NewOptString("Posted 30+ Days Ago"), BulletFields: []string{"JR2015916"}},
			{Title: NewOptString("Senior Software Golang Kubernetes Engineer"), ExternalPath: NewOptString("/job/Israel-Tel-Aviv/Senior-Software-Golang-Kubernetes-Engineer_JR2016621"), LocationsText: NewOptString("3 Locations"), PostedOn: NewOptString("Posted 30+ Days Ago"), BulletFields: []string{"JR2016621"}},
			{Title: NewOptString("Senior Software Engineer, GoLang - DSX MaxQ"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-Software-Engineer--GoLang---DSX-MaxQ_JR2017740-1"), LocationsText: NewOptString("3 Locations"), PostedOn: NewOptString("Posted 7 Days Ago"), BulletFields: []string{"JR2017740"}},
			{Title: NewOptString("Senior C++ Software Engineer - Chip Design Tools"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-C---Software-Engineer---Chip-Design-Tools_JR2009389"), LocationsText: NewOptString("4 Locations"), PostedOn: NewOptString("Posted 30+ Days Ago"), BulletFields: []string{"JR2009389"}},
			{Title: NewOptString("Senior Full Stack Software Engineer - DGX Cloud"), ExternalPath: NewOptString("/job/US-NC-Remote/Senior-Full-Stack-Software-Engineer---DGX-Cloud_JR2017922"), LocationsText: NewOptString("5 Locations"), PostedOn: NewOptString("Posted 18 Days Ago"), BulletFields: []string{"JR2017922"}},
			{Title: NewOptString("Senior System Software Engineer – GeForce NOW Cloud"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-System-Software-Engineer---GeForce-NOW-Cloud_JR2018465"), LocationsText: NewOptString("US, CA, Santa Clara"), PostedOn: NewOptString("Posted 23 Days Ago"), BulletFields: []string{"JR2018465"}},
			{Title: NewOptString("Senior System Software Engineer for Cloud – GeForce NOW"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-System-Software-Engineer-for-Cloud---GeForce-NOW_JR2013549"), LocationsText: NewOptString("US, CA, Santa Clara"), PostedOn: NewOptString("Posted 30+ Days Ago"), BulletFields: []string{"JR2013549"}},
			{Title: NewOptString("Senior Software Development Tech Lead – AI Developer Experiences"), ExternalPath: NewOptString("/job/China-Shanghai/Senior-Software-Development-Tech-Lead---AI-Developer-Experiences_JR2017783"), LocationsText: NewOptString("China, Shanghai"), PostedOn: NewOptString("Posted 30+ Days Ago"), BulletFields: []string{"JR2017783"}},
			{Title: NewOptString("Senior C++ Software Engineer - Infrastructure Tools"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-C---Software-Engineer---Infrastructure-Tools_JR2018693"), LocationsText: NewOptString("4 Locations"), PostedOn: NewOptString("Posted 30+ Days Ago"), BulletFields: []string{"JR2018693"}},
			{Title: NewOptString("Senior DevOps Engineer"), ExternalPath: NewOptString("/job/India-Pune/Senior-DevOps-Engineer_JR2019008"), LocationsText: NewOptString("India, Pune"), PostedOn: NewOptString("Posted 23 Days Ago"), BulletFields: []string{"JR2019008"}},
			{Title: NewOptString("Senior Software Engineer, Cloud Automation"), ExternalPath: NewOptString("/job/Poland-Warsaw/Senior-Software-Engineer--Cloud-Automation_JR2019580-1"), LocationsText: NewOptString("2 Locations"), PostedOn: NewOptString("Posted 23 Days Ago"), BulletFields: []string{"JR2019580"}},
			{Title: NewOptString("Senior Data Backend Engineer"), ExternalPath: NewOptString("/job/US-OR-Hillsboro/Senior-Data-Backend-Engineer_JR2020354"), LocationsText: NewOptString("2 Locations"), PostedOn: NewOptString("Posted 5 Days Ago"), BulletFields: []string{"JR2020354"}},
			{Title: NewOptString("Senior Full-Stack Software Engineer - VLSI Tools"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-Full-Stack-Software-Engineer---VLSI-Tools_JR2012368"), LocationsText: NewOptString("4 Locations"), PostedOn: NewOptString("Posted 26 Days Ago"), BulletFields: []string{"JR2012368"}},
			{Title: NewOptString("Senior Infrastructure Engineer – Bazel Remote Execution"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-Infrastructure-Engineer---Bazel-Remote-Execution_JR2019387"), LocationsText: NewOptString("US, CA, Santa Clara"), PostedOn: NewOptString("Posted 19 Days Ago"), BulletFields: []string{"JR2019387"}},
			{Title: NewOptString("Senior Manager, Engineering - AI Developer Tools"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-Manager--Engineering---AI-Developer-Tools_JR2019726-1"), LocationsText: NewOptString("2 Locations"), PostedOn: NewOptString("Posted 18 Days Ago"), BulletFields: []string{"JR2019726"}},
			{Title: NewOptString("Senior Data Engineer - Financial Transactions & Automation"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-Data-Engineer---Financial-Transactions---Automation_JR2009512"), LocationsText: NewOptString("2 Locations"), PostedOn: NewOptString("Posted 30+ Days Ago"), BulletFields: []string{"JR2009512"}},
			{Title: NewOptString("Senior System Software Engineer for Cloud – GeForce NOW Platform"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-System-Software-Engineer-for-Cloud---GeForce-NOW-Platform_JR2018467"), LocationsText: NewOptString("US, CA, Santa Clara"), PostedOn: NewOptString("Posted 30+ Days Ago"), BulletFields: []string{"JR2018467"}},
			{Title: NewOptString("Senior Systems Software Engineer, Kubernetes Scale - DGX Cloud"), ExternalPath: NewOptString("/job/Germany-Remote/Senior-Systems-Software-Engineer--Kubernetes-Scale---DGX-Cloud_JR2020234-1"), LocationsText: NewOptString("6 Locations"), PostedOn: NewOptString("Posted 9 Days Ago"), BulletFields: []string{"JR2020234"}},
			{Title: NewOptString("Senior Cloud Software Engineer"), ExternalPath: NewOptString("/job/India-Bengaluru/Senior-Cloud-Software-Engineer_JR2020094"), LocationsText: NewOptString("2 Locations"), PostedOn: NewOptString("Posted 5 Days Ago"), BulletFields: []string{"JR2020094"}},
		},
		Facets: []FacetNode{
			{FacetParameter: NewOptString("jobFamilyGroup"), Descriptor: NewOptString("Job Category"), Values: []FacetNode{{Descriptor: NewOptString("Engineering"), ID: NewOptString("0c40f6bd1d8f10ae43ffaefd46dc7e78"), Count: NewOptInt(27)}, {Descriptor: NewOptString("Marketing"), ID: NewOptString("0c40f6bd1d8f10ae43ffc19725ec7e88"), Count: NewOptInt(1)}}},
			{FacetParameter: NewOptString("workerSubType"), Descriptor: NewOptString("Job Type"), Values: []FacetNode{{Descriptor: NewOptString("Regular Employee"), ID: NewOptString("0c40f6bd1d8f10adf6dae161b1844a15"), Count: NewOptInt(26)}, {Descriptor: NewOptString("Management"), ID: NewOptString("0c40f6bd1d8f10adf6dae2cd57444a16"), Count: NewOptInt(1)}}},
			{FacetParameter: NewOptString("timeType"), Descriptor: NewOptString("Time Type"), Values: []FacetNode{{Descriptor: NewOptString("Full time"), ID: NewOptString("5509c0b5959810ac0029943377d47364"), Count: NewOptInt(27)}}},
			{FacetParameter: NewOptString("locationMainGroup"), Values: []FacetNode{{FacetParameter: NewOptString("locationHierarchy2"), Descriptor: NewOptString("Location Type"), Values: []FacetNode{{Descriptor: NewOptString("Office"), ID: NewOptString("0c3f5f117e9a0101f6422f0fe79d0000"), Count: NewOptInt(24)}, {Descriptor: NewOptString("Remote"), ID: NewOptString("0c3f5f117e9a0101f63dc469c3010000"), Count: NewOptInt(9)}}}, {FacetParameter: NewOptString("locationHierarchy1"), Descriptor: NewOptString("Locations"), Values: []FacetNode{{Descriptor: NewOptString("United States"), ID: NewOptString("2fcb99c455831013ea52fb338f2932d8"), Count: NewOptInt(18)}, {Descriptor: NewOptString("Israel"), ID: NewOptString("2fcb99c455831013ea52bbe14cf9326c"), Count: NewOptInt(3)}, {Descriptor: NewOptString("Poland"), ID: NewOptString("2fcb99c455831013ea52d8783aa0329c"), Count: NewOptInt(3)}, {Descriptor: NewOptString("United Kingdom"), ID: NewOptString("2fcb99c455831013ea52f785717432d2"), Count: NewOptInt(2)}, {Descriptor: NewOptString("Switzerland"), ID: NewOptString("2fcb99c455831013ea52e9ef1a0032ba"), Count: NewOptInt(2)}, {Descriptor: NewOptString("Spain"), ID: NewOptString("2fcb99c455831013ea52e31a43e832ae"), Count: NewOptInt(2)}, {Descriptor: NewOptString("India"), ID: NewOptString("2fcb99c455831013ea52b82135ba3266"), Count: NewOptInt(2)}, {Descriptor: NewOptString("Germany"), ID: NewOptString("2fcb99c455831013ea52adc65f5d3254"), Count: NewOptInt(2)}, {Descriptor: NewOptString("France"), ID: NewOptString("2fcb99c455831013ea52aa2df70e324e"), Count: NewOptInt(2)}, {Descriptor: NewOptString("China"), ID: NewOptString("2fcb99c455831013ea529fe151e3323c"), Count: NewOptInt(1)}}}, {FacetParameter: NewOptString("locations"), Descriptor: NewOptString("Sites"), Values: []FacetNode{{Descriptor: NewOptString("US, CA, Santa Clara"), ID: NewOptString("91336993fab910af6d702fae0bb4c2e8"), Count: NewOptInt(16)}, {Descriptor: NewOptString("US, Remote"), ID: NewOptString("16fc4607fc4310011e929f7115f90000"), Count: NewOptInt(6)}, {Descriptor: NewOptString("US, WA, Seattle"), ID: NewOptString("d2088e737cbb01d5e2be9e52ce01926f"), Count: NewOptInt(5)}, {Descriptor: NewOptString("US, TX, Austin"), ID: NewOptString("91336993fab910af6d702b631b94c2de"), Count: NewOptInt(3)}, {Descriptor: NewOptString("US, NC, Durham"), ID: NewOptString("91336993fab910af6d7022e347dcc2ca"), Count: NewOptInt(3)}, {Descriptor: NewOptString("US, MA, Westford"), ID: NewOptString("91336993fab910af6d7008ff1774c28e"), Count: NewOptInt(3)}, {Descriptor: NewOptString("Poland, Remote"), ID: NewOptString("91336993fab910af6d6f931ae68cc18f"), Count: NewOptInt(3)}, {Descriptor: NewOptString("Israel, Yokneam"), ID: NewOptString("970bf8c909a701c749f87bdcd4008607"), Count: NewOptInt(3)}, {Descriptor: NewOptString("Israel, Raanana"), ID: NewOptString("970bf8c909a7013ea54a57dcd4008107"), Count: NewOptInt(3)}, {Descriptor: NewOptString("Israel, Tel Aviv"), ID: NewOptString("c7769ee377291036b08490819096b8bf"), Count: NewOptInt(3)}, {Descriptor: NewOptString("UK, Remote"), ID: NewOptString("91336993fab910af6d6f4954559cc126"), Count: NewOptInt(2)}, {Descriptor: NewOptString("France, Remote"), ID: NewOptString("91336993fab910af6d6fd6c8476cc220"), Count: NewOptInt(2)}, {Descriptor: NewOptString("Germany, Remote"), ID: NewOptString("91336993fab910af6d6fc96a405cc202"), Count: NewOptInt(2)}, {Descriptor: NewOptString("India, Pune"), ID: NewOptString("91336993fab910af6d6fb0782884c1cb"), Count: NewOptInt(2)}, {Descriptor: NewOptString("Spain, Remote"), ID: NewOptString("91336993fab910af6d6f87de293cc176"), Count: NewOptInt(2)}, {Descriptor: NewOptString("Switzerland, Remote"), ID: NewOptString("91336993fab910af6d6f7e35a07cc162"), Count: NewOptInt(2)}, {Descriptor: NewOptString("US, TX, Remote"), ID: NewOptString("91336993fab910af6d702939a7fcc2d9"), Count: NewOptInt(1)}, {Descriptor: NewOptString("US, OR, Hillsboro"), ID: NewOptString("91336993fab910af6d7027195454c2d4"), Count: NewOptInt(1)}, {Descriptor: NewOptString("US, WA, Redmond"), ID: NewOptString("91336993fab910af6d701e82d004c2c0"), Count: NewOptInt(1)}, {Descriptor: NewOptString("US, NC, Remote"), ID: NewOptString("91336993fab910af6d7006cdf31cc289"), Count: NewOptInt(1)}, {Descriptor: NewOptString("India, Bengaluru"), ID: NewOptString("91336993fab910af6d6fb9748af4c1df"), Count: NewOptInt(1)}, {Descriptor: NewOptString("Poland, Warsaw"), ID: NewOptString("811700f7388d0103b8ceed030d490e48"), Count: NewOptInt(1)}, {Descriptor: NewOptString("US, WA, Remote"), ID: NewOptString("91336993fab910af6d7169a81124c410"), Count: NewOptInt(1)}, {Descriptor: NewOptString("US, CA, Remote"), ID: NewOptString("91336993fab910af6d716528e9d4c406"), Count: NewOptInt(1)}, {Descriptor: NewOptString("China, Shanghai"), ID: NewOptString("91336993fab910af6d710b3449b4c33e"), Count: NewOptInt(1)}}}}},
		},
	}

	assert.Equal(t, want, resp)
}

// TestSearchJobsToleratesMissingFacets guards the JobsResponse contract: a
// tenant whose /jobs response carries usable total/jobPostings but omits the
// `facets` field must still decode (facets is intentionally not required),
// rather than failing the whole search. Regression guard for the CLI's
// generic, multi-tenant use.
func TestSearchJobsToleratesMissingFacets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"total":1,"jobPostings":[{"title":"X","externalPath":"/job/L/T_1"}]}`))
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	resp, err := client.SearchJobs(context.Background(), &JobsRequest{
		AppliedFacets: AppliedFacets{},
		Limit:         1,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	assert.Empty(t, resp.Facets)
}

func TestGetJobDetail(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	detail, err := client.GetJobDetail(context.Background(), GetJobDetailParams{
		Location:  "Israel-Yokneam",
		TitleSlug: "Senior-Software-Golang-Kubernetes-Engineer_JR2015916",
	})
	require.NoError(t, err)

	want := &JobDetailResponse{
		JobPostingInfo: JobPostingInfo{
			Title: "Senior Software Golang Kubernetes Engineer",
			JobDescription: "<p>NVIDIA Networking is looking for an excellent Software Developer to work on NVIDIA cloud platforms based on Kubernetes. We are seeking an experienced engineer who is deeply technical, hands-on, and has a wide system view. You will design, build and deploy high-performance and scalable clouds based on NVIDIA&#39;s superior ConnectX and Bluefield NICs and SpectrumX AI platform. We want to grow our teams with the smartest people in the world. If you&#39;re creative and autonomous, we want to hear from you!</p><p></p><p><b>What you&#39;ll be doing:</b></p><ul><li><p>Design and implement new features to accelerate Network and Storage</p></li><li><p>Work closely with open source communities, participate in the relevant working groups</p></li><li><p>Work with different teams across NVIDIA</p></li><li><p>Mentor members of the team, enabling them to deliver high-quality software</p></li></ul><p></p><p><b>What we need to see:</b></p><ul><li><p>BSc in Computer Science or equivalent program</p></li><li><p>5&#43; years of hands-on experience in software development, preferably with Python/Golang</p></li><li><p>Highly motivated with strong communication skills, the ability to work successfully with multi-functional teams, developers, and architects</p></li><li><p>Coordinate effectively across organizational boundaries and geographies</p></li><li><p>Strong self-initiative, independence, and flexibility to a new technology</p></li><li><p>Deep understanding of network protocols, virtualization, and containers</p></li><li><p>Strong background in designing, implementing, and debugging complex software</p></li><li><p><span>Hands-on experience with Kubernetes</span></p></li></ul><p></p><p><b>Ways to stand out from the crowd:</b></p><ul><li><p>Experience with working on open source projects</p></li><li><p>Background with SR-IOV, DPDK, ROCE technologies</p></li><li><p>Experience in developing Kubernetes Operators, CSI plugins, CNI Plugins</p></li></ul><p><br />We are an equal opportunity employer and value diversity at our company. We do not discriminate on the basis of race, religion, color, national origin, sex, gender, gender expression, sexual orientation, age, marital status, veteran status, or disability status. We will ensure that individuals with disabilities are provided reasonable accommodation to participate in the job application or interview process, perform essential job functions, and receive other benefits and privileges of employment.</p><p style=\"text-align:inherit\"></p><p style=\"text-align:inherit\"></p><p style=\"text-align:inherit\"></p><p style=\"text-align:inherit\"></p>",
			Location:            NewOptString("Israel, Yokneam"),
			AdditionalLocations: []string{"Israel, Raanana", "Israel, Tel Aviv"},
			PostedOn:            NewOptString("Posted 30+ Days Ago"),
			TimeType:            NewOptString("Full time"),
			JobReqId:            NewOptString("JR2015916"),
			ExternalUrl:         NewOptString("https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916"),
		},
	}

	assert.Equal(t, want, detail)
}
