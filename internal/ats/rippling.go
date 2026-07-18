package ats

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/rippling"
)

// RipplingAdapter serves Rippling-hosted companies. The Job Board API has
// no server-side search, so Search fetches the whole board and filters it
// via searchDump. The list carries no JD text and no timestamps, so query
// matching covers titles and departments only, and ordering falls back to
// job id. Detail uses the per-job endpoint, one light request.
type RipplingAdapter struct {
	client *rippling.Client
}

// ripplingCareersURLRE matches Rippling board URLs and captures the board
// slug (first path segment).
//
// Examples (hostname + escaped path):
//   - ats.rippling.com/pythian/jobs
//   - ats.rippling.com/boom-supersonic/jobs/144f31c4-...
var ripplingCareersURLRE = regexp.MustCompile(
	`(?i)^ats\.rippling\.com/(?P<slug>[^/]+)`,
)

func NewRipplingAdapter(baseURL string, hc *http.Client) (*RipplingAdapter, error) {
	c, err := rippling.NewClient(baseURL, rippling.WithClient(hc))
	if err != nil {
		return nil, err
	}
	return &RipplingAdapter{client: c}, nil
}

func (a *RipplingAdapter) Name() string { return "rippling" }

func (a *RipplingAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(rippling.Companies))
	for _, c := range rippling.Companies {
		infos = append(infos, CompanyInfo{Slug: c.BoardSlug, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes Rippling-hosted board URLs; the first path
// segment is the board slug. Rippling mints lowercase slugs and its API is
// case-sensitive, so the captured segment is lowercased before use.
func (a *RipplingAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	slug, ok := matchCareersSlug(ripplingCareersURLRE, u)
	return strings.ToLower(slug), ok
}

func (a *RipplingAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *RipplingAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *RipplingAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	res, err := a.client.GetJob(ctx, rippling.GetJobParams{BoardSlug: slug, JobUUID: jobID})
	if err != nil {
		return nil, fmt.Errorf("rippling: fetch job %q for %q: %w", jobID, slug, err)
	}
	switch r := res.(type) {
	case *rippling.JobDetail:
		return &JobDetail{
			JobID: jobID,
			Title: r.Name.Value,
			// The detail's own companyName is authoritative; the roster
			// name only covers boards the API leaves it empty for.
			Company:     cmp.Or(r.CompanyName.Value, rippling.CompaniesByBoardSlug[strings.ToLower(slug)].Name, slug),
			Location:    strings.Join(r.WorkLocations, "; "),
			PostedAt:    ripplingPostedAt(r.CreatedOn),
			URL:         r.URL.Value.String(),
			Description: ripplingDescription(r.Description.Value),
		}, nil
	case *rippling.JobNotFoundError:
		// One teaching error for both a malformed and an unknown job id —
		// the upstream 404s identically and the LLM's fix is the same
		// either way.
		return nil, fmt.Errorf("rippling: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
	default:
		return nil, fmt.Errorf("rippling: unexpected response type %T", res)
	}
}

// dump fetches the full board and reshapes it for the filter engine,
// merging the upstream's one-entry-per-(job, location) rows into one
// dumpJob per posting.
func (a *RipplingAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	res, err := a.client.ListJobs(ctx, rippling.ListJobsParams{BoardSlug: slug})
	if err != nil {
		return nil, fmt.Errorf("rippling: list jobs for %q: %w", slug, err)
	}
	var entries []rippling.JobListEntry
	switch r := res.(type) {
	case *rippling.ListJobsOKApplicationJSON:
		entries = []rippling.JobListEntry(*r)
	case *rippling.BoardNotFoundError:
		return nil, fmt.Errorf("rippling: board %q not found upstream", slug)
	default:
		return nil, fmt.Errorf("rippling: unexpected response type %T", res)
	}

	byID := make(map[string]int, len(entries))
	jobs := make([]dumpJob, 0, len(entries))
	for _, e := range entries {
		loc := e.WorkLocation.Value.Label.Value
		if i, ok := byID[e.UUID.Value]; ok {
			// Duplicate row for a job already seen: merge its location.
			if loc != "" {
				jobs[i].locations += "; " + loc
				jobs[i].summary.Location = jobs[i].locations
			}
			continue
		}
		byID[e.UUID.Value] = len(jobs)
		fields := map[string][]string{}
		dept := e.Department.Value.Label.Value
		if dept != "" {
			fields["department"] = []string{dept}
		}
		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    e.UUID.Value,
				Title:    e.Name.Value,
				Location: loc,
				// PostedAt stays empty: the list carries no timestamps,
				// and detail-fetching every job would turn one dump call
				// into N.
				URL: e.URL.Value.String(),
			},
			orgUnit:   dept,
			locations: loc,
			fields:    fields,
			// sortKey stays zero for the same no-timestamp reason;
			// searchDump's ordering falls back to job id, which is still
			// deterministic. isRemote stays false: Rippling has no remote
			// flag, and matchLocation's "remote" query already falls back
			// to substring-matching the locations text (e.g. "Remote
			// (United States)").
		})
	}
	return jobs, nil
}

// ripplingDescription joins the JD's company blurb and role text — both
// HTML — and converts them to plain text. Falls back to the raw HTML on
// conversion failure rather than failing the whole detail.
func ripplingDescription(d rippling.Description) string {
	parts := make([]string, 0, 2)
	for _, p := range []string{d.Company.Value, d.Role.Value} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	joined := strings.Join(parts, "\n\n")
	text, err := html2text.FromString(joined, html2text.Options{})
	if err != nil {
		return joined
	}
	return text
}

func ripplingPostedAt(t rippling.OptDateTime) string {
	v, ok := t.Get()
	if !ok {
		return ""
	}
	return isoDate(v)
}
