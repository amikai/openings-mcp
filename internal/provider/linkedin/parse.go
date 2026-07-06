package linkedin

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

var remoteKeywords = []string{"remote", "work from home", "wfh"}

// looksRemote is a heuristic, not a field LinkedIn provides: it's a plain
// substring scan over whatever text we hand it. No match defaults to false.
// A posting silent on remote work is assumed on-site, not unknown.
func looksRemote(parts ...string) bool {
	joined := strings.ToLower(strings.Join(parts, " "))
	for _, kw := range remoteKeywords {
		if strings.Contains(joined, kw) {
			return true
		}
	}
	return false
}

// parseJobsHTML parses job cards out of the seeMoreJobPostings/search HTML
// fragment.
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
	// The card div carries the canonical job ID in its urn; the link href is a
	// slugged fallback for cards that ever omit the attribute.
	urn, _ := card.Attr("data-entity-urn")
	id := jobIDFromURN(urn)

	// The sr-only span nested in the link is the canonical title; the
	// h3.base-search-card__title duplicates it but the link is checked first
	// and wins.
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

// jobIDFromURN extracts the numeric job ID from a job card's
// data-entity-urn ("urn:li:jobPosting:4422697744" -> "4422697744"). Returns
// "" when the attribute is absent or malformed so the caller can fall back to
// the href.
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

// jobIDFromHref extracts the numeric job ID: the last hyphen-separated
// segment of the path, ignoring any query string
// (".../software-engineer-at-boostdraft-4422697744?position=1..." -> "4422697744").
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

// parseJobDetailHTML parses a single jobs/view/{id} detail page.
func parseJobDetailHTML(doc *goquery.Document, id string) (*JobDetailResponse, bool) {
	detail := JobDetailResponse{ID: id}

	detail.Title = strings.TrimSpace(doc.Find("h1.topcard__title").First().Text())

	// span.topcard__flavor also holds the location, distinguished by the
	// extra topcard__flavor--bullet class; first match of each wins.
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
	// Scan only title+location, not the description: a full-text scan flips
	// Remote on any incidental mention (e.g. "not a remote position") and would
	// disagree with the search card, which sees title+location alone.
	detail.Remote = looksRemote(detail.Title, detail.Location)

	return &detail, detail.Title != ""
}

// parseCriteria reads the label/value pairs out of the job criteria list
// (Seniority level, Employment type, Job function, Industries, and
// occasionally Workplace type — not every label appears on every posting).
func parseCriteria(list *goquery.Selection, out map[string]string) {
	for _, li := range list.ChildrenFiltered("li").EachIter() {
		label := strings.TrimSpace(li.Find("h3").First().Text())

		// Prefer the value span LinkedIn documents (and openapi.yaml pins);
		// fall back to the first bare span so a markup tweak doesn't
		// silently blank the field.
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

// parseApplyURL extracts the external ATS apply URL from
// <code id="applyUrl"><!--"...?...&url=ENCODED"--></code>, present only for
// postings that redirect off-platform to apply (absent for LinkedIn's own
// Easy Apply, as in this package's fixture).
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

// appendNodeText flattens a node's text, inserting newlines around
// block-level elements so a rich-text description reads as plain text.
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
