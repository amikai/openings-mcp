package ats

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/engage"
)

var _ Adapter = (*EngageAdapter)(nil)

// EngageAdapter serves companies hosted on engage (エン・ジャパン), the free ATS
// whose tenants each get a careers site at en-gage.net/<slug>/. One request
// returns a tenant's board, so search and filtering are implemented over that
// dump; the board carries no posting text, so Detail fetches the posting page.
//
// The board is a capped dump, not a complete one: engage returns at most
// [engage.CategoryCap] jobs per employment category and offers no pagination.
// For a tenant over that ceiling, Search results and TotalCount are a lower
// bound on the true board.
type EngageAdapter struct {
	hc        *http.Client
	baseURL   string
	dumpCache *DumpCache
}

// engageCareersURLRE matches an engage tenant careers page and captures the
// slug. Board and posting URLs share the same first segment.
//
// Examples (hostname + escaped path):
//   - en-gage.net/nova_career/
//   - en-gage.net/nova_career/work_17046487/
var engageCareersURLRE = regexp.MustCompile(`(?i)^en-gage\.net/(?P<slug>[a-z0-9][a-z0-9_-]*)(?:/|$)`)

// engageReservedPaths are en-gage.net first segments that belong to the site
// itself rather than to a tenant, so a URL under them is not a careers URL.
var engageReservedPaths = map[string]bool{
	"user":           true,
	"api":            true,
	"apply":          true,
	"search":         true,
	"search2":        true,
	"common_user":    true,
	"common_new":     true,
	"imagefile":      true,
	"imagefile_user": true,
}

func NewEngageAdapter(hc *http.Client, dumpCache *DumpCache) *EngageAdapter {
	return &EngageAdapter{hc: hc, baseURL: "https://en-gage.net", dumpCache: dumpCache}
}

func (a *EngageAdapter) Name() string { return "engage" }

func (a *EngageAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(engage.Companies))
	for _, c := range engage.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Slug, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes en-gage.net tenant pages, both the board and a
// single posting. Like herp, it accepts slugs outside the curated roster:
// one request settles whether a slug resolves. Site-owned paths (/user/,
// /search/, sitemaps) are rejected, since they are not tenants.
func (a *EngageAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	slug, ok := matchCareersSlug(engageCareersURLRE, u)
	if !ok {
		return "", false
	}
	slug = strings.ToLower(slug)
	if engageReservedPaths[slug] || strings.HasPrefix(slug, "sitemap") {
		return "", false
	}
	return slug, true
}

// CareersURL renders the roster company's public engage tenant board.
func (a *EngageAdapter) CareersURL(slug string) (string, bool) {
	c, ok := engage.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return "", false
	}
	return c.CareersURL(), true
}

func (a *EngageAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *EngageAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

// Detail fetches the posting page, because the board lists jobs without their
// descriptions.
func (a *EngageAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	slug = strings.ToLower(slug)
	d, err := engage.NewClient(a.baseURL, a.hc).Job(ctx, slug, jobID)
	if err != nil {
		if errors.Is(err, engage.ErrJobNotFound) {
			return nil, fmt.Errorf(
				"engage: job %q not found for company %q; pass a job_id exactly as returned by the job search",
				jobID,
				slug,
			)
		}
		if errors.Is(err, engage.ErrBoardNotFound) {
			return nil, engageUnknownCompany(slug)
		}
		return nil, fmt.Errorf("engage: fetch job %q for %q: %w", jobID, slug, err)
	}

	return &JobDetail{
		JobID:       d.WorkID,
		Title:       d.Title,
		Company:     d.Organization.Name,
		Location:    engageLocationDisplay(d.Location),
		PostedAt:    engagePostedAt(d.DatePosted),
		URL:         a.jobURL(slug, d.WorkID),
		Description: engageDescription(d),
	}, nil
}

// dump returns a (possibly capped) board dump, reusing the process-local
// dump cache when enabled. Caching does not change lower-bound TotalCount
// semantics from engage.CategoryCap.
func (a *EngageAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	slug = strings.ToLower(slug)
	jobs, _, err := a.dumpCache.getOrLoadDump(ctx, a.Name(), slug, func(ctx context.Context) ([]dumpJob, any, error) {
		jobs, err := a.fetchDump(ctx, slug)
		return jobs, nil, err
	})
	return jobs, err
}

// fetchDump loads the engage board HTML and reshapes jobs for filtering.
func (a *EngageAdapter) fetchDump(ctx context.Context, slug string) ([]dumpJob, error) {
	board, err := engage.NewClient(a.baseURL, a.hc).Board(ctx, slug)
	if err != nil {
		switch {
		case errors.Is(err, engage.ErrBoardNotFound):
			return nil, engageUnknownCompany(slug)
		case errors.Is(err, engage.ErrEmptyBoard):
			// engage serves no empty boards, so zero parsed jobs means the
			// board markup moved rather than the tenant closing every role.
			return nil, fmt.Errorf("engage: board for %q returned no parsable jobs: %w", slug, err)
		default:
			return nil, fmt.Errorf("engage: fetch board %q: %w", slug, err)
		}
	}

	all := board.Jobs()
	jobs := make([]dumpJob, 0, len(all))
	for _, j := range all {
		// The board lists postings without their text, so the query's
		// description tier has nothing to match; title, salary, and area are
		// all engage exposes before Detail.
		searchText := strings.Join([]string{j.Area, j.Salary}, " ")
		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    j.WorkID,
				Title:    j.Title,
				Location: j.Area,
				PostedAt: engagePostedAt(j.LastUpdated),
				URL:      a.jobURL(slug, j.WorkID),
			},
			sortKey:   j.LastUpdated,
			orgUnit:   j.EmploymentType,
			locations: searchText,
			fields:    engageFields(j),
			isRemote:  engageIsRemote(j),
		})
	}
	return jobs, nil
}

