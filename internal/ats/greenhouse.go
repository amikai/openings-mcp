package ats

import (
	"cmp"
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/greenhouse"
)

// GreenhouseAdapter serves Greenhouse-hosted companies. The Job Board API
// has no server-side search, so Search fetches the whole board — with
// content=true, since JD text and department/office dimensions only exist
// on the content variant — and filters it via searchDump. Detail uses the
// per-job endpoint, one light request.
type GreenhouseAdapter struct {
	client *greenhouse.Client
}

// greenhouseHosts are the public board hosts Greenhouse serves careers
// pages from, including the EU data-residency variants.
var greenhouseHosts = map[string]bool{
	"job-boards.greenhouse.io":    true,
	"boards.greenhouse.io":        true,
	"job-boards.eu.greenhouse.io": true,
	"boards.eu.greenhouse.io":     true,
}

func NewGreenhouseAdapter(baseURL string, hc *http.Client) (*GreenhouseAdapter, error) {
	c, err := greenhouse.NewClient(baseURL, greenhouse.WithClient(hc))
	if err != nil {
		return nil, err
	}
	return &GreenhouseAdapter{client: c}, nil
}

func (a *GreenhouseAdapter) Name() string { return "greenhouse" }

func (a *GreenhouseAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(greenhouse.Companies))
	for _, c := range greenhouse.Companies {
		infos = append(infos, CompanyInfo{Slug: c.BoardToken, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes Greenhouse-hosted board URLs; the first path
// segment is the board token, which is already this adapter's slug form.
func (a *GreenhouseAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if !greenhouseHosts[strings.ToLower(u.Hostname())] {
		return "", false
	}
	token := firstPathSegment(u)
	return token, token != ""
}

func (a *GreenhouseAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *GreenhouseAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *GreenhouseAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	id, err := strconv.Atoi(jobID)
	if err != nil {
		return nil, errGreenhouseJobNotFound(slug, jobID)
	}
	res, err := a.client.GetJob(ctx, greenhouse.GetJobParams{BoardToken: slug, JobID: id})
	if err != nil {
		return nil, fmt.Errorf("greenhouse: fetch job %q for %q: %w", jobID, slug, err)
	}
	switch r := res.(type) {
	case *greenhouse.JobDetail:
		return &JobDetail{
			JobID:       jobID,
			Title:       r.Title.Value,
			Company:     cmp.Or(greenhouse.CompaniesByBoardToken[strings.ToLower(slug)].Name, slug),
			Location:    r.Location.Value.Name.Value,
			PostedAt:    greenhousePostedAt(r.FirstPublished),
			URL:         r.AbsoluteURL.Value.String(),
			Description: greenhouseDescription(r.Content.Value),
		}, nil
	case *greenhouse.GetJobNotFound:
		return nil, errGreenhouseJobNotFound(slug, jobID)
	default:
		return nil, fmt.Errorf("greenhouse: unexpected response type %T", res)
	}
}

// errGreenhouseJobNotFound is the one teaching error for both a malformed
// and an unknown job id — the LLM's fix is the same either way.
func errGreenhouseJobNotFound(slug, jobID string) error {
	return fmt.Errorf("greenhouse: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
}

// dump fetches the full board with content and reshapes it for the filter
// engine.
func (a *GreenhouseAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	res, err := a.client.ListJobs(ctx, greenhouse.ListJobsParams{
		BoardToken: slug,
		Content:    greenhouse.NewOptBool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("greenhouse: list jobs for %q: %w", slug, err)
	}
	var list *greenhouse.JobListResponse
	switch r := res.(type) {
	case *greenhouse.JobListResponse:
		list = r
	case *greenhouse.ListJobsNotFound:
		return nil, fmt.Errorf("greenhouse: board %q not found upstream", slug)
	default:
		return nil, fmt.Errorf("greenhouse: unexpected response type %T", res)
	}
	jobs := make([]dumpJob, 0, len(list.Jobs))
	for _, j := range list.Jobs {
		depts := make([]string, 0, len(j.Departments))
		for _, d := range j.Departments {
			if d.Name.Value != "" {
				depts = append(depts, d.Name.Value)
			}
		}
		offices := make([]string, 0, len(j.Offices))
		for _, o := range j.Offices {
			if o.Name.Value != "" {
				offices = append(offices, o.Name.Value)
			}
		}
		// A job can sit in several departments/offices; keep them all so
		// filters match secondary values and get_filters lists them.
		fields := map[string][]string{}
		if len(depts) > 0 {
			fields["department"] = depts
		}
		if len(offices) > 0 {
			fields["office"] = offices
		}
		loc := j.Location.Value.Name.Value
		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    strconv.Itoa(j.ID.Value),
				Title:    j.Title.Value,
				Location: loc,
				PostedAt: greenhousePostedAt(j.FirstPublished),
				URL:      j.AbsoluteURL.Value.String(),
			},
			sortKey:     j.FirstPublished.Value,
			orgUnit:     strings.Join(depts, " "),
			description: greenhouseDescription(j.Content.Value),
			locations:   strings.Join(append([]string{loc}, offices...), "; "),
			fields:      fields,
			// isRemote stays false: Greenhouse has no remote field, and
			// matchLocation's "remote" query already falls back to
			// substring-matching the locations text (documented
			// best-effort in the unified-search spec).
		})
	}
	return jobs, nil
}

// greenhouseDescription converts the content field — HTML that Greenhouse
// additionally entity-encodes — to plain text. Falls back to the decoded
// HTML on conversion failure rather than failing the whole dump.
func greenhouseDescription(content string) string {
	if content == "" {
		return ""
	}
	decoded := html.UnescapeString(content)
	text, err := html2text.FromString(decoded, html2text.Options{})
	if err != nil {
		return decoded
	}
	return text
}

func greenhousePostedAt(t greenhouse.OptNilDateTime) string {
	v, ok := t.Get()
	if !ok {
		return ""
	}
	return isoDate(v)
}
