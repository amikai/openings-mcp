package google

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var wantJobs = []Job{
	{ID: "106863362666570438", Path: "106863362666570438-software-engineer-gpu-system-software", Title: "Software Engineer, GPU System Software", Company: "Google", Location: "Taipei, Taiwan"},
	{ID: "82975510480462534", Path: "82975510480462534-soc-product-engineer-google-cloud", Title: "SoC Product Engineer, Google Cloud", Company: "Google", Location: "Zhubei, Zhubei City, Hsinchu County, Taiwan"},
	{ID: "81991011634422470", Path: "81991011634422470-silicon-physical-design-cad-engineer", Title: "Silicon Physical Design CAD Engineer", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "143985660506579654", Path: "143985660506579654-senior-signal-and-power-integrity-engineer-pixel", Title: "Senior Signal and Power Integrity Engineer, Pixel", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "78019461818262214", Path: "78019461818262214-software-engineering-manager-release-engineering-google-cloud-platforms", Title: "Software Engineering Manager, Release Engineering, Google Cloud Platforms", Company: "Google", Location: "Taipei, Taiwan"},
	{ID: "112498222319968966", Path: "112498222319968966-firmware-engineer-pixel-systems-power", Title: "Firmware Engineer, Pixel Systems Power", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "123677700068909766", Path: "123677700068909766-supplier-quality-engineer-memory", Title: "Supplier Quality Engineer, Memory", Company: "Google", Location: "Taipei, Taiwan"},
	{ID: "77507329917887174", Path: "77507329917887174-software-engineering-manager-pixel-camera", Title: "Software Engineering Manager, Pixel Camera", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "126487124076569286", Path: "126487124076569286-developer-relations-engineer-android-play-games", Title: "Developer Relations Engineer, Android, Play, Games", Company: "Google", Location: "Taipei, Taiwan"},
	{ID: "110315781933146822", Path: "110315781933146822-software-engineer-android-apps-pixel", Title: "Software Engineer, Android Apps, Pixel", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "120702421604147910", Path: "120702421604147910-firmware-engineer-wifi-pixel-connectivity", Title: "Firmware Engineer, Wi-Fi, Pixel Connectivity", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "84295530024182470", Path: "84295530024182470-senior-product-design-engineer-trackers-and-home", Title: "Senior Product Design Engineer, Trackers and Home", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "76806086312501958", Path: "76806086312501958-software-engineer-apps-pixel", Title: "Software Engineer, Apps, Pixel", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "92642283567882950", Path: "92642283567882950-test-engineer-user-experience-quality", Title: "Test Engineer, User Experience Quality", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "74113863372415686", Path: "74113863372415686-staff-hardware-system-engineer-home-and-health", Title: "Staff Hardware System Engineer, Home and Health", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "103770758580183750", Path: "103770758580183750-cpu-rtl-design-engineer", Title: "CPU RTL Design Engineer", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "87506232826307270", Path: "87506232826307270-software-engineer-manager-ii-silicon-tools", Title: "Software Engineer Manager II, Silicon Tools", Company: "Google", Location: "New Taipei, Banqiao District, New Taipei City, Taiwan"},
	{ID: "109039274703102662", Path: "109039274703102662-product-quality-engineer-global-hardware-quality-and-reliability", Title: "Product Quality Engineer, Global Hardware Quality and Reliability", Company: "Google", Location: "Taipei, Taiwan"},
	{ID: "107666671874777798", Path: "107666671874777798-test-development-engineer-global-manufacturing-engineering", Title: "Test Development Engineer, Global Manufacturing Engineering", Company: "Google", Location: "Taipei, Taiwan"},
	{ID: "141781579927036614", Path: "141781579927036614-senior-software-engineer-emerging-onprem-ai-infrastructure", Title: "Senior Software Engineer, Emerging On-prem AI Infrastructure", Company: "Google", Location: "Taipei, Taiwan"},
}

var wantDetail = &JobDetailResponse{
	ID:       "106863362666570438",
	Path:     "106863362666570438-software-engineer-gpu-system-software",
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
	data, err := os.ReadFile("testdata/search_jobs_rsp.html")
	require.NoError(t, err)

	got := parseSearchHTML(string(data))
	assert.Equal(t, wantJobs, got)
}

func TestParseDetailHTML(t *testing.T) {
	data, err := os.ReadFile("testdata/job_detail_rsp.html")
	require.NoError(t, err)

	got, ok := parseDetailHTML(string(data), "106863362666570438")
	require.True(t, ok)

	// path is set by client, not by parser
	got.Path = wantDetail.Path
	assert.Equal(t, *wantDetail, got)
}
