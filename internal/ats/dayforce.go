package ats

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/jaytaylor/html2text"
	"golang.org/x/sync/errgroup"

	"github.com/amikai/openings-mcp/internal/provider/dayforce"
)

var _ Adapter = (*DayforceAdapter)(nil)

// dayforceJobsHost is the public candidate portal origin every rendered
// URL uses, independent of the API base URL the client was constructed
// with (tests point the client at a mock server but still render the real
// public host, matching smartrecruiters.RosterCompany.CareersURL).
const dayforceJobsHost = "https://jobs.dayforcehcm.com"

// dayforcePageSize is the live API's fixed, non-negotiable search page
// size (verified live — see internal/provider/dayforce openapi.yaml).
// Search remaps it onto the unified ats.pageSize the way iCIMS does.
const dayforcePageSize = 25

// dayforceCultureRE matches one culture segment, e.g. "en-US".
var dayforceCultureRE = regexp.MustCompile(`(?i)^[a-z]{2}-[a-z]{2}$`)

// dayforceDefaultCulture is the default for locale-less board URLs and slugs.
const dayforceDefaultCulture = "en-US"

// dayforceDefaultBoardCode is the default job board code most tenants
// serve, and the fallback when a legacy-host URL omits the board segment.
const dayforceDefaultBoardCode = "CANDIDATEPORTAL"

// dayforceDefaultDistance is the search radius (miles) applied whenever a
// location string is supplied, matching the value observed in every live
// capture that combined locationString with a distance.
const dayforceDefaultDistance = 50

// dayforceFilterKeys names every key [DayforceAdapter.Search] accepts via
// SearchParams.Filters. department, pay_class, and pay_type resolve
// against the board's posting-attribute catalog; travel_required is a
// static true/false dimension advertised by [DayforceAdapter.Filters].
var dayforceFilterKeys = map[string]bool{
	"department":      true,
	"pay_class":       true,
	"pay_type":        true,
	"travel_required": true,
}

// dayforceBoardURLRE matches the current jobs.dayforcehcm.com board URL
// shape, with or without a culture segment and with or without a trailing
// posting id. ns is deliberately unconstrained beyond "no slash", so an
// unmatched-length path (e.g. the API's own /api/geo/... routes, which are
// four or more segments deep) fails the trailing `$` anchor rather than
// needing an explicit exclusion list.
//
// Examples (hostname + escaped path):
//   - jobs.dayforcehcm.com/en-US/pca/CANDIDATEPORTAL
//   - jobs.dayforcehcm.com/en-US/pca/CANDIDATEPORTAL/jobs/62374
//   - jobs.dayforcehcm.com/unknown/CANDIDATEPORTAL (locale-less form)
var dayforceBoardURLRE = regexp.MustCompile(
	`(?i)^jobs\.dayforcehcm\.com/(?:(?P<culture>[a-z]{2}-[a-z]{2})/)?(?P<ns>[^/]+)/(?P<xref>[^/]+)(?:/jobs/[^/]+)?/?$`,
)

// dayforceLegacyURLRE matches the legacy per-tenant host and the
// www.mydayforce.com host that redirects to it. The board segment is
// optional; a bare .../CandidatePortal/{culture}/{ns} addresses the
// default CANDIDATEPORTAL board.
//
// Examples (hostname + escaped path):
//   - us1234.dayforcehcm.com/CandidatePortal/en-US/pca
//   - us1234.dayforcehcm.com/CandidatePortal/en-US/mydayforce/Site/alljobs
//   - www.mydayforce.com/CandidatePortal/en-US/pca
var dayforceLegacyURLRE = regexp.MustCompile(
	`(?i)^(?:us\d+\.dayforcehcm\.com|www\.mydayforce\.com)/CandidatePortal/(?P<culture>[a-z]{2}-[a-z]{2})/(?P<ns>[^/]+?)(?:/Site/(?P<xref>[^/]+))?/?$`,
)

// dayforceReservedFirstSegments are jobs.dayforcehcm.com path prefixes that
// must never be mistaken for a tenant namespace, kept as an explicit
// defense-in-depth check alongside dayforceBoardURLRE's anchoring (every
// real route under these prefixes is deeper than the two segments the
// regex allows, but a future shallow route should still not resolve).
var dayforceReservedFirstSegments = map[string]bool{
	"api":     true,
	"_next":   true,
	"profile": true,
}

