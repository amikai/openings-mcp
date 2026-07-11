package ats

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/amikai/openings-mcp/internal/provider/ashby"
)

// AshbyAdapter serves Ashby-hosted companies. Ashby's public API is a
// single full-board endpoint — no server-side search and no per-job
// endpoint (returns 401) — so Search filters the dump via searchDump and
// Detail refetches the board and picks the one job out. The refetch is a
// bandwidth cost between this server and Ashby, invisible to the client.
type AshbyAdapter struct {
	client *ashby.Client
}

func NewAshbyAdapter(baseURL string, hc *http.Client) (*AshbyAdapter, error) {
	c, err := ashby.NewClient(baseURL, ashby.WithClient(hc))
	if err != nil {
		return nil, err
	}
	return &AshbyAdapter{client: c}, nil
}

func (a *AshbyAdapter) Name() string { return "ashby" }

func (a *AshbyAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(ashby.Companies))
	for _, c := range ashby.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Board, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes Ashby-hosted board URLs; the first path
// segment is the organization name, which is already this adapter's slug
// form (URL-decoded — Ashby org names may contain spaces).
func (a *AshbyAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if strings.ToLower(u.Hostname()) != "jobs.ashbyhq.com" {
		return "", false
	}
	org := firstPathSegment(u)
	return org, org != ""
}

func (a *AshbyAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *AshbyAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *AshbyAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	board, err := a.board(ctx, slug)
	if err != nil {
		return nil, err
	}
	for _, j := range board.Jobs {
		if j.ID.Value != jobID {
			continue
		}
		return &JobDetail{
			JobID:       j.ID.Value,
			Title:       j.Title.Value,
			Company:     cmp.Or(ashby.CompaniesByBoard[slug].Name, slug),
			Location:    ashbyLocations(j),
			PostedAt:    ashbyPostedAt(j.PublishedAt),
			URL:         j.JobUrl.Value,
			Description: j.DescriptionPlain.Value,
		}, nil
	}
	return nil, fmt.Errorf("ashby: job %q not found for company %q; pass a job_id exactly as returned by the job search", jobID, slug)
}

// board fetches the full job board, unwrapping ogen's union response.
func (a *AshbyAdapter) board(ctx context.Context, slug string) (*ashby.JobBoardResponse, error) {
	res, err := a.client.GetJobBoard(ctx, ashby.GetJobBoardParams{JobBoardName: slug})
	if err != nil {
		return nil, fmt.Errorf("ashby: fetch board %q: %w", slug, err)
	}
	switch r := res.(type) {
	case *ashby.JobBoardResponse:
		return r, nil
	case *ashby.GetJobBoardNotFound:
		return nil, fmt.Errorf("ashby: board %q not found upstream", slug)
	default:
		return nil, fmt.Errorf("ashby: unexpected response type %T", res)
	}
}

func (a *AshbyAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	board, err := a.board(ctx, slug)
	if err != nil {
		return nil, err
	}
	jobs := make([]dumpJob, 0, len(board.Jobs))
	for _, j := range board.Jobs {
		if !j.IsListed.Value {
			continue
		}
		fields := map[string][]string{}
		if j.Department.Value != "" {
			fields["department"] = []string{j.Department.Value}
		}
		if j.Team.Value != "" {
			fields["team"] = []string{j.Team.Value}
		}
		if string(j.EmploymentType.Value) != "" {
			fields["employmentType"] = []string{string(j.EmploymentType.Value)}
		}
		// Real boards send null for workplaceType/isRemote (see provider
		// fixture board_nulls_rsp.json); treat null as unspecified.
		if wt, ok := j.WorkplaceType.Get(); ok && string(wt) != "" {
			fields["workplaceType"] = []string{string(wt)}
		}
		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    j.ID.Value,
				Title:    j.Title.Value,
				Location: j.Location.Value,
				PostedAt: ashbyPostedAt(j.PublishedAt),
				URL:      j.JobUrl.Value,
			},
			sortKey:     j.PublishedAt.Value,
			orgUnit:     j.Department.Value + " " + j.Team.Value,
			description: j.DescriptionPlain.Value,
			locations:   ashbyLocations(j),
			fields:      fields,
			isRemote:    j.IsRemote.Or(false),
		})
	}
	return jobs, nil
}

// ashbyPostedAt guards against a present-but-null publishedAt: NilDateTime's
// zero Value would otherwise format as a fake date instead of the true
// "unknown" state.
func ashbyPostedAt(t ashby.NilDateTime) string {
	v, ok := t.Get()
	if !ok {
		return ""
	}
	return isoDate(v)
}

func ashbyLocations(j ashby.JobPosting) string {
	parts := make([]string, 0, 1+len(j.SecondaryLocations))
	parts = append(parts, j.Location.Value)
	for _, s := range j.SecondaryLocations {
		parts = append(parts, s.Location.Value)
	}
	return strings.Join(parts, "; ")
}
