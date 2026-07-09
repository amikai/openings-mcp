package linkedin

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var remoteKeywords = []string{"remote", "work from home", "wfh"}

// looksRemote is a text heuristic, not a structured LinkedIn field; no match
// is treated as on-site.
func looksRemote(parts ...string) bool {
	joined := strings.ToLower(strings.Join(parts, " "))
	for _, kw := range remoteKeywords {
		if strings.Contains(joined, kw) {
			return true
		}
	}
	return false
}

// parseJobsHTML parses cards from a seeMoreJobPostings/search fragment.
func parseJobsHTML(doc *goquery.Document) []Job {
	var jobs []Job
	for _, card := range doc.Find("div.base-search-card").EachIter() {
		if job, ok := parseJobCard(card); ok {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

func parseJobCard(card *goquery.Selection) (Job, bool) {
	// Prefer the canonical URN; the href is a slugged fallback.
	urn, _ := card.Attr("data-entity-urn")
	id := jobIDFromURN(urn)

	// Prefer the link's sr-only title over the duplicated h3 title.
	var title string
	if a := card.Find("a.base-card__full-link").First(); a.Length() > 0 {
		if id == "" {
			if href, ok := a.Attr("href"); ok {
				id = jobIDFromHref(href)
			}
		}
		title = strings.TrimSpace(a.Text())
	}
	if title == "" {
		title = strings.TrimSpace(card.Find("h3.base-search-card__title").First().Text())
	}

	var company, companyURL string
	if a := card.Find("h4.base-search-card__subtitle a").First(); a.Length() > 0 {
		company = strings.TrimSpace(a.Text())
		href, _ := a.Attr("href")
		companyURL = stripQuery(href)
	}

	location := strings.TrimSpace(card.Find("span.job-search-card__location").First().Text())

	var postedDate string
	if t := card.Find("time.job-search-card__listdate, time.job-search-card__listdate--new").First(); t.Length() > 0 {
		postedDate, _ = t.Attr("datetime")
	}

	return Job{
		ID:         id,
		Title:      title,
		Company:    company,
		CompanyURL: companyURL,
		Location:   location,
		PostedDate: postedDate,
		Remote:     looksRemote(title, location),
	}, id != "" && title != ""
}

// jobIDFromURN extracts the numeric ID from data-entity-urn, or returns empty.
func jobIDFromURN(urn string) string {
	if urn == "" {
		return ""
	}
	idx := strings.LastIndex(urn, ":")
	if idx == -1 {
		return ""
	}
	return urn[idx+1:]
}

// jobIDFromHref extracts the final hyphen-separated path segment before the query.
func jobIDFromHref(href string) string {
	path, _, _ := strings.Cut(href, "?")
	idx := strings.LastIndex(path, "-")
	if idx == -1 {
		return ""
	}
	return path[idx+1:]
}

func stripQuery(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.RawQuery = ""
	return u.String()
}

// parseJobDetailHTML parses a jobs/view/{id} detail page.
func parseJobDetailHTML(doc *goquery.Document, id string) (*JobDetailResponse, bool) {
	detail := JobDetailResponse{ID: id}

	detail.Title = strings.TrimSpace(doc.Find("h1.topcard__title").First().Text())

	// The bullet class distinguishes location from the other flavor spans.
	for _, s := range doc.Find("span.topcard__flavor").EachIter() {
		if s.HasClass("topcard__flavor--bullet") {
			if detail.Location == "" {
				detail.Location = strings.TrimSpace(s.Text())
			}
		} else if detail.Company == "" {
			detail.Company = strings.TrimSpace(s.Text())
		}
	}

	detail.Posted = strings.TrimSpace(doc.Find("span.posted-time-ago__text").First().Text())

	if markup := doc.Find("div.show-more-less-html__markup").First(); markup.Length() > 0 {
		var sb strings.Builder
		for c := markup.Nodes[0].FirstChild; c != nil; c = c.NextSibling {
			appendNodeText(&sb, c)
		}
		detail.Description = strings.TrimSpace(sb.String())
	}

	if img := doc.Find("img.artdeco-entity-image").First(); img.Length() > 0 {
		detail.CompanyLogo, _ = img.Attr("data-delayed-url")
	}

	detail.ApplyURL = parseApplyURL(doc.Find("code#applyUrl").First())

	criteria := map[string]string{}
	if list := doc.Find("ul.description__job-criteria-list").First(); list.Length() > 0 {
		parseCriteria(list, criteria)
	}
	detail.SeniorityLevel = criteria["Seniority level"]
	detail.EmploymentType = criteria["Employment type"]
	detail.JobFunction = criteria["Job function"]
	detail.Industries = criteria["Industries"]
	// Match the same title/location scope as search cards; descriptions contain
	// incidental phrases such as "not a remote position".
	detail.Remote = looksRemote(detail.Title, detail.Location)

	return &detail, detail.Title != ""
}

// parseCriteria reads the optional label/value pairs from the criteria list.
func parseCriteria(list *goquery.Selection, out map[string]string) {
	for _, li := range list.ChildrenFiltered("li").EachIter() {
		label := strings.TrimSpace(li.Find("h3").First().Text())

		// Prefer LinkedIn's documented value span, with a bare-span fallback.
		var classedValue, anyValue string
		for _, s := range li.Find("span").EachIter() {
			text := strings.TrimSpace(s.Text())
			if s.HasClass("description__job-criteria-text") {
				if classedValue == "" {
					classedValue = text
				}
			} else if anyValue == "" {
				anyValue = text
			}
		}
		value := classedValue
		if value == "" {
			value = anyValue
		}
		if label != "" {
			out[label] = value
		}
	}
}

var applyURLPattern = regexp.MustCompile(`\?url=([^"]+)`)

// parseApplyURL extracts the external ATS URL when LinkedIn redirects off-site.
// <code id="applyUrl"><!--"...?...&url=ENCODED"--></code>, present only for
func parseApplyURL(code *goquery.Selection) string {
	if code.Length() == 0 {
		return ""
	}
	for c := code.Nodes[0].FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.CommentNode {
			continue
		}
		m := applyURLPattern.FindStringSubmatch(c.Data)
		if m == nil {
			continue
		}
		if decoded, err := url.QueryUnescape(m[1]); err == nil {
			return decoded
		}
		return m[1]
	}
	return ""
}

// appendNodeText flattens rich text, inserting newlines around block elements.
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
