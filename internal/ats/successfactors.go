package ats

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/successfactors"
)

var _ Adapter = (*SuccessFactorsAdapter)(nil)

// successFactorsDateLayout matches java.util.Date#toString, the format
// SuccessFactors emits for itemprop="datePosted" (e.g.
// "Mon Jul 13 00:00:00 UTC 2026").
const successFactorsDateLayout = "Mon Jan 2 15:04:05 MST 2006"

// SuccessFactorsAdapter serves SAP SuccessFactors Career Site Builder
// tenants. Search and detail are server-rendered HTML pages (see
// internal/provider/successfactors/openapi.yaml); slugs are lowercase
// career-site hostnames rather than a shared-host tenant token, since every
// tenant has its own custom domain.
type SuccessFactorsAdapter struct {
	hc      *http.Client
	baseURL func(host string) string
}

func NewSuccessFactorsAdapter(hc *http.Client) *SuccessFactorsAdapter {
	return &SuccessFactorsAdapter{
		hc:      hc,
		baseURL: func(host string) string { return "https://" + host },
	}
}

func (a *SuccessFactorsAdapter) Name() string { return "successfactors" }

func (a *SuccessFactorsAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(successfactors.Companies))
	for _, c := range successfactors.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Host, Name: c.Name})
	}
	return infos
}

// ParseCareersURL only recognizes curated hosts: unlike Teamtailor's shared
// *.teamtailor.com suffix, every SuccessFactors CSB tenant uses its own
// custom domain with no common pattern to match uncurated tenants against.
func (a *SuccessFactorsAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	host := strings.ToLower(u.Hostname())
	if _, ok := successfactors.CompaniesByHost[host]; !ok {
		return "", false
	}
	return host, true
}

func (a *SuccessFactorsAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	c, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}

	filterValues, err := a.resolveFilters(ctx, c, p.Filters)
	if err != nil {
		return nil, err
	}

	page := clampPage(p.Page)
	client := successfactors.NewClient(a.baseURL(c.Host), a.hc)
	req := successfactors.SearchRequest{
		Query:          p.Query,
		LocationSearch: strings.TrimSpace(p.Location),
		Filters:        filterValues,
		StartRow:       (page - 1) * pageSize,
	}
	res, err := client.Search(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("successfactors: search %q: %w", c.Host, err)
	}
	// The upstream table always returns (up to) 25 rows regardless of
	// startRow (see openapi.yaml); trim to the unified pageSize.
	jobs := res.Jobs
	if len(jobs) > pageSize {
		jobs = jobs[:pageSize]
	}
	return &SearchResult{
		Jobs:       successFactorsJobSummaries(jobs, c.Host),
		TotalCount: res.TotalCount,
		Page:       page,
		TotalPages: totalPages(res.TotalCount),
	}, nil
}

func (a *SuccessFactorsAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	c, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	client := successfactors.NewClient(a.baseURL(c.Host), a.hc)
	res, err := client.FacetValues(ctx, &successfactors.SearchRequest{})
	if err != nil {
		return nil, fmt.Errorf("successfactors: facets %q: %w", c.Host, err)
	}

	seen := make(map[string]map[string]struct{}, len(res.Facets))
	for dimension, options := range res.Facets {
		if len(options) == 0 {
			continue
		}
		labels := make(map[string]struct{}, len(options))
		for _, o := range options {
			labels[displayLabel(o)] = struct{}{}
		}
		seen[dimension] = labels
	}
	return toFilterSet(seen), nil
}

