package linkedin

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

var wantJobs = []Job{
	{ID: "4422697744", Title: "Software Engineer", Company: "BoostDraft", CompanyURL: "https://www.linkedin.com/company/boostdraft", Location: "Taiwan", PostedDate: "2026-06-03", Remote: false},
	{ID: "4430577683", Title: "Software Engineer, Apps, Pixel", Company: "Google", CompanyURL: "https://www.linkedin.com/company/google", Location: "Banqiao District, New Taipei City, Taiwan", PostedDate: "2026-06-22", Remote: false},
	{ID: "4435540496", Title: "Software Engineer", Company: "Mphasis", CompanyURL: "https://in.linkedin.com/company/mphasis", Location: "Taipei, Taipei City, Taiwan", PostedDate: "2026-07-01", Remote: false},
	{ID: "4409906484", Title: "Software Engineer (Taipei)", Company: "Nitra", CompanyURL: "https://www.linkedin.com/company/nitrahq", Location: "Taipei, Taipei City, Taiwan", PostedDate: "2026-05-07", Remote: false},
	{ID: "4430941394", Title: "(f2pool) Software Engineer - Back-end / Full-stack", Company: "stakefish", CompanyURL: "https://vg.linkedin.com/company/stakefish", Location: "Taiwan", PostedDate: "2026-05-25", Remote: false},
	{ID: "4435420998", Title: "Full Stack Software Engineer", Company: "MediaTek", CompanyURL: "https://tw.linkedin.com/company/mediatek", Location: "Hsinchu, Taiwan, Taiwan", PostedDate: "2026-07-03", Remote: false},
	{ID: "4425114186", Title: "Software Engineer - Dajia, Taichung City, Taiwan", Company: "Winbro", CompanyURL: "https://www.linkedin.com/company/winbro", Location: "Dajia District, Taichung City, Taiwan", PostedDate: "2026-05-12", Remote: false},
	{ID: "4435546265", Title: "Software Engineer", Company: "Mphasis", CompanyURL: "https://in.linkedin.com/company/mphasis", Location: "Taipei, Taipei City, Taiwan", PostedDate: "2026-07-01", Remote: false},
	{ID: "4435541354", Title: "Software Engineer", Company: "Mphasis", CompanyURL: "https://in.linkedin.com/company/mphasis", Location: "Taipei, Taipei City, Taiwan", PostedDate: "2026-07-01", Remote: false},
	{ID: "4401701902", Title: "Senior Software Engineer / Lead Software Engineer", Company: "BoostDraft", CompanyURL: "https://www.linkedin.com/company/boostdraft", Location: "Taiwan", PostedDate: "2026-04-14", Remote: false},
}

func TestParseSearchHTML(t *testing.T) {
	f, err := os.Open("testdata/jobs_rsp.html")
	require.NoError(t, err)
	defer f.Close()

	doc, err := html.Parse(f)
	require.NoError(t, err)

	got := parseJobsHTML(doc)
	assert.Equal(t, wantJobs, got)
}

// The real fixture carries no remote postings (LinkedIn doesn't expose a
// structured remote flag on search cards), so remote detection is exercised
// with minimal literal markup instead.
func TestParseSearchHTMLRemote(t *testing.T) {
	const cardHTML = `<html><body><ul><li>
<div class="base-card base-search-card job-search-card" data-entity-urn="urn:li:jobPosting:999">
  <a class="base-card__full-link" href="https://www.linkedin.com/jobs/view/remote-backend-engineer-at-acme-999?position=1">
    <span class="sr-only">Remote Backend Engineer</span>
  </a>
  <h4 class="base-search-card__subtitle"><a href="https://www.linkedin.com/company/acme">Acme</a></h4>
  <div class="base-search-card__metadata">
    <span class="job-search-card__location">Worldwide</span>
    <time class="job-search-card__listdate" datetime="2026-01-02">1 day ago</time>
  </div>
</div>
</li></ul></body></html>`
	doc, err := html.Parse(strings.NewReader(cardHTML))
	require.NoError(t, err)

	got := parseJobsHTML(doc)

	want := []Job{{
		ID:         "999",
		Title:      "Remote Backend Engineer",
		Company:    "Acme",
		CompanyURL: "https://www.linkedin.com/company/acme",
		Location:   "Worldwide",
		PostedDate: "2026-01-02",
		Remote:     true,
	}}
	assert.Equal(t, want, got)
}

// wantDetail omits Description (asserted separately via assert.Contains
// below): the real posting's description is a very large bilingual blob and
// pinning it verbatim would make this test brittle to content edits on
// LinkedIn's side rather than to parsing logic.
var wantDetail = &JobDetailResponse{
	ID:             "4422697744",
	Title:          "Software Engineer",
	Company:        "BoostDraft",
	Location:       "Taiwan",
	Posted:         "1 month ago",
	SeniorityLevel: "Entry level",
	EmploymentType: "Full-time",
	JobFunction:    "Other",
	Industries:     "IT Services and IT Consulting",
	CompanyLogo:    "https://media.licdn.com/dms/image/v2/D560BAQH6X5Q3LYFEvA/company-logo_100_100/B56ZX0zJbWHQAQ-/0/1743568803895/boostdraft_logo?e=2147483647&v=beta&t=IQ7Xd6zntdf-iDGHH8n9cqTwd_GBsJXXb5kTSlDxi3I",
	Remote:         false,
}

func TestParseDetailHTML(t *testing.T) {
	f, err := os.Open("testdata/job_detail_rsp.html")
	require.NoError(t, err)
	defer f.Close()

	doc, err := html.Parse(f)
	require.NoError(t, err)

	got, ok := parseJobDetailHTML(doc, "4422697744")
	require.True(t, ok)

	description := got.Description
	got.Description = ""

	assert.Equal(t, wantDetail, got)
	assert.Contains(t, description, "BoostDraft is a software engineering company")
	assert.Contains(t, description, "Fluent in coding with C#")
}

// The real fixture's job used LinkedIn's own Easy Apply, so it carries no
// <code id="applyUrl">. Exercise the external-ATS-redirect path with a
// minimal literal snippet shaped like the reference implementation expects:
// an HTML comment where "url" is the first query parameter (matching
// python-jobspy's own `(?<=\?url=)` lookbehind, which likewise only matches
// "url" immediately after "?", not after a preceding "&other=..." param).
func TestParseDetailHTMLApplyURL(t *testing.T) {
	const detailHTML = `<html><head><title>x</title></head><body>
<h1 class="topcard__title">Widget Engineer</h1>
<code id="applyUrl"><!--"https://distro.example.com/redirect?url=https%3A%2F%2Fjobs.example.com%2Fapply%2F123"--></code>
</body></html>`
	doc, err := html.Parse(strings.NewReader(detailHTML))
	require.NoError(t, err)

	got, ok := parseJobDetailHTML(doc, "123")
	require.True(t, ok)
	assert.Equal(t, "https://jobs.example.com/apply/123", got.ApplyURL)
}
