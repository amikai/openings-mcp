package ats

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/teamtailor"
)

var _ Adapter = (*TeamtailorAdapter)(nil)

// teamtailorCareersHostRE matches *.teamtailor.com hosts (including regional
// labels like .na / .eu / .au). Reserved product hosts are rejected after
// the match — Go's RE2 has no negative lookahead. Curated custom domains
// are recognized via the roster.
//
// Examples (hostname):
//   - career.teamtailor.com
//   - acme.na.teamtailor.com
//   - acme.au.teamtailor.com
//
// Rejected:
//   - www.teamtailor.com
//   - api.na.teamtailor.com
var teamtailorCareersHostRE = regexp.MustCompile(`(?i)^[a-z0-9.-]+\.teamtailor\.com$`)

// TeamtailorAdapter serves Teamtailor career sites. The public /jobs.json
// endpoint returns the complete board with full descriptions, so all search,
// filter, and detail behavior is implemented over that dump.
type TeamtailorAdapter struct {
	hc      *http.Client
	baseURL func(host string) string
}

func NewTeamtailorAdapter(hc *http.Client) *TeamtailorAdapter {
	return &TeamtailorAdapter{
		hc:      hc,
		baseURL: func(host string) string { return "https://" + host },
	}
}

func (a *TeamtailorAdapter) Name() string { return "teamtailor" }

func (a *TeamtailorAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(teamtailor.Companies))
	for _, c := range teamtailor.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Host, Name: c.Name})
	}
	return infos
}

var teamtailorReservedHosts = map[string]bool{
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

func isTeamtailorCareerHost(host string) bool {
	if _, ok := teamtailor.CompaniesByHost[host]; ok {
		return true
	}
	if !teamtailorCareersHostRE.MatchString(host) {
		return false
	}
	prefix, _ := strings.CutSuffix(host, ".teamtailor.com")
	first, _, _ := strings.Cut(prefix, ".")
	return !teamtailorReservedHosts[first]
}

// ParseCareersURL recognizes every non-reserved Teamtailor-hosted career site
// and curated custom-domain sites. The lowercase hostname is already the slug
// form accepted by the other adapter methods.
func (a *TeamtailorAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	host := strings.ToLower(u.Hostname())
	if !isTeamtailorCareerHost(host) {
		return "", false
	}
	return host, true
}

func (a *TeamtailorAdapter) Search(
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

func (a *TeamtailorAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *TeamtailorAdapter) Detail(
	ctx context.Context,
	slug string,
	jobID string,
) (*JobDetail, error) {
	feed, err := a.feed(ctx, slug)
	if err != nil {
		return nil, err
	}
	for _, j := range feed.Items {
		if j.ID.String() != jobID {
			continue
		}
		location := teamtailorLocations(j.Jobposting.JobLocation)
		return &JobDetail{
			JobID:       jobID,
			Title:       j.Title,
			Company:     feed.Title,
			Location:    location.display,
			PostedAt:    isoDate(j.DatePublished),
			URL:         j.URL,
			Description: teamtailorDescription(j.ContentHTML),
		}, nil
	}
	return nil, fmt.Errorf(
		"teamtailor: job %q not found for company %q; pass a job_id exactly as returned by the job search",
		jobID,
		slug,
	)
}

func (a *TeamtailorAdapter) feed(
	ctx context.Context,
	slug string,
) (*teamtailor.CareerFeed, error) {
	slug = strings.ToLower(slug)
	if !isTeamtailorCareerHost(slug) {
		return nil, fmt.Errorf(
			"teamtailor: unknown career-site host %q; pass a curated host or a *.teamtailor.com careers URL",
			slug,
		)
	}
	client, err := teamtailor.NewClient(a.baseURL(slug), teamtailor.WithClient(a.hc))
	if err != nil {
		return nil, fmt.Errorf("teamtailor: create client for %q: %w", slug, err)
	}
	res, err := client.GetJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("teamtailor: fetch jobs for %q: %w", slug, err)
	}
	switch r := res.(type) {
	case *teamtailor.CareerFeed:
		return r, nil
	case *teamtailor.GetJobsNotFound:
		return nil, fmt.Errorf("teamtailor: career-site host %q not found upstream", slug)
	default:
		return nil, fmt.Errorf("teamtailor: unexpected response type %T", res)
	}
}

func (a *TeamtailorAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	feed, err := a.feed(ctx, slug)
	if err != nil {
		return nil, err
	}
	jobs := make([]dumpJob, 0, len(feed.Items))
	for _, j := range feed.Items {
		location := teamtailorLocations(j.Jobposting.JobLocation)
		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    j.ID.String(),
				Title:    j.Title,
				Location: location.display,
				PostedAt: isoDate(j.DatePublished),
				URL:      j.URL,
			},
			sortKey:     j.DatePublished,
			description: teamtailorDescription(j.ContentHTML),
			locations:   location.search,
			fields:      location.fields,
		})
	}
	return jobs, nil
}

type teamtailorLocation struct {
	// fields holds deduped facet values keyed by "city", "region", "country"
	// (only keys with at least one value are present), for filter matching.
	fields map[string][]string
	// display is the human-readable location, preferring city, then region,
	// then country, joined with "; " when a posting has multiple places.
	display string
	// search is every address component across all places joined with "; ",
	// for broad free-text location matching.
	search string
}

func teamtailorLocations(places []teamtailor.Place) teamtailorLocation {
	cities := make([]string, 0, len(places))
	regions := make([]string, 0, len(places))
	countries := make([]string, 0, len(places))
	search := make([]string, 0, len(places)*5)
	for _, p := range places {
		address := p.Address
		cities = appendDistinct(cities, address.AddressLocality)
		countries = appendDistinct(countries, address.AddressCountry)
		search = appendDistinct(search, address.AddressLocality)
		search = appendDistinct(search, address.AddressCountry)
		if region, ok := address.AddressRegion.Get(); ok {
			regions = appendDistinct(regions, region)
			search = appendDistinct(search, region)
		}
		if street, ok := address.StreetAddress.Get(); ok {
			search = appendDistinct(search, street)
		}
		if postalCode, ok := address.PostalCode.Get(); ok {
			search = appendDistinct(search, postalCode)
		}
	}

	display := cities
	if len(display) == 0 {
		display = regions
	}
	if len(display) == 0 {
		display = countries
	}
	fields := make(map[string][]string, 3)
	if len(cities) > 0 {
		fields["city"] = cities
	}
	if len(regions) > 0 {
		fields["region"] = regions
	}
	if len(countries) > 0 {
		fields["country"] = countries
	}
	return teamtailorLocation{
		display: strings.Join(display, "; "),
		search:  strings.Join(search, "; "),
		fields:  fields,
	}
}

func appendDistinct(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

func teamtailorDescription(content string) string {
	if content == "" {
		return ""
	}
	text, err := html2text.FromString(content, html2text.Options{})
	if err != nil {
		return content
	}
	return text
}
