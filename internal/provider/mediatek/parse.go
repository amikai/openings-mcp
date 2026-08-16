package mediatek

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func parseDetailHTML(doc *goquery.Document, jobID string) (JobDetail, bool) {
	detail := JobDetail{ID: jobID}
	detail.Title = strings.TrimSpace(doc.Find("main h1").First().Text())
	if detail.Title == "" {
		return JobDetail{}, false
	}

	for _, section := range doc.Find("main span").EachIter() {
		label := strings.TrimSpace(section.Text())
		if !isDetailLabel(label) {
			continue
		}
		value := strings.TrimSpace(section.Parent().Find("h2").First().Text())
		switch label {
		case "Category":
			detail.Category = value
		case "Location":
			detail.Location = value
		case "Experience":
			detail.Experience = value
		case "Education":
			detail.Education = value
		}
	}

	for _, heading := range doc.Find("main h3").EachIter() {
		label := strings.TrimSpace(heading.Text())
		container := heading.Parent()
		switch label {
		case "Job Description":
			detail.Description = strings.TrimSpace(container.Find("p").First().Text())
		case "Main Requirements and Qualifications":
			var lines []string
			for _, li := range container.Find("li").EachIter() {
				lines = append(lines, strings.TrimSpace(li.Text()))
			}
			detail.Qualifications = strings.Join(lines, "\n")
		}
	}
	return detail, true
}

func isDetailLabel(value string) bool {
	switch value {
	case "Category", "Location", "Experience", "Education":
		return true
	default:
		return false
	}
}
