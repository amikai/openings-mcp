package ats

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/amikai/openings-mcp/internal/provider/herp"
)

var _ Adapter = (*HerpAdapter)(nil)

// HerpAdapter serves companies listed on HERP Career, the job media that
// republishes the boards of HERP Hire ATS tenants. One request returns a
// company's whole board with full posting text, so all search, filter, and
// detail behavior is implemented over that dump.
type HerpAdapter struct {
	hc      *http.Client
	baseURL string
}

// herpCareersURLRE matches HERP Career company pages, and herpHireCareersURLRE
// the HERP Hire career pages on the same host. Both capture the company slug.
//
// Examples (hostname + escaped path):
//   - herp.careers/careers/companies/notainc
//   - herp.careers/careers/companies/notainc/jobs/i3jT6wNu2_Ym
//   - herp.careers/v1/herpinc
var (
	herpCareersURLRE     = regexp.MustCompile(`(?i)^herp\.careers/careers/companies/(?P<slug>[^/]+)`)
	herpHireCareersURLRE = regexp.MustCompile(`(?i)^herp\.careers/v1/(?P<slug>[^/]+)`)
)

func NewHerpAdapter(hc *http.Client) *HerpAdapter {
	return &HerpAdapter{hc: hc, baseURL: "https://herp.careers"}
}

func (a *HerpAdapter) Name() string { return "herp" }

func (a *HerpAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(herp.Companies))
	for _, c := range herp.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Slug, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes both herp.careers URL shapes. Unlike join, it
// accepts slugs outside the curated roster: one request settles whether a
// slug resolves. The HERP Hire form is accepted for convenience even though
// its tenant set is wider than HERP Career's — a tenant missing from the
// media surfaces as the not-listed error from Search or Detail.
func (a *HerpAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	for _, re := range []*regexp.Regexp{herpCareersURLRE, herpHireCareersURLRE} {
		if slug, ok := matchCareersSlug(re, u); ok {
			return strings.ToLower(slug), true
		}
	}
	return "", false
}

func (a *HerpAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	jobs, _, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *HerpAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, _, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *HerpAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	jobs, board, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	for _, j := range jobs {
		if j.summary.JobID != jobID {
			continue
		}
		return &JobDetail{
			JobID:       jobID,
			Title:       j.summary.Title,
			Company:     cmp.Or(board.CompanyName, slug),
			Location:    j.summary.Location,
			PostedAt:    j.summary.PostedAt,
			URL:         j.summary.URL,
			Description: joinHerpSections(j.description, herpCompanySection(board)),
		}, nil
	}
	return nil, fmt.Errorf(
		"herp: job %q not found for company %q; pass a job_id exactly as returned by the job search",
		jobID,
		slug,
	)
}

func (a *HerpAdapter) dump(ctx context.Context, slug string) ([]dumpJob, *herp.CompanyBoard, error) {
	slug = strings.ToLower(slug)
	client, err := herp.NewClient(a.baseURL, herp.WithClient(a.hc))
	if err != nil {
		return nil, nil, fmt.Errorf("herp: create client for %q: %w", slug, err)
	}
	res, err := client.GetCompany(ctx, herp.GetCompanyParams{Slug: slug})
	if err != nil {
		return nil, nil, fmt.Errorf("herp: fetch company %q: %w", slug, err)
	}

	var board *herp.CompanyBoard
	switch r := res.(type) {
	case *herp.CompanyResponse:
		board = &r.Company
	case *herp.ErrorResponse:
		// The upstream 404 body carries no usable message, so name the one
		// cause that actually matters: HERP Career lists fewer companies
		// than HERP Hire hosts.
		return nil, nil, fmt.Errorf(
			"herp: company %q is not listed on HERP Career (herp.careers/careers)",
			slug,
		)
	default:
		return nil, nil, fmt.Errorf("herp: unexpected response type %T", res)
	}

	jobs := make([]dumpJob, 0, len(board.Jobs))
	for _, j := range board.Jobs {
		loc := herpLocation(&j)
		posted, postedDate := herpParseTime(j.JobPublishedAt)

		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    j.ID,
				Title:    j.Name,
				Location: loc.display,
				PostedAt: postedDate,
				URL:      a.jobURL(board, slug, j.ID),
			},
			sortKey:     posted,
			orgUnit:     herpOrgUnit(&j),
			description: herpDescription(&j),
			locations:   loc.search,
			fields:      herpFields(&j, loc),
			isRemote:    loc.isRemote,
		})
	}
	return jobs, board, nil
}

