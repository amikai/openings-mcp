package ats

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
	"golang.org/x/sync/errgroup"

	"github.com/amikai/openings-mcp/internal/provider/bamboohr"
)

var _ Adapter = (*BambooHRAdapter)(nil)

// bambooHRCareersHostRE matches BambooHR hosted careers-site hosts and
// captures the tenant slug (subdomain). Reserved product hosts are rejected
// after the match.
//
// Examples (hostname):
//   - concept2.bamboohr.com
//   - acme.bamboohr.com
var bambooHRCareersHostRE = regexp.MustCompile(
	`(?i)^(?P<slug>[^.]+)\.bamboohr\.com$`,
)

// BambooHRAdapter serves BambooHR hosted careers sites. The public
// /careers/list endpoint returns the complete board in one response but
// only as sparse summaries - descriptions, compensation, and posting dates
// live on the per-job /careers/{id}/detail endpoint. Filters and unqueried
// Search run searchDump over the list dump alone (results carry no posted
// date). When Search receives a non-empty query, the adapter fans out to
// /careers/{id}/detail so tier-3 skill/technology matching can see full
// JDs. Detail hits the per-job endpoint directly.
type BambooHRAdapter struct {
	hc      *http.Client
	baseURL func(slug string) string
}

// bambooHRDetailConcurrency caps concurrent detail fetches when Search
// enriches descriptions for query matching. BambooHR boards are typically
// small SMB boards; this keeps a large board from opening unbounded sockets.
const bambooHRDetailConcurrency = 8

// NewBambooHRAdapter derives a redirect-blocking copy of hc: BambooHR
// 302-redirects unknown tenants to its marketing site, and following that
// redirect would turn a diagnosable "no such tenant" into an HTML decode
// error.
func NewBambooHRAdapter(hc *http.Client) *BambooHRAdapter {
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &BambooHRAdapter{
		hc: &c,
		baseURL: func(slug string) string {
			return "https://" + slug + ".bamboohr.com"
		},
	}
}

func (a *BambooHRAdapter) Name() string { return "bamboohr" }

func (a *BambooHRAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(bamboohr.Companies))
	for _, c := range bamboohr.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Slug, Name: c.Name})
	}
	return infos
}

var bambooHRReservedHosts = map[string]bool{
	"api":           true,
	"app":           true,
	"careers":       true,
	"developers":    true,
	"documentation": true,
	"help":          true,
	"marketplace":   true,
	"partners":      true,
	"status":        true,
	"support":       true,
	"www":           true,
}

// ParseCareersURL recognizes BambooHR subdomain careers pages.
func (a *BambooHRAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	m := bambooHRCareersHostRE.FindStringSubmatch(strings.ToLower(u.Hostname()))
	if m == nil {
		return "", false
	}
	slug := namedGroup(bambooHRCareersHostRE, m, "slug")
	if slug == "" || bambooHRReservedHosts[slug] {
		return "", false
	}
	return slug, true
}

func (a *BambooHRAdapter) Search(
	ctx context.Context,
	slug string,
	p SearchParams,
) (*SearchResult, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	// The list feed has no JD text. Populate descriptions from detail so
	// the unified query contract (titles + skills/technologies) holds.
	if strings.TrimSpace(p.Query) != "" {
		if err := a.enrichDescriptions(ctx, slug, jobs); err != nil {
			return nil, err
		}
	}
	return searchDump(jobs, p)
}

func (a *BambooHRAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *BambooHRAdapter) Detail(
	ctx context.Context,
	slug string,
	jobID string,
) (*JobDetail, error) {
	slug = strings.ToLower(slug)
	client, err := a.client(slug)
	if err != nil {
		return nil, err
	}
	res, err := client.GetJobDetail(ctx, bamboohr.GetJobDetailParams{ID: jobID})
	if err != nil {
		return nil, fmt.Errorf("bamboohr: fetch job %q for %q: %w", jobID, slug, err)
	}

	switch r := res.(type) {
	case *bamboohr.DetailResponse:
		jo := r.Result.JobOpening
		return &JobDetail{
			JobID:       jobID,
			Title:       jo.JobOpeningName,
			Company:     cmp.Or(bamboohr.CompaniesBySlug[slug].Name, slug),
			Location:    bambooHRDetailLocation(&jo),
			PostedAt:    jo.DatePosted.Or(""),
			URL:         cmp.Or(jo.JobOpeningShareUrl, bambooHRPostingURL(slug, jobID)),
			Description: bambooHRDescription(jo.Description),
		}, nil
	case *bamboohr.NotFoundError:
		return nil, fmt.Errorf(
			"bamboohr: job %q not found for company %q; pass a job_id exactly as returned by the job search",
			jobID,
			slug,
		)
	case *bamboohr.GetJobDetailFound:
		return nil, fmt.Errorf("bamboohr: careers-site subdomain %q not found upstream", slug)
	default:
		return nil, fmt.Errorf("bamboohr: unexpected response type %T", res)
	}
}

func (a *BambooHRAdapter) client(slug string) (*bamboohr.Client, error) {
	client, err := bamboohr.NewClient(a.baseURL(slug), bamboohr.WithClient(a.hc))
	if err != nil {
		return nil, fmt.Errorf("bamboohr: create client for %q: %w", slug, err)
	}
	return client, nil
}