// DayforceAdapter serves Dayforce (Ceridian) candidate portal boards.
// Search, its filter catalogs, and job detail are all server-side JSON
// behind a next-auth CSRF pre-flight (see dayforce.BoardClient); only the
// board's public identity page is HTML. Roster slugs are tenant
// namespaces, suffixed with the job board code for a non-default board
// (see dayforce.Company.Slug); ParseCareersURL mints a canonical slug for
// non-roster boards. jobBoardId is resolved lazily via SiteInfo on the
// paths that need it (Filters, Detail, filtered Search).
type DayforceAdapter struct {
	client   *dayforce.BoardClient
	boardIDs sync.Map // ns/xref/culture → jobBoardId
}

// NewDayforceAdapter creates a Dayforce ATS adapter. baseURL is the
// candidate portal API origin (production: dayforceJobsHost); tests point
// it at a mock server.
func NewDayforceAdapter(baseURL string, hc *http.Client) (*DayforceAdapter, error) {
	client, err := dayforce.NewBoardClient(baseURL, hc)
	if err != nil {
		return nil, fmt.Errorf("create dayforce board client: %w", err)
	}
	return &DayforceAdapter{client: client}, nil
}

func (a *DayforceAdapter) Name() string { return "dayforce" }

func (a *DayforceAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(dayforce.Companies))
	for _, c := range dayforce.Companies {
		infos = append(infos, CompanyInfo{Slug: strings.ToLower(c.Slug()), Name: c.Name})
	}
	return infos
}

// ParseCareersURL accepts current jobs.dayforcehcm.com board or job URLs and
// legacy <region>.dayforcehcm.com CandidatePortal URLs.
//
// Search has two entry points that cannot share one slug shape. A curated
// row keeps namespace, board, and culture in YAML, so its Search key is the
// short [dayforce.Company.Slug] (usually just the namespace). A careers URL
// for a board that is not in the roster has nowhere else to put those three
// fields, so this method mints "ns/board" and appends "/culture" only when
// culture is not en-US — [DayforceAdapter.resolveSlug] splits that form back
// apart. Roster matching is the namespace+board fields, not slug strings: a
// hit returns Company.Slug() (e.g. "pca"); "pca" and "pca/candidateportal"
// are never compared to each other.
//
// Examples:
//   - https://jobs.dayforcehcm.com/en-US/pca/CANDIDATEPORTAL/jobs/62374
//     returns ("pca", true)
//   - https://jobs.dayforcehcm.com/en-US/pca/ENGINEERING
//     returns ("pca/engineering", true)
//   - https://jobs.dayforcehcm.com/fr-CA/unknown/CANDIDATEPORTAL
//     returns ("unknown/candidateportal/fr-CA", true)
//   - https://us1234.dayforcehcm.com/CandidatePortal/en-US/mydayforce/Site/alljobs
//     returns ("mydayforce/alljobs", true)
//
// Non-Dayforce, API, asset, and incomplete board URLs return ("", false).
func (a *DayforceAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	ns, xref, culture, ok := parseDayforceCareersURL(u)
	if !ok {
		return "", false
	}
	for _, c := range dayforce.Companies {
		if strings.EqualFold(c.Namespace, ns) && strings.EqualFold(c.JobBoardCode, xref) {
			return strings.ToLower(c.Slug()), true
		}
	}
	slug := strings.ToLower(ns) + "/" + strings.ToLower(xref)
	culture = canonicalDayforceCulture(culture)
	if culture != "" && culture != dayforceDefaultCulture {
		slug += "/" + culture
	}
	return slug, true
}

// parseDayforceCareersURL extracts the tenant namespace, job board code, and
// culture from a recognized Dayforce URL, or reports ok=false for anything
// else, including /api/..., /_next/..., /profile, and the bare host.
func parseDayforceCareersURL(u *url.URL) (ns, xref, culture string, ok bool) {
	host := strings.ToLower(u.Hostname())
	subject := host + u.EscapedPath()

	if m := dayforceBoardURLRE.FindStringSubmatch(subject); m != nil {
		ns = namedGroup(dayforceBoardURLRE, m, "ns")
		xref = namedGroup(dayforceBoardURLRE, m, "xref")
		culture = namedGroup(dayforceBoardURLRE, m, "culture")
		if dayforceReservedFirstSegments[strings.ToLower(ns)] {
			return "", "", "", false
		}
		// A two-segment path is ambiguous: the optional culture group
		// backtracks into ns, so /en-US/pca parses as ns "en-US". A
		// culture-shaped namespace is far less likely than a truncated board
		// URL, so reject rather than mint a slug that can only 404.
		if culture == "" && dayforceCultureRE.MatchString(ns) {
			return "", "", "", false
		}
		return dayforceUnescapeSegment(ns), dayforceUnescapeSegment(xref), canonicalDayforceCulture(culture), ns != "" && xref != ""
	}

	if m := dayforceLegacyURLRE.FindStringSubmatch(subject); m != nil {
		ns = namedGroup(dayforceLegacyURLRE, m, "ns")
		xref = namedGroup(dayforceLegacyURLRE, m, "xref")
		culture = namedGroup(dayforceLegacyURLRE, m, "culture")
		if xref == "" {
			xref = dayforceDefaultBoardCode
		}
		return dayforceUnescapeSegment(ns), dayforceUnescapeSegment(xref), canonicalDayforceCulture(culture), ns != ""
	}

	return "", "", "", false
}

