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
	"golang.org/x/sync/errgroup"

	"github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

var _ Adapter = (*SmartRecruitersAdapter)(nil)

// smartRecruitersWalkPageSize is the Posting API's maximum page size, used
// when enumerating a whole board.
const smartRecruitersWalkPageSize = 100

// smartRecruitersWalkConcurrency bounds parallel page fetches during a
// board walk, keeping a ~50-page board (Bosch) fast without hammering the
// upstream.
const smartRecruitersWalkConcurrency = 8

// SmartRecruitersAdapter serves SmartRecruiters-hosted companies via the
// public Posting API. Boards reach ~5k postings, so unlike the other
// non-Workday adapters Search cannot dump-and-filter; it uses the API's
// real server-side narrowing (q, city/region/country, department, offset
// paging). Two unified inputs need translation the API doesn't offer
// directly: fuzzy location text is classified into the one structured
// location parameter that matches via one-row probe requests, and
// department display labels are resolved to the ids the upstream wants by
// walking the full posting list — the API has no dimension-enumeration
// endpoint, so that walk is also how Filters() works.
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
		infos = append(infos, CompanyInfo{Slug: c.CompanyIdentifier, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes jobs.smartrecruiters.com career sites; the
// first path segment is the companyIdentifier. Roster companies fold back
// to their roster casing. The reserved sr-jobs segment (the cross-company
// search page) is not a company: accepting it would silently search an
// empty board, since the API answers an unknown companyIdentifier with an
// empty 200 rather than a 404.
func (a *SmartRecruitersAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if strings.ToLower(u.Hostname()) != "jobs.smartrecruiters.com" {
		return "", false
	}
	seg := firstPathSegment(u)
	if seg == "" || strings.EqualFold(seg, "sr-jobs") {
		return "", false
	}
	if c, ok := smartrecruiters.CompaniesByIdentifier[strings.ToLower(seg)]; ok {
		return c.CompanyIdentifier, true
	}
	return seg, true
}

func (a *SmartRecruitersAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	page := clampPage(p.Page)
	pageIndex := page - 1
	if pageIndex > math.MaxInt/PageSize {
		return nil, fmt.Errorf("smartrecruiters: page %d is too large; retry with a smaller page", page)
	}
	params := smartrecruiters.ListPostingsParams{
		CompanyIdentifier: slug,
		Limit:             smartrecruiters.NewOptInt(PageSize),
		Offset:            smartrecruiters.NewOptInt(pageIndex * PageSize),
	}
	if q := strings.TrimSpace(p.Query); q != "" {
		params.Q = smartrecruiters.NewOptString(q)
	}
	if loc := strings.TrimSpace(p.Location); loc != "" {
		apply, err := a.resolveLocation(ctx, slug, loc)
		if err != nil {
			return nil, err
		}
		apply(&params)
	}
	if len(p.Filters) > 0 {
		id, err := a.resolveDepartmentFilter(ctx, slug, p.Filters)
		if err != nil {
			return nil, err
		}
		params.Department = smartrecruiters.NewOptString(id)
	}
	rsp, err := a.client.ListPostings(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("smartrecruiters: search %q: %w", slug, err)
	}
	jobs := make([]JobSummary, 0, len(rsp.Content))
	for _, ps := range rsp.Content {
		id := ps.ID.Value
		if id == "" {
			// A row without an id can't be detailed; skip rather than hand
			// out an unusable job_id.
			continue
		}
		jobs = append(jobs, JobSummary{
			JobID:    id,
			Title:    ps.Name.Or(""),
			Location: ps.Location.Value.FullLocation.Or(""),
			PostedAt: smartRecruitersPostedAt(ps.ReleasedDate),
			URL:      smartRecruitersPostingURL(cmp.Or(ps.Company.Value.Identifier.Or(""), slug), id),
		})
	}
	return &SearchResult{
		Jobs:       jobs,
		TotalCount: rsp.TotalFound,
		Page:       page,
		TotalPages: totalPages(rsp.TotalFound),
	}, nil
}

func (a *SmartRecruitersAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	depts, err := a.departments(ctx, slug)
	if err != nil {
		return nil, err
	}
	if len(depts.labels) == 0 {
		// department is the only server-side dimension; a board without one
		// (or an unknown company — the API's empty-200 quirk makes them
		// indistinguishable) has nothing to filter on.
		return FilterSet{}, nil
	}
	return FilterSet{"department": depts.labels}, nil
}

func (a *SmartRecruitersAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	res, err := a.client.GetPosting(ctx, smartrecruiters.GetPostingParams{
		CompanyIdentifier: slug,
		PostingId:         jobID,
	})
	if err != nil {
		return nil, fmt.Errorf("smartrecruiters: fetch job %q for %q: %w", jobID, slug, err)
	}
	switch d := res.(type) {
	case *smartrecruiters.PostingDetail:
		return &JobDetail{
			JobID: jobID,
			Title: d.Name.Or(""),
			Company: cmp.Or(
				smartrecruiters.CompaniesByIdentifier[strings.ToLower(slug)].Name,
				d.Company.Value.Name.Or(""),
				slug,
			),
			Location:    d.Location.Value.FullLocation.Or(""),
			PostedAt:    smartRecruitersPostedAt(d.ReleasedDate),
			URL:         d.PostingUrl.Or(""),
			Description: smartRecruitersDescription(d.JobAd),
		}, nil
	case *smartrecruiters.PostingErrorResponse:
		return nil, fmt.Errorf("smartrecruiters: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
	default:
		return nil, fmt.Errorf("smartrecruiters: unexpected response type %T", res)
	}
}

// resolveLocation classifies fuzzy location text into the one structured
// query parameter the Posting API accepts — city, region, or country — by
// probing each with a one-row request until one matches, mirroring
// Workday's facet probe. The API has no remote filter (remote=true is
// accepted and silently ignored), so "remote" gets a teaching error rather
// than a parameter that matches everything.
func (a *SmartRecruitersAdapter) resolveLocation(ctx context.Context, slug, loc string) (func(*smartrecruiters.ListPostingsParams), error) {
	if strings.EqualFold(loc, "remote") {
		return nil, fmt.Errorf("smartrecruiters: company %q cannot be filtered by remote; search without a location and check each posting's detail instead", slug)
	}
	probes := []struct {
		value string
		apply func(*smartrecruiters.ListPostingsParams, string)
	}{
		{loc, func(p *smartrecruiters.ListPostingsParams, v string) { p.City = smartrecruiters.NewOptString(v) }},
		{loc, func(p *smartrecruiters.ListPostingsParams, v string) { p.Region = smartrecruiters.NewOptString(v) }},
		// The upstream matches location.country codes only in lowercase.
		{strings.ToLower(loc), func(p *smartrecruiters.ListPostingsParams, v string) { p.Country = smartrecruiters.NewOptString(v) }},
	}
	for _, pr := range probes {
		params := smartrecruiters.ListPostingsParams{
			CompanyIdentifier: slug,
			Limit:             smartrecruiters.NewOptInt(1),
		}
		pr.apply(&params, pr.value)
		rsp, err := a.client.ListPostings(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("smartrecruiters: probe location %q for %q: %w", loc, slug, err)
		}
		if rsp.TotalFound > 0 {
			return func(p *smartrecruiters.ListPostingsParams) { pr.apply(p, pr.value) }, nil
		}
	}
	return nil, fmt.Errorf("no postings matching location %q for company %q (or the company has none at all); pass a city, a state/region code (e.g. TX), or an ISO country code (e.g. us), or drop location", loc, slug)
}

// resolveDepartmentFilter maps the unified filters onto the Posting API's
// single filterable dimension. Only one department value is accepted: the
// upstream keeps just one value per parameter (verified live), so OR
// semantics across several would silently drop all but the first.
func (a *SmartRecruitersAdapter) resolveDepartmentFilter(ctx context.Context, slug string, filters FilterSet) (string, error) {
	for key := range filters {
		if key != "department" {
			return "", errUnknownFilterKey(key, map[string]bool{"department": true})
		}
	}
	values := filters["department"]
	if len(values) != 1 {
		return "", fmt.Errorf("smartrecruiters: the department filter takes exactly one value, got %d", len(values))
	}
	depts, err := a.departments(ctx, slug)
	if err != nil {
		return "", err
	}
	id, ok := depts.idByLabel[strings.ToLower(values[0])]
	if !ok {
		if len(depts.labels) == 0 {
			return "", errUnknownFilterKey("department", nil)
		}
		const maxListed = 20
		listed := depts.labels
		suffix := ""
		if len(listed) > maxListed {
			listed = listed[:maxListed]
			suffix = ", …"
		}
		return "", fmt.Errorf("filter value %q not found for \"department\"; available: %s%s", values[0], strings.Join(listed, ", "), suffix)
	}
	return id, nil
}

// smartRecruitersDepartments is a board's department dimension: sorted
// display labels for Filters(), and a label→id index for Search, since the
// department query parameter wants the id, not the label.
type smartRecruitersDepartments struct {
	labels    []string
	idByLabel map[string]string // key: lowercased label
}

func (a *SmartRecruitersAdapter) departments(ctx context.Context, slug string) (*smartRecruitersDepartments, error) {
	postings, err := a.walkPostings(ctx, slug)
	if err != nil {
		return nil, err
	}
	d := &smartRecruitersDepartments{idByLabel: make(map[string]string)}
	for _, ps := range postings {
		dep, ok := ps.Department.Get()
		if !ok {
			continue
		}
		label := dep.Label.Or("")
		id := smartRecruitersDepartmentID(dep)
		if label == "" || id == "" {
			continue
		}
		if _, seen := d.idByLabel[strings.ToLower(label)]; seen {
			continue
		}
		d.idByLabel[strings.ToLower(label)] = id
		d.labels = append(d.labels, label)
	}
	slices.Sort(d.labels)
	return d, nil
}

// walkPostings fetches a company's complete posting list, 100 rows per
// request, pages after the first fetched concurrently. A ~5k-posting board
// costs ~50 requests — the stateless price of enumerating a dimension the
// API can't. A first page shorter than requested ends the walk regardless
// of what totalFound claims.
func (a *SmartRecruitersAdapter) walkPostings(ctx context.Context, slug string) ([]smartrecruiters.PostingSummary, error) {
	first, err := a.listPage(ctx, slug, 0)
	if err != nil {
		return nil, err
	}
	out := first.Content
	if len(out) < smartRecruitersWalkPageSize || first.TotalFound <= len(out) {
		return out, nil
	}
	rest := (first.TotalFound - 1) / smartRecruitersWalkPageSize
	pages := make([][]smartrecruiters.PostingSummary, rest)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(smartRecruitersWalkConcurrency)
	for i := range pages {
		g.Go(func() error {
			rsp, err := a.listPage(gctx, slug, (i+1)*smartRecruitersWalkPageSize)
			if err != nil {
				return err
			}
			pages[i] = rsp.Content
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	for _, p := range pages {
		out = append(out, p...)
	}
	return out, nil
}

func (a *SmartRecruitersAdapter) listPage(ctx context.Context, slug string, offset int) (*smartrecruiters.PostingListResponse, error) {
	rsp, err := a.client.ListPostings(ctx, smartrecruiters.ListPostingsParams{
		CompanyIdentifier: slug,
		Limit:             smartrecruiters.NewOptInt(smartRecruitersWalkPageSize),
		Offset:            smartrecruiters.NewOptInt(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("smartrecruiters: list postings for %q at offset %d: %w", slug, offset, err)
	}
	return rsp, nil
}

// smartRecruitersDepartmentID renders department.id, which the upstream
// serves as a string on list responses but an integer on detail responses.
func smartRecruitersDepartmentID(dep smartrecruiters.Department) string {
	id, ok := dep.ID.Get()
	if !ok {
		return ""
	}
	if s, ok := id.GetString(); ok {
		return s
	}
	if n, ok := id.GetInt(); ok {
		return strconv.Itoa(n)
	}
	return ""
}

// smartRecruitersPostingURL builds the public posting page. The canonical
// URL carries an SEO title suffix, but the id-only form serves the same
// page (verified live), and the list response has no URL field to copy.
func smartRecruitersPostingURL(companyIdentifier, postingID string) string {
	return fmt.Sprintf("https://jobs.smartrecruiters.com/%s/%s",
		url.PathEscape(companyIdentifier), url.PathEscape(postingID))
}

func smartRecruitersPostedAt(t smartrecruiters.OptNilDateTime) string {
	v, ok := t.Get()
	if !ok {
		return ""
	}
	return isoDate(v)
}

// smartRecruitersDescription renders the posting's jobAd sections as plain
// text in the upstream's fixed order, falling back to a generic heading
// when a section omits its own title and to the raw HTML when conversion
// fails, rather than dropping the section.
func smartRecruitersDescription(ad smartrecruiters.OptJobAd) string {
	sections, ok := ad.Value.Sections.Get()
	if !ok {
		return ""
	}
	ordered := []struct {
		fallbackTitle string
		section       smartrecruiters.OptJobAdSection
	}{
		{"Company Description", sections.CompanyDescription},
		{"Job Description", sections.JobDescription},
		{"Qualifications", sections.Qualifications},
		{"Additional Information", sections.AdditionalInformation},
	}
	var b strings.Builder
	for _, s := range ordered {
		sec, ok := s.section.Get()
		if !ok || sec.Text.Or("") == "" {
			continue
		}
		text, err := html2text.FromString(sec.Text.Or(""), html2text.Options{})
		if err != nil {
			text = sec.Text.Or("")
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "%s:\n%s", cmp.Or(sec.Title.Or(""), s.fallbackTitle), text)
	}
	return b.String()
}
