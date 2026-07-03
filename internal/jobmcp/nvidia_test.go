package jobmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/nvidia"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testNvidiaMCPClientServer(t *testing.T) (*mcp.ClientSession, *mcp.ServerSession) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := nvidia.NewMockServer()
	t.Cleanup(srv.Close)
	client, err := nvidia.NewClient(srv.URL, nvidia.WithClient(srv.Client()))
	require.NoError(t, err)
	RegisterNvidia(server, client)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		serverSession.Close()
	})

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	clientSession, err := mcpClient.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		clientSession.Close()
	})
	return clientSession, serverSession
}

func TestRegisterNvidia(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client, err := nvidia.NewClient("https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite")
	require.NoError(t, err)
	RegisterNvidia(server, client)

	assertTools(t, server, "nvidia_search_jobs", "nvidia_get_job_detail")
}

func TestNvidiaSearchJobsE2E(t *testing.T) {
	clientSession, _ := testNvidiaMCPClientServer(t)

	res, err := clientSession.ListTools(t.Context(), nil)
	require.NoError(t, err)

	tool := findTool(res.Tools, "nvidia_search_jobs")
	require.NotNil(t, tool)

	schema, ok := tool.InputSchema.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "object", schema["type"])

	// Test calling nvidia_search_jobs tool
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "nvidia_search_jobs",
		Arguments: map[string]any{
			"keyword":       "golang",
			"job_category":  "Engineering",
			"job_type":      "Regular Employee",
			"time_type":     "Full time",
			"location_type": "Remote",
			"country":       "Taiwan",
			"site":          "Taiwan, Taipei",
			"limit":         5,
			"offset":        0,
		},
	})
	require.NoError(t, err)
	assert.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var output nvidiaSearchOutput
	err = json.Unmarshal(data, &output)
	require.NoError(t, err)

	want := nvidiaSearchOutput{
		Total: 27,
		Data: []nvidiaJobSummary{
			{Title: "Senior Software Golang Kubernetes Engineer", ExternalPath: "/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916", LocationsText: "3 Locations", PostedOn: "Posted 30+ Days Ago"},
			{Title: "Senior Software Golang Kubernetes Engineer", ExternalPath: "/job/Israel-Tel-Aviv/Senior-Software-Golang-Kubernetes-Engineer_JR2016621", LocationsText: "3 Locations", PostedOn: "Posted 30+ Days Ago"},
			{Title: "Senior Software Engineer, GoLang - DSX MaxQ", ExternalPath: "/job/US-CA-Santa-Clara/Senior-Software-Engineer--GoLang---DSX-MaxQ_JR2017740-1", LocationsText: "3 Locations", PostedOn: "Posted 4 Days Ago"},
			{Title: "Senior C++ Software Engineer - Chip Design Tools", ExternalPath: "/job/US-CA-Santa-Clara/Senior-C---Software-Engineer---Chip-Design-Tools_JR2009389", LocationsText: "4 Locations", PostedOn: "Posted 30+ Days Ago"},
			{Title: "Senior Full Stack Software Engineer - DGX Cloud", ExternalPath: "/job/US-NC-Remote/Senior-Full-Stack-Software-Engineer---DGX-Cloud_JR2017922", LocationsText: "5 Locations", PostedOn: "Posted 15 Days Ago"},
			{Title: "Senior System Software Engineer – GeForce NOW Cloud", ExternalPath: "/job/US-CA-Santa-Clara/Senior-System-Software-Engineer---GeForce-NOW-Cloud_JR2018465", LocationsText: "US, CA, Santa Clara", PostedOn: "Posted 20 Days Ago"},
			{Title: "Senior System Software Engineer for Cloud – GeForce NOW", ExternalPath: "/job/US-CA-Santa-Clara/Senior-System-Software-Engineer-for-Cloud---GeForce-NOW_JR2013549", LocationsText: "US, CA, Santa Clara", PostedOn: "Posted 30+ Days Ago"},
			{Title: "Senior Software Development Tech Lead – AI Developer Experiences", ExternalPath: "/job/China-Shanghai/Senior-Software-Development-Tech-Lead---AI-Developer-Experiences_JR2017783", LocationsText: "China, Shanghai", PostedOn: "Posted 30+ Days Ago"},
			{Title: "Senior C++ Software Engineer - Infrastructure Tools", ExternalPath: "/job/US-CA-Santa-Clara/Senior-C---Software-Engineer---Infrastructure-Tools_JR2018693", LocationsText: "4 Locations", PostedOn: "Posted 30+ Days Ago"},
			{Title: "Senior DevOps Engineer", ExternalPath: "/job/India-Pune/Senior-DevOps-Engineer_JR2019008", LocationsText: "India, Pune", PostedOn: "Posted 20 Days Ago"},
			{Title: "Senior Software Engineer, Cloud Automation", ExternalPath: "/job/Poland-Warsaw/Senior-Software-Engineer--Cloud-Automation_JR2019580-1", LocationsText: "2 Locations", PostedOn: "Posted 20 Days Ago"},
			{Title: "Senior Data Backend Engineer", ExternalPath: "/job/US-OR-Hillsboro/Senior-Data-Backend-Engineer_JR2020354", LocationsText: "2 Locations", PostedOn: "Posted 2 Days Ago"},
			{Title: "Senior Full-Stack Software Engineer - VLSI Tools", ExternalPath: "/job/US-CA-Santa-Clara/Senior-Full-Stack-Software-Engineer---VLSI-Tools_JR2012368", LocationsText: "4 Locations", PostedOn: "Posted 23 Days Ago"},
			{Title: "Senior Infrastructure Engineer – Bazel Remote Execution", ExternalPath: "/job/US-CA-Santa-Clara/Senior-Infrastructure-Engineer---Bazel-Remote-Execution_JR2019387", LocationsText: "US, CA, Santa Clara", PostedOn: "Posted 16 Days Ago"},
			{Title: "Senior Manager, Engineering - AI Developer Tools", ExternalPath: "/job/US-CA-Santa-Clara/Senior-Manager--Engineering---AI-Developer-Tools_JR2019726-1", LocationsText: "2 Locations", PostedOn: "Posted 15 Days Ago"},
			{Title: "Senior Data Engineer - Financial Transactions & Automation", ExternalPath: "/job/US-CA-Santa-Clara/Senior-Data-Engineer---Financial-Transactions---Automation_JR2009512", LocationsText: "2 Locations", PostedOn: "Posted 30+ Days Ago"},
			{Title: "Senior System Software Engineer for Cloud – GeForce NOW Platform", ExternalPath: "/job/US-CA-Santa-Clara/Senior-System-Software-Engineer-for-Cloud---GeForce-NOW-Platform_JR2018467", LocationsText: "US, CA, Santa Clara", PostedOn: "Posted 30+ Days Ago"},
			{Title: "Senior Systems Software Engineer, Kubernetes Scale - DGX Cloud", ExternalPath: "/job/Germany-Remote/Senior-Systems-Software-Engineer--Kubernetes-Scale---DGX-Cloud_JR2020234-1", LocationsText: "6 Locations", PostedOn: "Posted 6 Days Ago"},
			{Title: "Senior Cloud Software Engineer", ExternalPath: "/job/India-Bengaluru/Senior-Cloud-Software-Engineer_JR2020094", LocationsText: "2 Locations", PostedOn: "Posted 2 Days Ago"},
			{Title: "Systems Software Engineer, Kubernetes Scale - DGX Cloud", ExternalPath: "/job/Germany-Remote/Systems-Software-Engineer--Kubernetes-Scale---DGX-Cloud_JR2020236", LocationsText: "6 Locations", PostedOn: "Posted 5 Days Ago"},
		},
	}
	assert.Equal(t, want, output)
}