// jobURL picks the surface that can actually take an application, because
// JobSummary.URL is where the applicant is handed off.
// companyIsApplicationEnabled governs the HERP Career media pages only: when
// a company opts out there, its media page renders without an apply action
// while its HERP Hire career page still accepts applications. Sending the
// applicant to the media page in that case is a dead end.
func (a *HerpAdapter) jobURL(board *herp.CompanyBoard, slug, jobID string) string {
	if board.CompanyIsApplicationEnabled.Or(true) {
		return fmt.Sprintf("%s/careers/companies/%s/jobs/%s", a.baseURL, slug, jobID)
	}
	return fmt.Sprintf("%s/v1/%s/%s", a.baseURL, slug, jobID)
}

// herpLocationText is one posting's location rendered three ways: what to
// show, what to match against, and whether "remote" should hit it.
type herpLocationText struct {
	display     string
	search      string
	prefectures []string
	cities      []string
	isRemote    bool
}

// herpRemoteLabels maps the upstream remote enum to the label the site shows,
// the Latin alias that makes location:"remote" work, and the shared
// workplaceType filter value.
//
// Note the missing third mode: a null jobRemoteworkType means the company
// said nothing, not that the role is on-site, so herp never emits the
// "On-site" value the other workplaceType adapters can.
var herpRemoteLabels = map[herp.JobJobRemoteworkType]struct{ ja, en, workplace string }{
	herp.JobJobRemoteworkTypeFULLREMOTEWORK:   {"フルリモート", "Remote", "Remote"},
	herp.JobJobRemoteworkTypeHYBRIDREMOTEWORK: {"ハイブリッドリモート", "Hybrid remote", "Hybrid"},
}

// herpLocation composes a posting's location. Structured entries win; the
// free-text field is the fallback; the remote label is appended, or stands
// alone when there is nothing else. An absent location stays empty rather
// than being guessed at.
func herpLocation(j *herp.Job) herpLocationText {
	var out herpLocationText

	parts := make([]string, 0, len(j.JobLocations)+1)
	for _, l := range j.JobLocations {
		out.prefectures = appendDistinct(out.prefectures, l.PrefName)
		city := l.CityName.Or("")
		out.cities = appendDistinct(out.cities, city)
		// Japanese address order, so both "東京都" and "中央区" stay
		// substrings of the composed text.
		parts = appendDistinct(parts, l.PrefName+city)
	}
	if len(parts) == 0 {
		firstLine, _, _ := strings.Cut(strings.TrimSpace(j.Location), "\n")
		parts = appendDistinct(parts, strings.TrimSpace(firstLine))
	}

	aliases := make([]string, 0, len(out.prefectures)+2)
	for _, l := range j.JobLocations {
		aliases = appendDistinct(aliases, herpPrefectureRomaji[l.PrefCode])
	}
	// The structured entries drive the concise display, but they routinely
	// drop detail the company did write out: a posting can name 京都市上京区
	// in the free text while its Kyoto entry carries a null city. Index the
	// whole upstream string so cities, stations, and addresses stay findable.
	aliases = appendDistinct(aliases, strings.Join(strings.Fields(j.Location), " "))

	if label, ok := herpRemoteLabels[j.JobRemoteworkType.Or("")]; ok {
		out.isRemote = true
		parts = appendDistinct(parts, label.ja)
		aliases = appendDistinct(aliases, label.en)
	}

	out.display = strings.Join(parts, "; ")
	// search is a superset of display: everything shown stays findable, and
	// the romanized aliases add Latin-script reach on a board whose location
	// text is otherwise Japanese-only.
	out.search = strings.Join(append(parts, aliases...), "; ")
	return out
}

func herpFields(j *herp.Job, loc herpLocationText) map[string][]string {
	fields := make(map[string][]string, 6)

	var roles, categories []string
	for _, r := range j.JobRoles {
		roles = appendDistinct(roles, r.Name)
		categories = appendDistinct(categories, r.ParentJobRoleName)
	}
	setHerpField(fields, "jobRole", roles)
	setHerpField(fields, "jobCategory", categories)
	setHerpField(fields, "prefecture", loc.prefectures)
	setHerpField(fields, "city", loc.cities)

	employment := make([]string, 0, len(j.JobEmploymentTypeIds))
	for _, e := range j.JobEmploymentTypeIds {
		employment = appendDistinct(employment, string(e))
	}
	setHerpField(fields, "employmentType", employment)

	// workplaceType is the shared key ashby, lever, and bamboohr already use,
	// with bamboohr's normalized labels rather than this upstream's enum —
	// the same dimension must not answer to two names across adapters.
	if label, ok := herpRemoteLabels[j.JobRemoteworkType.Or("")]; ok {
		setHerpField(fields, "workplaceType", []string{label.workplace})
	}
	return fields
}

func setHerpField(fields map[string][]string, key string, values []string) {
	if len(values) > 0 {
		fields[key] = values
	}
}