func canonicalDayforceCulture(culture string) string {
	if !dayforceCultureRE.MatchString(culture) {
		return ""
	}
	parts := strings.SplitN(culture, "-", 2)
	return strings.ToLower(parts[0]) + "-" + strings.ToUpper(parts[1])
}

// dayforceUnescapeSegment URL-decodes one path segment, falling back to
// the raw (still-escaped) value on a malformed escape rather than failing
// the whole match — an unlikely edge case not worth rejecting the URL over.
func dayforceUnescapeSegment(s string) string {
	if decoded, err := url.PathUnescape(s); err == nil {
		return decoded
	}
	return s
}

// CareersURL renders the roster company's public candidate portal page.
func (a *DayforceAdapter) CareersURL(slug string) (string, bool) {
	c, ok := dayforce.CompaniesBySlug[strings.ToLower(slug)]
	if !ok {
		return "", false
	}
	return dayforceBoardURL(c), true
}

func dayforceBoardURL(c dayforce.Company) string {
	return fmt.Sprintf("%s/%s/%s/%s", dayforceJobsHost, c.Culture(), c.Namespace, c.JobBoardCode)
}

// dayforceJobURL renders one posting's public page, matching
// [DayforceAdapter.CareersURL]'s host and board segments.
func dayforceJobURL(board dayforce.Company, jobPostingID int) string {
	return fmt.Sprintf("%s/jobs/%d", dayforceBoardURL(board), jobPostingID)
}

