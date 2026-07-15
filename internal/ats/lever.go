package ats

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"

	"github.com/amikai/openings-mcp/internal/provider/lever"
)

// LeverAdapter serves Lever-hosted companies. Lever's list endpoint dumps
// the whole board (its native filter params are exact-match only, verified
// useless for fuzzy search), so searching happens in searchDump.
type LeverAdapter struct {
	client *lever.Client
}

// leverHosts are Lever's public board hosts, including the EU variant.
var leverHosts = map[string]bool{
	"jobs.lever.co":    true,
	"jobs.eu.lever.co": true,
}

func NewLeverAdapter(baseURL string, hc *http.Client) (*LeverAdapter, error) {
	c, err := lever.NewClient(baseURL, lever.WithClient(hc))
	if err != nil {
		return nil, err
	}
	return &LeverAdapter{client: c}, nil
}

func (a *LeverAdapter) Name() string { return "lever" }

func (a *LeverAdapter) Roster() []CompanyInfo {
	infos := make([]CompanyInfo, 0, len(lever.Companies))
	for _, c := range lever.Companies {
		infos = append(infos, CompanyInfo{Slug: c.Site, Name: c.Name})
	}
	return infos
}

// ParseCareersURL recognizes Lever-hosted board URLs; the first path
// segment is the organization, which is already this adapter's slug form.
func (a *LeverAdapter) ParseCareersURL(u *url.URL) (string, bool) {
	if !leverHosts[strings.ToLower(u.Hostname())] {
		return "", false
	}
	org := firstPathSegment(u)
	return org, org != ""
}

func (a *LeverAdapter) Search(ctx context.Context, slug string, p SearchParams) (*SearchResult, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return searchDump(jobs, p)
}

func (a *LeverAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	jobs, err := a.dump(ctx, slug)
	if err != nil {
		return nil, err
	}
	return distinctFilters(jobs), nil
}

func (a *LeverAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	p, err := a.client.GetPosting(ctx, lever.GetPostingParams{Site: slug, PostingId: jobID})
	if err != nil {
		return nil, fmt.Errorf("lever: fetch posting %q for %q: %w", jobID, slug, err)
	}
	desc, err := leverDescription(p)
	if err != nil {
		return nil, err
	}
	return &JobDetail{
		JobID:       p.ID,
		Title:       p.Text.Value,
		Company:     cmp.Or(lever.CompaniesBySite[slug].Name, slug),
		Location:    leverLocations(p),
		PostedAt:    leverPostedAt(p),
		URL:         p.HostedUrl.Value,
		Description: desc,
	}, nil
}

// dump fetches the full board and reshapes it for the filter engine.
func (a *LeverAdapter) dump(ctx context.Context, slug string) ([]dumpJob, error) {
	postings, err := a.client.ListPostings(ctx, lever.ListPostingsParams{
		Site: slug,
		Mode: lever.ListPostingsModeJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("lever: list postings for %q: %w", slug, err)
	}
	jobs := make([]dumpJob, 0, len(postings))
	for _, p := range postings {
		description, err := leverDescription(&p)
		if err != nil {
			return nil, err
		}
		cat := p.Categories.Value
		fields := map[string][]string{}
		if cat.Team.Value != "" {
			fields["team"] = []string{cat.Team.Value}
		}
		if cat.Department.Value != "" {
			fields["department"] = []string{cat.Department.Value}
		}
		if cat.Commitment.Value != "" {
			fields["commitment"] = []string{cat.Commitment.Value}
		}
		if p.WorkplaceType.Value != "" {
			fields["workplaceType"] = []string{p.WorkplaceType.Value}
		}
		jobs = append(jobs, dumpJob{
			summary: JobSummary{
				JobID:    p.ID,
				Title:    p.Text.Value,
				Location: cat.Location.Value,
				PostedAt: leverPostedAt(&p),
				URL:      p.HostedUrl.Value,
			},
			sortKey:     time.UnixMilli(p.CreatedAt.Value).UTC(),
			orgUnit:     cat.Team.Value + " " + cat.Department.Value,
			description: description,
			locations:   cat.Location.Value + " " + strings.Join(cat.AllLocations, " "),
			fields:      fields,
			isRemote:    strings.EqualFold(p.WorkplaceType.Value, "remote"),
		})
	}
	return jobs, nil
}

// leverDescription assembles the full plain-text JD from Lever's sectioned
// fields: opening+body (descriptionPlain), then each list section (its
// content is HTML), then the closing (additionalPlain).
func leverDescription(p *lever.Posting) (string, error) {
	var parts []string
	if s := strings.TrimSpace(p.DescriptionPlain.Value); s != "" {
		parts = append(parts, s)
	}
	for _, l := range p.Lists {
		content, err := html2text.FromString(l.Content.Value, html2text.Options{})
		if err != nil {
			return "", fmt.Errorf("lever: convert list section %q: %w", l.Text.Value, err)
		}
		parts = append(parts, l.Text.Value+"\n"+content)
	}
	if s := strings.TrimSpace(p.AdditionalPlain.Value); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n\n"), nil
}

func leverLocations(p *lever.Posting) string {
	cat := p.Categories.Value
	if len(cat.AllLocations) > 0 {
		return strings.Join(cat.AllLocations, "; ")
	}
	return cat.Location.Value
}

func leverPostedAt(p *lever.Posting) string {
	v, ok := p.CreatedAt.Get()
	if !ok {
		return ""
	}
	return isoDate(time.UnixMilli(v))
}
