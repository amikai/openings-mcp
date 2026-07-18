package mynavi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var jobIDFromHrefPattern = regexp.MustCompile(`jobinfo-(\d+-\d+-\d+-\d+)`)

// parseJobsHTML parses the total hit count and job cassettes out of a
// /list/ results page. A page with no result counter is not a results page
// at all (selector drift, or the legacy fallback page the site serves for
// malformed URL prefixes) and must error rather than read as zero results.
func parseJobsHTML(doc *goquery.Document) (*JobsResponse, error) {
	numText := strings.TrimSpace(doc.Find("p.result__num em").First().Text())
	if numText == "" {
		return nil, errors.New("unrecognized search response: no result counter on page")
	}
	total, err := strconv.Atoi(numText)
	if err != nil {
		return nil, fmt.Errorf("unrecognized search response: result counter %q is not a number", numText)
	}

	var jobs []Job
	for _, card := range doc.Find("div.cassetteRecruit__content").EachIter() {
		if job, ok := parseJobCard(card); ok {
			jobs = append(jobs, job)
		}
	}
	return &JobsResponse{Total: total, Jobs: jobs}, nil
}

func parseJobCard(card *goquery.Selection) (Job, bool) {
	var job Job

	// h3.cassetteRecruit__name reads "{company name} | {catch copy}".
	name := strings.TrimSpace(card.Find("h3.cassetteRecruit__name").First().Text())
	job.Company, job.CatchCopy, _ = strings.Cut(name, " | ")
	job.Company = strings.TrimSpace(job.Company)
	job.CatchCopy = strings.TrimSpace(job.CatchCopy)

	if a := card.Find("p.cassetteRecruit__copy a").First(); a.Length() > 0 {
		job.Title = strings.TrimSpace(a.Text())
		if href, ok := a.Attr("href"); ok {
			if m := jobIDFromHrefPattern.FindStringSubmatch(href); m != nil {
				job.ID = m[1]
			}
		}
	}

	job.EmploymentStatus = strings.TrimSpace(card.Find("span.labelEmploymentStatus").First().Text())
	for _, s := range card.Find("span.labelCondition").EachIter() {
		if text := strings.TrimSpace(s.Text()); text != "" {
			job.Conditions = append(job.Conditions, text)
		}
	}

	for _, row := range card.Find("table.tableCondition tr").EachIter() {
		head := strings.TrimSpace(row.Find("th.tableCondition__head").First().Text())
		body := strings.TrimSpace(row.Find("td.tableCondition__body").First().Text())
		switch head {
		case "仕事内容":
			job.Description = body
		case "対象となる方":
			job.Target = body
		case "勤務地":
			job.Location = body
		case "給与":
			job.Salary = body
		case "初年度年収":
			job.FirstYearIncome = body
		}
	}

	job.UpdatedDate = strings.TrimSpace(card.Find("p.cassetteRecruit__updateDate span").First().Text())
	job.EndDate = strings.TrimSpace(card.Find("p.cassetteRecruit__endDate span").First().Text())

	return job, job.ID != "" && job.Title != ""
}