func herpOrgUnit(j *herp.Job) string {
	var parts []string
	for _, r := range j.JobRoles {
		parts = appendDistinct(parts, r.Name)
		parts = appendDistinct(parts, r.ParentJobRoleName)
	}
	return strings.Join(parts, " / ")
}

// herpDescription renders the posting the way the site lays it out. The
// unified JobDetail has one free-text field, so everything HERP Career
// publishes about the job goes here rather than being dropped.
func herpDescription(j *herp.Job) string {
	var b strings.Builder
	writeHerpSection(&b, "求人タイトル", j.Title)
	writeHerpSection(&b, "仕事概要", j.Summary)
	writeHerpSection(&b, "必須スキル", j.RequiredSkills.Or(""))
	writeHerpSection(&b, "歓迎スキル", j.PreferredSkills.Or(""))
	writeHerpSection(&b, "求める人物像", j.Personality.Or(""))
	writeHerpSection(&b, "給与", herpSalary(j))
	writeHerpSection(&b, "勤務地", j.Location)
	writeHerpSection(&b, "雇用形態", j.FormOfEmployment)
	writeHerpSection(&b, "勤務体系", j.WorkingConditions.Or(""))
	writeHerpSection(&b, "試用期間", j.Trial.Or(""))
	writeHerpSection(&b, "福利厚生", j.Welfare.Or(""))

	if _, updated := herpParseTime(j.UpdatedAt); updated != "" {
		writeHerpSection(&b, "最終更新", updated)
	}
	writeHerpSection(&b, "応募受付", herpApplicationStatus(j))
	return b.String()
}

// herpApplicationStatus reports whether this posting is still open, which is
// isApplicable alone. companyIsApplicationEnabled is a different fact — it
// says which surface takes applications, not whether the opening is live —
// and folding it in here marked active roles as closed (see jobURL).
func herpApplicationStatus(j *herp.Job) string {
	applicable, ok := j.IsApplicable.Get()
	if !ok {
		return ""
	}
	if applicable {
		return "受付中"
	}
	return "停止中"
}

// herpSalary keeps the company's own wording and adds a normalized line
// derived from the parsed bounds, so a reader never has to parse the
// free-text range.
func herpSalary(j *herp.Job) string {
	s, ok := j.Salary.Get()
	if !ok {
		return ""
	}
	parts := make([]string, 0, 2)
	if normalized := herpSalaryRange(s); normalized != "" {
		parts = append(parts, normalized)
	}
	if s.Text != "" {
		parts = append(parts, s.Text)
	}
	return strings.Join(parts, "\n")
}

func herpSalaryRange(s herp.Salary) string {
	minimum, hasMin := s.Minimum.Get()
	maximum, hasMax := s.Maximum.Get()
	if !hasMin && !hasMax {
		return ""
	}
	label, unit := "給与", herpManYen
	switch s.Period.Or("") {
	case herp.SalaryPeriodANNUAL:
		label = "年収"
	case herp.SalaryPeriodMONTHLY:
		label = "月給"
	case herp.SalaryPeriodDAILY:
		label, unit = "日給", herpYen
	case herp.SalaryPeriodHOURLY:
		label, unit = "時給", herpYen
	}

	switch {
	case hasMin && hasMax:
		return fmt.Sprintf("%s %s〜%s", label, unit(minimum), unit(maximum))
	case hasMin:
		return fmt.Sprintf("%s %s〜", label, unit(minimum))
	default:
		return fmt.Sprintf("%s 〜%s", label, unit(maximum))
	}
}

// herpManYen renders JPY the way Japanese postings do, in 万円 (10,000 JPY).
func herpManYen(jpy int) string {
	man := float64(jpy) / 10000
	return strconv.FormatFloat(man, 'f', -1, 64) + "万円"
}

func herpYen(jpy int) string { return strconv.Itoa(jpy) + "円" }

