package avature

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// jobDetailPathRE matches a portal JobDetail link and captures the numeric
// job id in its final path segment. Social-share links never match: they
// carry the posting URL percent-encoded inside a query string, so the
// literal "/JobDetail/" segment does not appear.
var _jobDetailPathRE = regexp.MustCompile(`/JobDetail/[^/?#]+/(\d+)(?:[?#]|$)`)

var (
	_totalOfRE      = regexp.MustCompile(`of\s+([\d,]+)`)
	_totalResultsRE = regexp.MustCompile(`([\d,]+)\s+results?`)
)

func parseSearchHTML(doc *goquery.Document) *SearchResponse {
	res := &SearchResponse{
		Jobs:    []Job{},
		Total:   parseTotal(doc),
		HasNext: doc.Find("a.paginationNextLink").Length() > 0,
	}
	// Item markup varies by portal theme, so anchor on JobDetail links and
	// dedupe by id: the title link precedes any Apply link for the same
	// posting in document order.
	index := make(map[string]int)
	for _, a := range doc.Find("a[href]").EachIter() {
		href, _ := a.Attr("href")
		m := _jobDetailPathRE.FindStringSubmatch(href)
		if m == nil {
			continue
		}
		id, title := m[1], normSpace(a.Text())
		if i, ok := index[id]; ok {
			// An icon-only title link can be empty; take the first
			// non-empty text seen for the id.
			if res.Jobs[i].Title == "" {
				res.Jobs[i].Title = title
			}
			continue
		}
		index[id] = len(res.Jobs)
		res.Jobs = append(res.Jobs, Job{
			ID:       id,
			Title:    title,
			Location: itemLocation(a),
			URL:      href,
		})
	}
	return res
}

// parseTotal reads the results legend. The legend text is the primary
// source ("1-12 of 436 results"); zero-result pages leave the text empty
// but keep an aria-label ("0 results"). No legend element means the portal
// hides counts entirely.
func parseTotal(doc *goquery.Document) int {
	legend := doc.Find(".list-controls__text__legend").First()
	if legend.Length() == 0 {
		return -1
	}
	text := normSpace(legend.Text())
	if m := _totalOfRE.FindStringSubmatch(text); m != nil {
		return atoiCommas(m[1])
	}
	for _, s := range []string{text, legend.AttrOr("aria-label", "")} {
		if m := _totalResultsRE.FindStringSubmatch(s); m != nil {
			return atoiCommas(m[1])
		}
	}
	return -1
}

// itemLocation extracts a listing item's location, trying each known portal
// theme in turn.
func itemLocation(a *goquery.Selection) string {
	item := a.Closest("article, li, .list__item").First()
	if item.Length() == 0 {
		return ""
	}
	// Bloomberg-style subtitle span.
	if loc := normSpace(item.Find(".list-item-location").First().Text()); loc != "" {
		return loc
	}
	// Koch-style label/value card fields ("Location:" / "Atlanta, Georgia").
	loc := ""
	for _, f := range item.Find(".article__content__field").EachIter() {
		if !strings.Contains(strings.ToLower(f.Find(".article__content__field__label").Text()), "location") {
			continue
		}
		loc = normSpace(f.Find(".article__content__field__value").Text())
		break
	}
	if loc != "" {
		return loc
	}
	// Deloitte-style subtitle list ("<strong>Location:</strong> <span>Zaventem</span>").
	for _, s := range item.Find("strong").EachIter() {
		if !strings.Contains(strings.ToLower(s.Text()), "location") {
			continue
		}
		loc = normSpace(s.NextFiltered("span").First().Text())
		if loc != "" {
			break
		}
	}
	if loc != "" {
		return loc
	}
	// Footer info tagged with an address icon ("<span class='icon icon-address'/> Argentina").
	if icon := item.Find(".icon-address").First(); icon.Length() > 0 {
		return normSpace(icon.Parent().Text())
	}
	return ""
}

func parseJobDetailHTML(doc *goquery.Document, id string) (*JobDetailResponse, bool) {
	d := &JobDetailResponse{ID: id, Title: detailTitle(doc)}

	// Metadata fields. Portals duplicate the section for mobile and
	// desktop, so dedupe by label keeping the first occurrence.
	seen := make(map[string]bool)
	for _, f := range doc.Find(".article__content__view__field").EachIter() {
		label := normSpace(f.Find(".article__content__view__field__label").First().Text())
		if label == "" || seen[strings.ToLower(label)] {
			continue
		}
		seen[strings.ToLower(label)] = true
		d.Fields = append(d.Fields, Field{
			Label: label,
			Value: normSpace(f.Find(".article__content__view__field__value").First().Text()),
		})
	}

	// Description: every details section's content minus the label/value
	// fields captured above. Some themes put the body in a rich-text field,
	// others directly in the section content — removing labeled fields
	// keeps both.
	var parts []string
	for _, sec := range doc.Find("article.article--details").EachIter() {
		for _, f := range sec.Find(".article__content__view__field").EachIter() {
			if f.Find(".article__content__view__field__label").Length() > 0 {
				f.Remove()
			}
		}
		for _, c := range sec.Find(".article__content").EachIter() {
			if normSpace(c.Text()) == "" {
				continue
			}
			if h, err := c.Html(); err == nil {
				parts = append(parts, h)
			}
		}
	}
	d.DescriptionHTML = strings.Join(parts, "\n")

	return d, d.Title != "" && (len(d.Fields) > 0 || d.DescriptionHTML != "")
}

// detailTitle prefers og:title (present on every observed portal), then the
// banner heading, then the raw <title> text.
func detailTitle(doc *goquery.Document) string {
	if t := normSpace(doc.Find(`meta[property="og:title"]`).AttrOr("content", "")); t != "" {
		return t
	}
	if t := normSpace(doc.Find(".banner__text__title").First().Text()); t != "" {
		return t
	}
	return normSpace(doc.Find("title").First().Text())
}

func normSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func atoiCommas(s string) int {
	n, err := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
	if err != nil {
		return -1
	}
	return n
}
