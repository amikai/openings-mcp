package ats

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/amikai/openings-mcp/internal/provider/smartrecruiters"
)

var _ Adapter = (*SmartRecruitersAdapter)(nil)

// SmartRecruitersAdapter serves SmartRecruiters-hosted companies via the
// public Posting API. Search runs server-side: the unified Location folds
// into the q param (which full-text matches titles and location text), and
// department filter labels resolve to ids via one departments call when
// set — the stateless price, like Workday's facet probe.
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
	return nil, errors.New("smartrecruiters: Search not implemented yet")
}

func (a *SmartRecruitersAdapter) Filters(ctx context.Context, slug string) (FilterSet, error) {
	return nil, errors.New("smartrecruiters: Filters not implemented yet")
}

func (a *SmartRecruitersAdapter) Detail(ctx context.Context, slug, jobID string) (*JobDetail, error) {
	return nil, errors.New("smartrecruiters: Detail not implemented yet")
}
