package workday

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchJobs_Nvidia(t *testing.T) {
	srv := NewMockServer(MockNvidiaJobsRsp, MockNvidiaJobDetailRsp)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	resp, err := client.SearchJobs(t.Context(), &JobsRequest{
		AppliedFacets: AppliedFacets{"jobFamilyGroup": {"0c40f6bd1d8f10ae43ffaefd46dc7e78"}},
		Limit:         20,
		Offset:        0,
		SearchText:    "golang",
	})
	require.NoError(t, err)

	assert.Equal(t, &JobsResponse{
		Total: NewNilInt(27),
		JobPostings: []JobSummary{
			{Title: NewOptNilString("Software Golang Kubernetes Engineer"), ExternalPath: NewOptString("/job/Israel-Tel-Aviv/Software-Golang-Kubernetes-Engineer_JR2020442"), LocationsText: NewOptNilString("3 Locations"), PostedOn: NewOptNilString("Posted 3 Days Ago"), BulletFields: []string{"JR2020442"}},
			{Title: NewOptNilString("Senior Software Golang Kubernetes Engineer"), ExternalPath: NewOptString("/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916"), LocationsText: NewOptNilString("3 Locations"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"JR2015916"}},
			{Title: NewOptNilString("Senior Software Golang Kubernetes Engineer"), ExternalPath: NewOptString("/job/Israel-Tel-Aviv/Senior-Software-Golang-Kubernetes-Engineer_JR2016621"), LocationsText: NewOptNilString("3 Locations"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"JR2016621"}},
			{Title: NewOptNilString("Senior Software Engineer, GoLang - DSX MaxQ"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-Software-Engineer--GoLang---DSX-MaxQ_JR2017740-1"), LocationsText: NewOptNilString("3 Locations"), PostedOn: NewOptNilString("Posted 7 Days Ago"), BulletFields: []string{"JR2017740"}},
			{Title: NewOptNilString("Senior C++ Software Engineer - Chip Design Tools"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-C---Software-Engineer---Chip-Design-Tools_JR2009389"), LocationsText: NewOptNilString("4 Locations"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"JR2009389"}},
			{Title: NewOptNilString("Senior Full Stack Software Engineer - DGX Cloud"), ExternalPath: NewOptString("/job/US-NC-Remote/Senior-Full-Stack-Software-Engineer---DGX-Cloud_JR2017922"), LocationsText: NewOptNilString("5 Locations"), PostedOn: NewOptNilString("Posted 18 Days Ago"), BulletFields: []string{"JR2017922"}},
			{Title: NewOptNilString("Senior System Software Engineer – GeForce NOW Cloud"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-System-Software-Engineer---GeForce-NOW-Cloud_JR2018465"), LocationsText: NewOptNilString("US, CA, Santa Clara"), PostedOn: NewOptNilString("Posted 23 Days Ago"), BulletFields: []string{"JR2018465"}},
			{Title: NewOptNilString("Senior System Software Engineer for Cloud – GeForce NOW"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-System-Software-Engineer-for-Cloud---GeForce-NOW_JR2013549"), LocationsText: NewOptNilString("US, CA, Santa Clara"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"JR2013549"}},
			{Title: NewOptNilString("Senior Software Development Tech Lead – AI Developer Experiences"), ExternalPath: NewOptString("/job/China-Shanghai/Senior-Software-Development-Tech-Lead---AI-Developer-Experiences_JR2017783"), LocationsText: NewOptNilString("China, Shanghai"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"JR2017783"}},
			{Title: NewOptNilString("Senior C++ Software Engineer - Infrastructure Tools"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-C---Software-Engineer---Infrastructure-Tools_JR2018693"), LocationsText: NewOptNilString("4 Locations"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"JR2018693"}},
			{Title: NewOptNilString("Senior DevOps Engineer"), ExternalPath: NewOptString("/job/India-Pune/Senior-DevOps-Engineer_JR2019008"), LocationsText: NewOptNilString("India, Pune"), PostedOn: NewOptNilString("Posted 23 Days Ago"), BulletFields: []string{"JR2019008"}},
			{Title: NewOptNilString("Senior Software Engineer, Cloud Automation"), ExternalPath: NewOptString("/job/Poland-Warsaw/Senior-Software-Engineer--Cloud-Automation_JR2019580-1"), LocationsText: NewOptNilString("2 Locations"), PostedOn: NewOptNilString("Posted 23 Days Ago"), BulletFields: []string{"JR2019580"}},
			{Title: NewOptNilString("Senior Data Backend Engineer"), ExternalPath: NewOptString("/job/US-OR-Hillsboro/Senior-Data-Backend-Engineer_JR2020354"), LocationsText: NewOptNilString("2 Locations"), PostedOn: NewOptNilString("Posted 5 Days Ago"), BulletFields: []string{"JR2020354"}},
			{Title: NewOptNilString("Senior Full-Stack Software Engineer - VLSI Tools"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-Full-Stack-Software-Engineer---VLSI-Tools_JR2012368"), LocationsText: NewOptNilString("4 Locations"), PostedOn: NewOptNilString("Posted 26 Days Ago"), BulletFields: []string{"JR2012368"}},
			{Title: NewOptNilString("Senior Infrastructure Engineer – Bazel Remote Execution"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-Infrastructure-Engineer---Bazel-Remote-Execution_JR2019387"), LocationsText: NewOptNilString("US, CA, Santa Clara"), PostedOn: NewOptNilString("Posted 19 Days Ago"), BulletFields: []string{"JR2019387"}},
			{Title: NewOptNilString("Senior Manager, Engineering - AI Developer Tools"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-Manager--Engineering---AI-Developer-Tools_JR2019726-1"), LocationsText: NewOptNilString("2 Locations"), PostedOn: NewOptNilString("Posted 18 Days Ago"), BulletFields: []string{"JR2019726"}},
			{Title: NewOptNilString("Senior Data Engineer - Financial Transactions & Automation"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-Data-Engineer---Financial-Transactions---Automation_JR2009512"), LocationsText: NewOptNilString("2 Locations"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"JR2009512"}},
			{Title: NewOptNilString("Senior System Software Engineer for Cloud – GeForce NOW Platform"), ExternalPath: NewOptString("/job/US-CA-Santa-Clara/Senior-System-Software-Engineer-for-Cloud---GeForce-NOW-Platform_JR2018467"), LocationsText: NewOptNilString("US, CA, Santa Clara"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"JR2018467"}},
			{Title: NewOptNilString("Senior Systems Software Engineer, Kubernetes Scale - DGX Cloud"), ExternalPath: NewOptString("/job/Germany-Remote/Senior-Systems-Software-Engineer--Kubernetes-Scale---DGX-Cloud_JR2020234-1"), LocationsText: NewOptNilString("6 Locations"), PostedOn: NewOptNilString("Posted 9 Days Ago"), BulletFields: []string{"JR2020234"}},
			{Title: NewOptNilString("Senior Cloud Software Engineer"), ExternalPath: NewOptString("/job/India-Bengaluru/Senior-Cloud-Software-Engineer_JR2020094"), LocationsText: NewOptNilString("2 Locations"), PostedOn: NewOptNilString("Posted 5 Days Ago"), BulletFields: []string{"JR2020094"}},
		},
		Facets: NewOptNilFacetNodeArray([]FacetNode{
			{FacetParameter: NewOptNilString("jobFamilyGroup"), Descriptor: NewOptNilString("Job Category"), Values: []FacetNode{{Descriptor: NewOptNilString("Engineering"), ID: NewOptString("0c40f6bd1d8f10ae43ffaefd46dc7e78"), Count: NewOptNilInt(27)}, {Descriptor: NewOptNilString("Marketing"), ID: NewOptString("0c40f6bd1d8f10ae43ffc19725ec7e88"), Count: NewOptNilInt(1)}}},
			{FacetParameter: NewOptNilString("workerSubType"), Descriptor: NewOptNilString("Job Type"), Values: []FacetNode{{Descriptor: NewOptNilString("Regular Employee"), ID: NewOptString("0c40f6bd1d8f10adf6dae161b1844a15"), Count: NewOptNilInt(26)}, {Descriptor: NewOptNilString("Management"), ID: NewOptString("0c40f6bd1d8f10adf6dae2cd57444a16"), Count: NewOptNilInt(1)}}},
			{FacetParameter: NewOptNilString("timeType"), Descriptor: NewOptNilString("Time Type"), Values: []FacetNode{{Descriptor: NewOptNilString("Full time"), ID: NewOptString("5509c0b5959810ac0029943377d47364"), Count: NewOptNilInt(27)}}},
			{FacetParameter: NewOptNilString("locationMainGroup"), Values: []FacetNode{{FacetParameter: NewOptNilString("locationHierarchy2"), Descriptor: NewOptNilString("Location Type"), Values: []FacetNode{{Descriptor: NewOptNilString("Office"), ID: NewOptString("0c3f5f117e9a0101f6422f0fe79d0000"), Count: NewOptNilInt(24)}, {Descriptor: NewOptNilString("Remote"), ID: NewOptString("0c3f5f117e9a0101f63dc469c3010000"), Count: NewOptNilInt(9)}}}, {FacetParameter: NewOptNilString("locationHierarchy1"), Descriptor: NewOptNilString("Locations"), Values: []FacetNode{{Descriptor: NewOptNilString("United States"), ID: NewOptString("2fcb99c455831013ea52fb338f2932d8"), Count: NewOptNilInt(18)}, {Descriptor: NewOptNilString("Israel"), ID: NewOptString("2fcb99c455831013ea52bbe14cf9326c"), Count: NewOptNilInt(3)}, {Descriptor: NewOptNilString("Poland"), ID: NewOptString("2fcb99c455831013ea52d8783aa0329c"), Count: NewOptNilInt(3)}, {Descriptor: NewOptNilString("United Kingdom"), ID: NewOptString("2fcb99c455831013ea52f785717432d2"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("Switzerland"), ID: NewOptString("2fcb99c455831013ea52e9ef1a0032ba"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("Spain"), ID: NewOptString("2fcb99c455831013ea52e31a43e832ae"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("India"), ID: NewOptString("2fcb99c455831013ea52b82135ba3266"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("Germany"), ID: NewOptString("2fcb99c455831013ea52adc65f5d3254"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("France"), ID: NewOptString("2fcb99c455831013ea52aa2df70e324e"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("China"), ID: NewOptString("2fcb99c455831013ea529fe151e3323c"), Count: NewOptNilInt(1)}}}, {FacetParameter: NewOptNilString("locations"), Descriptor: NewOptNilString("Sites"), Values: []FacetNode{{Descriptor: NewOptNilString("US, CA, Santa Clara"), ID: NewOptString("91336993fab910af6d702fae0bb4c2e8"), Count: NewOptNilInt(16)}, {Descriptor: NewOptNilString("US, Remote"), ID: NewOptString("16fc4607fc4310011e929f7115f90000"), Count: NewOptNilInt(6)}, {Descriptor: NewOptNilString("US, WA, Seattle"), ID: NewOptString("d2088e737cbb01d5e2be9e52ce01926f"), Count: NewOptNilInt(5)}, {Descriptor: NewOptNilString("US, TX, Austin"), ID: NewOptString("91336993fab910af6d702b631b94c2de"), Count: NewOptNilInt(3)}, {Descriptor: NewOptNilString("US, NC, Durham"), ID: NewOptString("91336993fab910af6d7022e347dcc2ca"), Count: NewOptNilInt(3)}, {Descriptor: NewOptNilString("US, MA, Westford"), ID: NewOptString("91336993fab910af6d7008ff1774c28e"), Count: NewOptNilInt(3)}, {Descriptor: NewOptNilString("Poland, Remote"), ID: NewOptString("91336993fab910af6d6f931ae68cc18f"), Count: NewOptNilInt(3)}, {Descriptor: NewOptNilString("Israel, Yokneam"), ID: NewOptString("970bf8c909a701c749f87bdcd4008607"), Count: NewOptNilInt(3)}, {Descriptor: NewOptNilString("Israel, Raanana"), ID: NewOptString("970bf8c909a7013ea54a57dcd4008107"), Count: NewOptNilInt(3)}, {Descriptor: NewOptNilString("Israel, Tel Aviv"), ID: NewOptString("c7769ee377291036b08490819096b8bf"), Count: NewOptNilInt(3)}, {Descriptor: NewOptNilString("UK, Remote"), ID: NewOptString("91336993fab910af6d6f4954559cc126"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("France, Remote"), ID: NewOptString("91336993fab910af6d6fd6c8476cc220"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("Germany, Remote"), ID: NewOptString("91336993fab910af6d6fc96a405cc202"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("India, Pune"), ID: NewOptString("91336993fab910af6d6fb0782884c1cb"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("Spain, Remote"), ID: NewOptString("91336993fab910af6d6f87de293cc176"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("Switzerland, Remote"), ID: NewOptString("91336993fab910af6d6f7e35a07cc162"), Count: NewOptNilInt(2)}, {Descriptor: NewOptNilString("US, TX, Remote"), ID: NewOptString("91336993fab910af6d702939a7fcc2d9"), Count: NewOptNilInt(1)}, {Descriptor: NewOptNilString("US, OR, Hillsboro"), ID: NewOptString("91336993fab910af6d7027195454c2d4"), Count: NewOptNilInt(1)}, {Descriptor: NewOptNilString("US, WA, Redmond"), ID: NewOptString("91336993fab910af6d701e82d004c2c0"), Count: NewOptNilInt(1)}, {Descriptor: NewOptNilString("US, NC, Remote"), ID: NewOptString("91336993fab910af6d7006cdf31cc289"), Count: NewOptNilInt(1)}, {Descriptor: NewOptNilString("India, Bengaluru"), ID: NewOptString("91336993fab910af6d6fb9748af4c1df"), Count: NewOptNilInt(1)}, {Descriptor: NewOptNilString("Poland, Warsaw"), ID: NewOptString("811700f7388d0103b8ceed030d490e48"), Count: NewOptNilInt(1)}, {Descriptor: NewOptNilString("US, WA, Remote"), ID: NewOptString("91336993fab910af6d7169a81124c410"), Count: NewOptNilInt(1)}, {Descriptor: NewOptNilString("US, CA, Remote"), ID: NewOptString("91336993fab910af6d716528e9d4c406"), Count: NewOptNilInt(1)}, {Descriptor: NewOptNilString("China, Shanghai"), ID: NewOptString("91336993fab910af6d710b3449b4c33e"), Count: NewOptNilInt(1)}}}}},
		}),
	}, resp)
}

func TestGetJobDetail_Nvidia(t *testing.T) {
	srv := NewMockServer(MockNvidiaJobsRsp, MockNvidiaJobDetailRsp)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	detail, err := client.GetJobDetail(t.Context(), GetJobDetailParams{
		Location:  "Israel-Yokneam",
		TitleSlug: "Senior-Software-Golang-Kubernetes-Engineer_JR2015916",
	})
	require.NoError(t, err)

	assert.Equal(t, &JobDetailResponse{
		JobPostingInfo: JobPostingInfo{
			Title:               NewNilString("Senior Software Golang Kubernetes Engineer"),
			JobDescription:      NewNilString("<p>NVIDIA Networking is looking for an excellent Software Developer to work on NVIDIA cloud platforms based on Kubernetes. We are seeking an experienced engineer who is deeply technical, hands-on, and has a wide system view. You will design, build and deploy high-performance and scalable clouds based on NVIDIA&#39;s superior ConnectX and Bluefield NICs and SpectrumX AI platform. We want to grow our teams with the smartest people in the world. If you&#39;re creative and autonomous, we want to hear from you!</p><p></p><p><b>What you&#39;ll be doing:</b></p><ul><li><p>Design and implement new features to accelerate Network and Storage</p></li><li><p>Work closely with open source communities, participate in the relevant working groups</p></li><li><p>Work with different teams across NVIDIA</p></li><li><p>Mentor members of the team, enabling them to deliver high-quality software</p></li></ul><p></p><p><b>What we need to see:</b></p><ul><li><p>BSc in Computer Science or equivalent program</p></li><li><p>5&#43; years of hands-on experience in software development, preferably with Python/Golang</p></li><li><p>Highly motivated with strong communication skills, the ability to work successfully with multi-functional teams, developers, and architects</p></li><li><p>Coordinate effectively across organizational boundaries and geographies</p></li><li><p>Strong self-initiative, independence, and flexibility to a new technology</p></li><li><p>Deep understanding of network protocols, virtualization, and containers</p></li><li><p>Strong background in designing, implementing, and debugging complex software</p></li><li><p><span>Hands-on experience with Kubernetes</span></p></li></ul><p></p><p><b>Ways to stand out from the crowd:</b></p><ul><li><p>Experience with working on open source projects</p></li><li><p>Background with SR-IOV, DPDK, ROCE technologies</p></li><li><p>Experience in developing Kubernetes Operators, CSI plugins, CNI Plugins</p></li></ul><p><br />We are an equal opportunity employer and value diversity at our company. We do not discriminate on the basis of race, religion, color, national origin, sex, gender, gender expression, sexual orientation, age, marital status, veteran status, or disability status. We will ensure that individuals with disabilities are provided reasonable accommodation to participate in the job application or interview process, perform essential job functions, and receive other benefits and privileges of employment.</p><p style=\"text-align:inherit\"></p><p style=\"text-align:inherit\"></p><p style=\"text-align:inherit\"></p><p style=\"text-align:inherit\"></p>"),
			Location:            NewOptNilString("Israel, Yokneam"),
			AdditionalLocations: []string{"Israel, Raanana", "Israel, Tel Aviv"},
			PostedOn:            NewOptNilString("Posted 30+ Days Ago"),
			TimeType:            NewOptNilString("Full time"),
			JobReqId:            NewOptNilString("JR2015916"),
			ExternalUrl:         NewOptNilString("https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916"),
		},
	}, detail)
}

// TestSearchJobs_TrendMicro exercises a second tenant. Workday is multi-tenant
// and every company runs its own instance, so one tenant can't prove the
// contract is tenant-agnostic. Trend Micro (wd3 pod, site "External"; see
// testdata/trendmicro_*.hurl) is that second tenant.
func TestSearchJobs_TrendMicro(t *testing.T) {
	srv := NewMockServer(MockTrendMicroJobsRsp, MockTrendMicroJobDetailRsp)
	defer srv.Close()

	client, err := NewClient(srv.URL)
	require.NoError(t, err)

	search, err := client.SearchJobs(t.Context(), &JobsRequest{
		AppliedFacets: AppliedFacets{},
		Limit:         5,
		Offset:        0,
		SearchText:    "backend engineer",
	})
	require.NoError(t, err)

	assert.Equal(t, &JobsResponse{
		Total: NewNilInt(42),
		JobPostings: []JobSummary{
			{Title: NewOptNilString("(Sr.) Backend Engineer"), ExternalPath: NewOptString("/job/Taipei/XMLNAME--Sr--Backend-Engineer_R0006260-1"), LocationsText: NewOptNilString("Taipei"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"R0006260"}},
			{Title: NewOptNilString("(Sr.) Backend Engineer"), ExternalPath: NewOptString("/job/Taipei/XMLNAME--Sr--Backend-Engineer_R0009324"), LocationsText: NewOptNilString("Taipei"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"R0009324"}},
			{Title: NewOptNilString("Sr. Backend Engineer (IIA)"), ExternalPath: NewOptString("/job/Taipei/Sr-Backend-Engineer--IIA-_R0009294"), LocationsText: NewOptNilString("Taipei"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"R0009294"}},
			{Title: NewOptNilString("(Sr.) Applied AI Backend Engineer"), ExternalPath: NewOptString("/job/Taipei/XMLNAME--Sr--Backend-Engineer_R0007813"), LocationsText: NewOptNilString("Taipei"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"R0007813"}},
			{Title: NewOptNilString("(Sr.) Applied AI Backend Engineer"), ExternalPath: NewOptString("/job/Taipei/Sr-Engineer-Backend_R0007867"), LocationsText: NewOptNilString("Taipei"), PostedOn: NewOptNilString("Posted 30+ Days Ago"), BulletFields: []string{"R0007867"}},
		},
		Facets: NewOptNilFacetNodeArray([]FacetNode{
			{FacetParameter: NewOptNilString("jobFamilyGroup"), Descriptor: NewOptNilString("Job Category"), Values: []FacetNode{{Descriptor: NewOptNilString("Product Development"), ID: NewOptString("904e04688bb0016982d07b383e1dab0c"), Count: NewOptNilInt(40)}, {Descriptor: NewOptNilString("Threat"), ID: NewOptString("904e04688bb001c112547b383e1da90c"), Count: NewOptNilInt(1)}, {Descriptor: NewOptNilString("DataIQ"), ID: NewOptString("904e04688bb0010d914c7a383e1da50c"), Count: NewOptNilInt(1)}}},
			{FacetParameter: NewOptNilString("workerSubType"), Descriptor: NewOptNilString("Job Type"), Values: []FacetNode{{Descriptor: NewOptNilString("Regular"), ID: NewOptString("904e04688bb00110d3d92e873c1dfc04"), Count: NewOptNilInt(42)}}},
			{FacetParameter: NewOptNilString("timeType"), Descriptor: NewOptNilString("Time Type"), Values: []FacetNode{{Descriptor: NewOptNilString("Full time"), ID: NewOptString("f6887f8ffb6901a0ae6afd8c1a1d8900"), Count: NewOptNilInt(42)}}},
			{FacetParameter: NewOptNilString("locationMainGroup"), Values: []FacetNode{{FacetParameter: NewOptNilString("locationCountry"), Descriptor: NewOptNilString("Location Country"), Values: []FacetNode{{Descriptor: NewOptNilString("Canada"), ID: NewOptString("a30a87ed25634629aa6c3958aa2b91ea"), Count: NewOptNilInt(6)}, {Descriptor: NewOptNilString("Philippines"), ID: NewOptString("e56f1daf83e04bacae794ba5c5593560"), Count: NewOptNilInt(1)}, {Descriptor: NewOptNilString("Taiwan"), ID: NewOptString("a4e08b475d6a4176853c9d1cb9854e02"), Count: NewOptNilInt(27)}, {Descriptor: NewOptNilString("United States of America"), ID: NewOptString("bc33aa3152ec42d4995f4791a106ed09"), Count: NewOptNilInt(3)}}}}},
		}),
	}, search)

	// extract first external path
	externalPath := search.JobPostings[0].ExternalPath.Value
	// location: Taipei, titleSlug: XMLNAME--Sr--Backend-Engineer_R0006260-1
	location, titleSlug, ok := JobDetailKeyFromPath(externalPath)
	require.True(t, ok)

	detail, err := client.GetJobDetail(t.Context(), GetJobDetailParams{
		Location:  location,
		TitleSlug: titleSlug,
	})
	require.NoError(t, err)

	assert.Equal(t, &JobDetailResponse{
		JobPostingInfo: JobPostingInfo{
			Title:               NewNilString("(Sr.) Backend Engineer"),
			JobDescription:      NewNilString(`<p style="text-align:left"><b>Join Trend ‧ Join New Generation</b></p><p style="text-align:inherit"></p><p style="text-align:left"><b>趨勢科技 - 全球雲端資安領航者 / 全亞洲最大軟體公司 / 企業版圖橫跨五大洲 / 趨勢全球研發基地在台灣 </b><br /><span><span><span><span><span class="WL01">&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;</span></span></span></span></span></p><p>Trend Micro Vision One is a purpose-built threat defense platform that provides added value and new benefits beyond XDR (Extended Detection and Response) solutions, allowing customer to see more and respond faster. Vision One providing deep and broad extended detection and response capabilities that collect and automatically correlate data across multiple security layers—email, endpoints, servers, cloud workloads, and networks—Trend Micro Vision One prevents the majority of attacks with automated protection.</p><p>The objective of TW Engineering Group is to provide the best endpoint security solution to help customers, from small &amp; medium businesses to vary large enterprises, conquer cybersecurity challenges effectively and efficiently. Every partner within Group 1 will have opportunity to join diverse squad team and demonstrate his/her expertise and enthusiasm to make customer success in their business without cyber threat. Along this journey, we are looking for partners with not only exceptional engineering expertise, but also great curiosity and customer empathy, so we can learn from customer behavior and create business impact together.<br /><br /> </p><p><b>Responsibilities &#xff1a;</b></p><ul><li>Co-work with other teams to design, develop, &amp; operate scalable backend services to support endpoint security and peripheral use cases on public clouds (AWS, Azure, …etc.)</li><li>Design &amp; implement ways to measure and validate the quality of customer experience we deliver</li><li>Analyze &amp; resolve customer problems by listening to data and customer feedbacks</li><li>Identify and report defects, working closely with development teams to resolve issues promptly</li><li>Perform tasks and projects as assigned.</li><li>Document solutions for knowledge sharing within the team.</li></ul><p><b>BASIC QUALIFICATIONS</b></p><ul><li>Bachelor degree or higher in computer science or related fields</li><li>Strong programming skill with any of: Java or Go</li><li>Cloud Application development and debugging experience on AWS or Azure</li><li>Proactive, self-motivated and good teamwork</li><li>Good English communication skill</li></ul><p><b>BIG PLUS</b></p><ul><li>Cloud Application development on container technologies, such as Docker and Kubernetes.</li><li>Have experience with any of: PostgreSQL, MySQL, Redis, MongoDB, Elastic Search, Kibana, or Terraform</li></ul><p><b>Would be a plus</b></p><ul><li>Knowledge in enterprise design/integration pattern</li><li>Experience in the cybersecurity fields</li></ul><p></p><p>#LI-YJ1</p><p><span><span><span>&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;&#61;</span></span></span><br /><i>連結智慧 守護世界 --- Connected Intelligence for Securing a Connected World</i></p>`),
			Location:            NewOptNilString("Taipei"),
			AdditionalLocations: nil,
			PostedOn:            NewOptNilString("Posted 30+ Days Ago"),
			TimeType:            NewOptNilString("Full time"),
			JobReqId:            NewOptNilString("R0006260"),
			ExternalUrl:         NewOptNilString("https://trendmicro.wd3.myworkdayjobs.com/External/job/Taipei/XMLNAME--Sr--Backend-Engineer_R0006260-1"),
		},
	}, detail)
}