// resolveSlug maps a slug to its board and display name. A roster slug
// (from [dayforce.Company.Slug]) hits CompaniesBySlug and uses that YAML
// row. Anything else that contains "/" is the form
// [DayforceAdapter.ParseCareersURL] mints for an unlisted URL — split into
// namespace, board, and optional culture; there is no second YAML lookup.
// Non-roster rows leave JobBoardID unset; [DayforceAdapter.jobBoardID]
// fills it from a memoized SiteInfo on the paths that need it.
func (a *DayforceAdapter) resolveSlug(_ context.Context, slug string) (name string, board dayforce.Company, err error) {
	if c, ok := dayforce.CompaniesBySlug[strings.ToLower(slug)]; ok {
		return c.Name, c, nil
	}

	parts := strings.Split(slug, "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" {
		return "", dayforce.Company{}, fmt.Errorf("dayforce: unknown company %q; pass a roster slug or a jobs.dayforcehcm.com board URL", slug)
	}
	ns, xref := parts[0], parts[1]
	culture := dayforceDefaultCulture
	if len(parts) == 3 {
		culture = canonicalDayforceCulture(parts[2])
		if culture == "" {
			return "", dayforce.Company{}, fmt.Errorf("dayforce: unknown company %q; pass a roster slug or a jobs.dayforcehcm.com board URL", slug)
		}
	}

	board = dayforce.Company{
		Name:         ns,
		Namespace:    ns,
		JobBoardCode: xref,
		CultureCode:  culture,
	}
	return ns, board, nil
}

// jobBoardID returns board.JobBoardID, filling it from a memoized SiteInfo
// fetch for non-roster boards whose public URL never carries the id.
func (a *DayforceAdapter) jobBoardID(ctx context.Context, board *dayforce.Company) (int, error) {
	if board.JobBoardID > 0 {
		return board.JobBoardID, nil
	}
	key := board.Namespace + "/" + board.JobBoardCode + "/" + board.Culture()
	if v, ok := a.boardIDs.Load(key); ok {
		id := v.(int)
		board.JobBoardID = id
		return id, nil
	}
	info, err := a.client.SiteInfo(ctx, board.Namespace, board.JobBoardCode, board.Culture())
	if err != nil {
		return 0, fmt.Errorf("dayforce: resolve board %s/%s: %w", board.Namespace, board.JobBoardCode, err)
	}
	a.boardIDs.Store(key, info.JobBoardId)
	board.JobBoardID = info.JobBoardId
	return info.JobBoardId, nil
}

func (a *DayforceAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	_, board, err := a.resolveSlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	page := clampPage(p.Page)
	pageIndex := page - 1
	if pageIndex > math.MaxInt/pageSize {
		return nil, fmt.Errorf("dayforce: page %d is too large; retry with a smaller page", page)
	}
	start := pageIndex * pageSize

	req := dayforce.SearchRequest{
		ClientNamespace: board.Namespace,
		JobBoardCode:    board.JobBoardCode,
		CultureCode:     board.Culture(),
	}
	if p.Query != "" {
		req.SearchText = dayforce.NewOptString(p.Query)
	}
	location := strings.TrimSpace(p.Location)
	remoteOnly := strings.EqualFold(location, "remote")
	if location != "" && !remoteOnly {
		// distanceUnit 0 is miles — the value the portal's own search body
		// sends alongside distance (see the plan doc's captured payload).
		const distanceUnitMiles = 0
		req.LocationString = dayforce.NewOptString(location)
		req.Distance = dayforce.NewOptFloat64(dayforceDefaultDistance)
		req.DistanceUnit = dayforce.NewOptInt(distanceUnitMiles)
	}
	if err := a.applySearchFilters(ctx, &board, &req, p.Filters); err != nil {
		return nil, err
	}

	if remoteOnly {
		return a.searchRemote(ctx, slug, board, req, start, page)
	}

	correctPr := start / dayforcePageSize
	offsetInPage := start % dayforcePageSize

	first, err := a.searchPage(ctx, slug, req, correctPr*dayforcePageSize)
	if err != nil {
		return nil, err
	}
	if start >= first.MaxCount {
		// A non-roster board's culture is assumed, and an unsupported culture
		// returns 200/maxCount 0 rather than an error — so a board that reports
		// no jobs at all is the one place worth paying for SiteInfo, which 404s
		// on a culture the board does not serve. A page merely past the end of a
		// non-empty board needs no probe.
		if first.MaxCount == 0 && board.JobBoardID == 0 {
			if _, err := a.jobBoardID(ctx, &board); err != nil {
				return nil, err
			}
		}
		return &SearchResult{Jobs: []JobSummary{}, TotalCount: first.MaxCount, Page: page, TotalPages: totalPages(first.MaxCount)}, nil
	}

	selected := make([]dayforce.SearchPosting, 0, pageSize)
	if offsetInPage < len(first.JobPostings) {
		selected = append(selected, first.JobPostings[offsetInPage:]...)
	}
	for nextStart := (correctPr + 1) * dayforcePageSize; len(selected) < pageSize && nextStart < first.MaxCount; nextStart += dayforcePageSize {
		more, err := a.searchPage(ctx, slug, req, nextStart)
		if err != nil {
			return nil, err
		}
		selected = append(selected, more.JobPostings...)
	}
	if len(selected) > pageSize {
		selected = selected[:pageSize]
	}

	return &SearchResult{
		Jobs:       dayforceSummaries(selected, board),
		TotalCount: first.MaxCount,
		Page:       page,
		TotalPages: totalPages(first.MaxCount),
	}, nil
}

func (a *DayforceAdapter) searchPage(ctx context.Context, slug string, req dayforce.SearchRequest, paginationStart int) (*dayforce.SearchResponse, error) {
	req.PaginationStart = dayforce.NewOptInt(paginationStart)
	res, err := a.client.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("dayforce: search %q: %w", slug, err)
	}
	return res, nil
}

