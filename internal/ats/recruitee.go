package ats

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/recruitee"
)

var _ Adapter = (*RecruiteeAdapter)(nil)

// recruiteeCareersHostRE matches Recruitee career-site hosts and captures
// the company slug (subdomain). Reserved product hosts are rejected after
// the match.
//
// Examples (hostname):
//   - bunq.recruitee.com
//   - acme.recruitee.com
var _recruiteeCareersHostRE = regexp.MustCompile(
	`(?i)^(?P<slug>.+)\.recruitee\.com$`,
)

// RecruiteeAdapter serves Recruitee career sites. The public /api/offers
// endpoint returns the complete board with full descriptions, so all search,
// filter, and detail behavior is implemented over that dump.
type RecruiteeAdapter struct {
	hc        *http.Client
	baseURL   func(slug string) string
	dumpCache *DumpCache
}

// recruiteeBaseURLTpl formats a tenant subdomain into a base URL (e.g. "https://mous.recruitee.com").
const _recruiteeBaseURLTpl = "https://%s.recruitee.com"

func NewRecruiteeAdapter(hc *http.Client, dumpCache *DumpCache) *RecruiteeAdapter {
	return &RecruiteeAdapter{
		hc: hc,
		baseURL: func(slug string) string {
			return fmt.Sprintf(_recruiteeBaseURLTpl, slug)
		},
		dumpCache: dumpCache,
	}
}

func (a *RecruiteeAdapter) Name() string { return "recruitee" }

func (a *RecruiteeAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(recruitee.Companies))
	for _, c := range recruitee.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Slug, Name: c.Name})
	}
	return infos
}

var _recruiteeReservedHosts = map[string]bool{
	"api":          true,
	"app":          true,
	"assets":       true,
	"docs":         true,
	"images":       true,
	"integrations": true,
	"partner":      true,
	"support":      true,
	"trust":        true,
	"www":          true,
}

// ParseCareersURL recognizes Recruitee subdomain career pages.
func (a *RecruiteeAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	m := _recruiteeCareersHostRE.FindStringSubmatch(strings.ToLower(u.Hostname()))
	if m == nil {
		return "", false
	}
	slug := namedGroup(_recruiteeCareersHostRE, m, "slug")
	if slug == "" || _recruiteeReservedHosts[slug] {
		return "", false
	}
	return slug, true
}

// CareersURL renders the roster company's public jobs page.
func (a *RecruiteeAdapter) CareersURL(slug string) (string, bool) {
	c, ok := recruitee.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return "", false
	}
	return c.CareersURL(), true
}

func (a *RecruiteeAdapter) Search(
	ctx context.Context,
	slug string,
	p SearchParams,
) (*SearchResult, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *RecruiteeAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *RecruiteeAdapter) Detail(
	ctx context.Context,
	slug string,
	jobID string,
) (*JobDetail, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	for _, j := range jobs {
		if j.summary.JobID != jobID {
			continue
		}
		return &JobDetail{
			JobID:       jobID,
			Title:       j.summary.Title,
			Company:     cmp.Or(recruitee.CompaniesBySlug[slug].Name, slug),
			Location:    j.summary.Location,
			PostedAt:    j.summary.PostedAt,
			URL:         j.summary.URL,
			Description: j.description,
		}, nil
	}
	return nil, fmt.Errorf(
		"recruitee: job %q not found for company %q; pass a job_id exactly as returned by the job search",
		jobID,
		slug,
	)
}

// dump returns a full-board intermediate dump, reusing the process-local
// dump cache when enabled.
func (a *RecruiteeAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	slug = strings.ToLower(slug)
	jobs, _, err := a.dumpCache.getOrLoadDump(ctx, a.Name(), slug, func(ctx context.Context) ([]dumpJob, any, error) {
		jobs, err := a.fetchDump(ctx, slug)
		return jobs, nil, err
	})
	return jobs, err
}

