package ats

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/amikai/openings-mcp/internal/provider/join"
)

// JoinAdapter serves join.com-hosted companies. The public GraphQL search
// endpoint has no server-side keyword search (see the provider's API.md),
// so Search fetches a company's whole job dump and filters it via
// searchDump, the same shape as Greenhouse. Unlike Greenhouse, the dump
// carries no description — join.com's search endpoint never populates
// one — so Detail is a separate SSR page scrape, and searchDump's
// full-text tier only ever matches title and category.
type JoinAdapter struct {
	client *join.Client
}

// joinCareersURLRE matches join.com company page URLs and captures the
// slug (first path segment after /companies/).
//
// Example (hostname + escaped path): join.com/companies/routinelabs
var joinCareersURLRE = regexp.MustCompile(`(?i)^join\.com/companies/(?P<slug>[^/]+)`)

func NewJoinAdapter(baseURL string, hc *http.Client) *JoinAdapter {
	return &JoinAdapter{client: join.NewClient(baseURL, hc)}
}

func (a *JoinAdapter) Name() string { return "join" }

func (a *JoinAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(join.Companies))
	for _, c := range join.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Slug, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes join.com company page URLs. There is no
// public way to resolve an arbitrary slug's numeric companyId without a
// network call (see the provider's API.md), and this method has no
// context to make one with, so it only matches slugs already in the
// curated roster.
func (a *JoinAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	slug, ok := matchCareersSlug(joinCareersURLRE, u)
	if !ok {
		return "", false
	}
	c, ok := join.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return "", false
	}
	return c.Slug, true
}

func (a *JoinAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	jobs, _, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *JoinAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, _, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *JoinAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	c, ok := join.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return nil, errJoinCompanyNotFound(slug)
	}
	d, err := a.client.JobDetail(ctx, c.Slug, jobID)
	if err != nil {
		if errors.Is(err, join.ErrNotFound) {
			return nil, errJoinJobNotFound(slug, jobID)
		}
		return nil, fmt.Errorf("join: fetch job %q for %q: %w", jobID, slug, err)
	}
	return &JobDetail{
		JobID:       d.IdParam,
		Title:       d.Title,
		Company:     c.Name,
		Location:    joinLocation(d.City, d.Country, d.WorkplaceType, d.RemoteType),
		PostedAt:    joinPostedAt(d),
		URL:         c.CareersURL() + "/" + d.IdParam,
		Description: d.Description,
	}, nil
}

func errJoinCompanyNotFound(slug string) error {
	return fmt.Errorf("join: company %q not found in the curated roster", slug)
}

func errJoinJobNotFound(slug, jobID string) error {
	return fmt.Errorf("join: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
}

// dump fetches a company's whole board and reshapes it for the filter
// engine. Requires slug to be a curated roster entry — join.com's
// companyId can't be resolved from an arbitrary slug without a network
// call (see ParseCareersURL), so this adapter never serves non-roster
// companies, unlike Workday.
func (a *JoinAdapter) dump(ctx context.Context, slug string) ([]dumpJob, join.RosterCompany, error) {
	c, ok := join.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return nil, join.RosterCompany{}, errJoinCompanyNotFound(slug)
	}
	jobs, err := a.client.Jobs(ctx, c.CompanyID)
	if err != nil {
		return nil, c, fmt.Errorf("join: list jobs for %q: %w", slug, err)
	}
	out := make([]dumpJob, 0, len(jobs))
	for _, j := range jobs {
		fields := map[string][]string{}
		if j.Category != "" {
			fields["category"] = []string{j.Category}
		}
		if j.EmploymentType != "" {
			fields["employment_type"] = []string{j.EmploymentType}
		}
		loc := joinLocation(j.City, j.Country, j.WorkplaceType, j.RemoteType)
		out = append(out, dumpJob{
			summary: JobSummary{
				JobID:    j.IdParam,
				Title:    j.Title,
				Location: loc,
				PostedAt: joinJobPostedAt(j),
				URL:      c.CareersURL() + "/" + j.IdParam,
			},
			sortKey: j.CreatedAt,
			orgUnit: j.Category,
			// description stays empty: join.com's search endpoint never
			// populates one (see the provider's API.md), so the
			// full-text search tier only ever matches title/orgUnit.
			locations: loc,
			fields:    fields,
			isRemote:  j.WorkplaceType == "REMOTE",
		})
	}
	return out, c, nil
}

// joinLocation renders a job's location for display and fuzzy search.
// Collapsing to a bare city would mislead for a REMOTE job — city/country
// still carry the employer's base location even when remoteType is
// ANYWHERE (no location restriction at all), and a country-scoped remote
// role (remoteType COUNTRY) would otherwise be invisible to a country-name
// search since its city alone doesn't mention the country restriction (see
// API.md's remoteType note). Non-REMOTE jobs get a plain "City, Country".
func joinLocation(city, country, workplaceType, remoteType string) string {
	base := cityCountry(city, country)
	if workplaceType != "REMOTE" {
		return base
	}
	switch remoteType {
	case "ANYWHERE":
		if base != "" {
			return "Remote (Anywhere) · " + base
		}
		return "Remote (Anywhere)"
	case "COUNTRY":
		if country != "" {
			return "Remote (" + country + ")"
		}
		return "Remote"
	default:
		if base != "" {
			return "Remote · " + base
		}
		return "Remote"
	}
}

func cityCountry(city, country string) string {
	switch {
	case city != "" && country != "":
		return city + ", " + country
	case city != "":
		return city
	default:
		return country
	}
}

func joinJobPostedAt(j join.Job) string {
	if j.CreatedAt.IsZero() {
		return ""
	}
	return isoDate(j.CreatedAt)
}

func joinPostedAt(d *join.JobDetail) string {
	if d.CreatedAt.IsZero() {
		return ""
	}
	return isoDate(d.CreatedAt)
}
