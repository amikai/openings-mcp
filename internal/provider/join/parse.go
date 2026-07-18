package join

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// nextDataEnvelope is the shape of the job-ad-app's __NEXT_DATA__ script
// tag: Next.js's own server-rendered state, not a purpose-built API
// response. Only the fields this package reads are declared.
type nextDataEnvelope struct {
	Props struct {
		PageProps struct {
			InitialState struct {
				Job     *nextDataJob     `json:"job"`
				Company *nextDataCompany `json:"company"`
			} `json:"initialState"`
		} `json:"pageProps"`
	} `json:"props"`
}

type nextDataJob struct {
	ID                 int    `json:"id"`
	IdParam            string `json:"idParam"`
	Title              string `json:"title"`
	CompanyID          int    `json:"companyId"`
	Intro              string `json:"intro"`
	Tasks              string `json:"tasks"`
	Requirements       string `json:"requirements"`
	Benefits           string `json:"benefits"`
	Outro              string `json:"outro"`
	Description        string `json:"description"`
	UnifiedDescription bool   `json:"unifiedDescription"`
	WorkplaceType      string `json:"workplaceType"`
	RemoteType         string `json:"remoteType"`
	Status             string `json:"status"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
	City               *struct {
		CityName    string `json:"cityName"`
		CountryName string `json:"countryName"`
	} `json:"city"`
	Country *struct {
		Name string `json:"name"`
	} `json:"country"`
	Category *struct {
		Name string `json:"name"`
	} `json:"category"`
	EmploymentType *struct {
		Name string `json:"name"`
	} `json:"employmentType"`
}

type nextDataCompany struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// extractNextData pulls the Next.js __NEXT_DATA__ JSON blob out of an SSR
// page. Every page this package fetches (job detail, company) embeds one;
// its absence means the response isn't a job-ad-app page at all (a CDN
// error page, a redirect landing somewhere unexpected, ...).
func extractNextData(doc *goquery.Document) (*nextDataEnvelope, error) {
	sel := doc.Find("script#__NEXT_DATA__")
	if sel.Length() == 0 {
		return nil, errors.New("__NEXT_DATA__ script not found: not a recognized join.com page")
	}
	var env nextDataEnvelope
	if err := json.Unmarshal([]byte(sel.Text()), &env); err != nil {
		return nil, fmt.Errorf("parse __NEXT_DATA__: %w", err)
	}
	return &env, nil
}

// parseJobDetailHTML extracts the full posting from a job detail page.
// Returns an error if the page has no job in its initial state (a page
// that parses but genuinely carries none, distinct from a 404 the caller
// should already have handled via the HTTP status).
func parseJobDetailHTML(doc *goquery.Document) (*JobDetail, error) {
	env, err := extractNextData(doc)
	if err != nil {
		return nil, err
	}
	j := env.Props.PageProps.InitialState.Job
	if j == nil {
		return nil, errors.New("page has no job in its initial state")
	}
	d := &JobDetail{
		ID:            j.ID,
		IdParam:       j.IdParam,
		Title:         j.Title,
		CompanyID:     j.CompanyID,
		WorkplaceType: j.WorkplaceType,
		RemoteType:    j.RemoteType,
		Status:        j.Status,
		CreatedAt:     parseJoinTime(j.CreatedAt),
		UpdatedAt:     parseJoinTime(j.UpdatedAt),
		Description:   buildDescription(j),
	}
	if j.City != nil {
		d.City = j.City.CityName
		if d.Country == "" {
			d.Country = j.City.CountryName
		}
	}
	if j.Country != nil {
		d.Country = j.Country.Name
	}
	if j.Category != nil {
		d.Category = j.Category.Name
	}
	if j.EmploymentType != nil {
		d.EmploymentType = j.EmploymentType.Name
	}
	return d, nil
}

// buildDescription assembles a job's full body as Markdown. unifiedDescription
// jobs carry the whole body in one field; legacy jobs split it across
// intro/tasks/requirements/benefits/outro, rendered in that order with
// section headings — mirroring the job-ad-app's own render logic (see
// API.md's Key Behaviors). Benefits and outro are omitted entirely when
// empty, matching the app's `!!e.benefits` / `!!e.outro` guards.
func buildDescription(j *nextDataJob) string {
	if j.UnifiedDescription {
		return strings.TrimSpace(j.Description)
	}
	var parts []string
	if j.Intro != "" {
		parts = append(parts, j.Intro)
	}
	if j.Tasks != "" {
		parts = append(parts, "## Tasks\n\n"+j.Tasks)
	}
	if j.Requirements != "" {
		parts = append(parts, "## Skills\n\n"+j.Requirements)
	}
	if j.Benefits != "" {
		parts = append(parts, "## Benefits\n\n"+j.Benefits)
	}
	if j.Outro != "" {
		parts = append(parts, j.Outro)
	}
	return strings.Join(parts, "\n\n")
}

// parseCompanyHTML resolves a company's numeric id and canonical name from
// its /companies/{slug} page.
func parseCompanyHTML(doc *goquery.Document) (*Company, error) {
	env, err := extractNextData(doc)
	if err != nil {
		return nil, err
	}
	c := env.Props.PageProps.InitialState.Company
	if c == nil {
		return nil, errors.New("page has no company in its initial state")
	}
	return &Company{ID: c.ID, Name: c.Name, Slug: c.Domain}, nil
}

// parseJoinTime parses JOIN's RFC 3339 timestamps. An unparsable or empty
// value yields the zero time rather than an error — a malformed date on one
// field shouldn't fail an otherwise-usable job.
func parseJoinTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