func (a *SuccessFactorsAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	c, err := a.resolveSlug(slug)
	if err != nil {
		return nil, err
	}
	client := successfactors.NewClient(a.baseURL(c.Host), a.hc)
	d, err := client.JobDetail(ctx, jobID)
	if errors.Is(err, successfactors.ErrJobNotFound) {
		return nil, fmt.Errorf(
			"successfactors: job %q not found for company %q; pass a job_id exactly as returned by the job search",
			jobID, slug,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("successfactors: fetch job %q for %q: %w", jobID, slug, err)
	}

	desc := d.DescriptionHTML
	if desc != "" {
		if text, err := html2text.FromString(desc, html2text.Options{}); err == nil {
			desc = text
		}
	}

	return &JobDetail{
		JobID:       jobID,
		Title:       d.Title,
		Company:     c.Name,
		Location:    d.Location,
		PostedAt:    successFactorsPostedAt(d.PostedAtRaw),
		URL:         successFactorsJobURL(c.Host, jobID),
		Description: desc,
	}, nil
}

func (a *SuccessFactorsAdapter) resolveSlug(slug string) (successfactors.Company, error) {
	c, ok := successfactors.CompaniesByHost[strings.ToLower(slug)]
	if !ok {
		return successfactors.Company{}, fmt.Errorf("successfactors: unknown company %q; pass a roster career-site host", slug)
	}
	return c, nil
}

// resolveFilters turns unified filter labels (as reported by Filters())
// into the upstream's raw facet values, probing one unfiltered facetValues
// call to learn the tenant's current options — mirrors Workday's
// probeFacets and Eightfold's resolveFilters. The probe is deliberately
// unscoped by query/location; narrower facets aren't needed just to
// resolve labels.
//
// Each optionsFacetsDD_<dimension> query param is a single-select dropdown
// upstream. For several values in one dimension, this deliberately omits
// that dimension rather than fanning out: the resulting complete result set
// is a superset of the requested OR selection, which callers can filter
// locally. Other single-value dimensions still apply.
func (a *SuccessFactorsAdapter) resolveFilters(ctx context.Context, c successfactors.Company, filters FilterSet) (map[string]string, error) {
	if len(filters) == 0 {
		return nil, nil
	}

	hasSingleValue := false
	for _, values := range filters {
		if len(values) == 1 {
			hasSingleValue = true
			break
		}
	}
	if !hasSingleValue {
		return nil, nil
	}

	client := successfactors.NewClient(a.baseURL(c.Host), a.hc)
	probe, err := client.FacetValues(ctx, &successfactors.SearchRequest{})
	if err != nil {
		return nil, fmt.Errorf("successfactors: facets %q: %w", c.Host, err)
	}

	valid := make(map[string]bool, len(probe.Facets))
	for dimension, options := range probe.Facets {
		if len(options) > 0 {
			valid[dimension] = true
		}
	}

	resolved := make(map[string]string, len(filters))
	for key, values := range filters {
		if len(values) > 1 {
			continue
		}
		options, ok := probe.Facets[key]
		if !ok || len(options) == 0 {
			return nil, errUnknownFilterKey(key, valid)
		}
		if len(values) == 0 {
			continue
		}
		v, ok := resolveSuccessFactorsFacetValue(options, values[0])
		if !ok {
			labels := make([]string, len(options))
			for i, o := range options {
				labels[i] = displayLabel(o)
			}
			return nil, fmt.Errorf("filter value %q not found for %q; available: %s", values[0], key, strings.Join(labels, ", "))
		}
		resolved[key] = v
	}
	return resolved, nil
}

func resolveSuccessFactorsFacetValue(options []successfactors.FacetOption, label string) (string, bool) {
	for _, o := range options {
		if strings.EqualFold(displayLabel(o), label) {
			return o.Name, true
		}
	}
	return "", false
}

func displayLabel(o successfactors.FacetOption) string {
	if o.Translated != "" {
		return o.Translated
	}
	return o.Name
}

func successFactorsJobURL(host, id string) string {
	return fmt.Sprintf("https://%s/job/%s/%s/", host, id, id)
}

func successFactorsJobSummaries(jobs []successfactors.Job, host string) []JobSummary {
	out := make([]JobSummary, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, JobSummary{
			JobID:    j.ID,
			Title:    j.Title,
			Location: j.Location,
			URL:      successFactorsJobURL(host, j.ID),
		})
	}
	return out
}

// successFactorsPostedAt renders the unified PostedAt format when the
// upstream's Java Date#toString value parses cleanly; otherwise it passes
// the raw text through (some tenants omit datePosted entirely, in which
// case raw is already "").
func successFactorsPostedAt(raw string) string {
	if raw == "" {
		return ""
	}
	t, err := time.Parse(successFactorsDateLayout, raw)
	if err != nil {
		return raw
	}
	return isoDate(t)
}
