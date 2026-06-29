package tsmc

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var totalRE = regexp.MustCompile(`\d+\s*-\s*\d+\s*of\s*(\d+)`)

// parseSearchHTML parses job cards and total count from a search results page.
// xq -q "article.article--result" --html → job cards
// xq -q "article.article--result a.link" -a "href" --html → title text + jobId param
// xq -q "article.article--result span.list-item-location" --html → location
// xq -q "article.article--result span.list-item-careerArea" --html → career area
// xq -q "article.article--result span.list-item-employmentType" --html → employment type
// xq -q "article.article--result span.list-item-posted" --html → posted date (strip "Posted: ")
// xq -q "article.article--result a[id^='shareButton--email']" -a "href" --html → slug (URL-decode body path)
func parseSearchHTML(doc *html.Node) ([]Job, int) {
	total := 0
	var jobs []Job

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			if m := totalRE.FindStringSubmatch(n.Data); m != nil {
				if v, err := strconv.Atoi(m[1]); err == nil && v > total {
					total = v
				}
			}
		}
		if n.Type == html.ElementNode && n.Data == "article" &&
			hasClass(n, "article--result") {
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
	if total == 0 {
		total = len(jobs)
	}
	return jobs, total
}

func parseJobCard(article *html.Node) (Job, bool) {
	var job Job
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "a" && hasClass(n, "link"):
				// xq -q "a.link" -a "href" + text → title and jobId
				href := attrVal(n, "href")
				if u, err := url.Parse(href); err == nil {
					job.ID = u.Query().Get("jobId")
				}
				job.Title = strings.TrimSpace(textContent(n))
				return
			case n.Data == "span" && hasClass(n, "list-item-location"):
				job.Location = strings.TrimSpace(textContent(n))
				return
			case n.Data == "span" && hasClass(n, "list-item-careerArea"):
				job.CareerArea = strings.TrimSpace(textContent(n))
				return
			case n.Data == "span" && hasClass(n, "list-item-employmentType"):
				job.EmploymentType = strings.TrimSpace(textContent(n))
				return
			case n.Data == "span" && hasClass(n, "list-item-posted"):
				// zh_TW prefix: "職務張貼日: "
				t := strings.TrimSpace(textContent(n))
				_, after, _ := strings.Cut(t, ": ")
				job.Posted = after
				return
			case n.Data == "a" && strings.HasPrefix(attrVal(n, "id"), "shareButton--email"):
				// xq -q "a[id^='shareButton--email']" -a "href" → mailto href, URL-decode body to get slug
				href := attrVal(n, "href")
				if u, err := url.Parse(href); err == nil {
					body := u.Query().Get("body")
					if decoded, err := url.QueryUnescape(body); err == nil {
						// path: /zh_TW/careers/JobDetail/{slug}/{id}
						idx := strings.LastIndex(decoded, "/JobDetail/")
						if idx >= 0 {
							rest := decoded[idx+len("/JobDetail/"):]
							seg := strings.SplitN(rest, "/", 2)
							if len(seg) == 2 {
								job.Slug = seg[0]
							}
						}
					}
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(article)
	return job, job.ID != ""
}

// parseDetailHTML parses a job detail page.
// xq -q "h2.banner__text__title" --html → title
// xq -q "article.article--details .article__content__view__field__label" --html → field label
// xq -q "article.article--details .article__content__view__field__value" --html → field value
// Field labels (zh_TW): "公司名稱", "工作地點", "專業領域", "職別", "職務類型", "職務張貼日"
// Field labels (zh_TW): "職務說明" (Responsibilities), "職務要求" (Qualifications) → multiline div children
func parseDetailHTML(doc *html.Node) (JobDetailResponse, bool) {
	var detail JobDetailResponse

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "link" && attrVal(n, "rel") == "canonical":
				href := attrVal(n, "href")
				if href != "" {
					u, err := url.Parse(href)
					if err == nil {
						parts := strings.Split(strings.Trim(u.Path, "/"), "/")
						if len(parts) >= 2 {
							detail.ID = parts[len(parts)-1]
							detail.Slug = parts[len(parts)-2]
						}
					}
				}
				return
			case n.Data == "h2" && hasClass(n, "banner__text__title"):
				// xq -q "h2.banner__text__title" --html
				detail.Title = strings.TrimSpace(textContent(n))
				return
			case n.Data == "article" && hasClass(n, "article--details"):
				parseDetailArticle(n, &detail)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return detail, detail.Title != ""
}

func parseDetailArticle(article *html.Node, detail *JobDetailResponse) {
	// Collect label-value pairs from field divs.
	var label string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "div" && hasClass(n, "article__content__view__field__label"):
				label = strings.TrimSpace(textContent(n))
				return
			case n.Data == "div" && hasClass(n, "article__content__view__field__value"):
				val := divChildrenText(n)
				switch label {
				case "公司名稱":
					detail.Company = val
				case "工作地點":
					detail.Location = val
				case "專業領域":
					detail.CareerArea = val
				case "職別":
					detail.JobType = val
				case "職務類型":
					detail.EmploymentType = val
				case "職務張貼日":
					detail.Posted = val
				case "職務說明":
					detail.Responsibilities = val
				case "職務要求":
					detail.Qualifications = val
				}
				label = ""
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(article)
}

// divChildrenText collects text from <div> children joined by newlines,
// falling back to full text content if no <div> children exist.
func divChildrenText(n *html.Node) string {
	var parts []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "div" {
			t := strings.TrimSpace(textContent(c))
			if t != "" {
				parts = append(parts, t)
			}
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}
	return strings.TrimSpace(textContent(n))
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