// herpCompanySection appends the startup profile HERP Career carries
// alongside the posting — funding, headcount, and who runs the company —
// which is often what decides whether the job is worth pursuing.
func herpCompanySection(board *herp.CompanyBoard) string {
	var body strings.Builder
	writeHerpField(&body, "企業名", board.CompanyName)
	writeHerpField(&body, "事業概要", board.CompanySynopsis.Or(""))
	writeHerpField(&body, "業界タグ", strings.Join(board.CompanyTags, ", "))
	writeHerpField(&body, "設立", board.CompanyFoundedIn.Or(""))
	writeHerpField(&body, "本社", board.CompanyHeadquarterLocation.Or(""))
	writeHerpField(&body, "資本金", strings.TrimSpace(board.CompanyShareCapital.Or("")))
	writeHerpField(&body, "従業員数", herpEmployees(board))
	writeHerpField(&body, "累計調達額", herpOku(board.CompanyCumulativeFunding))
	writeHerpField(&body, "評価額", herpOku(board.CompanyValuation))
	writeHerpField(&body, "投資家", strings.Join(board.CompanyInvestors, ", "))
	writeHerpField(&body, "何を", strings.Join(board.CompanyWhat, " "))
	writeHerpField(&body, "なぜ", strings.Join(board.CompanyWhy, " "))
	writeHerpField(&body, "どこで", strings.Join(board.CompanyWhere, " "))
	writeHerpField(&body, "どうやって", strings.Join(board.CompanyHow, " "))
	writeHerpField(&body, "経営陣", herpDirectors(board))
	writeHerpField(&body, "企業サイト", board.CompanyUrl.Or(""))

	var b strings.Builder
	writeHerpSection(&b, "企業情報", strings.TrimRight(body.String(), "\n"))
	return b.String()
}

// joinHerpSections separates sections the same way writeHerpSection does, so
// a section built by one builder still reads as a section when appended to
// another.
func joinHerpSections(sections ...string) string {
	kept := make([]string, 0, len(sections))
	for _, s := range sections {
		if strings.TrimSpace(s) != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, "\n\n")
}

// herpEmployees reports headcount with its most recent earlier reading, so
// the direction of travel is visible and not just the current number.
func herpEmployees(board *herp.CompanyBoard) string {
	now, ok := board.CompanyNumberOfEmployees.Get()
	if !ok {
		return ""
	}
	out := strconv.Itoa(now) + "名"

	var oldest *herp.EmployeeCountHistory
	for i, h := range board.NumberOfEmployeesHistories {
		if oldest == nil || h.Timestamp < oldest.Timestamp {
			oldest = &board.NumberOfEmployeesHistories[i]
		}
	}
	if oldest == nil {
		return out
	}
	month, _, _ := strings.Cut(oldest.Timestamp, "T")
	return fmt.Sprintf("%s（%s時点 %d名）", out, month, oldest.CompanyNumberOfEmployees)
}

// herpOku renders the funding figures in the 億円 (100M JPY) unit the site
// uses.
func herpOku(v herp.OptNilFloat64) string {
	amount, ok := v.Get()
	if !ok {
		return ""
	}
	return strconv.FormatFloat(amount, 'f', -1, 64) + "億円"
}

func herpDirectors(board *herp.CompanyBoard) string {
	names := make([]string, 0, len(board.Directors))
	for _, d := range board.Directors {
		name := d.DirectorName
		// isFounder is a property of one history entry, which often describes
		// a previous company — kaminashi's 原 康紘 founded TRIDENT, not
		// カミナシ. isInside is what ties the entry to this company.
		if slices.ContainsFunc(d.Histories, func(h herp.DirectorHistory) bool {
			return h.IsFounder && h.IsInside
		}) {
			name = d.DirectorName + "（創業者）"
		}
		names = appendDistinct(names, name)
	}
	return strings.Join(names, ", ")
}

func writeHerpSection(b *strings.Builder, label, body string) {
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

func writeHerpField(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	b.WriteString(label)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString("\n")
}

func herpParseTime(s string) (time.Time, string) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, ""
	}
	return t, isoDate(t)
}

// herpPrefectureRomaji maps the JIS prefecture code to its romanized name.
// The upstream only ships Japanese place names; without these, an
// English-language location search would silently match nothing.
var herpPrefectureRomaji = map[string]string{
	"01": "Hokkaido", "02": "Aomori", "03": "Iwate", "04": "Miyagi",
	"05": "Akita", "06": "Yamagata", "07": "Fukushima", "08": "Ibaraki",
	"09": "Tochigi", "10": "Gunma", "11": "Saitama", "12": "Chiba",
	"13": "Tokyo", "14": "Kanagawa", "15": "Niigata", "16": "Toyama",
	"17": "Ishikawa", "18": "Fukui", "19": "Yamanashi", "20": "Nagano",
	"21": "Gifu", "22": "Shizuoka", "23": "Aichi", "24": "Mie",
	"25": "Shiga", "26": "Kyoto", "27": "Osaka", "28": "Hyogo",
	"29": "Nara", "30": "Wakayama", "31": "Tottori", "32": "Shimane",
	"33": "Okayama", "34": "Hiroshima", "35": "Yamaguchi", "36": "Tokushima",
	"37": "Kagawa", "38": "Ehime", "39": "Kochi", "40": "Fukuoka",
	"41": "Saga", "42": "Nagasaki", "43": "Kumamoto", "44": "Oita",
	"45": "Miyazaki", "46": "Kagoshima", "47": "Okinawa",
}
