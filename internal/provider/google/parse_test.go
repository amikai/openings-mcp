package google

import (
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var wantJobs = []Job{
	{
		ID: "106863362666570438", Title: "Software Engineer, GPU System Software", Company: "Google", Location: "Taipei, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor's degree in Electrical Engineering, Computer Science, a related technical field, or equivalent practical experience.",
			"2 years of experience in software, firmware and driver development in languages such as C/C++.",
		},
	},
	{
		ID: "82975510480462534", Title: "SoC Product Engineer, Google Cloud", Company: "Google", Location: "Zhubei, Zhubei City, Hsinchu County, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor's degree in Electrical Engineering, Computer Engineering, Computer Science, or a related field, or equivalent practical experience.",
			"8 years of experience with industry-standard tools, languages and methodologies relevant to the production Silicon SoC’s or ASIC’s, including product engineering or test engineering.",
		},
	},
	{
		ID: "81991011634422470", Title: "Silicon Physical Design CAD Engineer", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor's degree in Electrical Engineering, a similar field, or equivalent practical experience.",
			"4 years of experience in scripting languages such as Perl, TCL, Shell, or Python.",
		},
	},
	{
		ID: "143985660506579654", Title: "Senior Signal and Power Integrity Engineer, Pixel", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor’s degree in Electrical Engineering, Computer Engineering, or related field or equivalent practical experience.",
			"4 years of work experience in Consumer Electronics/embedded system design or electrical systems development.",
			"4 years of experience with signal and power integrity simulation of system boards, including SoC/chipset, Memory technology, and other communication interfaces.",
		},
	},
	{
		ID: "78019461818262214", Title: "Software Engineering Manager, Release Engineering, Google Cloud Platforms", Company: "Google", Location: "Taipei, Taiwan",
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
		ID: "112498222319968966", Title: "Firmware Engineer, Pixel Systems Power", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor's degree in Computer Science, Electrical Engineering, Computer Engineering, a related technical field, or equivalent practical experience.",
			"5 years of experience in embedded development.",
			"Experience in programming in C or C++.",
		},
	},
	{
		ID: "123677700068909766", Title: "Supplier Quality Engineer, Memory", Company: "Google", Location: "Taipei, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor's degree in Engineering (e.g., Quality, Electrical) or equivalent practical experience.",
			"5 years of experience in quality, reliability, product, or test engineering, specifically focused on DRAM technologies (e.g., DDR and LPDDR components, and DIMM modules).",
			"Ability to travel up to 20% of the time as needed.",
		},
	},
	{
		ID: "77507329917887174", Title: "Software Engineering Manager, Pixel Camera", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
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
		ID: "126487124076569286", Title: "Developer Relations Engineer, Android, Play, Games", Company: "Google", Location: "Taipei, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor's degree in Computer Science, a similar technical field, or equivalent practical experience.",
			"3 years of work experience in a technical role (e.g., software engineering, solutions consultant, etc.) or equivalent technical experience.",
		},
	},
	{
		ID: "110315781933146822", Title: "Software Engineer, Android Apps, Pixel", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor’s degree or equivalent practical experience.",
			"2 years of experience with Android software development in Kotlin or Java.",
		},
	},
	{
		ID: "120702421604147910", Title: "Firmware Engineer, Wi-Fi, Pixel Connectivity", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor's degree in Computer Science, Electrical Engineering, Computer Engineering, or a related technical field, or equivalent practical experience.",
			"5 years of experience in coding with a general purpose programming language (e.g., C/C++).",
			"Experience with Wi-Fi drivers, firmware or framework development.",
		},
	},
	{
		ID: "84295530024182470", Title: "Senior Product Design Engineer, Trackers and Home", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor's degree in Mechanical Engineering, Product Design, or a related field, or equivalent practical experience.",
			"5 years of experience designing mechanical components such as plastic or metal parts, mechanical assemblies, printed circuit boards, or flexes.",
			"5 years of experience using computer-aided design (CAD) tools such as 3D MCAD, NX, Creo, or Solidworks.",
		},
	},
	{
		ID: "76806086312501958", Title: "Software Engineer, Apps, Pixel", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Early",
		MinimumQualifications: []string{
			"Bachelor’s degree or equivalent practical experience.",
			"1 year of experience with software development in one or more programming languages (e.g., Java, Kotlin, C++, Python).",
			"1 year of experience with data structures or algorithms.",
		},
	},
	{
		ID: "92642283567882950", Title: "Test Engineer, User Experience Quality", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor's degree in computer science, electrical engineering, or equivalent practical experience.",
			"3 years of experience in coding (e.g., Python or Java), developing test methodologies, writing test plans, creating test cases, and debugging.",
			"2 years of experience with developing infrastructure, or experience with compute technologies, mobile application development.",
		},
	},
	{
		ID: "74113863372415686", Title: "Staff Hardware System Engineer, Home and Health", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Advanced",
		MinimumQualifications: []string{
			"Bachelor's degree in Electrical Engineering, Computer Engineering, Physics, a related field, or equivalent practical experience.",
			"6 years of experience working in consumer electronics system design.",
			"Experience in one of the following domains, such as architecture, power/battery/Soc, interfaces, or analog/camera/display engineering.",
		},
	},
	{
		ID: "103770758580183750", Title: "CPU RTL Design Engineer", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor’s degree in Electrical Engineering, Computer Engineering, Computer Science, or a related field, or equivalent practical experience.",
			"4 years of experience in high-performance CPU or AI accelerator logic design, RTL design or integration, including microarchitecture definition and PPA optimizations.",
			"Experience in CPU, and cache subsystem integration with SOCs.",
		},
	},
	{
		ID: "87506232826307270", Title: "Software Engineer Manager II, Silicon Tools", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan",
		ExperienceLevel: "Advanced",
		MinimumQualifications: []string{
			"Bachelor's degree or equivalent practical experience.",
			"8 years of experience in software development, with experience in building and scaling internal/external tools within a engineering organization.",
			"2 years of experience in a people management or team leadership role.",
		},
	},
	{
		ID: "109039274703102662", Title: "Product Quality Engineer, Global Hardware Quality and Reliability", Company: "Google", Location: "Taipei, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor's degree in Mechanical Engineering, Industrial Engineering, Materials Engineering, a related engineering discipline, or equivalent practical experience.",
			"5 years of experience in Quality/Reliability/Production operation, with direct experience in New Product development and qualification.",
		},
	},
	{
		ID: "107666671874777798", Title: "Test Development Engineer, Global Manufacturing Engineering", Company: "Google", Location: "Taipei, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor’s degree in Electrical Engineering, Computer Science, or a related technical field, or equivalent practical experience.",
			"5 years of experience in software development.",
			"Experience developing and validating Python scripts to automate manufacturing test processes.",
		},
	},
	{
		ID: "141781579927036614", Title: "Senior Software Engineer, Emerging On-prem AI Infrastructure", Company: "Google", Location: "Taipei, Taiwan",
		ExperienceLevel: "Mid",
		MinimumQualifications: []string{
			"Bachelor’s degree or equivalent practical experience.",
			"5 years of experience in software engineering, and with building tools focused on infrastructure operations, reliability, and architectural integrity.",
			"2 years of experience in software design and architecture, within a large-scale cloud infrastructure provider.",
			"Experience in testing, maintaining, and launching software products or infrastructure systems, and in debugging, troubleshooting, and diagnosing issues within distributed systems.",
		},
	},
}