// jobPostingLD mirrors the detail page's schema.org JobPosting JSON-LD.
// String-or-number and object-or-array shapes use tolerant types so a
// posting that serializes differently doesn't fail the whole parse.
type jobPostingLD struct {
	Type               string `json:"@type"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	EmploymentType     string `json:"employmentType"`
	Industry           string `json:"industry"`
	OccupationalCat    string `json:"occupationalCategory"`
	DatePosted         string `json:"datePosted"`
	ValidThrough       string `json:"validThrough"`
	URL                string `json:"url"`
	ExperienceRequired string `json:"experienceRequirements"`
	WorkHours          string `json:"workHours"`
	JobBenefits        string `json:"jobBenefits"`
	HiringOrganization struct {
		Name   string `json:"name"`
		SameAs string `json:"sameAs"`
	} `json:"hiringOrganization"`
	JobLocation oneOrMany[placeLD] `json:"jobLocation"`
	BaseSalary  struct {
		Currency string `json:"currency"`
		Value    struct {
			MinValue flexString `json:"minValue"`
			MaxValue flexString `json:"maxValue"`
			UnitText string     `json:"unitText"`
		} `json:"value"`
	} `json:"baseSalary"`
}

type placeLD struct {
	Address struct {
		Region   string `json:"addressRegion"`
		Locality string `json:"addressLocality"`
	} `json:"address"`
}

// oneOrMany accepts both a bare JSON object and an array of them.
type oneOrMany[T any] []T

func (o *oneOrMany[T]) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '[' {
		return json.Unmarshal(data, (*[]T)(o))
	}
	var one T
	if err := json.Unmarshal(data, &one); err != nil {
		return err
	}
	*o = []T{one}
	return nil
}

// flexString accepts both a JSON string and a bare number.
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, (*string)(f))
	}
	*f = flexString(data)
	return nil
}

// parseJobDetailHTML extracts the JobPosting JSON-LD from a detail page.
// The page carries a second JSON-LD block (a BreadcrumbList), so blocks are
// selected by @type, not position.
func parseJobDetailHTML(doc *goquery.Document, id string) (*JobDetailResponse, error) {
	var posting *jobPostingLD
	for _, script := range doc.Find(`script[type="application/ld+json"]`).EachIter() {
		var ld jobPostingLD
		if err := json.Unmarshal([]byte(script.Text()), &ld); err != nil {
			continue
		}
		if ld.Type == "JobPosting" {
			posting = &ld
			break
		}
	}
	if posting == nil {
		return nil, errors.New("no JobPosting JSON-LD on page")
	}

	detail := JobDetailResponse{
		ID:                     id,
		URL:                    posting.URL,
		Title:                  posting.Title,
		Company:                posting.HiringOrganization.Name,
		CompanyURL:             posting.HiringOrganization.SameAs,
		EmploymentType:         posting.EmploymentType,
		Industry:               posting.Industry,
		OccupationalCategory:   posting.OccupationalCat,
		DatePosted:             posting.DatePosted,
		ValidThrough:           posting.ValidThrough,
		SalaryCurrency:         posting.BaseSalary.Currency,
		SalaryMin:              string(posting.BaseSalary.Value.MinValue),
		SalaryMax:              string(posting.BaseSalary.Value.MaxValue),
		SalaryUnit:             posting.BaseSalary.Value.UnitText,
		Description:            htmlToText(posting.Description),
		ExperienceRequirements: htmlToText(posting.ExperienceRequired),
		WorkHours:              htmlToText(posting.WorkHours),
		JobBenefits:            htmlToText(posting.JobBenefits),
	}
	for _, place := range posting.JobLocation {
		detail.Locations = append(detail.Locations, Location{
			Region:   place.Address.Region,
			Locality: place.Address.Locality,
		})
	}
	return &detail, nil
}

// htmlToText flattens an HTML-valued JSON-LD field to plain text: block
// elements and <br> become newlines, entities are decoded, runs of blank
// lines collapse. Some employers' section-divider lines carry a
// double-escaped "&amp;lt;br&amp;gt;" that survives one round of entity
// decoding as a literal "&lt;br&gt;"; those become newlines too. A field
// with no markup passes through unchanged.
func htmlToText(s string) string {
	if s == "" {
		return ""
	}
	nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	})
	if err != nil {
		return s
	}
	var sb strings.Builder
	for _, n := range nodes {
		appendNodeText(&sb, n)
	}
	text := strings.ReplaceAll(sb.String(), "&lt;br&gt;", "\n")
	return strings.TrimSpace(blankLinePattern.ReplaceAllString(text, "\n\n"))
}

var blankLinePattern = regexp.MustCompile(`\n{3,}`)

// appendNodeText flattens a node's text, inserting newlines around
// block-level elements so a rich-text field reads as plain text.
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
