package engage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// workIDPathRE matches a posting link's /<slug>/work_<id>/ path and captures
// the numeric id. The link also carries a "?via_recruit_page=1" query string
// on the board, which this pattern ignores.
var workIDPathRE = regexp.MustCompile(`/work_(\d+)/`)

// parseBoard extracts every dt.category / dd.dataArea pair from a tenant
// board page. Each employment-type category renders as its own
// dl.jobList — a page with N categories carries N separate dl.jobList
// elements, not one dl holding every dt.category — so this walks all of
// them, not just the first. Returns [ErrEmptyBoard] when no dl.jobList is
// present or every category parses to zero jobs.
func parseBoard(doc *goquery.Document) (*Board, error) {
	lists := doc.Find("dl.jobList")
	if lists.Length() == 0 {
		return nil, ErrEmptyBoard
	}

	var categories []Category
	lists.Each(func(_ int, list *goquery.Selection) {
		list.Children().Each(func(_ int, s *goquery.Selection) {
			switch {
			case s.Is("dt.category"):
				categories = append(categories, Category{Name: normSpace(s.Text())})
			case s.Is("dd.dataArea") && len(categories) > 0:
				cur := &categories[len(categories)-1]
				if job, ok := parseJobEntry(s); ok {
					job.EmploymentType = cur.Name
					cur.Jobs = append(cur.Jobs, job)
				}
			}
		})
	})

	total := 0
	for i := range categories {
		categories[i].AtCap = len(categories[i].Jobs) == CategoryCap
		total += len(categories[i].Jobs)
	}
	if total == 0 {
		return nil, ErrEmptyBoard
	}
	return &Board{Categories: categories}, nil
}

// parseJobEntry reads one dd.dataArea's a.linkKoma into a [Job]. Returns
// false when the anchor's href doesn't carry a recognizable work id (e.g.
// markup drift), so the caller can skip the entry rather than emit a
// zero-value job.
func parseJobEntry(dd *goquery.Selection) (Job, bool) {
	a := dd.Find("a.linkKoma").First()
	href, _ := a.Attr("href")
	m := workIDPathRE.FindStringSubmatch(href)
	if m == nil {
		return Job{}, false
	}
	dateText := normSpace(a.Find("div.date").First().Text())
	dateText = strings.TrimPrefix(dateText, "最終更新日：")
	return Job{
		WorkID:      m[1],
		Title:       normSpace(a.Find("h2.catch").First().Text()),
		Salary:      normSpace(a.Find("dt.label--income").First().Next().Text()),
		Area:        normSpace(a.Find("dt.label--area").First().Next().Text()),
		LastUpdated: parseBoardDate(dateText),
	}, true
}

// parseBoardDate parses the board's "2026/7/16" style date (no zero
// padding). An unparsable or empty value yields the zero time rather than an
// error — a malformed date on one listing shouldn't fail the whole board.
func parseBoardDate(s string) time.Time {
	t, err := time.Parse("2006/1/2", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// jsonLDPosting is the schema.org JobPosting block embedded in every detail
// page, trimmed to the fields this package reads.
type jsonLDPosting struct {
	Type               string          `json:"@type"`
	DatePosted         string          `json:"datePosted"`
	Description        string          `json:"description"`
	Title              string          `json:"title"`
	EmploymentType     string          `json:"employmentType"`
	DirectApply        bool            `json:"directApply"`
	BaseSalary         json.RawMessage `json:"baseSalary"`
	HiringOrganization struct {
		Name   string `json:"name"`
		Logo   string `json:"logo"`
		SameAs string `json:"sameAs"`
	} `json:"hiringOrganization"`
	Identifier struct {
		Value string `json:"value"`
	} `json:"identifier"`
	JobLocation struct {
		Address struct {
			AddressCountry  string `json:"addressCountry"`
			AddressRegion   string `json:"addressRegion"`
			AddressLocality string `json:"addressLocality"`
			StreetAddress   string `json:"streetAddress"`
			PostalCode      string `json:"postalCode"`
		} `json:"address"`
	} `json:"jobLocation"`
}

// monetaryAmountWire is one baseSalary entry's wire shape. minValue/maxValue
// arrive as JSON strings, not numbers, on every tenant observed.
type monetaryAmountWire struct {
	Currency string `json:"currency"`
	Value    struct {
		UnitText string `json:"unitText"`
		MinValue string `json:"minValue"`
		MaxValue string `json:"maxValue"`
	} `json:"value"`
}

// parseJobDetail extracts a posting from its detail page's application/ld+json
// script, falling back to the page's h2.item / dd.data sections for
// structured content the JSON-LD doesn't carry (see doc.go).
func parseJobDetail(doc *goquery.Document) (*JobDetail, error) {
	script := doc.Find(`script[type="application/ld+json"]`).First()
	if script.Length() == 0 {
		// A minority of postings render the normal detail page without the
		// JSON-LD block (~1 in 34 sampled across tenants, e.g.
		// aspark-tokyo/work_17068421). The HTML carries the same content, so
		// fall back to it rather than failing the posting.
		return parseJobDetailHTML(doc)
	}
	var posting jsonLDPosting
	if err := json.Unmarshal([]byte(script.Text()), &posting); err != nil {
		return nil, fmt.Errorf("parse JobPosting JSON-LD: %w", err)
	}
	if posting.Type != "JobPosting" {
		return nil, fmt.Errorf("unexpected JSON-LD @type %q, want JobPosting", posting.Type)
	}

	salaries, err := parseSalaries(posting.BaseSalary)
	if err != nil {
		return nil, fmt.Errorf("parse baseSalary: %w", err)
	}

	return &JobDetail{
		WorkID:          posting.Identifier.Value,
		Title:           posting.Title,
		DescriptionHTML: posting.Description,
		DatePosted:      parseEngageTime(posting.DatePosted),
		EmploymentType:  posting.EmploymentType,
		Organization: Organization{
			Name:   posting.HiringOrganization.Name,
			Logo:   posting.HiringOrganization.Logo,
			SameAs: posting.HiringOrganization.SameAs,
		},
		Salaries: salaries,
		Location: Location{
			Country:    posting.JobLocation.Address.AddressCountry,
			Region:     posting.JobLocation.Address.AddressRegion,
			Locality:   posting.JobLocation.Address.AddressLocality,
			Street:     posting.JobLocation.Address.StreetAddress,
			PostalCode: posting.JobLocation.Address.PostalCode,
		},
		DirectApply: posting.DirectApply,
		Sections:    parseSections(doc),
	}, nil
}

// parseSalaries decodes baseSalary, which engage renders as either a single
// MonetaryAmount object (the common case) or an array of them (tenants that
// publish both an annual and a monthly figure for one posting, e.g.
// cookbiz_jobs). A missing or null field yields a nil slice, not an error.
func parseSalaries(raw json.RawMessage) ([]Salary, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []Salary
	if raw[0] == '[' {
		var wire []monetaryAmountWire
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, err
		}
		for _, w := range wire {
			out = append(out, salaryFromWire(w))
		}
		return out, nil
	}
	var w monetaryAmountWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, err
	}
	return []Salary{salaryFromWire(w)}, nil
}

