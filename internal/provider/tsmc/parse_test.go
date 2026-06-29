package tsmc

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

var wantJobs = []Job{
	{ID: "21826", Slug: "R-D-Advanced-Packaging-Integration-Engineer", Title: "R&D Advanced Packaging Integration Engineer", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/05/28"},
	{ID: "19509", Slug: "TSMC-R-D-Process-Engineer-FLM-Forward-Looking-Module", Title: "TSMC R&D Process Engineer / FLM (Forward Looking Module)", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/03/17"},
	{ID: "302", Slug: "A10-RD-Device-Engineer", Title: "A10 RD Device Engineer", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/03/17"},
	{ID: "5354", Slug: "A14-R-D-Device-Engineer", Title: "A14 R&D Device Engineer", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/03/17"},
	{ID: "353", Slug: "A10-A14-RD-Integration-Engineer", Title: "A10/A14 RD Integration Engineer", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/03/17"},
	{ID: "6152", Slug: "Research-Pathfinding-Engineer", Title: "Research & Pathfinding Engineer", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/03/17"},
	{ID: "5351", Slug: "A14-R-D-SRAM-Engineer", Title: "A14 R&D SRAM Engineer", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/03/17"},
	{ID: "5353", Slug: "A14-R-D-Integration-Engineer", Title: "A14 R&D Integration Engineer", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/03/17"},
	{ID: "16359", Slug: "Research-and-Development-Engineer-R-D", Title: "Research and Development Engineer (R&D)", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/03/17"},
	{ID: "6154", Slug: "R-D-Module-Engineer", Title: "R&D Module Engineer", Location: "台灣", CareerArea: "研究發展", EmploymentType: "正職", Posted: "2026/03/17"},
}

var wantDetail = &JobDetailResponse{
	ID:             "21826",
	Slug:           "R-D-Advanced-Packaging-Integration-Engineer",
	Title:          "R&D Advanced Packaging Integration Engineer",
	Company:        "台灣積體電路製造股份有限公司",
	Location:       "台灣",
	CareerArea:     "研究發展",
	JobType:        "工程師/管理師",
	EmploymentType: "正職",
	Posted:         "2026/05/28",
	Responsibilities: `1. Lead the development and integration of novel advanced packaging technologies, processes, and materials.
2. Design, execute, and analyze experiments to optimize packaging performance, reliability, and cost.
3. Collaborate cross-functionally with design, module, and operations teams to ensure seamless integration and manufacturability.
4. Identify and troubleshoot complex technical challenges in packaging integration, driving root cause analysis and solutions.
5. Contribute to the strategic roadmap for advanced packaging, evaluating new technologies and intellectual property.`,
	Qualifications: `1. MS or Ph.D. in Electrical, Mechanical, Chemical, Materials Science, Physics, or a related engineering/science discipline.
2. Possesses strong technical problem-solving and analytical skills, grounded in fundamental principles, with a proactive, hands-on approach and a strong sense of ownership; consistently demonstrates a growth mindset for continuous learning and development.
3. Exceptional team player who fosters trust, drives innovation with disciplined execution, and is fluent in both Mandarin and English with excellent cross-functional communication skills.`,
}

func TestParseSearchHTML(t *testing.T) {
	f, err := os.Open("testdata/jobs_rsp.html")
	require.NoError(t, err)
	defer f.Close()
	doc, err := html.Parse(f)
	require.NoError(t, err)

	gotJobs, gotTotal := parseSearchHTML(doc)
	assert.Equal(t, 22, gotTotal)
	assert.Equal(t, wantJobs, gotJobs)
}

func TestParseDetailHTML(t *testing.T) {
	f, err := os.Open("testdata/job_detail_rsp.html")
	require.NoError(t, err)
	defer f.Close()
	doc, err := html.Parse(f)
	require.NoError(t, err)

	got, ok := parseDetailHTML(doc)
	require.True(t, ok)

	assert.Equal(t, *wantDetail, got)
}