// searchRemote walks upstream pages and keeps hasVirtualLocation rows.
// Dayforce has no virtual-work search field; sending locationString
// "remote" geocodes to a 50-mile radius and returns zero jobs.
func (a *DayforceAdapter) searchRemote(ctx context.Context, slug string, board dayforce.Company, req dayforce.SearchRequest, start, page int) (*SearchResult, error) {
	first, err := a.searchPage(ctx, slug, req, 0)
	if err != nil {
		return nil, err
	}
	if first.MaxCount == 0 && board.JobBoardID == 0 {
		if _, err := a.jobBoardID(ctx, &board); err != nil {
			return nil, err
		}
		return &SearchResult{Jobs: []JobSummary{}, TotalCount: 0, Page: page, TotalPages: 0}, nil
	}

	matched := make([]dayforce.SearchPosting, 0)
	res := first
	for offset := 0; ; offset += dayforcePageSize {
		for _, p := range res.JobPostings {
			if p.HasVirtualLocation {
				matched = append(matched, p)
			}
		}
		if offset+dayforcePageSize >= res.MaxCount {
			break
		}
		res, err = a.searchPage(ctx, slug, req, offset+dayforcePageSize)
		if err != nil {
			return nil, err
		}
	}

	if start >= len(matched) {
		return &SearchResult{Jobs: []JobSummary{}, TotalCount: len(matched), Page: page, TotalPages: totalPages(len(matched))}, nil
	}
	end := min(start+pageSize, len(matched))
	return &SearchResult{
		Jobs:       dayforceSummaries(matched[start:end], board),
		TotalCount: len(matched),
		Page:       page,
		TotalPages: totalPages(len(matched)),
	}, nil
}

// applySearchFilters resolves the unified filter set onto req's typed
// fields. department/pay_class/pay_type each take exactly one value —
// Dayforce's search body has one int slot per dimension, with no way to OR
// several ids in one request — so more than one value for a key is a
// teaching error rather than a silently-dropped dimension.
func (a *DayforceAdapter) applySearchFilters(ctx context.Context, board *dayforce.Company, req *dayforce.SearchRequest, filters FilterSet) error {
	for key, values := range filters {
		if len(values) != 1 {
			return fmt.Errorf("dayforce: filter %q takes exactly one value, got %d", key, len(values))
		}
		value := values[0]

		switch key {
		case "department":
			id, err := a.resolveAttribute(ctx, a.client.Departments, board, value, key)
			if err != nil {
				return err
			}
			req.DepartmentId = dayforce.NewOptInt(id)
		case "pay_class":
			id, err := a.resolveAttribute(ctx, a.client.PayClasses, board, value, key)
			if err != nil {
				return err
			}
			req.PayClass = dayforce.NewOptInt(id)
		case "pay_type":
			id, err := a.resolveAttribute(ctx, a.client.PayTypes, board, value, key)
			if err != nil {
				return err
			}
			req.PayType = dayforce.NewOptInt(id)
		case "travel_required":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf(`filter value %q not found for %q; available: "true", "false"`, value, key)
			}
			req.TravelRequired = dayforce.NewOptBool(b)
		default:
			return errUnknownFilterKey(key, dayforceFilterKeys)
		}
	}
	return nil
}

// resolveAttribute accepts either a raw attributeId (passed through
// unchanged, so a value round-tripped from [DayforceAdapter.Filters]'s
// display labels still works if a caller instead echoes an id) or a
// display label (exact case-insensitive match against attributeValue).
func (a *DayforceAdapter) resolveAttribute(
	ctx context.Context,
	fetch func(context.Context, string, int, string) (*dayforce.PostingAttributeList, error),
	board *dayforce.Company,
	input, key string,
) (int, error) {
	if _, err := a.jobBoardID(ctx, board); err != nil {
		return 0, err
	}
	list, err := fetch(ctx, board.Namespace, board.JobBoardID, board.Culture())
	if err != nil {
		return 0, fmt.Errorf("dayforce: fetch %s attributes: %w", key, err)
	}

	if id, convErr := strconv.Atoi(input); convErr == nil {
		for _, attr := range list.PostingAttributesAttributesList {
			if attr.AttributeId == id {
				return id, nil
			}
		}
	}

	labels := make([]string, 0, len(list.PostingAttributesAttributesList))
	for _, attr := range list.PostingAttributesAttributesList {
		if strings.EqualFold(attr.AttributeValue, input) {
			return attr.AttributeId, nil
		}
		labels = append(labels, attr.AttributeValue)
	}
	return 0, fmt.Errorf("filter value %q not found for %q; available: %s", input, key, joinTruncated(labels))
}

func dayforceSummaries(postings []dayforce.SearchPosting, board dayforce.Company) []JobSummary {
	jobs := make([]JobSummary, 0, len(postings))
	for _, p := range postings {
		jobs = append(jobs, JobSummary{
			JobID:    strconv.Itoa(p.JobPostingId),
			Title:    p.JobTitle,
			Location: dayforceSearchLocation(p.PostingLocations, p.HasVirtualLocation),
			PostedAt: isoDate(p.PostingStartTimestampUTC),
			URL:      dayforceJobURL(board, p.JobPostingId),
		})
	}
	return jobs
}

