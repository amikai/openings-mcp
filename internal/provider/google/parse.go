package google

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var dsDataPattern = regexp.MustCompile(`(?s)data:(\[.*?\]), sideChannel`)

// parseJobsHTML parses job cards from search results HTML. It first attempts
// to extract the embedded AF_initDataCallback (key: 'ds:1') JSON payload,
// falling back to DOM parsing if the script block is not present.
//
// When no cards parse, the "N jobs matched" counter (span.SWhIm) distinguishes
// a genuine zero-result page from a bot challenge or redesigned page — the
// latter must error, not read as an empty search.
func parseJobsHTML(doc *goquery.Document) ([]Job, error) {
	if jobs, ok := parseJobsFromDS(doc); ok {
		return jobs, nil
	}
	return parseJobsFromDOM(doc)
}

func parseJobsFromDS(doc *goquery.Document) ([]Job, bool) {
	data, err := extractDSRaw(doc, "ds:1")
	if err != nil || len(data) == 0 {
		return nil, false
	}
	var records []json.RawMessage
	if err := json.Unmarshal(data[0], &records); err != nil || len(records) == 0 {
		return nil, false
	}

	var jobs []Job
	for _, recRaw := range records {
		if job, ok := parseJobRecord(recRaw); ok {
			jobs = append(jobs, job)
		}
	}
	if len(jobs) == 0 {
		return nil, false
	}
	return jobs, true
}

func parseJobRecord(recRaw json.RawMessage) (Job, bool) {
	var rec []json.RawMessage
	if err := json.Unmarshal(recRaw, &rec); err != nil {
		return Job{}, false
	}
	id := dsString(rec, 0)
	if id == "" {
		return Job{}, false
	}
	title := dsString(rec, 1)
	company := dsString(rec, 7)
	if company == "" {
		company = "Google"
	}
	location := dsLocations(rec, 9)
	levelCode := dsInt(rec, 20)
	experienceLevel := mapExperienceLevel(levelCode)

	var minQuals []string
	if minQualsHTML := dsHTMLField(rec, 19); minQualsHTML != "" {
		minQuals = parseBulletTextsFromHTML(minQualsHTML)
	}

	remote := dsInt(rec, 16) == 1 || strings.Contains(location, "Remote")

	return Job{
		ID:                    id,
		Title:                 title,
		Company:               company,
		Location:              location,
		Remote:                remote,
		ExperienceLevel:       experienceLevel,
		MinimumQualifications: minQuals,
	}, true
}

func parseJobsFromDOM(doc *goquery.Document) ([]Job, error) {
	var jobs []Job
	for _, li := range doc.Find("li.lLd3Je").EachIter() {
		if job, ok := parseJobCard(li); ok {
			jobs = append(jobs, job)
		}
	}
	if len(jobs) == 0 && doc.Find("span.SWhIm").Length() == 0 {
		return nil, errors.New("unrecognized search page: no job cards and no results counter")
	}
	return jobs, nil
}

func parseJobCard(li *goquery.Selection) (Job, bool) {
	ssk, _ := li.Attr("ssk")
	_, id, _ := strings.Cut(ssk, ":")
	if id == "" {
		return Job{}, false
	}

	title := strings.TrimSpace(li.Find("h3.QJPWVe").First().Text())
	if title == "" {
		return Job{}, false
	}

	var company string
	var remote bool
	// the company badge comes first, remote jobs add a second "Remote
	// eligible" badge with the same class.
	for _, s := range li.Find("span.RP7SMd").EachIter() {
		if t := spanChildText(s); t == "Remote eligible" {
			remote = true
		} else if company == "" {
			company = t
		}
	}

	location := strings.TrimSpace(li.Find("span.r0wTof").First().Text())
	experienceLevel := strings.TrimSpace(li.Find("span.wVSTAb").First().Text())
	minimumQualifications := bulletTexts(li.Find("div.Xsxa1e").First())

	return Job{
		ID:                    id,
		Title:                 title,
		Company:               company,
		Location:              location,
		Remote:                remote,
		ExperienceLevel:       experienceLevel,
		MinimumQualifications: minimumQualifications,
	}, true
}

// bulletTexts collects the whitespace-normalized text of every <li> under sel.
func bulletTexts(sel *goquery.Selection) []string {
	var bullets []string
	for _, li := range sel.Find("li").EachIter() {
		bullets = append(bullets, strings.Join(strings.Fields(li.Text()), " "))
	}
	return bullets
}

func parseBulletTextsFromHTML(htmlContent string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil
	}
	return bulletTexts(doc.Selection)
}

