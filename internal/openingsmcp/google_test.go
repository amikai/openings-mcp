package openingsmcp

import (
	"encoding/json"
	"testing"

	"github.com/amikai/openings-mcp/internal/provider/google"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGoogleMCPClientServer(t *testing.T) (*mcp.ClientSession, *mcp.ServerSession) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	srv := google.NewMockServer()
	t.Cleanup(srv.Close)
	client := google.NewClient(srv.URL, srv.Client())
	RegisterGoogle(server, client)

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

func TestRegisterGoogle(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	client := google.NewClient("https://www.google.com/about/careers/applications", nil)
	RegisterGoogle(server, client)

	assertTools(t, server, "google_search_jobs", "google_get_job_detail")
}

func TestGoogleSearchJobsE2E(t *testing.T) {
	clientSession, _ := testGoogleMCPClientServer(t)

	res, err := clientSession.ListTools(t.Context(), nil)
	require.NoError(t, err)

	tool := findTool(res.Tools, "google_search_jobs")
	require.NotNil(t, tool)

	schema, ok := tool.InputSchema.(map[string]any)
	require.True(t, ok)

	// Full golden schema: every searchJobs query parameter from openapi.yaml,
	// with keyword and location required.
	want := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword": map[string]any{
				"type":        "string",
				"description": "Free-text search query matched against job title and description.",
			},
			"location": map[string]any{
				"type":        "string",
				"description": `Location filter; a city, region, or country name (e.g. "Taiwan", "New York, NY, USA").`,
			},
			"has_remote": map[string]any{
				"type":        "boolean",
				"description": "When true, restricts results to jobs marked Remote eligible.",
			},
			"target_level": map[string]any{
				"type":        "string",
				"description": "Experience level filter.",
				"enum":        []any{"EARLY", "MID", "ADVANCED", "INTERN_AND_APPRENTICE", "DIRECTOR_PLUS"},
			},
			"skills": map[string]any{
				"type":        "string",
				"description": "Free-text skills and qualifications filter.",
			},
			"degree": map[string]any{
				"type":        "string",
				"description": "Minimum education level filter.",
				"enum":        []any{"PURSUING_DEGREE", "ASSOCIATE", "BACHELORS", "MASTERS", "PHD"},
			},
			"employment_type": map[string]any{
				"type":        "string",
				"description": "Job type filter.",
				"enum":        []any{"FULL_TIME", "PART_TIME", "TEMPORARY", "INTERN"},
			},
			"company": map[string]any{
				"type":        "string",
				"description": "Organization (sub-company) filter.",
				"enum":        []any{"DeepMind", "GFiber", "Google", "Verily Life Sciences", "Waymo", "Wing", "YouTube"},
			},
			"sort_by": map[string]any{
				"type":        "string",
				"description": "Sort order. Defaults to relevance; date sorts newest first.",
				"enum":        []any{"relevance", "date"},
			},
			"page": map[string]any{
				"type":        "integer",
				"description": "1-based page number; 20 results per page.",
				"minimum":     float64(1),
			},
		},
		"required":             []any{"keyword", "location"},
		"additionalProperties": false,
	}
	assert.Equal(t, want, schema)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "google_search_jobs",
		Arguments: map[string]any{"keyword": "software engineer", "location": "Taiwan"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got googleSearchOutput
	require.NoError(t, json.Unmarshal(data, &got))

	wantResp := &googleSearchOutput{
		Data: []googleJobSummary{
			{
				ID: "106863362666570438", URL: "https://www.google.com/about/careers/applications/jobs/results/106863362666570438",
				Title: "Software Engineer, GPU System Software", Company: "Google", Location: "Taipei, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor's degree in Electrical Engineering, Computer Science, a related technical field, or equivalent practical experience.",
					"2 years of experience in software, firmware and driver development in languages such as C/C++.",
				},
			},
			{
				ID: "82975510480462534", URL: "https://www.google.com/about/careers/applications/jobs/results/82975510480462534",
				Title: "SoC Product Engineer, Google Cloud", Company: "Google", Location: "Zhubei, Zhubei City, Hsinchu County, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor's degree in Electrical Engineering, Computer Engineering, Computer Science, or a related field, or equivalent practical experience.",
					"8 years of experience with industry-standard tools, languages and methodologies relevant to the production Silicon SoC’s or ASIC’s, including product engineering or test engineering.",
				},
			},
			{
				ID: "81991011634422470", URL: "https://www.google.com/about/careers/applications/jobs/results/81991011634422470",
				Title: "Silicon Physical Design CAD Engineer", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor's degree in Electrical Engineering, a similar field, or equivalent practical experience.",
					"4 years of experience in scripting languages such as Perl, TCL, Shell, or Python.",
				},
			},
			{
				ID: "143985660506579654", URL: "https://www.google.com/about/careers/applications/jobs/results/143985660506579654",
				Title: "Senior Signal and Power Integrity Engineer, Pixel", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor’s degree in Electrical Engineering, Computer Engineering, or related field or equivalent practical experience.",
					"4 years of work experience in Consumer Electronics/embedded system design or electrical systems development.",
					"4 years of experience with signal and power integrity simulation of system boards, including SoC/chipset, Memory technology, and other communication interfaces.",
				},
			},
			{
				ID: "78019461818262214", URL: "https://www.google.com/about/careers/applications/jobs/results/78019461818262214",
				Title: "Software Engineering Manager, Release Engineering, Google Cloud Platforms", Company: "Google", Location: "Taipei, Taiwan",
				ExperienceLevel: "Advanced",
				MinimumQualifications: []string{
					"Bachelor’s degree, or equivalent practical experience.",
					"8 years of experience in software development in C++, GO, or Python.",
					"3 years of experience across testing, maintaining, or launching software products.",
					"3 years of experience in a technical leadership role.",
					"2 years of experience in a people management or team leadership role.",
				},
			},
			{
				ID: "112498222319968966", URL: "https://www.google.com/about/careers/applications/jobs/results/112498222319968966",
				Title: "Firmware Engineer, Pixel Systems Power", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor's degree in Computer Science, Electrical Engineering, Computer Engineering, a related technical field, or equivalent practical experience.",
					"5 years of experience in embedded development.",
					"Experience in programming in C or C++.",
				},
			},
			{
				ID: "123677700068909766", URL: "https://www.google.com/about/careers/applications/jobs/results/123677700068909766",
				Title: "Supplier Quality Engineer, Memory", Company: "Google", Location: "Taipei, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor's degree in Engineering (e.g., Quality, Electrical) or equivalent practical experience.",
					"5 years of experience in quality, reliability, product, or test engineering, specifically focused on DRAM technologies (e.g., DDR and LPDDR components, and DIMM modules).",
					"Ability to travel up to 20% of the time as needed.",
				},
			},
			{
				ID: "77507329917887174", URL: "https://www.google.com/about/careers/applications/jobs/results/77507329917887174",
				Title: "Software Engineering Manager, Pixel Camera", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Advanced",
				MinimumQualifications: []string{
					"Bachelor’s degree, or equivalent practical experience.",
					"8 years of experience in software development.",
					"3 years of experience in a technical leadership role.",
					"2 years of experience in a people management or team leadership role.",
					"Experience with Andriod or mobile app development.",
					"Experience using Java or Kotlin programming language.",
				},
			},
			{
				ID: "126487124076569286", URL: "https://www.google.com/about/careers/applications/jobs/results/126487124076569286",
				Title: "Developer Relations Engineer, Android, Play, Games", Company: "Google", Location: "Taipei, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor's degree in Computer Science, a similar technical field, or equivalent practical experience.",
					"3 years of work experience in a technical role (e.g., software engineering, solutions consultant, etc.) or equivalent technical experience.",
				},
			},
			{
				ID: "110315781933146822", URL: "https://www.google.com/about/careers/applications/jobs/results/110315781933146822",
				Title: "Software Engineer, Android Apps, Pixel", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor’s degree or equivalent practical experience.",
					"2 years of experience with Android software development in Kotlin or Java.",
				},
			},
			{
				ID: "120702421604147910", URL: "https://www.google.com/about/careers/applications/jobs/results/120702421604147910",
				Title: "Firmware Engineer, Wi-Fi, Pixel Connectivity", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor's degree in Computer Science, Electrical Engineering, Computer Engineering, or a related technical field, or equivalent practical experience.",
					"5 years of experience in coding with a general purpose programming language (e.g., C/C++).",
					"Experience with Wi-Fi drivers, firmware or framework development.",
				},
			},
			{
				ID: "84295530024182470", URL: "https://www.google.com/about/careers/applications/jobs/results/84295530024182470",
				Title: "Senior Product Design Engineer, Trackers and Home", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor's degree in Mechanical Engineering, Product Design, or a related field, or equivalent practical experience.",
					"5 years of experience designing mechanical components such as plastic or metal parts, mechanical assemblies, printed circuit boards, or flexes.",
					"5 years of experience using computer-aided design (CAD) tools such as 3D MCAD, NX, Creo, or Solidworks.",
				},
			},
			{
				ID: "76806086312501958", URL: "https://www.google.com/about/careers/applications/jobs/results/76806086312501958",
				Title: "Software Engineer, Apps, Pixel", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Early",
				MinimumQualifications: []string{
					"Bachelor’s degree or equivalent practical experience.",
					"1 year of experience with software development in one or more programming languages (e.g., Java, Kotlin, C++, Python).",
					"1 year of experience with data structures or algorithms.",
				},
			},
			{
				ID: "92642283567882950", URL: "https://www.google.com/about/careers/applications/jobs/results/92642283567882950",
				Title: "Test Engineer, User Experience Quality", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor's degree in computer science, electrical engineering, or equivalent practical experience.",
					"3 years of experience in coding (e.g., Python or Java), developing test methodologies, writing test plans, creating test cases, and debugging.",
					"2 years of experience with developing infrastructure, or experience with compute technologies, mobile application development.",
				},
			},
			{
				ID: "74113863372415686", URL: "https://www.google.com/about/careers/applications/jobs/results/74113863372415686",
				Title: "Staff Hardware System Engineer, Home and Health", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Advanced",
				MinimumQualifications: []string{
					"Bachelor's degree in Electrical Engineering, Computer Engineering, Physics, a related field, or equivalent practical experience.",
					"6 years of experience working in consumer electronics system design.",
					"Experience in one of the following domains, such as architecture, power/battery/Soc, interfaces, or analog/camera/display engineering.",
				},
			},
			{
				ID: "103770758580183750", URL: "https://www.google.com/about/careers/applications/jobs/results/103770758580183750",
				Title: "CPU RTL Design Engineer", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor’s degree in Electrical Engineering, Computer Engineering, Computer Science, or a related field, or equivalent practical experience.",
					"4 years of experience in high-performance CPU or AI accelerator logic design, RTL design or integration, including microarchitecture definition and PPA optimizations.",
					"Experience in CPU, and cache subsystem integration with SOCs.",
				},
			},
			{
				ID: "87506232826307270", URL: "https://www.google.com/about/careers/applications/jobs/results/87506232826307270",
				Title: "Software Engineer Manager II, Silicon Tools", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
				ExperienceLevel: "Advanced",
				MinimumQualifications: []string{
					"Bachelor's degree or equivalent practical experience.",
					"8 years of experience in software development, with experience in building and scaling internal/external tools within a engineering organization.",
					"2 years of experience in a people management or team leadership role.",
				},
			},
			{
				ID: "109039274703102662", URL: "https://www.google.com/about/careers/applications/jobs/results/109039274703102662",
				Title: "Product Quality Engineer, Global Hardware Quality and Reliability", Company: "Google", Location: "Taipei, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor's degree in Mechanical Engineering, Industrial Engineering, Materials Engineering, a related engineering discipline, or equivalent practical experience.",
					"5 years of experience in Quality/Reliability/Production operation, with direct experience in New Product development and qualification.",
				},
			},
			{
				ID: "107666671874777798", URL: "https://www.google.com/about/careers/applications/jobs/results/107666671874777798",
				Title: "Test Development Engineer, Global Manufacturing Engineering", Company: "Google", Location: "Taipei, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor’s degree in Electrical Engineering, Computer Science, or a related technical field, or equivalent practical experience.",
					"5 years of experience in software development.",
					"Experience developing and validating Python scripts to automate manufacturing test processes.",
				},
			},
			{
				ID: "141781579927036614", URL: "https://www.google.com/about/careers/applications/jobs/results/141781579927036614",
				Title: "Senior Software Engineer, Emerging On-prem AI Infrastructure", Company: "Google", Location: "Taipei, Taiwan",
				ExperienceLevel: "Mid",
				MinimumQualifications: []string{
					"Bachelor’s degree or equivalent practical experience.",
					"5 years of experience in software engineering, and with building tools focused on infrastructure operations, reliability, and architectural integrity.",
					"2 years of experience in software design and architecture, within a large-scale cloud infrastructure provider.",
					"Experience in testing, maintaining, and launching software products or infrastructure systems, and in debugging, troubleshooting, and diagnosing issues within distributed systems.",
				},
			},
		},
	}
	assert.Equal(t, wantResp, &got)
}

