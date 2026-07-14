package ats

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/url"
	"slices"
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

// maxFilterCombinations caps the cartesian product of OR'd filter values
// across dimensions (see searchWithFanout) so a broad selection fails
// loudly, asking the caller to narrow it, instead of firing an unbounded
// number of upstream requests.
const maxFilterCombinations = 12

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

	// filterValues holds every resolved value per dimension (preserving
	// FilterSet's OR semantics within a key); combos expands that into the
	// single-value-per-dimension requests the upstream single-select
	// dropdowns can each express.
	filterValues, err := a.resolveFilters(ctx, c, p.Filters)
	if err != nil {
		return nil, err
	}
	combos, err := filterCombinations(filterValues)
	if err != nil {
		return nil, err
	}

	client := successfactors.NewClient(a.baseURL(c.Host), a.hc)
	base := successfactors.SearchRequest{Query: p.Query, LocationSearch: strings.TrimSpace(p.Location)}
	page := clampPage(p.Page)

	if len(combos) <= 1 {
		// Fast path: no OR fan-out needed (zero or one value per
		// dimension), so the unified page maps directly onto one upstream
		// request, same as before dynamic filters existed.
		req := base
		req.Filters = combos[0]
		req.StartRow = (page - 1) * pageSize
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

	jobs, total, err := searchWithFanout(ctx, client, base, combos, page)
	if err != nil {
		return nil, fmt.Errorf("successfactors: search %q: %w", c.Host, err)
	}
	return &SearchResult{
		Jobs:       successFactorsJobSummaries(jobs, c.Host),
		TotalCount: total,
		Page:       page,
		TotalPages: totalPages(total),
	}, nil
}

// filterCombinations expands OR'd filter values into every AND'd
// single-value combination the upstream's single-select dropdowns can
// express in one request each: for dimensions d1={a,b} and d2={x}, that's
// {d1:a,d2:x} and {d1:b,d2:x} — the union of those two requests' results is
// exactly "(d1=a OR d1=b) AND d2=x". Always returns at least one (possibly
// nil-map) combination, so callers don't special-case the no-filter case.
// It validates the product before allocating the result slice, preventing a
// malicious or accidental high-dimensional selection from exhausting memory.
func filterCombinations(filterValues map[string][]string) ([]map[string]string, error) {
	if len(filterValues) == 0 {
		return []map[string]string{nil}, nil
	}
	dims := slices.Sorted(maps.Keys(filterValues))
	valuesByDimension := make(map[string][]string, len(dims))
	combinationCount := 1
	for _, dim := range dims {
		remainingCapacity := maxFilterCombinations / combinationCount
		initialCapacity := min(len(filterValues[dim]), remainingCapacity+1)
		seen := make(map[string]struct{}, initialCapacity)
		values := make([]string, 0, initialCapacity)
		for _, value := range filterValues[dim] {
			if _, ok := seen[value]; ok {
				continue
			}
			if len(values) == remainingCapacity {
				return nil, fmt.Errorf(
					"filter selection expands to more than %d combinations; narrow the OR'd values",
					maxFilterCombinations,
				)
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
		if len(values) == 0 {
			continue
		}
		combinationCount *= len(values)
		valuesByDimension[dim] = values
	}

	combos := make([]map[string]string, 1, combinationCount)
	combos[0] = map[string]string{}
	for _, dim := range dims {
		values := valuesByDimension[dim]
		if len(values) == 0 {
			continue
		}
		next := make([]map[string]string, 0, len(combos)*len(values))
		for _, combo := range combos {
			for _, v := range values {
				c := make(map[string]string, len(combo)+1)
				maps.Copy(c, combo)
				c[dim] = v
				next = append(next, c)
			}
		}
		combos = next
	}
	return combos, nil
}

// searchWithFanout treats the disjoint filter combinations as one logical,
// combination-major result stream. It probes each combination once to get its
// count, but fetches only the slice intersecting the requested unified page.
// This keeps work bounded even when one valid filter has thousands of jobs.
func searchWithFanout(
	ctx context.Context,
	client *successfactors.Client,
	base successfactors.SearchRequest,
	combos []map[string]string,
	page int,
) ([]successfactors.Job, int, error) {
	pageStart, pageEnd := successFactorsPageBounds(page)
	jobs := make([]successfactors.Job, 0, pageSize)
	total := 0
	for _, combo := range combos {
		req := base
		req.Filters = combo
		res, err := client.Search(ctx, &req)
		if err != nil {
			return nil, 0, err
		}
		if res.TotalCount < 0 || res.TotalCount > math.MaxInt-total {
			return nil, 0, fmt.Errorf("invalid total count %d for filter combination %v", res.TotalCount, combo)
		}

		comboStart := total
		comboEnd := comboStart + res.TotalCount
		total = comboEnd
		if pageStart >= comboEnd || pageEnd <= comboStart {
			continue
		}

		localStart := max(0, pageStart-comboStart)
		localEnd := min(res.TotalCount, pageEnd-comboStart)
		need := localEnd - localStart
		pageJobs := res.Jobs
		if localStart+need > len(pageJobs) {
			req.StartRow = localStart
			res, err = client.Search(ctx, &req)
			if err != nil {
				return nil, 0, err
			}
			pageJobs = res.Jobs
			localStart = 0
		}
		if localStart+need > len(pageJobs) {
			return nil, 0, fmt.Errorf(
				"filter combination %v returned %d jobs for a requested slice of %d",
				combo, len(pageJobs)-localStart, need,
			)
		}
		jobs = append(jobs, pageJobs[localStart:localStart+need]...)
	}
	return jobs, total, nil
}

func successFactorsPageBounds(page int) (int, int) {
	pageIndex := page - 1
	if pageIndex > (math.MaxInt-pageSize)/pageSize {
		return math.MaxInt - pageSize, math.MaxInt
	}
	start := pageIndex * pageSize
	return start, start + pageSize
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
// into the upstream's raw facet values, preserving OR semantics (every
// resolved value per key, not just the first), probing one unfiltered
// facetValues call to learn the tenant's current options — mirrors
// Workday's probeFacets and Eightfold's resolveFilters. The probe is
// deliberately unscoped by query/location; narrower facets aren't needed
// just to resolve labels. Search expands the result via filterCombinations
// into the single-value-per-dimension requests the upstream's single-select
// dropdowns can each express.
func (a *SuccessFactorsAdapter) resolveFilters(ctx context.Context, c successfactors.Company, filters FilterSet) (map[string][]string, error) {
	if len(filters) == 0 {
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

	resolved := make(map[string][]string, len(filters))
	for key, values := range filters {
		options, ok := probe.Facets[key]
		if !ok || len(options) == 0 {
			return nil, errUnknownFilterKey(key, valid)
		}
		if len(values) == 0 {
			continue
		}
		resolvedValues := make([]string, 0, len(values))
		for _, label := range values {
			v, ok := resolveSuccessFactorsFacetValue(options, label)
			if !ok {
				labels := make([]string, len(options))
				for i, o := range options {
					labels[i] = displayLabel(o)
				}
				return nil, fmt.Errorf("filter value %q not found for %q; available: %s", label, key, strings.Join(labels, ", "))
			}
			resolvedValues = append(resolvedValues, v)
		}
		resolved[key] = resolvedValues
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
