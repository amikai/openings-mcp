package ats

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/greenhouse"
)

// GreenhouseAdapter serves Greenhouse companies. Search fetches the full board
// with content=true because descriptions, departments, and offices only exist
// in that variant; the API has no server-side filtering.
type GreenhouseAdapter struct {
	client *greenhouse.Client
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

func (a *GreenhouseAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	return searchViaDump(ctx, a.dump, slug, p)
}

func (a *GreenhouseAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	return filtersViaDump(ctx, a.dump, slug)
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
			Company:     greenhouse.CompaniesByBoardToken[strings.ToLower(slug)].Name,
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

// errGreenhouseJobNotFound gives the same recovery hint for malformed and
// unknown job IDs.
func errGreenhouseJobNotFound(slug, jobID string) error {
	return fmt.Errorf("greenhouse: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
}

// dump fetches the full content board and reshapes it for the filter engine.
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
		fields := map[string]string{}
		if len(depts) > 0 {
			fields["department"] = depts[0]
		}
		if len(offices) > 0 {
			fields["office"] = offices[0]
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
			// Greenhouse has no remote field; location matching still handles
			// "remote" as a best-effort text search.
		})
	}
	return jobs, nil
}

// greenhouseDescription decodes and converts HTML content to plain text,
// falling back to decoded HTML if conversion fails.
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

func greenhousePostedAt(t greenhouse.OptDateTime) string {
	if !t.Set {
		return ""
	}
	return isoDate(t.Value)
}
