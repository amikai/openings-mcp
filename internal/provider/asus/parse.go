package asus

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	jobNoPattern = regexp.MustCompile(`^([A-Z0-9]{5,10})\s+`)
	pagePattern  = regexp.MustCompile(`[?&]page=(\d+)`)
)

func parseSearchHTML(r io.Reader, baseURL string) (*SearchResponse, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("parse search HTML: %w", err)
	}

	var jobs []JobSummary
	doc.Find("table.Rwd-Table tbody tr").Each(func(_ int, s *goquery.Selection) {
		title := cleanText(s.Find("td[data-label*=\"職缺名稱\"], td[data-label*=\"Job Title\"]").Text())
		if title == "" {
			return
		}

		jobNo := ""
		if m := jobNoPattern.FindStringSubmatch(title); len(m) > 1 {
			jobNo = m[1]
		}

		category := cleanText(s.Find("td[data-label*=\"職務類別\"], td[data-label*=\"Category\"]").Text())
		location := cleanText(s.Find("td[data-label*=\"工作地區\"], td[data-label*=\"Location\"]").Text())
		experience := cleanText(s.Find("td[data-label*=\"工作年資\"], td[data-label*=\"Experience\"]").Text())
		education := cleanText(s.Find("td[data-label*=\"教育程度\"], td[data-label*=\"Education\"]").Text())

		id := ""
		detailURL := ""
		if detailLink := s.Find("a[href*=\"Jobs/Detail?sn=\"], a[href*=\"Detail?sn=\"]"); detailLink.Length() > 0 {
			if href, exists := detailLink.Attr("href"); exists {
				detailURL = resolveURL(baseURL, href)
				if u, err := url.Parse(href); err == nil {
					id = u.Query().Get("sn")
				}
			}
		}

		applyURL := ""
		if applyLink := s.Find("a[href*=\"SignIn\"]"); applyLink.Length() > 0 {
			if href, exists := applyLink.Attr("href"); exists {
				applyURL = resolveURL(baseURL, href)
			}
		}

		jobs = append(jobs, JobSummary{
			ID:         id,
			JobNo:      jobNo,
			Title:      title,
			Category:   category,
			Location:   location,
			Experience: experience,
			Education:  education,
			DetailURL:  detailURL,
			ApplyURL:   applyURL,
		})
	})

	currentPage := 0
	totalPages := 0
	if len(jobs) > 0 {
		currentPage = 1
		totalPages = 1

		if activeText := cleanText(doc.Find(".pagination li.active span, .w3-pagination li.active span").Text()); activeText != "" {
			if p, err := strconv.Atoi(activeText); err == nil {
				currentPage = p
			}
		}

		if lastLink := doc.Find(".pagination li.PagedList-skipToLast a, .w3-pagination li.PagedList-skipToLast a"); lastLink.Length() > 0 {
			if href, exists := lastLink.Attr("href"); exists {
				if m := pagePattern.FindStringSubmatch(href); len(m) > 1 {
					if tp, err := strconv.Atoi(m[1]); err == nil {
						totalPages = tp
					}
				}
			}
		} else {
			// Find the highest page link if skipToLast is absent
			doc.Find(".pagination a[href*=\"page=\"], .w3-pagination a[href*=\"page=\"]").Each(func(_ int, s *goquery.Selection) {
				if href, exists := s.Attr("href"); exists {
					if m := pagePattern.FindStringSubmatch(href); len(m) > 1 {
						if tp, err := strconv.Atoi(m[1]); err == nil && tp > totalPages {
							totalPages = tp
						}
					}
				}
			})
			if totalPages < currentPage {
				totalPages = currentPage
			}
		}
	}

	return &SearchResponse{
		Jobs:        jobs,
		TotalPages:  totalPages,
		CurrentPage: currentPage,
	}, nil
}

func parseDetailHTML(r io.Reader, id string, baseURL string) (*JobDetail, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("parse detail HTML: %w", err)
	}

	title := cleanText(doc.Find(".Detail-Title h1").Text())
	if title == "" {
		return nil, fmt.Errorf("job detail not found for id %q", id)
	}

	jobNo := ""
	if m := jobNoPattern.FindStringSubmatch(title); len(m) > 1 {
		jobNo = m[1]
	}

	category := cleanText(doc.Find("table.Rwd-Table-Info td[data-label=\"職務類別\"], table.Rwd-Table-Info td[data-label=\"Category\"]").Text())
	location := cleanText(doc.Find("table.Rwd-Table-Info td[data-label=\"工作地區\"], table.Rwd-Table-Info td[data-label=\"Location\"]").Text())
	experience := cleanText(doc.Find("table.Rwd-Table-Info td[data-label=\"工作年資\"], table.Rwd-Table-Info td[data-label=\"Experience\"]").Text())
	education := cleanText(doc.Find("table.Rwd-Table-Info td[data-label=\"教育程度\"], table.Rwd-Table-Info td[data-label=\"Education\"]").Text())
	empType := cleanText(doc.Find("table.Rwd-Table-Info td[data-label=\"性質\"], table.Rwd-Table-Info td[data-label=\"Job Type\"]").Text())

	var description, requirements string
	doc.Find(".Detail-InfoText > div").Each(func(_ int, s *goquery.Selection) {
		h3 := cleanText(s.Find("h3").Text())
		text := cleanBlockText(s.Find("p"))
		if strings.Contains(h3, "工作說明") || strings.Contains(strings.ToLower(h3), "job description") || strings.Contains(strings.ToLower(h3), "description") {
			description = text
		} else if strings.Contains(h3, "需求條件") || strings.Contains(strings.ToLower(h3), "requirements") || strings.Contains(strings.ToLower(h3), "qualifications") {
			requirements = text
		}
	})

	applyURL := ""
	if applyLink := doc.Find("a[href*=\"SignIn\"][href*=\"Detail\"]"); applyLink.Length() > 0 {
		if href, exists := applyLink.Attr("href"); exists {
			applyURL = resolveURL(baseURL, href)
		}
	}

	return &JobDetail{
		ID:             id,
		JobNo:          jobNo,
		Title:          title,
		Category:       category,
		Location:       location,
		Experience:     experience,
		Education:      education,
		EmploymentType: empType,
		Description:    description,
		Requirements:   requirements,
		ApplyURL:       applyURL,
	}, nil
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.Join(strings.Fields(s), " ")
}

func cleanBlockText(s *goquery.Selection) string {
	if s.Length() == 0 {
		return ""
	}
	html, err := s.Html()
	if err != nil {
		return cleanText(s.Text())
	}
	// Convert <br> or <br/> or <p> tags to newlines
	replacer := strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n", "</p>", "\n", "<p>", "")
	text := replacer.Replace(html)
	// Strip any remaining html tags
	tagPattern := regexp.MustCompile(`<[^>]*>`)
	text = tagPattern.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")

	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}

func resolveURL(base, ref string) string {
	if ref == "" {
		return ""
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return baseURL.ResolveReference(refURL).String()
}