// fetchDump loads offers for the subdomain and reshapes them for filtering.
func (a *RecruiteeAdapter) fetchDump(ctx context.Context, slug string) ([]dumpJob, error) {
	client, err := recruitee.NewClient(a.baseURL(slug), recruitee.WithClient(a.hc))
	if err != nil {
		return nil, fmt.Errorf("recruitee: create client for %q: %w", slug, err)
	}
	res, err := client.GetOffers(ctx)
	if err != nil {
		return nil, fmt.Errorf("recruitee: fetch offers for %q: %w", slug, err)
	}

	var offers []recruitee.Offer
	switch r := res.(type) {
	case *recruitee.OffersResponse:
		offers = r.Offers
	case *recruitee.GetOffersNotFound:
		return nil, fmt.Errorf("recruitee: career-site subdomain %q not found upstream", slug)
	default:
		return nil, fmt.Errorf("recruitee: unexpected response type %T", res)
	}

	jobs := make([]dumpJob, 0, len(offers))
	for _, o := range offers {
		title := o.Title.Or("")
		jobURL := o.CareersURL.Or("")
		if jobURL == "" && !o.Slug.Null && o.Slug.Value != "" {
			jobURL = fmt.Sprintf("https://%s.recruitee.com/o/%s", slug, o.Slug.Value)
		}

		postedTime, postedDateStr := recruiteeParseDate(o.PublishedAt.Or(o.CreatedAt.Or("")))
		location := recruiteeLocations(o)

		// Structured fields
		fields := location.fields
		if o.Department.Set && o.Department.Value != "" {
			fields["department"] = []string{o.Department.Value}
		}
		if o.EmploymentTypeCode.Set && o.EmploymentTypeCode.Value != "" {
			fields["employmentType"] = []string{o.EmploymentTypeCode.Value}
		}
		if o.ExperienceCode.Set && o.ExperienceCode.Value != "" {
			fields["experience"] = []string{o.ExperienceCode.Value}
		}

		// Description plain text
		descHTML := o.Description.Or("")
		reqHTML := o.Requirements.Or("")
		fullHTML := descHTML
		if reqHTML != "" {
			fullHTML = fullHTML + "\n\n<h3>Requirements</h3>\n" + reqHTML
		}
		descriptionText := recruiteeDescription(fullHTML)

		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    strconv.Itoa(o.ID),
				Title:    title,
				Location: location.display,
				PostedAt: postedDateStr,
				URL:      jobURL,
			},
			sortKey:     postedTime,
			orgUnit:     o.Department.Or(""),
			description: descriptionText,
			locations:   location.search,
			fields:      fields,
			isRemote:    location.isRemote,
		})
	}

	return jobs, nil
}

type recruiteeLocation struct {
	fields   map[string][]string
	display  string
	search   string
	isRemote bool
}

func recruiteeLocations(o recruitee.Offer) recruiteeLocation {
	cities := make([]string, 0, len(o.Locations)+1)
	countries := make([]string, 0, len(o.Locations)+1)
	displayParts := make([]string, 0, len(o.Locations))
	for _, loc := range o.Locations {
		city := loc.City.Or("")
		country := loc.Country.Or("")
		cities = appendDistinct(cities, city)
		countries = appendDistinct(countries, country)
		displayParts = appendDistinct(
			displayParts,
			recruiteeLocationPart(city, country, loc.Name.Or("")),
		)
	}

	topCity := o.City.Or("")
	topCountry := o.Country.Or("")
	cities = appendDistinct(cities, topCity)
	countries = appendDistinct(countries, topCountry)
	if len(displayParts) == 0 {
		displayParts = appendDistinct(displayParts, o.Location.Or(""))
	}
	if len(displayParts) == 0 {
		displayParts = appendDistinct(
			displayParts,
			recruiteeLocationPart(topCity, topCountry, ""),
		)
	}

	display := strings.Join(displayParts, "; ")
	isRemote := o.Remote.Or(false) || strings.Contains(strings.ToLower(display), "remote")
	if display == "" && isRemote {
		display = "Remote"
	}

	fields := make(map[string][]string, 2)
	if len(cities) > 0 {
		fields["city"] = cities
	}
	if len(countries) > 0 {
		fields["country"] = countries
	}
	return recruiteeLocation{
		fields:   fields,
		display:  display,
		search:   display,
		isRemote: isRemote,
	}
}

func recruiteeLocationPart(city, country, name string) string {
	switch {
	case city != "" && country != "":
		return city + ", " + country
	case city != "":
		return city
	case country != "":
		return country
	default:
		return name
	}
}

func recruiteeParseDate(s string) (time.Time, string) {
	if s == "" {
		return time.Time{}, ""
	}
	// Example format: "2026-07-13 13:42:26 UTC"
	t, err := time.Parse("2006-01-02 15:04:05 MST", s)
	if err != nil {
		// Try fallback ISO 8601
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, ""
		}
	}
	return t, t.UTC().Format("2006-01-02")
}

func recruiteeDescription(content string) string {
	if content == "" {
		return ""
	}
	text, err := html2text.FromString(content, html2text.Options{})
	if err != nil {
		return content
	}
	return text
}
