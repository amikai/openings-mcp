package nodesk

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Job is one search hit from the jobPosts index.
type Job struct {
	ID          string // permalink slug, e.g. "sticker-mule-software-engineer"; pass to [Client.Detail]
	ObjectID    string // Algolia record ID, an opaque hash; ID is the durable identifier
	Title       string
	Company     string
	CompanyURL  string   // NoDesk company profile page; often empty
	URL         string   // job page URL
	Role        string   // site role category, e.g. "Engineering"
	Types       []string // employment types, e.g. "Full-Time", "Contract"
	Keywords    []string // tags, e.g. "Golang", "Backend"
	Locations   []string // applicant location display names, e.g. "Worldwide", "USA Only"
	Regions     []string // applicantLocationRegions labels, e.g. "Remote - Europe"; the reliable location signal
	BaseSalary  string   // display range, e.g. "$150K – $250K"; empty when unlisted
	DateLabel   string   // site display label: "Featured", "Today", "1d", …
	PublishedAt time.Time
	LogoURL     string
	Featured    bool
}

// searchResponse mirrors the subset of Algolia's query response the
// client consumes.
type searchResponse struct {
	Hits        []hit `json:"hits"`
	NbHits      int   `json:"nbHits"`
	Page        int   `json:"page"`
	NbPages     int   `json:"nbPages"`
	HitsPerPage int   `json:"hitsPerPage"`
	Facets      struct {
		SearchFilter             map[string]int `json:"searchFilter"`
		ApplicantLocationRegions map[string]int `json:"applicantLocationRegions"`
	} `json:"facets"`
}

