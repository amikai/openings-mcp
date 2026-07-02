package google

import (
	"strings"

	"golang.org/x/net/html"
)

// parseJobsHTML parses job cards from search results HTML.
// xq -q "li.lLd3Je" -a "ssk" --html → "18:{jobID}"
// xq -q "li.lLd3Je h3.QJPWVe" --html → title
// xq -q "li.lLd3Je span.RP7SMd span" --html → company (inner span, skips icon)
// xq -q "li.lLd3Je span.r0wTof" --html → primary location (first match)
func parseJobsHTML(doc *html.Node) []Job {
	var jobs []Job
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" && hasClass(n, "lLd3Je") {
			if job, ok := parseJobCard(n); ok {
				jobs = append(jobs, job)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return jobs
}

func parseJobCard(li *html.Node) (Job, bool) {
	ssk := attrVal(li, "ssk")
	_, id, _ := strings.Cut(ssk, ":")
	if id == "" {
		return Job{}, false
	}

	var title, company, location string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "h3" && hasClass(n, "QJPWVe"):
				// xq -q "li.lLd3Je h3.QJPWVe" --html
				title = strings.TrimSpace(textContent(n))
				return
			case n.Data == "span" && hasClass(n, "RP7SMd"):
				// xq -q "li.lLd3Je span.RP7SMd span" --html
				company = spanChildText(n)
				return
			case n.Data == "span" && hasClass(n, "r0wTof") && location == "":
				// xq -q "li.lLd3Je span.r0wTof" --html (first match = primary location)
				location = strings.TrimSpace(textContent(n))
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(li)

	return Job{ID: id, Title: title, Company: company, Location: location}, title != ""
}

// parseJobDetailHTML parses a single job detail page.
// xq -q "title" --html → title (strip " — Google Careers" suffix)
// xq -q "main span.RP7SMd span" --html → company (inner span, skips icon)
// xq -q "main span.r0wTof" --html → location (scoped to <main> to exclude sidebar)
// xq -q "main div.aG5W3" --html → About the job section
// xq -q "main div.BDNOWe" --html → Responsibilities section
// xq -q "h3" --html (matched by text prefix "Minimum/Preferred qualifications") → Qualifications
func parseJobDetailHTML(doc *html.Node, id string) (*JobDetailResponse, bool) {
	detail := JobDetailResponse{ID: id}
	var inMain bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "title":
				t := strings.TrimSpace(textContent(n))
				detail.Title = strings.TrimSuffix(t, " — Google Careers")
				return
			case n.Data == "main":
				inMain = true
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				inMain = false
				return
			case inMain && n.Data == "span" && hasClass(n, "RP7SMd"):
				// xq -q "main span.RP7SMd span" --html
				detail.Company = spanChildText(n)
				return
			case inMain && n.Data == "span" && hasClass(n, "r0wTof") && detail.Location == "":
				// xq -q "main span.r0wTof" --html
				detail.Location = strings.TrimSpace(textContent(n))
				return
			case inMain && n.Data == "div" && hasClass(n, "aG5W3"):
				// xq -q "main div.aG5W3" --html
				var sb strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					appendNodeText(&sb, c)
				}
				detail.About = strings.TrimSpace(strings.ReplaceAll(sb.String(), "\r", ""))
				return
			case inMain && n.Data == "div" && hasClass(n, "BDNOWe"):
				// xq -q "main div.BDNOWe" --html
				var sb strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					appendNodeText(&sb, c)
				}
				detail.Responsibilities = strings.TrimSpace(strings.ReplaceAll(sb.String(), "\r", ""))
				return
			case inMain && n.Data == "h3" && detail.Qualifications == "":
				t := strings.TrimSpace(textContent(n))
				if strings.HasPrefix(t, "Minimum qualifications") {
					var sb strings.Builder
					appendNodeText(&sb, n)
					for sib := n.NextSibling; sib != nil; sib = sib.NextSibling {
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
					detail.Qualifications = strings.TrimSpace(sb.String())
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return &detail, detail.Title != ""
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

func hasClass(n *html.Node, cls string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == cls {
					return true
				}
			}
		}
	}
	return false
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
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

func spanChildText(n *html.Node) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "span" {
			return strings.TrimSpace(textContent(c))
		}
	}
	return ""
}