func TestGoogleSearchJobsMissingRequiredE2E(t *testing.T) {
	clientSession, _ := testGoogleMCPClientServer(t)

	// Missing required params are rejected by the SDK's input-schema
	// validation before the handler runs, as an IsError tool result.
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"no keyword", map[string]any{"location": "Taiwan"}, `validating "arguments": validating root: required: missing properties: ["keyword"]`},
		{"no location", map[string]any{"keyword": "software engineer"}, `validating "arguments": validating root: required: missing properties: ["location"]`},
		{"empty args", map[string]any{}, `validating "arguments": validating root: required: missing properties: ["keyword" "location"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
				Name:      "google_search_jobs",
				Arguments: tc.args,
			})
			require.NoError(t, err)
			require.True(t, callRes.IsError)
			text, ok := callRes.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.Equal(t, tc.want, text.Text)
		})
	}
}

func TestGoogleSearchJobsInvalidEnumE2E(t *testing.T) {
	clientSession, _ := testGoogleMCPClientServer(t)

	// A value outside a property's enum is rejected by the SDK's
	// input-schema validation before the handler runs, as an IsError
	// tool result.
	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "google_search_jobs",
		Arguments: map[string]any{"keyword": "software engineer", "location": "Taiwan", "employment_type": "valueNotInEnum"},
	})
	require.NoError(t, err)
	require.True(t, callRes.IsError)
	text, ok := callRes.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, `validating "arguments": validating root: validating /properties/employment_type: enum: valueNotInEnum does not equal any of: [FULL_TIME PART_TIME TEMPORARY INTERN]`, text.Text)
}

func TestGoogleGetJobDetailE2E(t *testing.T) {
	clientSession, _ := testGoogleMCPClientServer(t)

	callRes, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "google_get_job_detail",
		Arguments: map[string]any{"job_id": "106863362666570438"},
	})
	require.NoError(t, err)
	require.False(t, callRes.IsError)

	data, err := json.Marshal(callRes.StructuredContent)
	require.NoError(t, err)
	var got googleDetailOutput
	require.NoError(t, json.Unmarshal(data, &got))

	want := &googleDetailOutput{
		ID:       "106863362666570438",
		URL:      "https://www.google.com/about/careers/applications/jobs/results/106863362666570438",
		Title:    "Software Engineer, GPU System Software",
		Company:  "Google",
		Location: "Taipei, Taiwan",
		About: `About the job

Google's software engineers develop the next-generation technologies that change how billions of users connect, explore, and interact with information and one another. Our products need to handle information at massive scale, and extend well beyond web search. We're looking for engineers who bring fresh ideas from all areas, including information retrieval, distributed computing, large-scale system design, networking and data storage, security, artificial intelligence, natural language processing, UI design and mobile; the list goes on and is growing every day. As a software engineer, you will work on a specific project critical to Google’s needs with opportunities to switch teams and projects as you and our fast-paced business grow and evolve. We need our engineers to be versatile, display leadership qualities and be enthusiastic to take on new problems across the full-stack as we continue to push technology forward.

The GPU System Software team is responsible for building GPU compute solutions that power various Google services like Google Cloud, YouTube, Deep Mind, etc. We also maintain the systems deployed in the data centers with reliability monitoring services, kernel rollouts, firmware and driver upgrades.
As a Software Engineering Manager for the Graphics Processing Unit (GPU) Platforms Software team, you will lead the team and work with many cross-functional teams (e.g., hardware, system, data center deployment) to provide the foundational AI infrastructure that enables the AI applications for Google and Cloud customers.`,
		Qualifications: `Minimum qualifications:


Bachelor's degree in Electrical Engineering, Computer Science, a related technical field, or equivalent practical experience.

2 years of experience in software, firmware and driver development in languages such as C/C++.

Preferred qualifications:


Master's degree or PhD in Electrical Engineering, Computer Science, or a related technical field.

Experience designing and developing device drivers for peripherals like GPUs, PCIe Switches and connectivity buses like I2C, USB, PCIe.

Experience using revision control systems like Git and Perforce.

Experience in doing the debug, development, and testing work in the linux environment.

Expertise in server system architecture, networking or embedded system.

Expertise in problem-solving technical innovation.`,
		Responsibilities: `Responsibilities


Design, develop and maintain the system software stack for GPU system software.

Help identify dependencies in cross-functional teams and drive NPI execution with a peculiar focus on development velocity and quality.

Drive system software integration to enable next generation GPU accelerators for Google data center.

Manage data center GPUs software/kernel driver/firmware development, integration and validation.

Develop test suites that enable unit, integration and system level testing of our system software.`,
	}
	assert.Equal(t, want, &got)
}

func TestGoogleHTTPToMCPResponse(t *testing.T) {
	in := google.JobsResponse{
		Jobs: []google.Job{
			{ID: "1", Title: "t1", Company: "Google", Location: "Taipei, Taiwan", Remote: true, ExperienceLevel: "Mid", MinimumQualifications: []string{"q1", "q2"}},
			{Title: "t2"},
		},
	}
	got := googleHTTPToMCPResponse(&in)

	want := &googleSearchOutput{
		Data: []googleJobSummary{
			{ID: "1", URL: "https://www.google.com/about/careers/applications/jobs/results/1", Title: "t1", Company: "Google", Location: "Taipei, Taiwan", Remote: true, ExperienceLevel: "Mid", MinimumQualifications: []string{"q1", "q2"}},
			{Title: "t2"},
		},
	}
	assert.Equal(t, want, got)
}

func TestGoogleHTTPToMCPDetail(t *testing.T) {
	in := google.JobDetailResponse{
		ID:               "7",
		Title:            "t",
		Company:          "Google",
		Location:         "Taipei, Taiwan",
		Remote:           true,
		About:            "a",
		Qualifications:   "q",
		Responsibilities: "r",
	}
	got := googleHTTPToMCPDetail(&in)

	want := &googleDetailOutput{
		ID:               "7",
		URL:              "https://www.google.com/about/careers/applications/jobs/results/7",
		Title:            "t",
		Company:          "Google",
		Location:         "Taipei, Taiwan",
		Remote:           true,
		About:            "a",
		Qualifications:   "q",
		Responsibilities: "r",
	}
	assert.Equal(t, want, got)
}

func TestGoogleMCPToHTTPRequest(t *testing.T) {
	in := googleSearchInput{
		Keyword:        "software engineer",
		Location:       "Taiwan",
		HasRemote:      true,
		TargetLevel:    "MID",
		Skills:         "Python",
		Degree:         "MASTERS",
		EmploymentType: "FULL_TIME",
		Company:        "Google",
		SortBy:         "date",
		Page:           2,
	}
	got, err := googleMCPToHTTPRequest(&in)
	require.NoError(t, err)

	want := &google.JobsRequest{
		Query:          "software engineer",
		Locations:      []string{"Taiwan"},
		HasRemote:      true,
		TargetLevels:   []string{"MID"},
		Skills:         "Python",
		Degrees:        []string{"MASTERS"},
		EmploymentType: []string{"FULL_TIME"},
		Companies:      []string{"Google"},
		SortBy:         "date",
		Page:           2,
	}
	assert.Equal(t, want, got)
}

func TestGoogleMCPToHTTPRequestMinimal(t *testing.T) {
	got, err := googleMCPToHTTPRequest(&googleSearchInput{Keyword: "software engineer", Location: "Taiwan"})
	require.NoError(t, err)

	want := google.JobsRequest{
		Query:     "software engineer",
		Locations: []string{"Taiwan"},
	}
	assert.Equal(t, want, *got)
}

func TestGoogleMCPToHTTPRequestMissingRequired(t *testing.T) {
	cases := []struct {
		name string
		in   googleSearchInput
		want string
	}{
		{"all empty", googleSearchInput{}, "keyword is required"},
		{"filters only", googleSearchInput{Location: "Taiwan", Company: "Google", Page: 2}, "keyword is required"},
		{"keyword only", googleSearchInput{Keyword: "software engineer"}, "location is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := googleMCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestGoogleMCPToHTTPRequestInvalidLabels(t *testing.T) {
	cases := []struct {
		name string
		in   googleSearchInput
		want string
	}{
		{"target_level", googleSearchInput{Keyword: "x", Location: "Taiwan", TargetLevel: "JUNIOR"}, `invalid target_level "JUNIOR"`},
		{"degree", googleSearchInput{Keyword: "x", Location: "Taiwan", Degree: "HIGH_SCHOOL"}, `invalid degree "HIGH_SCHOOL"`},
		{"employment_type", googleSearchInput{Keyword: "x", Location: "Taiwan", EmploymentType: "CONTRACT"}, `invalid employment_type "CONTRACT"`},
		{"company", googleSearchInput{Keyword: "x", Location: "Taiwan", Company: "Alphabet"}, `invalid company "Alphabet"`},
		{"sort_by", googleSearchInput{Keyword: "x", Location: "Taiwan", SortBy: "newest"}, `invalid sort_by "newest"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := googleMCPToHTTPRequest(&tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