func TestNvidiaGetJobDetailE2E(t *testing.T) {
	clientSession, _ := testNvidiaMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "nvidia_get_job_detail",
		Arguments: map[string]any{
			"external_path": "/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916",
		},
	})
	require.NoError(t, err)
	assert.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var output nvidiaDetailOutput
	err = json.Unmarshal(data, &output)
	require.NoError(t, err)

	want := nvidiaDetailOutput{
		Title:               "Senior Software Golang Kubernetes Engineer",
		Description:         "NVIDIA Networking is looking for an excellent Software Developer to work on NVIDIA cloud platforms based on Kubernetes. We are seeking an experienced engineer who is deeply technical, hands-on, and has a wide system view. You will design, build and deploy high-performance and scalable clouds based on NVIDIA's superior ConnectX and Bluefield NICs and SpectrumX AI platform. We want to grow our teams with the smartest people in the world. If you're creative and autonomous, we want to hear from you!\n\n*What you'll be doing:*\n\n* \n\nDesign and implement new features to accelerate Network and Storage\n\n* \n\nWork closely with open source communities, participate in the relevant working groups\n\n* \n\nWork with different teams across NVIDIA\n\n* \n\nMentor members of the team, enabling them to deliver high-quality software\n\n*What we need to see:*\n\n* \n\nBSc in Computer Science or equivalent program\n\n* \n\n5+ years of hands-on experience in software development, preferably with Python/Golang\n\n* \n\nHighly motivated with strong communication skills, the ability to work successfully with multi-functional teams, developers, and architects\n\n* \n\nCoordinate effectively across organizational boundaries and geographies\n\n* \n\nStrong self-initiative, independence, and flexibility to a new technology\n\n* \n\nDeep understanding of network protocols, virtualization, and containers\n\n* \n\nStrong background in designing, implementing, and debugging complex software\n\n* \n\nHands-on experience with Kubernetes\n\n*Ways to stand out from the crowd:*\n\n* \n\nExperience with working on open source projects\n\n* \n\nBackground with SR-IOV, DPDK, ROCE technologies\n\n* \n\nExperience in developing Kubernetes Operators, CSI plugins, CNI Plugins\n\nWe are an equal opportunity employer and value diversity at our company. We do not discriminate on the basis of race, religion, color, national origin, sex, gender, gender expression, sexual orientation, age, marital status, veteran status, or disability status. We will ensure that individuals with disabilities are provided reasonable accommodation to participate in the job application or interview process, perform essential job functions, and receive other benefits and privileges of employment.",
		Location:            "Israel, Yokneam",
		AdditionalLocations: []string{"Israel, Raanana", "Israel, Tel Aviv"},
		PostedOn:            "Posted 30+ Days Ago",
		TimeType:            "Full time",
		JobReqID:            "JR2015916",
		ExternalURL:         "https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite/job/Israel-Yokneam/Senior-Software-Golang-Kubernetes-Engineer_JR2015916",
	}
	assert.Equal(t, want, output)
}

func TestNvidiaSearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession, _ := testNvidiaMCPClientServer(t)

	// A value outside a property's enum is rejected by the SDK's
	// input-schema validation before the handler runs, as an IsError
	// tool result.
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "nvidia_search_jobs",
		Arguments: map[string]any{
			"keyword":  "golang",
			"country":  "Taiwan",
			"job_type": "valueNotInEnum",
		},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, `validating /properties/job_type: enum`)
}

func TestNvidiaHTTPToMCPResponse(t *testing.T) {
	in := nvidia.JobsResponse{
		Total: 2,
		JobPostings: []nvidia.JobSummary{
			{
				Title:         nvidia.NewOptString("t1"),
				ExternalPath:  nvidia.NewOptString("p1"),
				LocationsText: nvidia.NewOptString("l1"),
				PostedOn:      nvidia.NewOptString("d1"),
			},
			{
				Title:         nvidia.NewOptString("t2"),
				ExternalPath:  nvidia.NewOptString("p2"),
				LocationsText: nvidia.NewOptString("l2"),
				PostedOn:      nvidia.NewOptString("d2"),
			},
		},
	}
	got := nvidiaHTTPToMCPResponse(&in)

	want := &nvidiaSearchOutput{
		Total: 2,
		Data: []nvidiaJobSummary{
			{Title: "t1", ExternalPath: "p1", LocationsText: "l1", PostedOn: "d1"},
			{Title: "t2", ExternalPath: "p2", LocationsText: "l2", PostedOn: "d2"},
		},
	}
	assert.Equal(t, want, got)
}

func TestNvidiaHTTPToMCPDetail(t *testing.T) {
	in := nvidia.JobDetailResponse{
		JobPostingInfo: nvidia.JobPostingInfo{
			Title:               "t",
			JobDescription:      "<p>d</p>",
			Location:            nvidia.NewOptString("l"),
			AdditionalLocations: []string{"al"},
			PostedOn:            nvidia.NewOptString("po"),
			TimeType:            nvidia.NewOptString("tt"),
			JobReqId:            nvidia.NewOptString("id"),
			ExternalUrl:         nvidia.NewOptString("url"),
		},
	}
	got := nvidiaHTTPToMCPDetail(&in)

	want := &nvidiaDetailOutput{
		Title:               "t",
		Description:         "d",
		Location:            "l",
		AdditionalLocations: []string{"al"},
		PostedOn:            "po",
		TimeType:            "tt",
		JobReqID:            "id",
		ExternalURL:         "url",
	}
	assert.Equal(t, want, got)
}