// parseJobDetailHTML parses a single job detail page. It first attempts to
// extract the embedded AF_initDataCallback (key: 'ds:0') payload, falling
// back to DOM parsing if absent.
func parseJobDetailHTML(doc *goquery.Document, id string) (*JobDetailResponse, bool) {
	if detail, ok := parseJobDetailFromDS(doc, id); ok {
		return detail, true
	}
	return parseJobDetailFromDOM(doc, id)
}

func parseJobDetailFromDS(doc *goquery.Document, id string) (*JobDetailResponse, bool) {
	data, err := extractDSRaw(doc, "ds:0")
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return parseJobDetailRecord(data[0], id)
}

func parseJobDetailRecord(recRaw json.RawMessage, id string) (*JobDetailResponse, bool) {
	var rec []json.RawMessage
	if err := json.Unmarshal(recRaw, &rec); err != nil || len(rec) == 0 {
		return nil, false
	}

	recID := dsString(rec, 0)
	if recID != "" && id != "" && recID != id {
		return nil, false
	}
	if recID == "" {
		recID = id
	}

	title := dsString(rec, 1)
	if title == "" {
		return nil, false
	}
	company := dsString(rec, 7)
	if company == "" {
		company = "Google"
	}
	location := dsLocations(rec, 9)

	var about string
	if aboutHTML := dsHTMLField(rec, 10); aboutHTML != "" {
		about = "About the job\n\n" + parseHTMLToText(aboutHTML)
	}

	var resp string
	if respHTML := dsHTMLField(rec, 3); respHTML != "" {
		resp = "Responsibilities\n\n\n" + parseHTMLToText(respHTML)
	}

	var qual string
	if qualHTML := dsHTMLField(rec, 4); qualHTML != "" {
		qual = parseQualificationsHTML(qualHTML)
	}

	remote := dsInt(rec, 16) == 1 || strings.Contains(location, "Remote")

	return &JobDetailResponse{
		ID:               recID,
		Title:            title,
		Company:          company,
		Location:         location,
		Remote:           remote,
		About:            about,
		Qualifications:   qual,
		Responsibilities: resp,
	}, true
}

func parseJobDetailFromDOM(doc *goquery.Document, id string) (*JobDetailResponse, bool) {
	detail := JobDetailResponse{ID: id}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	detail.Title = strings.TrimSuffix(title, " — Google Careers")

	// scoped to <main> to exclude sidebar content.
	main := doc.Find("main").First()

	// the company badge comes first, remote jobs add a second "Remote
	// eligible" badge with the same class.
	for _, s := range main.Find("span.RP7SMd").EachIter() {
		if t := spanChildText(s); t == "Remote eligible" {
			detail.Remote = true
		} else if detail.Company == "" {
			detail.Company = t
		}
	}

	detail.Location = strings.TrimSpace(main.Find("span.r0wTof").First().Text())

	if about := main.Find("div.aG5W3").First(); about.Length() > 0 {
		var sb strings.Builder
		for c := about.Nodes[0].FirstChild; c != nil; c = c.NextSibling {
			appendNodeText(&sb, c)
		}
		detail.About = strings.TrimSpace(strings.ReplaceAll(sb.String(), "\r", ""))
	}

	if resp := main.Find("div.BDNOWe").First(); resp.Length() > 0 {
		var sb strings.Builder
		for c := resp.Nodes[0].FirstChild; c != nil; c = c.NextSibling {
			appendNodeText(&sb, c)
		}
		detail.Responsibilities = strings.TrimSpace(strings.ReplaceAll(sb.String(), "\r", ""))
	}

	for _, h3 := range main.Find("h3").EachIter() {
		t := strings.TrimSpace(h3.Text())
		if !strings.HasPrefix(t, "Minimum qualifications") {
			continue
		}
		detail.Qualifications = parseQualifications(h3.Nodes[0])
		break
	}

	return &detail, detail.Title != ""
}

// parseQualifications collects the "Minimum qualifications" heading and, if
// immediately followed by a "Preferred qualifications" heading, that too —
// this sibling-order logic isn't expressible as a CSS selector.
func parseQualifications(h3 *html.Node) string {
	var sb strings.Builder
	appendNodeText(&sb, h3)
	for sib := h3.NextSibling; sib != nil; sib = sib.NextSibling {
		if sib.Type != html.ElementNode {
			appendNodeText(&sb, sib)
			continue
		}
		switch sib.Data {
		case "h3":
			qt := strings.TrimSpace(textContent(sib))
			if strings.HasPrefix(qt, "Preferred qualifications") {
				appendNodeText(&sb, sib)
				continue
			}
			return strings.TrimSpace(sb.String())
		case "div":
			return strings.TrimSpace(sb.String())
		case "br":
			continue
		default:
			appendNodeText(&sb, sib)
		}
	}
	return strings.TrimSpace(sb.String())
}

