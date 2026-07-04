package tsmc

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var totalRE = regexp.MustCompile(`\d+\s*-\s*\d+\s*of\s*(\d+)`)

// parseSearchHTML parses job cards and total count from a search results page.
func parseSearchHTML(doc *goquery.Document) ([]Job, int) {
	total := 0
	if m := totalRE.FindStringSubmatch(doc.Text()); m != nil {
		total, _ = strconv.Atoi(m[1])
	}

	var jobs []Job
	for _, article := range doc.Find("article.article--result").EachIter() {
		if job, ok := parseJobCard(article); ok {
			jobs = append(jobs, job)
		}
	}
	if total == 0 {
		total = len(jobs)
	}
	return jobs, total
}

func parseJobCard(article *goquery.Selection) (Job, bool) {
	var job Job

	if a := article.Find("a.link").First(); a.Length() > 0 {
		href, _ := a.Attr("href")
		if u, err := url.Parse(href); err == nil {
			job.ID = u.Query().Get("jobId")
		}
		job.Title = strings.TrimSpace(a.Text())
	}
	job.Location = strings.TrimSpace(article.Find("span.list-item-location").First().Text())
	job.CareerArea = strings.TrimSpace(article.Find("span.list-item-careerArea").First().Text())
	job.EmploymentType = strings.TrimSpace(article.Find("span.list-item-employmentType").First().Text())

	// zh_TW prefix: "職務張貼日: "
	if t := strings.TrimSpace(article.Find("span.list-item-posted").First().Text()); t != "" {
		_, after, _ := strings.Cut(t, ": ")
		job.Posted = after
	}

	// xq -q "a[id^='shareButton--email']" -a "href" → mailto href, URL-decode body to get slug
	if a := article.Find(`a[id^='shareButton--email']`).First(); a.Length() > 0 {
		href, _ := a.Attr("href")
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
	}

	return job, job.ID != ""
}

// parseDetailHTML parses a job detail page.
// Field labels (zh_TW): "公司名稱", "工作地點", "專業領域", "職別", "職務類型", "職務張貼日"
// Field labels (zh_TW): "職務說明" (Responsibilities), "職務要求" (Qualifications) → multiline div children
func parseDetailHTML(doc *goquery.Document) (JobDetailResponse, bool) {
	var detail JobDetailResponse

	if link := doc.Find(`link[rel="canonical"]`).First(); link.Length() > 0 {
		href, _ := link.Attr("href")
		if href != "" {
			if u, err := url.Parse(href); err == nil {
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				if len(parts) >= 2 {
					detail.ID = parts[len(parts)-1]
					detail.Slug = parts[len(parts)-2]
				}
			}
		}
	}

	detail.Title = strings.TrimSpace(doc.Find("h2.banner__text__title").First().Text())
	// each field group (label + value) lives in its own sibling
	// article.article--details element.
	for _, article := range doc.Find("article.article--details").EachIter() {
		parseDetailArticle(article, &detail)
	}

	return detail, detail.Title != ""
}

func parseDetailArticle(article *goquery.Selection, detail *JobDetailResponse) {
	// Collect label-value pairs from field divs, in document order.
	var label string
	for _, n := range article.Find("div.article__content__view__field__label, div.article__content__view__field__value").EachIter() {
		switch {
		case n.HasClass("article__content__view__field__label"):
			label = strings.TrimSpace(n.Text())
		case n.HasClass("article__content__view__field__value"):
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
		}
	}
}

// divChildrenText collects text from <div> children joined by newlines,
// falling back to full text content if no <div> children exist.
func divChildrenText(sel *goquery.Selection) string {
	var parts []string
	for _, c := range sel.ChildrenFiltered("div").EachIter() {
		if t := strings.TrimSpace(c.Text()); t != "" {
			parts = append(parts, t)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}
	return strings.TrimSpace(sel.Text())
}
