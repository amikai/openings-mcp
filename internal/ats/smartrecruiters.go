package ats

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

var _ Adapter = (*SmartRecruitersAdapter)(nil)

const (
	smartRecruitersCandidatePageSize = 100
	maxSmartRecruitersCandidates     = 2000
)

// SmartRecruitersAdapter serves SmartRecruiters-hosted companies via the
// public Posting API. Structured filters run server-side. Text search uses
// q only to select a bounded candidate set, then applies the unified Query
// and Location semantics locally because SmartRecruiters ORs q terms and
// searches both titles and locations.
type SmartRecruitersAdapter struct {
	client *smartrecruiters.Client
}

func NewSmartRecruitersAdapter(baseURL string, hc *http.Client) (*SmartRecruitersAdapter, error) {
	c, err := smartrecruiters.NewClient(baseURL, smartrecruiters.WithClient(hc))
	if err != nil {
		return nil, err
	}
	return &SmartRecruitersAdapter{client: c}, nil
}

func (a *SmartRecruitersAdapter) Name() string { return "smartrecruiters" }

func (a *SmartRecruitersAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(smartrecruiters.Companies))
	for _, c := range smartrecruiters.Companies {
		infos = append(infos, CompanyInfo{Slug: strings.ToLower(c.CompanyIdentifier), Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes jobs.smartrecruiters.com career-site URLs; the
// first path segment is the companyIdentifier, which alone addresses a
// company (the API accepts it case-insensitively), so non-roster companies
// need no special slug form. An unknown identifier cannot be validated —
// the list endpoint answers HTTP 200 with zero results — so a typo'd URL
// degrades to an empty search, mirroring the raw API.
func (a *SmartRecruitersAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if strings.ToLower(u.Hostname()) != "jobs.smartrecruiters.com" {
		return "", false
	}
	id := firstPathSegment(u)
	if id == "" {
		return "", false
	}
	return strings.ToLower(id), true
}

// resolveSmartRecruitersCompany maps a slug to the roster's
// canonically-cased identifier (used in derived public URLs) and display
// name. Non-roster slugs from ParseCareersURL pass through as both.
func resolveSmartRecruitersCompany(slug string) (identifier, name string) {
	if c, ok := smartrecruiters.CompaniesByIdentifier[slug]; ok {
		return c.CompanyIdentifier, c.Name
	}
	return slug, slug
}

func (a *SmartRecruitersAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	params := smartrecruiters.ListPostingsParams{CompanyIdentifier: slug}
	if err := a.applyFilters(ctx, slug, p.Filters, &params); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(p.Query)
	location := strings.TrimSpace(p.Location)
	if query == "" && location == "" {
		return a.searchSmartRecruitersPage(ctx, slug, p.Page, params)
	}
	return a.searchSmartRecruitersCandidates(ctx, slug, query, location, p.Page, params)
}

// searchSmartRecruitersPage is the cheap path when no local text matching is
// needed: the upstream total and pagination remain exact.
func (a *SmartRecruitersAdapter) searchSmartRecruitersPage(
	ctx context.Context,
	slug string,
	requestedPage int,
	params smartrecruiters.ListPostingsParams,
) (*SearchResult, error) {
	page := clampPage(requestedPage)
	pageIndex := page - 1
	if pageIndex > math.MaxInt/PageSize {
		return nil, fmt.Errorf("smartrecruiters: page %d is too large; retry with a smaller page", page)
	}
	params.Limit = smartrecruiters.NewOptInt(PageSize)
	params.Offset = smartrecruiters.NewOptInt(pageIndex * PageSize)
	rsp, err := a.client.ListPostings(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("smartrecruiters: search %q: %w", slug, err)
	}
	identifier, _ := resolveSmartRecruitersCompany(slug)
	jobs := smartRecruitersSummaries(identifier, rsp.Content)
	return &SearchResult{
		Jobs:       jobs,
		TotalCount: rsp.TotalFound,
		Page:       page,
		TotalPages: totalPages(rsp.TotalFound),
	}, nil
}

// searchSmartRecruitersCandidates obtains separate Query and Location
// candidate pages, keeps the smaller set, and applies both constraints
// locally with searchDump. This preserves AND semantics without dumping
// boards that can contain tens of thousands of postings.
func (a *SmartRecruitersAdapter) searchSmartRecruitersCandidates(
	ctx context.Context,
	slug, query, location string,
	page int,
	base smartrecruiters.ListPostingsParams,
) (*SearchResult, error) {
	seed, first, err := a.chooseSmartRecruitersCandidates(ctx, slug, query, location, base)
	if err != nil {
		return nil, err
	}
	if first.TotalFound > maxSmartRecruitersCandidates {
		return nil, fmt.Errorf("smartrecruiters: search is too broad (%d candidates); add a more specific query, location, department, or location_type filter", first.TotalFound)
	}
	items, err := a.collectSmartRecruitersCandidates(ctx, slug, seed, base, first)
	if err != nil {
		return nil, err
	}
	identifier, _ := resolveSmartRecruitersCompany(slug)
	jobs := make([]dumpJob, 0, len(items))
	for _, it := range items {
		if job, ok := smartRecruitersDumpJob(identifier, it); ok {
			jobs = append(jobs, job)
		}
	}
	return searchDump(jobs, SearchParams{Query: query, Location: location, Page: page})
}

func (a *SmartRecruitersAdapter) chooseSmartRecruitersCandidates(
	ctx context.Context,
	slug, query, location string,
	base smartrecruiters.ListPostingsParams,
) (string, *smartrecruiters.PostingList, error) {
	seed := query
	if seed == "" {
		seed = location
	}
	first, err := a.listSmartRecruitersCandidates(ctx, slug, seed, 0, smartRecruitersCandidatePageSize, base)
	if err != nil {
		return "", nil, err
	}
	hasDistinctLocation := location != "" && !strings.EqualFold(query, location)
	if query == "" || first.TotalFound == 0 || !hasDistinctLocation {
		return seed, first, nil
	}
	locationFirst, err := a.listSmartRecruitersCandidates(ctx, slug, location, 0, smartRecruitersCandidatePageSize, base)
	if err != nil {
		return "", nil, err
	}
	if locationFirst.TotalFound < first.TotalFound {
		return location, locationFirst, nil
	}
	return seed, first, nil
}

func (a *SmartRecruitersAdapter) listSmartRecruitersCandidates(
	ctx context.Context,
	slug, query string,
	offset, limit int,
	params smartrecruiters.ListPostingsParams,
) (*smartrecruiters.PostingList, error) {
	params.Q = smartrecruiters.NewOptString(query)
	params.Offset = smartrecruiters.NewOptInt(offset)
	params.Limit = smartrecruiters.NewOptInt(limit)
	rsp, err := a.client.ListPostings(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("smartrecruiters: search %q candidates for %q: %w", slug, query, err)
	}
	return rsp, nil
}

func (a *SmartRecruitersAdapter) collectSmartRecruitersCandidates(
	ctx context.Context,
	slug, query string,
	base smartrecruiters.ListPostingsParams,
	first *smartrecruiters.PostingList,
) ([]smartrecruiters.PostingItem, error) {
	items := slices.Clone(first.Content)
	for len(items) < first.TotalFound {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("smartrecruiters: collect candidates for %q: %w", slug, err)
		}
		offset := len(items)
		limit := min(smartRecruitersCandidatePageSize, first.TotalFound-offset)
		rsp, err := a.listSmartRecruitersCandidates(ctx, slug, query, offset, limit, base)
		if err != nil {
			return nil, err
		}
		if len(rsp.Content) == 0 {
			return nil, fmt.Errorf("smartrecruiters: candidate pagination for %q stopped at %d of %d postings", slug, offset, first.TotalFound)
		}
		items = append(items, rsp.Content...)
	}
	return items, nil
}

func smartRecruitersSummaries(identifier string, items []smartrecruiters.PostingItem) []JobSummary {
	jobs := make([]JobSummary, 0, len(items))
	for _, item := range items {
		job, ok := smartRecruitersSummary(identifier, item)
		if ok {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

func smartRecruitersSummary(identifier string, item smartrecruiters.PostingItem) (JobSummary, bool) {
	id := item.ID.Value
	if id == "" {
		return JobSummary{}, false
	}
	return JobSummary{
		JobID:    id,
		Title:    item.Name.Value,
		Location: item.Location.Value.FullLocation.Value,
		PostedAt: smartRecruitersPostedAt(item.ReleasedDate),
		URL:      smartRecruitersPostingURL(identifier, id),
	}, true
}

func smartRecruitersDumpJob(identifier string, item smartrecruiters.PostingItem) (dumpJob, bool) {
	summary, ok := smartRecruitersSummary(identifier, item)
	if !ok {
		return dumpJob{}, false
	}
	released, _ := item.ReleasedDate.Get()
	return dumpJob{
		summary:   summary,
		sortKey:   released,
		locations: summary.Location,
		isRemote:  item.Location.Value.Remote.Or(false),
	}, true
}

// smartRecruitersLocationTypes maps the location_type filter's display
// values to the API's locationType enum.
var smartRecruitersLocationTypes = map[string]smartrecruiters.ListPostingsLocationTypeItem{
	"remote": smartrecruiters.ListPostingsLocationTypeItemREMOTE,
	"hybrid": smartrecruiters.ListPostingsLocationTypeItemHYBRID,
	"onsite": smartrecruiters.ListPostingsLocationTypeItemONSITE,
}

// applyFilters maps unified filters onto the list endpoint's query params,
// failing with teaching errors that name the valid alternatives.
func (a *SmartRecruitersAdapter) applyFilters(ctx context.Context, slug string, filters FilterSet, params *smartrecruiters.ListPostingsParams) error {
	for key, values := range filters {
		switch key {
		case "department":
			ids, err := a.resolveDepartments(ctx, slug, values)
			if err != nil {
				return err
			}
			// Comma-joined ids OR together (verified live against Equinox:
			// 129 + 23 postings filter to 152).
			params.Department = smartrecruiters.NewOptString(strings.Join(ids, ","))
		case "location_type":
			for _, v := range values {
				lt, ok := smartRecruitersLocationTypes[strings.ToLower(strings.TrimSpace(v))]
				if !ok {
					return fmt.Errorf("filter value %q not found for %q; available: Hybrid, Onsite, Remote", v, key)
				}
				params.LocationType = append(params.LocationType, lt)
			}
		default:
			return errUnknownFilterKey(key, map[string]bool{"department": true, "location_type": true})
		}
	}
	return nil
}

// resolveDepartments maps department display labels to ids via one
// departments call, matching labels case-insensitively.
func (a *SmartRecruitersAdapter) resolveDepartments(ctx context.Context, slug string, values []string) ([]string, error) {
	catalog, err := a.departmentCatalog(ctx, slug)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		resolved, ok := catalog.idsByLabel[smartRecruitersDepartmentKey(v)]
		if !ok {
			const maxListed = 20
			listed := catalog.labels
			suffix := ""
			if len(listed) > maxListed {
				listed = listed[:maxListed]
				suffix = ", …"
			}
			return nil, fmt.Errorf("filter value %q not found for %q; available: %s%s", v, "department", strings.Join(listed, ", "), suffix)
		}
		for _, id := range resolved {
			if seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// smartRecruitersPostedAt guards a present-but-missing releasedDate:
// OptDateTime's zero Value would otherwise format as a fake date.
func smartRecruitersPostedAt(t smartrecruiters.OptDateTime) string {
	v, ok := t.Get()
	if !ok {
		return ""
	}
	return isoDate(v)
}

// smartRecruitersPostingURL derives the public posting page. List items
// carry no postingUrl; slug-less URLs (no title suffix) resolve fine on
// jobs.smartrecruiters.com.
func smartRecruitersPostingURL(identifier, id string) string {
	return "https://jobs.smartrecruiters.com/" + url.PathEscape(identifier) + "/" + url.PathEscape(id)
}

// smartRecruitersDepartment is one non-archived, labeled department: the
// id the API's department query param takes and the display label
// Filters() reports.
type smartRecruitersDepartment struct {
	id    string
	label string
}

type smartRecruitersDepartmentCatalog struct {
	idsByLabel map[string][]string
	labels     []string
}

func smartRecruitersDepartmentKey(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

func newSmartRecruitersDepartmentCatalog(deps []smartRecruitersDepartment) smartRecruitersDepartmentCatalog {
	catalog := smartRecruitersDepartmentCatalog{
		idsByLabel: make(map[string][]string, len(deps)),
	}
	displayByKey := make(map[string]string, len(deps))
	for _, dep := range deps {
		label := strings.TrimSpace(dep.label)
		key := smartRecruitersDepartmentKey(label)
		if key == "" || dep.id == "" {
			continue
		}
		if _, ok := displayByKey[key]; !ok {
			displayByKey[key] = label
		}
		catalog.idsByLabel[key] = append(catalog.idsByLabel[key], dep.id)
	}
	catalog.labels = make([]string, 0, len(displayByKey))
	for _, label := range displayByKey {
		catalog.labels = append(catalog.labels, label)
	}
	slices.Sort(catalog.labels)
	return catalog
}

func (a *SmartRecruitersAdapter) departmentCatalog(ctx context.Context, slug string) (smartRecruitersDepartmentCatalog, error) {
	deps, err := a.departments(ctx, slug)
	if err != nil {
		return smartRecruitersDepartmentCatalog{}, err
	}
	return newSmartRecruitersDepartmentCatalog(deps), nil
}

// departments fetches the company's departments, dropping archived and
// unlabeled entries. DepartmentId is a string-or-int sum (the API returns
// both); ids normalize to their decimal string form either way.
func (a *SmartRecruitersAdapter) departments(ctx context.Context, slug string) ([]smartRecruitersDepartment, error) {
	rsp, err := a.client.ListDepartments(ctx, smartrecruiters.ListDepartmentsParams{CompanyIdentifier: slug})
	if err != nil {
		return nil, fmt.Errorf("smartrecruiters: list departments for %q: %w", slug, err)
	}
	deps := make([]smartRecruitersDepartment, 0, len(rsp.Content))
	for _, d := range rsp.Content {
		label := strings.TrimSpace(d.Label.Value)
		if d.Archived.Or(false) || label == "" {
			continue
		}
		id, ok := smartRecruitersDepartmentID(d.ID)
		if !ok {
			continue
		}
		deps = append(deps, smartRecruitersDepartment{id: id, label: label})
	}
	return deps, nil
}

func smartRecruitersDepartmentID(opt smartrecruiters.OptDepartmentId) (string, bool) {
	v, ok := opt.Get()
	if !ok {
		return "", false
	}
	if s, ok := v.GetString(); ok {
		s = strings.TrimSpace(s)
		return s, s != ""
	}
	if n, ok := v.GetInt(); ok {
		return strconv.Itoa(n), true
	}
	return "", false
}

func (a *SmartRecruitersAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	catalog, err := a.departmentCatalog(ctx, slug)
	if err != nil {
		return nil, err
	}
	// location_type is a static API enum, not tenant data.
	fs := FilterSet{"location_type": []string{"Hybrid", "Onsite", "Remote"}}
	if len(catalog.labels) > 0 {
		fs["department"] = slices.Clone(catalog.labels)
	}
	return fs, nil
}

func (a *SmartRecruitersAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	res, err := a.client.GetPosting(ctx, smartrecruiters.GetPostingParams{
		CompanyIdentifier: slug,
		PostingId:         jobID,
	})
	if err != nil {
		return nil, fmt.Errorf("smartrecruiters: fetch job %q for %q: %w", jobID, slug, err)
	}
	d, ok := res.(*smartrecruiters.Posting)
	if !ok {
		// The only other GetPostingRes variant is the 404
		// PostingErrorResponse, for an unknown company or posting id.
		return nil, fmt.Errorf("smartrecruiters: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
	}
	_, name := resolveSmartRecruitersCompany(slug)
	return &JobDetail{
		JobID:       cmp.Or(d.ID.Value, jobID),
		Title:       d.Name.Value,
		Company:     cmp.Or(d.Company.Value.Name.Value, name),
		Location:    d.Location.Value.FullLocation.Value,
		PostedAt:    smartRecruitersPostedAt(d.ReleasedDate),
		URL:         d.PostingUrl.Value,
		Description: smartRecruitersDescription(d.JobAd),
	}, nil
}

// smartRecruitersDescription joins the jobAd's non-empty HTML sections as
// titled plain-text blocks, in the API's canonical section order.
func smartRecruitersDescription(jobAd smartrecruiters.OptJobAdSections) string {
	sections, ok := jobAd.Value.Sections.Get()
	if !ok {
		return ""
	}
	ordered := []struct {
		fallbackTitle string
		sec           smartrecruiters.OptJobAdSection
	}{
		{"Company Description", sections.CompanyDescription},
		{"Job Description", sections.JobDescription},
		{"Qualifications", sections.Qualifications},
		{"Additional Information", sections.AdditionalInformation},
	}
	var parts []string
	for _, s := range ordered {
		sec, ok := s.sec.Get()
		if !ok || sec.Text.Value == "" {
			continue
		}
		text, err := html2text.FromString(sec.Text.Value, html2text.Options{})
		if err != nil {
			// Keep the section as raw HTML rather than dropping it
			// (mirrors cmd/smartrecruiters's printSection).
			text = sec.Text.Value
		}
		parts = append(parts, cmp.Or(sec.Title.Value, s.fallbackTitle)+":\n"+text)
	}
	return strings.Join(parts, "\n\n")
}