func salaryFromWire(w monetaryAmountWire) Salary {
	return Salary{
		Currency: w.Currency,
		UnitText: w.Value.UnitText,
		MinValue: w.Value.MinValue,
		MaxValue: w.Value.MaxValue,
	}
}

// parseSections reads every dl.dataSet block that carries an h2.item
// heading — the detail page's own rendering of description sub-sections
// (仕事内容, 応募資格・条件, 勤務時間, ...). Board-page dl.dataSet blocks (salary/area)
// have no h2.item and are skipped.
// parseJobDetailHTML builds a posting from the detail page's own markup, for
// the postings engage renders without a JSON-LD block. The page layout is
// identical either way, so the headline, employer, and section bodies are all
// recoverable; the machine-readable extras the JSON-LD carries — datePosted,
// employmentType, structured salary bounds, and the postal address — have no
// HTML equivalent and stay zero. The salary and location text survive as
// sections.
func parseJobDetailHTML(doc *goquery.Document) (*JobDetail, error) {
	sections := parseSections(doc)
	title, company := parseDetailHeadline(doc)
	if title == "" && len(sections) == 0 {
		return nil, errors.New("no application/ld+json script and no parsable detail markup: not a recognized detail page")
	}
	if company == "" {
		company = parseDetailCompany(doc)
	}

	return &JobDetail{
		Title:        title,
		Organization: Organization{Name: company},
		Sections:     sections,
	}, nil
}

// parseDetailHeadline splits the detail page's h1, which engage renders as
// "<job title> / <company name>".
func parseDetailHeadline(doc *goquery.Document) (title, company string) {
	h1 := normSpace(doc.Find("h1").First().Text())
	if h1 == "" {
		return "", ""
	}
	before, after, ok := strings.Cut(h1, "/")
	if !ok {
		return h1, ""
	}
	return normSpace(before), normSpace(after)
}

// parseDetailCompany reads the employer from the company table, the fallback
// for a page whose h1 carries no company half.
func parseDetailCompany(doc *goquery.Document) string {
	var name string
	doc.Find("th").EachWithBreak(func(_ int, th *goquery.Selection) bool {
		if normSpace(th.Text()) != "会社名" {
			return true
		}
		name = normSpace(th.Next().Text())
		return false
	})
	return name
}

func parseSections(doc *goquery.Document) []Section {
	var sections []Section
	doc.Find("dl.dataSet").Each(func(_ int, dl *goquery.Selection) {
		h2 := dl.Find("h2.item").First()
		if h2.Length() == 0 {
			return
		}
		text := normSpace(dl.Find("dd.data").First().Text())
		if text == "" {
			return
		}
		sections = append(sections, Section{
			Heading: normSpace(h2.Text()),
			Text:    text,
		})
	})
	return sections
}

// parseEngageTime parses engage's RFC 3339 datePosted. An unparsable or
// empty value yields the zero time rather than an error.
func parseEngageTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func normSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
