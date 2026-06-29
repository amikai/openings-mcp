package synopsys

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJobsResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/search_jobs_rsp.json")
	require.NoError(t, err)

	var raw struct {
		Results string `json:"results"`
	}
	require.NoError(t, json.Unmarshal(data, &raw))

	got, err := parseSearchResults(raw.Results)
	require.NoError(t, err)

	want := &JobsResponse{
		TotalResults: 604,
		TotalPages:   300,
		CurrentPage:  1,
		Jobs: []Job{
			{Title: "Staff Software Engineer", Location: "Bengaluru, India", Category: "Engineering", Posted: "03/31/2026", DisplayID: "16567", JobID: "93498496944", City: "bengaluru", Slug: "staff-software-engineer"},
			{Title: "Staff Software Engineer", Location: "Bengaluru, India", Category: "Engineering", Posted: "03/31/2026", DisplayID: "16566", JobID: "93498496928", City: "bengaluru", Slug: "staff-software-engineer"},
		},
	}
	assert.Equal(t, want, got)
}

func TestParseJobDetailResponse(t *testing.T) {
	f, err := os.Open("testdata/job_detail_rsp.html")
	require.NoError(t, err)
	defer f.Close()

	got, err := parseJobDetail(f)
	require.NoError(t, err)

	require.NoError(t, err)

	want := &JobDetailResponse{
		Title:          "Staff Software Engineer",
		DatePosted:     "2026-4-1",
		Locations:      []string{"Bengaluru, India"},
		DisplayID:      "16567",
		Category:       "Engineering",
		HireType:       "Employee",
		RemoteEligible: "No",
		Description: `We Are:

At Synopsys, we drive the innovations that shape the way we live and connect. Our technology is central to the Era of Pervasive Intelligence, from self-driving cars to learning machines. We lead in chip design, verification, and IP integration, empowering the creation of high-performance silicon chips and software content. Join us to transform the future through continuous technological innovation.

You Are:

You are a passionate and driven R&D Engineer with a deep understanding of data structures, algorithms, and their applications. You have a strong background in software development, particularly with C/C++ on UNIX/Linux platforms, and are eager to tackle complex, large-scale software code-based tool development. With a minimum of 8 years of related experience, you have honed your analytical, debugging, and problem-solving skills. You thrive in both self-directed and collaborative environments and are committed to continuous learning and exploration of new technologies. Your excellent communication skills in English enable you to effectively collaborate with team members and present your ideas clearly.

What You’ll Be Doing:

Supporting the existing functionality of our tools and continually enhancing their versatility, performance, and memory utilization while improving software quality.
Applying extensive knowledge of algorithms and data structure design to develop robust and efficient implementations that improve tool performance and customer adoption.
Interacting with other Synopsys R&D members and customers to understand their needs and product goals.
Contributing to the development of complex software code-based tools in a multi-person product development environment with high dependencies and tight schedules.
Exercising judgment in developing methods, techniques, and evaluation criteria to meet project goals.
Collaborating with a team of enthusiastic and creative engineers to drive innovation and excellence.
The Impact You Will Have:

Enhancing the performance and quality of our verification tools, leading to increased customer satisfaction and adoption.
Driving continuous improvement in software development processes and practices.
Contributing to the development of cutting-edge technologies that power innovations in various industries.
Helping Synopsys maintain its leadership position in the market by delivering high-performance solutions.
Influencing the direction and success of our hardware verification tools through your expertise and innovation.
Fostering a collaborative and innovative work environment that encourages growth and learning.
What You’ll Need:

A Bachelor’s degree in Electrical/Electronics/Computer-Science Engineering with a minimum of 5 years of related experience, or a Master’s degree with 3 years of relevant experience.
In-depth understanding of data structures, algorithms, and their applications.
Excellent software development experience with C/C++ on UNIX/Linux platforms.
Exposure to Python, TCL, and shell scripting languages is preferable.
Exposure to HDL languages like Verilog or System Verilog is desirable, with a willingness to learn their nuances.
Demonstrated history of good analytical, debugging, and problem-solving skills.
Experience with complex and large software code-based tool development.
Who You Are:

You are a motivated and enthusiastic engineer who excels in both independent and collaborative settings. You have a solid desire to learn and explore new technologies, and you exercise good judgment in developing methods and techniques to meet project goals. Your excellent written and oral communication skills in English enable you to collaborate effectively and present your ideas clearly. Special consideration will be given to candidates with a background in hardware functional verification and/or synthesis techniques, as well as knowledge of software specification, design processes, and regression testing.

The Team You’ll Be A Part Of:

You will join the Hardware Assisted Verification team at Synopsys, a group of dedicated and innovative engineers focused on developing and enhancing our verification tools. Our team is committed to pushing the boundaries of technology and delivering high-performance solutions that meet the needs of our customers. We work in a collaborative and dynamic environment, where creativity and innovation are encouraged and valued.

Rewards and Benefits:

We offer a comprehensive range of health, wellness, and financial benefits to cater to your needs. Our total rewards include both monetary and non-monetary offerings. Your recruiter will provide more details about the salary range and benefits during the hiring process.



#TPG

            
                    


At Synopsys, we want talented people of every background to feel valued and supported to do their best work. Synopsys considers all applicants for employment without regard to race, color, religion, national origin, gender, sexual orientation, age, military veteran status, or disability.`,
	}
	assert.Equal(t, want, got)
}

func TestStripHTML(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"<p>Hello <b>world</b></p>", "Hello world"},
		{"<p>a &amp; b</p>", "a & b"},
		{"<p>foo &lt;bar&gt;</p>", "foo <bar>"},
		{"<p>&nbsp;padded&nbsp;</p>", "padded"},
		{"plain text", "plain text"},
		{"", ""},
	}
	for _, c := range cases {
		got := stripHTML(c.in)
		assert.Equal(t, c.want, got, "input: %q", c.in)
	}
}