// hit is one raw jobPosts record. Fields that render as `false` instead
// of null when absent use falsible types.
type hit struct {
	ObjectID string `json:"objectID"`
	Title    string `json:"title"`
	Company  struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"company"`
	Permalink       string         `json:"permalink"`
	Role            named          `json:"role"`
	EmploymentTypes []named        `json:"employmentTypes"`
	Keywords        []named        `json:"keywords"`
	Locations       []named        `json:"applicantLocations"`
	Regions         []string       `json:"applicantLocationRegions"`
	BaseSalary      falsibleString `json:"baseSalary"`
	Date            string         `json:"date"`
	DatePublished   string         `json:"datePublished"`
	Logo            string         `json:"logo"`
	Highlight       bool           `json:"highlight"`
	IsAd            bool           `json:"is_ad"`
}

// named is the index's recurring {name, url, comma} shape; only the name
// carries job data (url is site navigation, comma is list rendering).
type named struct {
	Name string `json:"name"`
}

// falsibleString decodes fields the index publishes as either a string or
// literal false, mapping false to "".
type falsibleString string

func (f *falsibleString) UnmarshalJSON(b []byte) error {
	if string(b) == "false" {
		*f = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*f = falsibleString(s)
	return nil
}

func (h hit) toJob(siteBaseURL string) Job {
	return Job{
		ID:          permalinkSlug(h.Permalink),
		ObjectID:    h.ObjectID,
		Title:       h.Title,
		Company:     h.Company.Name,
		CompanyURL:  absoluteURL(siteBaseURL, h.Company.URL),
		URL:         absoluteURL(siteBaseURL, h.Permalink),
		Role:        h.Role.Name,
		Types:       names(h.EmploymentTypes),
		Keywords:    names(h.Keywords),
		Locations:   names(h.Locations),
		Regions:     h.Regions,
		BaseSalary:  string(h.BaseSalary),
		DateLabel:   h.Date,
		PublishedAt: parseSiteTime(h.DatePublished),
		LogoURL:     absoluteURL(siteBaseURL, h.Logo),
		Featured:    h.Highlight,
	}
}

func names(ns []named) []string {
	if len(ns) == 0 {
		return nil
	}
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.Name
	}
	return out
}

// permalinkSlug extracts <slug> from a "/remote-jobs/<slug>/" permalink.
func permalinkSlug(permalink string) string {
	s := strings.TrimPrefix(permalink, "/remote-jobs/")
	return strings.Trim(s, "/")
}

// absoluteURL resolves the index's site-relative paths ("/remote-jobs/…")
// against the site base URL, passing through absolute URLs and "".
func absoluteURL(siteBaseURL, path string) string {
	if path == "" || strings.Contains(path, "://") {
		return path
	}
	return siteBaseURL + path
}

// parseSiteTime parses the index's zone-less "2026-06-19T14:15:26"
// timestamps; an unparseable value yields the zero time.
func parseSiteTime(s string) time.Time {
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// JobDetail is one job page: the JobPosting JSON-LD block plus the
// page's outbound apply link.
type JobDetail struct {
	ID              string
	URL             string
	Title           string
	Company         string
	CompanyLinks    []string // employer sites/socials from sameAs
	CompanyLogoURL  string
	DescriptionHTML string
	Types           []string // schema.org codes, e.g. "FULL_TIME", "CONTRACTOR"
	LocationType    string   // "TELECOMMUTE" on every posting observed
	Locations       []string // from JSON-LD; boilerplate "Anywhere" even on region-locked postings — prefer [Job.Regions]
	Salary          *Salary  // nil when unlisted
	DatePosted      time.Time
	ValidThrough    time.Time
	ApplyURL        string // outbound application link (employer site)
}

// Salary is a JSON-LD MonetaryAmount range.
type Salary struct {
	Currency string
	Min      float64
	Max      float64
	Unit     string // e.g. "YEAR"
}

// jobPostingLD mirrors the page's schema.org JobPosting block.
type jobPostingLD struct {
	Type               string       `json:"@type"`
	Title              string       `json:"title"`
	Description        string       `json:"description"`
	DatePosted         string       `json:"datePosted"`
	ValidThrough       string       `json:"validThrough"`
	EmploymentType     stringOrList `json:"employmentType"`
	JobLocationType    string       `json:"jobLocationType"`
	HiringOrganization struct {
		Name   string       `json:"name"`
		SameAs stringOrList `json:"sameAs"`
		Logo   string       `json:"logo"`
	} `json:"hiringOrganization"`
	ApplicantLocationRequirements oneOrMany[struct {
		Name string `json:"name"`
	}] `json:"applicantLocationRequirements"`
	BaseSalary *struct {
		Currency string `json:"currency"`
		Value    struct {
			Min  float64 `json:"minValue"`
			Max  float64 `json:"maxValue"`
			Unit string  `json:"unitText"`
		} `json:"value"`
	} `json:"baseSalary"`
}

// stringOrList decodes schema.org fields that are a bare string for one
// value and an array for several.
type stringOrList []string

func (s *stringOrList) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '[' {
		return json.Unmarshal(b, (*[]string)(s))
	}
	var one string
	if err := json.Unmarshal(b, &one); err != nil {
		return err
	}
	*s = []string{one}
	return nil
}

// oneOrMany decodes schema.org fields that are a bare object for one
// value and an array for several.
type oneOrMany[T any] []T

func (o *oneOrMany[T]) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '[' {
		return json.Unmarshal(b, (*[]T)(o))
	}
	var one T
	if err := json.Unmarshal(b, &one); err != nil {
		return err
	}
	*o = []T{one}
	return nil
}

// parseDetailPage extracts the JobPosting JSON-LD block and apply URL
// from a job page. ID and URL are filled in by the caller.
func parseDetailPage(r io.Reader) (*JobDetail, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	var ld *jobPostingLD
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		var candidate jobPostingLD
		if json.Unmarshal([]byte(s.Text()), &candidate) == nil && candidate.Type == "JobPosting" {
			ld = &candidate
			return false
		}
		return true
	})
	if ld == nil {
		return nil, fmt.Errorf("no JobPosting JSON-LD block in page")
	}

	detail := &JobDetail{
		Title:           ld.Title,
		Company:         ld.HiringOrganization.Name,
		CompanyLinks:    ld.HiringOrganization.SameAs,
		CompanyLogoURL:  ld.HiringOrganization.Logo,
		DescriptionHTML: ld.Description,
		Types:           ld.EmploymentType,
		LocationType:    ld.JobLocationType,
		DatePosted:      parseLDDate(ld.DatePosted),
		ValidThrough:    parseLDDate(ld.ValidThrough),
		ApplyURL:        doc.Find("a[data-apply-url]").AttrOr("data-apply-url", ""),
	}
	for _, loc := range ld.ApplicantLocationRequirements {
		detail.Locations = append(detail.Locations, loc.Name)
	}
	if ld.BaseSalary != nil {
		detail.Salary = &Salary{
			Currency: ld.BaseSalary.Currency,
			Min:      ld.BaseSalary.Value.Min,
			Max:      ld.BaseSalary.Value.Max,
			Unit:     ld.BaseSalary.Value.Unit,
		}
	}
	return detail, nil
}

// parseLDDate parses the JSON-LD "2026-07-10" date-only layout; an
// unparseable value yields the zero time.
func parseLDDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}
	}
	return t
}