// dayforceSearchLocation joins a search row's posting locations, prefixed
// with "Remote" when hasVirtualLocation is set — the list-level remote
// signal (detail carries the same field; see dayforceDetailLocation).
func dayforceSearchLocation(locations []dayforce.SearchPostingLocation, hasVirtualLocation bool) string {
	addrs := make([]string, 0, len(locations))
	for _, l := range locations {
		if l.FormattedAddress != "" {
			addrs = append(addrs, l.FormattedAddress)
		}
	}
	return dayforceJoinLocation(addrs, hasVirtualLocation)
}

func dayforceDetailLocation(locations []dayforce.DetailPostingLocation, hasVirtualLocation bool) string {
	addrs := make([]string, 0, len(locations))
	for _, l := range locations {
		if l.FormattedAddress != "" {
			addrs = append(addrs, l.FormattedAddress)
		}
	}
	return dayforceJoinLocation(addrs, hasVirtualLocation)
}

func dayforceJoinLocation(addrs []string, hasVirtualLocation bool) string {
	base := strings.Join(addrs, "; ")
	if !hasVirtualLocation {
		return base
	}
	if base == "" {
		return "Remote"
	}
	return "Remote · " + base
}

func (a *DayforceAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	_, board, err := a.resolveSlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if _, err := a.jobBoardID(ctx, &board); err != nil {
		return nil, err
	}

	var departments, payClasses, payTypes *dayforce.PostingAttributeList
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		departments, err = a.client.Departments(gCtx, board.Namespace, board.JobBoardID, board.Culture())
		return err
	})
	g.Go(func() (err error) {
		payClasses, err = a.client.PayClasses(gCtx, board.Namespace, board.JobBoardID, board.Culture())
		return err
	})
	g.Go(func() (err error) {
		payTypes, err = a.client.PayTypes(gCtx, board.Namespace, board.JobBoardID, board.Culture())
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("dayforce: filters %q: %w", slug, err)
	}

	return FilterSet{
		"department":      dayforceAttributeLabels(departments),
		"pay_class":       dayforceAttributeLabels(payClasses),
		"pay_type":        dayforceAttributeLabels(payTypes),
		"travel_required": []string{"true", "false"},
	}, nil
}

func dayforceAttributeLabels(list *dayforce.PostingAttributeList) []string {
	labels := make([]string, 0, len(list.PostingAttributesAttributesList))
	for _, attr := range list.PostingAttributesAttributesList {
		labels = append(labels, attr.AttributeValue)
	}
	return labels
}

func (a *DayforceAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	name, board, err := a.resolveSlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	id, convErr := strconv.Atoi(jobID)
	if convErr != nil {
		return nil, fmt.Errorf("dayforce: job id %q must be numeric; pass a job_id exactly as returned by the job search", jobID)
	}
	if _, err := a.jobBoardID(ctx, &board); err != nil {
		return nil, err
	}

	d, err := a.client.Job(ctx, board.Namespace, board.Culture(), board.JobBoardID, id)
	if err != nil {
		if errors.Is(err, dayforce.ErrJobNotFound) {
			return nil, fmt.Errorf("dayforce: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
		}
		return nil, fmt.Errorf("dayforce: fetch job %q for %q: %w", jobID, slug, err)
	}

	return &JobDetail{
		JobID:       strconv.Itoa(d.JobPostingId),
		Title:       d.JobTitle,
		Company:     name,
		Location:    dayforceDetailLocation(d.PostingLocations, d.HasVirtualLocation),
		PostedAt:    isoDate(d.PostingStartTimestampUTC),
		URL:         dayforceJobURL(board, d.JobPostingId),
		Description: dayforceDescription(d.JobPostingContent),
	}, nil
}

// dayforceDescription renders jobPostingContent's three HTML fields as one
// plain-text body, in header/body/footer order, separated by a blank line.
// Any field that fails to convert falls back to its raw HTML rather than
// dropping it.
func dayforceDescription(c dayforce.JobPostingContent) string {
	var b strings.Builder
	for _, html := range []string{
		c.JobDescriptionHeader.Or(""),
		c.JobDescription.Or(""),
		c.JobDescriptionFooter.Or(""),
	} {
		if html == "" {
			continue
		}
		text, err := html2text.FromString(html, html2text.Options{})
		if err != nil {
			text = html
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(text)
	}
	return b.String()
}
