package google

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// parseJobsHTML parses job cards from search results HTML.
func parseJobsHTML(doc *goquery.Document) []Job {
	var jobs []Job
	for _, li := range doc.Find("li.lLd3Je").EachIter() {
		if job, ok := parseJobCard(li); ok {
			jobs = append(jobs, job)
		}
	}
	return jobs
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

// parseJobDetailHTML parses a single job detail page.
func parseJobDetailHTML(doc *goquery.Document, id string) (*JobDetailResponse, bool) {
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
		if sib.Type == html.ElementNode {
			if sib.Data == "h3" {
				qt := strings.TrimSpace(textContent(sib))
				if strings.HasPrefix(qt, "Preferred qualifications") {
					appendNodeText(&sb, sib)
					continue
				}
				break
			}
			if sib.Data == "div" {
				break
			}
			if sib.Data == "br" {
				continue
			}
		}
		appendNodeText(&sb, sib)
	}
	return strings.TrimSpace(sb.String())
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
		sb.WriteByte('\n')
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