func (a *BambooHRAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	slug = strings.ToLower(slug)
	client, err := a.client(slug)
	if err != nil {
		return nil, err
	}
	res, err := client.ListJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("bamboohr: fetch board for %q: %w", slug, err)
	}

	var rows []bamboohr.ListJob
	switch r := res.(type) {
	case *bamboohr.ListResponse:
		rows = r.Result
	case *bamboohr.ListJobsFound:
		return nil, fmt.Errorf("bamboohr: careers-site subdomain %q not found upstream", slug)
	default:
		return nil, fmt.Errorf("bamboohr: unexpected response type %T", res)
	}

	jobs := make([]dumpJob, 0, len(rows))
	for _, row := range rows {
		fields := map[string][]string{}
		if v := row.DepartmentLabel.Or(""); v != "" {
			fields["department"] = []string{v}
		}
		if row.EmploymentStatusLabel != "" {
			fields["employmentType"] = []string{row.EmploymentStatusLabel}
		}
		workMode := bamboohr.WorkModeLabel(row.LocationType.Or(""))
		if workMode != "" {
			fields["workplaceType"] = []string{workMode}
		}

		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    row.ID,
				Title:    row.JobOpeningName,
				Location: bambooHRListLocation(&row),
				// The list feed carries no posting date; it lives only on
				// the detail endpoint.
				PostedAt: "",
				URL:      bambooHRPostingURL(slug, row.ID),
			},
			sortKey: time.Time{}, // no posting date in the dump; ordering falls to rank, then id
			orgUnit: row.DepartmentLabel.Or(""),
			// List rows carry no JD; Search fills this when Query is set.
			description: "",
			locations:   bambooHRSearchLocations(&row, workMode),
			fields:      fields,
			isRemote:    row.LocationType.Or("") == "1",
		})
	}
	return jobs, nil
}

// enrichDescriptions fills dumpJob.description from the detail endpoint so
// searchDump's tier-3 query matching can see skills and technologies. Jobs
// that 404 between list and detail are left blank rather than failing the
// whole search (boards can race with removals).
func (a *BambooHRAdapter) enrichDescriptions(
	ctx context.Context,
	slug string,
	jobs []dumpJob,
) error {
	if len(jobs) == 0 {
		return nil
	}
	client, err := a.client(slug)
	if err != nil {
		return err
	}
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(bambooHRDetailConcurrency)
	for i := range jobs {
		i := i
		g.Go(func() error {
			id := jobs[i].summary.JobID
			res, err := client.GetJobDetail(gCtx, bamboohr.GetJobDetailParams{ID: id})
			if err != nil {
				return fmt.Errorf("bamboohr: fetch job %q for %q: %w", id, slug, err)
			}
			switch r := res.(type) {
			case *bamboohr.DetailResponse:
				jobs[i].description = bambooHRDescription(r.Result.JobOpening.Description)
				return nil
			case *bamboohr.NotFoundError:
				return nil
			case *bamboohr.GetJobDetailFound:
				return fmt.Errorf("bamboohr: careers-site subdomain %q not found upstream", slug)
			default:
				return fmt.Errorf("bamboohr: unexpected response type %T", res)
			}
		})
	}
	return g.Wait()
}

// bambooHRPostingURL builds the human-clickable posting page, the same URL
// the detail endpoint reports as jobOpeningShareUrl.
func bambooHRPostingURL(slug, id string) string {
	return fmt.Sprintf("https://%s.bamboohr.com/careers/%s", slug, id)
}

// bambooHRListLocation renders a list row's display location, preferring
// the structured `location` and falling back to `atsLocation` (which alone
// carries the country) when the former is all-null.
func bambooHRListLocation(row *bamboohr.ListJob) string {
	if s := bambooHRJoin(row.Location.City.Or(""), row.Location.State.Or("")); s != "" {
		return s
	}
	return bambooHRJoin(row.AtsLocation.City.Or(""), row.AtsLocation.State.Or(""), row.AtsLocation.Country.Or(""))
}

// bambooHRDetailLocation prefers jobOpening.location and falls back to
// jobOpening.atsLocation when the structured location is all-null — the
// same fallback the list path uses. Some live postings (e.g. Ashtead job
// 35) publish their only usable locality on atsLocation.
func bambooHRDetailLocation(jo *bamboohr.JobOpening) string {
	if s := bambooHRJoin(
		jo.Location.City.Or(""),
		jo.Location.State.Or(""),
		jo.Location.AddressCountry.Or(""),
	); s != "" {
		return s
	}
	return bambooHRJoin(
		jo.AtsLocation.City.Or(""),
		jo.AtsLocation.State.Or(""),
		jo.AtsLocation.Country.Or(""),
	)
}

// bambooHRSearchLocations joins every location string a row carries for
// fuzzy matching, including the work-mode label so "remote"/"hybrid"
// queries hit rows whose only locality signal is locationType.
func bambooHRSearchLocations(row *bamboohr.ListJob, workMode string) string {
	parts := []string{
		row.Location.City.Or(""),
		row.Location.State.Or(""),
		row.AtsLocation.City.Or(""),
		row.AtsLocation.State.Or(""),
		row.AtsLocation.Province.Or(""),
		row.AtsLocation.Country.Or(""),
		workMode,
	}
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "; ")
}

func bambooHRJoin(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}

func bambooHRDescription(content string) string {
	if content == "" {
		return ""
	}
	text, err := html2text.FromString(content, html2text.Options{})
	if err != nil {
		return content
	}
	return text
}