func engageUnknownCompany(slug string) error {
	return fmt.Errorf("engage: company %q has no board at en-gage.net/%s/", slug, slug)
}

func (a *EngageAdapter) jobURL(slug, workID string) string {
	return fmt.Sprintf("%s/%s/work_%s/", a.baseURL, slug, workID)
}

func engagePostedAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return isoDate(t)
}

// engageFields exposes employment type, the board's only filter dimension.
func engageFields(j engage.Job) map[string][]string {
	if strings.TrimSpace(j.EmploymentType) == "" {
		return nil
	}
	return map[string][]string{"employmentType": {j.EmploymentType}}
}

// engageRemoteMarkers are the terms engage tenants write into a posting's
// title or location line to mean remote work. The board has no structured
// remote field, so matching this text is the only way location:"remote" can
// hit anything.
var engageRemoteMarkers = []string{"リモート", "在宅", "テレワーク", "remote"}

func engageIsRemote(j engage.Job) bool {
	text := strings.ToLower(j.Title + " " + j.Area)
	for _, m := range engageRemoteMarkers {
		if strings.Contains(text, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

func engageLocationDisplay(l engage.Location) string {
	var parts []string
	for _, p := range []string{l.Region, l.Locality, l.Street} {
		parts = appendDistinct(parts, p)
	}
	return strings.Join(parts, "")
}

// engageDescription renders the posting as the site lays it out. The unified
// JobDetail has one free-text field, so the JSON-LD description carries the
// body and the remaining structured facts are appended as labeled sections
// rather than dropped.
func engageDescription(d *engage.JobDetail) string {
	var b strings.Builder
	writeEngageSection(&b, "仕事内容", engagePlainText(d.DescriptionHTML))
	for _, s := range d.Sections {
		writeEngageSection(&b, s.Heading, s.Text)
	}
	writeEngageSection(&b, "雇用形態", d.EmploymentType)
	writeEngageSection(&b, "給与", engageSalaries(d.Salaries))
	writeEngageSection(&b, "勤務地", engageFullAddress(d.Location))
	writeEngageSection(&b, "企業サイト", d.Organization.SameAs)
	return b.String()
}

func engageSalaries(salaries []engage.Salary) string {
	lines := make([]string, 0, len(salaries))
	for _, s := range salaries {
		if line := engageSalaryLine(s); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// engageSalaryLine renders one baseSalary entry. The JSON-LD carries the
// bounds as strings and frequently omits the upper one.
func engageSalaryLine(s engage.Salary) string {
	label := engageSalaryPeriods[strings.ToUpper(s.UnitText)]
	if label == "" {
		label = "給与"
	}
	currency := s.Currency
	if currency == "JPY" {
		currency = "円"
	}

	switch {
	case s.MinValue != "" && s.MaxValue != "":
		return fmt.Sprintf("%s %s〜%s%s", label, s.MinValue, s.MaxValue, currency)
	case s.MinValue != "":
		return fmt.Sprintf("%s %s%s〜", label, s.MinValue, currency)
	case s.MaxValue != "":
		return fmt.Sprintf("%s 〜%s%s", label, s.MaxValue, currency)
	default:
		return ""
	}
}

// engageSalaryPeriods maps schema.org pay periods to the label Japanese
// postings use.
var engageSalaryPeriods = map[string]string{
	"YEAR":  "年収",
	"MONTH": "月給",
	"WEEK":  "週給",
	"DAY":   "日給",
	"HOUR":  "時給",
}

func engageFullAddress(l engage.Location) string {
	var parts []string
	for _, p := range []string{l.PostalCode, l.Region, l.Locality, l.Street} {
		parts = appendDistinct(parts, p)
	}
	return strings.Join(parts, " ")
}

// engageTagRE strips the markup engage embeds in the JSON-LD description,
// which is an HTML fragment rather than plain text.
var engageTagRE = regexp.MustCompile(`(?s)<[^>]*>`)

// engageBlockTagRE matches the tags that end a visual line, so removing markup
// does not run separate paragraphs together.
var engageBlockTagRE = regexp.MustCompile(`(?i)<(?:br\s*/?|/p|/div|/li|/h[1-6])>`)

func engagePlainText(html string) string {
	s := engageBlockTagRE.ReplaceAllString(html, "\n")
	s = engageTagRE.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")

	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, "\n")
}

func writeEngageSection(b *strings.Builder, label, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(label)
	b.WriteString("\n")
	b.WriteString(body)
}