func TestNvidiaMCPToHTTPRequest(t *testing.T) {
	in := nvidiaSearchInput{
		Keyword:      "golang",
		JobCategory:  "Engineering",
		JobType:      "Regular Employee",
		TimeType:     "Full time",
		LocationType: "Remote",
		Country:      "Taiwan",
		Site:         "Taiwan, Taipei",
		Limit:        5,
		Offset:       10,
	}
	got, err := nvidiaMCPToHTTPRequest(&in)
	require.NoError(t, err)

	want := &nvidia.JobsRequest{
		AppliedFacets: nvidia.AppliedFacets{
			JobFamilyGroup:     []nvidia.AppliedFacetsJobFamilyGroupItem{nvidia.JobCategoryIDs["Engineering"]},
			WorkerSubType:      []nvidia.AppliedFacetsWorkerSubTypeItem{nvidia.JobTypeIDs["Regular Employee"]},
			TimeType:           []nvidia.AppliedFacetsTimeTypeItem{nvidia.TimeTypeIDs["Full time"]},
			LocationHierarchy2: []nvidia.AppliedFacetsLocationHierarchy2Item{nvidia.LocationTypeIDs["Remote"]},
			LocationHierarchy1: []nvidia.AppliedFacetsLocationHierarchy1Item{nvidia.CountryIDs["Taiwan"]},
			Locations:          []nvidia.AppliedFacetsLocationsItem{nvidia.SiteIDs["Taiwan, Taipei"]},
		},
		Limit:      5,
		Offset:     10,
		SearchText: "golang",
	}
	assert.Equal(t, want, got)
}

func TestNvidiaMCPToHTTPRequestMinimal(t *testing.T) {
	got, err := nvidiaMCPToHTTPRequest(&nvidiaSearchInput{Keyword: "golang", Country: "Taiwan"})
	require.NoError(t, err)

	want := &nvidia.JobsRequest{
		AppliedFacets: nvidia.AppliedFacets{
			LocationHierarchy1: []nvidia.AppliedFacetsLocationHierarchy1Item{nvidia.CountryIDs["Taiwan"]},
		},
		Limit:      20,
		SearchText: "golang",
	}
	assert.Equal(t, want, got)
}

func TestNvidiaMCPToHTTPRequestMissingRequired(t *testing.T) {
	cases := []struct {
		name string
		in   nvidiaSearchInput
		want string
	}{
		{"all empty", nvidiaSearchInput{}, "keyword is required"},
		{"filters only", nvidiaSearchInput{Country: "Taiwan", JobCategory: "Engineering"}, "keyword is required"},
		{"keyword only", nvidiaSearchInput{Keyword: "golang"}, `invalid country ""`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := nvidiaMCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestNvidiaMCPToHTTPRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   nvidiaSearchInput
		want string
	}{
		{"job_category", nvidiaSearchInput{Keyword: "x", Country: "Taiwan", JobCategory: "valueNotInEnum"}, `invalid job_category "valueNotInEnum"`},
		{"job_type", nvidiaSearchInput{Keyword: "x", Country: "Taiwan", JobType: "valueNotInEnum"}, `invalid job_type "valueNotInEnum"`},
		{"time_type", nvidiaSearchInput{Keyword: "x", Country: "Taiwan", TimeType: "valueNotInEnum"}, `invalid time_type "valueNotInEnum"`},
		{"location_type", nvidiaSearchInput{Keyword: "x", Country: "Taiwan", LocationType: "valueNotInEnum"}, `invalid location_type "valueNotInEnum"`},
		{"country", nvidiaSearchInput{Keyword: "x", Country: "valueNotInEnum"}, `invalid country "valueNotInEnum"`},
		{"site", nvidiaSearchInput{Keyword: "x", Country: "Taiwan", Site: "valueNotInEnum"}, `invalid site "valueNotInEnum"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := nvidiaMCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