var wantDetail = &JobDetailResponse{
	ID:       "106863362666570438",
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

func TestParseSearchHTML(t *testing.T) {
	f, err := os.Open("testdata/jobs_rsp.html")
	require.NoError(t, err)
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	require.NoError(t, err)

	got, err := parseJobsHTML(doc)
	require.NoError(t, err)
	assert.Equal(t, wantJobs, got)
}

// A genuine no-results page keeps the "N jobs matched" counter (span.SWhIm)
// with a zero count; a bot challenge or redesigned page has neither cards
// nor the counter and must error instead of reading as an empty search.
func TestParseSearchHTMLNoResultsPage(t *testing.T) {
	const page = `<html><body><main><div class="rZt9ff"><span class="SWhIm">0</span>  jobs matched</div></main></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	require.NoError(t, err)

	jobs, err := parseJobsHTML(doc)
	require.NoError(t, err)
	assert.Empty(t, jobs)
}

func TestParseSearchHTMLUnknownPageErrors(t *testing.T) {
	const page = `<html><body>unusual response</body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page))
	require.NoError(t, err)

	_, err = parseJobsHTML(doc)
	require.Error(t, err)
}

// Minimal card/detail markup mirroring the live structure of a remote-
// eligible job: the fixtures carry no remote jobs, and on the live site the
// "Remote eligible" badge shares span.RP7SMd with the company badge.

func TestParseSearchHTMLRemoteEligible(t *testing.T) {
	const cardHTML = `<html><body><ul>
<li class="lLd3Je" ssk="18:123">
<h3 class="QJPWVe">Forward Deployed Engineer</h3>
<span class="RP7SMd"><i class="google-material-icons">corporate_fare</i><span>Google</span></span>
<span class="RP7SMd"><i class="google-material-icons">laptop_windows</i><span>Remote eligible</span></span>
<span class="r0wTof">San Francisco, CA, USA</span>
</li></ul></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(cardHTML))
	require.NoError(t, err)

	got, err := parseJobsHTML(doc)
	require.NoError(t, err)

	want := []Job{{
		ID:       "123",
		Title:    "Forward Deployed Engineer",
		Company:  "Google",
		Location: "San Francisco, CA, USA",
		Remote:   true,
	}}
	assert.Equal(t, want, got)
}

func TestParseDetailHTMLRemoteEligible(t *testing.T) {
	const detailHTML = `<html><head><title>Forward Deployed Engineer — Google Careers</title></head><body><main>
<span class="RP7SMd"><i class="google-material-icons">corporate_fare</i><span>Google</span></span>
<span class="RP7SMd"><i class="google-material-icons">laptop_windows</i><span>Remote eligible</span></span>
<span class="r0wTof">San Francisco, CA, USA</span>
</main></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(detailHTML))
	require.NoError(t, err)

	got, ok := parseJobDetailHTML(doc, "123")
	require.True(t, ok)

	want := &JobDetailResponse{
		ID:       "123",
		Title:    "Forward Deployed Engineer",
		Company:  "Google",
		Location: "San Francisco, CA, USA",
		Remote:   true,
	}
	assert.Equal(t, want, got)
}

func TestParseDetailHTML(t *testing.T) {
	f, err := os.Open("testdata/job_detail_rsp.html")
	require.NoError(t, err)
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	require.NoError(t, err)

	id := "106863362666570438"
	got, ok := parseJobDetailHTML(doc, id)
	require.True(t, ok)

	assert.Equal(t, *wantDetail, *got)
}
