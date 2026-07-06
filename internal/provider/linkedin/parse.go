package linkedin

import (
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var remoteKeywords = []string{"remote", "work from home", "wfh"}

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
func parseJobsHTML(doc *html.Node) []Job {
	var jobs []Job
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "base-search-card") {
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

func parseJobCard(card *html.Node) (Job, bool) {
	var title, company, companyURL, location, postedDate string

	// The card div carries the canonical job ID in its urn; the link href is a
	// slugged fallback for cards that ever omit the attribute.
	id := jobIDFromURN(attrVal(card, "data-entity-urn"))

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "a" && hasClass(n, "base-card__full-link"):
				if href := attrVal(n, "href"); href != "" && id == "" {
					id = jobIDFromHref(href)
				}
				if t := textContent(n); title == "" {
					title = strings.TrimSpace(t)
				}
				return
			case n.Data == "h4" && hasClass(n, "base-search-card__subtitle"):
				if a := findDescendant(n, "a"); a != nil {
					company = strings.TrimSpace(textContent(a))
					companyURL = stripQuery(attrVal(a, "href"))
				}
				return
			case n.Data == "span" && hasClass(n, "job-search-card__location"):
				location = strings.TrimSpace(textContent(n))
				return
			case n.Data == "time" && (hasClass(n, "job-search-card__listdate") || hasClass(n, "job-search-card__listdate--new")):
				postedDate = attrVal(n, "datetime")
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(card)

	// The sr-only span nested in the link is the canonical title; the
	// h3.base-search-card__title duplicates it but the link is checked first
	// above and wins since title is only set when still empty.
	if title == "" {
		if h3 := findDescendant(card, "h3"); h3 != nil && hasClass(h3, "base-search-card__title") {
			title = strings.TrimSpace(textContent(h3))
		}
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
func parseJobDetailHTML(doc *html.Node, id string) (*JobDetailResponse, bool) {
	detail := JobDetailResponse{ID: id}
	criteria := map[string]string{}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "h1" && hasClass(n, "topcard__title"):
				detail.Title = strings.TrimSpace(textContent(n))
				return
			case n.Data == "span" && hasClass(n, "topcard__flavor") && hasClass(n, "topcard__flavor--bullet") && detail.Location == "":
				detail.Location = strings.TrimSpace(textContent(n))
				return
			case n.Data == "span" && hasClass(n, "topcard__flavor") && !hasClass(n, "topcard__flavor--bullet") && detail.Company == "":
				detail.Company = strings.TrimSpace(textContent(n))
				return
			case n.Data == "span" && hasClass(n, "posted-time-ago__text") && detail.Posted == "":
				detail.Posted = strings.TrimSpace(textContent(n))
				return
			case n.Data == "div" && hasClass(n, "show-more-less-html__markup") && detail.Description == "":
				var sb strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					appendNodeText(&sb, c)
				}
				detail.Description = strings.TrimSpace(sb.String())
				return
			case n.Data == "img" && hasClass(n, "artdeco-entity-image") && detail.CompanyLogo == "":
				detail.CompanyLogo = attrVal(n, "data-delayed-url")
				return
			case n.Data == "code" && attrVal(n, "id") == "applyUrl":
				detail.ApplyURL = parseApplyURL(n)
				return
			case n.Data == "ul" && hasClass(n, "description__job-criteria-list"):
				parseCriteria(n, criteria)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

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
func parseCriteria(list *html.Node, out map[string]string) {
	for li := list.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		var label, classedValue, anyValue string
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode {
				switch {
				case n.Data == "h3" && label == "":
					label = strings.TrimSpace(textContent(n))
					return
				case n.Data == "span":
					text := strings.TrimSpace(textContent(n))
					// Prefer the value span LinkedIn documents (and openapi.yaml
					// pins); fall back to the first bare span so a markup tweak
					// doesn't silently blank the field.
					if hasClass(n, "description__job-criteria-text") {
						if classedValue == "" {
							classedValue = text
						}
					} else if anyValue == "" {
						anyValue = text
					}
					return
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(li)
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
func parseApplyURL(code *html.Node) string {
	for c := code.FirstChild; c != nil; c = c.NextSibling {
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

func findDescendant(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			return c
		}
		if found := findDescendant(c, tag); found != nil {
			return found
		}
	}
	return nil
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