func parseQualificationsHTML(rawHTML string) string {
	doc, err := html.Parse(strings.NewReader("<div>" + rawHTML + "</div>"))
	if err != nil {
		return ""
	}
	var sb strings.Builder
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		appendNodeText(&sb, c)
	}
	return strings.TrimSpace(strings.ReplaceAll(sb.String(), "\r", ""))
}

func parseHTMLToText(rawHTML string) string {
	doc, err := html.Parse(strings.NewReader("<div>" + rawHTML + "</div>"))
	if err != nil {
		return ""
	}
	var sb strings.Builder
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		appendNodeText(&sb, c)
	}
	return strings.TrimSpace(strings.ReplaceAll(sb.String(), "\r", ""))
}

func appendNodeText(sb *strings.Builder, n *html.Node) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		return
	}
	if n.Type != html.ElementNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			appendNodeText(sb, c)
		}
		return
	}
	switch n.Data {
	case "h1", "h2", "h3", "h4", "h5", "p", "li":
		sb.WriteByte('\n')
	case "br":
		if !strings.HasSuffix(sb.String(), "\n") {
			sb.WriteByte('\n')
		}
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		appendNodeText(sb, c)
	}
	switch n.Data {
	case "h1", "h2", "h3", "h4", "h5", "p":
		sb.WriteByte('\n')
	}
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// spanChildText returns the text of the first direct <span> child of sel.
func spanChildText(sel *goquery.Selection) string {
	return strings.TrimSpace(sel.ChildrenFiltered("span").First().Text())
}

// extractDSRaw pulls the top-level JSON array of the AF_initDataCallback for key.
func extractDSRaw(doc *goquery.Document, key string) ([]json.RawMessage, error) {
	script := findDSScript(doc, key)
	if script == "" {
		return nil, fmt.Errorf("%s script not found", key)
	}
	m := dsDataPattern.FindStringSubmatch(script)
	if m == nil {
		return nil, fmt.Errorf("%s data array not found", key)
	}
	var data []json.RawMessage
	if err := json.Unmarshal([]byte(m[1]), &data); err != nil {
		return nil, fmt.Errorf("decode %s data: %w", key, err)
	}
	return data, nil
}

func findDSScript(doc *goquery.Document, key string) string {
	target := fmt.Sprintf("key: '%s'", key)
	altTarget := fmt.Sprintf(`key: "%s"`, key)
	var found string
	doc.Find("script").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		t := s.Text()
		if strings.Contains(t, target) || strings.Contains(t, altTarget) {
			found = t
			return false
		}
		return true
	})
	return found
}

func dsString(rec []json.RawMessage, i int) string {
	if i >= len(rec) {
		return ""
	}
	var s string
	_ = json.Unmarshal(rec[i], &s)
	return s
}

func dsInt(rec []json.RawMessage, i int) int {
	if i >= len(rec) {
		return 0
	}
	var val int
	if err := json.Unmarshal(rec[i], &val); err == nil {
		return val
	}
	var b bool
	if err := json.Unmarshal(rec[i], &b); err == nil && b {
		return 1
	}
	var s string
	if err := json.Unmarshal(rec[i], &s); err == nil {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
			return n
		}
	}
	return 0
}

func dsHTMLField(rec []json.RawMessage, i int) string {
	if i >= len(rec) {
		return ""
	}
	var pair []json.RawMessage
	if err := json.Unmarshal(rec[i], &pair); err != nil || len(pair) < 2 {
		return ""
	}
	var s string
	_ = json.Unmarshal(pair[1], &s)
	return s
}

func dsLocations(rec []json.RawMessage, i int) string {
	if i >= len(rec) {
		return ""
	}
	var locs [][]json.RawMessage
	if err := json.Unmarshal(rec[i], &locs); err != nil {
		return ""
	}
	var names []string
	for _, loc := range locs {
		if len(loc) == 0 {
			continue
		}
		var name string
		if json.Unmarshal(loc[0], &name) == nil && name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, "; ")
}

func mapExperienceLevel(code int) string {
	switch code {
	case 1:
		return "Early"
	case 2:
		return "Mid"
	case 3:
		return "Advanced"
	case 4:
		return "Intern and Apprentice"
	case 5:
		return "Director+"
	default:
		return ""
	}
}
